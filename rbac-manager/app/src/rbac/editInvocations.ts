// Resolves the shared, parameterized edit Invocations (created by setup.sh in the
// `rbac-edits` Space) to their IDs, so a structured edit can be executed by
// referencing the stored Invocation via ParameterizedInvocations and supplying
// only its parameter values.

import { useMemo } from 'react';

import { useListAllInvocationsQuery } from '../sdk/confighubapi.gen';
import { EDIT_LIBRARY_SPACE, type CompiledEdit } from './edits';

export interface EditInvocationIds {
  /** slug → InvocationID for the installed edit Invocations. */
  idBySlug: Record<string, string>;
  /** True once the lookup has resolved (regardless of how many were found). */
  loaded: boolean;
}

export function useEditInvocationIds(): EditInvocationIds {
  const { data } = useListAllInvocationsQuery({
    where: `Space.Slug = '${EDIT_LIBRARY_SPACE}'`,
  });
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

/** Message shown when the edit Invocations have not been installed. */
export const EDIT_INVOCATIONS_MISSING =
  `Edit Invocations not found in Space "${EDIT_LIBRARY_SPACE}". Run setup.sh to install them.`;

/**
 * Builds the FunctionInvocationsRequest fragment that executes an edit by
 * referencing its stored parameterized Invocation. Returns null if the
 * Invocation is not installed (caller should surface EDIT_INVOCATIONS_MISSING).
 */
export function editRequest(
  idBySlug: Record<string, string>,
  edit: CompiledEdit,
): { ParameterizedInvocations: { InvocationID: string; Parameters: Record<string, string> }[] } | null {
  const id = idBySlug[edit.slug];
  if (!id) return null;
  return { ParameterizedInvocations: [{ InvocationID: id, Parameters: edit.params }] };
}
