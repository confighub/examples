// Server-side resource extraction. `get-resources` runs on the server against every Unit
// matching the request, so a fleet view never downloads whole configurations to pick the
// few resources it cares about. Two filters narrow the work, and they are not the same
// thing: `whereData` decides which Units run the function at all, `whereResource` decides
// which resources within those Units come back.

import type { components } from '@confighub/api';

import { confighub } from './client';
import { b64decodeUtf8 } from './encoding';

export type FunctionInvocationsResponse = components['schemas']['FunctionInvocationsResponse'];

/** One resource as `get-resources` reports it. ResourceBody is a JSON document. */
export interface RawResource {
  ResourceType?: string;
  ResourceName?: string;
  ResourceBody?: string;
}

/** Outputs.ResourceList is base64-encoded JSON (api.Resource[]). */
export function decodeResourceList(encoded: string): RawResource[] {
  try {
    const parsed: unknown = JSON.parse(b64decodeUtf8(encoded));
    return Array.isArray(parsed) ? (parsed as RawResource[]) : [];
  } catch {
    return [];
  }
}

/** The parsed resource documents one invocation response carries, in order. */
export function resourceDocs(response: FunctionInvocationsResponse): { raw: RawResource; doc: unknown }[] {
  const out: { raw: RawResource; doc: unknown }[] = [];
  for (const raw of decodeResourceList(response.Outputs?.['ResourceList'] ?? '')) {
    if (raw.ResourceBody === undefined || raw.ResourceBody === '') continue;
    try {
      out.push({ raw, doc: JSON.parse(raw.ResourceBody) });
    } catch {
      // A resource whose body is not JSON is not one this app can reason about.
    }
  }
  return out;
}

export interface GetResourcesOptions {
  /** Which Units to consider, as a `where` expression over Units. */
  where: string;
  /** Which of those Units to actually run on, as an expression over their configuration. */
  whereData?: string;
  /** Which resources within each Unit to return, as a ConfigHub metadata path expression. */
  whereResource?: string;
}

/**
 * Extract resources across the organization. This reads: the invocation is
 * `get-resources` with no mutation, and nothing is written.
 */
export async function getResources(
  options: GetResourcesOptions,
): Promise<FunctionInvocationsResponse[]> {
  const { data, error, response } = await confighub().POST('/function/invoke', {
    params: { query: { where: options.where, where_data: options.whereData } },
    body: {
      WhereResource: options.whereResource,
      FunctionInvocations: [
        { FunctionName: 'get-resources', Arguments: [{ ParameterName: 'body', Value: 'json' }] },
      ],
    },
  });
  if (error !== undefined || data === undefined) {
    throw new Error(`get-resources: HTTP ${response.status}`);
  }
  return data;
}
