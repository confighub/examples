package flux

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A pull Secret whose token ConfigHub lists as live keeps it: no new token,
// and only this cluster's other tokens are revoked.
func TestHandoverKeepsALivePullToken(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "HELD_JTI=j-held", "LIVE_JTIS=j-held j-old")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	if strings.Contains(r.cub, "token create") {
		t.Errorf("a live token is kept, not replaced:\n%s", r.cub)
	}
	if !strings.Contains(r.log, "confighub.com/token-jti=j-held") {
		t.Errorf("the Secret records the token it holds:\n%s", r.log)
	}
	if !strings.Contains(r.cub, "token revoke flux-delivery j-old --quiet") || strings.Contains(r.cub, "revoke flux-delivery j-held") || strings.Contains(r.cub, "j-other") {
		t.Errorf("only this cluster's other tokens are revoked:\n%s", r.cub)
	}
	checkPullSecret(t, r, "s3cr3t-held")
}

// A Secret from an earlier version records no token: a new one is issued, and
// the tokens earlier runs issued for this cluster are revoked.
func TestHandoverReplacesAPullTokenItCannotFind(t *testing.T) {
	r := runHandover(t, "FLUX_CONTEXT=ctx-a", "LIVE_JTIS=j-one j-two")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	if !strings.Contains(r.cub, "token create flux-delivery --description Flux, cluster dev-1") {
		t.Errorf("a new token is issued:\n%s", r.cub)
	}
	if !strings.Contains(r.log, "confighub.com/token-jti=jti-new") {
		t.Errorf("the Secret records the new token:\n%s", r.log)
	}
	for _, old := range []string{"j-one", "j-two"} {
		if !strings.Contains(r.cub, "token revoke flux-delivery "+old+" --quiet") {
			t.Errorf("%s is revoked:\n%s", old, r.cub)
		}
	}
	if strings.Contains(r.cub, "revoke flux-delivery jti-new") || strings.Contains(r.cub, "j-other") {
		t.Errorf("neither the new token nor another cluster's is revoked:\n%s", r.cub)
	}
	checkPullSecret(t, r, "s3cr3t-new")
}

// checkPullSecret: the Secret is the dockerconfigjson an OCIRepository reads,
// with the username token, written with create or replace, and the token is on
// no command line.
func checkPullSecret(t *testing.T, r handoverRun, token string) {
	t.Helper()
	b := r.pull
	if b == "" {
		t.Fatalf("no dockerconfigjson was handed to kubectl:\n%s", r.log)
	}
	// auth is base64 of token:<token>, as kubectl create secret docker-registry
	// writes it.
	auth := base64.StdEncoding.EncodeToString([]byte("token:" + token))
	if want := `{"auths":{"gw.example:5000":{"username":"token","password":"` + token + `","auth":"` + auth + `"}}}`; b != want {
		t.Errorf("dockerconfigjson:\n%s\nwant\n%s", b, want)
	}
	if !strings.Contains(r.log, "create secret generic confighub-flux-targets --type=kubernetes.io/dockerconfigjson") {
		t.Errorf("the pull Secret is a dockerconfigjson Secret:\n%s", r.log)
	}
	for _, l := range strings.Split(r.log+r.cub, "\n") {
		if strings.Contains(l, token) {
			t.Errorf("the token is on a command line: %s", l)
		}
		if strings.Contains(l, "create secret") && strings.Contains(l, "apply -f -") {
			t.Errorf("a Secret is applied: %s", l)
		}
	}
	if !strings.Contains(r.log, "replace -f -") && !strings.Contains(r.log, "create -f -") {
		t.Errorf("no Secret is written with create or replace:\n%s", r.log)
	}
}

// A cluster an earlier version handed over is not handed over again, but its
// pull Secret is brought up to date: running handover.sh again is how 0.5.0's
// applied, never-revoked token is replaced. With no CONFIGHUB_OCI it keeps
// the gateway the Secret names.
func TestHandoverRefreshesAHandedOverClustersPullSecret(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	const fleet = "gitops/flux/expert-fleet"
	p := planAt(t, handedOver(t, fleet, "dev", "flux-dev-1-layers"), fleet)
	dir, bin := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "handover.sh"), []byte(HandoverScript(p, "flux", ".")), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"kubectl": stubKubectl, "cub": stubCub} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	calls := filepath.Join(t.TempDir(), "kubectl.log")
	cmd := exec.Command("bash", filepath.Join(dir, "handover.sh"))
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"STUB_LOG="+calls, "CLUSTER=dev-1", "FLUX_CONTEXT=ctx-a", "LIVE_JTIS=j-one")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	log, _ := os.ReadFile(calls)
	cub, _ := os.ReadFile(calls + ".cub")
	pull, _ := os.ReadFile(calls + ".dockerconfig")
	if !strings.Contains(string(out), "Only its pull Secret is brought up to date") {
		t.Errorf("it says what it does:\n%s", out)
	}
	if !strings.Contains(string(cub), "token create flux-delivery --description Flux, cluster dev-1") || !strings.Contains(string(cub), "token revoke flux-delivery j-one --quiet") {
		t.Errorf("a token is issued and the cluster's earlier one revoked:\n%s", cub)
	}
	if !strings.HasPrefix(string(pull), `{"auths":{"gw.example:5000":`) {
		t.Errorf("the gateway the Secret named is kept: %s", pull)
	}
	if strings.Contains(string(log), "patch kustomization") || strings.Contains(string(log), "apply -f bootstrap/") {
		t.Errorf("nothing is handed over again:\n%s", log)
	}
}
