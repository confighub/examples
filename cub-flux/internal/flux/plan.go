package flux

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/confighub/sveltos-confighub/chartrender"

	"github.com/confighub/examples/cub-flux/internal/catalog"
)

// Options shape the plan.
type Options struct {
	// Prefix starts the name of everything the plan would create in ConfigHub.
	Prefix string
	// Stages orders the clusters, by directory name or cluster name. By
	// default dev, staging and prod come first, then the rest by name.
	Stages []string
	// RepoRoot is the checkout that Flux paths are relative to. When empty it
	// is found by walking up from the input directory to .git.
	RepoRoot string
	// ClustersDir is the directory, relative to the input, holding one
	// directory per cluster. Default "clusters".
	ClustersDir string
	// Require is what each stage after the first also waits for, in the
	// stage before it. Healthy is the one offered: it passes on the live
	// status `cub flux status` reports.
	Require []string
}

// Plan is what ConfigHub would hold for a Flux fleet. Building it changes
// nothing.
type Plan struct {
	Inputs     Inputs       `json:"inputs"`
	Clusters   []Cluster    `json:"clusters"`
	Stages     []string     `json:"stages"`
	Order      [][]string   `json:"reconcileOrder,omitempty"`
	Components []*Component `json:"components"`
	Sources    []Source     `json:"sources,omitempty"`
	Automation []string     `json:"writesToGit,omitempty"`
	NotInGit   []string     `json:"notInGit,omitempty"`
	Tenants    []string     `json:"tenants,omitempty"`
	Boundary   []string     `json:"boundary,omitempty"`
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

// Cluster is one directory under the clusters directory: what `flux
// bootstrap --path` points one cluster at.
type Cluster struct {
	Name  string `json:"name"`
	Dir   string `json:"dir"`
	Stage string `json:"stage"`
	// Path is the cluster's directory from the repository root, where the
	// files flux-system applies for it live.
	Path string `json:"path,omitempty"`
	// LayersSpace, when set, says the cluster is handed over: its directory
	// holds the ConfigHub root, and its layers are Units in this Space rather
	// than files in Git.
	LayersSpace string `json:"layersSpace,omitempty"`
	// PullSecret is the Secret a handed-over root pulls its layers with.
	PullSecret string `json:"pullSecret,omitempty"`
}

// layers is the Space holding the cluster's layers: the one its root reads
// once it is handed over, and the one handover.sh will point it at otherwise.
func (c Cluster) layers(prefix string) string {
	if c.LayersSpace != "" {
		return c.LayersSpace
	}
	return DeliverySpace(prefix, c.Name)
}

// pullSecret is the Secret the cluster's layers pull with: the one its root
// names once it is handed over, and the one handover.sh will make otherwise.
// A handed-over root with no secretRef pulls anonymously, so there is none.
func (c Cluster) pullSecret(prefix string) string {
	if c.LayersSpace != "" {
		return c.PullSecret
	}
	return "confighub-" + prefix + "-targets"
}

// Component is one layer (a Flux Kustomization name) across the clusters: a
// base inferred from the overlays, and one variant per cluster.
type Component struct {
	// Require is what each stage after the first also waits for.
	Require      []string `json:"require,omitempty"`
	Name         string   `json:"name"`
	Base         string   `json:"base"`
	BaseDir      string   `json:"baseDir"`
	DependsOn    []string `json:"dependsOn,omitempty"`
	Stages       []Stage  `json:"stages"`
	HelmReleases []string `json:"helmReleases,omitempty"`
	InFlight     []string `json:"inFlight,omitempty"`
	Notes        []string `json:"notes,omitempty"`
}

// Stage is the variants a change reaches together.
type Stage struct {
	Name     string    `json:"name"`
	Variants []Variant `json:"variants"`
}

// Variant is one cluster's copy of the layer: its Flux Kustomization, kept
// under its own name so Flux keeps its inventory, and what its overlay
// changes from the base.
type Variant struct {
	Cluster       string `json:"cluster"`
	Space         string `json:"space"`
	Target        string `json:"target"`
	Kustomization string `json:"kustomization"`
	Path          string `json:"path"`
	// TargetNamespace is where the layer puts an object whose manifest names
	// no namespace. Without it a check cannot tell one of those apart from an
	// object the layer does not apply at all.
	TargetNamespace string            `json:"targetNamespace,omitempty"`
	Departures      []string          `json:"departures"`
	Images          map[string]string `json:"images,omitempty"`
	stage           string
	layer           map[string]any
	// layerFile is the repository file that defines the layer's
	// Kustomization for this cluster: what a bootstrapped handover removes.
	layerFile    string
	overlay      map[string]any
	resourceDirs []string
}

// Source is a Flux source and the branch each cluster reads it at.
type Source struct {
	Kind     string            `json:"kind"`
	Name     string            `json:"name"`
	URL      string            `json:"url"`
	Branches map[string]string `json:"branches,omitempty"`
	File     string            `json:"file"`
}

const kustomizeToolkit = "kustomize.toolkit.fluxcd.io/"

type builder struct {
	rootGuessed bool
	opts        Options
	root        string
	input       string
	plan        *Plan
	docs        []Doc
	clusters    []Cluster
}

// Build plans a Flux fleet repository. It reads the input directory and the
// files the Flux Kustomizations point at.
func Build(in *Input, opts Options) (*Plan, error) {
	if len(in.Dirs) != 1 {
		return nil, fmt.Errorf("cub flux plan reads one fleet repository directory; live exports come next")
	}
	if opts.Prefix == "" {
		opts.Prefix = "flux"
	}
	if opts.ClustersDir == "" {
		opts.ClustersDir = "clusters"
	}
	for _, r := range opts.Require {
		if r != "Healthy" {
			return nil, fmt.Errorf("--require %s: only Healthy is offered, which passes on the live status `cub flux status` reports", r)
		}
	}
	b := &builder{opts: opts, input: in.Dirs[0], plan: &Plan{}, docs: in.Docs}
	b.root = opts.RepoRoot
	if b.root == "" {
		if b.root = findRepoRoot(b.input); b.root == "" {
			// Flux paths are written from the top of the repository, so
			// without a checkout the input directory is only a guess at the
			// root. Every path then looks missing, which reads as a broken
			// fleet rather than as the wrong starting point.
			b.root, b.rootGuessed = b.input, true
		}
	}
	b.plan.Inputs = Inputs{Objects: len(in.Docs), Skipped: in.Skipped, RepoRoot: b.root}

	layers := b.readClusters()
	b.stages()
	b.components(layers)
	b.order()
	b.sources()
	b.automation()
	b.tenants()
	b.substitutions()
	b.explainGuessedRoot()
	b.handover()
	return b.plan, nil
}

// explainGuessedRoot names the likely cause when a guessed root made every
// path look missing, so the plan does not report a healthy fleet as broken.
func (b *builder) explainGuessedRoot() {
	if !b.rootGuessed {
		return
	}
	missing := 0
	for _, pr := range b.plan.Problems {
		if strings.Contains(pr, "does not exist in this checkout") {
			missing++
		}
	}
	if missing == 0 {
		return
	}
	b.plan.Problems = append([]string{fmt.Sprintf(
		"no repository checkout found above %s, so paths were resolved against it and all %d look missing. "+
			"Flux paths are written from the top of the repository: pass --repo-root <checkout> before reading the %d problems below as real",
		b.input, missing, missing)}, b.plan.Problems...)
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

func (b *builder) rel(p string) string {
	if r, err := filepath.Rel(b.root, p); err == nil {
		return filepath.ToSlash(r)
	}
	return p
}

func under(file, dir string) bool {
	return file == dir || strings.HasPrefix(file, dir+string(filepath.Separator))
}

// readClusters finds one cluster per directory under the clusters directory,
// and each cluster's Flux Kustomizations: its layers.
func (b *builder) readClusters() map[string]map[string]Doc {
	p := b.plan
	dir := filepath.Join(b.input, b.opts.ClustersDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		p.Problems = append(p.Problems, fmt.Sprintf("no %s directory with one directory per cluster; pass --clusters", b.opts.ClustersDir))
		return nil
	}
	layers := map[string]map[string]Doc{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cdir := filepath.Join(dir, e.Name())
		boot := filepath.Join(cdir, "flux-system")
		if _, err := os.Stat(boot); err == nil {
			p.Boundary = append(p.Boundary, b.rel(boot))
		}
		byName := map[string]Doc{}
		name := e.Name()
		layersSpace, pullSecret, rootName := "", "", ""
		for _, d := range b.docs {
			if !under(d.File, cdir) || under(d.File, boot) {
				continue
			}
			api, kind := str(d.Value["apiVersion"]), str(d.Value["kind"])
			dname := str(get(d.Value, "metadata", "name"))
			// The ConfigHub root is not a layer. Its source says which Space
			// holds this cluster's layers, and that Space's name says which
			// cluster it is: the layers that carried cluster_name are gone.
			if dname == RootName {
				if cn := str(get(d.Value, "metadata", "labels", RootClusterLabel)); cn != "" {
					rootName = cn
				}
				if kind == "OCIRepository" {
					url := str(get(d.Value, "spec", "url"))
					if i := strings.Index(url, "/space/"); i >= 0 {
						layersSpace = strings.TrimSuffix(url[i+len("/space/"):], "/")
					}
					pullSecret = str(get(d.Value, "spec", "secretRef", "name"))
				} else if layersSpace == "" {
					layersSpace = DeliverySpace(b.opts.Prefix, name)
				}
				continue
			}
			if !strings.HasPrefix(api, kustomizeToolkit) {
				continue
			}
			byName[dname] = d
			if cn := str(get(d.Value, "spec", "postBuild", "substitute", "cluster_name")); cn != "" {
				name = cn
			}
		}
		if layersSpace != "" {
			// The root names its cluster. A root made before it did is read by
			// its Space's name, under this prefix or the one its Secret was
			// made under.
			// The Secret's prefix goes first: this one may extend it.
			prefixes := []string{b.opts.Prefix}
			if old, ok := strings.CutPrefix(pullSecret, "confighub-"); ok {
				if old, ok = strings.CutSuffix(old, "-targets"); ok && old != b.opts.Prefix {
					prefixes = []string{old, b.opts.Prefix}
				}
			}
			if rootName != "" && len(byName) == 0 {
				name = rootName
			} else if len(byName) == 0 {
				for _, pre := range prefixes {
					if cn, ok := strings.CutPrefix(layersSpace, pre+"-"); ok {
						if cn, ok = strings.CutSuffix(cn, "-layers"); ok {
							name = cn
							break
						}
					}
				}
			}
			if len(byName) > 0 {
				var still []string
				for n := range byName {
					still = append(still, n)
				}
				sort.Strings(still)
				p.Problems = append(p.Problems, fmt.Sprintf("%s holds the ConfigHub root and still defines %s: the handover commit is half made. Finish it, removing those files, or revert it", b.rel(cdir), strings.Join(still, ", ")))
			}
			b.clusters = append(b.clusters, Cluster{Name: name, Dir: e.Name(), Path: b.rel(cdir), LayersSpace: layersSpace, PullSecret: pullSecret})
			continue
		}
		if len(byName) == 0 {
			continue
		}
		b.clusters = append(b.clusters, Cluster{Name: name, Dir: e.Name(), Path: b.rel(cdir)})
		layers[name] = byName
	}
	if len(b.clusters) == 0 && len(p.Problems) == 0 {
		p.Problems = append(p.Problems, fmt.Sprintf("no Flux Kustomizations under %s/*", b.opts.ClustersDir))
	}
	return layers
}

// stages orders the clusters; each cluster is its own stage.
func (b *builder) stages() {
	p := b.plan
	order := b.opts.Stages
	if len(order) == 0 {
		rank := map[string]int{"dev": 0, "staging": 1, "prod": 2}
		sort.SliceStable(b.clusters, func(i, j int) bool {
			ri, oki := rank[b.clusters[i].Dir]
			rj, okj := rank[b.clusters[j].Dir]
			switch {
			case oki && okj:
				return ri < rj
			case oki != okj:
				return oki
			}
			return b.clusters[i].Dir < b.clusters[j].Dir
		})
		for i := range b.clusters {
			b.clusters[i].Stage = b.clusters[i].Dir
			p.Stages = append(p.Stages, b.clusters[i].Dir)
		}
	} else {
		p.Stages = order
		pos := map[string]int{}
		for i, s := range order {
			pos[s] = i
		}
		var kept []Cluster
		for _, c := range b.clusters {
			stage := c.Dir
			if _, ok := pos[stage]; !ok {
				stage = c.Name
			}
			if _, ok := pos[stage]; !ok {
				p.Problems = append(p.Problems, fmt.Sprintf("cluster %s (%s) is not one of the stages (%s)", c.Name, c.Dir, strings.Join(order, ", ")))
				continue
			}
			c.Stage = stage
			kept = append(kept, c)
		}
		sort.SliceStable(kept, func(i, j int) bool { return pos[kept[i].Stage] < pos[kept[j].Stage] })
		b.clusters = kept
	}
	p.Clusters = b.clusters
}

// components groups the clusters' layers by name, infers each layer's base
// from what every overlay builds on, and lists each variant's departures.
func (b *builder) components(layers map[string]map[string]Doc) {
	p := b.plan
	names := map[string]bool{}
	for _, byName := range layers {
		for n := range byName {
			names[n] = true
		}
	}
	for name := range names {
		c := &Component{Name: name, Base: fmt.Sprintf("%s-%s-base", b.opts.Prefix, name)}
		var variants []*Variant
		for _, cl := range b.clusters {
			d, ok := layers[cl.Name][name]
			if !ok {
				continue
			}
			v := b.variant(c, cl, d)
			variants = append(variants, v)
			for _, dep := range list(get(d.Value, "spec", "dependsOn")) {
				c.DependsOn = appendOnce(c.DependsOn, str(get(dep, "name")))
			}
		}
		sort.Strings(c.DependsOn)
		b.inferBase(c, variants)
		b.layerDiffs(variants)
		for _, st := range p.Stages {
			s := Stage{Name: st}
			for _, v := range variants {
				if v.stage == st {
					s.Variants = append(s.Variants, *v)
				}
			}
			if len(s.Variants) > 0 {
				c.Stages = append(c.Stages, s)
			}
		}
		// A handed-over cluster runs its layers from ConfigHub, not from this
		// repository, so it is left out of saying where a layer runs.
		inGit := 0
		for _, cl := range b.clusters {
			if cl.LayersSpace == "" {
				inGit++
			}
		}
		if len(variants) < inGit {
			var on []string
			for _, v := range variants {
				on = append(on, v.Cluster)
			}
			c.Notes = append(c.Notes, "only on "+strings.Join(on, ", "))
		}
		c.InFlight = inFlight(c)
		c.HelmReleases = b.helmReleases(c)
		c.Require = b.opts.Require
		p.Components = append(p.Components, c)
	}
}

func appendOnce(l []string, s string) []string {
	for _, x := range l {
		if x == s {
			return l
		}
	}
	return append(l, s)
}

func (b *builder) variant(c *Component, cl Cluster, d Doc) *Variant {
	p := b.plan
	spec := obj(d.Value["spec"])
	path := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(str(spec["path"]))), "./")
	v := &Variant{
		Cluster:         cl.Name,
		Space:           fmt.Sprintf("%s-%s-%s", b.opts.Prefix, c.Name, cl.Name),
		Target:          fmt.Sprintf("%s-targets/%s", b.opts.Prefix, cl.Name),
		Kustomization:   str(get(d.Value, "metadata", "namespace")) + "/" + str(get(d.Value, "metadata", "name")),
		Path:            path,
		TargetNamespace: str(spec["targetNamespace"]),
		stage:           cl.Stage,
		layer:           d.Value,
		layerFile:       b.rel(d.File),
	}
	local := filepath.Join(b.root, filepath.FromSlash(path))
	if info, err := os.Stat(local); err != nil || !info.IsDir() {
		p.Problems = append(p.Problems, fmt.Sprintf("%s on %s: path %s does not exist in this checkout, so Flux would fail to build it", c.Name, cl.Name, path))
		return v
	}
	k, raw := readKustomization(local)
	v.overlay = k
	if k == nil {
		v.Departures = append(v.Departures, "plain manifests, no kustomization.yaml")
		return v
	}
	for _, r := range list(k["resources"]) {
		sub := filepath.Clean(filepath.Join(local, filepath.FromSlash(str(r))))
		if info, err := os.Stat(sub); err == nil && info.IsDir() {
			v.resourceDirs = append(v.resourceDirs, b.rel(sub))
		} else if err == nil {
			v.Departures = append(v.Departures, "adds "+str(r))
		}
	}
	if ns := str(k["namespace"]); ns != "" {
		v.Departures = append(v.Departures, "namespace "+ns)
	}
	for _, l := range list(k["labels"]) {
		pairs := strMap(get(l, "pairs"))
		keys := sortedKeys(pairs)
		for _, key := range keys {
			v.Departures = append(v.Departures, fmt.Sprintf("label %s=%s", key, pairs[key]))
		}
	}
	automated := strings.Contains(raw, "$imagepolicy")
	for _, i := range list(k["images"]) {
		name, tag := str(get(i, "name")), str(get(i, "newTag"))
		if nn := str(get(i, "newName")); nn != "" {
			tag = nn + ":" + tag
		}
		if v.Images == nil {
			v.Images = map[string]string{}
		}
		v.Images[name] = tag
		line := fmt.Sprintf("image %s %s", name, tag)
		if automated {
			line += " (written by image automation)"
		}
		v.Departures = append(v.Departures, line)
	}
	for _, pt := range list(k["patches"]) {
		v.Departures = append(v.Departures, describePatch(pt)...)
	}
	if len(list(k["patchesStrategicMerge"])) > 0 || len(list(k["patchesJson6902"])) > 0 {
		v.Departures = append(v.Departures, "legacy patch fields (patchesStrategicMerge, patchesJson6902)")
	}
	subs := strMap(get(spec, "postBuild", "substitute"))
	for _, key := range sortedKeys(subs) {
		v.Departures = append(v.Departures, fmt.Sprintf("substitutes %s=%s", key, subs[key]))
	}
	for _, from := range list(get(spec, "postBuild", "substituteFrom")) {
		v.Departures = append(v.Departures, fmt.Sprintf("substitutes from %s %s", str(get(from, "kind")), str(get(from, "name"))))
	}
	return v
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// describePatch turns one kustomize patch into readable departures.
func describePatch(pt any) []string {
	target := str(get(pt, "target", "kind")) + "/" + str(get(pt, "target", "name"))
	var body any
	if err := yaml.Unmarshal([]byte(str(get(pt, "patch"))), &body); err != nil {
		return []string{target + ": a patch that does not parse"}
	}
	ops := list(body)
	if ops == nil {
		return []string{target + ": a strategic merge patch"}
	}
	var out []string
	for _, o := range ops {
		op, path := str(get(o, "op")), str(get(o, "path"))
		if val, ok := obj(o)["value"]; ok {
			out = append(out, fmt.Sprintf("%s %s %s = %s", target, op, path, str(val)))
		} else {
			out = append(out, fmt.Sprintf("%s %s %s", target, op, path))
		}
	}
	return out
}

// inferBase picks the directory every variant's overlay builds on. With one
// variant, or no shared directory, it says what it chose instead.
func (b *builder) inferBase(c *Component, variants []*Variant) {
	if len(variants) == 0 {
		return
	}
	common := map[string]int{}
	for _, v := range variants {
		for _, d := range v.resourceDirs {
			common[d]++
		}
	}
	var shared []string
	for d, n := range common {
		if n == len(variants) {
			shared = append(shared, d)
		}
	}
	sort.Strings(shared)
	switch {
	case len(shared) > 0:
		c.BaseDir = strings.Join(shared, ", ")
	case len(variants) == 1:
		c.BaseDir = variants[0].Path
		c.Notes = append(c.Notes, "one cluster uses this layer, so its whole directory is the base")
	default:
		c.BaseDir = variants[0].Path
		c.Notes = append(c.Notes, "the overlays share no base directory; the plan would make "+variants[0].Cluster+"'s the base")
	}
}

// layerDiffs adds the fields of the Flux Kustomization itself that differ
// between clusters, other than path and postBuild, which are listed already.
func (b *builder) layerDiffs(variants []*Variant) {
	if len(variants) < 2 {
		return
	}
	flat := make([]map[string]string, len(variants))
	keys := map[string]bool{}
	for i, v := range variants {
		flat[i] = map[string]string{}
		flattenAny("spec", obj(v.layer["spec"]), flat[i])
		for k := range flat[i] {
			keys[k] = true
		}
	}
	var diff []string
	for k := range keys {
		if k == "spec.path" || strings.HasPrefix(k, "spec.postBuild.") {
			continue
		}
		first, ok := flat[0][k]
		for _, f := range flat[1:] {
			if val, has := f[k]; has != ok || val != first {
				diff = append(diff, k)
				break
			}
		}
	}
	sort.Strings(diff)
	for i, v := range variants {
		for _, k := range diff {
			if val, ok := flat[i][k]; ok {
				v.Departures = append(v.Departures, fmt.Sprintf("Flux %s = %s", k, val))
			}
		}
	}
}

func flattenAny(prefix string, v any, out map[string]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			flattenAny(prefix+"."+k, val, out)
		}
	case []any:
		for i, val := range t {
			flattenAny(fmt.Sprintf("%s[%d]", prefix, i), val, out)
		}
	default:
		out[prefix] = str(t)
	}
}

func inFlight(c *Component) []string {
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
	var out []string
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
			out = append(out, img+": "+strings.Join(parts, "; "))
		}
	}
	return out
}

// baseDirs are the absolute directories a component's base covers.
func (b *builder) baseDirs(c *Component) []string {
	var out []string
	for _, d := range strings.Split(c.BaseDir, ", ") {
		if d != "" {
			out = append(out, filepath.Join(b.root, filepath.FromSlash(d)))
		}
	}
	return out
}

func (b *builder) inBase(c *Component, file string) bool {
	for _, d := range b.baseDirs(c) {
		if under(file, d) {
			return true
		}
	}
	return false
}

func (b *builder) helmReleases(c *Component) []string {
	var out []string
	for _, d := range b.docs {
		if str(d.Value["kind"]) != "HelmRelease" || !b.inBase(c, d.File) {
			continue
		}
		spec := obj(get(d.Value, "spec", "chart", "spec"))
		chart, version := str(spec["chart"]), str(spec["version"])
		out = append(out, fmt.Sprintf("%s: chart %s %s from %s %s", str(get(d.Value, "metadata", "name")),
			chart, version, str(get(spec, "sourceRef", "kind")), str(get(spec, "sourceRef", "name"))))

		// A range is not a problem to fix before onboarding: the HelmRelease is
		// stored as it is, helm-controller goes on resolving it, and the
		// handover does not change that. It is a governance gap worth naming,
		// because what runs can change with nothing changing in ConfigHub. It
		// would be fatal only if this chart were flattened, which needs one
		// exact version, and that is why no verdict can be looked up for it.
		if !chartrender.ExactVersion(version) {
			b.plan.NotInGit = appendOnce(b.plan.NotInGit, fmt.Sprintf(
				"which chart %s runs: HelmRelease %s pins it to %q, so helm-controller decides at reconcile time and what runs can change with no change in ConfigHub. Pin one exact version to close that, and to make a flattening verdict possible",
				chart, str(get(d.Value, "metadata", "name")), version))
			continue
		}
		// What the Workshop Catalog has already decided about this chart, per
		// audited values base.
		for _, line := range catalog.Describe(chart, version) {
			b.plan.NotInGit = appendOnce(b.plan.NotInGit, line)
		}
	}
	sort.Strings(out)
	if len(out) > 0 {
		b.plan.NotInGit = appendOnce(b.plan.NotInGit, "what the HelmReleases in "+c.Name+" render: the charts are pulled at reconcile time")
	}
	return out
}

// order lays the components out in the order dependsOn makes Flux reconcile
// them: each step waits for the ones before it.
func (b *builder) order() {
	p := b.plan
	done := map[string]bool{}
	byName := map[string]*Component{}
	for _, c := range p.Components {
		byName[c.Name] = c
	}
	for len(done) < len(p.Components) {
		var step []string
		for _, c := range p.Components {
			if done[c.Name] {
				continue
			}
			ready := true
			for _, dep := range c.DependsOn {
				if _, known := byName[dep]; known && !done[dep] {
					ready = false
				}
			}
			if ready {
				step = append(step, c.Name)
			}
		}
		if len(step) == 0 {
			p.Problems = append(p.Problems, "dependsOn forms a cycle between the layers")
			break
		}
		sort.Strings(step)
		for _, s := range step {
			done[s] = true
		}
		p.Order = append(p.Order, step)
	}
	rank := map[string]int{}
	i := 0
	for _, step := range p.Order {
		for _, s := range step {
			rank[s] = i
			i++
		}
	}
	sort.SliceStable(p.Components, func(i, j int) bool { return rank[p.Components[i].Name] < rank[p.Components[j].Name] })
	for _, c := range p.Components {
		for _, dep := range c.DependsOn {
			if byName[dep] == nil {
				p.Problems = append(p.Problems, fmt.Sprintf("%s depends on %s, which no cluster defines", c.Name, dep))
			}
		}
	}
}

// sources lists the Git and Helm sources the layers deploy, and the branch
// each cluster reads a GitRepository at after its overlay's patches.
func (b *builder) sources() {
	p := b.plan
	for _, d := range b.docs {
		kind := str(d.Value["kind"])
		if kind != "GitRepository" && kind != "HelmRepository" && kind != "OCIRepository" {
			continue
		}
		name := str(get(d.Value, "metadata", "name"))
		s := Source{Kind: kind, Name: name, URL: str(get(d.Value, "spec", "url")), File: b.rel(d.File)}
		if kind == "GitRepository" {
			base := str(get(d.Value, "spec", "ref", "branch"))
			for _, c := range p.Components {
				if !b.inBase(c, d.File) {
					continue
				}
				for _, st := range c.Stages {
					for _, v := range st.Variants {
						branch := base
						for _, pt := range list(get(v.overlay, "patches")) {
							if str(get(pt, "target", "kind")) == "GitRepository" && str(get(pt, "target", "name")) == name {
								if nb := patchedValue(pt, "/spec/ref/branch"); nb != "" {
									branch = nb
								}
							}
						}
						if s.Branches == nil {
							s.Branches = map[string]string{}
						}
						s.Branches[v.Cluster] = branch
					}
				}
			}
		}
		p.Sources = append(p.Sources, s)
	}
	sort.Slice(p.Sources, func(i, j int) bool { return p.Sources[i].File+p.Sources[i].Name < p.Sources[j].File+p.Sources[j].Name })
}

func patchedValue(pt any, path string) string {
	var body any
	if err := yaml.Unmarshal([]byte(str(get(pt, "patch"))), &body); err != nil {
		return ""
	}
	for _, o := range list(body) {
		if str(get(o, "path")) == path {
			return str(get(o, "value"))
		}
	}
	return ""
}

// automation lists what writes to Git by itself.
func (b *builder) automation() {
	p := b.plan
	for _, d := range b.docs {
		if str(d.Value["kind"]) != "ImageUpdateAutomation" {
			continue
		}
		spec := obj(d.Value["spec"])
		var where []string
		for _, c := range p.Components {
			if b.inBase(c, d.File) {
				for _, st := range c.Stages {
					for _, v := range st.Variants {
						where = append(where, v.Cluster)
					}
				}
			}
		}
		line := fmt.Sprintf("ImageUpdateAutomation %s commits to branch %s, path %s",
			str(get(d.Value, "metadata", "name")), str(get(spec, "git", "push", "branch")),
			strings.TrimPrefix(str(get(spec, "update", "path")), "./"))
		if len(where) > 0 {
			line += ", running on " + strings.Join(where, ", ")
		}
		p.Automation = append(p.Automation, line)
	}
}

// tenants lists Flux Kustomizations inside the layers that run as their own
// service account: a tenant boundary that stays as it is.
func (b *builder) tenants() {
	p := b.plan
	for _, d := range b.docs {
		if !strings.HasPrefix(str(d.Value["apiVersion"]), kustomizeToolkit) {
			continue
		}
		sa := str(get(d.Value, "spec", "serviceAccountName"))
		if sa == "" {
			continue
		}
		ns := str(get(d.Value, "metadata", "namespace"))
		line := fmt.Sprintf("%s/%s runs as ServiceAccount %s, reading %s %s",
			ns, str(get(d.Value, "metadata", "name")), sa,
			str(get(d.Value, "spec", "sourceRef", "kind")), str(get(d.Value, "spec", "sourceRef", "name")))
		for _, c := range p.Components {
			if b.inBase(c, d.File) {
				line += "; governed as part of " + c.Name
			}
		}
		p.Tenants = append(p.Tenants, line)
	}
	sort.Strings(p.Tenants)
}

var substVar = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:?[-=][^}]*)?\}`)

// substitutions checks that every ${var} a layer's manifests use is supplied
// by that cluster's postBuild, and lists the values that live only there.
func (b *builder) substitutions() {
	p := b.plan
	for _, c := range p.Components {
		used := map[string]bool{}
		for _, dir := range b.baseDirs(c) {
			_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || (filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml") {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				for _, m := range substVar.FindAllStringSubmatch(string(data), -1) {
					if m[2] == "" {
						used[m[1]] = true
					}
				}
				return nil
			})
		}
		if len(used) == 0 {
			continue
		}
		var vars []string
		for v := range used {
			vars = append(vars, v)
		}
		sort.Strings(vars)
		p.NotInGit = append(p.NotInGit, fmt.Sprintf("%s: %s, supplied per cluster by postBuild", c.Name, "${"+strings.Join(vars, "}, ${")+"}"))
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				if len(list(get(v.layer, "spec", "postBuild", "substituteFrom"))) > 0 {
					continue
				}
				subs := strMap(get(v.layer, "spec", "postBuild", "substitute"))
				for _, name := range vars {
					if _, ok := subs[name]; !ok {
						p.Problems = append(p.Problems, fmt.Sprintf("%s on %s: the manifests use ${%s}, which this cluster's postBuild does not supply, so Flux would apply it literally", c.Name, v.Cluster, name))
					}
				}
			}
		}
	}
}

// handover says what handing the fleet to ConfigHub would involve. Nothing
// here runs; `apply` will write it as handover.sh.
func (b *builder) handover() {
	p := b.plan
	if len(p.Components) == 0 {
		return
	}
	var pruned []string
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				if get(v.layer, "spec", "prune") == true {
					pruned = appendOnce(pruned, c.Name)
				}
			}
		}
	}
	p.Handover = append(p.Handover, "keep each layer's Flux Kustomization under its own name and switch its sourceRef to an OCIRepository on the ConfigHub gateway, so Flux keeps its inventory and nothing is reinstalled")
	if len(pruned) > 0 {
		p.Handover = append(p.Handover, fmt.Sprintf("first prove each variant renders exactly what Git renders today: %s prune, so anything the release lacks is deleted", strings.Join(pruned, ", ")))
	}
	for _, a := range p.Automation {
		name := strings.Fields(a)[1]
		p.Handover = append(p.Handover, fmt.Sprintf("suspend ImageUpdateAutomation %s: it would commit to Git that no longer deploys; its tag should become a proposed ConfigHub change on that variant", name))
	}
	for _, s := range p.Sources {
		seen := map[string]bool{}
		for _, br := range s.Branches {
			seen[br] = true
		}
		if len(seen) > 1 {
			var parts []string
			for _, cl := range p.Clusters {
				if br, ok := s.Branches[cl.Name]; ok {
					parts = append(parts, cl.Name+" "+br)
				}
			}
			p.Handover = append(p.Handover, fmt.Sprintf("GitRepository %s promotes by branch (%s); after handover the ChangeWorkflow's stages decide, not the branch", s.Name, strings.Join(parts, ", ")))
		}
	}
	if len(p.Boundary) > 0 {
		p.Handover = append(p.Handover, "leave flux-system alone: flux bootstrap owns it, like the Sveltos management record")
	}
}

func readKustomization(dir string) (map[string]any, string) {
	for _, name := range []string{"kustomization.yaml", "kustomization.yml", "Kustomization"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		docs, err := parse(data, "")
		if err != nil || len(docs) == 0 {
			return nil, ""
		}
		return docs[0].Value, string(data)
	}
	return nil, ""
}
