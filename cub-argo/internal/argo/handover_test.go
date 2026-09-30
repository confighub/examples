package argo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These run the handover.sh that apply writes against stand-ins for kubectl,
// cub, kustomize and sleep, so what the script does when a step fails is
// tested rather than read. ROOT_READS says what root's sync status reads after
// its repoint (default Synced); a status of Unknown is a source it cannot read.
const argoStubKubectl = `#!/usr/bin/env bash
echo "$*" >> "$STUB_LOG"
a=" $* "
case "$a" in
  *" config current-context "*) echo ctx-current ;;
  *" get deployment argocd-repo-server "*) echo "quay.io/argoproj/argocd:v3.5.3" ;;
  *" get appproject "*) echo '["https://github.com/confighub/examples","oci://gw.example:5000/**"]' ;;
  *" get application root "*"metadata.name"*) echo "root|https://github.com/confighub/examples|gitops/argo/expert-app-of-apps/bootstrap/children|main" ;;
  *" get application root "*"spec.source.repoURL"*) echo "https://github.com/confighub/examples" ;;
  *" get application root "*"status.sync.status"*) echo "${ROOT_READS:-Synced}" ;;
  *" get application root "*"status.sync.revision"*) echo "${ROOT_SYNCED:-sha256:root}" ;;
  *" get application root "*"conditions"*) echo "repository not accessible" ;;
  *" apply -f - "*) cat >/dev/null ;;
esac
exit 0
`

const argoStubCub = `#!/usr/bin/env bash
case " $* " in
  *" release get "*) echo '"sha256:root"' ;;
  *" unit data "*) echo "kind: Same" ;;
  *" worker get "*) echo 'id' ;;
  *" argo check "*) exit 0 ;;
esac
exit 0
`

type argoRun struct {
	out, log, dir string
	err           error
}

func runArgoHandover(t *testing.T, env ...string) argoRun {
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
		"kubectl": argoStubKubectl, "cub": argoStubCub,
		"kustomize": "#!/usr/bin/env bash\ncase \"$1\" in build) echo 'kind: Same' ;; *) echo v5 ;; esac\n",
		"sleep":     "#!/usr/bin/env bash\nexit 0\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	calls := filepath.Join(t.TempDir(), "kubectl.log")
	cmd := exec.Command("bash", filepath.Join(dir, "handover.sh"))
	cmd.Stdin = strings.NewReader("")
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"STUB_LOG="+calls, "CONFIGHUB_OCI=gw.example:5000", "REPO_ROOT="+repoRoot(t),
		"DEST_CONTEXT_dev_1=ctx-dev", "DEST_CONTEXT_staging_1=ctx-staging", "DEST_CONTEXT_prod_1=ctx-prod")
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	log, _ := os.ReadFile(calls)
	return argoRun{string(out), string(log), dir, err}
}

// root cannot read its new source: the run stops, and prints the way back with
// root's source as it was, on the context the run used, leaves first.
func TestArgoHandoverStopsWithTheWayBack(t *testing.T) {
	r := runArgoHandover(t, "ARGOCD_CONTEXT=ctx-a", "ROOT_READS=Unknown")
	if r.err == nil {
		t.Fatalf("a root that cannot read its new source must stop the run:\n%s", r.out)
	}
	if !strings.Contains(r.out, "HANDOVER STOPPED") || !strings.Contains(r.out, "These parents read ConfigHub: root") {
		t.Errorf("should say it stopped with root moved:\n%s", r.out)
	}
	back := `kubectl --context 'ctx-a' -n 'argocd' patch application root --type merge -p '{"spec":{"source":{"repoURL":"https://github.com/confighub/examples","path":"gitops/argo/expert-app-of-apps/bootstrap/children","targetRevision":"main"}}}'`
	if !strings.Contains(r.out, back) {
		t.Errorf("want root's recorded source back, on ctx-a:\n%s", r.out)
	}
	leaves := strings.Index(r.out, "1. Any ApplicationSet you retired")
	if leaves < 0 || leaves > strings.Index(r.out, back) {
		t.Errorf("the way back starts from the leaves:\n%s", r.out)
	}
	state, err := os.ReadFile(filepath.Join(r.dir, "handover-state", "argo.txt"))
	if err != nil || !strings.HasPrefix(string(state), "root|https://github.com/confighub/examples|") {
		t.Errorf("root's original should be recorded: %v %q", err, state)
	}
}

// A root that synced something other than the control Space's newest release
// stops the run: a release went out unchecked.
func TestArgoHandoverStopsOnAnUncheckedRootRelease(t *testing.T) {
	r := runArgoHandover(t, "ARGOCD_CONTEXT=ctx-a", "ROOT_SYNCED=sha256:other")
	if r.err == nil || !strings.Contains(r.out, "root synced sha256:other, but argo-root-children's newest release is sha256:root") {
		t.Fatalf("want a stop naming both digests: %v\n%s", r.err, r.out)
	}
	if !strings.Contains(r.out, "HANDOVER STOPPED") {
		t.Errorf("root had moved, so the way back should follow:\n%s", r.out)
	}
}

// A clean run ends with the way back, built from what was recorded.
func TestArgoHandoverEndsWithTheWayBack(t *testing.T) {
	r := runArgoHandover(t, "ARGOCD_CONTEXT=ctx-a")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	if !strings.Contains(r.out, "The way back, leaves first:") || !strings.Contains(r.out, `patch application root --type merge -p '{"spec":{"source":{"repoURL":"https://github.com/confighub/examples"`) {
		t.Errorf("want the way back at the end:\n%s", r.out)
	}
	if strings.Contains(r.out, "HANDOVER STOPPED") {
		t.Errorf("a clean run did not stop:\n%s", r.out)
	}
}
