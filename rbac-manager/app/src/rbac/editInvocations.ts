// Resolves the shared, parameterized edit Invocations (created by setup.sh in the
// `rbac-edits` Space) to their IDs, so a structured edit can be executed by
// referencing the stored Invocation via ParameterizedInvocations and supplying
// only its parameter values.

import { useMemo } from 'react';

import { useListAllInvocationsQuery } from '@confighub/rtk-query';
import { EDIT_LIBRARY_SPACE, orderEdits, type CompiledEdit } from './edits';

export interface EditInvocationIds {
  /** slug → InvocationID for the installed edit Invocations. */
  idBySlug: Record<string, string>;
  /** True once the lookup has resolved (regardless of how many were found). */
  loaded: boolean;
}

export function useEditInvocationIds(): EditInvocationIds {
  // Re-read on mount: the edit library is installed out of band (setup.sh), so a session
  // that started before the install would otherwise hold an empty list for its lifetime
  // and report the Invocations missing when they are not.
  const { data } = useListAllInvocationsQuery(
    { where: `Space.Slug = '${EDIT_LIBRARY_SPACE}'` },
    { refetchOnMountOrArgChange: true },
  );
  const idBySlug = useMemo(() => {
    const m: Record<string, string> = {};
    for (const ext of data ?? []) {
      const inv = ext.Invocation;
      if (inv?.Slug && inv?.InvocationID) m[inv.Slug] = inv.InvocationID;
    }
    return m;
  }, [data]);
  return { idBySlug, loaded: data !== undefined };
}

/** Message shown when an edit Invocation this edit needs is not in the library. */
export const EDIT_INVOCATIONS_MISSING =
  `This edit needs an Invocation that is not in Space "${EDIT_LIBRARY_SPACE}". ` +
  'Run setup.sh to install the library, or reload the page if you just installed it.';

/**
 * Builds the FunctionInvocationsRequest fragment that executes a batch of edits by
 * referencing their stored parameterized Invocations. The whole batch is one request, so
 * it is one dry run, one diff, and one revision — several related changes to a role do
 * not have to be committed and reviewed one at a time. StopOnError keeps a batch from
 * landing half-applied.
 *
 * Returns null if any Invocation is not installed (caller should surface
 * EDIT_INVOCATIONS_MISSING).
 */
export function editRequest(
  idBySlug: Record<string, string>,
  edits: CompiledEdit[],
): {
  ParameterizedInvocations: { InvocationID: string; Parameters: Record<string, string> }[];
  StopOnError: boolean;
} | null {
  const ordered = orderEdits(edits);
  const refs: { InvocationID: string; Parameters: Record<string, string> }[] = [];
  for (const edit of ordered) {
    const id = idBySlug[edit.slug];
    if (!id) return null;
    refs.push({ InvocationID: id, Parameters: edit.params });
  }
  if (refs.length === 0) return null;
  return { ParameterizedInvocations: refs, StopOnError: true };
}
