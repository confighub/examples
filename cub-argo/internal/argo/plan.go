package argo

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Options shape the plan.
type Options struct {
	// Prefix starts the name of everything the plan would create in ConfigHub.
	Prefix string
	// StageLabel is the cluster label whose value decides a cluster's stage.
	StageLabel string
	// Stages is the stage order, by value of StageLabel.
	Stages []string
	// RepoRoot is the checkout that Applications' source paths are relative
	// to. When empty it is found by walking up from a directory input to .git.
	RepoRoot string
}

// Plan is what ConfigHub would hold for an Argo CD estate. Building it
// changes nothing.
type Plan struct {
	Inputs     Inputs       `json:"inputs"`
	Clusters   []Cluster    `json:"clusters"`
	Tree       []*Node      `json:"controlTree,omitempty"`
	StageLabel string       `json:"stageLabel,omitempty"`
	Stages     []string     `json:"stages"`
	Components []*Component `json:"components"`
	Windows    []Window     `json:"syncWindows,omitempty"`
	Unselected []string     `json:"unselectedClusters,omitempty"`
	Live       []string     `json:"liveApplications,omitempty"`
	Handover   []string     `json:"handover,omitempty"`
	LeftOut    []string     `json:"leftOut,omitempty"`
	Problems   []string     `json:"problems,omitempty"`
}

// Inputs says what the plan read.
type Inputs struct {
	Objects  int      `json:"objects"`
	Skipped  []string `json:"skipped,omitempty"`
	RepoRoot string   `json:"repoRoot,omitempty"`
}

// Node is one Argo CD control object in the tree the root Application grows.
// The tree stays as it is; it is the management record.
type Node struct {
	Kind     string  `json:"kind"`
	Name     string  `json:"name"`
	Role     string  `json:"role"`
	Wave     int     `json:"wave"`
	File     string  `json:"file,omitempty"`
	Children []*Node `json:"children,omitempty"`
}

// Component is one base and its variants: an ApplicationSet (or one list
// element of it), or a standalone Application.
type Component struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Source     string   `json:"source"`
	Owner      string   `json:"owner,omitempty"`
	Project    string   `json:"project"`
	Generator  string   `json:"generator,omitempty"`
	Wave       int      `json:"wave"`
	Base       string   `json:"base"`
	Departures []string `json:"departures"`
	Stages     []Stage  `json:"stages"`
	InFlight   []string `json:"inFlight,omitempty"`
	Notes      []string `json:"notes,omitempty"`
}

// Stage is the variants a change reaches together.
type Stage struct {
	Name     string    `json:"name"`
	Variants []Variant `json:"variants"`
}

// Variant is one cluster's copy of the base: one Application, kept under the
// name Argo CD already gave it so its tracking ID does not change.
type Variant struct {
	Cluster     string            `json:"cluster"`
	Space       string            `json:"space"`
	Target      string            `json:"target"`
	Application string            `json:"application"`
	Namespace   string            `json:"namespace"`
	Path        string            `json:"path"`
	Images      map[string]string `json:"images,omitempty"`
	stage       string
}

// Window is an AppProject sync window and the planned Applications it covers.
type Window struct {
	Project      string   `json:"project"`
	Kind         string   `json:"kind"`
	Schedule     string   `json:"schedule"`
	Duration     string   `json:"duration"`
	TimeZone     string   `json:"timeZone,omitempty"`
	ManualSync   bool     `json:"manualSync"`
	Applications []string `json:"applications"`
}

type object struct {
	kind, name, namespace, file string
	wave                        int
	value                       map[string]any
}

func objectFrom(d Doc) object {
	v := d.Value
	wave, _ := strconv.Atoi(str(get(v, "metadata", "annotations", "argocd.argoproj.io/sync-wave")))
	return object{
		kind:      str(v["kind"]),
		name:      str(get(v, "metadata", "name")),
		namespace: str(get(v, "metadata", "namespace")),
		file:      d.File,
		wave:      wave,
		value:     v,
	}
}

func (o object) spec() map[string]any { return obj(o.value["spec"]) }

// generatedBy names the ApplicationSet that owns a live Application.
func (o object) generatedBy() string {
	for _, r := range list(get(o.value, "metadata", "ownerReferences")) {
		if str(get(r, "kind")) == "ApplicationSet" {
			return str(get(r, "name"))
		}
	}
	return ""
}

type builder struct {
	opts     Options
	root     string
	plan     *Plan
	clusters []Cluster
	stageOf  map[string]string
}

// Build plans an Argo CD estate. It reads nothing but the input and, when a
// repository checkout is known, the files Applications point at.
func Build(in *Input, opts Options) (*Plan, error) {
	if opts.Prefix == "" {
		opts.Prefix = "argo"
	}
	b := &builder{opts: opts, plan: &Plan{StageLabel: opts.StageLabel}}
	p := b.plan

	var appsets, apps, projects []object
	for _, d := range in.Docs {
		if c, ok := clusterFrom(d); ok {
			b.clusters = append(b.clusters, c)
			continue
		}
		if !strings.HasPrefix(str(d.Value["apiVersion"]), "argoproj.io/") {
			continue
		}
		o := objectFrom(d)
		switch o.kind {
		case "ApplicationSet":
			appsets = append(appsets, o)
		case "Application":
			apps = append(apps, o)
		case "AppProject":
			projects = append(projects, o)
		}
	}
	sort.Slice(b.clusters, func(i, j int) bool { return b.clusters[i].Name < b.clusters[j].Name })
	for _, l := range [][]object{appsets, apps, projects} {
		sort.Slice(l, func(i, j int) bool { return l[i].name < l[j].name })
	}
	p.Clusters = b.clusters

	b.root = opts.RepoRoot
	if b.root == "" && len(in.Dirs) > 0 {
		b.root = findRepoRoot(in.Dirs[0])
	}
	p.Inputs = Inputs{Objects: len(in.Docs), Skipped: in.Skipped, RepoRoot: b.root}

	// Without a checkout the source paths cannot be checked, and a path that
	// does not exist is the failure this plan is most useful for catching. Say
	// so rather than passing silently: a clean plan would otherwise mean only
	// that nothing was looked at.
	if b.root == "" && len(in.Dirs) > 0 {
		p.Problems = append(p.Problems, "no repository checkout found above "+in.Dirs[0]+
			": source paths were not checked, so a path that does not exist would not be reported here. "+
			"Pass --repo-root <checkout> to check them")
	}

	if len(b.clusters) == 0 {
		p.Problems = append(p.Problems, "no Argo CD cluster Secrets in the input: add them with "+
			"'kubectl get secrets -n argocd -l argocd.argoproj.io/secret-type=cluster -o yaml' "+
			"(the plan reads names, servers and labels, never credentials)")
	}
	b.stages()

	for _, o := range appsets {
		b.appset(o)
	}

	var standalone []object
	for _, a := range apps {
		if owner := a.generatedBy(); owner != "" {
			p.Live = append(p.Live, fmt.Sprintf("%s (from ApplicationSet %s)", a.name, owner))
			continue
		}
		standalone = append(standalone, a)
	}
	b.tree(standalone, appsets, projects)
	b.windows(projects)
	b.unselected()
	b.handover(appsets, standalone, projects)
	b.checkLive()
	return p, nil
}

func findRepoRoot(dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}

// stages decides the stage order and each cluster's stage.
func (b *builder) stages() {
	p := b.plan
	b.stageOf = map[string]string{}
	if b.opts.StageLabel == "" {
		p.Stages = []string{"fleet"}
		for _, c := range b.clusters {
			b.stageOf[c.Name] = "fleet"
		}
		return
	}
	p.Stages = b.opts.Stages
	if len(p.Stages) == 0 {
		seen := map[string]bool{}
		for _, c := range b.clusters {
			if v, ok := c.Labels[b.opts.StageLabel]; ok && !seen[v] {
				seen[v] = true
				p.Stages = append(p.Stages, v)
			}
		}
		sort.Strings(p.Stages)
	}
	known := map[string]bool{}
	for _, s := range p.Stages {
		known[s] = true
	}
	for _, c := range b.clusters {
		v, ok := c.Labels[b.opts.StageLabel]
		switch {
		case !ok:
			p.Problems = append(p.Problems, fmt.Sprintf("cluster %s has no %s label, so it is in no stage", c.Name, b.opts.StageLabel))
		case !known[v]:
			p.Problems = append(p.Problems, fmt.Sprintf("cluster %s has %s=%s, which is not one of the stages (%s)", c.Name, b.opts.StageLabel, v, strings.Join(p.Stages, ", ")))
		default:
			b.stageOf[c.Name] = v
		}
	}
}

type renderedApp struct {
	set    paramSet
	app    map[string]any
	fields map[string]string
}

// appset expands one ApplicationSet into components: one per list element
// when a list is crossed with clusters, otherwise one.
func (b *builder) appset(o object) {
	p := b.plan
	spec := o.spec()
	tmpl := obj(spec["template"])
	r := rendererFor(spec)

	var sets []paramSet
	var descs []string
	for _, g := range list(spec["generators"]) {
		ps, desc, err := expand(obj(g), b.clusters)
		if err != nil {
			p.LeftOut = append(p.LeftOut, fmt.Sprintf("ApplicationSet %s: %v", o.name, err))
			return
		}
		sets = append(sets, ps...)
		descs = append(descs, desc)
	}

	groups := map[string][]renderedApp{}
	var keys []string
	for _, ps := range sets {
		fields := map[string]string{}
		v, err := r.render(tmpl, ps.values, "", fields)
		if err != nil {
			who := "a parameter set"
			if ps.cluster != nil {
				who = "cluster " + ps.cluster.Name
			}
			p.Problems = append(p.Problems, fmt.Sprintf("ApplicationSet %s does not render for %s: %v", o.name, who, err))
			continue
		}
		key := ""
		if ps.element != nil && ps.cluster != nil {
			key = elementKey(ps.element)
		}
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], renderedApp{set: ps, app: obj(v), fields: fields})
	}
	sort.Strings(keys)

	for _, key := range keys {
		name := o.name
		if key != "" {
			name += "-" + key
		}
		rs := groups[key]
		c := &Component{
			Name:      name,
			Kind:      "ApplicationSet",
			Source:    o.name,
			Project:   str(get(rs[0].app, "spec", "project")),
			Generator: strings.Join(descs, "; "),
			Wave:      o.wave,
			Base:      fmt.Sprintf("%s-%s-base", b.opts.Prefix, name),
		}
		var all []map[string]string
		for _, ra := range rs {
			all = append(all, ra.fields)
		}
		c.Departures = departures(all)

		var variants []Variant
		for _, ra := range rs {
			cl := ra.set.cluster
			if cl == nil {
				cl = b.clusterFor(ra.app)
			}
			variants = append(variants, b.variant(c, ra.app, cl, ra.fields))
		}
		b.finish(c, variants)
		if strings.EqualFold(str(get(spec, "strategy", "type")), "RollingSync") {
			c.Notes = append(c.Notes, "orders its own rollout with RollingSync; the ChangeWorkflow would own the order, so handover turns it off")
		}
		p.Components = append(p.Components, c)
	}
}

func elementKey(el map[string]any) string {
	keys := make([]string, 0, len(el))
	for k := range el {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, normalize(str(el[k])))
	}
	return strings.Join(parts, "-")
}

// departures are the templated fields whose value differs between variants;
// with one variant, every templated field.
func departures(all []map[string]string) []string {
	seen := map[string]bool{}
	for _, f := range all {
		for k := range f {
			seen[k] = true
		}
	}
	var out []string
	for k := range seen {
		if len(all) == 1 {
			out = append(out, k)
			continue
		}
		first, ok := all[0][k]
		for _, f := range all[1:] {
			if v, has := f[k]; !has || !ok || v != first {
				out = append(out, k)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func (b *builder) clusterFor(app map[string]any) *Cluster {
	server := str(get(app, "spec", "destination", "server"))
	name := str(get(app, "spec", "destination", "name"))
	for i := range b.clusters {
		c := b.clusters[i]
		if (server != "" && c.Server == server) || (name != "" && c.Name == name) {
			return &c
		}
	}
	if server == "https://kubernetes.default.svc" || name == "in-cluster" {
		return &Cluster{Name: "in-cluster", Server: "https://kubernetes.default.svc"}
	}
	return nil
}

func (b *builder) variant(c *Component, app map[string]any, cl *Cluster, fields map[string]string) Variant {
	p := b.plan
	clusterName := "unknown"
	if cl != nil {
		clusterName = cl.Name
	}
	v := Variant{
		Cluster:     clusterName,
		Space:       fmt.Sprintf("%s-%s-%s", b.opts.Prefix, c.Name, clusterName),
		Target:      fmt.Sprintf("%s-targets/%s", b.opts.Prefix, clusterName),
		Application: str(get(app, "metadata", "name")),
		Namespace:   str(get(app, "spec", "destination", "namespace")),
		Path:        str(get(app, "spec", "source", "path")),
	}
	if cl == nil {
		p.Problems = append(p.Problems, fmt.Sprintf("%s: Application %s names a destination no cluster Secret in the input matches", c.Name, v.Application))
	} else if cl.Name == "in-cluster" && len(p.Stages) > 0 {
		v.stage = p.Stages[0]
	} else if s, ok := b.stageOf[cl.Name]; ok {
		v.stage = s
	}
	if get(app, "spec", "source") == nil && get(app, "spec", "sources") != nil {
		v.Path = "(multi-source)"
		c.Notes = appendOnce(c.Notes, "uses spec.sources; only single-source Applications are checked offline")
	}

	var keys []string
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		val := fields[k]
		// A label the template reads but the cluster lacks renders as
		// "<no value>" through index, or as a gap: "overlays/", "a//b",
		// "storefront-". Argo CD renders it the same way and syncs it anyway.
		gap := strings.Contains(strings.ReplaceAll(val, "://", ""), "//")
		if val == "" || gap || strings.Contains(val, "<no value>") || strings.HasSuffix(val, "/") || strings.HasSuffix(val, "-") {
			p.Problems = append(p.Problems, fmt.Sprintf("%s on %s: %s renders as %q; check that cluster's labels", c.Name, clusterName, k, val))
		}
	}

	if b.root != "" && v.Path != "" && v.Path != "(multi-source)" {
		local := filepath.Join(b.root, filepath.FromSlash(v.Path))
		if info, err := os.Stat(local); err != nil || !info.IsDir() {
			p.Problems = append(p.Problems, fmt.Sprintf("%s on %s: source path %s does not exist in this checkout, so Argo CD would fail to sync %s", c.Name, clusterName, v.Path, v.Application))
		} else {
			v.Images = overlayImages(local)
			if helmIn(local, 4) {
				c.Notes = appendOnce(c.Notes, "a Helm chart inflated by Kustomize: Argo CD must run Kustomize with --enable-helm, and the chart's resources do not take the overlay's namespace field")
			}
		}
	}
	return v
}

func appendOnce(l []string, s string) []string {
	for _, x := range l {
		if x == s {
			return l
		}
	}
	return append(l, s)
}

// finish groups a component's variants into the plan's stages and notes image
// tags that differ between them.
func (b *builder) finish(c *Component, variants []Variant) {
	for _, s := range b.plan.Stages {
		st := Stage{Name: s}
		for _, v := range variants {
			if v.stage == s {
				st.Variants = append(st.Variants, v)
			}
		}
		sort.Slice(st.Variants, func(i, j int) bool { return st.Variants[i].Cluster < st.Variants[j].Cluster })
		if len(st.Variants) > 0 {
			c.Stages = append(c.Stages, st)
		}
	}
	var unstaged []string
	for _, v := range variants {
		if v.stage == "" {
			unstaged = append(unstaged, v.Cluster)
		}
	}
	if len(unstaged) > 0 {
		c.Notes = append(c.Notes, "not planned, because their cluster is in no stage: "+strings.Join(unstaged, ", "))
	}

	// Which tag each image has, in stage order.
	images := map[string]bool{}
	for _, st := range c.Stages {
		for _, v := range st.Variants {
			for img := range v.Images {
				images[img] = true
			}
		}
	}
	var names []string
	for img := range images {
		names = append(names, img)
	}
	sort.Strings(names)
	for _, img := range names {
		var tags []string
		on := map[string][]string{}
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				t := v.Images[img]
				if t == "" {
					t = "(base)"
				}
				if _, ok := on[t]; !ok {
					tags = append(tags, t)
				}
				on[t] = append(on[t], v.Cluster)
			}
		}
		if len(tags) > 1 {
			var parts []string
			for _, t := range tags {
				parts = append(parts, fmt.Sprintf("%s on %s", t, strings.Join(on[t], ", ")))
			}
			c.InFlight = append(c.InFlight, img+": "+strings.Join(parts, "; "))
		}
	}
}

// overlayImages reads the images an overlay's kustomization sets.
func overlayImages(dir string) map[string]string {
	k := readKustomization(dir)
	if k == nil {
		return nil
	}
	out := map[string]string{}
	for _, i := range list(k["images"]) {
		name := str(get(i, "name"))
		tag := str(get(i, "newTag"))
		if nn := str(get(i, "newName")); nn != "" {
			tag = nn + ":" + tag
		}
		if d := str(get(i, "digest")); d != "" {
			tag = "@" + d
		}
		if name != "" {
			out[name] = tag
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// helmIn reports whether a kustomization, or one it builds on, inflates a Helm
// chart.
func helmIn(dir string, depth int) bool {
	k := readKustomization(dir)
	if k == nil || depth == 0 {
		return false
	}
	if len(list(k["helmCharts"])) > 0 {
		return true
	}
	for _, r := range list(k["resources"]) {
		sub := filepath.Join(dir, filepath.FromSlash(str(r)))
		if info, err := os.Stat(sub); err == nil && info.IsDir() && helmIn(sub, depth-1) {
			return true
		}
	}
	return false
}

func readKustomization(dir string) map[string]any {
	for _, name := range []string{"kustomization.yaml", "kustomization.yml", "Kustomization"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		docs, err := parse(data, "")
		if err != nil || len(docs) == 0 {
			return nil
		}
		return docs[0].Value
	}
	return nil
}

// tree builds the control tree from standalone Applications that point at a
// folder of Argo CD objects, and plans the ones that deploy workloads.
func (b *builder) tree(apps, appsets, projects []object) {
	p := b.plan
	all := append(append(append([]object{}, apps...), appsets...), projects...)
	children := map[string][]object{}
	isChild := map[string]bool{}
	for _, a := range apps {
		for _, o := range b.childrenOf(a, all) {
			children[a.name] = append(children[a.name], o)
			isChild[o.kind+"/"+o.name] = true
		}
	}

	generated := map[string]int{}
	for _, c := range p.Components {
		for _, st := range c.Stages {
			generated[c.Source] += len(st.Variants)
		}
	}

	var node func(o object, depth int, seen map[string]bool) *Node
	node = func(o object, depth int, seen map[string]bool) *Node {
		n := &Node{Kind: o.kind, Name: o.name, Wave: o.wave, File: b.rel(o.file)}
		key := o.kind + "/" + o.name
		if seen[key] {
			n.Role = "cycle: already shown above"
			return n
		}
		seen[key] = true
		switch o.kind {
		case "Application":
			switch {
			case len(children[o.name]) == 0:
				n.Role = "deploys workloads; planned below"
			case depth == 0:
				n.Role = "root, applied by hand"
			default:
				n.Role = "app of apps"
			}
		case "ApplicationSet":
			n.Role = fmt.Sprintf("generates %d Applications", generated[o.name])
		case "AppProject":
			n.Role = "project"
			if w := len(list(get(o.spec(), "syncWindows"))); w > 0 {
				n.Role = fmt.Sprintf("project, %d sync window", w)
				if w > 1 {
					n.Role += "s"
				}
			}
		}
		kids := children[o.name]
		if o.kind != "Application" {
			kids = nil
		}
		sort.SliceStable(kids, func(i, j int) bool {
			if kids[i].wave != kids[j].wave {
				return kids[i].wave < kids[j].wave
			}
			if kids[i].kind != kids[j].kind {
				return kindOrder(kids[i].kind) < kindOrder(kids[j].kind)
			}
			return kids[i].name < kids[j].name
		})
		for _, k := range kids {
			if k.kind == "ApplicationSet" {
				for _, c := range p.Components {
					if c.Source == k.name {
						c.Owner = o.name
					}
				}
			}
			n.Children = append(n.Children, node(k, depth+1, seen))
		}
		return n
	}

	seen := map[string]bool{}
	for _, a := range apps {
		if !isChild["Application/"+a.name] && len(children[a.name]) > 0 {
			p.Tree = append(p.Tree, node(a, 0, seen))
		}
	}

	// A standalone Application that deploys workloads is a component with one
	// variant: its own destination.
	for _, a := range apps {
		if len(children[a.name]) > 0 {
			continue
		}
		if b.root == "" && str(get(a.spec(), "destination", "namespace")) == a.namespace {
			p.LeftOut = append(p.LeftOut, fmt.Sprintf("Application %s deploys into %s, so it is likely an app of apps; pass the repository directory to see its children", a.name, a.namespace))
			continue
		}
		c := &Component{
			Name:       a.name,
			Kind:       "Application",
			Source:     a.name,
			Project:    str(get(a.spec(), "project")),
			Wave:       a.wave,
			Base:       fmt.Sprintf("%s-%s-base", b.opts.Prefix, a.name),
			Departures: []string{"spec.destination"},
		}
		v := b.variant(c, a.value, b.clusterFor(a.value), nil)
		b.finish(c, []Variant{v})
		p.Components = append(p.Components, c)
	}
	sort.SliceStable(p.Components, func(i, j int) bool { return p.Components[i].Name < p.Components[j].Name })
}

func kindOrder(k string) int {
	switch k {
	case "AppProject":
		return 0
	case "ApplicationSet":
		return 1
	}
	return 2
}

func (b *builder) childrenOf(a object, all []object) []object {
	if b.root == "" {
		return nil
	}
	src := str(get(a.spec(), "source", "path"))
	if src == "" {
		return nil
	}
	dir := filepath.Clean(filepath.Join(b.root, filepath.FromSlash(src)))
	recurse := get(a.spec(), "source", "directory", "recurse") == true
	var out []object
	for _, o := range all {
		if o.file == "" || (o.kind == a.kind && o.name == a.name) {
			continue
		}
		od := filepath.Dir(o.file)
		if od == dir || (recurse && strings.HasPrefix(od, dir+string(filepath.Separator))) {
			out = append(out, o)
		}
	}
	return out
}

func (b *builder) rel(file string) string {
	if file == "" || b.root == "" {
		return file
	}
	if r, err := filepath.Rel(b.root, file); err == nil {
		return filepath.ToSlash(r)
	}
	return file
}

// windows matches each AppProject sync window to the planned Applications it
// covers, so a promotion knows when Argo CD will hold it back.
func (b *builder) windows(projects []object) {
	p := b.plan
	for _, pr := range projects {
		for _, w := range list(get(pr.spec(), "syncWindows")) {
			win := Window{
				Project:    pr.name,
				Kind:       str(get(w, "kind")),
				Schedule:   str(get(w, "schedule")),
				Duration:   str(get(w, "duration")),
				TimeZone:   str(get(w, "timeZone")),
				ManualSync: get(w, "manualSync") == true,
			}
			apps := globs(get(w, "applications"))
			clusters := globs(get(w, "clusters"))
			namespaces := globs(get(w, "namespaces"))
			for _, c := range p.Components {
				if c.Project != pr.name {
					continue
				}
				for _, st := range c.Stages {
					for _, v := range st.Variants {
						if matchAny(apps, v.Application) || matchAny(clusters, v.Cluster) || matchAny(namespaces, v.Namespace) {
							win.Applications = append(win.Applications, v.Application)
						}
					}
				}
			}
			sort.Strings(win.Applications)
			p.Windows = append(p.Windows, win)
		}
	}
}

func globs(v any) []string {
	var out []string
	for _, g := range list(v) {
		out = append(out, str(g))
	}
	return out
}

func matchAny(patterns []string, s string) bool {
	for _, pat := range patterns {
		if ok, _ := path.Match(pat, s); ok {
			return true
		}
	}
	return false
}

func (b *builder) unselected() {
	p := b.plan
	used := map[string]bool{}
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				used[v.Cluster] = true
			}
		}
	}
	for _, c := range b.clusters {
		if !used[c.Name] {
			p.Unselected = append(p.Unselected, c.Name)
		}
	}
}

// handover says what handing the live estate to ConfigHub would involve.
// Nothing here runs; `apply` will write it as handover.sh.
func (b *builder) handover(appsets, apps []object, projects []object) {
	p := b.plan
	if len(appsets) > 0 {
		var names []string
		for _, a := range appsets {
			names = append(names, a.name)
		}
		p.Handover = append(p.Handover, fmt.Sprintf("delete each ApplicationSet (%s) with 'kubectl delete --cascade=orphan', so its Applications stay", strings.Join(names, ", ")))
		p.Handover = append(p.Handover, "hand each Application to its variant under the same name, so Argo CD's tracking ID does not change and only the source moves to the ConfigHub gateway")
	}
	var guarded []string
	for _, a := range apps {
		for _, f := range list(get(a.value, "metadata", "finalizers")) {
			if str(f) == "resources-finalizer.argocd.argoproj.io" {
				guarded = append(guarded, a.name)
			}
		}
	}
	for _, a := range appsets {
		for _, f := range list(get(a.spec(), "template", "metadata", "finalizers")) {
			if str(f) == "resources-finalizer.argocd.argoproj.io" {
				guarded = append(guarded, "every Application "+a.name+" generates")
			}
		}
	}
	if len(guarded) > 0 {
		p.Handover = append(p.Handover, fmt.Sprintf("never delete %s: resources-finalizer.argocd.argoproj.io deletes everything it deployed", strings.Join(guarded, ", ")))
	}
	used := map[string]bool{}
	for _, c := range p.Components {
		used[c.Project] = true
	}
	for _, pr := range projects {
		if !used[pr.name] {
			continue
		}
		repos := globs(get(pr.spec(), "sourceRepos"))
		allowed := false
		for _, r := range repos {
			if r == "*" || strings.HasPrefix(r, "oci://") {
				allowed = true
			}
		}
		if !allowed {
			p.Handover = append(p.Handover, fmt.Sprintf("AppProject %s allows only %s: add the ConfigHub gateway's oci:// address to sourceRepos first", pr.name, strings.Join(repos, ", ")))
		}
	}
}

// checkLive compares live generated Applications with the ones the plan
// predicts, which catches a generator the plan reads differently from Argo CD.
func (b *builder) checkLive() {
	p := b.plan
	if len(p.Live) == 0 {
		return
	}
	planned := map[string]bool{}
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				planned[v.Application] = true
			}
		}
	}
	for _, l := range p.Live {
		name, _, _ := strings.Cut(l, " ")
		if !planned[name] {
			p.Problems = append(p.Problems, "live Application "+l+" is not one the plan predicts")
		}
	}
	sort.Strings(p.Live)
}
