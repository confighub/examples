package argo

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The jq that live_tokens hands to cub keeps a token that has neither been
// revoked nor expired. cub evaluates it; jq, where it is installed, evaluates
// it the same way.
func TestLiveTokensFilterDropsRevokedAndExpiredTokens(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("no jq")
	}
	expr := regexp.MustCompile(`token list "\$1" -o jq='([^']+)'`).FindStringSubmatch(strings.Join(credentialHelpers(), "\n"))
	if expr == nil {
		t.Fatal("live_tokens hands cub no jq expression")
	}
	tokens := `[
	  {"UserToken": {"Jti": "live", "Description": "Argo CD, argo estate", "RevokedAt": null, "ExpiresAt": null}},
	  {"UserToken": {"Jti": "revoked", "Description": "Argo CD, argo estate", "RevokedAt": "2026-10-26T10:00:00Z", "ExpiresAt": null}},
	  {"UserToken": {"Jti": "expired", "Description": "Argo CD, argo estate", "RevokedAt": null, "ExpiresAt": "2020-01-01T00:00:00.123Z"}},
	  {"UserToken": {"Jti": "later", "Description": "Argo CD, argo estate", "RevokedAt": null, "ExpiresAt": "2999-01-01T00:00:00.5Z"}}
	]`
	cmd := exec.Command("jq", "-r", expr[1])
	cmd.Stdin = strings.NewReader(tokens)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got := string(out); got != "live Argo CD, argo estate\nlater Argo CD, argo estate\n" {
		t.Errorf("live tokens:\n%s", got)
	}
}

// A repo-creds Secret whose token ConfigHub lists as live keeps it: no new
// token, and only the estate's other tokens are revoked.
func TestArgoHandoverKeepsALiveToken(t *testing.T) {
	r := runArgoHandover(t, "ARGOCD_CONTEXT=ctx-a", "HELD_JTI=j-held", "LIVE_JTIS=j-held j-old")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	if strings.Contains(r.log, "token create") {
		t.Errorf("a live token is kept, not replaced:\n%s", r.log)
	}
	if !strings.Contains(r.log, "confighub.com/token-jti=j-held") {
		t.Errorf("the Secret records the token it holds:\n%s", r.log)
	}
	if !strings.Contains(r.log, "token revoke argo-delivery j-old --quiet") || strings.Contains(r.log, "revoke argo-delivery j-held") || strings.Contains(r.log, "j-other") {
		t.Errorf("only the estate's other tokens are revoked:\n%s", r.log)
	}
	checkSecretsWithoutApply(t, r.log, "s3cr3t")
}

// A Secret from an earlier version records no token: a new one is issued, and
// the tokens earlier runs issued for the estate are revoked.
func TestArgoHandoverReplacesATokenItCannotFind(t *testing.T) {
	r := runArgoHandover(t, "ARGOCD_CONTEXT=ctx-a", "LIVE_JTIS=j-one j-two")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.out)
	}
	if !strings.Contains(r.log, "token create argo-delivery --description Argo CD, argo estate") {
		t.Errorf("a new token is issued:\n%s", r.log)
	}
	if !strings.Contains(r.log, "confighub.com/token-jti=jti-new") {
		t.Errorf("the Secret records the new token:\n%s", r.log)
	}
	for _, old := range []string{"j-one", "j-two"} {
		if !strings.Contains(r.log, "token revoke argo-delivery "+old+" --quiet") {
			t.Errorf("%s is revoked:\n%s", old, r.log)
		}
	}
	if strings.Contains(r.log, "revoke argo-delivery jti-new") {
		t.Errorf("the new token is not revoked:\n%s", r.log)
	}
	checkSecretsWithoutApply(t, r.log, "s3cr3t")
}

// checkSecretsWithoutApply: no Secret goes through kubectl apply, and the
// token is on no command line.
func checkSecretsWithoutApply(t *testing.T, log, token string) {
	t.Helper()
	for _, l := range strings.Split(log, "\n") {
		if strings.Contains(l, "apply -f -") && strings.Contains(strings.ToLower(l), "secret") {
			t.Errorf("a Secret is applied: %s", l)
		}
		if strings.Contains(l, token) {
			t.Errorf("the token is on a command line: %s", l)
		}
	}
	if !strings.Contains(log, "replace -f -") && !strings.Contains(log, "create -f -") {
		t.Errorf("no Secret is written with create or replace:\n%s", log)
	}
}

const argobotStubKubectl = `#!/usr/bin/env bash
echo "kubectl $*" >> "$STUB_LOG"
case " $* " in
  *" get secret argobot-secrets "*"key-id"*) echo "${HELD_KID:-}" ;;
  *" get secret argobot-secrets "*"CONFIGHUB_AUTH_PRIVATE_KEY"*) printf '{"d":"old-private"}' | base64 ;;
  *" --local "*) cat ;;
  *" replace -f - "*|*" create -f - "*) cat > "$STUB_LOG.secret" ;;
  *" create secret generic "*)
    for a in "$@"; do case "$a" in --from-file=*) echo "# key: $(cat "${a#--from-file=*=}")" >> "$STUB_LOG" ;; esac; done ;;
esac
exit 0
`

const argobotStubCub = `#!/usr/bin/env bash
echo "cub $*" >> "$STUB_LOG"
case " $* " in
  *" target list "*) echo dev-1 ;;
  *" key list "*)
    for k in ${KEYS:-}; do echo "$k argobot on ctx-a"; done
    echo "k-other fluxbot on dev"
    [ ! -f "$STUB_LOG.added" ] || echo "k-new argobot on ctx-a" ;;
  *" key add "*)
    set -- $*; while [ "$1" != --generate ]; do shift; done
    printf '{"d":"new-private"}' > "$2"; touch "$STUB_LOG.added" ;;
esac
exit 0
`

func runArgobot(t *testing.T, env ...string) (out, log string) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "argobot.sh")
	if err := os.WriteFile(script, []byte(ArgobotScript("argo")), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	for name, body := range map[string]string{"kubectl": argobotStubKubectl, "cub": argobotStubCub} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	calls := filepath.Join(t.TempDir(), "calls.log")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"STUB_LOG="+calls, "ARGOCD_CONTEXT=ctx-a", "CONFIGHUB_URL=https://hub.example")
	cmd.Env = append(cmd.Env, env...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	l, _ := os.ReadFile(calls)
	return string(b), string(l)
}

// A key ConfigHub still lists is kept: the Secret is rewritten with it, which
// drops a copy an earlier kubectl apply left, and nothing restarts.
func TestArgobotKeepsARegisteredKey(t *testing.T) {
	_, log := runArgobot(t, "HELD_KID=k-held", "KEYS=k-held k-old")
	if strings.Contains(log, "key add") || strings.Contains(log, "rollout restart") {
		t.Errorf("a registered key is kept:\n%s", log)
	}
	if !strings.Contains(log, `# key: {"d":"old-private"}`) || !strings.Contains(log, "confighub.com/key-id=k-held") {
		t.Errorf("the Secret is written with the key it held:\n%s", log)
	}
	if !strings.Contains(log, "key delete argo-delivery k-old") || strings.Contains(log, "delete argo-delivery k-held") || strings.Contains(log, "k-other") {
		t.Errorf("only argobot's other keys are removed:\n%s", log)
	}
	checkKeyOffCommandLines(t, log)
}

// A key ConfigHub no longer lists, after the account was deleted and made
// again say, or a Secret from an earlier version that records none, is
// replaced, and argobot restarts to read it.
func TestArgobotReplacesAKeyConfigHubDoesNotList(t *testing.T) {
	for _, held := range []string{"HELD_KID=k-gone", "HELD_KID="} {
		_, log := runArgobot(t, held, "KEYS=k-old")
		if !strings.Contains(log, `key add argo-delivery --generate`) || !strings.Contains(log, `# key: {"d":"new-private"}`) {
			t.Errorf("%s: a new key goes in the Secret:\n%s", held, log)
		}
		if !strings.Contains(log, "confighub.com/key-id=k-new") || !strings.Contains(log, "key delete argo-delivery k-old") || strings.Contains(log, "delete argo-delivery k-new") || strings.Contains(log, "k-other") {
			t.Errorf("%s: the Secret records the new key, and only argobot's earlier keys are removed:\n%s", held, log)
		}
		if !strings.Contains(log, "rollout restart deployment/argobot") {
			t.Errorf("%s: argobot restarts to read the new key:\n%s", held, log)
		}
		if strings.Contains(log, "apply -f - ") {
			t.Errorf("%s: no Secret is applied:\n%s", held, log)
		}
		checkKeyOffCommandLines(t, log)
	}
}

// checkKeyOffCommandLines: the private half reaches the Secret through a file
// descriptor, on no kubectl or cub command line.
func checkKeyOffCommandLines(t *testing.T, log string) {
	t.Helper()
	for _, l := range strings.Split(log, "\n") {
		if (strings.HasPrefix(l, "kubectl ") || strings.HasPrefix(l, "cub ")) && strings.Contains(l, "private") {
			t.Errorf("the key is on a command line: %s", l)
		}
	}
}
