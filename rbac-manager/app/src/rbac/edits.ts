// Structured RBAC edits expressed as stored, parameterized set-yq Invocations.
// The fixed yq templates live in ConfigHub — created by setup.sh in the
// `rbac-edits` Space under the slugs below — and each edit supplies only the
// variable values as parameters. The agent CLI (../rbac-manager-for-agents)
// references the same Invocations by the same slugs. The templates are validated
// offline against the example manifests with `cub function local`.

/** Space holding the shared, parameterized edit Invocations. */
export const EDIT_LIBRARY_SPACE = 'rbac-edits';

/** Invocation slugs (shared with the agent CLI). */
export const INV_ADD_VERB = 'rbac-add-verb';
export const INV_REMOVE_VERB = 'rbac-remove-verb';
export const INV_ADD_SUBJECT = 'rbac-add-subject';
export const INV_REMOVE_SUBJECT = 'rbac-remove-subject';

export interface CompiledEdit {
  /** Slug of the stored parameterized Invocation to execute. */
  slug: string;
  /** Parameter values to supply; keys match the Invocation's declared parameters. */
  params: Record<string, string>;
  /** Human summary, used as the default change description. */
  summary: string;
}

export function compileAddVerb(
  roleKind: string,
  roleName: string,
  ruleIdx: number,
  verb: string,
): CompiledEdit {
  return {
    slug: INV_ADD_VERB,
    params: { roleKind, roleName, ruleIdx: String(ruleIdx), verb },
    summary: `Add verb "${verb}" to ${roleKind} ${roleName} rule ${ruleIdx}`,
  };
}

export function compileRemoveVerb(
  roleKind: string,
  roleName: string,
  ruleIdx: number,
  verb: string,
): CompiledEdit {
  return {
    slug: INV_REMOVE_VERB,
    params: { roleKind, roleName, ruleIdx: String(ruleIdx), verb },
    summary: `Remove verb "${verb}" from ${roleKind} ${roleName} rule ${ruleIdx}`,
  };
}

export function compileAddSubject(
  bindingKind: string,
  bindingName: string,
  subjectKind: string,
  subjectName: string,
  subjectNamespace?: string,
): CompiledEdit {
  // The subject's structural difference is encoded by which field is non-empty:
  // a ServiceAccount carries a namespace, a User/Group carries an apiGroup. The
  // stored template drops whichever is empty.
  const subjectApiGroup =
    subjectKind === 'ServiceAccount' ? '' : 'rbac.authorization.k8s.io';
  return {
    slug: INV_ADD_SUBJECT,
    params: {
      bindingKind,
      bindingName,
      subjectKind,
      subjectName,
      subjectNamespace: subjectNamespace ?? '',
      subjectApiGroup,
    },
    summary: `Add ${subjectKind} "${subjectName}" to ${bindingKind} ${bindingName}`,
  };
}

export function compileRemoveSubject(
  bindingKind: string,
  bindingName: string,
  subjectKind: string,
  subjectName: string,
  subjectNamespace?: string,
): CompiledEdit {
  return {
    slug: INV_REMOVE_SUBJECT,
    params: {
      bindingKind,
      bindingName,
      subjectKind,
      subjectName,
      subjectNamespace: subjectNamespace ?? '',
    },
    summary: `Remove ${subjectKind} "${subjectName}" from ${bindingKind} ${bindingName}`,
  };
}
