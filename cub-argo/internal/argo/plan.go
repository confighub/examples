package argo

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/confighub/sveltos-confighub/chartrender"

	"github.com/confighub/examples/cub-argo/internal/catalog"
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
	// RestrictedProjects are the AppProjects whose sourceRepos would refuse an
	// oci:// source, as the repository reads today. handover.sh re-reads them
	// on the cluster before it stops for them.
	RestrictedProjects []string `json:"restrictedProjects,omitempty"`
	LeftOut            []string `json:"leftOut,omitempty"`
	Problems           []string `json:"problems,omitempty"`
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
	// Recurse is spec.source.directory.recurse, for a plain directory: Argo
	// CD then reads the manifests below the path, not only at it.
	Recurse bool `json:"recurse,omitempty"`
	// Key is what tells this variant from its siblings: the cluster, or the
	// Application's own name where one cluster carries several.
	Key   string `json:"key,omitempty"`
	stage string
	// app is the Application the ApplicationSet's template renders for this
	// cluster, which is what a cluster joining after the retirement is given.
	app map[string]any
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
	// missedCluster is set when a generator selected no cluster or an
	// Application named a destination no cluster Secret matches.
	missedCluster bool
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
	// A file that would not parse is not the same as a file that is not
	// Kubernetes YAML, and the difference has teeth. A control Space holds the
	// children of an app of apps byte for byte; a child that was skipped is
	// simply absent from it, and the parent, which prunes, DELETES the objects
	// that file defined from the cluster the moment its source is repointed.
	//
	// Measured: one broken indent in projects.yaml dropped it silently, and the
	// repointed root removed both live AppProjects. It was reported as "skipped
	// 1 file that is not Kubernetes YAML" -- a count, under a banner that reads
	// as benign.
	if len(in.Skipped) > 0 {
		// The two path forms differ: a node's File is relative to the repository
		// root, while a skipped file is relative to the input directory. Compare
		// on the tail they share rather than joining either to a root.
		var controlDirs []string
		for _, cs := range p.controlSpaces("x") {
			for _, f := range cs.Files {
				controlDirs = append(controlDirs, filepath.ToSlash(filepath.Dir(f)))
			}
		}
		for _, sk := range in.Skipped {
			skDir := filepath.ToSlash(filepath.Dir(sk))
			inControl := false
			for _, d := range controlDirs {
				if d == skDir || strings.HasSuffix(d, "/"+skDir) || strings.HasSuffix(skDir, "/"+d) {
					inControl = true
				}
			}
			if inControl {
				p.Problems = append(p.Problems, fmt.Sprintf(
					"%s sits beside objects an app of apps syncs, and could not be read as Kubernetes YAML. "+
						"It would be missing from the Space that replaces that directory, and the parent prunes, "+
						"so whatever it defines would be DELETED from the cluster at handover. Fix the file, or "+
						"move it out of the directory the parent syncs.", sk))
			}
		}
	}

	// Argo CD's own cluster needs no Secret, so an estate deploying only
	// there plans without any. Only when something looked for a cluster and
	// found none is the missing export the likely reason.
	if len(b.clusters) == 0 && b.missedCluster {
		p.Problems = append(p.Problems, "no Argo CD cluster Secrets in the input: add them with "+
			"'kubectl get secrets -n argocd -l argocd.argoproj.io/secret-type=cluster -o yaml' "+
			"(the plan reads names, servers and labels, never credentials)")
	}

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
		ps, desc, err := expand(obj(g), b.clusters, b.root)
		if err != nil {
			p.LeftOut = append(p.LeftOut, fmt.Sprintf("ApplicationSet %s: %v", o.name, err))
			return
		}
		sets = append(sets, ps...)
		descs = append(descs, desc)
	}

	if len(sets) == 0 {
		b.missedCluster = true
		p.Problems = append(p.Problems, fmt.Sprintf(
			"ApplicationSet %s selects nothing here, so it would govern no cluster. Its generator (%s) matched none of the %d clusters in the input; export the cluster Secrets it selects on, or say why it is empty",
			o.name, strings.Join(descs, "; "), len(b.clusters)))
		return
	}

	groups := map[string][]renderedApp{}
	var keys []string
	for _, ps := range sets {
		fields := map[string]string{}
		v, err := r.render(tmpl, ps, "", fields)
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
		if _, patched := spec["templatePatch"]; patched {
			// Rendered from spec.template alone, a joined cluster's Application
			// would miss what the patch adds, so none is written for it.
			for i := range variants {
				variants[i].app = nil
			}
			c.Notes = append(c.Notes, "uses templatePatch, which the plan does not render: a cluster that joins after the handover gets its Application by hand")
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
	if server == localServer || name == "in-cluster" {
		c := localCluster()
		return &c
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
		app:         app,
		Path:        str(get(app, "spec", "source", "path")),
	}
	if cl == nil {
		b.missedCluster = true
		p.Problems = append(p.Problems, fmt.Sprintf("%s: Application %s names a destination no cluster Secret in the input matches", c.Name, v.Application))
	} else if cl.Name == "in-cluster" && len(p.Stages) > 0 {
		v.stage = p.Stages[0]
	} else if s, ok := b.stageOf[cl.Name]; ok {
		v.stage = s
	}
	if ch := str(get(app, "spec", "source", "chart")); ch != "" {
		p.Problems = append(p.Problems, fmt.Sprintf(
			"%s on %s: Application %s deploys Helm chart %q from %s, not a path in a repository. What ConfigHub would hold is the chart's render, and this plan does not produce it, so the variant would be empty. Chart sources are not onboarded yet",
			c.Name, clusterName, v.Application, ch, str(get(app, "spec", "source", "repoURL"))))
		v.Path = "(helm chart " + ch + ")"
		return v
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
			// An Application that names its tool gets that tool, whatever is
			// at the path. Otherwise Argo CD picks it from the files: a
			// kustomization wins, then a Chart.yaml (Helm), then a plain
			// directory of manifests.
			src := obj(get(app, "spec", "source"))
			switch {
			case src["plugin"] != nil:
				p.Problems = appendOnce(p.Problems, fmt.Sprintf(
					"%s: %s is rendered by a config management plugin (spec.source.plugin), which the script cannot reproduce, so what ConfigHub stored could differ from what Argo applies",
					c.Name, v.Path))
			case src["directory"] != nil:
				b.plainDirectory(c, &v, app, local)
			case src["kustomize"] != nil && !hasKustomization(local):
				p.Problems = appendOnce(p.Problems, fmt.Sprintf(
					"%s: %s sets spec.source.kustomize but holds no kustomization.yaml, so kustomize would build nothing from it",
					c.Name, v.Path))
			case src["kustomize"] != nil:
			case !hasKustomization(local):
				if _, err := os.Stat(filepath.Join(local, "Chart.yaml")); err == nil {
					p.Problems = appendOnce(p.Problems, fmt.Sprintf(
						"%s: %s is a Helm chart (it has a Chart.yaml), which Argo CD renders with helm template. The script renders with kustomize build, which does not read a chart. Onboarding a chart kept in the repository is not supported yet",
						c.Name, v.Path))
				} else {
					b.plainDirectory(c, &v, app, local)
				}
			}
			charts := helmCharts(local, 4)
			if len(charts) > 0 {
				c.Notes = appendOnce(c.Notes, "a Helm chart inflated by Kustomize: Argo CD must run Kustomize with --enable-helm, and the chart's resources do not take the overlay's namespace field")
			}
			// What the Workshop Catalog has already decided about each chart,
			// per audited values base. A chart it has not audited is said to be
			// unchecked rather than assumed fine.
			for _, ch := range charts {
				for _, line := range catalog.Describe(ch.name, ch.version) {
					c.Notes = appendOnce(c.Notes, line)
				}
			}
			// What ConfigHub stores is a render. A render that cannot be
			// repeated cannot be checked against Git later, and the handover
			// turns on exactly that comparison.
			for _, ch := range charts {
				if !ch.reproducible() {
					p.Problems = appendOnce(p.Problems, fmt.Sprintf(
						"%s renders differently from one run to the next: %s. Pin it to an exact version, or vendor it, before what ConfigHub stores can be compared with what Git renders",
						c.Name, ch.why()))
				}
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
	// A cluster generator fans out one Application per cluster, so the cluster
	// names a variant. A git-directory generator fans out per directory, often
	// onto one cluster, and then the cluster does not tell two variants apart.
	// Argo requires Application names to be unique, so they always do.
	seen := map[string]bool{}
	collide := false
	for _, v := range variants {
		if seen[v.Cluster] {
			collide = true
		}
		seen[v.Cluster] = true
	}
	if collide {
		for i := range variants {
			key := strings.TrimPrefix(variants[i].Application, c.Name+"-")
			key = strings.TrimSuffix(key, "-"+c.Name)
			if key == "" || key == variants[i].Application {
				key = variants[i].Application
			}
			variants[i].Space = fmt.Sprintf("%s-%s-%s", b.opts.Prefix, c.Name, key)
			variants[i].Key = key
		}
	}

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
	return len(helmCharts(dir, depth)) > 0
}

// chart is one Helm chart a kustomization inflates at render time.
type chart struct {
	name, version, repo string
}

// reproducible reports whether rendering this chart twice must give the same
// bytes. A chart vendored in the repository at an exact version does; one
// pulled from a remote repository, or pinned to a range, does not.
//
// What counts as one exact version is chartrender's, not this plugin's, so
// cub sveltos, cub argo and cub flux all mean the same thing by it.
func (c chart) reproducible() bool {
	return c.repo == "" && chartrender.ExactVersion(c.version)
}

func (c chart) why() string {
	switch {
	case c.repo != "" && !c.reproducible():
		return fmt.Sprintf("chart %s %s is pulled from %s at render time, and %q is not one exact version",
			c.name, c.version, c.repo, c.version)
	case c.repo != "":
		return fmt.Sprintf("chart %s %s is pulled from %s at render time, so what it renders depends on that repository",
			c.name, c.version, c.repo)
	case c.version == "":
		return fmt.Sprintf("chart %s names no version, so what it renders can change", c.name)
	default:
		return fmt.Sprintf("chart %s is pinned to %q rather than one exact version", c.name, c.version)
	}
}

// helmCharts collects the charts a kustomization, or one it builds on,
// inflates.
func helmCharts(dir string, depth int) []chart {
	k := readKustomization(dir)
	if k == nil || depth == 0 {
		return nil
	}
	var out []chart
	for _, h := range list(k["helmCharts"]) {
		out = append(out, chart{
			name:    str(get(h, "name")),
			version: str(get(h, "version")),
			repo:    str(get(h, "repo")),
		})
	}
	for _, r := range list(k["resources"]) {
		sub := filepath.Join(dir, filepath.FromSlash(str(r)))
		if info, err := os.Stat(sub); err == nil && info.IsDir() {
			out = append(out, helmCharts(sub, depth-1)...)
		}
	}
	return out
}

// hasKustomization says whether dir holds a kustomization file, as Argo CD
// and kustomize name it, whether or not it parses.
func hasKustomization(dir string) bool {
	for _, name := range []string{"kustomization.yaml", "kustomization.yml", "Kustomization"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
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
	var declared map[string]string
	for _, o := range all {
		if o.kind == a.kind && o.name == a.name {
			continue
		}
		if o.file == "" || !strings.HasPrefix(filepath.Clean(o.file), filepath.Clean(b.root)+string(filepath.Separator)) {
			// A live object, read from 'kubectl get', carries no repository
			// file: at most the export it was read from. It is
			// this parent's child if the directory the parent syncs declares
			// it, and that file is then where it comes from: the same file a
			// repository input would have given it, so the control Space holds
			// the same bytes either way.
			if declared == nil {
				declared = declaredIn(dir, recurse)
			}
			if f, ok := declared[o.kind+"/"+o.name]; ok {
				o.file = f
				out = append(out, o)
			}
			continue
		}
		od := filepath.Dir(o.file)
		if od == dir || (recurse && strings.HasPrefix(od, dir+string(filepath.Separator))) {
			out = append(out, o)
		}
	}
	return out
}

// declaredIn maps kind/name to the file that declares it, for every object in
// the YAML files of dir (and below it, with recurse), as Argo CD would read a
// directory source. A file that is not Kubernetes YAML declares nothing here;
// the plan reports such files separately.
func declaredIn(dir string, recurse bool) map[string]string {
	out := map[string]string{}
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir && !recurse {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		docs, err := parse(data, path)
		if err != nil {
			return nil
		}
		for _, doc := range docs {
			k, n := str(doc.Value["kind"]), str(get(doc.Value, "metadata", "name"))
			if k != "" && n != "" {
				if _, seen := out[k+"/"+n]; !seen {
					out[k+"/"+n] = path
				}
			}
		}
		return nil
	})
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

// childrenSpace names the Space that would hold what an app of apps syncs.
func (b *builder) childrenSpace(name string) string {
	return fmt.Sprintf("%s-%s-children", b.opts.Prefix, name)
}

// handover says what moving the live estate onto ConfigHub would involve.
// Nothing here runs; `apply` will write it as handover.sh.
//
// Each layer is repointed rather than orphaned. An app of apps does not own
// its children through ownerReferences: it owns them by syncing a directory
// and pruning what is not in it, so there is nothing to sever, and repointing
// the parent leaves the tree, every name and every tracking ID intact. Only an
// object nothing above it syncs has to be handled any other way.
func (b *builder) handover(appsets, apps []object, projects []object) {
	p := b.plan
	if len(p.Components) == 0 {
		return
	}

	// The projects gate every repoint below, so they come first.
	used := map[string]bool{}
	for _, c := range p.Components {
		used[c.Project] = true
	}
	var blocked []string
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
			blocked = append(blocked, fmt.Sprintf("%s (allows only %s)", pr.name, strings.Join(repos, ", ")))
			// The bare name as well: handover.sh re-checks these on the cluster,
			// because this reading comes from Git and cannot tell whether the
			// operator has already widened them.
			p.RestrictedProjects = append(p.RestrictedProjects, pr.name)
		}
	}
	if len(blocked) > 0 {
		p.Handover = append(p.Handover, fmt.Sprintf(
			"first, add the gateway's oci:// address to sourceRepos on AppProject %s: until then every repoint below is refused",
			strings.Join(blocked, " and ")))
	}
	p.Handover = append(p.Handover,
		"check Argo CD is v3.1 or newer, which is where an oci:// source is read natively; an older one cannot do this at all")

	// Walk the control tree top down. A node with children is an app of apps:
	// repoint it, and its children become Units in a Space of its own.
	hasParent := map[string]bool{}
	var mark func(n *Node)
	mark = func(n *Node) {
		for _, c := range n.Children {
			hasParent[c.Kind+"/"+c.Name] = true
			mark(c)
		}
	}
	for _, n := range p.Tree {
		mark(n)
	}

	var walk func(n *Node, parent *Node)
	walk = func(n *Node, parent *Node) {
		if n.Kind == "Application" && len(n.Children) > 0 {
			var kids []string
			for _, c := range n.Children {
				kids = append(kids, c.Kind+" "+c.Name)
			}
			space := b.childrenSpace(n.Name)
			how := "Nothing above it syncs it, so patch its spec.source in the cluster."
			if parent != nil {
				how = fmt.Sprintf("It is a Unit in %s by this point, so repoint it there and promote, and the repoint is reviewed like any other change.", b.childrenSpace(parent.Name))
			}
			p.Handover = append(p.Handover, fmt.Sprintf(
				"repoint Application %s at %s, which would hold %s. Publish that Space first: a parent left syncing an empty source prunes its children. %s",
				n.Name, space, strings.Join(kids, ", "), how))
		}
		for _, c := range n.Children {
			walk(c, n)
		}
	}
	for _, n := range p.Tree {
		walk(n, nil)
	}

	// Each ApplicationSet is retired, and each Application it generated is then
	// delivered from its own Space as a Unit, so none is orphaned or renamed.
	for _, c := range p.Components {
		if c.Kind != "ApplicationSet" {
			continue
		}
		var names []string
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				names = append(names, v.Application)
			}
		}
		if hasParent["ApplicationSet/"+c.Source] {
			p.Handover = append(p.Handover, fmt.Sprintf(
				"retire ApplicationSet %s through its Unit (applicationsSync: create-only, and generators that generate nothing), then run move-applications.sh stage by stage: %s each become a Unit reading its own Space, under the same name, so Argo's tracking does not change and nothing is orphaned",
				c.Source, strings.Join(names, ", ")))
			continue
		}
		p.Handover = append(p.Handover, fmt.Sprintf(
			"ApplicationSet %s is applied by hand, so its template cannot be repointed under review. Either patch the template in the cluster, or delete it with 'kubectl delete --cascade=orphan' and repoint %s one by one. Orphaning is the fallback here, not the plan",
			c.Source, strings.Join(names, ", ")))
	}

	var guarded []string
	for _, a := range apps {
		if cascades(get(a.value, "metadata", "finalizers")) {
			guarded = append(guarded, a.name)
		}
	}
	for _, a := range appsets {
		// An ApplicationSet gives what it generates the finalizer whether or
		// not its template names one (measured on Argo CD v3.5.3), and
		// preserveResourcesOnDeletion did not keep the workloads when it was
		// tried, so every generated Application is named.
		guarded = append(guarded, "every Application "+a.name+" generates")
	}
	if len(guarded) > 0 {
		p.Handover = append(p.Handover, fmt.Sprintf(
			"never delete %s: resources-finalizer.argocd.argoproj.io deletes everything it deployed. Every step above is a patch for exactly this reason",
			strings.Join(guarded, ", ")))
	}
}

// cascades reports whether a finalizer list holds Argo CD's resources
// finalizer, in any of its forms: bare, /foreground or /background.
func cascades(finalizers any) bool {
	for _, f := range list(finalizers) {
		if f := str(f); f == "resources-finalizer.argocd.argoproj.io" || strings.HasPrefix(f, "resources-finalizer.argocd.argoproj.io/") {
			return true
		}
	}
	return false
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

// plainDirectory reads a path with no kustomization and no chart as Argo CD
// reads it: every .yaml, .yml and .json file there, or below it with
// directory.recurse. What Argo would do differently from that is a problem.
func (b *builder) plainDirectory(c *Component, v *Variant, app map[string]any, local string) {
	p := b.plan
	dir := obj(get(app, "spec", "source", "directory"))
	v.Recurse = dir["recurse"] == true
	var unread []string
	for _, k := range []string{"include", "exclude"} {
		if str(dir[k]) != "" {
			unread = append(unread, "directory."+k)
		}
	}
	if dir["jsonnet"] != nil {
		unread = append(unread, "directory.jsonnet")
	}
	files, jsonnet := manifestFiles(local, v.Recurse)
	if len(jsonnet) > 0 {
		unread = append(unread, "jsonnet files ("+strings.Join(jsonnet, ", ")+")")
	}
	// Argo CD applies each document as an object, so a patch or a values
	// file left in the directory fails its sync, and would fail the render.
	var notObjects []string
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(local, filepath.FromSlash(f)))
		if err != nil {
			notObjects = append(notObjects, f+" (unreadable)")
			continue
		}
		docs, err := parse(data, f)
		if err != nil {
			notObjects = append(notObjects, f+" (does not parse)")
			continue
		}
		for _, d := range docs {
			if str(d.Value["apiVersion"]) == "" || str(d.Value["kind"]) == "" {
				notObjects = append(notObjects, f)
				break
			}
		}
	}
	switch {
	case len(notObjects) > 0:
		p.Problems = appendOnce(p.Problems, fmt.Sprintf(
			"%s: %s is a plain directory, and %s in it is not a whole Kubernetes object (apiVersion and kind), which Argo CD would fail to apply. Add a kustomization.yaml if these are patches, or move them out",
			c.Name, v.Path, strings.Join(notObjects, ", ")))
	case len(unread) > 0:
		p.Problems = appendOnce(p.Problems, fmt.Sprintf(
			"%s: %s is a plain directory Argo CD reads with %s, which the script does not reproduce yet, so what ConfigHub stored could differ from what Argo applies",
			c.Name, v.Path, strings.Join(unread, " and ")))
	case len(files) == 0:
		p.Problems = appendOnce(p.Problems, fmt.Sprintf(
			"%s: %s holds no .yaml, .yml or .json files, so Argo CD applies nothing from it and neither would ConfigHub", c.Name, v.Path))
	default:
		how := "at the top level"
		if v.Recurse {
			how = "and below it (directory.recurse)"
		}
		c.Notes = appendOnce(c.Notes, fmt.Sprintf("a plain directory of manifests, read as Argo CD reads it: every .yaml, .yml and .json file %s", how))
	}
}

// manifestFiles lists what Argo CD would read from a plain directory, and any
// jsonnet it would evaluate, relative to dir. Hidden files and directories are
// left out, as the script leaves them out.
func manifestFiles(dir string, recurse bool) (files, jsonnet []string) {
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if rel == "." {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if !recurse {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".yaml", ".yml", ".json":
			files = append(files, filepath.ToSlash(rel))
		case ".jsonnet", ".libsonnet":
			jsonnet = append(jsonnet, filepath.ToSlash(rel))
		}
		return nil
	})
	return files, jsonnet
}
