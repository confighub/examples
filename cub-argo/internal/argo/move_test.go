package argo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These run the move-applications.sh that apply writes against stand-ins for
// kubectl, cub and sleep. The cub stand-in records which Space each
// Application was made to read, and the kubectl one reports it back, so what
// the script checks is what it made. Knobs: PARENT_READS (root's and
// storefront's source), APPSET_SYNC (applicationsSync), UID_AFTER (the UID an
// Application reports once moved), UNIT_EXISTS (a Space whose Unit is there
// already), RELEASE (the digest a moved Application reports synced).
const moveStubKubectl = `#!/usr/bin/env bash
echo "kubectl $*" >> "$STUB_LOG"
a=" $* "
app=$(printf '%s\n' "$@" | grep -A1 '^application$' | tail -1)
case "$a" in
  *" get applicationset "*"spec.generators"*) none='[{"list":{"elements":[]}}]'; echo "${GENERATORS:-$none}" ;;
  *" get applicationset "*) echo "${APPSET_SYNC:-create-only}" ;;
  *" get application ${MISSING_APP:-none} ") exit 1 ;;
  *"metadata.uid}{"*) echo "u-1|https://github.com/confighub/examples|gitops/argo/x|main" ;;
  *"{.metadata.uid}"*) echo "${UID_AFTER:-u-1}" ;;
  *"spec.source.repoURL"*)
    case "$app" in
      root|storefront) echo "${PARENT_READS:-oci://gw.example:5000/space/argo-x-children}" ;;
      *) sp=$(grep "^$app " "$STUB_LOG.map" 2>/dev/null | cut -d' ' -f2); echo "oci://gw.example:5000/space/${sp:-none}" ;;
    esac ;;
  *"status.sync.revision"*)
    case "$app" in root|storefront) echo sha256:x ;; *) echo "${RELEASE:-sha256:x}" ;; esac ;;
  *"status.sync.status"*) echo "Synced/Healthy" ;;
esac
exit 0
`

const moveStubCub = `#!/usr/bin/env bash
echo "cub $*" >> "$STUB_LOG"
a=" $* "
case "$a" in
  *" unit get "*)
    u=$(printf '%s\n' "$@" | tail -1)
    case "$a" in *" ${UNIT_EXISTS:-none} "*) exit 0 ;; esac
    grep -qx "$u" "$STUB_LOG.units" 2>/dev/null && exit 0; exit 1 ;;
  *" unit create "*) printf '%s\n' "$@" | grep -A1 '^--space$' >/dev/null; printf '%s\n' "$@" | sed -n '5p' >> "$STUB_LOG.units" ;;
  *" argo application-unit "*)
    app=$(printf '%s\n' "$@" | grep -A1 '^--application$' | tail -1)
    sp=$(printf '%s\n' "$@" | grep -A1 '^--space$' | tail -1)
    echo "$app $sp" >> "$STUB_LOG.map"
    echo "kind: Application" ;;
  *" release get "*) case "$a" in *" ${UNRELEASED:-none} "*) exit 1 ;; esac; echo '"sha256:x"' ;;
esac
exit 0
`

type moveRun struct {
	out, log, dir string
	err           error
}

func runMove(t *testing.T, args []string, env ...string) moveRun {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	dir := t.TempDir()
	p := planOf(t, example, Options{Prefix: "argo", StageLabel: "rollout-phase", Stages: []string{"canary", "secondary", "primary"}, RepoRoot: repoRoot(t)})
	if _, err := WriteApply(p, "argo", dir); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	for name, body := range map[string]string{
		"kubectl": moveStubKubectl, "cub": moveStubCub,
		"sleep": "#!/usr/bin/env bash\nexit 0\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	calls := filepath.Join(t.TempDir(), "calls.log")
	for _, e := range env {
		// A Unit there already was made by an earlier run, reading its Space.
		if sp, ok := strings.CutPrefix(e, "UNIT_EXISTS="); ok && sp == "argo-apptique-dev-1" {
			if err := os.WriteFile(calls+".map", []byte("dev-1-apptique "+sp+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	cmd := exec.Command("bash", append([]string{filepath.Join(dir, "move-applications.sh")}, args...)...)
	cmd.Stdin = strings.NewReader("")
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"STUB_LOG="+calls, "CONFIGHUB_OCI=gw.example:5000", "ARGOCD_CONTEXT=ctx-a")
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	log, _ := os.ReadFile(calls)
	return moveRun{string(out), string(log), dir, err}
}

// One stage: each canary Application becomes a Unit, named after its Space, in
// the control Space that holds its ApplicationSet; that Space is published,
// and each Application is checked to read its Space at its release, the same
// object it was.
func TestMoveDeliversOneStage(t *testing.T) {
	r := runMove(t, []string{"canary"})
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	for _, want := range []string{
		"cub unit create --space argo-storefront-children argo-apptique-dev-1 render/app-argo-apptique-dev-1.yaml --target argo-targets/argocd",
		"cub unit create --space argo-storefront-children argo-checkout-cache-dev-1 ",
		"cub unit create --space argo-root-children argo-platform-addons-cluster-baseline-dev-1 ",
		"cub release publish argo-storefront-children",
		"cub release publish argo-root-children",
	} {
		if !strings.Contains(r.log, want) {
			t.Errorf("want %q in:\n%s", want, r.log)
		}
	}
	if strings.Contains(r.log, "staging-1") || strings.Contains(r.log, "prod-1") {
		t.Errorf("only the canary stage should move:\n%s", r.log)
	}
	if !strings.Contains(r.out, "dev-1-apptique reads argo-apptique-dev-1 at sha256:x (Synced/Healthy, UID unchanged)") {
		t.Errorf("should confirm each arrived:\n%s", r.out)
	}
	if !strings.Contains(r.out, "cub unit delete --space argo-storefront-children argo-apptique-dev-1 && cub release publish argo-storefront-children") ||
		!strings.Contains(r.out, `patch application dev-1-apptique --type merge -p '{"spec":{"source":{"repoURL":"https://github.com/confighub/examples"`) {
		t.Errorf("should end with the way back, Unit first:\n%s", r.out)
	}
	if strings.Contains(r.log, "patch application") {
		t.Errorf("nothing is patched on the cluster; the parent applies the Unit:\n%s", r.log)
	}
}

func TestMoveWaitsForTheHandover(t *testing.T) {
	r := runMove(t, nil, "PARENT_READS=https://github.com/confighub/examples")
	if r.err == nil || !strings.Contains(r.out, "still reads Git: run handover.sh first") {
		t.Fatalf("a parent still reading Git must stop it: %v\n%s", r.err, r.out)
	}
	if strings.Contains(r.log, "unit create") {
		t.Errorf("nothing may be made before the check passes:\n%s", r.log)
	}
}

func TestMoveWaitsForTheRetirement(t *testing.T) {
	r := runMove(t, nil, "APPSET_SYNC=sync")
	if r.err == nil || !strings.Contains(r.out, "is not retired yet") {
		t.Fatalf("a live generator reverts the move, so it must stop: %v\n%s", r.err, r.out)
	}
}

// A recreated Application is not a moved one: its workloads were deleted with
// it, or will be. The run stops and prints the way back.
func TestMoveStopsOnANewObject(t *testing.T) {
	r := runMove(t, []string{"canary"}, "UID_AFTER=u-2")
	if r.err == nil || !strings.Contains(r.out, "is a new object (UID u-2, was u-1)") {
		t.Fatalf("want a stop naming both UIDs: %v\n%s", r.err, r.out)
	}
	if !strings.Contains(r.out, "STOPPED") || !strings.Contains(r.out, "cub unit delete --space argo-storefront-children argo-apptique-dev-1") {
		t.Errorf("should print the way back:\n%s", r.out)
	}
}

func TestMoveStopsOnAnUncheckedRelease(t *testing.T) {
	r := runMove(t, []string{"canary"}, "RELEASE=sha256:other")
	if r.err == nil || !strings.Contains(r.out, "synced sha256:other, not argo-apptique-dev-1's newest release sha256:x") {
		t.Fatalf("want a stop naming both digests: %v\n%s", r.err, r.out)
	}
}

// A re-run leaves a Unit that is there already as it is.
func TestMoveCarriesOn(t *testing.T) {
	r := runMove(t, []string{"canary"}, "UNIT_EXISTS=argo-apptique-dev-1")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	if strings.Contains(r.log, "unit create --space argo-storefront-children argo-apptique-dev-1 ") ||
		!strings.Contains(r.out, "dev-1-apptique is a Unit in argo-storefront-children already") {
		t.Errorf("an existing Unit should be left as it is:\n%s", r.out)
	}
}

// Measured on Argo CD v3.5.3: a create-only ApplicationSet that still
// generates takes an Application back to its template once the move leaves it
// standing alone, so it must generate nothing first.
func TestMoveWaitsForTheGeneratorToStop(t *testing.T) {
	r := runMove(t, nil, `GENERATORS=[{"clusters":{}}]`)
	if r.err == nil || !strings.Contains(r.out, "still generates. Set its generators to [{list: {elements: []}}]") {
		t.Fatalf("a generating ApplicationSet must stop it: %v\n%s", r.err, r.out)
	}
}

// A cluster that joined after the retirement has no Application on the
// cluster: its Unit is made like a sibling's, with its own name, destination
// and stage label, and the way back deletes it rather than restoring a source.
func TestMoveMakesAJoinedClustersApplication(t *testing.T) {
	r := runMove(t, []string{"canary", "secondary"}, "MISSING_APP=staging-1-apptique")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	want := "cub argo application-unit --like argo-storefront-children/argo-apptique-dev-1 --application staging-1-apptique --space argo-apptique-staging-1 --gateway gw.example:5000 --destination-server "
	if !strings.Contains(r.log, want) || !strings.Contains(r.log, "--destination-namespace storefront-staging --label rollout-phase=secondary") {
		t.Errorf("want the joined cluster's Unit made like its sibling's:\n%s", r.log)
	}
	if !strings.Contains(r.out, "delete application staging-1-apptique") {
		t.Errorf("its way back is to delete it, as there is no source to restore:\n%s", r.out)
	}
}

// Found live: a joined cluster's Space had no release yet, and its
// Application was made anyway, reading nothing. It waits instead.
func TestMoveWaitsForTheRelease(t *testing.T) {
	r := runMove(t, []string{"canary"}, "UNRELEASED=argo-apptique-dev-1")
	if r.err == nil || !strings.Contains(r.out, "argo-apptique-dev-1 has no release yet") {
		t.Fatalf("a Space with no release must stop it: %v\n%s", r.err, r.out)
	}
	if strings.Contains(r.log, "unit create --space argo-storefront-children argo-apptique-dev-1 ") {
		t.Errorf("no Unit may be made for a Space with no release:\n%s", r.log)
	}
}

func TestMoveRefusesAnUnknownStage(t *testing.T) {
	r := runMove(t, []string{"canry"})
	if r.err == nil || !strings.Contains(r.out, "no stage canry; the stages are: canary secondary primary") {
		t.Fatalf("a mistyped stage must not move anything: %v\n%s", r.err, r.out)
	}
}
