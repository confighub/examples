// The transport seam: the abstract ConfigHub operations FQL needs, decoupled
// from how they're actually issued (RTK in the app, a mock in tests). The
// executor calls these with the compiled clause strings from a FetchSpec and
// gets back already-flattened FQL rows (so result-shape knowledge lives in the
// adapter, not the engine).

import type { Row } from './evaluate';

/** Params for an entity-list fetch (units / spaces). */
export interface ListParams {
  where?: string;
}

/** Params for a resource fetch (get-resources over Units). */
export interface ResourceParams {
  where?: string;
  whereData?: string;
  whereResource?: string;
  /** Read resources AS OF this revision instead of head: a RevisionNum, or the
   *  symbolic 'head' / 'live'. */
  revision?: string;
}

/** Params for a revisions fetch. Revisions are per-Unit, so `whereUnit` narrows
 *  which Units to read from, and `where` filters revision fields per unit. */
export interface RevisionParams {
  whereUnit?: string;
  where?: string;
}

/** The RBAC access question for a `grants` fetch, extracted from WHERE. Any field
 *  omitted is a wildcard; with none set, every grant is returned. The transport's
 *  materializer applies these with RBAC match semantics (wildcards, aggregation,
 *  builtins) — they are NOT literal row-field filters. */
export interface AccessQuerySpec {
  verb?: string;
  resource?: string;
  apiGroup?: string;
  namespace?: string;
  name?: string;
}

/** Params for a `grants` fetch: `where` narrows which Units' RBAC to read;
 *  `accessQuery` is the effective-access question applied by the materializer. */
export interface GrantsParams {
  where?: string;
  accessQuery?: AccessQuerySpec;
}

/**
 * A Transport knows how to fetch each virtual table's rows from ConfigHub and
 * return them as flat FQL rows (column name → value). Each method may be called
 * multiple times (once per DNF AND-group); the executor de-dupes across calls.
 */
export interface Transport {
  units(params: ListParams): Promise<Row[]>;
  resources(params: ResourceParams): Promise<Row[]>;
  spaces(params: ListParams): Promise<Row[]>;
  revisions(params: RevisionParams): Promise<Row[]>;
  /** Effective-access rows, materialized from RBAC resources (who can what). */
  grants(params: GrantsParams): Promise<Row[]>;
  /** Role/ClusterRole inventory rows, materialized from RBAC resources. */
  roles(params: ListParams): Promise<Row[]>;
  /** RoleBinding/ClusterRoleBinding inventory rows, materialized from RBAC. */
  bindings(params: ListParams): Promise<Row[]>;
  /** RBAC hygiene findings (analyzeFleet), materialized from RBAC resources. */
  rbacFindings(params: ListParams): Promise<Row[]>;
}
