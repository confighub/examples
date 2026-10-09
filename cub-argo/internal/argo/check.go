package argo

import "fmt"

// Check is one Application to compare with what ConfigHub holds for it.
type Check struct {
	Application string `json:"application"`
	Space       string `json:"space"`
	Unit        string `json:"unit"`
	// Namespace is the Application's destination, where an object whose
	// manifest names none lands.
	Namespace string `json:"namespace"`
	// Cluster is only for saying which one this is.
	Cluster string `json:"cluster,omitempty"`
	// Release names the release to compare against by its manifest digest;
	// empty is the newest published one.
	Release string `json:"release,omitempty"`
}

// ChecksFor is every Application a plan would govern, so `cub argo check`
// given the same input as `plan` needs no other argument. handover.sh checks
// exactly this set before it moves anything.
func ChecksFor(p *Plan) []Check {
	var out []Check
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				if v.Application == "" || v.Space == "" {
					continue
				}
				out = append(out, Check{
					Application: v.Application,
					Space:       v.Space,
					Unit:        c.Name,
					Namespace:   v.Namespace,
					Cluster:     v.Cluster,
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
	// Stale is every object Argo lists among the Application's resources that
	// the cluster says is not there: Argo's record is behind, so it is not a
	// record a handover can be checked against.
	Stale []string `json:"staleInventory,omitempty"`
	// Health is Argo CD's own word for the Application's health. It is part
	// of what --record claims, not of whether a handover would change the
	// cluster: Unhealthy, when set, makes the record a rejection, and
	// NotYet, when set, is why nothing is recorded yet.
	Health    string `json:"health,omitempty"`
	Unhealthy string `json:"unhealthy,omitempty"`
	NotYet    string `json:"notYet,omitempty"`
	// Recorded is the ID of the LiveCheck attestation --record wrote, and
	// Verdict what --record came to: pass, rejection, or none when nothing
	// was recorded yet.
	Recorded string `json:"recorded,omitempty"`
	Verdict  string `json:"verdict,omitempty"`
}

// judgeHealth makes health part of what a recorded check claims. Only Healthy
// can be a Pass. Degraded and Missing are wrong. Anything else, such as
// Progressing, is "not yet": it may still become Healthy, so it is neither a
// Pass nor a rejection.
func (r *Result) judgeHealth(health string) {
	r.Health = health
	switch health {
	case "Healthy":
	case "Degraded", "Missing":
		r.Unhealthy = fmt.Sprintf("Argo CD reports %s as %s", r.Check.Application, health)
	case "":
		r.NotYet = fmt.Sprintf("Argo CD reports no health for %s yet", r.Check.Application)
	default:
		r.NotYet = fmt.Sprintf("Argo CD reports %s as %s", r.Check.Application, health)
	}
}

// OK reports whether moving this Application's source would leave the cluster
// as it is.
func (r Result) OK() bool {
	return r.Inventory.OK() && len(r.Stale) == 0 && (r.Fields == nil || r.Fields.Clean())
}

// RunCheck compares one Application with what ConfigHub holds for it. With
// fields, it also compares every field the release sets. The Application is
// read with run, on the cluster Argo CD runs on; its objects are read with the
// runner workloads gives for the Application's destination, which may refuse.
func RunCheck(run Runner, hub Hub, c Check, fields bool, workloads func(Destination) (Runner, error)) (Result, error) {
	ns := checkNamespace
	live, err := LiveInventory(run, ns, c.Application)
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
	r.judgeHealth(live.Health)
	if fields {
		at, err := workloads(live.Destination)
		if err != nil {
			return Result{}, err
		}
		fc, err := CompareFields(at, c.Namespace, stored)
		if err != nil {
			return Result{}, err
		}
		r.Fields = &fc
		r.Stale = staleIn(live, fc.Absent)
	}
	return r, nil
}

// staleIn names the absent objects Argo still lists. An absent object Argo
// does not list is one the release would add, which the inventory comparison
// already says.
func staleIn(live Live, absent []Owned) []string {
	listed := map[string]bool{}
	for _, o := range live.Owned {
		listed[o.Key()] = true
	}
	var out []string
	for _, o := range absent {
		if listed[o.Key()] {
			out = append(out, fmt.Sprintf("%s: Argo lists it among the Application's resources, and the cluster does not have it, so that record is out of date", o))
		}
	}
	return out
}

// checkNamespace is where Argo CD's Applications live. It is a package
// variable so the command can set it once rather than thread it through every
// call, in the same way KubeContext is.
var checkNamespace = "argocd"

// SetApplicationNamespace names the namespace Argo CD's Applications live in.
func SetApplicationNamespace(ns string) {
	if ns != "" {
		checkNamespace = ns
	}
}
