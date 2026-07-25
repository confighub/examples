// "Save as Filter": promoting a panel's query into a real ConfigHub Filter.
//
// A panel's `where` lives inside a dashboard document, which only configboard reads. The
// same expression saved as a Filter becomes an org entity every tool can use —
// `cub unit list --filter configboard/<slug>`, a bulk patch, a Trigger's scope. This is
// the step that turns a chart someone built into fleet plumbing.

import { useCreateFilterMutation } from '@confighub/rtk-query';
import { useCallback } from 'react';

import type { SourceName } from '../model/types';

/** Filter.From values, which are entity type names rather than our source names. */
const FROM_BY_SOURCE: Partial<Record<SourceName, string>> = {
  Unit: 'Unit',
  Space: 'Space',
  Revision: 'Revision',
  Target: 'Target',
};

export interface SaveFilterArgs {
  spaceId: string;
  slug: string;
  source: SourceName;
  where: string;
  resourceType?: string;
}

export interface FilterStorage {
  save: (args: SaveFilterArgs) => Promise<void>;
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

/** A resource-grain panel has no Filter equivalent: `From` has no Resource entity. */
export function canSaveAsFilter(source: SourceName, where: string | undefined): boolean {
  return Boolean(FROM_BY_SOURCE[source]) && Boolean(where && where.trim().length > 0);
}

export function useFilterStorage(): FilterStorage {
  const [createFilter] = useCreateFilterMutation();

  const save = useCallback(
    async ({ spaceId, slug, source, where, resourceType }: SaveFilterArgs): Promise<void> => {
      const from = FROM_BY_SOURCE[source];
      if (!from) throw new Error(`${source} panels cannot be saved as a Filter.`);

      unwrap(
        await createFilter({
          spaceId,
          allowExists: 'true',
          filter: {
            Slug: slug,
            From: from,
            Where: where,
            ...(resourceType ? { ResourceType: resourceType } : {}),
          },
        }),
        `create filter ${slug}`,
      );
    },
    [createFilter],
  );

  return { save };
}
