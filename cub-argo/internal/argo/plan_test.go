package argo

import (
	"flag"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden plan files")

// The expert Argo CD example lives beside this module in the examples repo.
const example = "../../../gitops/argo/expert-app-of-apps"

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

var staged = Options{StageLabel: "rollout-phase", Stages: []string{"canary", "secondary", "primary"}}

func planOf(t *testing.T, dir string, opts Options) *Plan {
	t.Helper()
	in, err := Load(nil, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Build(in, opts)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExpertAppOfAppsGolden(t *testing.T) {
	opts := staged
	opts.RepoRoot = repoRoot(t)
	p := planOf(t, example, opts)
	got := Render(p)
	golden := filepath.Join("..", "..", "testdata", "expert-app-of-apps.plan.txt")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("plan differs from %s; rerun with -update and review the diff\n%s", golden, got)
	}
	if len(p.Problems) > 0 {
		t.Errorf("the example should plan cleanly, got problems: %v", p.Problems)
	}
}

// TestExpertAppOfAppsAnswers checks the questions the example's README says a
// tool must answer.
func TestExpertAppOfAppsAnswers(t *testing.T) {
	opts := staged
	opts.RepoRoot = repoRoot(t)
	p := planOf(t, example, opts)

	if len(p.Clusters) != 3 {
		t.Errorf("clusters = %d, want 3", len(p.Clusters))
	}
	if len(p.Tree) != 1 || p.Tree[0].Name != "root" {
		t.Fatalf("want one root Application, got %+v", p.Tree)
	}
	names := map[string]*Component{}
	for _, c := range p.Components {
		names[c.Name] = c
	}
	for _, want := range []string{"apptique", "checkout-cache", "platform-addons-cluster-baseline"} {
		if names[want] == nil {
			t.Errorf("missing component %s", want)
		}
	}
	if c := names["apptique"]; c != nil {
		if len(c.InFlight) != 1 || !strings.Contains(c.InFlight[0], "1.26-alpine on prod-1") {
			t.Errorf("apptique should be in flight with prod-1 behind, got %v", c.InFlight)
		}
		if c.Owner != "storefront" {
			t.Errorf("apptique owner = %q, want storefront", c.Owner)
		}
	}
	if c := names["checkout-cache"]; c != nil && !strings.Contains(strings.Join(c.Notes, " "), "--enable-helm") {
		t.Errorf("checkout-cache should carry the --enable-helm note, got %v", c.Notes)
	}
	if len(p.Windows) != 1 || strings.Join(p.Windows[0].Applications, ",") != "prod-1-apptique,prod-1-checkout-cache" {
		t.Errorf("the storefront window should cover the two prod-1 storefront apps, got %+v", p.Windows)
	}
}

// copyExample copies the example into a fresh checkout at the same repository
// path, so Applications' source paths resolve the same way.
func copyExample(t *testing.T) (root, dir string) {
	t.Helper()
	root = t.TempDir()
	dir = filepath.Join(root, "gitops", "argo", "expert-app-of-apps")
	err := filepath.WalkDir(example, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(example, p)
		dst := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root, dir
}

func edit(t *testing.T, file, old, new string) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), old) {
		t.Fatalf("%s does not contain %q", file, old)
	}
	if err := os.WriteFile(file, []byte(strings.Replace(string(data), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasProblem(p *Plan, parts ...string) bool {
	for _, pr := range p.Problems {
		ok := true
		for _, part := range parts {
			ok = ok && strings.Contains(pr, part)
		}
		if ok {
			return true
		}
	}
	return false
}

// Break it on purpose, number 1 in the example's README: a wrong env label
// points the generators at an overlay that does not exist.
func TestBadPathIsCaught(t *testing.T) {
	root, dir := copyExample(t)
	edit(t, filepath.Join(dir, "clusters", "prod-1.yaml"), "env: prod", "env: production")
	opts := staged
	opts.RepoRoot = root
	p := planOf(t, dir, opts)
	if !hasProblem(p, "apptique on prod-1", "overlays/production does not exist") {
		t.Errorf("want a missing-path problem for apptique on prod-1, got %v", p.Problems)
	}
}

// A cluster without the label a template reads renders "<no value>": index
// returns nothing for a missing key, so missingkey=error does not refuse it.
func TestMissingLabelIsCaught(t *testing.T) {
	root, dir := copyExample(t)
	edit(t, filepath.Join(dir, "clusters", "staging-1.yaml"), "    env: staging\n", "")
	opts := staged
	opts.RepoRoot = root
	p := planOf(t, dir, opts)
	if !hasProblem(p, "apptique on staging-1", "spec.destination.namespace", `"storefront-<no value>"`) {
		t.Errorf("want an empty-namespace problem for apptique on staging-1, got %v", p.Problems)
	}
	for _, c := range p.Components {
		if c.Name != "platform-addons-cluster-baseline" {
			continue
		}
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				if v.Cluster == "staging-1" {
					t.Errorf("platform-addons selects env Exists, so it should skip staging-1")
				}
			}
		}
	}
}

// A live export has generated Applications and no files: the plan names them
// and says handover is needed.
func TestLiveExport(t *testing.T) {
	export := `
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Secret
  metadata:
    name: cluster-a
    namespace: argocd
    labels: {argocd.argoproj.io/secret-type: cluster, env: prod}
  data:
    name: YQ==
    server: aHR0cHM6Ly9h
    config: c2VjcmV0
- apiVersion: argoproj.io/v1alpha1
  kind: ApplicationSet
  metadata: {name: web, namespace: argocd}
  spec:
    generators:
    - clusters: {selector: {matchLabels: {env: prod}}}
    template:
      metadata: {name: '{{name}}-web'}
      spec:
        project: default
        source: {repoURL: https://example.com/repo, path: 'web/{{metadata.labels.env}}'}
        destination: {server: '{{server}}', namespace: web}
- apiVersion: argoproj.io/v1alpha1
  kind: Application
  metadata:
    name: a-web
    namespace: argocd
    ownerReferences: [{kind: ApplicationSet, name: web}]
  spec: {project: default}
`
	in, err := Load(strings.NewReader(export), []string{"-"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Build(in, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Problems) > 0 {
		t.Fatalf("unexpected problems: %v", p.Problems)
	}
	if len(p.Clusters) != 1 || p.Clusters[0].Name != "a" || p.Clusters[0].Server != "https://a" {
		t.Errorf("cluster from base64 data: got %+v", p.Clusters)
	}
	if len(p.Live) != 1 {
		t.Errorf("want one live Application, got %v", p.Live)
	}
	v := p.Components[0].Stages[0].Variants[0]
	if v.Application != "a-web" || v.Path != "web/prod" {
		t.Errorf("fasttemplate render: got %+v", v)
	}
	if strings.Contains(Render(p), "c2VjcmV0") {
		t.Errorf("the plan must never show cluster credentials")
	}
}

func TestSelects(t *testing.T) {
	labels := map[string]string{"env": "prod", "tier": "gold"}
	cases := []struct {
		sel  map[string]any
		want bool
	}{
		{map[string]any{}, true},
		{map[string]any{"matchLabels": map[string]any{"env": "prod"}}, true},
		{map[string]any{"matchLabels": map[string]any{"env": "dev"}}, false},
		{map[string]any{"matchExpressions": []any{map[string]any{"key": "env", "operator": "In", "values": []any{"dev", "prod"}}}}, true},
		{map[string]any{"matchExpressions": []any{map[string]any{"key": "env", "operator": "NotIn", "values": []any{"prod"}}}}, false},
		{map[string]any{"matchExpressions": []any{map[string]any{"key": "zone", "operator": "Exists"}}}, false},
		{map[string]any{"matchExpressions": []any{map[string]any{"key": "zone", "operator": "DoesNotExist"}}}, true},
	}
	for i, c := range cases {
		got, err := selects(c.sel, labels)
		if err != nil || got != c.want {
			t.Errorf("case %d: got %v, %v; want %v", i, got, err, c.want)
		}
	}
}

// Without a checkout the source paths cannot be checked at all. A plan that
// reported nothing would look clean while having looked at nothing, so it
// says so instead.
func TestNoCheckoutIsReported(t *testing.T) {
	_, dir := copyExample(t)
	p := planOf(t, dir, Options{}) // no RepoRoot, and the copy has no .git
	if !hasProblem(p, "no repository checkout found", "--repo-root") {
		t.Errorf("want a skipped-path-check problem, got %v", p.Problems)
	}
}

// Handover repoints each layer rather than orphaning it, in an order that
// publishes a Space before the parent that syncs it is repointed.
func TestHandoverRepointsTopDown(t *testing.T) {
	opts := staged
	opts.RepoRoot = repoRoot(t)
	p := planOf(t, example, opts)

	joined := strings.Join(p.Handover, "\n")
	if strings.Contains(joined, "--cascade=orphan") {
		t.Errorf("every object here has a parent or a template, so nothing should be orphaned:\n%s", joined)
	}
	for _, want := range []string{
		"sourceRepos",                    // the projects gate every repoint
		"v3.1 or newer",                  // oci:// needs it
		"repoint Application root",       // hand-applied, patched in the cluster
		"repoint Application storefront", // a Unit by then, so promoted
		"template of ApplicationSet",     // generated apps are never touched
		"never delete root, storefront",  // the finalizer
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("handover is missing %q:\n%s", want, joined)
		}
	}
	// A parent must be published before whoever syncs it is repointed.
	root := strings.Index(joined, "repoint Application root")
	store := strings.Index(joined, "repoint Application storefront")
	if root < 0 || store < 0 || root > store {
		t.Errorf("root should be repointed before storefront, got %d and %d", root, store)
	}
	// The repoint of a child that is already a Unit goes through approval.
	if !strings.Contains(joined, "repoint it there and promote") {
		t.Errorf("a child layer's repoint should itself be reviewed:\n%s", joined)
	}
}

// writeApply runs apply into a temp dir and returns it.
func writeApply(t *testing.T) string {
	t.Helper()
	opts := staged
	opts.RepoRoot = repoRoot(t)
	p := planOf(t, example, opts)
	dir := t.TempDir()
	if _, err := WriteApply(p, "argo", dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestApplyGolden(t *testing.T) {
	dir := writeApply(t)
	for _, name := range []string{"apply.sh", "handover.sh"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		// The repo path differs per machine, so it is not part of the golden.
		body := regexp.MustCompile(`REPO_ROOT=\$\{REPO_ROOT:-'[^']*'\}`).
			ReplaceAll(got, []byte(`REPO_ROOT=${REPO_ROOT:-'<repo>'}`))
		golden := filepath.Join("..", "..", "testdata", name+".txt")
		if *update {
			if err := os.WriteFile(golden, body, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != string(want) {
			t.Errorf("%s differs from %s; rerun with -update and review the diff", name, golden)
		}
	}
}

// Both scripts must be valid bash, since a person is asked to read and run them.
func TestScriptsParseAsBash(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	dir := writeApply(t)
	for _, name := range []string{"apply.sh", "handover.sh"} {
		out, err := exec.Command("bash", "-n", filepath.Join(dir, name)).CombinedOutput()
		if err != nil {
			t.Errorf("%s is not valid bash: %v\n%s", name, err, out)
		}
	}
}

// Every path apply.sh renders has to exist in the repository, or the first
// step of a real run fails.
func TestEveryRenderPathExists(t *testing.T) {
	dir := writeApply(t)
	data, err := os.ReadFile(filepath.Join(dir, "apply.sh"))
	if err != nil {
		t.Fatal(err)
	}
	found := regexp.MustCompile(`(?m)^render '([^']+)'`).FindAllStringSubmatch(string(data), -1)
	if len(found) == 0 {
		t.Fatal("apply.sh renders nothing")
	}
	for _, m := range found {
		p := filepath.Join(repoRoot(t), filepath.FromSlash(m[1]))
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			t.Errorf("apply.sh would render %s, which is not a directory here", m[1])
		}
		if _, err := os.Stat(filepath.Join(p, "kustomization.yaml")); err != nil {
			t.Errorf("%s has no kustomization.yaml, so kustomize build would fail", m[1])
		}
	}
}

// The files apply.sh reads have to be written beside it.
func TestApplyWritesWhatItReads(t *testing.T) {
	dir := writeApply(t)
	data, err := os.ReadFile(filepath.Join(dir, "apply.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, re := range []string{`--filename (\S+)`, `cub unit create --space \S+ \S+ (control/\S+)`} {
		for _, m := range regexp.MustCompile(re).FindAllStringSubmatch(string(data), -1) {
			if _, err := os.Stat(filepath.Join(dir, m[1])); err != nil {
				t.Errorf("apply.sh reads %s, which apply did not write", m[1])
			}
		}
	}
}

// A shape with no ApplicationSet and no app of apps: two standalone
// Applications, one per environment, which CI promotes by pull request. It
// arrived in the repository after this plugin was written, so it is here to
// keep the plan honest about estates it was not designed against.
func TestStandaloneApplications(t *testing.T) {
	const dir = "../../../gitops/argo/intermediate-ci-to-gitops"
	if _, err := os.Stat(dir); err != nil {
		t.Skip("example not present")
	}
	opts := Options{RepoRoot: repoRoot(t)}
	p := planOf(t, dir, opts)

	if len(p.Tree) != 0 {
		t.Errorf("no Application here syncs another, so there is no control tree: %+v", p.Tree)
	}
	names := map[string]bool{}
	for _, c := range p.Components {
		if c.Kind != "Application" {
			t.Errorf("%s should be a standalone Application, got %s", c.Name, c.Kind)
		}
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				names[v.Application] = true
			}
		}
	}
	for _, want := range []string{"apptique-dev", "apptique-prod"} {
		if !names[want] {
			t.Errorf("missing variant for Application %s; got %v", want, names)
		}
	}
	// Both carry the finalizer, so the handover must say not to delete them.
	joined := strings.Join(p.Handover, "\n")
	if !strings.Contains(joined, "never delete apptique-dev, apptique-prod") {
		t.Errorf("the finalizer warning should name both Applications:\n%s", joined)
	}
	// Nothing syncs these two, so there is no parent to repoint them.
	if strings.Contains(joined, "repoint Application") {
		t.Errorf("neither Application has a parent, so none can be repointed under review:\n%s", joined)
	}
}

// What ConfigHub stores is a render, and the handover turns on comparing that
// render with what Git produces later. A chart that renders differently from
// one run to the next makes that comparison meaningless, so the plan stops.
func TestUnrepeatableRenderIsCaught(t *testing.T) {
	root, dir := copyExample(t)
	edit(t, filepath.Join(dir, "apps", "checkout-cache", "base", "kustomization.yaml"),
		"    version: 0.3.1", "    version: 0.3.x\n    repo: https://charts.example.com")
	opts := staged
	opts.RepoRoot = root
	p := planOf(t, dir, opts)
	if !hasProblem(p, "checkout-cache renders differently", "not one exact version") {
		t.Errorf("want an unrepeatable-render problem, got %v", p.Problems)
	}
}

func TestChartReproducibility(t *testing.T) {
	cases := []struct {
		c    chart
		want bool
	}{
		{chart{name: "a", version: "0.3.1"}, true},                                      // vendored, exact
		{chart{name: "a", version: "0.3.x"}, false},                                     // a range
		{chart{name: "a", version: "^1.2.0"}, false},                                    // a range
		{chart{name: "a"}, false},                                                       // no version at all
		{chart{name: "a", version: "0.3.1", repo: "https://charts.example.com"}, false}, // pulled at render time
	}
	for _, c := range cases {
		if got := c.c.reproducible(); got != c.want {
			t.Errorf("%+v: reproducible = %v, want %v", c.c, got, c.want)
		}
	}
}

// The commonest ApplicationSet after the cluster generator: one Application
// per directory, resolved against the checkout the way Argo resolves it
// against the repository.
func TestGitDirectoryGenerator(t *testing.T) {
	const dir = "../../../gitops/argo/beginner-applicationset"
	if _, err := os.Stat(dir); err != nil {
		t.Skip("example not present")
	}
	p := planOf(t, dir, Options{RepoRoot: repoRoot(t)})
	if len(p.Components) != 1 {
		t.Fatalf("want one component from the git generator, got %d", len(p.Components))
	}
	c := p.Components[0]
	apps, spaces := map[string]bool{}, map[string]bool{}
	for _, st := range c.Stages {
		for _, v := range st.Variants {
			apps[v.Application] = true
			if spaces[v.Space] {
				t.Errorf("two variants share Space %s; each needs its own", v.Space)
			}
			spaces[v.Space] = true
		}
	}
	for _, want := range []string{"apptique-dev", "apptique-prod"} {
		if !apps[want] {
			t.Errorf("git generator should produce %s; got %v", want, apps)
		}
	}
	if !strings.Contains(c.Generator, "2 directories") {
		t.Errorf("generator description should say what it matched, got %q", c.Generator)
	}
}

// Without a checkout the glob cannot be resolved, and the plan says so rather
// than producing nothing.
func TestGitGeneratorNeedsTheCheckout(t *testing.T) {
	const dir = "../../../gitops/argo/beginner-applicationset"
	if _, err := os.Stat(dir); err != nil {
		t.Skip("example not present")
	}
	in, err := Load(nil, []string{dir + "/bootstrap/applicationset.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Build(in, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(p.LeftOut, "\n"), "needs the checkout") {
		t.Errorf("want a left-out note naming the missing checkout, got %v", p.LeftOut)
	}
}

// An ApplicationSet that matches nothing used to vanish from the plan.
func TestApplicationSetThatSelectsNothingIsReported(t *testing.T) {
	const dir = "../../../gitops/argo/intermediate-git-as-database"
	if _, err := os.Stat(dir); err != nil {
		t.Skip("example not present")
	}
	p := planOf(t, dir, Options{RepoRoot: repoRoot(t)})
	for _, want := range []string{"canary", "mon"} {
		if !hasProblem(p, "ApplicationSet "+want, "selects nothing here") {
			t.Errorf("ApplicationSet %s matched no cluster and should be reported: %v", want, p.Problems)
		}
	}
}

// A Helm-chart source has no path to render, so the variant would be empty.
func TestHelmChartSourceIsReported(t *testing.T) {
	in, err := Load(strings.NewReader(`
apiVersion: v1
kind: Secret
metadata: {name: c1, namespace: argocd, labels: {argocd.argoproj.io/secret-type: cluster}}
stringData: {name: c1, server: https://c1}
---
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata: {name: mon, namespace: argocd}
spec:
  generators: [{clusters: {}}]
  template:
    metadata: {name: '{{name}}-mon'}
    spec:
      project: default
      source: {repoURL: https://charts.example.com, chart: mon, targetRevision: 1.4.0}
      destination: {server: '{{server}}', namespace: mon}
`), []string{"-"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Build(in, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasProblem(p, "deploys Helm chart", "variant would be empty") {
		t.Errorf("a chart source should be reported, not silently empty: %v", p.Problems)
	}
}

// A path with no kustomization.yaml renders for Argo but not for the script.
func TestPlainDirectoryIsReported(t *testing.T) {
	root, dir := copyExample(t)
	if err := os.Remove(filepath.Join(dir, "apps", "apptique", "overlays", "prod", "kustomization.yaml")); err != nil {
		t.Fatal(err)
	}
	opts := staged
	opts.RepoRoot = root
	p := planOf(t, dir, opts)
	if !hasProblem(p, "has no kustomization.yaml", "plain directory") {
		t.Errorf("want a plain-directory problem, got %v", p.Problems)
	}
}
