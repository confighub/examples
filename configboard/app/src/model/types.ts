// A dashboard is data. These types describe the document that a dashboard YAML
// parses into; nothing here knows about React or the ConfigHub API.

/**
 * Where a panel's rows come from.
 *
 * The first four are list endpoints, one row per entity. `Resource` is different: it
 * invokes the read-only `get-resources` function across the selected Units and emits
 * one row per *resource inside* them. A Unit can hold many resources — a rendered Helm
 * chart holds dozens — so resource counts and Unit counts are different questions.
 */
export type SourceName = 'Unit' | 'Space' | 'Revision' | 'Target' | 'Resource';

/** A value in a result row: a dimension string, a measure number, or missing. */
export type RowValue = string | number | null;

/**
 * One normalized result row. `values` is keyed by dimension/measure id — the same
 * ids that appear in a panel's `groupBy` and `aggregate.field`.
 */
export interface Row {
  id: string;
  /** Deep link into the ConfigHub UI, when the row denotes a linkable entity. */
  href?: string;
  values: Record<string, RowValue>;
}

export type AggregateFn =
  | 'count'
  | 'sum'
  | 'avg'
  | 'min'
  | 'max'
  | 'p50'
  | 'p95'
  | 'distinctCount'
  /**
   * The dominant string value in the group, with the *number of distinct values* as the
   * measure. This is what a version-skew matrix needs: the cell shows the image tag, and
   * a value above 1 says the group disagrees with itself.
   */
  | 'value';

export type BinUnit = 'day' | 'week' | 'month';

export interface Transform {
  /**
   * Dimensions computed from other dimensions before grouping. `coalesce` takes the
   * first non-empty value in order, which is how one panel covers the same concept
   * across providers that spell it differently — a region lives at
   * `spec.forProvider.region` in Crossplane and in an annotation in ACK.
   *
   * Derived names must start with `Derived.`.
   */
  derive?: { name: string; coalesce: string[] }[];
  /** Bucket a timestamp dimension before grouping. */
  bin?: { field: string; unit: BinUnit };
  /**
   * Bucket a numeric dimension into fixed-width buckets before grouping — a histogram.
   * `size` defaults to a round number derived from the observed range.
   */
  numericBin?: { field: string; size?: number };
  /**
   * Grouping keys. One key produces a single series; two produce a series per
   * distinct value of the second key (stacked bars, multi-line).
   */
  groupBy?: string | string[];
  aggregate?: { fn: AggregateFn; field?: string };
  /** Keep the top N categories by value. */
  topN?: number;
  /**
   * What happens to the categories past `topN`.
   *
   * `other` (default) folds them into a single "Other" bucket — right when the point
   * is part-to-whole, or when the alternative is minting a 9th categorical hue.
   *
   * `drop` omits them and reports the count instead. Right for a ranked "top 10"
   * where the residue would otherwise be the largest mark on the chart and crush the
   * scale of the ones you asked to see.
   */
  tail?: 'other' | 'drop';
  sort?: 'value-desc' | 'value-asc' | 'key-asc' | 'key-desc';
  /** Drop rows whose group key is null rather than bucketing them as "(none)". */
  dropEmpty?: boolean;
}

export type ChartForm =
  | 'statTile'
  | 'meter'
  | 'bar'
  | 'stackedBar'
  | 'line'
  | 'donut'
  | 'heatmap'
  | 'histogram'
  | 'table';

export type ColorRole = 'sequential' | 'categorical' | 'status' | 'emphasis';

export interface ChartSpec {
  form: ChartForm;
  orientation?: 'horizontal' | 'vertical';
  color?: ColorRole;
  /** statTile / meter: what the value is measured against. */
  totalField?: string;
  /** meter: inverted meters read "lower is better". */
  invert?: boolean;
  /** emphasis: the category rendered in the accent hue; the rest go gray. */
  emphasize?: string;
  unit?: string;
}

export interface PanelQuery {
  source: SourceName;
  /** Server-side filter. May contain `${var}` references to dashboard variables. */
  where?: string;
  /** Saved View (`space/slug`) whose columns become extra dimensions. */
  view?: string;
  /**
   * Narrow to Units containing a resource of this type, e.g. `apps/v1/Deployment`.
   * Server-side, and the right way to scope a data-path projection: a View column that
   * reads `spec.replicas` is meaningless on a Service.
   */
  resourceType?: string;
  /** Saved Filter (`space/slug`), ANDed with `where`. */
  filter?: string;
  /** Rows the panel deliberately excludes, reported in the panel footer. */
  excludes?: { field: string; isNull?: boolean; label: string };
}

export interface Panel {
  id: string;
  title: string;
  description?: string;
  /** 12-column grid width. */
  span?: number;
  query: PanelQuery;
  transform?: Transform;
  chart: ChartSpec;
}

export interface VariableSource {
  /** Distinct values of a Space label. */
  spaceLabel?: string;
  /** Distinct Target slugs (the cluster dimension). */
  target?: string;
}

export interface Variable {
  name: string;
  label: string;
  type?: 'select' | 'timeRange';
  from?: VariableSource;
  default?: string;
  /** Offer an "All" choice that omits the clause entirely. */
  allValue?: boolean;
}

export interface Dashboard {
  apiVersion: string;
  kind: 'Dashboard';
  slug: string;
  title: string;
  description?: string;
  variables?: Variable[];
  panels: Panel[];
}

// ---------------------------------------------------------------------------
// Aggregation output
// ---------------------------------------------------------------------------

export interface Point {
  key: string;
  value: number;
  /**
   * The string a cell displays, when the measure is a value rather than a count
   * (`aggregate: value`). Heatmaps render this; other forms ignore it.
   */
  label?: string;
  /** Source rows behind this point, for drill-down. */
  rows: Row[];
}

export interface Series {
  name: string;
  points: Point[];
}

export interface Frame {
  /** Ordered category keys shared by every series. */
  categories: string[];
  series: Series[];
  /** Sum of every point, for share-of-total forms. */
  total: number;
  /** Rows dropped by `dropEmpty` or a panel `excludes` clause. */
  excluded: number;
  /** Categories omitted by `tail: drop`, so the panel can say so. */
  omittedCategories: number;
}
