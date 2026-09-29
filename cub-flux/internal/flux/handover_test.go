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
//	FETCHED                 the digest the OCIRepository fetched (default the checked one)
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
  *" get ocirepository "*) echo "latest@${FETCHED:-sha256:checked}" ;;
  *" patch kustomization "*)
    [ -n "${FAIL_PATCH:-}" ] && [[ "$a" == *" patch kustomization $FAIL_PATCH "* ]] && exit 1 ;;
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
  *" release get "*) echo '"sha256:checked"' ;;
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

// The second layer fails after the first has moved: the run stops, says which
// layers moved, and prints how to put back exactly those, on the right cluster.
func TestHandoverStoppedMidwayNamesTheWayBack(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "FAIL_WAIT=apps")
	if r.err == nil {
		t.Fatalf("a layer that never became Ready must fail the run:\n%s", r.out)
	}
	if !strings.Contains(r.out, "HANDOVER STOPPED at apps") {
		t.Errorf("should say where it stopped:\n%s", r.out)
	}
	for _, moved := range []string{"infrastructure", "apps"} {
		if !strings.Contains(r.out, "--context 'ctx-a' -n 'flux-system' patch kustomization "+moved+" ") {
			t.Errorf("%s moved and needs a way back on ctx-a:\n%s", moved, r.out)
		}
	}
	if strings.Contains(r.out, "patch kustomization tenants ") {
		t.Errorf("tenants never moved and must not be offered a rollback:\n%s", r.out)
	}
	if !strings.Contains(r.state, "NOT Ready apps: the artifact could not be fetched") {
		t.Errorf("the reason the patch did not take should be kept:\n%s", r.state)
	}
}

// A patch that fails before any layer moved changes nothing, and says so.
func TestHandoverStoppedBeforeAnyMove(t *testing.T) {
	r := runHandover(t, "FAIL_PATCH=infrastructure")
	if r.err == nil || !strings.Contains(r.out, "before any layer's source was changed") {
		t.Errorf("should fail and say nothing moved: %v\n%s", r.err, r.out)
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

// Staging has no image automation and no image-automation layer. A handover
// there moves the layers it has, touches neither, and finishes.
func TestHandoverOnAClusterWithoutEveryLayer(t *testing.T) {
	r := runHandoverOn(t, "staging-1", "FLUX_CONTEXT=ctx-s")
	if r.err != nil {
		t.Fatalf("staging should hand over cleanly: %v\n%s", r.err, r.out)
	}
	if strings.Contains(r.log, "image-automation") || strings.Contains(r.log, "imageupdateautomation") {
		t.Errorf("staging has no image automation to touch:\n%s", r.log)
	}
	if !strings.Contains(r.log, "patch kustomization apps ") {
		t.Errorf("apps should still move on staging:\n%s", r.log)
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

// A release published between the check and the swap would go out unchecked.
// The swap confirms Flux fetched the digest that was checked, and moves
// nothing if not (confighub/helm-expt#2021).
func TestHandoverStopsWhenTheReleaseMovedAfterTheCheck(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "FETCHED=sha256:newer")
	if r.err == nil {
		t.Fatalf("a release other than the checked one must stop the run:\n%s", r.out)
	}
	if !strings.Contains(r.out, "checked at sha256:checked, but Flux fetched latest@sha256:newer") {
		t.Errorf("should name both digests:\n%s", r.out)
	}
	if strings.Contains(r.log, "patch kustomization") {
		t.Errorf("no layer may move:\n%s", r.log)
	}
}

// The check is run against the digest the script recorded, not "latest".
func TestHandoverChecksTheRecordedRelease(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	if !strings.Contains(r.state, "Ready apps at latest@sha256:checked") {
		t.Errorf("the applied revision should be recorded:\n%s", r.state)
	}
}
