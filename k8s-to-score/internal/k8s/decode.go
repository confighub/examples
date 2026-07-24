// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package k8s decodes the Kubernetes YAML held in ConfigHub Unit data into the
// typed objects the converter works with. A Unit normally holds exactly one
// resource (ConfigHub's one-resource-per-Unit doctrine), but the decoder
// accepts multi-document YAML so that coarser Units — the kind a Helm or
// Kustomize import can leave behind — convert too.
package k8s

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// Object is one decoded Kubernetes resource, carrying the Unit it came from so
// that warnings can name a Unit rather than an anonymous manifest.
type Object struct {
	// UnitSlug is the slug of the ConfigHub Unit the resource was read from.
	UnitSlug string
	// Kind and APIVersion are the resource's own type identity.
	Kind       string
	APIVersion string
	// Name is metadata.name.
	Name string
	// Namespace is metadata.namespace, empty if unset.
	Namespace string
	// Raw is the single-document YAML the resource was decoded from, kept so
	// callers can re-decode into a type the decoder does not know about.
	Raw []byte
}

// Decode splits multi-document YAML into one Object per document. Documents
// that are empty or that carry no kind are skipped rather than failing the
// whole Unit — Units imported from generators often end with a trailing `---`.
func Decode(unitSlug string, data []byte) ([]Object, error) {
	var objs []Object
	reader := k8syaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(data)))
	for i := 0; ; i++ {
		doc, err := reader.Read()
		if err == io.EOF {
			return objs, nil
		}
		if err != nil {
			return nil, fmt.Errorf("unit %s: document %d: %w", unitSlug, i, err)
		}
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		var tm struct {
			metav1.TypeMeta   `json:",inline"`
			metav1.ObjectMeta `json:"metadata,omitempty"`
		}
		if err := yaml.Unmarshal(doc, &tm); err != nil {
			return nil, fmt.Errorf("unit %s: document %d: %w", unitSlug, i, err)
		}
		if tm.Kind == "" {
			continue
		}
		objs = append(objs, Object{
			UnitSlug:   unitSlug,
			Kind:       tm.Kind,
			APIVersion: tm.APIVersion,
			Name:       tm.Name,
			Namespace:  tm.Namespace,
			Raw:        doc,
		})
	}
}

// As decodes an Object's raw YAML into a concrete Kubernetes type.
func As[T any](o Object) (*T, error) {
	var out T
	if err := yaml.Unmarshal(o.Raw, &out); err != nil {
		return nil, fmt.Errorf("unit %s: %s/%s: %w", o.UnitSlug, o.Kind, o.Name, err)
	}
	return &out, nil
}

// Typed decoders for the kinds the converter understands. Each returns the
// resource plus the Object it came from, so warnings stay traceable to a Unit.

func AsDeployment(o Object) (*appsv1.Deployment, error) { return As[appsv1.Deployment](o) }

func AsStatefulSet(o Object) (*appsv1.StatefulSet, error) { return As[appsv1.StatefulSet](o) }

func AsDaemonSet(o Object) (*appsv1.DaemonSet, error) { return As[appsv1.DaemonSet](o) }

func AsCronJob(o Object) (*batchv1.CronJob, error) { return As[batchv1.CronJob](o) }

func AsService(o Object) (*corev1.Service, error) { return As[corev1.Service](o) }

func AsConfigMap(o Object) (*corev1.ConfigMap, error) { return As[corev1.ConfigMap](o) }

func AsSecret(o Object) (*corev1.Secret, error) { return As[corev1.Secret](o) }

func AsPVC(o Object) (*corev1.PersistentVolumeClaim, error) {
	return As[corev1.PersistentVolumeClaim](o)
}

func AsIngress(o Object) (*networkingv1.Ingress, error) { return As[networkingv1.Ingress](o) }
