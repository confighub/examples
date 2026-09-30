package argo

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Once an ApplicationSet is retired, each Application it made is delivered
// from ConfigHub: a Unit named after the variant's Space, holding the
// Application as Argo CD runs it now with its source pointed at that Space.
// The Unit goes in the control Space that holds the ApplicationSet, so the
// parent that already syncs the ApplicationSet from ConfigHub applies it, and
// the repoint is a published, reviewed change rather than a kubectl patch.
//
// It is the shape `cub variant create` gives a cub-cluster Argo target (one
// Application Unit per variant, named after its Space, in the Space the root
// reads), keeping what this estate already has: the Application's own name,
// project, destination and sync policy.

// PruneFalse is the sync option a delivered Application carries, so that no
// parent ever deletes it: not when its Unit is removed on the way back, and
// not when the parent is pointed back at Git, which does not hold it.
// Deleting an Application with the resources finalizer deletes its workloads.
const PruneFalse = "Prune=false"

// ReplaceTrue is the other: the parent replaces the Application with its Unit
// rather than merging the Unit into it. A merge keeps what the Unit leaves
// out, and an ApplicationSet's template can set source fields, such as
// kustomize.version, that would make Argo CD build the rendered bundle as a
// kustomization and fail. A replace is an update, so the UID stays; it also
// takes off the ApplicationSet's ownerReference, so removing the retired
// ApplicationSet later cannot garbage-collect the Application.
const ReplaceTrue = "Replace=true"

// DeliversAnnotation names the Space a delivered Application reads, on the
// object, where both ConfigHub and a person reading the cluster can see it.
const DeliversAnnotation = "argo.confighub.com/delivers"

// ApplicationUnit reads an Application from the cluster Argo CD runs on and
// returns the Unit that delivers it from space on the gateway.
func ApplicationUnit(run Runner, application, gateway, space string) ([]byte, error) {
	out, err := run("kubectl", "-n", checkNamespace, "get", "application", application, "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("reading Application %s: %w", application, err)
	}
	var live map[string]any
	if err := json.Unmarshal(out, &live); err != nil {
		return nil, fmt.Errorf("reading Application %s: %w", application, err)
	}
	u, err := applicationUnit(live, gateway, space)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s, delivered from ConfigHub: it reads the Space %s.\n", application, space))
	b.WriteString("# Made by `cub argo application-unit` from the Application as Argo CD ran it.\n")
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(u); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// applicationUnit keeps what a person wrote into the Application and drops
// what the cluster wrote: status, the operation in flight, identity and
// bookkeeping, and the owner, which is the ApplicationSet being retired. The
// source is the variant's Space and nothing else.
func applicationUnit(live map[string]any, gateway, space string) (map[string]any, error) {
	name := str(get(live, "metadata", "name"))
	if str(live["kind"]) != "Application" || name == "" {
		return nil, fmt.Errorf("not an Argo CD Application")
	}
	spec := obj(live["spec"])
	if spec == nil {
		return nil, fmt.Errorf("Application %s has no spec", name)
	}
	if _, multi := spec["sources"]; multi {
		// The gateway serves one rendered bundle per Space, so a
		// multi-source Application has no one source to replace.
		return nil, fmt.Errorf("Application %s has several sources; move it by hand", name)
	}
	if gateway == "" || space == "" {
		return nil, fmt.Errorf("need the gateway address and the Space")
	}
	gateway = strings.TrimPrefix(strings.TrimRight(gateway, "/"), "oci://")

	meta := map[string]any{"name": name}
	if ns := str(get(live, "metadata", "namespace")); ns != "" {
		meta["namespace"] = ns
	}
	if labels := obj(get(live, "metadata", "labels")); len(labels) > 0 {
		meta["labels"] = labels
	}
	ann := map[string]any{}
	for k, v := range obj(get(live, "metadata", "annotations")) {
		switch k {
		case "kubectl.kubernetes.io/last-applied-configuration",
			"argocd.argoproj.io/refresh",
			"argocd.argoproj.io/tracking-id",
			"argocd.argoproj.io/installation-id":
			continue
		}
		ann[k] = v
	}
	ann["argocd.argoproj.io/sync-options"] = withOption(withOption(str(ann["argocd.argoproj.io/sync-options"]), PruneFalse), ReplaceTrue)
	ann[DeliversAnnotation] = space
	meta["annotations"] = ann
	// Kept, because a replace removes whatever the Unit leaves out: the
	// finalizer is what makes deleting the Application delete its workloads,
	// and that is the estate's choice, not this move's.
	if fin, ok := get(live, "metadata", "finalizers").([]any); ok && len(fin) > 0 {
		meta["finalizers"] = fin
	}

	out := map[string]any{}
	for k, v := range spec {
		out[k] = v
	}
	out["source"] = map[string]any{
		"repoURL":        "oci://" + gateway + "/space/" + space,
		"path":           ".",
		"targetRevision": "latest",
	}
	return map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   meta,
		"spec":       out,
	}, nil
}

// withOption adds opt to a comma-separated sync-options value, once.
func withOption(have, opt string) string {
	var opts []string
	seen := map[string]bool{}
	for _, o := range strings.Split(have, ",") {
		if o = strings.TrimSpace(o); o != "" && !seen[o] {
			seen[o] = true
			opts = append(opts, o)
		}
	}
	if !seen[opt] {
		opts = append(opts, opt)
	}
	sort.Strings(opts)
	return strings.Join(opts, ",")
}

// ApplicationUnitLike makes the Unit for an Application that is not on the
// cluster yet, from a sibling's Unit: a cluster that joined after its
// ApplicationSet was retired, so nothing generated its Application. The
// sibling is the same component on another cluster, delivered already; the
// name, destination and labels are the new cluster's, as the plan works them
// out from the ApplicationSet's template.
func ApplicationUnitLike(run Runner, likeSpace, likeUnit, application, server, destNamespace string, labels map[string]string, gateway, space string) ([]byte, error) {
	out, err := run("cub", "unit", "data", "--space", likeSpace, likeUnit)
	if err != nil {
		return nil, fmt.Errorf("reading %s/%s, the Unit to make %s like: %w", likeSpace, likeUnit, application, err)
	}
	var like map[string]any
	if err := yaml.Unmarshal(out, &like); err != nil {
		return nil, fmt.Errorf("reading %s/%s: %w", likeSpace, likeUnit, err)
	}
	if str(like["kind"]) != "Application" {
		return nil, fmt.Errorf("%s/%s does not hold an Application", likeSpace, likeUnit)
	}
	meta := obj(like["metadata"])
	meta["name"] = application
	if len(labels) > 0 {
		l := obj(meta["labels"])
		if l == nil {
			l = map[string]any{}
		}
		for k, v := range labels {
			l[k] = v
		}
		meta["labels"] = l
	}
	spec := obj(like["spec"])
	dest := map[string]any{"server": server}
	if destNamespace != "" {
		dest["namespace"] = destNamespace
	}
	spec["destination"] = dest
	u, err := applicationUnit(like, gateway, space)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s, delivered from ConfigHub: it reads the Space %s.\n", application, space))
	b.WriteString(fmt.Sprintf("# Made by `cub argo application-unit` like %s/%s, for a cluster that joined after\n# its ApplicationSet was retired.\n", likeSpace, likeUnit))
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(u); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}
