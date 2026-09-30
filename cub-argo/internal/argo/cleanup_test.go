package argo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cleanup.sh run against stand-ins for kubectl and cub. APPS is what the
// Application list returns, one "<name> <repoURL>" per line.
func runArgoCleanup(t *testing.T, apps string, env ...string) (out, log string, err error) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	dir := t.TempDir()
	p := planOf(t, "../../../gitops/argo/beginner-app-of-apps", Options{Prefix: "argo", RepoRoot: repoRoot(t)})
	if _, err := WriteApply(p, "argo", dir); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	calls := filepath.Join(t.TempDir(), "calls.log")
	stubs := map[string]string{
		"kubectl": `#!/usr/bin/env bash
echo "kubectl $*" >> "$STUB_LOG"
case " $* " in
  *" get applications "*) printf '%b' "$APPS" ;;
esac
exit 0
`,
		"cub": `#!/usr/bin/env bash
echo "cub $*" >> "$STUB_LOG"
case " $* " in
  *" space get "*) exit 1 ;;
esac
exit 0
`,
	}
	for name, body := range stubs {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", filepath.Join(dir, "cleanup.sh"))
	cmd.Stdin = strings.NewReader("")
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "STUB_LOG="+calls, "APPS="+apps)
	cmd.Env = append(cmd.Env, env...)
	b, err := cmd.CombinedOutput()
	l, _ := os.ReadFile(calls)
	return string(b), string(l), err
}

// With the Argo cluster to read, cleanup.sh checks for itself, and an
// Application still reading one of these Spaces stops it before any delete.
// Measured on the rig: the check once could not parse its own jsonpath; it
// failed closed, and nothing tested it.
func TestArgoCleanupRefusesWhileAnApplicationReadsASpace(t *testing.T) {
	out, log, err := runArgoCleanup(t, `apptique-apps oci://gw.example/space/argo-apptique-apps-children\napptique-dev https://github.com/confighub/examples.git\n`, "ARGOCD_CONTEXT=ctx-a")
	if err == nil {
		t.Fatalf("want a refusal:\n%s", out)
	}
	if !strings.Contains(out, "still read a Space this would delete: apptique-apps") {
		t.Errorf("want the Application named:\n%s", out)
	}
	if strings.Contains(log, "space delete") {
		t.Errorf("nothing may be deleted:\n%s", log)
	}
	if !strings.Contains(log, "kubectl --context ctx-a -n argocd get applications -o jsonpath=") || strings.Contains(out, "error parsing jsonpath") {
		t.Errorf("the check must read the Applications, on ctx-a:\n%s\n%s", log, out)
	}
}

// Once every source is back on Git, it deletes the Spaces and the credential
// handover.sh wrote, which would otherwise answer for these Spaces later.
func TestArgoCleanupDeletesSpacesAndTheCredential(t *testing.T) {
	out, log, err := runArgoCleanup(t, `apptique-apps https://github.com/confighub/examples.git\n`, "ARGOCD_CONTEXT=ctx-a")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, want := range []string{
		"cub space delete argo-apptique-apps-children --recursive --detach",
		"cub space delete argo-targets --recursive --detach",
		"kubectl --context ctx-a -n argocd delete secret confighub-argo-targets --ignore-not-found",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("want %q in:\n%s", want, log)
		}
	}
}

// Without the cluster to read, it asks rather than guesses.
func TestArgoCleanupAsksWithoutTheCluster(t *testing.T) {
	out, log, err := runArgoCleanup(t, "")
	if err == nil || !strings.Contains(out, "Stopping. Nothing was deleted.") {
		t.Fatalf("want it to ask and stop on no answer: %v\n%s", err, out)
	}
	if strings.Contains(log, "space delete") {
		t.Errorf("nothing may be deleted:\n%s", log)
	}
}

// Every jsonpath and template the scripts pass to kubectl sits on one line. A
// newline inside one is an unterminated string to kubectl, which the stubs
// above do not parse; it broke the check in cleanup.sh and in handover.sh.
func TestArgoScriptsKeepKubectlTemplatesOnOneLine(t *testing.T) {
	dir := t.TempDir()
	p := planOf(t, "../../../gitops/argo/beginner-app-of-apps", Options{Prefix: "argo", RepoRoot: repoRoot(t)})
	if _, err := WriteApply(p, "argo", dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"apply.sh", "handover.sh", "cleanup.sh"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, open := range []string{"jsonpath='", "go-template='"} {
				if j := strings.Index(line, open); j >= 0 && !strings.Contains(line[j+len(open):], "'") {
					t.Errorf("%s:%d: %s is not closed on its line: %s", name, i+1, strings.TrimSuffix(open, "='"), line)
				}
			}
		}
	}
}
