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
export const INV_SET_RULE = 'rbac-set-rule';
export const INV_ADD_RULE = 'rbac-add-rule';
export const INV_REMOVE_RULE = 'rbac-remove-rule';

export interface CompiledEdit {
  /** Slug of the stored parameterized Invocation to execute. */
  slug: string;
  /** Parameter values to supply; keys match the Invocation's declared parameters. */
  params: Record<string, string>;
  /** Human summary, used as the default change description. */
  summary: string;
  /**
   * Set when this edit deletes the rule at that position. Deleting shifts every later
   * rule down, so {@link orderEdits} runs deletions last and from the back.
   */
  removesRuleAt?: number;
}

/** One rule's grant, as the editor holds it. */
export interface RuleSpec {
  /** An empty string entry is the core API group, as in Kubernetes itself. */
  apiGroups: string[];
  resources: string[];
  verbs: string[];
}

/**
 * Rule fields as comma-separated lists, never JSON. The server expands argument-value
 * templates with html/template, so a quote character inside a parameter value comes back
 * HTML-escaped and silently corrupts the configuration. The core API group is the empty
 * string, which a comma-separated list cannot spell, so it travels as the sentinel `core`
 * and the stored yq template maps it back.
 */
function ruleParams(rule: RuleSpec): Record<string, string> {
  return {
    apiGroups: rule.apiGroups.map((g) => (g === '' ? 'core' : g)).join(','),
    resources: rule.resources.join(','),
    verbs: rule.verbs.join(','),
  };
}

/** `*` / `core` / a group name, for summaries. */
function describeRule(rule: RuleSpec): string {
  const groups = rule.apiGroups.map((g) => (g === '' ? 'core' : g)).join(',');
  return `${groups || 'core'}/${rule.resources.join(',')} [${rule.verbs.join(',')}]`;
}

export function compileSetRule(
  roleKind: string,
  roleName: string,
  ruleIdx: number,
  rule: RuleSpec,
): CompiledEdit {
  return {
    slug: INV_SET_RULE,
    params: { roleKind, roleName, ruleIdx: String(ruleIdx), ...ruleParams(rule) },
    summary: `Replace ${roleKind} ${roleName} rule ${ruleIdx} with ${describeRule(rule)}`,
  };
}

export function compileAddRule(
  roleKind: string,
  roleName: string,
  rule: RuleSpec,
): CompiledEdit {
  return {
    slug: INV_ADD_RULE,
    params: { roleKind, roleName, ...ruleParams(rule) },
    summary: `Add rule ${describeRule(rule)} to ${roleKind} ${roleName}`,
  };
}

export function compileRemoveRule(
  roleKind: string,
  roleName: string,
  ruleIdx: number,
): CompiledEdit {
  return {
    slug: INV_REMOVE_RULE,
    params: { roleKind, roleName, ruleIdx: String(ruleIdx) },
    summary: `Remove rule ${ruleIdx} from ${roleKind} ${roleName}`,
    removesRuleAt: ruleIdx,
  };
}

/**
 * Execution order for a batch of pending edits. Every edit's rule index was read against
 * the document as it stands now, and only deletion invalidates those indices — appending
 * does not, and replacing in place does not. So everything else runs first in the order it
 * was added, and deletions run last from the highest index down, leaving every index still
 * meaning what the user selected.
 */
export function orderEdits(edits: CompiledEdit[]): CompiledEdit[] {
  const keeps = edits.filter((e) => e.removesRuleAt === undefined);
  const removes = edits
    .filter((e) => e.removesRuleAt !== undefined)
    .sort((a, b) => (b.removesRuleAt ?? 0) - (a.removesRuleAt ?? 0));
  return [...keeps, ...removes];
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
