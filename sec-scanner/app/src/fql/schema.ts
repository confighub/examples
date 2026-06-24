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

export type PushdownTarget = 'where' | 'where_data' | 'whereResource';

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

export type TableSource = 'units' | 'resources' | 'spaces' | 'targets';

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
    headRev: { type: 'number', pushdown: { target: 'where', expr: 'HeadRevisionNum' } },
    // Derived/aggregate-ish columns evaluated client-side from the fetched Unit.
    gates: { type: 'number' }, // count of ApplyGates keys
    warnings: { type: 'number' }, // count of ApplyWarnings keys
  },
  mapPrefixes: {
    labels: { type: 'string', pushdown: { target: 'where', field: 'Labels' } },
    annotations: { type: 'string', pushdown: { target: 'where', field: 'Annotations' } },
  },
};

const RESOURCES: TableDef = {
  source: 'resources',
  columns: {
    // Identity (from the owning Unit / response metadata) — pushed to `where`.
    unit: { type: 'string', pushdown: { target: 'where', expr: 'Slug' } },
    space: { type: 'string', pushdown: { target: 'where', expr: 'Space.Slug' } },
    // Resource data fields — pushed to `where_data`.
    kind: { type: 'string', pushdown: { target: 'where_data', expr: 'kind' } },
    name: { type: 'string', pushdown: { target: 'where_data', expr: 'metadata.name' } },
    namespace: { type: 'string', pushdown: { target: 'where_data', expr: 'metadata.namespace' } },
    image: {
      type: 'string',
      pushdown: { target: 'where_data', expr: 'spec.template.spec.containers.*.image' },
    },
    replicas: { type: 'number', pushdown: { target: 'where_data', expr: 'spec.replicas' } },
    // Resource-type metadata — pushed to whereResource.
    resourceType: {
      type: 'string',
      pushdown: { target: 'whereResource', expr: 'ConfigHub.ResourceType' },
    },
    // Scanner verdict (annotations on the Deployment) — client-side only, since
    // they read from the resource body the scanner wrote, not an indexed field.
    severity: { type: 'string' },
    cveCount: { type: 'number' },
    scannedAt: { type: 'string' },
    cvedbVersion: { type: 'string' },
  },
  mapPrefixes: {
    labels: { type: 'string', pushdown: { target: 'where', field: 'Labels' } },
  },
  // Any column that isn't curated/`labels.` is a raw resource-data path
  // (e.g. spec.strategy.type). Curated columns above are sugar over common ones.
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

const TARGETS: TableDef = {
  source: 'targets',
  columns: {
    slug: { type: 'string', pushdown: { target: 'where', expr: 'Slug' } },
    displayName: { type: 'string', pushdown: { target: 'where', expr: 'DisplayName' } },
  },
  mapPrefixes: {
    labels: { type: 'string', pushdown: { target: 'where', field: 'Labels' } },
  },
};

export const TABLES: Record<string, TableDef> = {
  units: UNITS,
  resources: RESOURCES,
  spaces: SPACES,
  targets: TARGETS,
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
