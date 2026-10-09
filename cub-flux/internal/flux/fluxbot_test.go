package flux

import (
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
  *" create secret generic "*) echo "# the Secret, from: $(cat | tr '\n' ' ')" ;;
  *" apply -f - "*) cat >> "$STUB_LOG" ;;
  *" rollout status "*) [ -z "${ROLLOUT_FAILS:-}" ] || exit 1 ;;
esac
exit 0
`,
		"cub": `#!/usr/bin/env bash
echo "cub $*" >> "$STUB_LOG"
case " $* " in
  *" target get "*) case " $* " in *" nosuch "*) exit 1 ;; esac ;;
  *"--include-secret"*) echo "\"${WORKER_SECRET-s3cret}\"" ;;
  *"BridgeWorkerID"*) echo '"worker-id"' ;;
  *"UserID"*) echo '"bot-user"' ;;
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
	grant := strings.Index(log, `cub target update dev --space flux-targets --permission EditChildren:bot-user`)
	deploy := strings.Index(log, "kind: Deployment")
	if grant < 0 || deploy < 0 || grant > deploy {
		t.Errorf("the grant comes before the pod that needs it:\n%s", log)
	}
	for _, want := range []string{
		"kubectl --context ctx-dev apply -f -",
		"create secret generic fluxbot --from-env-file=/dev/stdin --dry-run=client",
		"# the Secret, from: CONFIGHUB_WORKER_ID=worker-id CONFIGHUB_WORKER_SECRET=s3cret",
		`{name: FROM, value: "https://github.com/confighub/examples/releases/download/cub-flux-v0.4.0"}`,
		`command: ["/tools/cub-flux", "status", "--discover", "--as-worker", "--watch", "--ready-file", "/tmp/ready", "--prefix", "flux", "--namespace", "flux-system"]`,
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
		t.Errorf("the worker's secret must not be printed:\n%s", out)
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
	if !strings.Contains(out, "delete deployment,serviceaccount,role,rolebinding,secret fluxbot") || !strings.Contains(out, "--permission -EditChildren:bot-user") {
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
		"a worker with no secret to read":           {"0.4.0", []string{"FLUX_CONTEXT=c", "CLUSTER=dev", "CONFIGHUB_URL=https://hub.example", "WORKER_SECRET=null"}, "its user, ID or secret is missing"},
	} {
		t.Run(name, func(t *testing.T) {
			out, log, err := runFluxbot(t, tc.version, tc.env...)
			if err == nil || !strings.Contains(out, tc.says) {
				t.Errorf("want a refusal saying %q: %v\n%s", tc.says, err, out)
			}
			if strings.Contains(log, "apply") || (strings.Contains(log, "target update") && name != "a worker with no secret to read") {
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
// worker, and its token goes on every request after.
func TestSDKHubSignsInAsAWorker(t *testing.T) {
	var signIns int
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/worker" {
			signIns++
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"worker_id":"worker-id"`) || !strings.Contains(string(body), `"worker_secret":"s3cret"`) {
				t.Errorf("the worker's ID and secret are what it signs in with: %s", body)
			}
			w.Write([]byte(`{"access_token":"worker-token"}`))
			return
		}
		auth = r.Header.Get("Authorization")
		w.Write([]byte(`[{"Space":{"SpaceID":"11111111-1111-4111-8111-111111111111","Slug":"s"}}]`))
	}))
	defer srv.Close()
	t.Setenv(EnvWorkerID, "worker-id")
	t.Setenv(EnvWorkerSecret, "s3cret")
	t.Setenv(EnvServerURL, srv.URL+"/")
	// The names are cub-worker's own, so they may be set for another reason:
	// only a Hub told to be a worker reads them.
	t.Setenv("CUB_SERVER", "")
	t.Setenv("CUB_TOKEN", "")
	t.Setenv("CUB_CONFIG", filepath.Join(t.TempDir(), "none.yaml"))
	if _, err := NewHub("test").Releases("s"); err == nil || signIns != 0 {
		t.Errorf("without being told, the worker's credential is not used: %v, %d sign-ins", err, signIns)
	}
	h := NewHub("test")
	h.AsWorker()
	if err := h.SignIn(); err != nil {
		t.Fatalf("a worker that can sign in and read is ready: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := h.Releases("s"); err != nil {
			t.Fatal(err)
		}
	}
	if signIns != 1 || auth != "Bearer worker-token" {
		t.Errorf("want one sign-in and its token on each request: %d sign-ins, Authorization %q", signIns, auth)
	}
	t.Setenv(EnvServerURL, "")
	lost := NewHub("test")
	lost.AsWorker()
	if err := lost.SignIn(); err == nil || !strings.Contains(err.Error(), EnvServerURL) {
		t.Errorf("a worker with no server to sign in to must say what is missing: %v", err)
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
