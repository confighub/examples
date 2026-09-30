package flux

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cleanup.sh run against stand-ins for kubectl and cub. OCI is what the
// OCIRepository list returns, one "<ns>/<name> <url>" per line.
func runFluxCleanup(t *testing.T, oci string, env ...string) (out, log string, err error) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	dir := t.TempDir()
	p := planOf(t, "../../../gitops/flux/beginner", repoRoot(t))
	if _, err := WriteApply(p, "flux", dir); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	calls := filepath.Join(t.TempDir(), "calls.log")
	for name, body := range map[string]string{
		"kubectl": "#!/usr/bin/env bash\necho \"kubectl $*\" >> \"$STUB_LOG\"\ncase \" $* \" in *\" get ocirepositories \"*) printf '%b' \"$OCI\" ;; esac\nexit 0\n",
		"cub":     "#!/usr/bin/env bash\necho \"cub $*\" >> \"$STUB_LOG\"\ncase \" $* \" in *\" space get \"*) exit 1 ;; esac\nexit 0\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", filepath.Join(dir, "cleanup.sh"))
	cmd.Stdin = strings.NewReader("")
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "STUB_LOG="+calls, "OCI="+oci)
	cmd.Env = append(cmd.Env, env...)
	b, err := cmd.CombinedOutput()
	l, _ := os.ReadFile(calls)
	return string(b), string(l), err
}

// With the clusters to read, cleanup.sh checks each one, and a layer still
// reading one of these Spaces stops it before any delete.
func TestFluxCleanupRefusesWhileALayerReadsASpace(t *testing.T) {
	out, log, err := runFluxCleanup(t, `flux-system/infrastructure oci://gw/space/flux-infrastructure-dev\nflux-system/podinfo oci://ghcr.io/stefanprodan/manifests/podinfo\n`, "FLUX_CONTEXTS=ctx-dev ctx-prod")
	if err == nil {
		t.Fatalf("want a refusal:\n%s", out)
	}
	if !strings.Contains(out, "On ctx-dev, these still read a Space this would delete: flux-system/infrastructure") {
		t.Errorf("want the OCIRepository named:\n%s", out)
	}
	if strings.Contains(out, "podinfo") {
		t.Errorf("an OCIRepository reading something else is not ConfigHub's:\n%s", out)
	}
	if strings.Contains(log, "space delete") {
		t.Errorf("nothing may be deleted:\n%s", log)
	}
}

// Once nothing reads them, it deletes the Spaces and, on each cluster, the
// pull Secret that holds the deleted worker.
func TestFluxCleanupDeletesSpacesAndTheCredential(t *testing.T) {
	out, log, err := runFluxCleanup(t, `flux-system/fleet oci://ghcr.io/acme/fleet\n`, "FLUX_CONTEXTS=ctx-dev ctx-prod")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, want := range []string{
		"kubectl --context ctx-dev get ocirepositories -A",
		"kubectl --context ctx-prod get ocirepositories -A",
		"cub space delete flux-targets --recursive --detach",
		"kubectl --context ctx-dev -n flux-system delete secret confighub-flux-targets --ignore-not-found",
		"kubectl --context ctx-prod -n flux-system delete secret confighub-flux-targets --ignore-not-found",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("want %q in:\n%s", want, log)
		}
	}
}

// Without the clusters, it asks rather than guesses.
func TestFluxCleanupAsksWithoutTheClusters(t *testing.T) {
	out, log, err := runFluxCleanup(t, "")
	if err == nil || !strings.Contains(out, "Stopping. Nothing was deleted.") {
		t.Fatalf("want it to ask and stop: %v\n%s", err, out)
	}
	if strings.Contains(log, "space delete") {
		t.Errorf("nothing may be deleted:\n%s", log)
	}
}

// No kubectl template in a generated script spans lines: a newline inside one
// is an unterminated string to kubectl, which the stubs do not parse.
func TestFluxScriptsKeepKubectlTemplatesOnOneLine(t *testing.T) {
	dir := t.TempDir()
	p := planOf(t, "../../../gitops/flux/beginner", repoRoot(t))
	if _, err := WriteApply(p, "flux", dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"apply.sh", "handover.sh", "cleanup.sh", "join.sh"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, open := range []string{"jsonpath='", "go-template='"} {
				if j := strings.Index(line, open); j >= 0 && !strings.Contains(line[j+len(open):], "'") {
					t.Errorf("%s:%d: not closed on its line: %s", name, i+1, line)
				}
			}
		}
	}
}
