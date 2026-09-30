package flux

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Each cluster gets one ConfigHub Space holding what Flux runs there: one Unit
// per layer, released to that cluster's Target. A single root on the cluster
// reads that Space. It is the Flux shape of what `cub cluster up` makes for
// Argo CD (an apps Space and a root Application), and a Unit per layer is the
// Flux shape of the Application `cub variant create` adds for each variant.
//
//	flux-system (Git)            the Flux controllers, and the root below
//	  confighub-root             reads <prefix>-<cluster>-layers from ConfigHub
//	    layer Kustomization      reads its variant Space, <prefix>-<layer>-<cluster>
//
// A layer's delivery Unit holds the layer's own Kustomization, as Git defines
// it, with only its source swapped, and the OCIRepository for that source.
// Adding a cluster adds a Space and its Units; changing how a layer is
// reconciled is a reviewed change to its Unit.

// DeliverySpace names the Space holding one cluster's layers.
func DeliverySpace(prefix, cluster string) string {
	return prefix + "-" + cluster + "-layers"
}

// RootName is the root Kustomization, and its OCIRepository, on each cluster.
const RootName = "confighub-root"

// The gateway address and whether it is plain HTTP are the cluster's view of
// ConfigHub, not the plan's, so the Unit is written with these markers and
// the scripts fill them in.
const (
	gatewayMarker  = "__CONFIGHUB_OCI__"
	insecureMarker = "__CONFIGHUB_OCI_INSECURE__"
)

// DeliveryUnit is the Unit for one layer on one cluster.
func DeliveryUnit(v *Variant, prefix string) (string, error) {
	if v.layer == nil {
		return "", fmt.Errorf("%s on %s: no Kustomization to deliver", v.Space, v.Cluster)
	}
	name := str(get(v.layer, "metadata", "name"))
	ns := str(get(v.layer, "metadata", "namespace"))
	if ns == "" {
		ns = "flux-system"
	}
	src := map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "OCIRepository",
		"metadata":   map[string]any{"name": name, "namespace": ns},
		"spec": map[string]any{
			"interval":  "1m",
			"url":       "oci://" + gatewayMarker + "/space/" + v.Space,
			"ref":       map[string]any{"tag": "latest"},
			"secretRef": map[string]any{"name": "confighub-" + prefix + "-targets"},
			"insecure":  insecureMarker,
		},
	}
	k := deepCopy(v.layer).(map[string]any)
	delete(k, "status")
	meta := obj(k["metadata"])
	for _, f := range []string{"uid", "resourceVersion", "generation", "creationTimestamp", "managedFields"} {
		delete(meta, f)
	}
	spec := obj(k["spec"])
	// The same two fields the kubectl handover moves, for the same reason:
	// path is inside the artifact, and a ConfigHub artifact is the rendered
	// objects at its root.
	spec["sourceRef"] = map[string]any{"kind": "OCIRepository", "name": name}
	spec["path"] = "./"
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s on %s: what Flux runs for this layer, reading %s.\n", name, v.Cluster, v.Space))
	for i, d := range []map[string]any{src, k} {
		if i > 0 {
			b.WriteString("---\n")
		}
		enc := yaml.NewEncoder(&b)
		enc.SetIndent(2)
		if err := enc.Encode(d); err != nil {
			return "", err
		}
		if err := enc.Close(); err != nil {
			return "", err
		}
	}
	// The markers are plain scalars, so the scripts' replacement leaves insecure
	// a boolean.
	return b.String(), nil
}

// RootManifests is what goes on the cluster once, into flux-system: the root
// source and Kustomization reading the cluster's layers Space. The credential
// is a Secret made beside it, never a Unit.
func RootManifests(prefix, cluster string) string {
	return fmt.Sprintf(`apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: %[1]s
  namespace: flux-system
spec:
  interval: 1m
  url: oci://%[2]s/space/%[3]s
  ref:
    tag: latest
  secretRef:
    name: confighub-%[4]s-targets
  insecure: %[5]s
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: %[1]s
  namespace: flux-system
spec:
  interval: 10m
  path: ./
  prune: true
  sourceRef:
    kind: OCIRepository
    name: %[1]s
`, RootName, gatewayMarker, DeliverySpace(prefix, cluster), prefix, insecureMarker)
}

func deepCopy(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, x := range t {
			m[k] = deepCopy(x)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i, x := range t {
			s[i] = deepCopy(x)
		}
		return s
	}
	return v
}
