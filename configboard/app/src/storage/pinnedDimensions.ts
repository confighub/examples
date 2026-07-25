// Pinned dimensions: promoting a config value into `Unit.Values`, where it is
// **filterable** in `where` and returned with the plain unit list.
//
// A Tier-1 dimension (a View's DataPath) is projection-only: it can be grouped and
// charted, but never filtered server-side, and every query reparses config to get it. A
// readonly function that returns an AttributeValueList — `get-image`, `get-replicas`,
// `get-env-var` — wired to a **Mutation Trigger** records its result into
// `Unit.Values["<trigger-slug>/<attribute>"]` on every data change. That value is
// indexed metadata: `where=Values."Image/container-image" LIKE '%:v1.2.%'` works.
//
// Two facts shape the flow, and neither is optional to explain:
//
//  1. A Trigger applies to a Space, not to the org. Created in configboard's own Space,
//     it records values for configboard's own units — which are dashboards. Recording
//     values across the fleet means attaching it to the Spaces that hold workloads,
//     i.e. modifying Spaces this app does not own. So the app creates the Trigger and
//     the Filter, and hands over the exact commands. It does not reach into other
//     people's Spaces.
//
//  2. Values populate on the *next* mutation of each Unit. A newly created Trigger
//     changes nothing until something touches the data, so the backfill is an explicit
//     no-op patch.

import {
  useCreateFilterMutation,
  useCreateTriggerMutation,
  useLazyListAllTriggersQuery,
  useLazyListSpacesQuery,
} from '@confighub/rtk-query';
import { useCallback } from 'react';

import { STORAGE_SPACE_SLUG } from './dashboards';

/** A readonly function whose result can be recorded into `Unit.Values`. */
export interface Recordable {
  /** Function name, e.g. `get-image`. */
  fn: string;
  /** Trigger slug, which becomes the first half of the Values key. */
  slug: string;
  /** The attribute the function returns, the second half of the key. */
  attribute: string;
  /** Function arguments, in order. */
  args: string[];
  label: string;
  description: string;
}

/**
 * The value-recording Triggers configboard offers. These use built-in functions, so no
 * Attribute has to be defined — an Attribute is only needed for a *custom* path, which
 * generates its own `get-<slug>` / `set-<slug>` pair.
 */
export const RECORDABLE: Recordable[] = [
  {
    fn: 'get-image',
    slug: 'Image',
    attribute: 'container-image',
    args: ['@0'],
    label: 'Container image (first container)',
    description: 'Records the first container image, so image tags become filterable.',
  },
  {
    fn: 'get-replicas',
    slug: 'Replicas',
    attribute: 'replicas',
    args: [],
    label: 'Replica count',
    description: 'Records spec.replicas, so replica counts become filterable.',
  },
];

/** The `Unit.Values` key a recorded value lands under. */
export function valuesKey(r: Pick<Recordable, 'slug' | 'attribute'>): string {
  return `${r.slug}/${r.attribute}`;
}

/** The dimension id a panel uses to read it. */
export function valuesDimension(r: Pick<Recordable, 'slug' | 'attribute'>): string {
  return `Unit.Values.${valuesKey(r)}`;
}

export interface DiscoveredTrigger {
  slug: string;
  space: string;
  fn: string;
  /** Values key this Trigger produces, best-effort from its slug and function. */
  key: string;
  /** The toolchain the Trigger fires on; a Space's other units are unaffected. */
  toolchain: string;
  /**
   * Spaces that **select** this Trigger, with their total unit counts. A Trigger nothing
   * selects records nothing — and a Trigger selected only by an empty Space records
   * nothing either, which is the failure mode that looks like a broken feature.
   *
   * The count is *all* units in those Spaces, not units of the Trigger's toolchain: per
   * Space that breakdown is not available in one request. So the label says which
   * toolchain the Trigger fires on and lets the reader judge, rather than implying a
   * precision it does not have.
   */
  selectedBy: { space: string; units: number }[];
}

/** Value-recording (non-validating, Mutation) Triggers that already exist in the org. */
/** Filter slug the attach command references; created alongside the Trigger. */
export const RECORDED_VALUES_FILTER = 'cb-recorded-values';

export interface PinnedDimensionStorage {
  discover: () => Promise<DiscoveredTrigger[]>;
  createTrigger: (spaceId: string, r: Recordable) => Promise<void>;
}

function unwrap<T>(result: { data?: T; error?: unknown }, what: string): T {
  if (result.error || result.data === undefined) {
    const detail =
      typeof result.error === 'object' && result.error !== null
        ? JSON.stringify((result.error as { data?: unknown }).data ?? result.error).slice(0, 200)
        : '';
    throw new Error(`${what} failed${detail ? `: ${detail}` : ''}`);
  }
  return result.data;
}

/** Maps a known recording function to the attribute it returns. */
const ATTRIBUTE_BY_FN: Record<string, string> = {
  'get-image': 'container-image',
  'get-image-reference': 'container-image',
  'get-replicas': 'replicas',
};

export function usePinnedDimensions(): PinnedDimensionStorage {
  const [listTriggers] = useLazyListAllTriggersQuery();
  const [listSpaces] = useLazyListSpacesQuery();
  const [createTrigger] = useCreateTriggerMutation();
  const [createFilter] = useCreateFilterMutation();

  const discover = useCallback(async (): Promise<DiscoveredTrigger[]> => {
    // Non-validating Mutation Triggers are the value-recording ones; a validator records
    // gates and warnings instead.
    const [triggers, spaces] = await Promise.all([
      listTriggers({ where: "Event = 'Mutation'" }).then((r) => unwrap(r, 'list triggers')),
      // A Space's resolved Trigger list is what actually decides whether a Trigger fires.
      listSpaces({ include: 'TriggerIDs', summary: true }).then((r) => unwrap(r, 'list spaces')),
    ]);

    // Invert the Space -> Triggers mapping: for each Trigger, who selects it.
    const selectors = new Map<string, { space: string; units: number }[]>();
    for (const e of spaces) {
      const slug = e.Space?.Slug ?? '';
      const units = e.TotalUnitCount ?? 0;
      for (const t of e.Triggers ?? []) {
        const key = `${t.SpaceSlug ?? ''}/${t.Slug ?? ''}`;
        const list = selectors.get(key);
        if (list) list.push({ space: slug, units });
        else selectors.set(key, [{ space: slug, units }]);
      }
    }

    return triggers
      .map((t) => t.Trigger)
      .filter(
        (t): t is NonNullable<typeof t> =>
          Boolean(t?.Slug) && t?.Disabled !== true && t?.Validating !== true,
      )
      .map((t) => {
        const fn = t.FunctionName ?? '';
        const attribute = ATTRIBUTE_BY_FN[fn];
        return {
          slug: t.Slug!,
          space: t.SpaceSlug ?? '',
          fn,
          key: attribute ? `${t.Slug}/${attribute}` : '',
          toolchain: t.ToolchainType ?? '',
          selectedBy: selectors.get(`${t.SpaceSlug ?? ''}/${t.Slug ?? ''}`) ?? [],
        };
      })
      // A Mutation Trigger on a setter (`set-*`, `yq-i`) mutates data rather than
      // recording a value; only functions that return an attribute produce Values.
      .filter((t) => t.key !== '');
  }, [listTriggers, listSpaces]);

  const create = useCallback(
    async (spaceId: string, r: Recordable): Promise<void> => {
      unwrap(
        await createTrigger({
          spaceId,
          allowExists: 'true',
          trigger: {
            Slug: r.slug,
            Event: 'Mutation',
            ToolchainType: 'Kubernetes/YAML',
            FunctionName: r.fn,
            Arguments: r.args.map((value) => ({ Value: value })),
          },
        }),
        `create trigger ${r.slug}`,
      );

      // The Filter the attach command references. Without it, the generated
      // `--trigger-filter` would point at nothing and the Space would select no Triggers.
      unwrap(
        await createFilter({
          spaceId,
          allowExists: 'true',
          filter: {
            Slug: RECORDED_VALUES_FILTER,
            From: 'Trigger',
            Where: `Space.Slug = '${STORAGE_SPACE_SLUG}' AND Event = 'Mutation'`,
          },
        }),
        `create filter ${RECORDED_VALUES_FILTER}`,
      );
    },
    [createTrigger, createFilter],
  );

  return { discover, createTrigger: create };
}

/**
 * The commands an operator runs to make a pinned dimension apply to a workload Space and
 * to backfill existing Units. Generated rather than executed: attaching a Trigger edits
 * a Space configboard does not own.
 */
export function attachCommands(spaceSlug: string, r: Recordable): string[] {
  return [
    `# 1. Make the ${STORAGE_SPACE_SLUG}/${r.slug} trigger apply to ${spaceSlug}.`,
    `#    --where-trigger "-" clears the default "triggers defined in this Space only".`,
    `cub space update --patch ${spaceSlug} \\`,
    `  --trigger-filter ${STORAGE_SPACE_SLUG}/${RECORDED_VALUES_FILTER} --where-trigger "-"`,
    '',
    `# 2. Values are recorded on the next data change, so backfill existing Units with a`,
    `#    no-op patch. This creates a revision per Unit — check the count first.`,
    `cub unit list --space ${spaceSlug} --where "ToolchainType = 'Kubernetes/YAML'"`,
    `cub unit update --patch --space ${spaceSlug} --where "ToolchainType = 'Kubernetes/YAML'" \\`,
    `  --label configboard.recorded=true \\`,
    `  --change-desc "Backfill ${valuesKey(r)} for configboard pinned dimension"`,
    '',
    `# 3. Confirm the values landed.`,
    `cub unit list --space ${spaceSlug} --select "Slug,Values"`,
  ];
}
