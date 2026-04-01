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

// A well-configured Deployment that should pass most kube-linter checks.
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
`

// A minimal Deployment missing resource limits, security context, etc.
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
`

func requireKubeLinterCLI(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(kubeLinterBinary); err != nil {
		t.Skipf("kube-linter CLI not found in PATH: %v", err)
	}
}

func TestVetKubeLinter_GoodDeployment(t *testing.T) {
	requireKubeLinterCLI(t)

	parsedData, err := gaby.ParseAll([]byte(testDeploymentGood))
	require.NoError(t, err)

	rp := k8skit.NewK8sResourceProvider()
	// With threshold High, Medium findings should pass.
	args := []api.FunctionArgument{{Value: "High"}}
	_, result, err := vetKubeLinter(rp, parsedData, args)
	require.NoError(t, err)

	vr, ok := result.(api.ValidationResult)
	require.True(t, ok)
	assert.True(t, vr.Passed)
}

func TestVetKubeLinter_BadDeployment(t *testing.T) {
	requireKubeLinterCLI(t)

	parsedData, err := gaby.ParseAll([]byte(testDeploymentBad))
	require.NoError(t, err)

	rp := k8skit.NewK8sResourceProvider()
	// With threshold Medium, Medium findings should fail.
	args := []api.FunctionArgument{{Value: "Medium"}}
	_, result, err := vetKubeLinter(rp, parsedData, args)
	require.NoError(t, err)

	vr, ok := result.(api.ValidationResult)
	require.True(t, ok)
	assert.False(t, vr.Passed)
	assert.NotEmpty(t, vr.Details)
	assert.NotEmpty(t, vr.FailedAttributes)
}

func TestParseKubeLinterJSON(t *testing.T) {
	// Test JSON parsing without CLI.
	jsonOutput := `{
		"Checks": [
			{"name": "no-read-only-root-fs", "description": "check desc", "remediation": "fix it"}
		],
		"Reports": [
			{
				"Diagnostic": {"Message": "container has no read-only root filesystem"},
				"Check": "no-read-only-root-fs",
				"Remediation": "Set readOnlyRootFilesystem to true",
				"Object": {
					"Metadata": {"FilePath": "/tmp/test.yaml"},
					"K8sObject": {
						"Namespace": "default",
						"Name": "my-deploy",
						"GroupVersionKind": {"Group": "apps", "Version": "v1", "Kind": "Deployment"}
					}
				}
			},
			{
				"Diagnostic": {"Message": "container has no resource limits"},
				"Check": "unset-cpu-requirements",
				"Remediation": "Set CPU requests and limits",
				"Object": {
					"Metadata": {"FilePath": "/tmp/test.yaml"},
					"K8sObject": {
						"Namespace": "",
						"Name": "my-pod",
						"GroupVersionKind": {"Group": "", "Version": "v1", "Kind": "Pod"}
					}
				}
			}
		],
		"Summary": {"ChecksStatus": "Failed"}
	}`

	var output kubeLinterOutput
	err := json.Unmarshal([]byte(jsonOutput), &output)
	require.NoError(t, err)

	assert.Len(t, output.Checks, 1)
	assert.Len(t, output.Reports, 2)
	assert.Equal(t, "Failed", output.Summary.ChecksStatus)

	assert.Equal(t, "no-read-only-root-fs", output.Reports[0].Check)
	assert.Equal(t, "Deployment", output.Reports[0].Object.K8sObject.GroupVersionKind.Kind)
	assert.Equal(t, "my-deploy", output.Reports[0].Object.K8sObject.Name)
	assert.Equal(t, "default", output.Reports[0].Object.K8sObject.Namespace)

	assert.Equal(t, "unset-cpu-requirements", output.Reports[1].Check)
	assert.Equal(t, "Pod", output.Reports[1].Object.K8sObject.GroupVersionKind.Kind)
}
