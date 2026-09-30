package flux

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These run the handover.sh that apply writes, against stand-ins for kubectl,
// cub, kustomize and git, so what the script does when a step fails is tested
// rather than read. The stand-in kubectl logs every call and answers from
// environment variables:
//
//	FAIL_PATCH / FAIL_WAIT  the layer whose patch, or whose Ready wait, fails
//	IUA_PATCH_FAIL          the image automation patch is denied
//	IUA_SUSPEND             what spec.suspend reads back (default true)
//	IUA_OWNER               the Kustomization label on it (default none)
//	LAYER_OWNER             the Kustomization label on every layer until the root is applied
//	OWNER_SOURCE_KIND       what that owner reads (default GitRepository)
//	GIT_COMMITTED           the owner's Git revision has the handover commit
//	FETCHED                 the digest each layer applied (default the checked one)
//	RELEASE_MOVES_AFTER     cub release get answers a newer digest after this many calls
const stubKubectl = `#!/usr/bin/env bash
echo "$*" >> "$STUB_LOG"
a=" $* "
case "$a" in
  *" config current-context "*) echo ctx-current ;;
  *" get namespace "*) ;;
  *" get kustomization confighub-root "*labels*) echo "${LAYER_OWNER:-}" ;;
  *" get kustomization "*labels*)
    # Once the root is applied it owns the layers, as it does on a cluster.
    if [ -f "$STUB_LOG.root" ]; then echo confighub-root; else echo "${LAYER_OWNER:-}"; fi ;;
  *" get kustomization "*metadata.name*)
    name=$(sed 's/.* get kustomization \([^ ]*\) .*/\1/' <<<"$a")
    echo "$name|GitRepository|fleet-repo||./gitops/$name" ;;
  *" get kustomization "*"{.spec.sourceRef.kind}"*) echo "${OWNER_SOURCE_KIND:-GitRepository}" ;;
  *" get kustomization "*"{.spec.sourceRef.name}"*) echo fleet-repo ;;
  *" get kustomization "*"{.spec.sourceRef.namespace}"*) ;;
  *" get kustomization "*"{.spec.suspend}"*) ;;
  *" get gitrepository "*) echo "main@sha1:0123456789abcdef0123456789abcdef01234567" ;;
  *" get kustomization "*lastAppliedRevision*)
    case "$a" in *" kustomization flux-system "*) echo "main@sha1:0123456789abcdef0123456789abcdef01234567" ;; *) echo "latest@${FETCHED:-sha256:checked}" ;; esac ;;
  *" get kustomization "*conditions*) echo "the artifact could not be fetched" ;;
  *" patch kustomization "*)
    [ -n "${FAIL_PATCH:-}" ] && [[ "$a" == *" patch kustomization $FAIL_PATCH "* ]] && exit 1 ;;
  *" wait --for=jsonpath"*) ;;
  *" wait "*)
    [ -n "${FAIL_WAIT:-}" ] && [[ "$a" == *"kustomization/$FAIL_WAIT "* ]] && exit 1 ;;
  *" get imageupdateautomation "*" -o name "*) echo "imageupdateautomation/x" ;;
  *" patch imageupdateautomation "*) [ -n "${IUA_PATCH_FAIL:-}" ] && exit 1 ;;
  *" get imageupdateautomation "*spec.suspend*) echo "${IUA_SUSPEND:-true}" ;;
  *" get imageupdateautomation "*labels*) echo "${IUA_OWNER:-}" ;;
  *" create secret "*) echo "kind: Secret" ;;
  *" apply -f - "*) cat >/dev/null ;;
  *" apply -f bootstrap/"*) touch "$STUB_LOG.root" ;;
esac
exit 0
`

// The stand-in git answers for a fleet checkout: GIT_COMMITTED says whether the
// revision the owner reads has the handover commit (layers gone, root in).
const stubGit = `#!/usr/bin/env bash
case " $* " in
  *" cat-file -e "*"^{commit}"*) exit 0 ;;
  *" cat-file -e "*:*) [ -n "${GIT_COMMITTED:-}" ] && exit 1; exit 0 ;;
  *" grep -q "*) [ -n "${GIT_COMMITTED:-}" ] && exit 0; exit 1 ;;
esac
exit 0
`

const stubCub = `#!/usr/bin/env bash
case " $* " in
  *" scout "*) exit 1 ;;
  *" unit data "*) echo "kind: Same" ;;
  *" release get "*)
    n=$(( $(cat "$STUB_LOG.releases" 2>/dev/null || echo 0) + 1 )); echo $n > "$STUB_LOG.releases"
    if [ -n "${RELEASE_MOVES_AFTER:-}" ] && [ "$n" -gt "$RELEASE_MOVES_AFTER" ]; then echo '"sha256:newer"'; else echo '"sha256:checked"'; fi ;;
  *" worker get "*) echo '"id"' ;;
esac
exit 0
`

const stubKustomize = `#!/usr/bin/env bash
case "$1" in build) echo "kind: Same" ;; *) echo v5 ;; esac
`

type handoverRun struct {
	out, log string
	state    string
	dir      string
	err      error
}

func runHandover(t *testing.T, env ...string) handoverRun {
	return runHandoverOn(t, "dev-1", env...)
}

func runHandoverOn(t *testing.T, cluster string, env ...string) handoverRun {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	dir := writeApply(t)
	bin := t.TempDir()
	for name, body := range map[string]string{
		"kubectl": stubKubectl, "cub": stubCub, "kustomize": stubKustomize,
		"git": stubGit,
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	calls := filepath.Join(t.TempDir(), "kubectl.log")
	for _, e := range env {
		// PRESEED_OWNER stands in for an earlier run that suspended this owner
		// and handed the layers to the root.
		if o, ok := strings.CutPrefix(e, "PRESEED_OWNER="); ok {
			if err := os.MkdirAll(filepath.Join(dir, "handover-state"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "handover-state", cluster+".owner"), []byte(o+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(calls+".root", nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	cmd := exec.Command("bash", filepath.Join(dir, "handover.sh"))
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"STUB_LOG="+calls, "CLUSTER="+cluster, "CONFIGHUB_OCI=gw.example:5000",
		"REPO_ROOT="+t.TempDir())
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	log, _ := os.ReadFile(calls)
	state, _ := os.ReadFile(filepath.Join(dir, "handover-state", cluster+".log"))
	return handoverRun{string(out), string(log), string(state), dir, err}
}

// Every kubectl call, and every command printed for a person to run later,
// names the one context the run was started on.
func TestHandoverIsBoundToOneContext(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a")
	if r.err != nil {
		t.Fatalf("a clean run should succeed: %v\n%s", r.err, r.out)
	}
	for _, l := range strings.Split(strings.TrimSpace(r.log), "\n") {
		if !strings.HasPrefix(l, "--context ctx-a ") {
			t.Errorf("kubectl ran without the chosen context: %s", l)
		}
	}
	for _, l := range strings.Split(r.out, "\n") {
		if strings.Contains(l, "kubectl ") && strings.Contains(l, "patch kustomization") && !strings.Contains(l, "--context 'ctx-a'") {
			t.Errorf("a printed way back does not name the context: %s", l)
		}
	}
	if !strings.Contains(r.out, `"sourceRef":{"kind":"GitRepository","name":"fleet-repo"},"path":"./gitops/apps"`) {
		t.Errorf("the way back should restore what the cluster had, both fields:\n%s", r.out)
	}
}

// Without FLUX_CONTEXT the context current at the start is pinned, so a switch
// in another terminal mid-run cannot move the rest of the handover.
func TestHandoverPinsTheCurrentContext(t *testing.T) {
	r := runHandover(t)
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	for _, l := range strings.Split(strings.TrimSpace(r.log), "\n") {
		if l != "config current-context" && !strings.HasPrefix(l, "--context ctx-current ") {
			t.Errorf("kubectl ran without the pinned context: %s", l)
		}
	}
}

func TestImageAutomationSuspension(t *testing.T) {
	for _, tc := range []struct {
		name, env string
		fails     bool
		want      string
	}{
		{"denied", "IUA_PATCH_FAIL=1", true, "was NOT suspended: the patch failed"},
		{"patch took no effect", "IUA_SUSPEND=false", true, "spec.suspend reads false after the patch"},
		{"suspended", "", false, "Suspended ImageUpdateAutomation apptique-dev: spec.suspend reads true"},
		// The image-automation layer is itself handed over, so after this run
		// the object's owner reads ConfigHub, and that is where the fix goes.
		{"owned by a moved layer", "IUA_OWNER=image-automation", false, "set spec.suspend: true in its ConfigHub unit"},
		{"owned by a layer still on Git", "IUA_OWNER=flux-system", false, "set spec.suspend: true in Git, which flux-system still reads"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := []string{"FLUX_CONTEXT=ctx-a"}
			if tc.env != "" {
				env = append(env, tc.env)
			}
			r := runHandover(t, env...)
			if (r.err != nil) != tc.fails {
				t.Errorf("failed=%v, want %v:\n%s", r.err != nil, tc.fails, r.out)
			}
			if !strings.Contains(r.out, tc.want) {
				t.Errorf("want %q in:\n%s", tc.want, r.out)
			}
			if tc.fails && !strings.Contains(r.out, "HANDOVER INCOMPLETE") {
				t.Errorf("an automation left running must be called incomplete:\n%s", r.out)
			}
			if tc.fails && strings.Contains(r.out, "Suspended ImageUpdateAutomation") {
				t.Errorf("must not claim a suspension it did not see:\n%s", r.out)
			}
		})
	}
}

// A layer that fails to become Ready after the root took it over stops the
// run. The root moved every layer at once, so every one is named, with the
// way back in the order that cannot delete anything: the root stops pruning
// first and goes last.
func TestHandoverStoppedMidwayNamesTheWayBack(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "FAIL_WAIT=apps")
	if r.err == nil {
		t.Fatalf("a layer that never became Ready must fail the run:\n%s", r.out)
	}
	if !strings.Contains(r.out, "HANDOVER STOPPED at apps") {
		t.Errorf("should say where it stopped:\n%s", r.out)
	}
	suspend := strings.Index(r.out, "patch kustomization confighub-root --type merge -p '{\"spec\":{\"suspend\":true,\"prune\":false}}'")
	restore := strings.Index(r.out, "--context 'ctx-a' -n 'flux-system' patch kustomization apps ")
	remove := strings.Index(r.out, "delete kustomization confighub-root")
	if suspend < 0 || restore < 0 || remove < 0 || !(suspend < restore && restore < remove) {
		t.Errorf("want: stop the root pruning, restore each layer, then remove the root; got %d, %d, %d:\n%s", suspend, restore, remove, r.out)
	}
	for _, l := range []string{"infrastructure", "tenants", "image-automation"} {
		if !strings.Contains(r.out, "patch kustomization "+l+" ") {
			t.Errorf("the root moved %s too, so it needs a way back:\n%s", l, r.out)
		}
	}
	// The root applied each layer's OCIRepository and is removed with pruning
	// off, so they are left behind reading ConfigHub unless the way back names
	// them, after the root. Measured by the e2e rig.
	for _, l := range []string{"apps", "infrastructure", "tenants", "image-automation"} {
		del := strings.Index(r.out, "-n 'flux-system' delete ocirepository "+l+"\n")
		if del < 0 || del < remove {
			t.Errorf("want %s's OCIRepository deleted after the root is gone:\n%s", l, r.out)
		}
	}
	if strings.Contains(r.out, "delete ocirepository apptique-examples") {
		t.Errorf("the layers' own Git sources are never deleted:\n%s", r.out)
	}
	if !strings.Contains(r.state, "NOT Ready apps: the artifact could not be fetched") {
		t.Errorf("the reason should be kept:\n%s", r.state)
	}
}

// A release published between the check and the root is caught before the
// root is applied, and nothing moves (confighub/helm-expt#2021).
func TestHandoverStopsBeforeTheRootWhenAReleaseMoved(t *testing.T) {
	// dev-1 has four layers: four digests read in step 1, one for the layers
	// Space, then the re-checks.
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "RELEASE_MOVES_AFTER=5")
	if r.err == nil || !strings.Contains(r.out, "before the root was applied") {
		t.Fatalf("should stop with nothing moved: %v\n%s", r.err, r.out)
	}
	if !strings.Contains(r.out, "checked release sha256:checked, but") || strings.Contains(r.log, "confighub-root.yaml") {
		t.Errorf("should name the digests and never apply the root:\n%s\n%s", r.out, r.log)
	}
}

// If Flux applied something other than the checked release, the run stops
// and says so. The root is checked first: it is what decides how each layer
// is reconciled.
func TestHandoverStopsWhenTheAppliedReleaseIsNotTheChecked(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "FETCHED=sha256:newer")
	if r.err == nil || !strings.Contains(r.out, "the root applied latest@sha256:newer, not the checked sha256:checked") {
		t.Fatalf("want a stop naming both digests: %v\n%s", r.err, r.out)
	}
}

// From review on #255: the layers Space republished between being read and
// the root going on would deliver an unchecked release. It stops first.
func TestHandoverStopsWhenTheLayersSpaceIsRepublished(t *testing.T) {
	// dev-1: four variant digests and the layers digest are read, the four
	// variants re-checked, then the layers Space re-checked: the tenth read.
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "RELEASE_MOVES_AFTER=9")
	if r.err == nil || !strings.Contains(r.out, "was republished since it was read") {
		t.Fatalf("want a stop before the root: %v\n%s", r.err, r.out)
	}
	if strings.Contains(r.log, "confighub-root.yaml") {
		t.Errorf("the root must not be applied:\n%s", r.log)
	}
}

// From review on #255: the root goes in the namespace the script uses.
func TestRootIsWrittenIntoTheSelectedNamespace(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "FLUX_NAMESPACE=flux-ops")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	root, err := os.ReadFile(filepath.Join(r.dir, "bootstrap", "dev-1", "confighub-root.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(root), "namespace: flux-ops") != 2 || strings.Contains(string(root), "namespace: flux-system") {
		t.Errorf("both root objects belong in flux-ops:\n%s", root)
	}
}

// The root is written for this cluster, with the gateway filled in, and
// every layer is recorded arriving at the checked release.
func TestHandoverWritesTheRootAndRecordsArrivals(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	if !strings.Contains(r.log, "apply -f bootstrap/dev-1/confighub-root.yaml") {
		t.Errorf("the root should be applied from bootstrap/dev-1/:\n%s", r.log)
	}
	if !strings.Contains(r.state, "Ready apps at latest@sha256:checked, from the root") {
		t.Errorf("each arrival should be recorded:\n%s", r.state)
	}
	if strings.Contains(r.log, "patch kustomization") {
		t.Errorf("the root moves the layers; nothing is patched:\n%s", r.log)
	}
}

// A second run finds the layers owned by the root this handover put there,
// which is not a reason to refuse.
func TestHandoverRerunAcceptsItsOwnRoot(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "LAYER_OWNER=confighub-root")
	if r.err != nil || strings.Contains(r.out, "applied by another Kustomization") {
		t.Errorf("the root owning a layer is this handover's own doing: %v\n%s", r.err, r.out)
	}
}

// Staging has no image automation and no image-automation layer.
func TestHandoverOnAClusterWithoutEveryLayer(t *testing.T) {
	r := runHandoverOn(t, "staging-1", "FLUX_CONTEXT=ctx-s")
	if r.err != nil {
		t.Fatalf("staging should hand over cleanly: %v\n%s", r.err, r.out)
	}
	if strings.Contains(r.log, "image-automation") || strings.Contains(r.log, "imageupdateautomation") {
		t.Errorf("staging has no image automation to touch:\n%s", r.log)
	}
	if !strings.Contains(r.log, "wait --for=condition=Ready kustomization/apps") {
		t.Errorf("apps should still arrive on staging:\n%s", r.log)
	}
}

// A bootstrapped fleet: flux-system owns the layers. It is suspended, the root
// takes them over, and the run pauses for the Git commit, leaving flux-system
// suspended: resuming it before the commit lands would re-apply the layers and
// then delete them, and all they run, when it does.
func TestBootstrappedHandoverPausesForTheCommit(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "LAYER_OWNER=flux-system")
	if ee, ok := r.err.(*exec.ExitError); !ok || ee.ExitCode() != 2 {
		t.Fatalf("want exit 2, paused: %v\n%s", r.err, r.out)
	}
	suspend := strings.Index(r.log, `patch kustomization flux-system --type merge -p {"spec":{"suspend":true}}`)
	root := strings.Index(r.log, "apply -f bootstrap/dev-1/confighub-root.yaml")
	if suspend < 0 || root < 0 || suspend > root {
		t.Errorf("flux-system must be suspended before the root goes on:\n%s", r.log)
	}
	if strings.Contains(r.log, `"suspend":false`) {
		t.Errorf("flux-system must stay suspended until Git has the commit:\n%s", r.log)
	}
	for _, want := range []string{"PAUSED", "git rm gitops/flux/expert-fleet/clusters/dev/apps.yaml", "Do NOT resume flux-system by hand"} {
		if !strings.Contains(r.out, want) {
			t.Errorf("want %q in:\n%s", want, r.out)
		}
	}
	if resume := strings.LastIndex(r.out, "patch kustomization flux-system --type merge -p '{\"spec\":{\"suspend\":false}}'"); resume < strings.LastIndex(r.out, "delete kustomization confighub-root") {
		t.Errorf("the way back resumes flux-system last, after the root is gone:\n%s", r.out)
	}
}

// Once the revision flux-system reads no longer defines the layers and does
// define the root, flux-system is resumed and the layers are still the root's.
func TestBootstrappedHandoverResumesOnceGitHasTheCommit(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "LAYER_OWNER=flux-system", "GIT_COMMITTED=1")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	if !strings.Contains(r.log, `patch kustomization flux-system --type merge -p {"spec":{"suspend":false}}`) {
		t.Errorf("flux-system should be resumed:\n%s", r.log)
	}
	if !strings.Contains(r.state, "resumed flux-system at main@sha1:") || !strings.Contains(r.out, "git revert") {
		t.Errorf("the resume should be recorded and the way back be a revert:\n%s\n%s", r.state, r.out)
	}
}

// An owner that does not read Git has no commit to wait for.
func TestHandoverRefusesAnOwnerNotReadingGit(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "LAYER_OWNER=platform", "OWNER_SOURCE_KIND=OCIRepository")
	if r.err == nil || !strings.Contains(r.out, "does not read Git") || strings.Contains(r.log, "patch ") {
		t.Errorf("want a refusal before anything changes: %v\n%s", r.err, r.out)
	}
}

// Found live: a re-run after the commit saw the layers already the root's,
// took the plain path, and left flux-system suspended while reporting success.
// The owner an earlier run suspended is in the state, and a re-run finishes.
func TestBootstrappedRerunFinishes(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "PRESEED_OWNER=flux-system", "GIT_COMMITTED=1")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	if !strings.Contains(r.out, "continuing: flux-system was suspended by an earlier run") ||
		!strings.Contains(r.log, `patch kustomization flux-system --type merge -p {"spec":{"suspend":false}}`) {
		t.Errorf("a re-run must resume the owner an earlier run suspended:\n%s\n%s", r.out, r.log)
	}
}
