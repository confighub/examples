// The app-side FQL Transport: implements the engine's abstract fetch operations
// over ConfigHub's REST API (same-origin /api + bearer token from api/auth).
// This is the ONLY file that couples the portable fql/ engine to the app — it
// knows ConfigHub's result shapes and flattens them into FQL rows.
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
import { materializeBindings, materializeRoles } from '../rbac/structural';
import { authHeaders } from './auth';
import { b64decodeUtf8 } from './encoding';

// ─── HTTP helpers (auth via api/auth.ts) ──────────────────────────────────

async function getJson<T>(path: string, query: Record<string, string | undefined>): Promise<T> {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) if (v !== undefined && v !== '') qs.set(k, v);
  const url = qs.toString() ? `${path}?${qs}` : path;
  const res = await fetch(url, { credentials: 'include', headers: authHeaders() });
  if (!res.ok) throw new Error(`${path}: HTTP ${res.status}`);
  return res.json() as Promise<T>;
}

async function getText(path: string): Promise<string> {
  const res = await fetch(path, { credentials: 'include', headers: authHeaders() });
  if (!res.ok) throw new Error(`${path}: HTTP ${res.status}`);
  return res.text();
}

async function postJson<T>(path: string, query: Record<string, string | undefined>, body: unknown): Promise<T> {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) if (v !== undefined && v !== '') qs.set(k, v);
  const url = qs.toString() ? `${path}?${qs}` : path;
  const res = await fetch(url, {
    method: 'POST',
    credentials: 'include',
    headers: authHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`${path}: HTTP ${res.status}`);
  return res.json() as Promise<T>;
}

// ─── Result-shape types (subset of the SDK) ─────────────────────────────────

interface UnitListEntry {
  Unit?: {
    Slug?: string;
    UnitID?: string;
    SpaceID?: string;
    DisplayName?: string;
    ToolchainType?: string;
    HeadRevisionNum?: number;
    LiveRevisionNum?: number;
    LastAppliedRevisionNum?: number;
    UpstreamRevisionNum?: number;
    UpstreamUnitID?: string;
    ProviderType?: string;
    Labels?: Record<string, string>;
    Annotations?: Record<string, string>;
    ApplyGates?: Record<string, boolean>;
    ApplyWarnings?: Record<string, boolean>;
  };
  Space?: { Slug?: string };
  Target?: { Slug?: string };
}

interface SpaceListEntry {
  Space?: { Slug?: string; DisplayName?: string; SpaceID?: string; Labels?: Record<string, string>; Annotations?: Record<string, string> };
}

interface RevisionListEntry {
  Revision?: {
    RevisionID?: string;
    RevisionNum?: number;
    Source?: string;
    Description?: string;
    CreatedAt?: string;
    UserID?: string;
    DataHash?: string;
  };
}

interface InvokeResponse {
  Success?: boolean;
  UnitID?: string;
  UnitSlug?: string;
  SpaceID?: string;
  SpaceSlug?: string;
  Outputs?: Record<string, string> | null;
}

// ─── Flattening: API shapes → FQL rows ──────────────────────────────────────

/** Spread a map under a `prefix.` key namespace so `labels.env` resolves. */
function spreadMap(row: Row, prefix: string, map?: Record<string, string>): void {
  if (!map) return;
  for (const [k, v] of Object.entries(map)) row[`${prefix}.${k}`] = v;
}

function unitRow(e: UnitListEntry): Row {
  const u = e.Unit ?? {};
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
    gates: Object.keys(u.ApplyGates ?? {}).length,
    warnings: Object.keys(u.ApplyWarnings ?? {}).length,
  };
  spreadMap(row, 'labels', u.Labels);
  spreadMap(row, 'annotations', u.Annotations);
  // Gate/warning maps spread so `applyGates['<key>']` resolves client-side too.
  for (const [k, v] of Object.entries(u.ApplyGates ?? {})) row[`applyGates.${k}`] = v;
  for (const [k, v] of Object.entries(u.ApplyWarnings ?? {})) row[`applyWarnings.${k}`] = v;
  return row;
}

const SELECT_UNIT =
  'UnitID,Slug,DisplayName,SpaceID,TargetID,ToolchainType,HeadRevisionNum,LiveRevisionNum,LastAppliedRevisionNum,UpstreamRevisionNum,UpstreamUnitID,ProviderType,Labels,Annotations,ApplyGates,ApplyWarnings';

// ─── Transport implementation ────────────────────────────────────────────────

export const fqlTransport: Transport = {
  async units(params: ListParams): Promise<Row[]> {
    const entries = await getJson<UnitListEntry[]>('/api/unit', {
      where: params.where,
      select: SELECT_UNIT,
      include: 'SpaceID,TargetID',
    });
    return entries.map(unitRow);
  },

  async spaces(params: ListParams): Promise<Row[]> {
    const entries = await getJson<SpaceListEntry[]>('/api/space', { where: params.where });
    return entries.map((e) => {
      const s = e.Space ?? {};
      const row: Row = { __id: s.SpaceID, slug: s.Slug, displayName: s.DisplayName };
      spreadMap(row, 'labels', s.Labels);
      spreadMap(row, 'annotations', s.Annotations);
      return row;
    });
  },

  async revisions(params: RevisionParams): Promise<Row[]> {
    // Revisions are per-Unit. First find the in-scope units (whereUnit), then
    // fan out a revision fetch per unit (the endpoint's `where` filters revision
    // fields). Rows carry their owning unit/space for identity and grouping.
    const units = await getJson<UnitListEntry[]>('/api/unit', {
      where: params.whereUnit,
      select: 'UnitID,Slug,SpaceID',
      include: 'SpaceID',
    });

    const perUnit = await Promise.all(
      units.map(async (e): Promise<Row[]> => {
        const sid = e.Unit?.SpaceID;
        const uid = e.Unit?.UnitID;
        if (!sid || !uid) return [];
        const revs = await getJson<RevisionListEntry[]>(
          `/api/space/${sid}/unit/${uid}/revision`,
          { where: params.where },
        );
        return revs.map((r): Row => {
          const rev = r.Revision ?? {};
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
    const [responses, clusters] = await Promise.all([
      postJson<InvokeResponse[]>(
        '/api/function/invoke',
        { where: params.where, where_data: params.whereData },
        {
          WhereResource: params.whereResource,
          FunctionInvocations: [
            { FunctionName: 'get-resources', Arguments: [{ ParameterName: 'body', Value: 'json' }] },
          ],
        },
      ),
      unitClusterMap(params.where),
    ]);

    const rows: Row[] = [];
    for (const resp of responses) {
      if (!resp.Success) continue;
      const encoded = resp.Outputs?.['ResourceList'];
      if (!encoded) continue;
      let list: { ResourceType?: string; ResourceName?: string; ResourceBody?: string }[];
      try {
        list = JSON.parse(b64decodeUtf8(encoded));
      } catch {
        continue;
      }
      const ci = clusters.get(`${resp.SpaceSlug ?? ''}/${resp.UnitSlug ?? ''}`);
      const origin: ResourceOrigin = {
        unit: resp.UnitSlug,
        space: resp.SpaceSlug,
        target: ci?.target ?? null,
        cluster: ci?.cluster ?? resp.SpaceSlug ?? null,
      };
      for (const raw of list) {
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
};

// ─── Shared RBAC fetch (grants / roles / bindings) ──────────────────────────

/** Fetch the in-scope units' resources (get-resources) + each unit's cluster,
 *  and assemble FleetResource[] for the rbac engine. We fetch all resources (no
 *  whereResource narrowing) and let buildClusterRbac keep only RBAC kinds —
 *  sound and simple; `where` (space) already narrows which units are read. */
async function fetchFleetRbac(where: string | undefined): Promise<FleetResource[]> {
  const [responses, clusters] = await Promise.all([
    postJson<InvokeResponse[]>(
      '/api/function/invoke',
      { where },
      {
        FunctionInvocations: [
          { FunctionName: 'get-resources', Arguments: [{ ParameterName: 'body', Value: 'json' }] },
        ],
      },
    ),
    unitClusterMap(where),
  ]);

  const fleet: FleetResource[] = [];
  for (const resp of responses) {
    if (!resp.Success) continue;
    const encoded = resp.Outputs?.['ResourceList'];
    if (!encoded) continue;
    let list: { ResourceType?: string; ResourceName?: string; ResourceBody?: string }[];
    try {
      list = JSON.parse(b64decodeUtf8(encoded));
    } catch {
      continue;
    }
    const ci = clusters.get(`${resp.SpaceSlug ?? ''}/${resp.UnitSlug ?? ''}`);
    for (const raw of list) {
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

const SELECT_UNIT_REV = 'UnitID,Slug,SpaceID,TargetID,HeadRevisionNum,LiveRevisionNum';

/** Read resources from a specific revision per in-scope unit (params.revision
 *  is a RevisionNum, or 'head' / 'live'). One row per K8s doc in that revision's
 *  data, stamped with the resolved RevisionNum. */
async function resourcesAtRevision(params: ResourceParams): Promise<Row[]> {
  const sel = params.revision!.toLowerCase();
  const units = await getJson<UnitListEntry[]>('/api/unit', {
    where: params.where,
    select: SELECT_UNIT_REV,
    include: 'SpaceID,TargetID',
  });

  const perUnit = await Promise.all(
    units.map(async (e): Promise<Row[]> => {
      const u = e.Unit;
      const sid = u?.SpaceID;
      const uid = u?.UnitID;
      if (!sid || !uid) return [];

      // Resolve the target RevisionNum: symbolic head/live, else the literal.
      let revNum: number | undefined;
      if (sel === 'head') revNum = u.HeadRevisionNum;
      else if (sel === 'live') revNum = u.LiveRevisionNum;
      else if (/^\d+$/.test(sel)) revNum = Number(sel);
      if (revNum === undefined || revNum <= 0) return []; // never-applied 'live', etc.

      // Map RevisionNum → RevisionID (the data endpoint keys on the UUID).
      const revs = await getJson<RevisionListEntry[]>(
        `/api/space/${sid}/unit/${uid}/revision`,
        { where: `RevisionNum = ${revNum}`, select: 'RevisionID,RevisionNum' },
      );
      const revId = revs[0]?.Revision?.RevisionID;
      if (!revId) return [];

      // Fetch that revision's data blob (YAML) and split into resource docs.
      let text: string;
      try {
        text = await getText(`/api/space/${sid}/unit/${uid}/revision/${revId}/data`);
      } catch {
        return [];
      }
      const origin: ResourceOrigin = {
        unit: u.Slug,
        space: e.Space?.Slug,
        target: e.Target?.Slug ?? null,
        cluster: e.Target?.Slug ?? e.Space?.Slug ?? null,
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

/** Fetch the matching units' deploy targets, keyed by `space/unit`, so the
 *  resources head path can stamp each resource with its cluster. One extra list
 *  call (scoped by the same `where`); cluster = target ?? space. */
async function unitClusterMap(
  where: string | undefined,
): Promise<Map<string, { cluster: string | null; target: string | null }>> {
  const units = await getJson<UnitListEntry[]>('/api/unit', {
    where,
    select: 'UnitID,Slug,SpaceID,TargetID',
    include: 'SpaceID,TargetID',
  });
  const m = new Map<string, { cluster: string | null; target: string | null }>();
  for (const e of units) {
    const space = e.Space?.Slug ?? '';
    const target = e.Target?.Slug ?? null;
    m.set(`${space}/${e.Unit?.Slug ?? ''}`, { cluster: target ?? e.Space?.Slug ?? null, target });
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
