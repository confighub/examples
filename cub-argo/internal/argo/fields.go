package argo

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

// FieldCheck is what comparing fields found, and how much of the release it
// could compare. An object the check could not read is not an object that
// matched, so a check that read less than the whole release is not clean.
type FieldCheck struct {
	Diffs []FieldDiff `json:"diffs,omitempty"`
	// Total is how many objects the release holds; Compared is how many were
	// read off the cluster and compared field by field.
	Total    int `json:"total"`
	Compared int `json:"compared"`
	// Unreadable is every object kubectl could not read for a reason other
	// than its absence: forbidden, timed out, an unknown kind. Nothing is
	// known about those fields, so they are neither clean nor drift.
	Unreadable []string `json:"unreadable,omitempty"`
	// Absent is every object the cluster positively said it does not have.
	Absent []Owned `json:"absent,omitempty"`
}

// Clean reports whether every object was read and none differs.
func (f FieldCheck) Clean() bool {
	return len(f.Diffs) == 0 && len(f.Unreadable) == 0 && len(f.Absent) == 0 && f.Compared == f.Total
}

// CompareFields reads each object the release holds off the cluster and
// compares the fields the release sets.
func CompareFields(run Runner, namespace string, release []byte) (FieldCheck, error) {
	var fc FieldCheck
	docs, err := documentsIn(release)
	if err != nil {
		return fc, err
	}
	for _, d := range docs {
		meta, _ := d["metadata"].(map[string]any)
		kind, name := str(d["kind"]), str(meta["name"])
		if kind == "" || name == "" {
			continue
		}
		fc.Total++
		ns := str(meta["namespace"])
		if ns == "" {
			ns = namespace
		}
		label := kind + " " + name
		if ns != "" {
			label = kind + " " + ns + "/" + name
		}
		// kubectl strips managedFields from -o json unless asked, and that is
		// the only record of who wrote each field.
		args := []string{"get", strings.ToLower(kind), name, "--show-managed-fields", "-o", "json"}
		if ns != "" {
			args = append([]string{"-n", ns}, args...)
		}
		raw, err := run("kubectl", args...)
		if err != nil {
			// Only the API server saying NotFound means the object is not
			// there. Forbidden, a timeout or a kind the server does not know
			// say nothing about it either way.
			if strings.Contains(err.Error(), "(NotFound)") {
				fc.Absent = append(fc.Absent, Owned{Group: groupOf(str(d["apiVersion"])), Kind: kind, Namespace: ns, Name: name})
			} else {
				fc.Unreadable = append(fc.Unreadable, label+": "+err.Error())
			}
			continue
		}
		var live map[string]any
		if err := json.Unmarshal(raw, &live); err != nil {
			fc.Unreadable = append(fc.Unreadable, label+": not JSON: "+err.Error())
			continue
		}
		fc.Compared++
		who := managersOf(live)
		for _, diff := range walkFields("", d, live) {
			diff.Object, diff.Managers = label, who
			fc.Diffs = append(fc.Diffs, diff)
		}
	}
	sort.Slice(fc.Diffs, func(i, j int) bool {
		if fc.Diffs[i].Object != fc.Diffs[j].Object {
			return fc.Diffs[i].Object < fc.Diffs[j].Object
		}
		return fc.Diffs[i].Path < fc.Diffs[j].Path
	})
	return fc, nil
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
