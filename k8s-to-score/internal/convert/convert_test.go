// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// convertFixtures runs the converter over testdata/sample, the same fixtures
// the README walkthrough uses.
func convertFixtures(t *testing.T) *Result {
	t.Helper()
	dir := filepath.Join("..", "..", "testdata", "sample")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading fixtures: %v", err)
	}
	var units []UnitData
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		units = append(units, UnitData{Slug: e.Name(), Data: data})
	}
	res, err := Convert(units)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	return res
}

func workloadNamed(t *testing.T, res *Result, name string) NamedWorkload {
	t.Helper()
	for _, w := range res.Workloads {
		if w.Name == name {
			return w
		}
	}
	t.Fatalf("no workload named %q (have %d)", name, len(res.Workloads))
	return NamedWorkload{}
}

func TestConvertProducesOneWorkloadPerController(t *testing.T) {
	res := convertFixtures(t)
	if got := len(res.Workloads); got != 2 {
		t.Fatalf("workloads = %d, want 2 (one Deployment, one StatefulSet)", got)
	}
	// Sorted by name, so output is stable run to run.
	if res.Workloads[0].Name != "checkout" || res.Workloads[1].Name != "ledger" {
		t.Errorf("workloads not sorted by name: %v", []string{res.Workloads[0].Name, res.Workloads[1].Name})
	}
}

func TestStatefulSetCarriesKindAnnotation(t *testing.T) {
	res := convertFixtures(t)

	ledger := workloadNamed(t, res, "ledger").Workload
	ann, ok := ledger.Metadata["annotations"].(map[string]string)
	if !ok {
		t.Fatalf("ledger has no annotations map, got %T", ledger.Metadata["annotations"])
	}
	if ann[WorkloadKindAnnotation] != "StatefulSet" {
		t.Errorf("%s = %q, want StatefulSet", WorkloadKindAnnotation, ann[WorkloadKindAnnotation])
	}

	// A Deployment is Score's default, so it must not carry the annotation.
	checkout := workloadNamed(t, res, "checkout").Workload
	if _, present := checkout.Metadata["annotations"]; present {
		t.Errorf("Deployment workload should not carry a kind annotation")
	}
}

func TestServicePortsResolveNamedTargetPort(t *testing.T) {
	res := convertFixtures(t)
	checkout := workloadNamed(t, res, "checkout").Workload

	if checkout.Service == nil {
		t.Fatal("checkout has no service block; the Service selector should have matched the pod labels")
	}
	p, ok := checkout.Service.Ports["http"]
	if !ok {
		t.Fatalf("no http port, have %v", checkout.Service.Ports)
	}
	if p.Port != 80 {
		t.Errorf("port = %d, want 80", p.Port)
	}
	// targetPort was the named container port "http" -> containerPort 8080.
	if p.TargetPort == nil || *p.TargetPort != 8080 {
		t.Errorf("targetPort = %v, want 8080 resolved from the named container port", p.TargetPort)
	}
}

func TestIngressBecomesRouteResource(t *testing.T) {
	res := convertFixtures(t)
	checkout := workloadNamed(t, res, "checkout").Workload

	route, ok := checkout.Resources["checkout-route"]
	if !ok {
		t.Fatalf("no route resource, have %v", keysOfResources(checkout.Resources))
	}
	if route.Type != "route" {
		t.Errorf("type = %q, want route", route.Type)
	}
	if route.Params["host"] != "shop.example.com" {
		t.Errorf("host = %v, want the literal Ingress host", route.Params["host"])
	}
	if route.Params["path"] != "/checkout" {
		t.Errorf("path = %v, want /checkout", route.Params["path"])
	}
	// score-k8s requires params.port to name a port the workload's service
	// block declares, so it must be the Service port, not the container port.
	if route.Params["port"] != 80 {
		t.Errorf("port = %v, want the Service port 80", route.Params["port"])
	}
}

func TestVolumesBecomeVolumeResources(t *testing.T) {
	res := convertFixtures(t)

	// emptyDir on a Deployment.
	checkout := workloadNamed(t, res, "checkout").Workload
	if r, ok := checkout.Resources["cache"]; !ok || r.Type != "volume" {
		t.Errorf("emptyDir did not become a volume resource: %v", checkout.Resources["cache"])
	}
	if v, ok := checkout.Containers["checkout"].Volumes["/var/cache"]; !ok || v.Source != "${resources.cache}" {
		t.Errorf("volume mount source = %q, want ${resources.cache}", v.Source)
	}

	// volumeClaimTemplate on a StatefulSet.
	ledger := workloadNamed(t, res, "ledger").Workload
	if r, ok := ledger.Resources["data"]; !ok || r.Type != "volume" {
		t.Errorf("volumeClaimTemplate did not become a volume resource: %v", ledger.Resources["data"])
	}
}

func TestConfigMapMountBecomesFilesWithLiteralContent(t *testing.T) {
	res := convertFixtures(t)
	c := workloadNamed(t, res, "checkout").Workload.Containers["checkout"]

	f, ok := c.Files["/etc/checkout/settings.yaml"]
	if !ok {
		t.Fatalf("no file at /etc/checkout/settings.yaml, have %v", keysOfFiles(c.Files))
	}
	if f.Content == nil || !strings.Contains(*f.Content, "retries: 3") {
		t.Errorf("file content did not come from the ConfigMap Unit: %v", f.Content)
	}
	// All three ConfigMap keys mount when the volume declares no items.
	if len(c.Files) != 3 {
		t.Errorf("files = %d, want 3 (one per ConfigMap key)", len(c.Files))
	}
}

func TestVariablesResolveConfigMapAndPlaceholderSecrets(t *testing.T) {
	res := convertFixtures(t)
	vars := workloadNamed(t, res, "checkout").Workload.Containers["checkout"].Variables

	if vars["LOG_LEVEL"] != "info" {
		t.Errorf("LOG_LEVEL = %q, want info", vars["LOG_LEVEL"])
	}
	// configMapKeyRef resolves against the ConfigMap Unit in the same Space.
	if vars["FEATURE_FLAGS"] != "promo=on,express=off" {
		t.Errorf("FEATURE_FLAGS = %q, want the resolved ConfigMap value", vars["FEATURE_FLAGS"])
	}
	// envFrom expands the ConfigMap's other keys.
	if vars["TIMEOUT_SECONDS"] != "30" {
		t.Errorf("TIMEOUT_SECONDS = %q, want 30 from envFrom", vars["TIMEOUT_SECONDS"])
	}
	// A Secret value is not config data, so it becomes a placeholder.
	if vars["DB_PASSWORD"] != Placeholder {
		t.Errorf("DB_PASSWORD = %q, want the placeholder sentinel", vars["DB_PASSWORD"])
	}
	// metadata.name has a Score equivalent.
	if vars["POD_NAME"] != "${metadata.name}" {
		t.Errorf("POD_NAME = %q, want ${metadata.name}", vars["POD_NAME"])
	}
	// settings.yaml is not a valid env var name; Kubernetes skips it on
	// envFrom, so the converter must too.
	if _, present := vars["settings.yaml"]; present {
		t.Error("settings.yaml must not become a variable: Kubernetes skips envFrom keys that are not valid env var names")
	}
}

func TestProbesResolveNamedPorts(t *testing.T) {
	res := convertFixtures(t)
	c := workloadNamed(t, res, "checkout").Workload.Containers["checkout"]

	if c.LivenessProbe == nil || c.LivenessProbe.HttpGet == nil {
		t.Fatal("no liveness httpGet probe")
	}
	// The source used the named port "http"; Score requires an integer.
	if got := c.LivenessProbe.HttpGet.Port; got != 8080 {
		t.Errorf("liveness port = %d, want 8080 resolved from the named container port", got)
	}
	if c.ReadinessProbe == nil || c.ReadinessProbe.HttpGet.Port != 8080 {
		t.Error("readiness probe did not carry its numeric port through")
	}
}

func TestLossyConversionsAreReported(t *testing.T) {
	res := convertFixtures(t)

	want := []string{"init container", "replicas=3", "DB_PASSWORD", "settings.yaml"}
	joined := strings.Join(messagesOf(res), "\n")
	for _, w := range want {
		if !strings.Contains(joined, w) {
			t.Errorf("no warning mentioning %q; the converter must report what it cannot express.\ngot:\n%s", w, joined)
		}
	}
}

func TestUnknownKindsAreSkippedNotFatal(t *testing.T) {
	crd := []byte(`apiVersion: example.com/v1
kind: Widget
metadata:
  name: sprocket
spec:
  size: large
`)
	res, err := Convert([]UnitData{{Slug: "widget", Data: crd}})
	if err != nil {
		t.Fatalf("an unknown kind must not fail the conversion: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Kind != "Widget" {
		t.Errorf("skipped = %v, want one Widget entry", res.Skipped)
	}
}

func TestMultiDocumentUnitData(t *testing.T) {
	// A Unit that holds several resources (a Helm or Kustomize import) still
	// converts, and a trailing separator is not an error.
	data := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  selector:
    matchLabels: {app: web}
  template:
    metadata:
      labels: {app: web}
    spec:
      containers:
        - name: web
          image: nginx:1.27
---
apiVersion: v1
kind: Service
metadata:
  name: web
spec:
  selector: {app: web}
  ports:
    - name: http
      port: 80
---
`)
	res, err := Convert([]UnitData{{Slug: "web", Data: data}})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if len(res.Workloads) != 1 {
		t.Fatalf("workloads = %d, want 1", len(res.Workloads))
	}
	w := res.Workloads[0].Workload
	if w.Service == nil || w.Service.Ports["http"].Port != 80 {
		t.Errorf("the Service in the same Unit should have folded into service.ports, got %v", w.Service)
	}
}

func TestSanitizeName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"checkout", "checkout"},
		{"Checkout-API", "checkout-api"},
		{"my.svc.name", "my-svc-name"},
		{"--leading", "leading"},
		{"trailing__", "trailing"},
	} {
		if got := sanitizeName(tc.in); got != tc.want {
			t.Errorf("sanitizeName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestValidEnvName(t *testing.T) {
	valid := []string{"LOG_LEVEL", "_x", "a1"}
	invalid := []string{"", "1abc", "settings.yaml", "a-b"}
	for _, s := range valid {
		if !validEnvName(s) {
			t.Errorf("validEnvName(%q) = false, want true", s)
		}
	}
	for _, s := range invalid {
		if validEnvName(s) {
			t.Errorf("validEnvName(%q) = true, want false", s)
		}
	}
}

func messagesOf(res *Result) []string {
	out := make([]string, 0, len(res.Warnings))
	for _, w := range res.Warnings {
		out = append(out, w.Message)
	}
	return out
}

func keysOfResources[V any](m map[string]V) []string { return keysOf(m) }
func keysOfFiles[V any](m map[string]V) []string     { return keysOf(m) }
