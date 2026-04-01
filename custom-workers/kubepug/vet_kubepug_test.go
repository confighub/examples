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

// A Deployment using a current stable API that should pass.
const testDeploymentGood = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: good-deployment
  namespace: default
spec:
  replicas: 1
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
`

// A CronJob using batch/v1beta1, deprecated in 1.21 and deleted in 1.25.
const testCronJobDeprecated = `apiVersion: batch/v1beta1
kind: CronJob
metadata:
  name: old-cronjob
  namespace: default
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: hello
            image: busybox
            command: ["echo", "hello"]
          restartPolicy: OnFailure
`

// An Ingress using extensions/v1beta1, deleted in 1.22.
const testIngressDeleted = `apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: old-ingress
  namespace: default
spec:
  rules:
  - host: example.com
    http:
      paths:
      - path: /
        backend:
          serviceName: web
          servicePort: 80
`

func requireKubepugCLI(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(kubepugBinary); err != nil {
		t.Skipf("kubepug CLI not found in PATH: %v", err)
	}
}

func TestVetKubepug_GoodDeployment(t *testing.T) {
	requireKubepugCLI(t)

	parsedData, err := gaby.ParseAll([]byte(testDeploymentGood))
	require.NoError(t, err)

	rp := k8skit.NewK8sResourceProvider()
	args := []api.FunctionArgument{{Value: "v1.25"}, {Value: "Low"}}
	_, result, err := vetKubepug(rp, parsedData, args)
	require.NoError(t, err)

	vr, ok := result.(api.ValidationResult)
	require.True(t, ok)
	assert.True(t, vr.Passed)
}

func TestVetKubepug_DeprecatedCronJob(t *testing.T) {
	requireKubepugCLI(t)

	parsedData, err := gaby.ParseAll([]byte(testCronJobDeprecated))
	require.NoError(t, err)

	rp := k8skit.NewK8sResourceProvider()
	// batch/v1beta1 CronJob was deprecated in 1.21 (removed in 1.25).
	// At v1.21 kubepug reports it as deprecated (High severity).
	// With threshold Critical, it should pass (High < Critical).
	args := []api.FunctionArgument{{Value: "v1.21"}, {Value: "Critical"}}
	_, result, err := vetKubepug(rp, parsedData, args)
	require.NoError(t, err)

	vr, ok := result.(api.ValidationResult)
	require.True(t, ok)
	assert.True(t, vr.Passed)
	assert.Equal(t, api.ScoreHigh, vr.MaxScore)

	// With threshold High, it should fail.
	args2 := []api.FunctionArgument{{Value: "v1.21"}, {Value: "High"}}
	_, result2, err := vetKubepug(rp, parsedData, args2)
	require.NoError(t, err)

	vr2, ok := result2.(api.ValidationResult)
	require.True(t, ok)
	assert.False(t, vr2.Passed)
	assert.Equal(t, api.ScoreHigh, vr2.MaxScore)
	assert.NotEmpty(t, vr2.Details)
	assert.NotEmpty(t, vr2.FailedAttributes)
}

func TestVetKubepug_DeletedCronJob(t *testing.T) {
	requireKubepugCLI(t)

	parsedData, err := gaby.ParseAll([]byte(testCronJobDeprecated))
	require.NoError(t, err)

	rp := k8skit.NewK8sResourceProvider()
	// At v1.25, batch/v1beta1 CronJob is deleted (Critical severity).
	args := []api.FunctionArgument{{Value: "v1.25"}, {Value: "Critical"}}
	_, result, err := vetKubepug(rp, parsedData, args)
	require.NoError(t, err)

	vr, ok := result.(api.ValidationResult)
	require.True(t, ok)
	assert.False(t, vr.Passed)
	assert.Equal(t, api.ScoreCritical, vr.MaxScore)
	assert.NotEmpty(t, vr.Details)
	assert.NotEmpty(t, vr.FailedAttributes)
}

func TestVetKubepug_DeletedIngress(t *testing.T) {
	requireKubepugCLI(t)

	parsedData, err := gaby.ParseAll([]byte(testIngressDeleted))
	require.NoError(t, err)

	rp := k8skit.NewK8sResourceProvider()
	// extensions/v1beta1 Ingress was deleted in 1.22.
	args := []api.FunctionArgument{{Value: "v1.22"}, {Value: "Critical"}}
	_, result, err := vetKubepug(rp, parsedData, args)
	require.NoError(t, err)

	vr, ok := result.(api.ValidationResult)
	require.True(t, ok)
	assert.False(t, vr.Passed)
	assert.Equal(t, api.ScoreCritical, vr.MaxScore)
	assert.NotEmpty(t, vr.Details)
	assert.NotEmpty(t, vr.FailedAttributes)
}

func TestParseKubepugJSON(t *testing.T) {
	// Test parsing without CLI.
	jsonOutput := `{
		"deprecated_apis": [
			{
				"group": "batch",
				"kind": "CronJob",
				"version": "v1beta1",
				"k8sversion": "1.21",
				"replacement": {"group": "batch", "version": "v1", "kind": "CronJob"},
				"deleted_items": [
					{"scope": "OBJECT", "objectname": "my-cronjob", "namespace": "default", "location": "/tmp/test.yaml"}
				]
			}
		],
		"deleted_apis": [
			{
				"group": "extensions",
				"kind": "Ingress",
				"version": "v1beta1",
				"k8sversion": "1.22",
				"replacement": {"group": "networking.k8s.io", "version": "v1", "kind": "Ingress"},
				"deleted_items": [
					{"scope": "OBJECT", "objectname": "my-ingress", "namespace": "default", "location": "/tmp/test.yaml"}
				]
			}
		]
	}`

	var result kubepugResult
	err := json.Unmarshal([]byte(jsonOutput), &result)
	require.NoError(t, err)

	assert.Len(t, result.DeprecatedAPIs, 1)
	assert.Len(t, result.DeletedAPIs, 1)

	assert.Equal(t, "CronJob", result.DeprecatedAPIs[0].Kind)
	assert.Equal(t, "v1beta1", result.DeprecatedAPIs[0].Version)
	assert.NotNil(t, result.DeprecatedAPIs[0].Replacement)
	assert.Equal(t, "batch", result.DeprecatedAPIs[0].Replacement.Group)
	assert.Len(t, result.DeprecatedAPIs[0].Items, 1)
	assert.Equal(t, "my-cronjob", result.DeprecatedAPIs[0].Items[0].ObjectName)

	assert.Equal(t, "Ingress", result.DeletedAPIs[0].Kind)
	assert.Equal(t, "networking.k8s.io", result.DeletedAPIs[0].Replacement.Group)
	assert.Len(t, result.DeletedAPIs[0].Items, 1)
}

func TestKubepugResourceKey(t *testing.T) {
	apiItem := kubepugResultItem{Kind: "Ingress"}
	item := kubepugItem{Namespace: "web", ObjectName: "my-ingress"}
	assert.Equal(t, "web/Ingress/my-ingress", kubepugResourceKey(apiItem, item))

	// Empty namespace defaults to "default".
	item2 := kubepugItem{Namespace: "", ObjectName: "my-cronjob"}
	apiItem2 := kubepugResultItem{Kind: "CronJob"}
	assert.Equal(t, "default/CronJob/my-cronjob", kubepugResourceKey(apiItem2, item2))
}

func TestExtractJSON(t *testing.T) {
	// Normal JSON.
	assert.Equal(t, `{"a":1}`, string(extractJSON([]byte(`{"a":1}`))))

	// JSON with trailing error text (kubepug appends errors after JSON).
	assert.Equal(t, `{"a":1}`, string(extractJSON([]byte(`{"a":1}Error: something`))))

	// JSON with leading warnings.
	assert.Equal(t, `{"a":1}`, string(extractJSON([]byte(`warning: blah\n{"a":1}`))))

	// JSON with nested braces.
	input := `{"items":[{"name":"x"}]}`
	assert.Equal(t, input, string(extractJSON([]byte(input))))

	// No JSON.
	assert.Nil(t, extractJSON([]byte("no json here")))

	// JSON with strings containing braces.
	input2 := `{"msg":"hello {world}"}`
	assert.Equal(t, input2, string(extractJSON([]byte(input2+"trailing"))))
}
