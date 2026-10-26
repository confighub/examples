package flux

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// fluxbot.sh run against stand-ins for kubectl and cub, which log each call
// and what kubectl was given on stdin.
func runFluxbot(t *testing.T, version string, env ...string) (out, log string, err error) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	dir, bin := t.TempDir(), t.TempDir()
	script := filepath.Join(dir, "fluxbot.sh")
	if err := os.WriteFile(script, []byte(FluxbotScript("flux", version)), 0o755); err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(t.TempDir(), "calls.log")
	for name, body := range map[string]string{
		"kubectl": `#!/usr/bin/env bash
echo "kubectl $*" >> "$STUB_LOG"
case " $* " in
  *" get secret "*"key-id"*) echo "${HELD_KID:-}" ;;
  *" get secret "*) [ -n "${KEY_IN_SECRET:-}" ] || [ -f "$STUB_LOG.secret" ] && echo a2V5 ;;
  *" --local "*) cat ;;
  *" replace -f - "*|*" create -f - "*) cat >> "$STUB_LOG" ;;
  *" create secret generic "*)
    for a in "$@"; do case "$a" in --from-file=*) echo "# the Secret, from: $(cat "${a#--from-file=*=}")" ;; esac; done
    touch "$STUB_LOG.secret" ;;
  *" apply -f - "*) cat >> "$STUB_LOG" ;;
  *" rollout status "*) [ -z "${ROLLOUT_FAILS:-}" ] || exit 1 ;;
esac
exit 0
`,
		"cub": `#!/usr/bin/env bash
echo "cub $*" >> "$STUB_LOG"
case " $* " in
  *" target get "*) case " $* " in *" nosuch "*) exit 1 ;; esac ;;
  *" key add "*) [ -z "${KEY_ADD_FAILS:-}" ] || exit 1; printf '{"d":"s3cret"}' > "${@: -3:1}"; touch "$STUB_LOG.added" ;;
  *" key list "*)
    for k in ${KEYS:-}; do echo "$k fluxbot on dev"; done
    echo "k-other argobot on ctx-a"
    [ ! -f "$STUB_LOG.added" ] || echo "k-new fluxbot on dev" ;;
esac
exit 0
`,
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "STUB_LOG="+calls)
	cmd.Env = append(cmd.Env, env...)
	b, err := cmd.CombinedOutput()
	l, _ := os.ReadFile(calls)
	return string(b), string(l), err
}

// The reporter is granted before it is installed, runs this release's binary
// checked against the release's checksum, and may read two kinds of object in
// one namespace.
func TestFluxbotInstallsTheReporter(t *testing.T) {
	out, log, err := runFluxbot(t, "0.4.0", "FLUX_CONTEXT=ctx-dev", "CLUSTER=dev", "CONFIGHUB_URL=https://hub.example")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	grant := strings.Index(log, `cub target update dev --space flux-targets --permission View:serviceaccount:flux-delivery --permission ViewChildren:serviceaccount:flux-delivery --permission EditChildren:serviceaccount:flux-delivery`)
	deploy := strings.Index(log, "kind: Deployment")
	if grant < 0 || deploy < 0 || grant > deploy {
		t.Errorf("the grant comes before the pod that needs it:\n%s", log)
	}
	for _, want := range []string{
		"kubectl --context ctx-dev apply -f -",
		"cub serviceaccount key add flux-delivery --generate ",
		"create secret generic fluxbot --from-file=CONFIGHUB_AUTH_PRIVATE_KEY=",
		`# the Secret, from: {"d":"s3cret"}`,
		`{name: FROM, value: "https://github.com/confighub/examples/releases/download/cub-flux-v0.4.0"}`,
		`command: ["/tools/cub-flux", "status", "--discover", "--as-service-account", "--watch", "--ready-file", "/tmp/ready", "--prefix", "flux", "--namespace", "flux-system"]`,
		`readinessProbe: {exec: {command: ["/tools/cub-flux", "status", "--ready", "/tmp/ready"]}`,
		`{name: CONFIGHUB_URL, value: "https://hub.example"}`,
		`resources: ["kustomizations"]`, `resources: ["ocirepositories"]`, `verbs: ["get", "list"]`,
		"image: " + fluxbotFetch, "image: " + fluxbotRuntime,
		`case "$(uname -m)" in`, `sha256sum cub-flux.new`, "mv -f cub-flux.new cub-flux",
		"maxUnavailable: 0", "flux.confighub.com/made-from:",
		"rollout status deployment/fluxbot",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("want %q in what was applied and run:\n%s", want, log)
		}
	}
	if strings.Contains(log, "kind: ClusterRole") || strings.Contains(log, `"*"`) {
		t.Errorf("the reporter reads one namespace, and no more:\n%s", log)
	}
	if strings.Contains(out, "s3cret") {
		t.Errorf("the private key must not be printed:\n%s", out)
	}
	if strings.Contains(log, "rollout restart") {
		t.Errorf("a working pod is not restarted: only a change to what it was made from replaces it:\n%s", log)
	}
	// What is applied is YAML a cluster would take, with the pod as intended.
	var deployment map[string]any
	dec := yaml.NewDecoder(strings.NewReader(log[strings.Index(log, "apiVersion: v1\nkind: ServiceAccount"):strings.Index(log, "kubectl --context ctx-dev -n flux-system rollout status")]))
	kinds := map[string]bool{}
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			if err != io.EOF {
				t.Fatalf("what fluxbot.sh applies is not YAML: %v", err)
			}
			break
		}
		kinds[str(doc["kind"])] = true
		if doc["kind"] == "Deployment" {
			deployment = doc
		}
	}
	if !kinds["ServiceAccount"] || !kinds["Role"] || !kinds["RoleBinding"] || deployment == nil {
		t.Fatalf("want a ServiceAccount, Role, RoleBinding and Deployment: %v", kinds)
	}
	pod := obj(get(deployment, "spec", "template", "spec"))
	if get(pod, "securityContext", "runAsNonRoot") != true || str(pod["serviceAccountName"]) != "fluxbot" {
		t.Errorf("the pod runs as its own account, and not as root: %v", pod)
	}
	for _, c := range append(list(pod["initContainers"]), list(pod["containers"])...) {
		if get(c, "securityContext", "readOnlyRootFilesystem") != true || get(c, "securityContext", "allowPrivilegeEscalation") != false {
			t.Errorf("every container is locked down: %v", c)
		}
	}
	if !strings.Contains(out, "delete deployment,serviceaccount,role,rolebinding,secret fluxbot") || !strings.Contains(out, "--permission -EditChildren:serviceaccount:flux-delivery") {
		t.Errorf("say how to take it out again, the grant too:\n%s", out)
	}
}

// It asks for what it cannot work out, and changes nothing until it has it.
func TestFluxbotRefusesWithoutWhatItNeeds(t *testing.T) {
	for name, tc := range map[string]struct {
		version string
		env     []string
		says    string
	}{
		"no cluster":   {"0.4.0", []string{"FLUX_CONTEXT=c", "CONFIGHUB_URL=https://hub.example"}, "set CLUSTER"},
		"no address":   {"0.4.0", []string{"FLUX_CONTEXT=c", "CLUSTER=dev"}, "set CONFIGHUB_URL"},
		"not a Target": {"0.4.0", []string{"FLUX_CONTEXT=c", "CLUSTER=nosuch", "CONFIGHUB_URL=https://hub.example"}, "flux-targets/nosuch is not a Target"},
		"an unreleased build has no binary to name": {"dev", []string{"FLUX_CONTEXT=c", "CLUSTER=dev", "CONFIGHUB_URL=https://hub.example"}, "set FLUXBOT_URL"},
		"a key that cannot be added":                {"0.4.0", []string{"FLUX_CONTEXT=c", "CLUSTER=dev", "CONFIGHUB_URL=https://hub.example", "KEY_ADD_FAILS=1"}, "could not add a key for flux-delivery"},
	} {
		t.Run(name, func(t *testing.T) {
			out, log, err := runFluxbot(t, tc.version, tc.env...)
			if err == nil || !strings.Contains(out, tc.says) {
				t.Errorf("want a refusal saying %q: %v\n%s", tc.says, err, out)
			}
			if strings.Contains(log, "apply") || (strings.Contains(log, "target update") && name != "a key that cannot be added") {
				t.Errorf("nothing may be changed:\n%s", log)
			}
		})
	}
}

// With no fleet repository, the layers to report are the ones on the cluster
// that read a ConfigHub Space of this fleet.
func TestDiscoverLayersReadsThemFromTheCluster(t *testing.T) {
	run := fake(map[string]string{
		"get ocirepositories": `{"items":[
 {"metadata":{"name":"apps"},"spec":{"url":"oci://gw.example/space/flux-apps-dev"}},
 {"metadata":{"name":"confighub-root"},"spec":{"url":"oci://gw.example/space/flux-dev-layers/"}},
 {"metadata":{"name":"other"},"spec":{"url":"oci://gw.example/space/another-fleet-apps-dev"}},
 {"metadata":{"name":"podinfo"},"spec":{"url":"oci://ghcr.io/stefanprodan/manifests/podinfo"}}]}`,
		"get kustomizations": `{"items":[
 {"metadata":{"name":"confighub-root"},"spec":{"sourceRef":{"kind":"OCIRepository","name":"confighub-root"}}},
 {"metadata":{"name":"apps"},"spec":{"sourceRef":{"kind":"OCIRepository","name":"apps"}}},
 {"metadata":{"name":"other"},"spec":{"sourceRef":{"kind":"OCIRepository","name":"other"}}},
 {"metadata":{"name":"podinfo"},"spec":{"sourceRef":{"kind":"OCIRepository","name":"podinfo"}}},
 {"metadata":{"name":"elsewhere"},"spec":{"sourceRef":{"kind":"OCIRepository","name":"apps","namespace":"tenant-a"}}},
 {"metadata":{"name":"flux-system"},"spec":{"sourceRef":{"kind":"GitRepository","name":"flux-system"}}}]}`,
	})
	got, err := DiscoverLayers(run, "flux")
	if err != nil {
		t.Fatal(err)
	}
	want := []Check{{Kustomization: "apps", Space: "flux-apps-dev"}, {Kustomization: "confighub-root", Space: "flux-dev-layers"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("want %+v, got %+v", want, got)
	}
	if all, _ := DiscoverLayers(run, ""); len(all) != 3 {
		t.Errorf("with no prefix every layer reading a Space is found: %+v", all)
	}
	if _, err := DiscoverLayers(fake(nil), "flux"); err == nil {
		t.Error("a cluster that cannot be read is an error, not an empty fleet")
	}
}

// On the cluster the reporter has no signed-in user: it signs in as the
// service account whose key it holds, its session goes on every request
// after, and it signs in again when ConfigHub refuses that session.
func TestSDKHubSignsInAsAServiceAccount(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.RawURLEncoding.EncodeToString
	key := `{"kty":"OKP","crv":"Ed25519","x":"` + b64(pub) + `","d":"` + b64(priv.Seed()) + `","confighub_user_external_id":"sa-user"}`
	var signIns int
	var expired bool
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/token" {
			signIns++
			r.ParseForm()
			if r.Form.Get("assertion") == "" || !strings.Contains(r.Form.Get("grant_type"), "jwt-bearer") {
				t.Errorf("a signed assertion is what it signs in with: %v", r.Form)
			}
			fmt.Fprintf(w, `{"access_token":"sa-token-%d"}`, signIns)
			return
		}
		auth = r.Header.Get("Authorization")
		if expired && auth == "Bearer sa-token-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`[{"Space":{"SpaceID":"11111111-1111-4111-8111-111111111111","Slug":"s"}}]`))
	}))
	defer srv.Close()
	t.Setenv(EnvPrivateKey, key)
	t.Setenv(EnvServerURL, srv.URL+"/")
	// Only a Hub told to sign in with the key reads it.
	t.Setenv("CUB_SERVER", "")
	t.Setenv("CUB_TOKEN", "")
	t.Setenv("CUB_CONFIG", filepath.Join(t.TempDir(), "none.yaml"))
	if _, err := NewHub("test").Releases("s"); err == nil || signIns != 0 {
		t.Errorf("without being told, the key is not used: %v, %d sign-ins", err, signIns)
	}
	h := NewHub("test")
	h.AsServiceAccount()
	if err := h.SignIn(); err != nil {
		t.Fatalf("a service account that can sign in and read is ready: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := h.Releases("s"); err != nil {
			t.Fatal(err)
		}
	}
	if signIns != 1 || auth != "Bearer sa-token-1" {
		t.Errorf("want one sign-in and its session on each request: %d sign-ins, Authorization %q", signIns, auth)
	}
	expired = true
	if _, err := h.Releases("s"); err != nil || signIns != 2 || auth != "Bearer sa-token-2" {
		t.Errorf("a refused session is renewed with the key: %v, %d sign-ins, Authorization %q", err, signIns, auth)
	}
	t.Setenv(EnvServerURL, "")
	lost := NewHub("test")
	lost.AsServiceAccount()
	if err := lost.SignIn(); err == nil || !strings.Contains(err.Error(), EnvServerURL) {
		t.Errorf("a service account with no server to sign in to must say what is missing: %v", err)
	}
	t.Setenv(EnvServerURL, srv.URL)
	t.Setenv(EnvPrivateKey, `{"kty":"OKP","crv":"Ed25519","x":"`+b64(pub)+`"}`)
	public := NewHub("test")
	public.AsServiceAccount()
	if err := public.SignIn(); err == nil || !strings.Contains(err.Error(), EnvPrivateKey) {
		t.Errorf("a public key is refused, naming where it came from: %v", err)
	}
}

// When the pod does not come up, the script says why from the pod's own
// words, and fails.
func TestFluxbotSaysWhyThePodDidNotComeUp(t *testing.T) {
	out, log, err := runFluxbot(t, "0.4.0", "FLUX_CONTEXT=ctx-dev", "CLUSTER=dev", "CONFIGHUB_URL=https://hub.example", "ROLLOUT_FAILS=1")
	if err == nil || !strings.Contains(out, "fluxbot did not come up") {
		t.Fatalf("want a failure that says so: %v\n%s", err, out)
	}
	if !strings.Contains(log, "--sort-by=.metadata.creationTimestamp") || !strings.Contains(log, "-c fetch --tail=10") || !strings.Contains(log, "-c fluxbot --previous --tail=10") {
		t.Errorf("show what the newest pod said, its fetch and the reporter, not an older pod's:\n%s", log)
	}
	if strings.Contains(out, "fluxbot runs in") {
		t.Errorf("it must not go on to say it runs:\n%s", out)
	}
}

// Run again, it keeps the key the Secret holds while ConfigHub lists it,
// rather than adding another. The Secret is written again with it, which
// drops a copy an earlier kubectl apply left, and only fluxbot's other keys
// on this cluster are removed.
func TestFluxbotKeepsTheKeyItHas(t *testing.T) {
	out, log, err := runFluxbot(t, "0.4.0", "FLUX_CONTEXT=ctx-dev", "CLUSTER=dev", "CONFIGHUB_URL=https://hub.example", "KEY_IN_SECRET=1", "HELD_KID=k-held", "KEYS=k-held k-old")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if strings.Contains(log, "key add") {
		t.Errorf("a registered key is kept:\n%s", log)
	}
	if !strings.Contains(log, "# the Secret, from: key") || !strings.Contains(log, "confighub.com/key-id=k-held") {
		t.Errorf("the Secret is written again with the key it held:\n%s", log)
	}
	if !strings.Contains(log, "key delete flux-delivery k-old") || strings.Contains(log, "delete flux-delivery k-held") || strings.Contains(log, "k-other") {
		t.Errorf("only fluxbot's other keys on this cluster are removed:\n%s", log)
	}
	if !strings.Contains(log, "kind: Deployment") {
		t.Errorf("the pod is still applied:\n%s", log)
	}
}

// A key ConfigHub no longer lists, or a Secret from an earlier version that
// records none, is replaced: the new key's id goes on the Secret, and
// fluxbot's earlier keys on this cluster are removed.
func TestFluxbotReplacesAKeyConfigHubDoesNotList(t *testing.T) {
	for _, held := range []string{"HELD_KID=k-gone", "HELD_KID="} {
		out, log, err := runFluxbot(t, "0.4.0", "FLUX_CONTEXT=ctx-dev", "CLUSTER=dev", "CONFIGHUB_URL=https://hub.example", "KEY_IN_SECRET=1", held, "KEYS=k-old")
		if err != nil {
			t.Fatalf("%s: %v\n%s", held, err, out)
		}
		if !strings.Contains(log, "key add flux-delivery --generate") || !strings.Contains(log, `# the Secret, from: {"d":"s3cret"}`) || !strings.Contains(log, "confighub.com/key-id=k-new") {
			t.Errorf("%s: a new key goes in the Secret, with its id:\n%s", held, log)
		}
		if !strings.Contains(log, "key delete flux-delivery k-old") || strings.Contains(log, "delete flux-delivery k-new") || strings.Contains(log, "k-other") {
			t.Errorf("%s: only fluxbot's earlier keys on this cluster are removed:\n%s", held, log)
		}
		for _, l := range strings.Split(log, "\n") {
			if (strings.HasPrefix(l, "kubectl ") || strings.HasPrefix(l, "cub ")) && strings.Contains(l, "s3cret") {
				t.Errorf("%s: the key is on a command line: %s", held, l)
			}
		}
	}
}
