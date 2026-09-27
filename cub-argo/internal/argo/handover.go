package argo

import (
	"fmt"
	"strings"
)

// HandoverScript writes the step that moves the estate: each layer's source is
// repointed at ConfigHub, top down, and nothing is deleted. Run it only after
// apply.sh, because a parent pointed at a Space that holds nothing is a parent
// syncing an empty source, and it prunes what it applied.
func HandoverScript(p *Plan, prefix string) string {
	targets := prefix + "-targets"
	var L []string
	add := func(f string, a ...any) { L = append(L, fmt.Sprintf(f, a...)) }

	add("#!/usr/bin/env bash")
	add("# Move this Argo CD estate onto ConfigHub. Written by `cub argo apply`.")
	add("# Read it, then run it:")
	add("#")
	add("#   ARGOCD_CONTEXT=<kubectl context of the cluster Argo CD runs on> bash handover.sh")
	add("#")
	add("# Run apply.sh first. Every step here is a patch, never a delete: the")
	add("# objects, their names and Argo's tracking of what they own all survive, so")
	add("# no workload is recreated. It goes top down, because a parent pointed at a")
	add("# Space that holds nothing prunes the children it applied.")
	add("#")
	add("# This has not been rehearsed on a cluster. Read every step before running")
	add("# it, and start with one non-production estate.")
	add("set -euo pipefail")
	add(`cd "$(dirname "$0")"`)
	add(`k() { kubectl ${ARGOCD_CONTEXT:+--context "$ARGOCD_CONTEXT"} "$@"; }`)
	add(`step() { printf '\n== %%s\n' "$*"; }`)
	add(`ns=${ARGOCD_NAMESPACE:-argocd}`)
	add("")
	add("# The gateway address a repointed source reads. cub reports it for a Target.")
	add("gateway() { cub target get --space %s \"$1\" -o jq=.Target.Parameters.OCIRepository 2>/dev/null || true; }", targets)
	add("")

	add(`step "0/4 Check before changing anything"`)
	add(`cub space list --quiet >/dev/null || { echo "cub is not logged in: run cub auth login"; exit 1; }`)
	add(`image=$(k get deployment argocd-repo-server -n "$ns" -o jsonpath='{.spec.template.spec.containers[0].image}')`)
	add(`version=${image##*:}; version=${version#v}`)
	add(`if [ "$(printf '%%s\n' 3.1 "${version%%.*}.${version#*.}" | sort -V | head -1)" != 3.1 ]; then`)
	add(`  echo "Argo CD runs $version; an oci:// source is read natively from v3.1, so this handover is not available here"; exit 1`)
	add("fi")

	// The projects gate every repoint, so they are checked before anything moves.
	var blocked []string
	for _, line := range p.Handover {
		if strings.HasPrefix(line, "first, add the gateway's oci:// address to sourceRepos") {
			blocked = append(blocked, line)
		}
	}
	if len(blocked) > 0 {
		add("# Until the gateway is allowed as a source, every repoint below is refused.")
		add("echo %s", q(blocked[0]))
		add(`echo "  cub target get --space %s argocd -o jq=.Target.Parameters.OCIRepository   # the address to add"`, targets)
		add(`read -r -p "Add it to sourceRepos on those AppProjects, then press Return to carry on. " _`)
	}
	add("")

	add(`step "1/4 The credential Argo reads the gateway with"`)
	add("# A repository Secret for the gateway, holding the Targets' server worker.")
	add("# The ID and secret go from cub into the Secret through file descriptors,")
	add("# never to disk, the command line, or the terminal.")
	add(`addr=${CONFIGHUB_OCI:-$(gateway argocd)}`)
	add(`if [ -z "$addr" ]; then`)
	add(`  echo "Could not read the gateway address from Target %s/argocd."`, targets)
	add(`  echo "Find it with: cub target get --space %s argocd"`, targets)
	add(`  echo "then re-run with CONFIGHUB_OCI=<address> bash handover.sh"; exit 1`)
	add("fi")
	add("# The repository Secret's type for a plain OCI source is the one field here")
	add("# that has not been checked against a live Argo CD. Verify it against your")
	add("# version's repository-Secret docs before trusting this step.")
	add(`k create secret generic confighub-%s --namespace "$ns" \`, targets)
	add(`  --from-literal=type=helm --from-literal=enableOCI=true \`)
	add(`  --from-literal=url="${addr%%%%/*}" \`)
	add(`  --from-file=username=<(cub worker get --space %s server-worker -o jq=.BridgeWorker.BridgeWorkerID | tr -d '\n') \`, targets)
	add(`  --from-file=password=<(cub worker get --space %s server-worker --include-secret -o jq=.BridgeWorker.Secret | tr -d '\n') \`, targets)
	add(`  --dry-run=client -o yaml | k apply -f -`)
	add(`k -n "$ns" label secret confighub-%s argocd.argoproj.io/secret-type=repository --overwrite`, targets)
	add("")

	cs := p.controlSpaces(prefix)
	step := 2
	for _, s := range cs {
		add(`step "%d/4 Repoint %s at %s"`, step, s.Parent, s.Space)
		add("# Its children are Units there by now, released to the argocd Target.")
		add(`addr=$(gateway argocd)`)
		add(`k -n "$ns" get application %s -o jsonpath='{.spec.source.repoURL}' | grep -q '^oci://' && echo %s || \`,
			s.Parent, q(s.Parent+" already reads ConfigHub"))
		add(`  k -n "$ns" patch application %s --type merge -p "{\"spec\":{\"source\":{\"repoURL\":\"oci://${addr}\",\"path\":\".\",\"targetRevision\":\"latest\"}}}"`, s.Parent)
		add(`k -n "$ns" wait --for=jsonpath='{.status.sync.status}'=Synced application/%s --timeout=3m`, s.Parent)
		step++
	}

	add(`step "%d/4 Point each ApplicationSet's template at its clusters' Targets"`, step)
	add("# The ApplicationSet goes on generating the same Applications under the same")
	add("# names, so Argo's tracking does not change and nothing is orphaned. Each")
	add("# generated Application reads its own cluster's Target.")
	for _, c := range p.Components {
		if c.Kind != "ApplicationSet" {
			continue
		}
		add("# %s generates:", c.Source)
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				add("#   %-28s from %s/%s", v.Application, targets, v.Cluster)
			}
		}
		add("echo %s", q(fmt.Sprintf(
			"Edit the %s Unit's template: set spec.template.spec.source.repoURL to oci://<the gateway>/<this cluster's Target>, path '.', targetRevision latest. Promote it like any other change.",
			c.Source)))
	}
	for _, w := range p.Windows {
		if w.Kind != "deny" || len(w.Applications) == 0 {
			continue
		}
		add("echo %s", q(fmt.Sprintf(
			"Note: AppProject %s denies syncing %s on schedule %q for %s. The repoint lands, but its first sync waits for the window to close.",
			w.Project, strings.Join(w.Applications, ", "), w.Schedule, w.Duration)))
	}
	add("")
	add("echo")
	add("echo %s", q("Done once every Application reports Synced from an oci:// source:"))
	add("echo %s", q("  kubectl -n $ns get applications -o custom-columns=NAME:.metadata.name,SOURCE:.spec.source.repoURL,SYNC:.status.sync.status"))
	add("echo %s", q("Nothing was deleted. To go back, patch each source to its Git repoURL and path."))
	return strings.Join(L, "\n") + "\n"
}
