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
//	LAYER_OWNER             the Kustomization label on every layer (default none)
//	FETCHED                 the digest each layer applied (default the checked one)
//	RELEASE_MOVES_AFTER     cub release get answers a newer digest after this many calls
const stubKubectl = `#!/usr/bin/env bash
echo "$*" >> "$STUB_LOG"
a=" $* "
case "$a" in
  *" config current-context "*) echo ctx-current ;;
  *" get namespace "*) ;;
  *" get kustomization "*labels*) echo "${LAYER_OWNER:-}" ;;
  *" get kustomization "*metadata.name*)
    name=$(sed 's/.* get kustomization \([^ ]*\) .*/\1/' <<<"$a")
    echo "$name|GitRepository|fleet-repo||./gitops/$name" ;;
  *" get kustomization "*conditions*) echo "the artifact could not be fetched" ;;
  *" get kustomization "*lastAppliedRevision*) echo "latest@${FETCHED:-sha256:checked}" ;;
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
		"git": "#!/usr/bin/env bash\nexit 0\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	calls := filepath.Join(t.TempDir(), "kubectl.log")
	cmd := exec.Command("bash", filepath.Join(dir, "handover.sh"))
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"STUB_LOG="+calls, "CLUSTER="+cluster, "CONFIGHUB_OCI=gw.example:5000",
		"REPO_ROOT="+t.TempDir())
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	log, _ := os.ReadFile(calls)
	state, _ := os.ReadFile(filepath.Join(dir, "handover-state", cluster+".log"))
	return handoverRun{string(out), string(log), string(state), err}
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

// In a fleet made with flux bootstrap, flux-system applies every layer, and a
// kubectl patch of one is undone on its next reconcile (measured on Flux
// v2.8.6). The script must find that before it changes anything.
func TestHandoverRefusesLayersAnotherKustomizationOwns(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "LAYER_OWNER=flux-system")
	if r.err == nil {
		t.Fatalf("owned layers must stop the run:\n%s", r.out)
	}
	if !strings.Contains(r.out, "apps(applied-by-flux-system)") || !strings.Contains(r.out, "hand them over in") {
		t.Errorf("should name the owned layers and the Git route:\n%s", r.out)
	}
	if strings.Contains(r.log, " patch ") || strings.Contains(r.log, " apply ") || strings.Contains(r.log, " create ") {
		t.Errorf("nothing may change before the refusal:\n%s", r.log)
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
// and says so.
func TestHandoverStopsWhenTheAppliedReleaseIsNotTheChecked(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "FETCHED=sha256:newer")
	if r.err == nil || !strings.Contains(r.out, "Flux applied latest@sha256:newer, but sha256:checked was checked") {
		t.Fatalf("want a stop naming both digests: %v\n%s", r.err, r.out)
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
