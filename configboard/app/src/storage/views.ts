// Tier-1 dimensions: Views whose columns pull values out of the configuration data.
//
// A View column with a `DataPath` reads any path in any resource type — a Deployment's
// replica count, a Crossplane managed resource's region, an ACK DBInstance's class —
// without configboard knowing the type exists. `GET /unit?view=<slug>` then returns
// those values per Unit as `ViewColumns`, which `rows.ts` exposes as `View.<Name>`
// dimensions.
//
// The Views live in the app's own Space alongside the dashboards, and are created only
// when the user asks.

import {
  useCreateViewMutation,
  useLazyListAllViewsQuery,
} from '@confighub/rtk-query';
import { useCallback } from 'react';

import { STORAGE_SPACE_SLUG } from './dashboards';

/** A column definition in the shape the View API expects. */
interface ColumnSpec {
  Name: string;
  ColumnType: 'MetadataAttribute' | 'DataPath' | 'DataExpression' | 'MetadataExpression';
  DataType: 'string' | 'int' | 'bool';
  ColumnSource: {
    MetadataAttribute?: string;
    DataPath?: { Path: string; WhereResource?: string };
    DataExpression?: string;
    MetadataExpression?: string;
  };
}

const meta = (name: string, attribute: string): ColumnSpec => ({
  Name: name,
  ColumnType: 'MetadataAttribute',
  DataType: 'string',
  ColumnSource: { MetadataAttribute: attribute },
});

const path = (
  name: string,
  p: string,
  whereResource?: string,
  dataType: 'string' | 'int' = 'string',
): ColumnSpec => ({
  Name: name,
  ColumnType: 'DataPath',
  DataType: dataType,
  ColumnSource: { DataPath: { Path: p, ...(whereResource ? { WhereResource: whereResource } : {}) } },
});

export interface ViewSeed {
  slug: string;
  description: string;
  columns: ColumnSpec[];
  /** Dimension ids this View makes available, for the dimension picker. */
  dimensions: string[];
}

/**
 * The Views configboard seeds. Each one is a projection: identity columns so a row can
 * be grouped and drilled into, plus the data paths that answer a specific question.
 */
export const VIEW_SEEDS: ViewSeed[] = [
  {
    slug: 'cb-workload',
    description: 'Container image and replica count for Kubernetes workloads.',
    columns: [
      meta('Unit', 'Unit.Slug'),
      meta('Space', 'Space.Slug'),
      meta('Component', 'Space.Labels.Component'),
      meta('Environment', 'Space.Labels.Environment'),
      // The first container of any workload that has a pod template. `*` matches the
      // workload kinds without naming them.
      path('Image', 'spec.template.spec.containers.0.image'),
      path('Replicas', 'spec.replicas', undefined, 'int'),
    ],
    dimensions: [
      'View.Unit',
      'View.Space',
      'View.Component',
      'View.Environment',
      'View.Image',
      'View.Replicas',
    ],
  },
  {
    slug: 'cb-cloud-resource',
    description:
      'Provider fields common to Crossplane and ACK managed resources: region and size.',
    columns: [
      meta('Unit', 'Unit.Slug'),
      meta('Space', 'Space.Slug'),
      meta('Environment', 'Space.Labels.Environment'),
      // The same concept lives in three different places depending on the provider:
      //
      //   Crossplane  spec.forProvider.region
      //   ACK (spec)  spec.region                     — e.g. RDS DBInstance
      //   ACK (anno)  services.k8s.aws/region annotation, which is what the ACK
      //               controllers actually write for adopted resources
      //
      // A `.` inside a map key is escaped as `~1` in a data path, so the annotation key
      // `services.k8s.aws/region` is addressed as `services~1k8s~1aws/region`.
      // All three are declared; a panel coalesces them into one dimension.
      path('Region', 'spec.forProvider.region'),
      path('AckRegion', 'spec.region'),
      path('AckAnnotationRegion', 'metadata.annotations.services~1k8s~1aws/region'),
      path('InstanceType', 'spec.forProvider.instanceType'),
      path('AckInstanceType', 'spec.instanceType'),
      path('InstanceClass', 'spec.dbInstanceClass'),
    ],
    dimensions: [
      'View.Unit',
      'View.Space',
      'View.Environment',
      'View.Region',
      'View.AckRegion',
      'View.AckAnnotationRegion',
      'View.InstanceType',
      'View.AckInstanceType',
      'View.InstanceClass',
    ],
  },
];

export interface ViewStorage {
  /** Slugs of the seed Views that already exist. */
  existing: () => Promise<string[]>;
  seed: (spaceId: string) => Promise<void>;
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

export function useViewStorage(): ViewStorage {
  const [listViews] = useLazyListAllViewsQuery();
  const [createView] = useCreateViewMutation();

  const existing = useCallback(async (): Promise<string[]> => {
    const slugs = VIEW_SEEDS.map((v) => `'${v.slug}'`).join(', ');
    const views = unwrap(await listViews({ where: `Slug IN (${slugs})` }), 'list views');
    return views.map((v) => v.View?.Slug).filter((s): s is string => Boolean(s));
  }, [listViews]);

  const seed = useCallback(
    async (spaceId: string): Promise<void> => {
      const have = new Set(await existing());
      for (const view of VIEW_SEEDS) {
        if (have.has(view.slug)) continue;
        unwrap(
          await createView({
            spaceId,
            allowExists: 'true',
            view: {
              Slug: view.slug,
              DisplayName: view.slug,
              Of: 'Unit',
              Columns: view.columns,
            },
          }),
          `create view ${view.slug}`,
        );
      }
    },
    [createView, existing],
  );

  return { existing, seed };
}

/** `space/slug` reference a panel query uses. */
export function viewRef(slug: string): string {
  return `${STORAGE_SPACE_SLUG}/${slug}`;
}
