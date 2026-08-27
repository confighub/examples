// The Tier-0 dimension registry: entity metadata that is filterable server-side and
// needs no config parsing. Each dimension knows which `include` it costs, so the
// compiler can add joins automatically (see DESIGN.md §2, the JOIN row).

import type { SourceName } from '../model/types';

export type DimKind = 'string' | 'number' | 'time';

export interface Dimension {
  /** Id used in dashboard YAML, e.g. `Space.Labels.Environment`. */
  id: string;
  label: string;
  kind: DimKind;
  source: SourceName;
  /**
   * The `include` field this dimension requires. Present means the value lives on a
   * related entity, and the same prefix is legal in `where`.
   */
  include?: string;
  /** Filter expression fragment for this dimension, when it can be pushed down. */
  whereKey?: string;
}

const dim = (d: Dimension): Dimension => d;

export const UNIT_DIMENSIONS: Dimension[] = [
  dim({ id: 'Unit.Slug', label: 'Unit', kind: 'string', source: 'Unit', whereKey: 'Slug' }),
  dim({
    id: 'Unit.ToolchainType',
    label: 'Toolchain',
    kind: 'string',
    source: 'Unit',
    whereKey: 'ToolchainType',
  }),
  dim({
    id: 'Unit.ProviderType',
    label: 'Provider',
    kind: 'string',
    source: 'Unit',
    whereKey: 'ProviderType',
  }),
  dim({
    id: 'Unit.Deployable',
    label: 'Deployable',
    kind: 'string',
    source: 'Unit',
    // Derived client-side from TargetID. An unbound Unit is normally a base unit held
    // for cloning, or config that is not deployable on its own (an AppConfig unit
    // consumed through a Link) — a category, not missing data.
  }),
  dim({
    id: 'Unit.ReleaseState',
    label: 'Release state',
    kind: 'string',
    source: 'Unit',
  }),
  dim({ id: 'Unit.UpdatedAt', label: 'Last updated', kind: 'time', source: 'Unit', whereKey: 'UpdatedAt' }),
  dim({
    id: 'Unit.HeadRevisionNum',
    label: 'Head revision',
    kind: 'number',
    source: 'Unit',
    whereKey: 'HeadRevisionNum',
  }),
  dim({
    id: 'Unit.LastReleasedRevisionNum',
    label: 'Last released revision',
    kind: 'number',
    source: 'Unit',
    whereKey: 'LastReleasedRevisionNum',
  }),
  dim({
    id: 'Unit.GateCount',
    label: 'Apply gates',
    kind: 'number',
    source: 'Unit',
  }),
  dim({
    id: 'Unit.WarningCount',
    label: 'Apply warnings',
    kind: 'number',
    source: 'Unit',
  }),
  dim({ id: 'Space.Slug', label: 'Space', kind: 'string', source: 'Unit', include: 'SpaceID' }),
  dim({
    id: 'Target.Slug',
    label: 'Cluster (Target)',
    kind: 'string',
    source: 'Unit',
    include: 'TargetID',
    whereKey: 'Target.Slug',
  }),
];

export const SPACE_DIMENSIONS: Dimension[] = [
  dim({ id: 'Space.Slug', label: 'Space', kind: 'string', source: 'Space', whereKey: 'Slug' }),
  dim({
    id: 'Space.TotalUnitCount',
    label: 'Units',
    kind: 'number',
    source: 'Space',
  }),
  dim({ id: 'Space.UnreleasedUnitCount', label: 'Unreleased', kind: 'number', source: 'Space' }),
  dim({
    id: 'Space.CurrentUnitCount',
    label: 'Released and current',
    kind: 'number',
    source: 'Space',
    // Derived client-side: the complement of UnreleasedUnitCount, which is the
    // numerator a meter wants. Not a server field, so it cannot appear in `where`.
  }),
  dim({ id: 'Space.UnlinkedUnitCount', label: 'Unlinked', kind: 'number', source: 'Space' }),
  dim({ id: 'Space.TotalLinkCount', label: 'Links', kind: 'number', source: 'Space' }),
  dim({ id: 'Space.UnapprovedUnitCount', label: 'Unapproved', kind: 'number', source: 'Space' }),
  dim({ id: 'Space.GatedUnitCount', label: 'Gated', kind: 'number', source: 'Space' }),
  dim({ id: 'Space.WarnedUnitCount', label: 'Warned', kind: 'number', source: 'Space' }),
  dim({ id: 'Space.UpgradableUnitCount', label: 'Behind upstream', kind: 'number', source: 'Space' }),
];

export const REVISION_DIMENSIONS: Dimension[] = [
  dim({ id: 'Revision.Source', label: 'Change source', kind: 'string', source: 'Revision', whereKey: 'Source' }),
  dim({ id: 'Revision.CreatedAt', label: 'Created', kind: 'time', source: 'Revision', whereKey: 'CreatedAt' }),
  dim({ id: 'Revision.Released', label: 'Released', kind: 'string', source: 'Revision' }),
  dim({ id: 'Space.Slug', label: 'Space', kind: 'string', source: 'Revision', include: 'SpaceID' }),
  dim({ id: 'Unit.Slug', label: 'Unit', kind: 'string', source: 'Revision', include: 'UnitID' }),
];

export const TARGET_DIMENSIONS: Dimension[] = [
  dim({ id: 'Target.Slug', label: 'Target', kind: 'string', source: 'Target', whereKey: 'Slug' }),
  dim({
    id: 'Target.ProviderType',
    label: 'Provider',
    kind: 'string',
    source: 'Target',
    whereKey: 'ProviderType',
  }),
  dim({
    id: 'Target.KubernetesVersion',
    label: 'Kubernetes version',
    kind: 'string',
    source: 'Target',
    whereKey: 'Facts.Cluster.KubernetesVersion',
  }),
];

/**
 * Resource-grain dimensions. These come from the `get-resources` function rather than
 * an entity field, so none of them is filterable in `where` — a resource panel filters
 * at the *Unit* level (which Units to look inside) and groups at the resource level.
 */
export const RESOURCE_DIMENSIONS: Dimension[] = [
  dim({ id: 'Resource.Type', label: 'Resource type', kind: 'string', source: 'Resource' }),
  dim({ id: 'Resource.Kind', label: 'Kind', kind: 'string', source: 'Resource' }),
  dim({ id: 'Resource.Group', label: 'API group', kind: 'string', source: 'Resource' }),
  dim({ id: 'Resource.Version', label: 'API version', kind: 'string', source: 'Resource' }),
  dim({ id: 'Resource.Family', label: 'Provider family', kind: 'string', source: 'Resource' }),
  dim({ id: 'Resource.Category', label: 'Category', kind: 'string', source: 'Resource' }),
  dim({ id: 'Resource.Name', label: 'Name', kind: 'string', source: 'Resource' }),
  dim({ id: 'Resource.Scope', label: 'Namespace / scope', kind: 'string', source: 'Resource' }),
  dim({ id: 'Resource.Scoped', label: 'Scope kind', kind: 'string', source: 'Resource' }),
  dim({ id: 'Unit.Slug', label: 'Unit', kind: 'string', source: 'Resource' }),
  dim({ id: 'Space.Slug', label: 'Space', kind: 'string', source: 'Resource' }),
  dim({ id: 'Target.Slug', label: 'Cluster (Target)', kind: 'string', source: 'Resource' }),
];

/**
 * Finding-grain dimensions: one row per failing check. `Finding.*` values are derived
 * from the Unit's gate/warning maps client-side, so they are not filterable in `where` —
 * scope a findings panel at the Unit level.
 */
export const FINDING_DIMENSIONS: Dimension[] = [
  dim({ id: 'Finding.Kind', label: 'Gate or warning', kind: 'string', source: 'Finding' }),
  dim({ id: 'Finding.Trigger', label: 'Check', kind: 'string', source: 'Finding' }),
  dim({ id: 'Finding.PolicySpace', label: 'Policy Space', kind: 'string', source: 'Finding' }),
  dim({ id: 'Finding.Function', label: 'Validator', kind: 'string', source: 'Finding' }),
  dim({ id: 'Finding.Key', label: 'Full key', kind: 'string', source: 'Finding' }),
  dim({ id: 'Unit.Slug', label: 'Unit', kind: 'string', source: 'Finding' }),
  dim({ id: 'Unit.ReleaseState', label: 'Release state', kind: 'string', source: 'Finding' }),
  dim({ id: 'Space.Slug', label: 'Space', kind: 'string', source: 'Finding', include: 'SpaceID' }),
  dim({ id: 'Target.Slug', label: 'Cluster (Target)', kind: 'string', source: 'Finding', include: 'TargetID' }),
];

const BY_SOURCE: Record<SourceName, Dimension[]> = {
  Unit: UNIT_DIMENSIONS,
  Space: SPACE_DIMENSIONS,
  Revision: REVISION_DIMENSIONS,
  Target: TARGET_DIMENSIONS,
  Resource: RESOURCE_DIMENSIONS,
  Finding: FINDING_DIMENSIONS,
};

/**
 * Dimensions available on a source. Label- and fact-keyed dimensions are open-ended
 * (`Space.Labels.<anything>`, `Target.Facts.<anything>`), so `lookup` also resolves
 * those by prefix rather than requiring registration.
 */
export function dimensionsFor(source: SourceName): Dimension[] {
  return BY_SOURCE[source] ?? [];
}

const LABEL_PREFIXES: { prefix: string; include?: string; label: (k: string) => string }[] = [
  { prefix: 'Space.Labels.', include: 'SpaceID', label: (k) => k },
  { prefix: 'Unit.Labels.', label: (k) => k },
  { prefix: 'Target.Labels.', include: 'TargetID', label: (k) => k },
  { prefix: 'Target.Facts.', include: 'TargetID', label: (k) => k },
  { prefix: 'Unit.Values.', label: (k) => k },
  { prefix: 'View.', label: (k) => k },
  // Computed by `transform.derive`, so never pushed into `where`.
  { prefix: 'Derived.', label: (k) => k },
];

export function lookupDimension(source: SourceName, id: string): Dimension | undefined {
  const registered = dimensionsFor(source).find((d) => d.id === id);
  if (registered) return registered;

  for (const { prefix, include, label } of LABEL_PREFIXES) {
    if (!id.startsWith(prefix)) continue;
    const key = id.slice(prefix.length);
    // View columns are extracted from the response, never pushed into `where`.
    const computed = prefix === 'View.' || prefix === 'Derived.';
    const whereKey = computed ? undefined : id.replace(/^Unit\./, '');
    return {
      id,
      label: label(key),
      kind: 'string',
      source,
      // On a Space source the Space fields are native, not a join.
      include: source === 'Space' && include === 'SpaceID' ? undefined : include,
      whereKey,
    };
  }
  return undefined;
}

/** The `include` set a set of dimensions requires, deduplicated. */
export function includesFor(source: SourceName, ids: string[]): string[] {
  const includes = new Set<string>();
  for (const id of ids) {
    const d = lookupDimension(source, id);
    if (d?.include) includes.add(d.include);
  }
  return [...includes];
}
