package argo

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

const secretTypeLabel = "argocd.argoproj.io/secret-type"

// Cluster is an Argo CD cluster Secret, reduced to what generators select on.
// Its credentials (the Secret's config) are never read.
type Cluster struct {
	Name   string            `json:"name"`
	Server string            `json:"server"`
	Labels map[string]string `json:"labels"`
}

func clusterFrom(d Doc) (Cluster, bool) {
	v := d.Value
	if str(v["kind"]) != "Secret" {
		return Cluster{}, false
	}
	labels := strMap(get(v, "metadata", "labels"))
	if labels[secretTypeLabel] != "cluster" {
		return Cluster{}, false
	}
	return Cluster{
		Name:   secretField(v, "name", str(get(v, "metadata", "name"))),
		Server: secretField(v, "server", ""),
		Labels: labels,
	}, true
}

// secretField reads a field from stringData, then from base64 data.
func secretField(v map[string]any, key, fallback string) string {
	if s := str(get(v, "stringData", key)); s != "" {
		return s
	}
	if s := str(get(v, "data", key)); s != "" {
		if b, err := base64.StdEncoding.DecodeString(s); err == nil {
			return string(b)
		}
	}
	return fallback
}

// paramSet is one set of generator parameters: what one Application's
// template is rendered with.
type paramSet struct {
	values map[string]any
	// legacy holds the flat {{param}} names, for an ApplicationSet without
	// goTemplate. Argo's git generator exposes `path` as a string there and as
	// an object under goTemplate, so the two forms cannot be derived from one
	// another.
	legacy  map[string]string
	cluster *Cluster
	// element holds the parameters a list generator contributed, which name
	// the component when a list is crossed with clusters.
	element map[string]any
}

// expand runs one ApplicationSet generator offline. It returns the parameter
// sets and a short description of what the generator selects.
func expand(g map[string]any, clusters []Cluster, root string) ([]paramSet, string, error) {
	switch {
	case hasKey(g, "git"):
		return expandGit(obj(g["git"]), root)
	case hasKey(g, "clusters"):
		c := obj(g["clusters"])
		sel := obj(c["selector"])
		values := obj(c["values"])
		var out []paramSet
		for i := range clusters {
			ok, err := selects(sel, clusters[i].Labels)
			if err != nil {
				return nil, "", err
			}
			if ok {
				cl := clusters[i]
				out = append(out, paramSet{values: clusterParams(cl, values), cluster: &cl})
			}
		}
		// Argo CD's own cluster has no Secret, so no labels to match. With an
		// empty selector the generator includes it anyway, unless a Secret
		// for it is already among the clusters.
		if len(strMap(sel["matchLabels"])) == 0 && len(list(sel["matchExpressions"])) == 0 && !hasLocalCluster(clusters) {
			local := localCluster()
			out = append(out, paramSet{values: clusterParams(local, values), cluster: &local})
			sort.SliceStable(out, func(i, j int) bool { return out[i].cluster.Name < out[j].cluster.Name })
		}
		return out, "clusters where " + describeSelector(sel), nil
	case g["list"] != nil:
		var out []paramSet
		for _, e := range list(get(g, "list", "elements")) {
			el := obj(e)
			if el == nil {
				return nil, "", fmt.Errorf("list generator: an element is not a map")
			}
			out = append(out, paramSet{values: copyMap(el), element: el})
		}
		return out, fmt.Sprintf("a list of %d", len(out)), nil
	case g["matrix"] != nil:
		gens := list(get(g, "matrix", "generators"))
		if len(gens) != 2 {
			return nil, "", fmt.Errorf("matrix generator: needs exactly two generators, has %d", len(gens))
		}
		a, da, err := expand(obj(gens[0]), clusters, root)
		if err != nil {
			return nil, "", err
		}
		b, db, err := expand(obj(gens[1]), clusters, root)
		if err != nil {
			return nil, "", err
		}
		var out []paramSet
		for _, x := range a {
			for _, y := range b {
				p := paramSet{values: copyMap(x.values), cluster: x.cluster, element: x.element}
				for k, v := range y.values {
					p.values[k] = v
				}
				if y.cluster != nil {
					p.cluster = y.cluster
				}
				if y.element != nil {
					p.element = y.element
				}
				out = append(out, p)
			}
		}
		return out, da + " x " + db, nil
	}
	var kinds []string
	for k := range g {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return nil, "", fmt.Errorf("%s generator is not read offline yet", strings.Join(kinds, ", "))
}

func hasKey(m map[string]any, k string) bool {
	_, ok := m[k]
	return ok
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// localServer is how Argo CD addresses the cluster it runs on.
const localServer = "https://kubernetes.default.svc"

func localCluster() Cluster { return Cluster{Name: "in-cluster", Server: localServer} }

func hasLocalCluster(clusters []Cluster) bool {
	for _, c := range clusters {
		if c.Server == localServer {
			return true
		}
	}
	return false
}

// clusterParams are the parameters Argo CD's cluster generator provides.
func clusterParams(c Cluster, values map[string]any) map[string]any {
	labels := map[string]any{}
	for k, v := range c.Labels {
		labels[k] = v
	}
	p := map[string]any{
		"name":           c.Name,
		"nameNormalized": normalize(c.Name),
		"server":         c.Server,
		"metadata":       map[string]any{"labels": labels, "annotations": map[string]any{}},
	}
	if len(values) > 0 {
		p["values"] = values
	}
	return p
}

var notDNS = regexp.MustCompile(`[^a-z0-9.-]+`)

func normalize(name string) string {
	return notDNS.ReplaceAllString(strings.ToLower(name), "-")
}

// selects reports whether a label selector matches a cluster's labels.
func selects(sel map[string]any, labels map[string]string) (bool, error) {
	for k, v := range strMap(sel["matchLabels"]) {
		if labels[k] != v {
			return false, nil
		}
	}
	for _, e := range list(sel["matchExpressions"]) {
		ex := obj(e)
		key, op := str(ex["key"]), str(ex["operator"])
		val, has := labels[key]
		var in bool
		for _, v := range list(ex["values"]) {
			if str(v) == val {
				in = true
			}
		}
		switch op {
		case "In":
			if !has || !in {
				return false, nil
			}
		case "NotIn":
			if has && in {
				return false, nil
			}
		case "Exists":
			if !has {
				return false, nil
			}
		case "DoesNotExist":
			if has {
				return false, nil
			}
		default:
			return false, fmt.Errorf("selector operator %q is not supported", op)
		}
	}
	return true, nil
}

func describeSelector(sel map[string]any) string {
	var parts []string
	ml := strMap(sel["matchLabels"])
	keys := make([]string, 0, len(ml))
	for k := range ml {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k == secretTypeLabel {
			continue
		}
		parts = append(parts, k+"="+ml[k])
	}
	for _, e := range list(sel["matchExpressions"]) {
		ex := obj(e)
		var vals []string
		for _, v := range list(ex["values"]) {
			vals = append(vals, str(v))
		}
		key := str(ex["key"])
		switch str(ex["operator"]) {
		case "In":
			parts = append(parts, fmt.Sprintf("%s in (%s)", key, strings.Join(vals, ", ")))
		case "NotIn":
			parts = append(parts, fmt.Sprintf("%s not in (%s)", key, strings.Join(vals, ", ")))
		case "Exists":
			parts = append(parts, key+" is set")
		case "DoesNotExist":
			parts = append(parts, key+" is not set")
		}
	}
	if len(parts) == 0 {
		return "any cluster"
	}
	return strings.Join(parts, ", ")
}

// renderer renders an ApplicationSet template the way Argo CD does: Go
// templates when goTemplate is set, otherwise {{param}} substitution.
type renderer struct {
	goTemplate   bool
	missingError bool
}

func rendererFor(spec map[string]any) renderer {
	r := renderer{goTemplate: get(spec, "goTemplate") == true}
	for _, o := range list(spec["goTemplateOptions"]) {
		if str(o) == "missingkey=error" {
			r.missingError = true
		}
	}
	return r
}

// render returns a copy of v with every templated string rendered, and records
// each templated field's dotted path and rendered value in fields.
func (r renderer) render(v any, ps paramSet, path string, fields map[string]string) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			rv, err := r.render(val, ps, join(path, k), fields)
			if err != nil {
				return nil, err
			}
			out[k] = rv
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			rv, err := r.render(val, ps, fmt.Sprintf("%s[%d]", path, i), fields)
			if err != nil {
				return nil, err
			}
			out[i] = rv
		}
		return out, nil
	case string:
		if !strings.Contains(t, "{{") {
			return t, nil
		}
		s, err := r.str(t, ps)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		fields[path] = s
		return s, nil
	}
	return v, nil
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

var fastParam = regexp.MustCompile(`\{\{\s*([^{}\s]+)\s*\}\}`)

func (r renderer) str(s string, ps paramSet) (string, error) {
	if r.goTemplate {
		t := template.New("field")
		if r.missingError {
			t = t.Option("missingkey=error")
		}
		t, err := t.Parse(s)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		if err := t.Execute(&b, ps.values); err != nil {
			return "", err
		}
		return b.String(), nil
	}
	flat := map[string]string{}
	flatten("", ps.values, flat)
	for k, v := range ps.legacy {
		flat[k] = v
	}
	var missing []string
	out := fastParam.ReplaceAllStringFunc(s, func(m string) string {
		key := fastParam.FindStringSubmatch(m)[1]
		if v, ok := flat[key]; ok {
			return v
		}
		missing = append(missing, key)
		return m
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("no parameter %s", strings.Join(missing, ", "))
	}
	return out, nil
}

func flatten(prefix string, v any, out map[string]string) {
	if m := obj(v); m != nil {
		for k, val := range m {
			flatten(join(prefix, k), val, out)
		}
		return
	}
	out[prefix] = str(v)
}

// expandGit resolves a git generator's directories against the checkout. Argo
// resolves the same glob against the repository at the revision named, so with
// the checkout at that revision this gives the same directories, and the same
// Applications. Without a checkout it cannot be resolved, and says so.
//
// The `files` form reads config out of the files it matches, which is a
// different job; it is named rather than guessed at.
func expandGit(g map[string]any, root string) ([]paramSet, string, error) {
	if len(list(g["files"])) > 0 {
		return nil, "", fmt.Errorf("git generator reads parameters out of files, which is not read offline yet")
	}
	dirs := list(g["directories"])
	if len(dirs) == 0 {
		return nil, "", fmt.Errorf("git generator names neither directories nor files")
	}
	if root == "" {
		return nil, "", fmt.Errorf("git generator matches paths in the repository, so it needs the checkout: pass the repository directory, or --repo-root")
	}

	type entry struct {
		path    string
		exclude bool
	}
	var globs []entry
	for _, d := range dirs {
		globs = append(globs, entry{
			path:    strings.TrimPrefix(filepath.ToSlash(str(get(d, "path"))), "./"),
			exclude: get(d, "exclude") == true,
		})
	}

	matched := map[string]bool{}
	for _, e := range globs {
		hits, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(e.path)))
		if err != nil {
			return nil, "", fmt.Errorf("git generator path %q: %w", e.path, err)
		}
		for _, h := range hits {
			info, err := os.Stat(h)
			if err != nil || !info.IsDir() {
				continue
			}
			rel, err := filepath.Rel(root, h)
			if err != nil {
				continue
			}
			key := filepath.ToSlash(rel)
			if e.exclude {
				delete(matched, key)
			} else {
				matched[key] = true
			}
		}
	}

	var paths []string
	for k := range matched {
		paths = append(paths, k)
	}
	sort.Strings(paths)

	var out []paramSet
	for _, path := range paths {
		segments := strings.Split(path, "/")
		base := segments[len(segments)-1]
		legacy := map[string]string{
			"path":                    path,
			"path.basename":           base,
			"path.basenameNormalized": normalize(base),
		}
		for i, seg := range segments {
			legacy[fmt.Sprintf("path[%d]", i)] = seg
		}
		out = append(out, paramSet{
			values: map[string]any{"path": map[string]any{
				"path":               path,
				"basename":           base,
				"basenameNormalized": normalize(base),
				"segments":           toAny(segments),
			}},
			legacy: legacy,
		})
	}
	var names []string
	for _, e := range globs {
		if !e.exclude {
			names = append(names, e.path)
		}
	}
	return out, fmt.Sprintf("%d directories under %s", len(out), strings.Join(names, ", ")), nil
}

func toAny(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}
