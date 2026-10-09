package flux

import (
	"fmt"
	"strings"
)

// Check is one layer to compare with what ConfigHub holds for it.
type Check struct {
	Kustomization string `json:"kustomization"`
	Space         string `json:"space"`
	Unit          string `json:"unit"`
	// Namespace is the layer's targetNamespace, where an object whose
	// manifest names none lands.
	Namespace string `json:"namespace"`
	// Cluster is only for saying which one this is.
	Cluster string `json:"cluster,omitempty"`
	// Release names the release to compare against by its manifest digest;
	// empty is the newest published one.
	Release string `json:"release,omitempty"`
}

// ChecksFor is every layer a plan would govern, per cluster, so
// `cub flux check` given the same input as `plan` needs no other argument.
func ChecksFor(p *Plan) []Check {
	var out []Check
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				name := v.Kustomization
				if i := strings.Index(name, "/"); i >= 0 {
					name = name[i+1:]
				}
				if name == "" || v.Space == "" {
					continue
				}
				out = append(out, Check{
					Kustomization: name,
					Space:         v.Space,
					Unit:          c.Name,
					Namespace:     v.TargetNamespace,
					Cluster:       v.Cluster,
				})
			}
		}
	}
	return out
}

// Result is what one Check found.
type Result struct {
	Check     Check               `json:"check"`
	Release   Release             `json:"release"`
	Inventory InventoryComparison `json:"inventory"`
	Fields    *FieldCheck         `json:"fields,omitempty"`
	// Stale is every object the layer's inventory lists as applied that the
	// cluster says is not there: the controller's record is behind, so it is
	// not a record a handover can be checked against.
	Stale []string `json:"staleInventory,omitempty"`
	// Health is what Flux says of the layer: Healthy, or Unknown where Flux
	// was not asked to check the workloads, or Degraded, Progressing or
	// Suspended. It is part of what --record claims, not of whether a
	// handover would change the cluster: Unhealthy, when set, makes the
	// record a rejection, and NotYet, when set, is why nothing is recorded
	// yet.
	Health    string `json:"health,omitempty"`
	Unhealthy string `json:"unhealthy,omitempty"`
	NotYet    string `json:"notYet,omitempty"`
	// Recorded is the ID of the LiveCheck attestation --record wrote, and
	// Verdict what --record came to: pass, rejection, or none when nothing
	// was recorded yet.
	Recorded string `json:"recorded,omitempty"`
	Verdict  string `json:"verdict,omitempty"`
}

// OK reports whether swapping this layer's source would leave the cluster as
// it is.
func (r Result) OK() bool {
	return r.Inventory.OK() && len(r.Stale) == 0 && (r.Fields == nil || r.Fields.Clean())
}

// RunCheck compares one layer with what ConfigHub holds for it. With fields,
// it also compares every field the release sets.
func RunCheck(run Runner, hub Hub, c Check, fields bool) (Result, error) {
	ns := checkNamespace
	live, err := LiveInventory(run, ns, c.Kustomization)
	if err != nil {
		return Result{}, err
	}
	rel, stored, err := ReleasedData(hub, c.Space, c.Unit, c.Release)
	if err != nil {
		return Result{}, err
	}
	held, err := ObjectsIn(stored)
	if err != nil {
		return Result{}, err
	}
	r := Result{Check: c, Release: rel, Inventory: CompareInventory(live, held, c.Namespace)}
	r.Health, r.Unhealthy, r.NotYet = live.Health, live.Unhealthy, live.NotYet
	if fields {
		fc, err := CompareFields(run, c.Namespace, stored)
		if err != nil {
			return Result{}, err
		}
		r.Fields = &fc
		r.Stale = staleIn(live, fc.Absent)
	}
	return r, nil
}

// staleIn names the absent objects the inventory still lists. An absent object
// the inventory does not list is one the release would add, which the
// inventory comparison already says.
func staleIn(live Live, absent []Owned) []string {
	listed := map[string]bool{}
	for _, o := range live.Owned {
		listed[o.Key()] = true
	}
	var out []string
	for _, o := range absent {
		if listed[o.Key()] {
			out = append(out, fmt.Sprintf("%s: the layer's inventory lists it as applied, and the cluster does not have it, so that inventory is out of date", o))
		}
	}
	return out
}

// checkNamespace is where Flux's Kustomizations live. It is a package variable
// so the command can set it once rather than thread it through every call, in
// the same way KubeContext is.
var checkNamespace = "flux-system"

// SetControllerNamespace names the namespace Flux's Kustomizations live in.
func SetControllerNamespace(ns string) {
	if ns != "" {
		checkNamespace = ns
	}
}

// ChecksFromLayersSpace is every layer of a handed-over cluster, read from its
// layers Space: once the cluster's directory in Git holds only the root, the
// layers are defined there and nowhere else. Each Unit holds the layer's
// Kustomization and the OCIRepository naming its variant Space, whose Unit is
// named after the layer too.
func ChecksFromLayersSpace(hub Hub, cluster, space string) ([]Check, error) {
	units, err := hub.UnitSlugs(space)
	if err != nil {
		return nil, fmt.Errorf("listing the layers of %s in %s: %w", cluster, space, err)
	}
	var checks []Check
	for _, unit := range units {
		// The root reads the published release, so the layer is what that
		// release holds: a Unit's head may be ahead of it, and a Unit may not
		// be in it at all.
		_, data, err := ReleasedData(hub, space, unit, "latest")
		if err != nil {
			if strings.Contains(err.Error(), "does not hold") {
				continue
			}
			return nil, fmt.Errorf("reading layer %s of %s: %w", unit, cluster, err)
		}
		docs, err := documentsIn(data)
		if err != nil {
			return nil, fmt.Errorf("reading layer %s of %s: %w", unit, cluster, err)
		}
		c := Check{Unit: unit, Cluster: cluster}
		for _, d := range docs {
			switch str(d["kind"]) {
			case "Kustomization":
				c.Kustomization = str(get(d, "metadata", "name"))
				c.Namespace = str(get(d, "spec", "targetNamespace"))
			case "OCIRepository":
				url := str(get(d, "spec", "url"))
				if i := strings.Index(url, "/space/"); i >= 0 {
					c.Space = strings.TrimSuffix(url[i+len("/space/"):], "/")
				}
			}
		}
		if c.Kustomization == "" || c.Space == "" {
			return nil, fmt.Errorf("layer %s of %s in %s holds no Kustomization reading a variant Space", unit, cluster, space)
		}
		checks = append(checks, c)
	}
	if len(checks) == 0 {
		return nil, fmt.Errorf("%s holds no layers for %s", space, cluster)
	}
	return checks, nil
}
