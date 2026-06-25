// The virtual-table catalog: the compile-target map at the heart of FQL.
//
// Each table lists the API source it fetches from and its columns. A column
// declares:
//   - `type`     scalar type, for operator validation (mirrors ConfigHub).
//   - `pushdown` how (and whether) a predicate on it reaches the server:
//       where        → entity filter on the list endpoint (e.g. "Slug")
//       where_data   → JSONB path filter on resource data (e.g. "kind")
//       whereResource→ resource-type metadata filter (ConfigHub.ResourceType)
//       none         → client-side only (computed/derived columns)
//
// Dynamic map prefixes (labels.*, annotations.*) are declared once and match
// any `prefix.key` column; the key becomes a map-key on the server field.

export type ColumnType = 'string' | 'number' | 'boolean';

// where         entity filter on the primary list endpoint
// where_data    JSONB path filter on resource data (resources)
// whereResource resource-type metadata filter (resources)
// whereUnit     on `revisions`: narrows WHICH units we pull revisions from
//               (the revision endpoint is per-unit); revision-field predicates
//               use `where` against that endpoint.
// revision      on `resources`: selects WHICH revision's data to read (a unit's
//               head by default, or a specific RevisionNum / 'head' / 'live').
export type PushdownTarget =
  | 'where'
  | 'where_data'
  | 'whereResource'
  | 'whereUnit'
  | 'revision';

export interface Pushdown {
  target: PushdownTarget;
  /** Server-side field/path the FQL column compiles to, e.g. "Space.Slug" or
   *  "spec.template.spec.containers.*.image". */
  expr: string;
}

export interface ColumnDef {
  type: ColumnType;
  /** Absent → client-side only (never pushed down). */
  pushdown?: Pushdown;
}

/** A dynamic `prefix.key` map column (e.g. labels.env). */
export interface MapPrefixDef {
  /** type of the map values (always string for Labels/Annotations). */
  type: ColumnType;
  pushdown?: { target: PushdownTarget; field: string }; // server map field, e.g. "Labels"
}

export type TableSource = 'units' | 'resources' | 'spaces' | 'revisions';

export interface TableDef {
  source: TableSource;
  columns: Record<string, ColumnDef>;
  /** Dynamic map prefixes, keyed by FQL prefix (e.g. "labels"). */
  mapPrefixes: Record<string, MapPrefixDef>;
  /** When true, an unresolved column is treated as a raw YAML data path and
   *  pushed to `where_data` verbatim (resources only). Backtick-quoted columns
   *  are always treated as raw paths regardless of this flag. */
  rawDataPaths?: boolean;
}

// ─── Tables ───────────────────────────────────────────────────────────────────

const UNITS: TableDef = {
  source: 'units',
  columns: {
    slug: { type: 'string', pushdown: { target: 'where', expr: 'Slug' } },
    displayName: { type: 'string', pushdown: { target: 'where', expr: 'DisplayName' } },
    space: { type: 'string', pushdown: { target: 'where', expr: 'Space.Slug' } },
    toolchain: { type: 'string', pushdown: { target: 'where', expr: 'ToolchainType' } },
    target: { type: 'string', pushdown: { target: 'where', expr: 'Target.Slug' } },
    // cluster = the deploy Target's slug, falling back to the Space slug for
    // unbound ("paper cluster") units (mirrors the rbac model's origin.cluster).
    // The fallback can't be expressed as one sound pushdown clause (Target.Slug=x
    // would miss unbound units in Space x), so cluster is computed/filtered
    // client-side; use `target` or `space` directly when you want server narrowing.
    cluster: { type: 'string' },
    headRev: { type: 'number', pushdown: { target: 'where', expr: 'HeadRevisionNum' } },
    // ConfigHub field names accepted verbatim so revision/drift idioms work:
    // `HeadRevisionNum > LiveRevisionNum` (unapplied changes), `LiveRevisionNum
    // = 0` (never applied), `UpstreamRevisionNum > 0` (clones). All push to where.
    HeadRevisionNum: { type: 'number', pushdown: { target: 'where', expr: 'HeadRevisionNum' } },
    LiveRevisionNum: { type: 'number', pushdown: { target: 'where', expr: 'LiveRevisionNum' } },
    LastAppliedRevisionNum: {
      type: 'number',
      pushdown: { target: 'where', expr: 'LastAppliedRevisionNum' },
    },
    UpstreamRevisionNum: {
      type: 'number',
      pushdown: { target: 'where', expr: 'UpstreamRevisionNum' },
    },
    UpstreamUnitID: { type: 'string', pushdown: { target: 'where', expr: 'UpstreamUnitID' } },
    ProviderType: { type: 'string', pushdown: { target: 'where', expr: 'ProviderType' } },
    // Derived/aggregate-ish columns evaluated client-side from the fetched Unit.
    gates: { type: 'number' }, // count of ApplyGates keys
    warnings: { type: 'number' }, // count of ApplyWarnings keys
  },
  mapPrefixes: {
    labels: { type: 'string', pushdown: { target: 'where', field: 'Labels' } },
    annotations: { type: 'string', pushdown: { target: 'where', field: 'Annotations' } },
    // Policy/gate audit: ApplyGates['<space>/<trigger>/<function>'] = true.
    ApplyGates: { type: 'boolean', pushdown: { target: 'where', field: 'ApplyGates' } },
    ApplyWarnings: { type: 'boolean', pushdown: { target: 'where', field: 'ApplyWarnings' } },
  },
};

const RESOURCES: TableDef = {
  source: 'resources',
  columns: {
    // Identity (from the owning Unit / response metadata) — pushed to `where`.
    unit: { type: 'string', pushdown: { target: 'where', expr: 'Slug' } },
    space: { type: 'string', pushdown: { target: 'where', expr: 'Space.Slug' } },
    // The owning Unit's deploy Target, and cluster = target ?? space. Both are
    // resolved from a unit→cluster enrichment and filtered client-side (cluster's
    // fallback isn't a sound single pushdown; keeping target client-side too
    // avoids emitting a Target.Slug clause the invoke endpoint may not join).
    target: { type: 'string' },
    cluster: { type: 'string' },
    // Generic, single-valued Kubernetes shorthands — unambiguous sugar over a
    // real path, pushed to `where_data`. (NOT domain fictions: every K8s
    // resource has exactly one kind/name/namespace; replicas is a scalar.)
    kind: { type: 'string', pushdown: { target: 'where_data', expr: 'kind' } },
    name: { type: 'string', pushdown: { target: 'where_data', expr: 'metadata.name' } },
    namespace: { type: 'string', pushdown: { target: 'where_data', expr: 'metadata.namespace' } },
    replicas: { type: 'number', pushdown: { target: 'where_data', expr: 'spec.replicas' } },
    // Resource-type metadata — pushed to whereResource.
    resourceType: {
      type: 'string',
      pushdown: { target: 'whereResource', expr: 'ConfigHub.ResourceType' },
    },
    // Time-travel: `WHERE revision = N` reads each unit's resources AS OF that
    // revision (instead of head). Also accepts 'head' / 'live'. Requires unit or
    // space scoping (it fetches a specific revision's data blob per unit). The
    // value is stamped onto every returned row so the residual check passes.
    revision: { type: 'number', pushdown: { target: 'revision', expr: 'revision' } },
    // NOTE: there is deliberately no `image`, `severity`, `cveCount`, etc.
    // `image` was a lossy comma-join of an array; the scanner verdict fields are
    // sec-scanner annotations, not generic resource columns. Query them as the
    // real paths they are:
    //   `spec.template.spec.containers.*.image` LIKE '%:latest'
    //   metadata.annotations['sec-scanner.confighub.com/max-severity'] = 'CRITICAL'
  },
  mapPrefixes: {
    labels: { type: 'string', pushdown: { target: 'where', field: 'Labels' } },
  },
  // Any column that isn't curated/`labels.` is a raw resource-data path
  // (e.g. spec.strategy.type, metadata.annotations['...']). The shorthands above
  // are just sugar over the most common single-valued paths.
  rawDataPaths: true,
};

const SPACES: TableDef = {
  source: 'spaces',
  columns: {
    slug: { type: 'string', pushdown: { target: 'where', expr: 'Slug' } },
    displayName: { type: 'string', pushdown: { target: 'where', expr: 'DisplayName' } },
  },
  mapPrefixes: {
    labels: { type: 'string', pushdown: { target: 'where', field: 'Labels' } },
    annotations: { type: 'string', pushdown: { target: 'where', field: 'Annotations' } },
  },
};

// Revisions are per-Unit (endpoint: /space/{id}/unit/{id}/revision). `unit` and
// `space` narrow WHICH units we pull revisions from (whereUnit); revision-field
// columns (RevisionNum, Source, …) push to the revision endpoint's `where`.
const REVISIONS: TableDef = {
  source: 'revisions',
  columns: {
    // Scoping: which units to read revisions from.
    unit: { type: 'string', pushdown: { target: 'whereUnit', expr: 'Slug' } },
    space: { type: 'string', pushdown: { target: 'whereUnit', expr: 'Space.Slug' } },
    // Revision fields — pushed to the revision endpoint's `where`.
    RevisionNum: { type: 'number', pushdown: { target: 'where', expr: 'RevisionNum' } },
    Source: { type: 'string', pushdown: { target: 'where', expr: 'Source' } },
    Description: { type: 'string', pushdown: { target: 'where', expr: 'Description' } },
    CreatedAt: { type: 'string', pushdown: { target: 'where', expr: 'CreatedAt' } },
    UserID: { type: 'string', pushdown: { target: 'where', expr: 'UserID' } },
  },
  mapPrefixes: {},
};

export const TABLES: Record<string, TableDef> = {
  units: UNITS,
  resources: RESOURCES,
  spaces: SPACES,
  revisions: REVISIONS,
};

/** A resolved column: its type and (optional) compiled pushdown, with the FQL
 *  name for diagnostics. Returned by resolveColumn(). */
export interface ResolvedColumn {
  name: string;
  type: ColumnType;
  pushdown?: Pushdown;
  /** True for a raw YAML data path: evaluate traverses the resource doc (with
   *  `*`/index support) rather than reading a flat row field. */
  raw?: boolean;
}

/**
 * Resolve a dotted column path against a table. Handles fixed columns, dynamic
 * `prefix.key` map columns, and — on tables with rawDataPaths (or for any
 * backtick-quoted column) — arbitrary YAML data paths pushed to `where_data`.
 * Returns null if the column is unknown and the table has no raw-path fallback.
 */
/** Readable name for a raw path: bracket-quote any segment that isn't a plain
 *  path token (so a dotted/slashed annotation key renders unambiguously). */
function rawName(path: string[]): string {
  let out = '';
  for (const seg of path) {
    const simple = /^[A-Za-z0-9_*-]+$/.test(seg);
    if (out === '') out += simple ? seg : `['${seg}']`;
    else out += simple ? `.${seg}` : `['${seg}']`;
  }
  return out;
}

export function resolveColumn(
  table: TableDef,
  path: string[],
  quoted = false,
): ResolvedColumn | null {
  const full = path.join('.');

  // Curated columns and map prefixes apply only to BARE (unquoted) names; a
  // backtick-quoted name is always a verbatim data path.
  if (!quoted) {
    const fixed = table.columns[full];
    if (fixed) return { name: full, type: fixed.type, pushdown: fixed.pushdown };

    if (path.length >= 2) {
      const prefix = path[0];
      const mp = table.mapPrefixes[prefix];
      if (mp) {
        const key = path.slice(1).join('.');
        const pushdown: Pushdown | undefined = mp.pushdown
          ? { target: mp.pushdown.target, expr: `${mp.pushdown.field}.${key}` }
          : undefined;
        return { name: full, type: mp.type, pushdown };
      }
    }
  }

  // Raw data path: evaluate by traversing the resource doc client-side. Push to
  // where_data ONLY when every segment is a clean path token — a key containing
  // dots, slashes, etc. (e.g. an annotation key) can't be expressed in
  // ConfigHub's dotted where_data, so such paths stay client-side only and the
  // residual filter (which always runs) decides them.
  if (quoted || table.rawDataPaths) {
    const pushable = path.every((seg) => /^[A-Za-z0-9_*-]+$/.test(seg));
    return {
      name: rawName(path),
      type: 'string', // permissive; the literal's type drives the comparison
      pushdown: pushable ? { target: 'where_data', expr: path.join('.') } : undefined,
      raw: true,
    };
  }

  return null;
}

/** All column names a table exposes (fixed + map prefixes as `prefix.*`). */
export function columnNames(table: TableDef): string[] {
  return [
    ...Object.keys(table.columns),
    ...Object.keys(table.mapPrefixes).map((p) => `${p}.*`),
  ];
}

// ─── Introspection (for the explorer sidebar + autocomplete) ─────────────────

/** A column as shown in the explorer: name, type, whether it can push down, and
 *  how it's addressed (fixed column, `prefix.key` map, or any raw data path). */
export interface ColumnInfo {
  name: string;
  type: ColumnType;
  kind: 'column' | 'map' | 'raw';
  pushdown: PushdownTarget | null;
}

export interface TableInfo {
  name: string;
  source: TableSource;
  columns: ColumnInfo[];
  /** True if arbitrary YAML data paths are also queryable (resources). */
  rawDataPaths: boolean;
}

/** Structured description of one table for the UI. */
export function describeTable(name: string): TableInfo | null {
  const t = TABLES[name];
  if (!t) return null;
  const columns: ColumnInfo[] = [
    ...Object.entries(t.columns).map(([n, c]) => ({
      name: n,
      type: c.type,
      kind: 'column' as const,
      pushdown: c.pushdown?.target ?? null,
    })),
    ...Object.entries(t.mapPrefixes).map(([p, m]) => ({
      name: `${p}['key']`,
      type: m.type,
      kind: 'map' as const,
      pushdown: m.pushdown?.target ?? null,
    })),
  ];
  return { name, source: t.source, columns, rawDataPaths: t.rawDataPaths === true };
}

/** Every table, described — the explorer's left panel. */
export function describeTables(): TableInfo[] {
  return Object.keys(TABLES).map((n) => describeTable(n)!);
}

/** Flat token list for autocomplete (table names + all column names). */
export function tableNames(): string[] {
  return Object.keys(TABLES);
}
