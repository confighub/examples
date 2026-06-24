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
}

/** Params for a revisions fetch. Revisions are per-Unit, so `whereUnit` narrows
 *  which Units to read from, and `where` filters revision fields per unit. */
export interface RevisionParams {
  whereUnit?: string;
  where?: string;
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
}
