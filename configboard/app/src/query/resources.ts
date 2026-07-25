// Resource-grain rows, via the read-only `get-resources` function.
//
// A Unit holds one or more resources — a rendered Helm chart holds dozens — so
// "how many Deployments do we run?" is not answerable from the Unit list. Invoking
// `get-resources` with `body=none` returns each resource's metadata (type, name,
// scope, category) without its body, which is exactly the projection a resource
// inventory needs.

import { confighubApi } from '@confighub/rtk-query';

import type { Row, RowValue } from '../model/types';
import { realId } from './ids';

/** One entry of the function's `ResourceList` output. */
export interface ResourceInfo {
  ResourceName?: string;
  ResourceNameWithoutScope?: string;
  ResourceType?: string;
  ResourceCategory?: string;
}

/** One element of the invoke response: the function's result for a single Unit. */
export interface InvokeResult {
  UnitID?: string;
  UnitSlug?: string;
  SpaceID?: string;
  SpaceSlug?: string;
  TargetID?: string;
  Success?: boolean;
  Outputs?: Record<string, string>;
}

export interface ResourceQueryArgs {
  where?: string;
  filter?: string;
  resourceType?: string;
}

// `get-resources` is a read, but the endpoint is a POST, so it is not one of the
// generated list queries. Injecting it as a query (rather than using the generated
// mutation) keeps it inside RTK Query's cache: two panels over the same scope share
// one invocation, and the 60s stale time applies as it does everywhere else.
export const resourceApi = confighubApi.injectEndpoints({
  endpoints: (build) => ({
    resourceList: build.query<InvokeResult[], ResourceQueryArgs>({
      query: ({ where, filter, resourceType }) => ({
        url: '/function/invoke',
        method: 'POST',
        params: {
          ...(where ? { where } : {}),
          ...(filter ? { filter } : {}),
          ...(resourceType ? { resource_type: resourceType } : {}),
        },
        body: {
          ToolchainType: 'Kubernetes/YAML',
          FunctionInvocations: [
            {
              FunctionName: 'get-resources',
              // `none` omits each resource's body. `native` would return the YAML —
              // never what a count needs, and orders of magnitude more payload.
              Arguments: [{ ParameterName: 'body', Value: 'none' }],
            },
          ],
        },
      }),
    }),
  }),
});

export const { useResourceListQuery } = resourceApi;

/**
 * Splits a ConfigHub resource type into its parts.
 *
 * `apps/v1/Deployment` -> group `apps`, version `v1`, kind `Deployment`
 * `v1/Namespace`       -> group `` (core), version `v1`, kind `Namespace`
 * `ec2.services.k8s.aws/v1alpha1/VPC` -> group `ec2.services.k8s.aws`
 *
 * The group is the useful fleet dimension: it separates core Kubernetes from ACK
 * (`*.services.k8s.aws`), Crossplane (`*.upbound.io`, `*.crossplane.io`), and every
 * other CRD family, without configboard knowing any of them by name.
 */
export function splitResourceType(type: string): {
  group: string;
  version: string;
  kind: string;
} {
  const parts = type.split('/');
  if (parts.length >= 3) {
    return { group: parts[0], version: parts[1], kind: parts.slice(2).join('/') };
  }
  if (parts.length === 2) {
    return { group: '', version: parts[0], kind: parts[1] };
  }
  return { group: '', version: '', kind: type };
}

/** The scope (namespace, project, account…) encoded ahead of the `/` in ResourceName. */
export function resourceScope(name: string | undefined): string | null {
  if (!name) return null;
  const idx = name.indexOf('/');
  if (idx < 0) return null;
  const scope = name.slice(0, idx);
  return scope.length > 0 ? scope : null;
}

/** A human label for the API family a group belongs to. */
export function providerFamily(group: string): string {
  if (group === '') return 'Kubernetes core';
  if (group.endsWith('.services.k8s.aws')) return 'ACK (AWS)';
  if (group.endsWith('.upbound.io') || group.endsWith('.crossplane.io')) return 'Crossplane';
  if (group.endsWith('.k8s.io') || group === 'apps' || group === 'batch') return 'Kubernetes core';
  return group;
}

const RESOURCE_LIST_OUTPUT = 'ResourceList';

function decodeOutput(encoded: string): ResourceInfo[] {
  try {
    // Outputs are base64-encoded JSON. Decode as UTF-8: resource names may legally
    // contain multibyte characters, and atob alone would corrupt them.
    const bytes = Uint8Array.from(atob(encoded), (c) => c.charCodeAt(0));
    const parsed: unknown = JSON.parse(new TextDecoder().decode(bytes));
    return Array.isArray(parsed) ? (parsed as ResourceInfo[]) : [];
  } catch {
    return [];
  }
}

/**
 * Flattens the per-Unit invoke results into one Row per resource. `targetSlugs` maps
 * TargetID to slug — the invoke response carries the id only, and the Target list is
 * already fetched for the scope bar.
 */
export function resourceRows(
  results: InvokeResult[],
  targetSlugs: Record<string, string>,
  baseUrl: string,
): Row[] {
  const rows: Row[] = [];

  for (const result of results) {
    const encoded = result.Outputs?.[RESOURCE_LIST_OUTPUT];
    if (!encoded) continue;

    const spaceId = realId(result.SpaceID);
    const unitId = realId(result.UnitID);
    // The zero UUID here means "no Target", not "a Target I could not resolve".
    const targetId = realId(result.TargetID);

    const unitHref =
      spaceId && unitId ? `${baseUrl.replace(/\/+$/, '')}/units/${spaceId}/${unitId}` : undefined;

    for (const [i, info] of decodeOutput(encoded).entries()) {
      const type = info.ResourceType ?? '';
      const { group, version, kind } = splitResourceType(type);
      const values: Record<string, RowValue> = {
        'Resource.Type': type || null,
        'Resource.Kind': kind || null,
        'Resource.Group': group === '' ? 'core' : group,
        'Resource.Version': version || null,
        'Resource.Family': providerFamily(group),
        'Resource.Category': info.ResourceCategory ?? null,
        'Resource.Name': info.ResourceNameWithoutScope ?? info.ResourceName ?? null,
        'Resource.Scope': resourceScope(info.ResourceName),
        // Cluster-scoped resources have no namespace, which is a fact about them
        // rather than a missing value — worth its own dimension.
        'Resource.Scoped': resourceScope(info.ResourceName) ? 'Namespaced' : 'Cluster-scoped',
        'Unit.Slug': result.UnitSlug ?? null,
        'Space.Slug': result.SpaceSlug ?? null,
        'Target.Slug': targetId ? (targetSlugs[targetId] ?? '(unknown)') : null,
      };
      rows.push({ id: `${unitId ?? result.UnitSlug}#${i}`, href: unitHref, values });
    }
  }

  return rows;
}
