// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package convert

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"

	"github.com/confighub/examples/k8s-to-score/internal/k8s"
)

// workload is a Deployment or StatefulSet paired with the pod template the
// converter reads containers out of. Score models both as a Workload; the kind
// is carried forward as the k8s.score.dev/kind annotation so score-k8s renders
// the same shape back.
type workload struct {
	src      k8s.Object
	kind     string // "Deployment" or "StatefulSet"
	name     string
	ns       string
	labels   map[string]string // pod template labels, used to match Services
	pod      corev1.PodSpec
	replicas *int32
	// claimTemplates are a StatefulSet's volumeClaimTemplates, which become
	// score volume resources just like standalone PVCs do.
	claimTemplates []corev1.PersistentVolumeClaim
}

// inventory is everything decoded from a Space, indexed the way the converter
// needs to walk it: workloads to convert, plus the Services, Ingresses,
// ConfigMaps, Secrets and PVCs that get folded into them.
type inventory struct {
	workloads  []*workload
	services   []*corev1.Service
	ingresses  []*networkingv1.Ingress
	configMaps map[nsName]*corev1.ConfigMap
	secrets    map[nsName]*corev1.Secret
	pvcs       map[nsName]*corev1.PersistentVolumeClaim

	// unsupported records objects the converter deliberately does not map, so
	// the report can say what was left behind rather than dropping it silently.
	unsupported []k8s.Object
}

type nsName struct {
	ns   string
	name string
}

func key(ns, name string) nsName { return nsName{ns: ns, name: name} }

// buildInventory decodes every object in every Unit and files it by kind.
// Decode failures are surfaced as errors; unknown kinds are recorded, not
// fatal, so one CRD in a Space cannot block the whole conversion.
func buildInventory(objs []k8s.Object) (*inventory, error) {
	inv := &inventory{
		configMaps: map[nsName]*corev1.ConfigMap{},
		secrets:    map[nsName]*corev1.Secret{},
		pvcs:       map[nsName]*corev1.PersistentVolumeClaim{},
	}

	for _, o := range objs {
		switch o.Kind {
		case "Deployment":
			d, err := k8s.AsDeployment(o)
			if err != nil {
				return nil, err
			}
			inv.workloads = append(inv.workloads, &workload{
				src: o, kind: "Deployment", name: d.Name, ns: d.Namespace,
				labels: d.Spec.Template.Labels, pod: d.Spec.Template.Spec,
				replicas: d.Spec.Replicas,
			})
		case "StatefulSet":
			s, err := k8s.AsStatefulSet(o)
			if err != nil {
				return nil, err
			}
			inv.workloads = append(inv.workloads, &workload{
				src: o, kind: "StatefulSet", name: s.Name, ns: s.Namespace,
				labels: s.Spec.Template.Labels, pod: s.Spec.Template.Spec,
				replicas: s.Spec.Replicas, claimTemplates: s.Spec.VolumeClaimTemplates,
			})
		case "Service":
			s, err := k8s.AsService(o)
			if err != nil {
				return nil, err
			}
			inv.services = append(inv.services, s)
		case "Ingress":
			i, err := k8s.AsIngress(o)
			if err != nil {
				return nil, err
			}
			inv.ingresses = append(inv.ingresses, i)
		case "ConfigMap":
			c, err := k8s.AsConfigMap(o)
			if err != nil {
				return nil, err
			}
			inv.configMaps[key(c.Namespace, c.Name)] = c
		case "Secret":
			s, err := k8s.AsSecret(o)
			if err != nil {
				return nil, err
			}
			inv.secrets[key(s.Namespace, s.Name)] = s
		case "PersistentVolumeClaim":
			p, err := k8s.AsPVC(o)
			if err != nil {
				return nil, err
			}
			inv.pvcs[key(p.Namespace, p.Name)] = p
		default:
			inv.unsupported = append(inv.unsupported, o)
		}
	}
	return inv, nil
}

// servicesFor returns the Services whose selector matches a workload's pod
// labels. A Service with an empty selector (an ExternalName or a manually
// managed Endpoints Service) never matches: it fronts something other than
// these pods.
func (inv *inventory) servicesFor(w *workload) []*corev1.Service {
	var out []*corev1.Service
	for _, s := range inv.services {
		if s.Namespace != w.ns || len(s.Spec.Selector) == 0 {
			continue
		}
		if selectorMatches(s.Spec.Selector, w.labels) {
			out = append(out, s)
		}
	}
	return out
}

// workloadForService is the reverse lookup, used to attach an Ingress to the
// workload behind its backend Service.
func (inv *inventory) workloadForService(ns, name string) *workload {
	for _, s := range inv.services {
		if s.Namespace != ns || s.Name != name || len(s.Spec.Selector) == 0 {
			continue
		}
		for _, w := range inv.workloads {
			if w.ns == ns && selectorMatches(s.Spec.Selector, w.labels) {
				return w
			}
		}
	}
	return nil
}

// selectorMatches reports whether every selector label is present with the same
// value in the pod labels — the same subset rule the Kubernetes Service
// controller applies.
func selectorMatches(selector, podLabels map[string]string) bool {
	for k, v := range selector {
		if podLabels[k] != v {
			return false
		}
	}
	return true
}

// podVolume finds a named volume in a workload's pod spec.
func podVolume(w *workload, name string) *corev1.Volume {
	for i := range w.pod.Volumes {
		if w.pod.Volumes[i].Name == name {
			return &w.pod.Volumes[i]
		}
	}
	return nil
}

// claimTemplate finds a StatefulSet volumeClaimTemplate by name. A container
// volumeMount in a StatefulSet may reference one of these instead of a pod
// volume.
func claimTemplate(w *workload, name string) *corev1.PersistentVolumeClaim {
	for i := range w.claimTemplates {
		if w.claimTemplates[i].Name == name {
			return &w.claimTemplates[i]
		}
	}
	return nil
}
