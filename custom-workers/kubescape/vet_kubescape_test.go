// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/confighub/sdk/configkit/k8skit"
	"github.com/confighub/sdk/core/function/api"
	"github.com/confighub/sdk/core/third_party/gaby"
)

// A well-configured Deployment with security context.
const testDeploymentGood = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: good-deployment
  namespace: default
  labels:
    app: good
spec:
  replicas: 3
  selector:
    matchLabels:
      app: good
  template:
    metadata:
      labels:
        app: good
    spec:
      containers:
      - name: web
        image: nginx:1.21
        ports:
        - containerPort: 80
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 200m
            memory: 256Mi
        securityContext:
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          allowPrivilegeEscalation: false
`

// A minimal Deployment missing security controls.
const testDeploymentBad = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: bad-deployment
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: bad
  template:
    metadata:
      labels:
        app: bad
    spec:
      containers:
      - name: web
        image: nginx:latest
        securityContext:
          privileged: true
`

func requireKubescapeCLI(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(kubescapeBinary); err != nil {
		t.Skipf("kubescape CLI not found in PATH: %v", err)
	}
}

func TestVetKubescape_GoodDeployment(t *testing.T) {
	requireKubescapeCLI(t)

	parsedData, err := gaby.ParseAll([]byte(testDeploymentGood))
	require.NoError(t, err)

	rp := k8skit.NewK8sResourceProvider()
	// Good deploy has Medium-level findings; threshold High should pass.
	args := []api.FunctionArgument{{Value: "High"}}
	_, result, err := vetKubescape(rp, parsedData, args)
	require.NoError(t, err)

	vr, ok := result.(api.ValidationResult)
	require.True(t, ok)
	assert.True(t, vr.Passed)
}

func TestVetKubescape_BadDeployment(t *testing.T) {
	requireKubescapeCLI(t)

	parsedData, err := gaby.ParseAll([]byte(testDeploymentBad))
	require.NoError(t, err)

	rp := k8skit.NewK8sResourceProvider()
	// Bad deploy has High-level findings; threshold High should fail.
	args := []api.FunctionArgument{{Value: "High"}}
	_, result, err := vetKubescape(rp, parsedData, args)
	require.NoError(t, err)

	vr, ok := result.(api.ValidationResult)
	require.True(t, ok)
	assert.False(t, vr.Passed)
	assert.NotEmpty(t, vr.Details)
}

func TestParseKubescapeJSON(t *testing.T) {
	// Test JSON parsing without CLI, using actual kubescape output format.
	jsonOutput := `{
		"summaryDetails": {
			"status": "failed",
			"score": 42.3,
			"ResourceCounters": {"passedResources": 5, "failedResources": 3, "skippedResources": 0},
			"controls": {
				"C-0013": {"controlID": "C-0013", "name": "Non-root containers", "severity": "Medium"},
				"C-0017": {"controlID": "C-0017", "name": "Immutable container filesystem", "severity": "Low"}
			}
		},
		"results": [
			{
				"resourceID": "path=123/api=apps/v1/default/Deployment/my-deploy",
				"controls": [
					{
						"controlID": "C-0013",
						"name": "Non-root containers",
						"severity": "Medium",
						"status": {"status": "failed"},
						"rules": [
							{
								"name": "non-root-containers",
								"status": {"status": "failed"},
								"paths": [
									{"fixPath": {"path": "spec.template.spec.containers[0].securityContext.runAsNonRoot", "value": "true"}}
								]
							}
						]
					},
					{
						"controlID": "C-0057",
						"name": "Privileged container",
						"severity": "High",
						"status": {"status": "failed"},
						"rules": [
							{
								"name": "privileged-container",
								"status": {"status": "failed"},
								"paths": [
									{"failedPath": "spec.template.spec.containers[0].securityContext.privileged"}
								]
							}
						]
					}
				]
			}
		]
	}`

	var report kubescapeReport
	err := json.Unmarshal([]byte(jsonOutput), &report)
	require.NoError(t, err)

	assert.Equal(t, 3, report.SummaryDetails.StatusCounters.Failed)
	assert.Len(t, report.Results, 1)
	assert.Len(t, report.Results[0].AssociatedControls, 2)
	assert.Equal(t, "C-0013", report.Results[0].AssociatedControls[0].ControlID)
	assert.Equal(t, "failed", report.Results[0].AssociatedControls[0].Status.Status)

	// fixPath is an object with path and value.
	fixPath := report.Results[0].AssociatedControls[0].Rules[0].Paths[0].FixPath
	assert.Equal(t, "spec.template.spec.containers[0].securityContext.runAsNonRoot", fixPath.Path)

	// failedPath is a string.
	failedPath := report.Results[0].AssociatedControls[1].Rules[0].Paths[0].FailedPath
	assert.Equal(t, "spec.template.spec.containers[0].securityContext.privileged", failedPath)
}

func TestKubescapeSeverityToScore(t *testing.T) {
	assert.Equal(t, api.ScoreCritical, kubescapeSeverityToScore("critical"))
	assert.Equal(t, api.ScoreCritical, kubescapeSeverityToScore("Critical"))
	assert.Equal(t, api.ScoreHigh, kubescapeSeverityToScore("High"))
	assert.Equal(t, api.ScoreMedium, kubescapeSeverityToScore("medium"))
	assert.Equal(t, api.ScoreLow, kubescapeSeverityToScore("low"))
	assert.Equal(t, api.ScoreLow, kubescapeSeverityToScore("unknown"))
}

func TestNormalizePath(t *testing.T) {
	// JSON Pointer form (slash-separated).
	assert.Equal(t, "spec.template.spec.containers.0.securityContext", normalizePath("spec/template/spec/containers/0/securityContext"))
	assert.Equal(t, "spec.containers.0.image", normalizePath("/spec/containers/0/image/"))
	// Kubescape bracket notation (already dot-separated).
	assert.Equal(t, "spec.template.spec.containers[0].resources.limits.memory", normalizePath("spec.template.spec.containers[0].resources.limits.memory"))
	// Empty.
	assert.Equal(t, "", normalizePath(""))
}

func TestLookupResourceInfoByKubescapeID(t *testing.T) {
	infoMap := map[string]api.ResourceInfo{
		"default/Deployment/my-deploy": {
			ResourceName: "default/my-deploy",
			ResourceType: "apps/v1/Deployment",
		},
	}

	// Kubescape format with path= prefix.
	info := lookupResourceInfoByKubescapeID(infoMap, "path=123/api=apps/v1/default/Deployment/my-deploy")
	assert.Equal(t, api.ResourceName("default/my-deploy"), info.ResourceName)

	// No match returns empty.
	info = lookupResourceInfoByKubescapeID(infoMap, "path=456/api=v1/default/Pod/other-pod")
	assert.Equal(t, api.ResourceInfo{}, info)
}
