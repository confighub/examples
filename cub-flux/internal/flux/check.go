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
	Inventory InventoryComparison `json:"inventory"`
	Fields    []FieldDiff         `json:"fields,omitempty"`
}

// OK reports whether swapping this layer's source would leave the cluster as
// it is.
func (r Result) OK() bool { return r.Inventory.OK() && len(r.Fields) == 0 }

// RunCheck compares one layer with what ConfigHub holds for it. With fields,
// it also compares every field the release sets.
func RunCheck(run Runner, c Check, fields bool) (Result, error) {
	ns := checkNamespace
	live, err := LiveInventory(run, ns, c.Kustomization)
	if err != nil {
		return Result{}, err
	}
	stored, err := run("cub", "unit", "data", "--space", c.Space, c.Unit)
	if err != nil {
		return Result{}, fmt.Errorf("reading %s/%s from ConfigHub: %w", c.Space, c.Unit, err)
	}
	held, err := ObjectsIn(stored)
	if err != nil {
		return Result{}, err
	}
	r := Result{Check: c, Inventory: CompareInventory(live, held, c.Namespace)}
	if fields {
		r.Fields, err = CompareFields(run, c.Namespace, stored)
		if err != nil {
			return Result{}, err
		}
	}
	return r, nil
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
