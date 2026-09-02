// Fleet-wide resource inventory. Resources are a first-class ConfigHub entity: the server
// extracts them from each Unit's configuration as it changes and indexes them, so a fleet
// view asks the Resource list for exactly the resources it wants — filtered, joined to
// their Unit/Space/Target, across the whole organization — instead of running an
// extraction function over every Unit and unpacking per-Unit responses.
//
// `Resource.Data` is the queryable JSON projection of the resource. It carries the
// authored comments as `$comment$…` keys, which is why the original toolchain-native text
// is a separate on-demand read (`getResourceRaw`) that a fleet load never pays for.

import type { components } from '@confighub/api';

import { confighub } from './client';
import { b64decodeUtf8 } from './encoding';

export type ExtendedResource = components['schemas']['ExtendedResource'];

export interface GetResourcesOptions {
  /** Which resources to return, as a `where` expression over Resources. */
  where: string;
  /** Related entities to expand into the response envelope, e.g. `'UnitID,SpaceID'`. */
  include?: string;
  /** Fields of Resource to return. All of them when omitted. */
  select?: string;
}

/**
 * List resources across the organization. Read-only, and the server already limits the
 * result to what the caller may view.
 */
export async function getResources(
  options: GetResourcesOptions,
): Promise<ExtendedResource[]> {
  const { data, error, response } = await confighub().GET('/resource', {
    params: {
      query: { where: options.where, include: options.include, select: options.select },
    },
  });
  if (error !== undefined || data === undefined) {
    throw new Error(`GET /resource: HTTP ${response.status}`);
  }
  return data;
}

/**
 * The resource's configuration document. The generated client types `Data` as opaque, so
 * this is the one place that widens it to `unknown` for the parsers to narrow again.
 */
export function resourceDoc(resource: ExtendedResource): unknown {
  return resource.Resource?.Data as unknown;
}

/**
 * One resource's configuration in its original toolchain-native text — YAML for
 * Kubernetes, with comments and formatting as authored. Read per resource, on demand:
 * the bodies are bulk, and only a detail view wants the source rather than the JSON
 * projection.
 */
export async function getResourceRaw(
  spaceId: string,
  unitId: string,
  resourceId: string,
): Promise<string> {
  const { data, error, response } = await confighub().GET(
    '/space/{space_id}/unit/{unit_id}/resource/{resource_id}',
    {
      params: {
        path: { space_id: spaceId, unit_id: unitId, resource_id: resourceId },
        query: { raw_data: true },
      },
    },
  );
  if (error !== undefined || data === undefined) {
    throw new Error(`GET resource ${resourceId}: HTTP ${response.status}`);
  }
  return data.RawData === undefined ? '' : b64decodeUtf8(data.RawData);
}

/**
 * Drop the `$comment$…` keys the JSON projection uses to carry authored comments, so a
 * document rendered from `Data` reads as configuration rather than as its own encoding.
 * Only for display — the raw text is the faithful rendering.
 */
export function stripCommentKeys(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(stripCommentKeys);
  if (typeof value !== 'object' || value === null) return value;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(value)) {
    if (k.startsWith('$comment$')) continue;
    out[k] = stripCommentKeys(v);
  }
  return out;
}
