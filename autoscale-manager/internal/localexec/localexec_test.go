// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package localexec

import (
	"context"
	"strings"
	"testing"
)

const hpaYAML = `apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: web
  namespace: shop
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: web
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 80
  - type: Resource
    resource:
      name: memory
      target:
        type: AverageValue
        averageValue: 512Mi
`

func TestConvertHPAToKEDA(t *testing.T) {
	out, changed, err := ConvertHPAToKEDA(context.Background(), []byte(hpaYAML))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if !changed {
		t.Fatalf("expected a mutation")
	}
	s := string(out)
	for _, want := range []string{
		"kind: ScaledObject",
		"apiVersion: keda.sh/v1alpha1",
		"name: web",
		"namespace: shop",
		"minReplicaCount: 2",
		"maxReplicaCount: 10",
		"type: cpu",
		"metricType: Utilization",
		"type: memory",
		"metricType: AverageValue",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q\n---\n%s", want, s)
		}
	}
	if strings.Contains(s, "HorizontalPodAutoscaler") {
		t.Errorf("HPA should have been replaced\n---\n%s", s)
	}
	// The cpu trigger keeps the utilization value; memory keeps the quantity.
	if !strings.Contains(s, `value: "80"`) || !strings.Contains(s, `value: "512Mi"`) {
		t.Errorf("trigger values not carried through\n---\n%s", s)
	}
}

// A non-HPA document is passed through unchanged.
func TestConvert_NonHPAUnchanged(t *testing.T) {
	cm := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c\ndata:\n  k: v\n"
	out, changed, err := ConvertHPAToKEDA(context.Background(), []byte(cm))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if changed {
		t.Errorf("a ConfigMap should not be changed")
	}
	if !strings.Contains(string(out), "kind: ConfigMap") {
		t.Errorf("ConfigMap should pass through: %s", out)
	}
}
