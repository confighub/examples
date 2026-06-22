// Reads the org's component/variant catalog from ConfigHub Spaces. A variant
// is a Space carrying `Component` and `Variant` labels; "variant X of
// component Y" is the Space where Component=Y and Variant=X. The workflow
// editor offers these as choices; nothing here is written back.

import { useMemo } from 'react';

import { useListSpacesQuery, SpaceRead } from '../sdk/confighubapi.gen';

export const COMPONENT_LABEL = 'Component';
export const VARIANT_LABEL = 'Variant';

/** A concrete (component, variant) pair resolved to its Space. */
export interface VariantRef {
  component: string;
  variant: string;
  spaceId: string;
  spaceSlug: string;
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
}

function variantOf(space: SpaceRead): VariantRef | null {
  const component = space.Labels?.[COMPONENT_LABEL];
  if (!component || !space.SpaceID) return null;
  // A Space tagged with a Component but no explicit Variant is still a usable
  // variant; fall back to its slug so it's selectable and uniquely keyed.
  const variant = space.Labels?.[VARIANT_LABEL] ?? space.Slug ?? space.SpaceID;
  return { component, variant, spaceId: space.SpaceID, spaceSlug: space.Slug ?? '' };
}

export function useCatalog(): Catalog {
  // List every Space once; group client-side. Selecting only what we need
  // keeps the payload small even for orgs with many Spaces.
  const { data, isLoading, error } = useListSpacesQuery({ select: 'SpaceID,Slug,Labels' });

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
    };
  }, [data, isLoading, error]);
}
