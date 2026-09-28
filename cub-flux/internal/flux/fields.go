package flux

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// A field comparison answers the question an object-set comparison cannot:
// not "are the same objects there" but "would delivering this release change
// any of them". Only the fields the release actually sets are compared, which
// is how a reconciler decides too: Kubernetes fills in defaults the release
// never mentions, and a controller owns others, so comparing everything the
// cluster holds would report noise as drift.

// FieldDiff is one field the release would change on the cluster.
type FieldDiff struct {
	Object  string `json:"object"`
	Path    string `json:"path"`
	Live    string `json:"live"`
	Release string `json:"release"`
	// Managers are the field managers Kubernetes recorded for this object. A
	// human's kubectl among them is why a field can differ from a release
	// nobody changed. They are not ordered by time: kubectl records no
	// timestamp on some writes, so claiming which was last would be a guess.
	Managers []string `json:"managers,omitempty"`
}

func (d FieldDiff) String() string {
	s := fmt.Sprintf("%s %s: cluster has %s, the release holds %s", d.Object, d.Path, d.Live, d.Release)
	if len(d.Managers) > 0 {
		s += " (written on this object by " + strings.Join(d.Managers, ", ") + ")"
	}
	return s
}

// CompareFields reads each object the release holds off the cluster and
// compares the fields the release sets. An object the release holds that is
// not on the cluster is reported by CompareInventory, not here.
func CompareFields(run Runner, namespace string, release []byte) ([]FieldDiff, error) {
	docs, err := documentsIn(release)
	if err != nil {
		return nil, err
	}
	var out []FieldDiff
	for _, d := range docs {
		meta, _ := d["metadata"].(map[string]any)
		kind, name := str(d["kind"]), str(meta["name"])
		if kind == "" || name == "" {
			continue
		}
		ns := str(meta["namespace"])
		if ns == "" {
			ns = namespace
		}
		// kubectl strips managedFields from -o json unless asked, and that is
		// the only record of who wrote each field.
		args := []string{"get", strings.ToLower(kind), name, "--show-managed-fields", "-o", "json"}
		if ns != "" {
			args = append([]string{"-n", ns}, args...)
		}
		raw, err := run("kubectl", args...)
		if err != nil {
			// Absent objects are the inventory check's business, not this one.
			continue
		}
		var live map[string]any
		if err := json.Unmarshal(raw, &live); err != nil {
			return nil, fmt.Errorf("reading %s %s: %w", kind, name, err)
		}
		who := managersOf(live)
		label := kind + " " + name
		if ns != "" {
			label = kind + " " + ns + "/" + name
		}
		for _, diff := range walkFields("", d, live) {
			diff.Object, diff.Managers = label, who
			out = append(out, diff)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Object != out[j].Object {
			return out[i].Object < out[j].Object
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

// walkFields compares only what the release sets. metadata and status are left
// alone: a reconciler writes its own labels and annotations, and status is the
// cluster's to report.
func walkFields(path string, want, live any) []FieldDiff {
	if path == "" {
		w, _ := want.(map[string]any)
		l, _ := live.(map[string]any)
		var out []FieldDiff
		for _, k := range sortedAnyKeys(w) {
			if k == "metadata" || k == "status" || k == "apiVersion" || k == "kind" {
				continue
			}
			out = append(out, walkFields("."+k, w[k], l[k])...)
		}
		return out
	}
	switch w := want.(type) {
	case map[string]any:
		l, ok := live.(map[string]any)
		if !ok {
			return []FieldDiff{{Path: path, Live: "absent", Release: "a block of fields"}}
		}
		var out []FieldDiff
		for _, k := range sortedAnyKeys(w) {
			out = append(out, walkFields(path+"."+k, w[k], l[k])...)
		}
		return out
	case []any:
		l, ok := live.([]any)
		if !ok || len(l) != len(w) {
			return []FieldDiff{{Path: path, Live: describe(live), Release: fmt.Sprintf("a list of %d", len(w))}}
		}
		var out []FieldDiff
		for i := range w {
			out = append(out, walkFields(fmt.Sprintf("%s[%d]", path, i), w[i], l[i])...)
		}
		return out
	default:
		if describe(want) != describe(live) {
			return []FieldDiff{{Path: path, Live: describe(live), Release: describe(want)}}
		}
	}
	return nil
}

func describe(v any) string {
	if v == nil {
		return "absent"
	}
	switch t := v.(type) {
	case string:
		return fmt.Sprintf("%q", t)
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	case map[string]any:
		return "a block of fields"
	case []any:
		return fmt.Sprintf("a list of %d", len(t))
	}
	return fmt.Sprintf("%v", v)
}

func sortedAnyKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// managersOf reads Kubernetes' own record of who has written this object. It
// is what tells a controller's write from a person's. Entries carrying a time
// come first, newest first; entries without one keep their order, because
// kubectl leaves the time off some writes and inventing an order for those
// would put a name where a fact should be.
func managersOf(live map[string]any) []string {
	meta, _ := live["metadata"].(map[string]any)
	entries, _ := meta["managedFields"].([]any)
	type mf struct {
		name, at string
	}
	var all []mf
	for _, e := range entries {
		m, _ := e.(map[string]any)
		if n := str(m["manager"]); n != "" {
			all = append(all, mf{n, str(m["time"])})
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if (all[i].at == "") != (all[j].at == "") {
			return all[j].at == ""
		}
		return all[i].at > all[j].at
	})
	var out []string
	seen := map[string]bool{}
	for _, m := range all {
		if !seen[m.name] {
			seen[m.name] = true
			out = append(out, m.name)
		}
	}
	return out
}

// ByHand reports the managers that are a person at a terminal rather than a
// controller. A field that differs and was last written by one of these is a
// hand edit, which is the case no render-side check can see.
func ByHand(managers []string) []string {
	var out []string
	for _, m := range managers {
		if strings.HasPrefix(m, "kubectl") || strings.HasPrefix(m, "kubie") || m == "oc" {
			out = append(out, m)
		}
	}
	return out
}

func documentsIn(data []byte) ([]map[string]any, error) {
	docs, err := parse(data, "")
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, d := range docs {
		out = append(out, d.Value)
	}
	return out, nil
}
