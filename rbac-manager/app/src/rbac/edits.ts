// Structured RBAC edits compiled to server-side yq-i expressions. Shared by
// the single-unit Quick Edit panel and fleet-wide bulk operations; the
// expressions are validated offline against the example manifests with
// `cub function local`.

export interface CompiledEdit {
  /** yq expression for the yq-i function. */
  expr: string;
  /** Human summary, used as the default change description. */
  summary: string;
}

export function compileAddVerb(
  roleKind: string,
  roleName: string,
  ruleIdx: number,
  verb: string,
): CompiledEdit {
  const sel = `select(.kind == "${roleKind}" and .metadata.name == "${roleName}").rules[${ruleIdx}].verbs`;
  return {
    // `unique` makes the edit idempotent: re-applying never duplicates.
    expr: `(${sel}) |= ((. + ["${verb}"]) | unique)`,
    summary: `Add verb "${verb}" to ${roleKind} ${roleName} rule ${ruleIdx}`,
  };
}

export function compileRemoveVerb(
  roleKind: string,
  roleName: string,
  ruleIdx: number,
  verb: string,
): CompiledEdit {
  const sel = `select(.kind == "${roleKind}" and .metadata.name == "${roleName}").rules[${ruleIdx}].verbs`;
  return {
    expr: `(${sel}) |= (. - ["${verb}"])`,
    summary: `Remove verb "${verb}" from ${roleKind} ${roleName} rule ${ruleIdx}`,
  };
}

export function compileRemoveSubject(
  bindingKind: string,
  bindingName: string,
  subjectKind: string,
  subjectName: string,
  subjectNamespace?: string,
): CompiledEdit {
  const match =
    subjectKind === 'ServiceAccount'
      ? `(.kind == "ServiceAccount") and (.name == "${subjectName}") and (.namespace == "${subjectNamespace ?? ''}")`
      : `(.kind == "${subjectKind}") and (.name == "${subjectName}")`;
  return {
    expr: `select(.kind == "${bindingKind}" and .metadata.name == "${bindingName}").subjects |= map(select((${match}) | not))`,
    summary: `Remove ${subjectKind} "${subjectName}" from ${bindingKind} ${bindingName}`,
  };
}

export function compileAddSubject(
  bindingKind: string,
  bindingName: string,
  subjectKind: string,
  subjectName: string,
  subjectNamespace?: string,
): CompiledEdit {
  const subject =
    subjectKind === 'ServiceAccount'
      ? `{"kind": "ServiceAccount", "name": "${subjectName}", "namespace": "${subjectNamespace ?? ''}"}`
      : `{"kind": "${subjectKind}", "name": "${subjectName}", "apiGroup": "rbac.authorization.k8s.io"}`;
  return {
    expr: `select(.kind == "${bindingKind}" and .metadata.name == "${bindingName}").subjects += [${subject}]`,
    summary: `Add ${subjectKind} "${subjectName}" to ${bindingKind} ${bindingName}`,
  };
}
