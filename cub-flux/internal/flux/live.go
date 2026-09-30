package flux

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

// Owned is one object a layer applied, or one object a release holds. The two
// are compared by Key.
type Owned struct {
	Group, Kind, Namespace, Name string
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

// Live is what a Flux Kustomization says about itself right now.
type Live struct {
	Owned []Owned
	// Prunes is spec.prune, which the CRD requires, so it is always an
	// explicit choice and can be false. It decides whether an object the layer
	// applied and a new source does not hold is deleted or merely left behind.
	Prunes bool
}

// LiveInventory reads what a Flux Kustomization says it applied. This is the
// controller's own record: status.inventory is what it garbage-collects from,
// so it is what a handover has to match.
func LiveInventory(run Runner, namespace, name string) (Live, error) {
	out, err := run("kubectl", "-n", namespace, "get", "kustomization", name, "-o", "json")
	if err != nil {
		return Live{}, fmt.Errorf("reading what %s applied: %w", name, err)
	}
	var k struct {
		Spec struct {
			Prune bool `json:"prune"`
		} `json:"spec"`
		Status struct {
			Inventory struct {
				Entries []struct {
					ID string `json:"id"`
					V  string `json:"v"`
				} `json:"entries"`
			} `json:"inventory"`
		} `json:"status"`
	}
	if err := json.Unmarshal(out, &k); err != nil {
		return Live{}, fmt.Errorf("reading Kustomization %s: %w", name, err)
	}
	entries := k.Status.Inventory.Entries
	if len(entries) == 0 {
		return Live{}, fmt.Errorf("Kustomization %s reports an empty inventory. It may not have reconciled yet, and a handover cannot be checked against nothing", name)
	}
	l := Live{Prunes: k.Spec.Prune}
	for _, e := range entries {
		o, err := parseID(e.ID)
		if err != nil {
			return Live{}, err
		}
		l.Owned = append(l.Owned, o)
	}
	return l, nil
}

// parseID reads Flux's inventory id, which is
// "<namespace>_<name>_<group>_<kind>" with an empty namespace for a
// cluster-scoped object and an empty group for the core one.
func parseID(id string) (Owned, error) {
	parts := strings.Split(id, "_")
	if len(parts) != 4 {
		return Owned{}, fmt.Errorf("inventory entry %q is not <namespace>_<name>_<group>_<kind>", id)
	}
	return Owned{Namespace: parts[0], Name: parts[1], Group: parts[2], Kind: parts[3]}, nil
}

// ObjectsIn reads the object set out of rendered YAML: what a release would
// hold.
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

func groupOf(apiVersion string) string {
	if i := strings.Index(apiVersion, "/"); i >= 0 {
		return apiVersion[:i]
	}
	return ""
}

// InventoryComparison is how what a layer applied compares with what a release
// holds. It answers what a render-side check cannot: would swapping this
// layer's source change the cluster?
type InventoryComparison struct {
	Same       int      `json:"same"`
	WouldPrune []string `json:"wouldPrune,omitempty"`
	WouldAdd   []string `json:"wouldAdd,omitempty"`
}

// OK reports whether the swap would leave the cluster as it is.
func (c InventoryComparison) OK() bool { return len(c.WouldPrune) == 0 && len(c.WouldAdd) == 0 }

// CompareInventory compares what the layer applied with what the release
// holds. targetNamespace, when the Kustomization sets one, is where an object
// without a namespace of its own lands.
func CompareInventory(live Live, stored []Owned, targetNamespace string) InventoryComparison {
	owned := live.Owned
	var c InventoryComparison
	at := map[string]int{}
	for i, o := range stored {
		at[o.Key()] = i
		if o.Namespace == "" && targetNamespace != "" {
			n := o
			n.Namespace = targetNamespace
			at[n.Key()] = i
		}
	}
	used := make([]bool, len(stored))
	for _, o := range owned {
		if i, ok := at[o.Key()]; ok {
			used[i] = true
			c.Same++
			continue
		}
		what := "so it would be left on the cluster, managed by nothing"
		if live.Prunes {
			what = "and this layer prunes, so Flux would DELETE it from the cluster"
		}
		c.WouldPrune = append(c.WouldPrune,
			fmt.Sprintf("%s: the layer applied it and the release does not hold it, %s", o, what))
	}
	for i, s := range stored {
		if used[i] {
			continue
		}
		c.WouldAdd = append(c.WouldAdd,
			fmt.Sprintf("%s: the release holds it and the layer has not applied it, so it would be added to the cluster", s))
	}
	sort.Strings(c.WouldPrune)
	sort.Strings(c.WouldAdd)
	return c
}

// CubWriter sets a Space's live status with `cub space update --patch`, which
// merges the body into the Space rather than replacing it.
func CubWriter(space string, patch []byte) error {
	cmd := exec.Command("cub", "space", "update", "--patch", space, "--from-stdin", "--quiet")
	cmd.Stdin = bytes.NewReader(patch)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cub space update %s: %s", space, strings.TrimSpace(stderr.String()))
	}
	return nil
}
