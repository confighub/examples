package argo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Runner runs a command and returns what it printed. Tests stand in for
// kubectl, cub and cub-scout with one.
type Runner func(name string, args ...string) ([]byte, error)

// KubeContext, when set, is the kubectl context every kubectl call here uses.
// Without it kubectl uses whatever context happens to be current, which during
// a handover is very likely the wrong cluster.
var KubeContext string

// Run runs a command on this machine.
func Run(name string, args ...string) ([]byte, error) {
	if name == "kubectl" && KubeContext != "" {
		args = append([]string{"--context", KubeContext}, args...)
	}
	cmd := exec.Command(name, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		text := strings.TrimSpace(stderr.String())
		if i := strings.LastIndex(text, "\n"); i >= 0 {
			text = text[i+1:]
		}
		return nil, fmt.Errorf("%s %s: %s", name, strings.Join(args[:min(len(args), 3)], " "), text)
	}
	return out, nil
}

// Owned is one object a controller says it manages, or one object a release
// holds. The two are compared by Key.
type Owned struct {
	Group, Kind, Namespace, Name string
	// Pruning is Argo's own answer to what would be deleted if the desired
	// state stopped holding this object.
	Pruning bool
	// Hook marks an object Argo runs as a sync hook rather than holding as
	// part of the desired state.
	Hook bool
}

func (o Owned) Key() string {
	return o.Group + "|" + o.Kind + "|" + o.Namespace + "|" + o.Name
}

func (o Owned) String() string {
	if o.Namespace == "" {
		return o.Kind + " " + o.Name
	}
	return o.Kind + " " + o.Namespace + "/" + o.Name
}

// Live is what Argo CD says about an Application right now.
type Live struct {
	Owned []Owned
	// Prunes is spec.syncPolicy.automated.prune. It decides what happens to an
	// object Argo owns that a new source does not hold, so it is the signal a
	// handover turns on. The per-resource requiresPruning flag describes the
	// state today, where nothing is out of sync, and is absent in that case.
	Prunes bool
}

// LiveInventory reads what Argo CD says an Application owns right now. This is
// the Application's own record, not an inference from labels: status.resources
// is what Argo will reconcile.
func LiveInventory(run Runner, namespace, app string) (Live, error) {
	out, err := run("kubectl", "-n", namespace, "get", "application", app, "-o", "json")
	if err != nil {
		return Live{}, fmt.Errorf("reading what Argo says %s owns: %w", app, err)
	}
	var a struct {
		Spec struct {
			SyncPolicy struct {
				Automated *struct {
					Prune bool `json:"prune"`
				} `json:"automated"`
			} `json:"syncPolicy"`
		} `json:"spec"`
		Status struct {
			Resources []struct {
				Group           string `json:"group"`
				Kind            string `json:"kind"`
				Namespace       string `json:"namespace"`
				Name            string `json:"name"`
				Hook            bool   `json:"hook"`
				RequiresPruning bool   `json:"requiresPruning"`
			} `json:"resources"`
		} `json:"status"`
	}
	if err := json.Unmarshal(out, &a); err != nil {
		return Live{}, fmt.Errorf("reading Application %s: %w", app, err)
	}
	if len(a.Status.Resources) == 0 {
		return Live{}, fmt.Errorf("Application %s reports owning nothing. It may not have synced yet; a handover cannot be checked against an empty inventory", app)
	}
	l := Live{Prunes: a.Spec.SyncPolicy.Automated != nil && a.Spec.SyncPolicy.Automated.Prune}
	for _, r := range a.Status.Resources {
		l.Owned = append(l.Owned, Owned{Group: r.Group, Kind: r.Kind, Namespace: r.Namespace, Name: r.Name, Hook: r.Hook, Pruning: r.RequiresPruning})
	}
	return l, nil
}

// ObjectsIn reads the object set out of rendered YAML: what a release would
// hold. Group is taken from apiVersion, so it matches what a controller
// reports.
func ObjectsIn(data []byte) ([]Owned, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var out []Owned
	for {
		var v map[string]any
		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		meta, _ := v["metadata"].(map[string]any)
		o := Owned{
			Group:     groupOf(str(v["apiVersion"])),
			Kind:      str(v["kind"]),
			Namespace: str(meta["namespace"]),
			Name:      str(meta["name"]),
		}
		if o.Kind == "" || o.Name == "" {
			continue
		}
		out = append(out, o)
	}
}

// groupOf takes the group from an apiVersion: apps/v1 is apps, v1 is "".
func groupOf(apiVersion string) string {
	if i := strings.Index(apiVersion, "/"); i >= 0 {
		return apiVersion[:i]
	}
	return ""
}

// InventoryComparison is how what a controller owns compares with what a
// release holds. It answers the question a render-side check cannot: would
// moving this Application's source change the cluster?
type InventoryComparison struct {
	Same       int      `json:"same"`
	WouldPrune []string `json:"wouldPrune,omitempty"`
	WouldAdd   []string `json:"wouldAdd,omitempty"`
	Notes      []string `json:"notes,omitempty"`
}

// OK reports whether the handover would leave the cluster as it is.
func (c InventoryComparison) OK() bool { return len(c.WouldPrune) == 0 && len(c.WouldAdd) == 0 }

// CompareInventory compares what the controller owns with what the release
// holds. A namespace the release leaves to the destination matches an object
// the controller reports in that destination namespace, because that is where
// it landed.
func CompareInventory(live Live, stored []Owned, destNamespace string) InventoryComparison {
	owned := live.Owned
	var c InventoryComparison

	// One stored object can be found under two keys: as written, and with the
	// destination namespace filled in, because that is where it lands. Both
	// keys point at the same object, so matching either marks it used.
	at := map[string]int{}
	for i, o := range stored {
		at[o.Key()] = i
		if o.Namespace == "" && destNamespace != "" {
			n := o
			n.Namespace = destNamespace
			at[n.Key()] = i
		}
	}
	used := make([]bool, len(stored))

	for _, o := range owned {
		if o.Hook {
			c.Notes = append(c.Notes, o.String()+" is a sync hook, which Argo runs rather than holds, so it is not compared")
			continue
		}
		if i, ok := at[o.Key()]; ok {
			used[i] = true
			c.Same++
			continue
		}
		what := "so it would be left on the cluster, managed by nothing"
		if live.Prunes || o.Pruning {
			what = "and this Application prunes, so it would be DELETED from the cluster"
		}
		c.WouldPrune = append(c.WouldPrune, fmt.Sprintf("%s: Argo owns it and the release does not hold it, %s", o, what))
	}

	for i, s := range stored {
		if used[i] {
			continue
		}
		c.WouldAdd = append(c.WouldAdd, fmt.Sprintf("%s: the release holds it and Argo does not own it, so it would be added to the cluster", s))
	}
	sort.Strings(c.WouldPrune)
	sort.Strings(c.WouldAdd)
	sort.Strings(c.Notes)
	return c
}

// Unclaimed asks cub-scout what on this cluster no controller claims. It is a
// cross-check rather than the gate: cub-scout infers ownership, where
// status.resources is Argo's own record. Absent or failing cub-scout is not a
// failure, because nothing here depends on it.
func Unclaimed(run Runner, namespace string) ([]string, error) {
	out, err := run("cub", "scout", "map", "list", "--json", "-q", "owner=Native AND namespace="+namespace)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Kind, Namespace, Name string
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, err
	}
	var names []string
	for _, r := range rows {
		names = append(names, r.Kind+" "+r.Namespace+"/"+r.Name)
	}
	sort.Strings(names)
	return names, nil
}
