// The app-side FQL Transport: implements the engine's abstract fetch operations
// over ConfigHub's REST API (same-origin /api + bearer token, exactly like
// raw.ts). This is the ONLY file that couples the portable fql/ engine to the
// app — it knows ConfigHub's result shapes and flattens them into FQL rows.
//
// Resource rows carry generic identity + the raw resource doc (__doc); FQL
// evaluates arbitrary YAML data paths (images, scanner annotations, etc.)
// against __doc client-side, so there are no domain-specific curated columns.

import type { ListParams, ResourceParams, Row, Transport } from '../fql';
import { b64decodeUtf8 } from './encoding';
import { getStoredToken } from '../sdk/confighubapi';

// ─── HTTP helpers (mirror raw.ts auth) ──────────────────────────────────────

function authHeaders(extra?: Record<string, string>): HeadersInit {
  const token = getStoredToken();
  return {
    ...(extra ?? {}),
    ...(token !== null ? { Authorization: `Bearer ${token}` } : {}),
  };
}

async function getJson<T>(path: string, query: Record<string, string | undefined>): Promise<T> {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) if (v !== undefined && v !== '') qs.set(k, v);
  const url = qs.toString() ? `${path}?${qs}` : path;
  const res = await fetch(url, { credentials: 'include', headers: authHeaders() });
  if (!res.ok) throw new Error(`${path}: HTTP ${res.status}`);
  return res.json() as Promise<T>;
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

interface TargetListEntry {
  Target?: { Slug?: string; DisplayName?: string; TargetID?: string; Labels?: Record<string, string> };
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
    headRev: u.HeadRevisionNum,
    gates: Object.keys(u.ApplyGates ?? {}).length,
    warnings: Object.keys(u.ApplyWarnings ?? {}).length,
  };
  spreadMap(row, 'labels', u.Labels);
  spreadMap(row, 'annotations', u.Annotations);
  return row;
}

const SELECT_UNIT =
  'UnitID,Slug,DisplayName,SpaceID,TargetID,ToolchainType,HeadRevisionNum,Labels,Annotations,ApplyGates,ApplyWarnings';

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

  async targets(params: ListParams): Promise<Row[]> {
    // Targets are scoped per-space in the API; query org-wide via the unit
    // endpoint's sibling /target list is not available, so use the flat list.
    const entries = await getJson<TargetListEntry[]>('/api/target', { where: params.where });
    return entries.map((e) => {
      const t = e.Target ?? {};
      const row: Row = { __id: t.TargetID, slug: t.Slug, displayName: t.DisplayName };
      spreadMap(row, 'labels', t.Labels);
      return row;
    });
  },

  async resources(params: ResourceParams): Promise<Row[]> {
    // get-resources over the matching Units, then explode each Unit's resource
    // list into one FQL row per Kubernetes resource.
    const responses = await postJson<InvokeResponse[]>(
      '/api/function/invoke',
      { where: params.where, where_data: params.whereData },
      {
        WhereResource: params.whereResource,
        FunctionInvocations: [
          { FunctionName: 'get-resources', Arguments: [{ ParameterName: 'body', Value: 'json' }] },
        ],
      },
    );

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
      for (const raw of list) {
        if (!raw.ResourceBody) continue;
        let doc: Record<string, unknown>;
        try {
          doc = JSON.parse(raw.ResourceBody);
        } catch {
          continue;
        }
        rows.push(resourceRow(resp, raw.ResourceType, doc));
      }
    }
    return rows;
  },
};

function resourceRow(
  resp: InvokeResponse,
  resourceType: string | undefined,
  doc: Record<string, unknown>,
): Row {
  const md = doc['metadata'] as { name?: string; namespace?: string } | undefined;
  const spec = doc['spec'] as { replicas?: number } | undefined;
  return {
    // Generic identity + single-valued K8s shorthands only. Domain fields like
    // image / severity / cveCount are NOT curated columns — query them as the
    // real paths they are (containers.*.image,
    // metadata.annotations['sec-scanner.confighub.com/max-severity']), evaluated
    // client-side against __doc.
    unit: resp.UnitSlug,
    space: resp.SpaceSlug,
    kind: doc['kind'],
    name: md?.name,
    namespace: md?.namespace ?? null,
    replicas: spec?.replicas ?? null,
    resourceType,
    // The raw resource doc, so FQL can evaluate arbitrary YAML data paths
    // client-side. Reserved key (leading __) — excluded from SELECT * columns.
    __doc: doc,
  };
}
