package flux

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// The scripts here follow `cub sveltos`'s apply.sh, which is the recorded shape
// for this model: --allow-exists and --quiet throughout, each step checking
// what ConfigHub already says, and the whole script safe to re-run.

func q(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func short(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", sum[:4])
}

// Workflow is the stages a layer's changes move through.
func Workflow(c *Component) string {
	var b strings.Builder
	b.WriteString("# The stages a change to this layer moves through, and what each waits\n")
	b.WriteString("# for. A release is published into a stage only once the change is approved\n")
	b.WriteString("# there, and a stage is entered only once the stage ahead has released it.\n")
	b.WriteString("#\n")
	b.WriteString("# approval is not one of the built-in gates (Validated, Released, Healthy),\n")
	b.WriteString("# so it is declared here as the Attestation each stage's release waits for.\n")
	b.WriteString("# AllowAuthors belongs to that declaration, not to the workflow: it lets\n")
	b.WriteString("# whoever promoted a change also approve it, which one person trying this\n")
	b.WriteString("# out needs.\n")
	b.WriteString("#\n")
	b.WriteString("# As written this is a SINGLE-OPERATOR workflow. An approval recorded under\n")
	b.WriteString("# it shows the change was approved, not that anyone but its author looked.\n")
	b.WriteString("# To require a separate reviewer, set AllowAuthors: false (and, if wanted,\n")
	b.WriteString("# Count: 2 or FromUserIDs), then replace the live workflow with this file:\n")
	b.WriteString("#   cub changeworkflow update --space <base Space> rollout --filename change-workflow.yaml\n")
	b.WriteString("# It governs change orders created after that, not one already under way.\n")
	b.WriteString("Slug: rollout\n")
	b.WriteString("AttestationPrerequisites:\n")
	b.WriteString("  - Name: approval\n")
	b.WriteString("    AllowAuthors: true\n")
	b.WriteString("Stages:\n")
	for i, st := range c.Stages {
		b.WriteString(fmt.Sprintf("  - Name: %s\n", st.Name))
		b.WriteString(fmt.Sprintf("    WhereSpace: Labels.Stage = '%s'\n", st.Name))
		if i > 0 {
			b.WriteString("    Prerequisites:\n      - Released\n")
			for _, r := range c.Require {
				// Healthy reads the live status of the stage before, which
				// `cub flux status` writes; without a reporter it never passes.
				b.WriteString(fmt.Sprintf("      - %s\n", r))
			}
		}
		b.WriteString("    ReleasePrerequisites:\n      - approval\n")
	}
	return b.String()
}

func stagesJSON(c *Component) string {
	type stage struct {
		Name                 string   `json:"Name"`
		WhereSpace           string   `json:"WhereSpace"`
		Prerequisites        []string `json:"Prerequisites,omitempty"`
		ReleasePrerequisites []string `json:"ReleasePrerequisites"`
	}
	var stages []stage
	for i, st := range c.Stages {
		s := stage{Name: st.Name, WhereSpace: fmt.Sprintf("Labels.Stage = '%s'", st.Name),
			ReleasePrerequisites: []string{"approval"}}
		if i > 0 {
			s.Prerequisites = append([]string{"Released"}, c.Require...)
		}
		stages = append(stages, s)
	}
	out, _ := json.Marshal(map[string]any{"Stages": stages})
	return string(out)
}

func stageNames(c *Component) string {
	var n []string
	for _, st := range c.Stages {
		n = append(n, st.Name)
	}
	return strings.Join(n, ",")
}

// ApplyScript fills ConfigHub. It touches no cluster: Flux goes on
// reconciling Git throughout, and handover.sh is what moves the fleet.
func ApplyScript(p *Plan, prefix, repoRel string) string {
	targets := prefix + "-targets"
	var L []string
	add := func(f string, a ...any) { L = append(L, fmt.Sprintf(f, a...)) }

	add("#!/usr/bin/env bash")
	add("# Fill ConfigHub with this Flux fleet: one base per layer, one variant per")
	add("# cluster, one Target per cluster. Written by `cub flux apply`. Read it,")
	add("# then run it:")
	add("#")
	add("#   bash apply.sh")
	add("#")
	add("# Nothing here touches a cluster. Flux goes on reconciling Git throughout,")
	add("# and ConfigHub ends up holding a parallel copy that nothing reads.")
	add("# handover.sh is what moves the fleet onto it; until you run that, deleting")
	add("# these Spaces puts you back exactly where you started.")
	add("set -euo pipefail")
	add(`cd "$(dirname "$0")"`)
	add(`step() { printf '\n== %%s\n' "$*"; }`)
	add("# Flux paths are written from the top of the repository, so the script needs")
	add("# to know where that is.")
	if repoRel == "" {
		repoRel = "."
	}
	add("REPO_ROOT=${REPO_ROOT:-%s}", q(repoRel))
	add(`[ -d "$REPO_ROOT" ] || { echo "REPO_ROOT=$REPO_ROOT is not a directory: point it at the repository checkout"; exit 1; }`)
	add("")
	// Completed says the order was promoted through every stage, not that its
	// variants were released: a run stopped between promoting and publishing,
	// or run with PROPOSE_ONLY, leaves it Completed with a variant unreleased.
	add("# rolled_out <order> <variant Spaces>: promoted through every stage, and")
	add("# every variant released.")
	add(`rolled_out() {`)
	add(`  local o=$1 s; shift`)
	add(`  [ "$(cub changeorder get --space "${o%%/*}" "${o#*/}" -o jq=.ChangeOrder.Stage)" = Completed ] || return 1`)
	add(`  for s in "$@"; do cub release get --space "$s" --oci-reference latest -o jq=.Release.ManifestDigest >/dev/null 2>&1 || return 1; done`)
	add(`}`)
	add("# A variant must hold every unit of its base before it is released: Flux")
	add("# prunes from a cluster whatever a release no longer holds.")
	add("holds() {")
	add(`  local n; n=$(cub unit list --space "$1" -o jq=length)`)
	add(`  [ "$n" -ge "$2" ] || { echo "$1 holds $n of its $2 units; run this script again" >&2; return 1; }`)
	add("}")
	add(`stages_are() { [ "$(cub changeworkflow get --space "$1" "$2" -o 'jq=[.ChangeWorkflow.Stages[].Name] | join(",")')" = "$3" ]; }`)
	add("# add_stage <base> <stage> <earlier stages, nearest first> <stage JSON>:")
	add("# insert a stage the live workflow lacks after the nearest earlier stage it")
	add("# has (first if none), keeping every stage it has as it is.")
	add(`add_stage() {`)
	add(`  local have after="" e`)
	add(`  have=$(cub changeworkflow get --space "$1" rollout -o 'jq=[.ChangeWorkflow.Stages[].Name] | join(",")')`)
	add(`  case ",$have," in *",$2,"*) return 0 ;; esac`)
	add(`  for e in ${3//,/ }; do case ",$have," in *",$e,"*) after=$e; break ;; esac; done`)
	add(`  cub changeworkflow get --space "$1" rollout -o "jq=.ChangeWorkflow.Stages | ((map(.Name) | index(\"$after\")) // -1) as \$i | {Stages: (.[:\$i+1] + [$4] + .[\$i+1:])}" |`)
	add(`    cub changeworkflow update --patch --space "$1" rollout --from-stdin --quiet`)
	add(`  echo "  $1: added stage $2 after ${after:-nothing}, keeping every stage it had"`)
	add(`}`)
	add("promote() {")
	add("  local out")
	add("  for _ in 1 2 3; do")
	add(`    out=$(cub variant promote --change-order "$1" --target-stage "$2" --quiet 2>&1 >/dev/null) && return 0`)
	add(`    case "$out" in *"nothing left to promote"*) return 0 ;; esac`)
	add("    sleep 15")
	add("  done")
	add(`  echo "$out" >&2; return 1`)
	add("}")
	add("# PROPOSE_ONLY=1 approves nothing: each stage is promoted, and a release that")
	add("# needs an approval waits in ConfigHub for a person to give it. Run the script")
	add("# again to publish what was approved. A stage with nothing new for its")
	add("# variants needs no approval, so a joining cluster waits in its own stage")
	add("# only. `cub flux watch` runs the script this way when a cluster joins; the")
	add("# pattern is cub sveltos watch's.")
	add(`approve() { [ -n "${PROPOSE_ONLY:-}" ] || cub variant approve --change-order "$1" --stage "$2" --quiet; }`)
	add("publish() {")
	add("  local out")
	add(`  holds "$1" "$3" || return 1`)
	add(`  out=$(cub release publish "$1" --revision "ChangeOrder:$2" --quiet 2>&1) && return 0`)
	add(`  case "$out" in`)
	add(`    *"no changes were made since :latest bundle"*) echo "$1 already released" ;;`)
	add(`    *"requires "*"attestation(s)"*)`)
	add(`      [ -n "${PROPOSE_ONLY:-}" ] || { echo "$out" >&2; return 1; }`)
	add(`      echo "$1 waits for approval: cub variant approve --change-order $2 --stage $4" ;;`)
	add(`    *) echo "$out" >&2; return 1 ;;`)
	add(`  esac`)
	add("}")
	add("# released <space>: the Space has a published release.")
	add(`released() { cub release get --space "$1" --oci-reference latest -o jq=.Release.ManifestDigest >/dev/null 2>&1; }`)
	add("# Each layer is rendered by kustomize here rather than by the plugin, so")
	add("# what ConfigHub stores is exactly what Flux builds today. postBuild")
	add("# substitution is Flux's, applied on the cluster, so it is left in place.")
	add("render() {")
	add("  mkdir -p render")
	add(`  kustomize build ${KUSTOMIZE_FLAGS:-} "$REPO_ROOT/$1" > "render/$2.yaml"`)
	add(`  [ -s "render/$2.yaml" ] || { echo "kustomize build $REPO_ROOT/$1 rendered nothing" >&2; return 1; }`)
	add("}")
	add("")

	add(`step "0/5 Check before changing anything"`)
	// cub auth status, not a list of anything. A list call goes through the
	// entity API, which fails on a version skew between client and server --
	// measured: cub v0.6.2 against server v0.5.1 returns "field 'ComponentID'
	// does not exist on entity type Space" and exits 1 while the session is
	// perfectly good. Using that as the login check aborts this script on its
	// own first line and sends the reader to re-login, which cannot help.
	add(`cub auth status >/dev/null 2>&1 || { echo "cub is not logged in: run cub auth login"; exit 1; }`)
	// The skew is worth saying out loud, because it is what later steps fail
	// on, and nothing else in the run would explain them.
	add(`cub auth status 2>&1 | grep -i '^Warning:' && echo "  Steps below may fail on that skew rather than on anything here."`)
	add(`command -v kustomize >/dev/null || { echo "kustomize is not on PATH; the layers are rendered with it"; exit 1; }`)
	add("")

	add(`step "1/5 One named Target per cluster, in %s"`, targets)
	add("cub space create %s --allow-exists --quiet", targets)
	add("# A server-hosted worker has no process behind it and no role in the")
	add("# organization; it holds the Targets and is the credential Flux reads with.")
	add("cub worker create --space %s server-worker --is-server-worker --org-role none --allow-exists --quiet", targets)
	for _, c := range p.Clusters {
		add("cub target create %s '{}' server-worker --space %s --provider OCI --toolchain Any --allow-exists --quiet", c.Name, targets)
	}
	add("")

	add(`step "2/5 One component per layer: a base holding what it renders to, and a rollout workflow"`)
	for _, c := range p.Components {
		add("cub component create %s-%s --allow-exists --quiet", prefix, c.Name)
		add("cub space create %s --component %s-%s --label Component=%s-%s --label Role=base --allow-exists --quiet", c.Base, prefix, c.Name, prefix, c.Name)
		if c.BaseDir != "" && !strings.Contains(c.BaseDir, ", ") {
			add("render %s %s", q(c.BaseDir), q(c.Name+"-base"))
			add("cub unit create --space %s %s render/%s-base.yaml --change-desc %s --allow-exists --quiet",
				c.Base, c.Name, c.Name, q(fmt.Sprintf("Onboard the %s layer from %s", c.Name, c.BaseDir)))
		}
		// The first release of every variant is made before anything reads
		// ConfigHub, so it cannot wait for Healthy. A change order keeps the
		// workflow it was created under, so onboarding runs under one without
		// what --require adds, and the workflow is replaced once it is done.
		file, oc := "change-workflow.yaml", c
		if len(c.Require) > 0 {
			file, oc = onboardingWorkflow, c.onboarding()
		}
		add("cub changeworkflow create --space %s rollout --filename %s/%s --allow-exists --quiet", c.Base, c.Name, file)
		if p.handedOver() {
			// A handed-over cluster's stage is in the live workflow but not in
			// this repository's view of it, so rewriting the stages from here
			// would drop it and let a change skip that cluster. A cluster that
			// joins still needs its stage, so each stage the plan has and the
			// workflow lacks is inserted after the one before it, and every
			// stage already there is kept as it is, prerequisites and all.
			add("# Some clusters are handed over, so their stages are not in this view:")
			add("# stages the workflow lacks are added, and every stage it has is kept.")
			for _, st := range oc.Stages {
				// The stages before this one in the whole fleet's order,
				// nearest first: a handed-over cluster's stage is among them,
				// though this plan has no variant of it.
				var before []string
				pos := -1
				for i, name := range p.Stages {
					if name == st.Name {
						pos = i
					}
				}
				for i := pos - 1; i >= 0; i-- {
					before = append(before, p.Stages[i])
				}
				one := map[string]any{"Name": st.Name, "WhereSpace": fmt.Sprintf("Labels.Stage = '%s'", st.Name), "ReleasePrerequisites": []string{"approval"}}
				if pos > 0 {
					one["Prerequisites"] = append([]string{"Released"}, oc.Require...)
				}
				js, _ := json.Marshal(one)
				add("add_stage %s %s %s %s", c.Base, q(st.Name), q(strings.Join(before, ",")), q(string(js)))
			}
		} else {
			add("stages_are %s rollout %s || echo %s | cub changeworkflow update --patch --space %s rollout --from-stdin --quiet",
				c.Base, q(stageNames(c)), q(stagesJSON(oc)), c.Base)
		}
	}
	add("")

	add(`step "3/5 One variant per cluster, each holding what its own path renders to"`)
	add("# Step 5 makes each layer's Unit itself, so a cub that would add one when")
	add("# it makes a variant for a Flux cluster is told not to.")
	add(`no_flux_layer=; case "$(cub variant create --help 2>&1)" in *--no-flux-layer*) no_flux_layer=--no-flux-layer ;; esac`)
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				add("cub variant create %s %s --stage %s --space-pattern template:%s --target %s/%s --space-label Role=deployment --space-label Cluster=%s $no_flux_layer --allow-exists --quiet",
					v.Cluster, c.Base, st.Name, v.Space, targets, v.Cluster, v.Cluster)
				add("render %s %s", q(v.Path), q(v.Space))
				add("cub unit update --space %s %s render/%s.yaml --change-desc %s --quiet",
					v.Space, c.Name, v.Space, q(fmt.Sprintf("What %s renders for %s", v.Path, v.Cluster)))
				add("holds %s 1", v.Space)
			}
		}
	}
	add("")

	add(`step "4/5 Release each variant, stage by stage: promote, approve, publish"`)
	for _, c := range p.Components {
		var all []string
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				all = append(all, v.Space)
			}
		}
		if len(all) == 0 {
			continue
		}
		order := "onboard-" + short(c.Base, strings.Join(all, ","))
		add("cub changeorder create --space %s %s --change-workflow %s/rollout --description %s --allow-exists --quiet",
			c.Base, order, c.Base, q("First release of "+strings.Join(all, ", ")))
		add("if rolled_out %s/%s %s; then", c.Base, order, strings.Join(all, " "))
		add("  echo %s", q(c.Name+": every variant in this plan is released"))
		add("else")
		// Every stage in the fleet's order. A handed-over cluster's stage has no
		// variant in this plan, but its variant Space is still in ConfigHub, and
		// the stages after it need it to have taken this order: without that,
		// the next promotion is refused ("has not taken change order").
		byStage := map[string]Stage{}
		for _, st := range c.Stages {
			byStage[st.Name] = st
		}
		for _, name := range p.Stages {
			if st, ok := byStage[name]; ok {
				add("  promote %s/%s %s", c.Base, order, st.Name)
				add("  approve %s/%s %s", c.Base, order, st.Name)
				for _, v := range st.Variants {
					add("  publish %s %s/%s 1 %s", v.Space, c.Base, order, st.Name)
				}
				continue
			}
			for _, cl := range p.Clusters {
				if cl.Dir != name || cl.LayersSpace == "" {
					continue
				}
				space := fmt.Sprintf("%s-%s-%s", prefix, c.Name, cl.Name)
				add("  if cub space get %s >/dev/null 2>&1; then  # %s is handed over", space, cl.Name)
				add("    promote %s/%s %s", c.Base, order, name)
				add("    approve %s/%s %s", c.Base, order, name)
				add("    publish %s %s/%s 1 %s", space, c.Base, order, name)
				add("  fi")
			}
		}
		add("fi")
	}
	add("")
	add(`step "5/5 One Space per cluster holding its layers, which one root on that cluster reads"`)
	add("# Each layer's Unit is its own Kustomization, reading its variant's Space,")
	add("# and the OCIRepository for that Space. The gateway address is the one the")
	add("# cluster reaches, which only you know; nothing reads these until the root")
	add("# is on the cluster, which handover.sh puts there.")
	raw := func(s string) { L = append(L, s) }
	raw(`if [ -z "${CONFIGHUB_OCI:-}" ]; then`)
	raw(`  echo "  skipped: set CONFIGHUB_OCI to the gateway host the clusters reach (and CONFIGHUB_OCI_PLAIN_HTTP=1 if it serves plain HTTP), then re-run. handover.sh needs these Spaces."`)
	raw(`else`)
	raw(`  insecure=false; [ -z "${CONFIGHUB_OCI_PLAIN_HTTP:-}" ] || insecure=true`)
	raw(`  mkdir -p render`)
	raw(`  # layer_unit <space> <unit> <file>`)
	raw(`  layer_unit() {`)
	raw(`    sed -e "s|` + gatewayMarker + `|$CONFIGHUB_OCI|" -e "s|` + insecureMarker + `|$insecure|" "$3" > "render/$1-$2.yaml"`)
	raw(`    cub unit create --space "$1" "$2" "render/$1-$2.yaml" --allow-exists --quiet`)
	raw(`    cub unit update --space "$1" "$2" "render/$1-$2.yaml" --quiet`)
	raw(`  }`)
	raw(`  # A release with nothing new is refused, which here means it is current.`)
	raw(`  all_released() { local s; for s in "$@"; do released "$s" || return 1; done; }`)
	raw(`  publish_layers() { out=$(cub release publish "$1" --quiet 2>&1) || case "$out" in *"no changes"*) ;; *) echo "$out" >&2; return 1 ;; esac; }`)
	for _, cl := range p.Clusters {
		space := cl.layers(prefix)
		raw(fmt.Sprintf(`  cub space create %s --release-target %s/%s --label Role=layers --label Cluster=%s --allow-exists --quiet`, space, targets, cl.Name, cl.Name))
		// Once the Space exists, the Target names it, and the Secret its layers
		// pull with: the signal a Flux-aware `cub variant create` reads to add a
		// variant made outside this plan as a layer here (the Flux counterpart of
		// confighub.com/argo-apps-space).
		// A root that pulls anonymously has no Secret; null removes one an
		// earlier run named, as the patch merges.
		secret := "null"
		if s := cl.pullSecret(prefix); s != "" {
			secret = strconv.Quote(s)
		}
		ann := fmt.Sprintf(`{"Annotations":{"confighub.com/flux-layers-space":%s,"confighub.com/flux-pull-secret":%s}}`, strconv.Quote(space), secret)
		raw(fmt.Sprintf(`  echo %s | cub target update --patch --space %s %s --from-stdin --quiet`, q(ann), targets, cl.Name))
		for _, c := range p.Components {
			for _, st := range c.Stages {
				for _, v := range st.Variants {
					if v.Cluster == cl.Name {
						raw(fmt.Sprintf(`  layer_unit %s %s %s`, q(space), q(c.Name), q(layerFile(cl.Name, c.Name))))
					}
				}
			}
		}
		var spaces []string
		for _, c := range p.Components {
			for _, st := range c.Stages {
				for _, v := range st.Variants {
					if v.Cluster == cl.Name {
						spaces = append(spaces, v.Space)
					}
				}
			}
		}
		// A layer reading a Space with no release yet fails "latest: not
		// found", so a joining cluster's root has nothing to read until every
		// one of its variants is released.
		raw(fmt.Sprintf(`  if [ -z "${PROPOSE_ONLY:-}" ] || all_released %s; then publish_layers %s; else echo "  %s waits: not every variant of %s is released yet"; fi`, strings.Join(spaces, " "), space, space, cl.Name))
	}
	raw(`fi`)
	add("")
	add("echo")
	var requiring []*Component
	for _, c := range p.Components {
		if len(c.Require) > 0 {
			requiring = append(requiring, c)
		}
	}
	if len(requiring) > 0 && !p.handedOver() {
		add("")
		add(`step "Every change from now on also waits for %s in the stage before"`, strings.Join(requiring[0].Require, ", "))
		add("# Change orders created from here on use this workflow; the onboarding")
		add("# ones above keep the one they were created under.")
		add("# Healthy passes on the live status `cub flux status` writes, so once a")
		add("# cluster reads ConfigHub, keep it running there.")
		for _, c := range requiring {
			add("cub changeworkflow update --space %s rollout --filename %s/change-workflow.yaml --quiet", c.Base, c.Name)
		}
	}
	add("echo %s", q("Done. ConfigHub holds this fleet and nothing reads it yet."))
	add("echo %s", q("Flux is still reconciling Git. Read handover.sh next: that is the step that moves it."))
	return strings.Join(L, "\n") + "\n"
}

// onboardingWorkflow is the file apply.sh creates the workflow from when
// --require adds a prerequisite the first releases cannot meet.
const onboardingWorkflow = "change-workflow.onboarding.yaml"

// onboarding is the component as its first release sees it: without the
// prerequisites --require adds, which nothing can meet before the handover.
func (c *Component) onboarding() *Component {
	o := *c
	o.Require = nil
	return &o
}

// layerFile is where apply writes one layer's delivery Unit for one cluster.
func layerFile(cluster, layer string) string {
	return "layers/" + cluster + "/" + layer + ".yaml"
}

// handedOver reports whether any cluster's layers now come from ConfigHub, so
// this repository no longer shows every stage of a shared workflow.
func (p *Plan) handedOver() bool {
	for _, c := range p.Clusters {
		if c.LayersSpace != "" {
			return true
		}
	}
	return false
}
