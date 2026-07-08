// Reads the org's component/variant catalog from ConfigHub Spaces. A variant
// is a Space carrying `Component` and `Variant` labels; "variant X of
// component Y" is the Space where Component=Y and Variant=X. The Space's full
// label set is carried through so the status provider can read the live-status
// label. Nothing here is written back.

import { useMemo } from 'react';

import { useListSpacesQuery, SpaceRead } from '@confighub/rtk-query';

export const COMPONENT_LABEL = 'Component';
export const VARIANT_LABEL = 'Variant';

/** A concrete (component, variant) pair resolved to its Space. */
export interface VariantRef {
  component: string;
  variant: string;
  spaceId: string;
  spaceSlug: string;
  /** All labels on the Space — includes the live-status label. */
  labels: Record<string, string>;
}

export interface ComponentInfo {
  component: string;
  variants: VariantRef[];
}

export interface Catalog {
  components: ComponentInfo[];
  /** Resolve a choice to its Space, or undefined if it no longer exists. */
  resolve: (component: string, variant: string) => VariantRef | undefined;
  isLoading: boolean;
  error: string | null;
  /** Force an immediate re-read of Spaces (labels). */
  refetch: () => void;
}

function variantOf(space: SpaceRead): VariantRef | null {
  const labels = space.Labels ?? {};
  const component = labels[COMPONENT_LABEL];
  if (!component || !space.SpaceID) return null;
  // A Space tagged with a Component but no explicit Variant is still a usable
  // variant; fall back to its slug so it's selectable and uniquely keyed.
  const variant = labels[VARIANT_LABEL] ?? space.Slug ?? space.SpaceID;
  return { component, variant, spaceId: space.SpaceID, spaceSlug: space.Slug ?? '', labels };
}

/**
 * @param pollingIntervalMs when > 0, re-reads Spaces on that interval so live
 *   status-label changes appear without a manual refresh.
 */
export function useCatalog(pollingIntervalMs = 0): Catalog {
  const { data, isLoading, error, refetch } = useListSpacesQuery(
    { select: 'SpaceID,Slug,Labels' },
    pollingIntervalMs > 0 ? { pollingInterval: pollingIntervalMs } : undefined,
  );

  return useMemo<Catalog>(() => {
    const byComponent = new Map<string, VariantRef[]>();
    for (const extSpace of data ?? []) {
      if (!extSpace.Space) continue;
      const ref = variantOf(extSpace.Space);
      if (!ref) continue;
      const list = byComponent.get(ref.component) ?? [];
      list.push(ref);
      byComponent.set(ref.component, list);
    }
    const components: ComponentInfo[] = [...byComponent.entries()]
      .map(([component, variants]) => ({
        component,
        variants: variants.sort((a, b) => a.variant.localeCompare(b.variant)),
      }))
      .sort((a, b) => a.component.localeCompare(b.component));

    const resolve = (component: string, variant: string): VariantRef | undefined =>
      byComponent.get(component)?.find((v) => v.variant === variant);

    return {
      components,
      resolve,
      isLoading,
      error: error ? 'Failed to load component catalog' : null,
      refetch: () => void refetch(),
    };
  }, [data, isLoading, error, refetch]);
}
