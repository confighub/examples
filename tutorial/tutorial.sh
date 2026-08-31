#!/usr/bin/env bash
#
# The ConfigHub get-started tutorial, as a demo you drive with the spacebar.
# https://docs.confighub.com/get-started/tutorial/
#
#   ./tutorial.sh                 every section, in order
#   ./tutorial.sh change flow     just those sections
#   ./tutorial.sh --list          what the sections are
#
# Keys while a command is waiting: space runs it, s skips it, f stops the
# typing effect, ! opens a subshell, q quits. See demo-lib.sh.
#
# Wants: cub (authenticated), docker, kind, kubectl. The cluster sections
# create real kind clusters and take a few minutes each; `cleanup` removes
# everything this demo makes.
#
# The slow setup here is the clusters, so DEMO_SETUP=INIT builds both of them
# before the narration starts rather than in the middle of it, and the two
# cluster sections explain what was done instead of doing it. For a live demo,
# where minutes of kind output is dead air. Unset, or anything else, keeps the
# tutorial's own order.

source "$(dirname "${BASH_SOURCE[0]}")/demo-lib.sh"

SECTIONS="cluster install release change prod flow undo cleanup"

usage() {
    cat <<EOF
usage: tutorial.sh [section ...]

sections, in tutorial order:
  cluster   bring up the dev kind cluster, wired to ConfigHub
  install   install the CubbyChat component from a config bundle
  release   create the dev deployment and publish a release
  change    change the base, promote to dev, release
  prod      a second cluster and a production deployment
  flow      flow a change base -> dev -> prod, with protection and conflicts
  undo      an urgent prod change, undone by restoring a released revision
  cleanup   tear down both clusters and the spaces

with no arguments, runs everything except cleanup.
EOF
}

section_cluster() {
    heading "Set up a cluster"

    if [ "$DEMO_SETUP" = "INIT" ]; then
        desc "ConfigHub manages configuration for live infrastructure, so we start"
        desc "with some. The dev cluster is already up: cub cluster up --name dev,"
        desc "which we ran before starting, built a local kind cluster, installed"
        desc "Argo CD into it, and created the ConfigHub spaces and target that"
        desc "address it."
    else
        desc "ConfigHub manages configuration for live infrastructure, so we start"
        desc "with some. One command builds a local kind cluster, installs Argo CD"
        desc "into it, and creates the ConfigHub spaces and target that address it."
        run "cub cluster up --name dev"
    fi

    desc "Argo CD is running in the cluster. Nothing has been deployed to it yet."
    run "source ~/.confighub/clusters/dev.env"
    run "kubectl get pods -n argocd"

    desc "On the ConfigHub side: a space holding the OCI worker and the target,"
    desc "and a companion space holding the root app-of-apps Argo Application."
    run "cub space get dev"
    run "cub unit list --space dev-argo-apps"

    desc "ConfigHub never pushes into the cluster. The cluster pulls from it."
}

section_install() {
    heading "Install a component"

    desc "A component is a piece of software plus the configuration that runs it."
    desc "It is never deployed itself: it is a base, plus one deployment per place"
    desc "it runs. This pulls a bundle of plain Kubernetes YAML into a base."
    run "cub variant upload --component cubbychat --variant base --granularity per-file oci://ghcr.io/confighub/configs/cubbychat"

    desc "Nothing is running. ConfigHub stores configuration; going live is a"
    desc "separate, deliberate step."
    run "cub component list"

    desc "Each variant of a component is a space, grouped by labels."
    run "cub space list --where \"Labels.Component = 'cubbychat'\""

    desc "The base records where it came from -- the bundle and its digest."
    run "cub space get cubbychat-base -o jq='.Space.Annotations'"

    desc "One unit per file in the bundle."
    run "cub unit list --space cubbychat-base"

    desc "Ordinary Kubernetes YAML, with literal values. Note the namespace:"
    desc "confighubplaceholder -- the base does not know where it will run, and"
    desc "a placeholder is a value that must be replaced before it is deployable."
    run "cub unit data --space cubbychat-base backend"
}

section_release() {
    heading "Go live"

    desc "A deployment is a clone of the base, adapted to a concrete place to run."
    desc "This clones every unit, links each copy to the base unit it came from,"
    desc "attaches the dev target, and fills in the namespace placeholder."
    run "cub variant create dev cubbychat-base --target dev/target --namespace cubbychat"

    desc "Each clone remembers its upstream unit and the revision it was taken"
    desc "from. That memory is what lets base changes flow down later."
    run "cub unit get --space cubbychat-dev backend"

    desc "Configuration never goes live implicitly. Publishing a release bundles"
    desc "this deployment's config into ConfigHub's OCI endpoint, where Argo pulls it."
    run "cub release publish cubbychat-dev"

    desc "Argo notices the release on its next sync and applies it."
    run "kubectl wait --for=condition=Ready pods --all -n cubbychat --timeout=180s"
    run "kubectl get pods -n cubbychat"

    desc "Nothing was pushed at the cluster: ConfigHub published, the cluster pulled."
    run "cub release list --space cubbychat-dev"
}

section_change() {
    heading "Make a change"

    desc "Configuration is data, so you change it with functions that operate on"
    desc "fields rather than by editing YAML. Change the base, not the deployment."
    run "cub function set --space cubbychat-base --unit backend set-replicas 2"

    desc "The base has a new revision and ConfigHub knows dev is behind it."
    run "cub unit list --space cubbychat-dev"

    desc "Promotion is per-deployment and deliberate. Preview it field by field,"
    desc "then promote."
    run "cub variant promote cubbychat-dev --dry-run -o mutations"
    run "cub variant promote cubbychat-dev"

    desc "The config changed; the cluster is still running the previous release."
    run "cub release publish cubbychat-dev"
    run "kubectl get pods -n cubbychat"

    desc "Change the base, promote to the deployment, release to the cluster."
    desc "That loop is the daily rhythm."
}

section_prod() {
    heading "Add a production deployment"

    desc "The point of the base/deployment split is that the second one is cheap."
    if [ "$DEMO_SETUP" = "INIT" ]; then
        desc "The prod cluster went up front with dev, the same one command:"
        desc "cub cluster up --name prod. Nothing is deployed to it yet."
    else
        run "cub cluster up --name prod"
    fi

    desc "Same clone as dev, with a production label and a delete gate on the units."
    run "cub variant create prod cubbychat-base --target prod/target --namespace cubbychat --environment Prod --unit-delete-gate critical"

    desc "Nothing left undecided: with the namespace filled in, no placeholders remain."
    run "cub function get --space cubbychat-prod get-placeholders --show output"

    run "cub release publish cubbychat-prod"
    run "source ~/.confighub/clusters/prod.env"
    run "kubectl wait --for=condition=Ready pods --all -n cubbychat --timeout=180s"
    run "kubectl get pods -n cubbychat"

    desc "One component, three variants: a base and two deployments."
    run "cub space list --where \"Labels.Component = 'cubbychat'\""

    desc "Prod picked up the replica change at clone time -- a fresh clone starts"
    desc "from the base as it stands. From here the two deployments move apart."
}

section_flow() {
    heading "Flow a change"

    desc "Real deployments accumulate local differences. Someone renames the chat"
    desc "title in dev, and wants it to stay that way whatever the base says later."
    desc "--protect is what says so: those paths are dev's, and a merge leaves them alone."
    run "cub function set --space cubbychat-dev --unit backend --protect set-env-var backend CHAT_TITLE \"Cubby Chat (dev)\""

    desc "Without --protect the edit still lands, but claims nothing: a later"
    desc "upstream change to the same field would flow in over it."
    run "cub unit get --space cubbychat-dev backend -o mutations"

    desc "Now the team ships two changes on the base: a rebrand and a scale-up."
    run "cub function set --space cubbychat-base --unit backend set-env-var backend CHAT_TITLE \"Cubby Chat 2.0\""
    run "cub function set --space cubbychat-base --unit backend set-replicas 3"

    desc "Promote to dev and verify there before prod sees it."
    run "cub variant promote cubbychat-dev --dry-run -o mutations"
    run "cub variant promote cubbychat-dev"

    desc "Replicas flowed to 3. CHAT_TITLE is still dev's -- protected, so the"
    desc "promote left it alone while merging everything else."
    run "cub unit data --space cubbychat-dev backend | grep -E 'replicas:|CHAT_TITLE' -A1"

    desc "The rebrand did not vanish. What a merge cannot apply is recorded on the"
    desc "unit as a conflict, and waits there until someone decides."
    run "cub unit conflicts --space cubbychat-dev backend"

    desc "See what taking the base's value would mean, without taking it:"
    desc "--dry-run writes nothing -- no revision, and the conflict stays outstanding."
    run "cub unit conflicts --space cubbychat-dev backend --apply --dry-run -o mutations"

    desc "Dev keeps its title, so accept the state as it is and stop reporting it."
    desc "Dismissing clears the report, not the decision: the protection stands."
    run "cub unit conflicts --space cubbychat-dev backend --dismiss"

    desc "Dev is healthy, so release it."
    run "source ~/.confighub/clusters/dev.env"
    run "cub release publish cubbychat-dev"
    run "kubectl get pods -n cubbychat"

    desc "Prod never claimed the chat title, so it takes both changes."
    run "cub variant promote cubbychat-prod --dry-run -o mutations"
    run "cub variant promote cubbychat-prod"
    run "cub release publish cubbychat-prod"

    run "source ~/.confighub/clusters/prod.env"
    run "kubectl get pods -n cubbychat"
    run "cub unit data --space cubbychat-prod backend | grep -E 'replicas:|CHAT_TITLE' -A1"

    desc "The same base change, two outcomes, each correct for where it landed."
}

section_undo() {
    heading "Make and undo a change"

    desc "An urgent capacity change to production, released and then undone"
    desc "exactly. You never touch the cluster:"
    desc "you change the config in ConfigHub, the GitOps tool pulls as usual."
    run "cub function set --space cubbychat-prod --unit backend --protect set-replicas 5 -o mutations --change-desc \"Temporary boost in capacity to handle news event\""
    run "cub release publish cubbychat-prod"

    desc "Notice what that did not need: no branch, no PR, no pause of"
    desc "reconciliation, no kubectl. Validated like any change, recorded"
    desc "with the reason. Watch it land."
    run "source ~/.confighub/clusters/prod.env"
    run "kubectl get pods -n cubbychat"

    desc "The boost was meant to be temporary. set-replicas would put it back,"
    desc "but a larger set of changes you want to undo, not retype. Every change"
    desc "writes a revision, so there is always something to go back to."
    run "cub revision list --space cubbychat-prod backend"

    desc "Publishing tags each revision it shipped, so the previous release"
    desc "tag identifies the revision to restore."
    run "cub unit update --patch --space cubbychat-prod backend --restore Tag:release-2 -o mutations --change-desc \"Revert capacity boost\""
    run "cub release publish cubbychat-prod"

    desc "Restore goes forward, not back: a new revision carrying the old data"
    desc "and its protection settings."
    run "cub revision list --space cubbychat-prod backend"
    run "cub unit get --space cubbychat-prod backend -o mutations"

    desc "With config in git this is where people break glass: suspend"
    desc "reconciliation, edit the cluster, leave no record, skip validation,"
    desc "and hope reconciliation clobbers it later. Here the urgent path is"
    desc "the normal path: validated, recorded, and undone the way it was made."
}

section_cleanup() {
    heading "Cleanup"

    desc "cub cluster down removes the kind cluster; --delete-config also deletes"
    desc "the cluster's spaces and every deployment bound to its target."
    run "cub cluster down --name dev --delete-config"

    desc "Prod's units carry a delete gate, so the recursive delete refuses them."
    desc "That is the gate doing its job. Force past it deliberately."
    run "cub space delete cubbychat-prod --recursive-force"
    run "cub cluster down --name prod --delete-config"

    desc "And the base, which is not tied to any cluster."
    run "cub space delete cubbychat-base --recursive"
}

# setup_clusters builds both kind clusters up front, under DEMO_SETUP=INIT.
# Each takes a few minutes, which the tutorial's order spends in front of the
# audience -- once at the start, once again in the middle.
setup_clusters() {
    heading "Setting up the clusters"

    desc "DEMO_SETUP=INIT: building both kind clusters now, before the tutorial,"
    desc "so it does not stop for them later. This takes a few minutes."
    run_now "cub cluster up --name dev"
    run_now "cub cluster up --name prod"
}

main() {
    case "${1-}" in
        -h|--help)  usage; exit 0 ;;
        -l|--list)  usage; exit 0 ;;
    esac

    requested="$*"
    if [ -z "$requested" ]; then
        requested="cluster install release change prod flow undo"
    fi

    for name in $requested; do
        case " $SECTIONS " in
            *" $name "*) ;;
            *) echo "unknown section: $name" >&2; usage >&2; exit 1 ;;
        esac
    done

    if [ "$DEMO_SETUP" = "INIT" ]; then
        case " $requested " in
            *" cluster "*|*" prod "*) setup_clusters ;;
        esac
    fi

    for name in $requested; do
        "section_$name"
    done

    demo_end
    printf '\n%sDone. Sections: %s%s\n' "$_c_dim" "$SECTIONS" "$_c_off"
}

main "$@"
