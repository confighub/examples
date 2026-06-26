#!/usr/bin/env bash
# Shared installer for the parameterized set-yq edit Invocations.
#
# The structured edits (add/remove a verb or subject) live in ConfigHub as
# parameterized set-yq Invocations. Both the web app (UnitPage/FleetPage) and the
# agent CLI (../rbac-manager-for-agents) reference them by slug and supply only
# the variable values as parameters, instead of compiling yq client-side.
#
# Source this file and call install_edit_invocations. Requires $cub to be set to
# the cub binary. Idempotent (get-or-create).

EDIT_LIBRARY_SPACE="rbac-edits"

# _create_edit_invocation SLUG EXPR PARAM[:int]...
# Declares each parameter and stores a set-yq call whose `param` argument values
# are templated from those parameters ({{ .Params.<name> }}). The yq template
# references the declared parameters as $params.<name>; ints use `| tonumber`.
_create_edit_invocation() {
  local slug="$1" expr="$2"; shift 2
  if $cub invocation get "$slug" --space "$EDIT_LIBRARY_SPACE" --quiet &>/dev/null; then
    return 1
  fi
  local decl=() params=() p name
  for p in "$@"; do
    name="${p%%:*}"   # strip an optional ":int" suffix to get the parameter name
    decl+=(--parameter "$p")
    params+=("--param=template:${name}={{ .Params.${name} }}")
  done
  $cub invocation create --space "$EDIT_LIBRARY_SPACE" "$slug" Kubernetes/YAML \
    "${decl[@]}" \
    -- set-yq --yq-expression="$expr" "${params[@]}" >/dev/null
  echo "  created invocation ${EDIT_LIBRARY_SPACE}/${slug}"
  return 0
}

install_edit_invocations() {
  local created=0

  if $cub space get "$EDIT_LIBRARY_SPACE" --quiet &>/dev/null; then
    echo "Edit library Space ${EDIT_LIBRARY_SPACE} exists."
  else
    $cub space create "$EDIT_LIBRARY_SPACE" --label app=rbac-manager --label role=edits >/dev/null
    echo "Created edit library Space ${EDIT_LIBRARY_SPACE}."
    ((created += 1))
  fi

  # Single-quoted so the shell leaves $params.* literal for set-yq.
  local add_verb='(select(.kind == $params.roleKind and .metadata.name == $params.roleName).rules[$params.ruleIdx | tonumber].verbs) |= ((. + [$params.verb]) | unique)'
  local remove_verb='(select(.kind == $params.roleKind and .metadata.name == $params.roleName).rules[$params.ruleIdx | tonumber].verbs) |= (. - [$params.verb])'
  local add_subject='select(.kind == $params.bindingKind and .metadata.name == $params.bindingName).subjects += [ {"kind": $params.subjectKind, "name": $params.subjectName, "namespace": $params.subjectNamespace, "apiGroup": $params.subjectApiGroup} | with_entries(select(.value != "")) ]'
  local remove_subject='select(.kind == $params.bindingKind and .metadata.name == $params.bindingName).subjects |= map(select((.kind == $params.subjectKind and .name == $params.subjectName and (.namespace // "") == $params.subjectNamespace) | not))'

  _create_edit_invocation rbac-add-verb "$add_verb" roleKind roleName ruleIdx:int verb && ((created += 1))
  _create_edit_invocation rbac-remove-verb "$remove_verb" roleKind roleName ruleIdx:int verb && ((created += 1))
  _create_edit_invocation rbac-add-subject "$add_subject" bindingKind bindingName subjectKind subjectName subjectNamespace subjectApiGroup && ((created += 1))
  _create_edit_invocation rbac-remove-subject "$remove_subject" bindingKind bindingName subjectKind subjectName subjectNamespace && ((created += 1))

  echo "Edit Invocations ready (${created} created)."
}
