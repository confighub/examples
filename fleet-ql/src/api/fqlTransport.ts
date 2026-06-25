// The app-side FQL Transport: implements the engine's abstract fetch operations
// over ConfigHub's REST API. This is the ONLY file that couples the portable
// fql/ engine to the app — it knows ConfigHub's result shapes and flattens them
// into FQL rows.
//
// HTTP goes through the generated, typed `cub` client (openapi-fetch over the
// OpenAPI `paths` map); request params and response bodies are checked against
// the spec, so there are no hand-written endpoint shapes here.
//
// Resource rows carry generic identity + the raw resource doc (__doc); FQL
// evaluates arbitrary YAML data paths (images, scanner annotations, etc.)
// against __doc client-side, so there are no domain-specific curated columns.

import { parseAllDocuments } from 'yaml';

import type {
  GrantsParams,
  ListParams,
  ResourceParams,
  RevisionParams,
  Row,
  Transport,
} from '../fql';
import { materializeGrants } from '../rbac/grants';
import type { FleetResource } from '../rbac/model';
import { materializeBindings, materializeFindings, materializeRoles } from '../rbac/structural';
import { cub, type Schemas } from '../sdk/client';
import { spreadGatesByTrigger } from './gates';
import { b64decodeUtf8 } from './encoding';

// Result shapes come from the generated OpenAPI components (authoritative).
type ExtendedUnit = Schemas['ExtendedUnit'];
type ExtendedSpace = Schemas['ExtendedSpace'];
type ExtendedRevision = Schemas['ExtendedRevision'];

// ─── Flattening: API shapes → FQL rows ──────────────────────────────────────

/** Spread a map under a `prefix.` key namespace so `labels.env` resolves. */
function spreadMap(row: Row, prefix: string, map?: Record<string, string>): void {
  if (!map) return;
  for (const [k, v] of Object.entries(map)) row[`${prefix}.${k}`] = v;
}

function unitRow(e: ExtendedUnit): Row {
  const u: Partial<Schemas['Unit']> = e.Unit ?? {};
  const row: Row = {
    __id: u.UnitID,
    slug: u.Slug,
    displayName: u.DisplayName,
    space: e.Space?.Slug,
    toolchain: u.ToolchainType,
    target: e.Target?.Slug ?? null,
    // cluster = deploy target, falling back to the Space slug for unbound units.
    cluster: e.Target?.Slug ?? e.Space?.Slug ?? null,
    headRevisionNum: u.HeadRevisionNum,
    liveRevisionNum: u.LiveRevisionNum,
    lastAppliedRevisionNum: u.LastAppliedRevisionNum,
    upstreamRevisionNum: u.UpstreamRevisionNum,
    upstreamUnitId: u.UpstreamUnitID,
    providerType: u.ProviderType,
    // Well-known fleet labels as flat fields, so they project / GROUP BY (the
    // `environment` etc. columns also push to where via Labels.* for filtering).
    environment: u.Labels?.Environment ?? null,
    component: u.Labels?.Component ?? null,
    region: u.Labels?.Region ?? null,
    gates: Object.keys(u.ApplyGates ?? {}).length,
    warnings: Object.keys(u.ApplyWarnings ?? {}).length,
  };
  spreadMap(row, 'labels', u.Labels);
  spreadMap(row, 'annotations', u.Annotations);
  // Gate/warning maps spread two ways: by the full key, so `applyGates['<key>']`
  // resolves client-side too; and re-keyed by trigger-slug, so `gate['<trigger>']`
  // works without knowing the fully-qualified key.
  for (const [k, v] of Object.entries(u.ApplyGates ?? {})) row[`applyGates.${k}`] = v;
  for (const [k, v] of Object.entries(u.ApplyWarnings ?? {})) row[`applyWarnings.${k}`] = v;
  spreadGatesByTrigger(row, 'gate', u.ApplyGates);
  spreadGatesByTrigger(row, 'warning', u.ApplyWarnings);
  return row;
}

const SELECT_UNIT =
  'UnitID,Slug,DisplayName,SpaceID,TargetID,ToolchainType,HeadRevisionNum,LiveRevisionNum,LastAppliedRevisionNum,UpstreamRevisionNum,UpstreamUnitID,ProviderType,Labels,Annotations,ApplyGates,ApplyWarnings';

// The get-resources invocation body, shared by the resources and RBAC paths.
const GET_RESOURCES: Schemas['FunctionInvocationsRequest']['FunctionInvocations'] = [
  { FunctionName: 'get-resources', Arguments: [{ ParameterName: 'body', Value: 'json' }] },
];

/** Decode the base64 ResourceList output of a get-resources invocation into the
 *  list of raw resources, or [] if it's missing / unparseable. */
function resourceList(
  resp: Schemas['FunctionInvocationsResponse'],
): { ResourceType?: string; ResourceName?: string; ResourceBody?: string }[] {
  if (!resp.Success) return [];
  const encoded = resp.Outputs?.['ResourceList'];
  if (!encoded) return [];
  try {
    return JSON.parse(b64decodeUtf8(encoded));
  } catch {
    return [];
  }
}

// ─── Transport implementation ────────────────────────────────────────────────

export const fqlTransport: Transport = {
  async units(params: ListParams): Promise<Row[]> {
    const { data, error, response } = await cub.GET('/unit', {
      params: { query: { where: params.where, select: SELECT_UNIT, include: 'SpaceID,TargetID' } },
    });
    if (error || !data) throw new Error(`/unit: HTTP ${response.status}`);
    return data.map(unitRow);
  },

  async spaces(params: ListParams): Promise<Row[]> {
    const { data, error, response } = await cub.GET('/space', {
      params: { query: { where: params.where } },
    });
    if (error || !data) throw new Error(`/space: HTTP ${response.status}`);
    return data.map((e: ExtendedSpace) => {
      const s: Partial<Schemas['Space']> = e.Space ?? {};
      const row: Row = {
        __id: s.SpaceID,
        slug: s.Slug,
        displayName: s.DisplayName,
        environment: s.Labels?.Environment ?? null,
        component: s.Labels?.Component ?? null,
        region: s.Labels?.Region ?? null,
      };
      spreadMap(row, 'labels', s.Labels);
      spreadMap(row, 'annotations', s.Annotations);
      return row;
    });
  },

  async revisions(params: RevisionParams): Promise<Row[]> {
    // Revisions are per-Unit. First find the in-scope units (whereUnit), then
    // fan out a revision fetch per unit (the endpoint's `where` filters revision
    // fields). Rows carry their owning unit/space for identity and grouping.
    const { data: units, error, response } = await cub.GET('/unit', {
      params: { query: { where: params.whereUnit, select: 'UnitID,Slug,SpaceID', include: 'SpaceID' } },
    });
    if (error || !units) throw new Error(`/unit: HTTP ${response.status}`);

    const perUnit = await Promise.all(
      units.map(async (e: ExtendedUnit): Promise<Row[]> => {
        const sid = e.Unit?.SpaceID;
        const uid = e.Unit?.UnitID;
        if (!sid || !uid) return [];
        const { data: revs } = await cub.GET('/space/{space_id}/unit/{unit_id}/revision', {
          params: { path: { space_id: sid, unit_id: uid }, query: { where: params.where } },
        });
        return (revs ?? []).map((r: ExtendedRevision): Row => {
          const rev: Partial<Schemas['Revision']> = r.Revision ?? {};
          return {
            unit: e.Unit?.Slug,
            space: e.Space?.Slug,
            revisionNum: rev.RevisionNum,
            source: rev.Source,
            description: rev.Description,
            createdAt: rev.CreatedAt,
            userId: rev.UserID,
          };
        });
      }),
    );
    return perUnit.flat();
  },

  async resources(params: ResourceParams): Promise<Row[]> {
    // Time-travel path: read each in-scope unit's resources AS OF a specific
    // revision (instead of head's get-resources). We resolve the unit, find the
    // target revision's ID, fetch its data blob, and parse the K8s docs out.
    if (params.revision !== undefined) {
      return resourcesAtRevision(params);
    }

    // get-resources over the matching Units, then explode each Unit's resource
    // list into one FQL row per Kubernetes resource. In parallel, resolve each
    // unit's cluster (deploy target, Space fallback) to stamp onto the rows.
    const [inv, clusters] = await Promise.all([
      cub.POST('/function/invoke', {
        params: { query: { where: params.where, where_data: params.whereData } },
        body: { WhereResource: params.whereResource, FunctionInvocations: GET_RESOURCES },
      }),
      unitClusterMap(params.where),
    ]);
    if (inv.error || !inv.data) throw new Error(`/function/invoke: HTTP ${inv.response.status}`);

    const rows: Row[] = [];
    for (const resp of inv.data) {
      const ci = clusters.get(`${resp.SpaceSlug ?? ''}/${resp.UnitSlug ?? ''}`);
      const origin: ResourceOrigin = {
        unit: resp.UnitSlug,
        space: resp.SpaceSlug,
        target: ci?.target ?? null,
        cluster: ci?.cluster ?? resp.SpaceSlug ?? null,
        environment: ci?.environment ?? null,
        component: ci?.component ?? null,
        region: ci?.region ?? null,
      };
      for (const raw of resourceList(resp)) {
        if (!raw.ResourceBody) continue;
        let doc: Record<string, unknown>;
        try {
          doc = JSON.parse(raw.ResourceBody);
        } catch {
          continue;
        }
        rows.push(resourceRow(origin, raw.ResourceType, doc, 'head'));
      }
    }
    return rows;
  },

  // The RBAC tables share one fetch+parse (fetchFleetRbac); each runs a
  // different materializer over the same FleetResource[].
  async grants(params: GrantsParams): Promise<Row[]> {
    return materializeGrants(await fetchFleetRbac(params.where), params.accessQuery ?? {});
  },

  async roles(params: ListParams): Promise<Row[]> {
    return materializeRoles(await fetchFleetRbac(params.where));
  },

  async bindings(params: ListParams): Promise<Row[]> {
    return materializeBindings(await fetchFleetRbac(params.where));
  },

  async rbacFindings(params: ListParams): Promise<Row[]> {
    return materializeFindings(await fetchFleetRbac(params.where));
  },
};

// ─── Shared RBAC fetch (grants / roles / bindings) ──────────────────────────

/** Fetch the in-scope units' resources (get-resources) + each unit's cluster,
 *  and assemble FleetResource[] for the rbac engine. We fetch all resources (no
 *  whereResource narrowing) and let buildClusterRbac keep only RBAC kinds —
 *  sound and simple; `where` (space) already narrows which units are read. */
async function fetchFleetRbac(where: string | undefined): Promise<FleetResource[]> {
  const [inv, clusters] = await Promise.all([
    cub.POST('/function/invoke', {
      params: { query: { where } },
      body: { FunctionInvocations: GET_RESOURCES },
    }),
    unitClusterMap(where),
  ]);
  if (inv.error || !inv.data) throw new Error(`/function/invoke: HTTP ${inv.response.status}`);

  const fleet: FleetResource[] = [];
  for (const resp of inv.data) {
    const ci = clusters.get(`${resp.SpaceSlug ?? ''}/${resp.UnitSlug ?? ''}`);
    for (const raw of resourceList(resp)) {
      if (!raw.ResourceBody) continue;
      let doc: unknown;
      try {
        doc = JSON.parse(raw.ResourceBody);
      } catch {
        continue;
      }
      fleet.push({
        origin: {
          cluster: ci?.cluster ?? resp.SpaceSlug ?? '',
          target: ci?.target ?? undefined,
          space: resp.SpaceSlug ?? '',
          spaceId: resp.SpaceID ?? '',
          unitId: resp.UnitID ?? '',
          unitSlug: resp.UnitSlug ?? '',
          resourceName: raw.ResourceName ?? '',
        },
        doc,
      });
    }
  }
  return fleet;
}

// ─── Time-travel: resources as of a specific revision ───────────────────────

const SELECT_UNIT_REV = 'UnitID,Slug,SpaceID,TargetID,Labels,HeadRevisionNum,LiveRevisionNum';

/** Read resources from a specific revision per in-scope unit (params.revision
 *  is a RevisionNum, or 'head' / 'live'). One row per K8s doc in that revision's
 *  data, stamped with the resolved RevisionNum. */
async function resourcesAtRevision(params: ResourceParams): Promise<Row[]> {
  const sel = params.revision!.toLowerCase();
  const { data: units, error, response } = await cub.GET('/unit', {
    params: { query: { where: params.where, select: SELECT_UNIT_REV, include: 'SpaceID,TargetID' } },
  });
  if (error || !units) throw new Error(`/unit: HTTP ${response.status}`);

  const perUnit = await Promise.all(
    units.map(async (e: ExtendedUnit): Promise<Row[]> => {
      const u = e.Unit;
      const sid = u?.SpaceID;
      const uid = u?.UnitID;
      if (!u || !sid || !uid) return [];

      // Resolve the target RevisionNum: symbolic head/live, else the literal.
      let revNum: number | undefined;
      if (sel === 'head') revNum = u.HeadRevisionNum;
      else if (sel === 'live') revNum = u.LiveRevisionNum;
      else if (/^\d+$/.test(sel)) revNum = Number(sel);
      if (revNum === undefined || revNum <= 0) return []; // never-applied 'live', etc.

      // Map RevisionNum → RevisionID (the data endpoint keys on the UUID).
      const { data: revs } = await cub.GET('/space/{space_id}/unit/{unit_id}/revision', {
        params: {
          path: { space_id: sid, unit_id: uid },
          query: { where: `RevisionNum = ${revNum}`, select: 'RevisionID,RevisionNum' },
        },
      });
      const revId = revs?.[0]?.Revision?.RevisionID;
      if (!revId) return [];

      // Fetch that revision's data blob (YAML) and split into resource docs.
      const { data: text, error: dErr } = await cub.GET(
        '/space/{space_id}/unit/{unit_id}/revision/{revision_id}/data',
        { params: { path: { space_id: sid, unit_id: uid, revision_id: revId } }, parseAs: 'text' },
      );
      if (dErr || text === undefined) return [];

      const origin: ResourceOrigin = {
        unit: u.Slug,
        space: e.Space?.Slug,
        target: e.Target?.Slug ?? null,
        cluster: e.Target?.Slug ?? e.Space?.Slug ?? null,
        environment: u.Labels?.Environment ?? null,
        component: u.Labels?.Component ?? null,
        region: u.Labels?.Region ?? null,
      };
      const out: Row[] = [];
      for (const d of parseAllDocuments(text)) {
        const doc = d.toJS() as Record<string, unknown> | null;
        if (!doc || typeof doc !== 'object' || !doc['kind']) continue;
        out.push(resourceRow(origin, resourceTypeOf(doc), doc, String(revNum)));
      }
      return out;
    }),
  );
  return perUnit.flat();
}

/** Where a resource came from: its owning Unit's identity and deploy target.
 *  cluster = target ?? space (mirrors the rbac model's origin.cluster). */
interface ResourceOrigin {
  unit?: string;
  space?: string;
  target?: string | null;
  cluster?: string | null;
  environment?: string | null;
  component?: string | null;
  region?: string | null;
}

/** One FQL row for a Kubernetes resource document. `revision` is the selector
 *  the rows were read at ('head' for the default get-resources path), stamped so
 *  the residual `revision = N` check matches. */
function resourceRow(
  origin: ResourceOrigin,
  resourceType: string | undefined,
  doc: Record<string, unknown>,
  revision: string,
): Row {
  const md = doc['metadata'] as { name?: string; namespace?: string } | undefined;
  const spec = doc['spec'] as { replicas?: number } | undefined;
  return {
    // Generic identity + single-valued K8s shorthands only. Domain fields like
    // image / severity / cveCount are NOT curated columns — query them as the
    // real paths they are (containers.*.image,
    // metadata.annotations['sec-scanner.confighub.com/max-severity']), evaluated
    // client-side against __doc.
    unit: origin.unit,
    space: origin.space,
    target: origin.target ?? null,
    cluster: origin.cluster ?? origin.space ?? null,
    environment: origin.environment ?? null,
    component: origin.component ?? null,
    region: origin.region ?? null,
    kind: doc['kind'],
    name: md?.name,
    namespace: md?.namespace ?? null,
    replicas: spec?.replicas ?? null,
    resourceType,
    revision,
    // The raw resource doc, so FQL can evaluate arbitrary YAML data paths
    // client-side. Reserved key (leading __) — excluded from SELECT * columns.
    __doc: doc,
  };
}

/** Per-unit origin enrichment keyed by `space/unit`, so the resources head path
 *  can stamp each resource with its cluster (deploy target, Space fallback) and
 *  the unit's well-known fleet labels. One extra list call (scoped by the same
 *  `where`). */
interface UnitOrigin {
  cluster: string | null;
  target: string | null;
  environment: string | null;
  component: string | null;
  region: string | null;
}

async function unitClusterMap(where: string | undefined): Promise<Map<string, UnitOrigin>> {
  const { data: units } = await cub.GET('/unit', {
    params: { query: { where, select: 'UnitID,Slug,SpaceID,TargetID,Labels', include: 'SpaceID,TargetID' } },
  });
  const m = new Map<string, UnitOrigin>();
  for (const e of units ?? []) {
    const space = e.Space?.Slug ?? '';
    const target = e.Target?.Slug ?? null;
    const labels = e.Unit?.Labels ?? {};
    m.set(`${space}/${e.Unit?.Slug ?? ''}`, {
      cluster: target ?? e.Space?.Slug ?? null,
      target,
      environment: labels.Environment ?? null,
      component: labels.Component ?? null,
      region: labels.Region ?? null,
    });
  }
  return m;
}

/** ResourceType ("apiVersion/kind") for a parsed K8s doc, matching ConfigHub's
 *  form, so the resourceType column and whereResource line up. */
function resourceTypeOf(doc: Record<string, unknown>): string | undefined {
  const apiVersion = doc['apiVersion'];
  const kind = doc['kind'];
  if (typeof apiVersion === 'string' && typeof kind === 'string') return `${apiVersion}/${kind}`;
  return typeof kind === 'string' ? kind : undefined;
}
