#!/usr/bin/env bash
#
# A change workflow and its backing Unit, as a demo you drive with the spacebar.
#
#   ./changeworkflow-demo.sh            the demo
#   ./changeworkflow-demo.sh cleanup    delete the space it made
#
# Keys while a command is waiting: space runs it, s skips it, f stops the
# typing effect, ! opens a subshell, q quits. See ../scripts/demo-lib.sh.
#
# Wants: cub, authenticated. No cluster and no worker.
#
# The edit step opens $EDITOR (vi if unset) for you to make the change by hand:
# add Validated to the prod stage's Prerequisites. With DEMO_AUTO=1 an editor
# that makes that same change stands in, so a rehearsal runs end to end.
#
#   DEMO_SPACE=name    the space to work in (default workflow-demo)

source "$(dirname "${BASH_SOURCE[0]}")/../scripts/demo-lib.sh"

SPACE="${DEMO_SPACE:-workflow-demo}"
WORKFLOW=main-line

# backing_unit_slug prints the slug of the workflow's backing Unit. The server names it:
# change-workflow-<slug> on older servers, id<workflow ID> on newer ones. Asking keeps the
# demo right on either.
backing_unit_slug() {
    local id
    id=$(cub changeworkflow get --space "$SPACE" "$WORKFLOW" -o jq=.ChangeWorkflow.BackingUnitID 2>/dev/null | tr -d '"')
    [ -n "$id" ] && [ "$id" != null ] &&
        cub unit get --space "$SPACE" "$id" -o jq=.Unit.Slug 2>/dev/null | tr -d '"'
}

if [ "${1-}" = cleanup ]; then
    run_now "cub space delete --recursive $SPACE"
    exit 0
fi

# Nobody is at the keyboard in a rehearsal, so the editor is a script that adds
# Validated after the last gate of the prod stage and shows what it changed.
if [ -n "$DEMO_AUTO" ] || [ ! -t 0 ]; then
    _editor="$(mktemp)"
    trap 'rm -f "$_editor"' EXIT
    cat > "$_editor" <<'EOF'
#!/bin/bash
cp "$1" "$1.orig"
awk '{ print }
     /Name: prod/ { in_prod = 1 }
     in_prod && /- Healthy/ { sub(/Healthy/, "Validated"); print; in_prod = 0 }' "$1.orig" > "$1"
diff "$1.orig" "$1"
rm -f "$1.orig"
exit 0
EOF
    chmod +x "$_editor"
    export EDITOR="$_editor"
fi

heading "A change workflow"

desc "A change workflow says how a change is promoted: the stages it moves through,"
desc "in order, and the gates that have to pass before it enters each one."
run "cub space create $SPACE"

desc "Three stages, each gated on the stage before it having released the change and"
desc "reporting healthy. --with-backing-units also gives the workflow a Unit holding"
desc "its configuration."
run "cub changeworkflow create --space $SPACE $WORKFLOW --stage dev --stage staging --stage prod --prerequisites Released,Healthy --with-backing-units"
UNIT=$(backing_unit_slug)
UNIT=${UNIT:-backing-unit}

desc "The workflow: its stages, the Spaces each selects, and its gates. It names its"
desc "backing Unit."
run "cub changeworkflow get --space $SPACE $WORKFLOW"

desc "The same entity as the API returns it."
run "cub changeworkflow get --space $SPACE $WORKFLOW -o yq=.ChangeWorkflow"

heading "Its backing Unit"

desc "The backing Unit's data is the workflow as configuration: the fields you set,"
desc "without the IDs and timestamps the server fills in."
run "cub unit data --space $SPACE $UNIT"

desc "It is an ordinary Unit, of toolchain ConfigHub/YAML, and it says what it backs."
run "cub unit get --space $SPACE $UNIT"

desc "Being a Unit, it has Revisions. So far, only its creation."
run "cub revision list --space $SPACE $UNIT"

heading "Change the workflow"

desc "Prod should also refuse a change that has validation errors in staging. Editing"
desc "the workflow edits its backing Unit: add Validated to the prod stage's"
desc "Prerequisites, and the workflow takes the change from the Unit."
run "cub changeworkflow edit --space $SPACE $WORKFLOW"

desc "The workflow has the new gate."
run "cub changeworkflow get --space $SPACE $WORKFLOW"

desc "And the edit is a new Revision of the Unit, so the workflow has a history: who"
desc "changed it, and when."
run "cub revision list --space $SPACE $UNIT -o wide"

demo_end
