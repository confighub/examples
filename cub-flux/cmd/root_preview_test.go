package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanPreviewJSONUsesEnvelopeAndPinnedExportTime(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "gitops", "flux", "expert-fleet")
	const stamp = "2026-09-30T00:00:00Z"
	command := newRoot()
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"plan", input, "--repo-root", root, "--format", "preview-json", "--exported-at", stamp})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute preview-json: %v; stderr=%s", err, stderr.String())
	}
	var document map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("stdout is not a JSON document: %v\n%s", err, stdout.String())
	}
	if document["apiVersion"] != "confighub.com/plugin-preview/v1" || document["exportedAt"] != stamp {
		t.Fatalf("unexpected preview header: %v", document)
	}
	if stderr.Len() != 0 {
		t.Fatalf("successful command wrote diagnostics: %s", stderr.String())
	}
}

func TestPlanJSONKeepsExistingPlanSchema(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "gitops", "flux", "expert-fleet")
	command := newRoot()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"plan", input, "--repo-root", root, "--json"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute legacy --json: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("stdout is not a JSON plan: %v\n%s", err, stdout.String())
	}
	if document["apiVersion"] != nil || document["components"] == nil || document["clusters"] == nil {
		t.Fatalf("legacy --json schema changed: keys=%v", document)
	}
}

func TestPlanPreviewEmitsErrorEnvelopeBeforeReturningPlanningError(t *testing.T) {
	input := t.TempDir()
	command := newRoot()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"plan", input, "--format", "preview-json", "--exported-at", "2026-09-30T00:00:00Z"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "plan has problems") {
		t.Fatalf("expected non-zero planning result, got %v", err)
	}
	var document struct {
		APIVersion string `json:"apiVersion"`
		Issues     []struct {
			Severity string `json:"severity"`
			Code     string `json:"code"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("planning error did not retain a valid JSON envelope: %v\n%s", err, stdout.String())
	}
	if document.APIVersion != "confighub.com/plugin-preview/v1" || len(document.Issues) != 1 || document.Issues[0].Severity != "error" || document.Issues[0].Code != "flux-plan-problem" {
		t.Fatalf("planning error envelope = %+v", document)
	}
}

func TestExportedAtRequiresPreviewFormat(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "gitops", "flux", "expert-fleet")
	command := newRoot()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"plan", input, "--repo-root", root, "--exported-at", "2026-09-30T00:00:00Z"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "only valid with --format preview-json") {
		t.Fatalf("expected preview-only flag error, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected output before rejecting --exported-at: %s", stdout.String())
	}
}
