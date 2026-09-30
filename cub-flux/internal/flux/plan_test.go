package flux

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

// The expert Flux example lives beside this module in the examples repo.
const example = "../../../gitops/flux/expert-fleet"

func planOf(t *testing.T, dir, root string) *Plan {
	t.Helper()
	in, err := Load(nil, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Build(in, Options{RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestExpertFleetGolden(t *testing.T) {
	p := planOf(t, example, repoRoot(t))
	got := Render(p)
	golden := filepath.Join("..", "..", "testdata", "expert-fleet.plan.txt")
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

// TestExpertFleetAnswers checks the questions the example's README says a
// tool must answer.
func TestExpertFleetAnswers(t *testing.T) {
	p := planOf(t, example, repoRoot(t))
	var names []string
	for _, c := range p.Clusters {
		names = append(names, c.Name)
	}
	if strings.Join(names, ",") != "dev-1,staging-1,prod-1" {
		t.Errorf("clusters in stage order = %v", names)
	}
	var order []string
	for _, s := range p.Order {
		order = append(order, strings.Join(s, "+"))
	}
	if strings.Join(order, " ") != "infrastructure apps+tenants image-automation" {
		t.Errorf("reconcile order = %v", order)
	}
	byName := map[string]*Component{}
	for _, c := range p.Components {
		byName[c.Name] = c
	}
	if c := byName["apps"]; c == nil || c.BaseDir != "gitops/flux/expert-fleet/apps/base/apptique" {
		t.Errorf("apps base should be inferred as apps/base/apptique, got %+v", c)
	}
	if c := byName["apps"]; c == nil || len(c.InFlight) != 1 || !strings.Contains(c.InFlight[0], "1.26.3-alpine on prod-1") {
		t.Errorf("apps should be in flight with prod-1 behind")
	}
	if len(p.Automation) != 1 || !strings.Contains(p.Automation[0], "apps/dev") {
		t.Errorf("image automation writing apps/dev should be listed, got %v", p.Automation)
	}
	branches := map[string]string{}
	for _, s := range p.Sources {
		if s.Name == "fleet-repo" {
			branches = s.Branches
		}
	}
	if branches["prod-1"] != "production" || branches["dev-1"] != "main" {
		t.Errorf("fleet-repo branches = %v", branches)
	}
}

func copyExample(t *testing.T) (root, dir string) {
	t.Helper()
	root = t.TempDir()
	dir = filepath.Join(root, "gitops", "flux", "expert-fleet")
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

// A cluster that stops supplying a substituted value would get the literal
// ${environment} applied.
func TestMissingSubstitutionIsCaught(t *testing.T) {
	root, dir := copyExample(t)
	edit(t, filepath.Join(dir, "clusters", "prod", "apps.yaml"), "      environment: prod\n", "")
	p := planOf(t, dir, root)
	if !hasProblem(p, "apps on prod-1", "${environment}") {
		t.Errorf("want a missing-substitution problem, got %v", p.Problems)
	}
}

// A layer pointing at a directory that does not exist fails to build in Flux.
func TestBadPathIsCaught(t *testing.T) {
	root, dir := copyExample(t)
	edit(t, filepath.Join(dir, "clusters", "staging", "apps.yaml"), "apps/staging", "apps/stage")
	p := planOf(t, dir, root)
	if !hasProblem(p, "apps on staging-1", "apps/stage does not exist") {
		t.Errorf("want a missing-path problem, got %v", p.Problems)
	}
}

// Flux paths are written from the top of the repository, so without a
// checkout every path looks missing. The plan names that cause first, rather
// than reporting a healthy fleet as broken.
func TestGuessedRootIsExplained(t *testing.T) {
	_, dir := copyExample(t)
	p := planOf(t, dir, "") // no RepoRoot, and the copy has no .git
	if len(p.Problems) == 0 || !strings.Contains(p.Problems[0], "no repository checkout found") {
		t.Errorf("want the guessed-root explanation first, got %v", p.Problems)
	}
}

func writeApply(t *testing.T) string {
	t.Helper()
	in, err := Load(nil, []string{example})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Build(in, Options{RepoRoot: repoRoot(t)})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := WriteApply(p, "flux", dir); err != nil {
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

func TestScriptsParseAsBash(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	dir := writeApply(t)
	for _, name := range []string{"apply.sh", "handover.sh", "cleanup.sh"} {
		out, err := exec.Command("bash", "-n", filepath.Join(dir, name)).CombinedOutput()
		if err != nil {
			t.Errorf("%s is not valid bash: %v\n%s", name, err, out)
		}
	}
}

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
		if _, err := os.Stat(filepath.Join(p, "kustomization.yaml")); err != nil {
			t.Errorf("apply.sh would render %s, which has no kustomization.yaml", m[1])
		}
	}
}

// The bootstrap is the recovery path: it is never repointed, and the two
// things a layer needs before it can read ConfigHub go into it.
func TestHandoverLeavesTheBootstrapAlone(t *testing.T) {
	dir := writeApply(t)
	data, err := os.ReadFile(filepath.Join(dir, "handover.sh"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if regexp.MustCompile(`patch kustomization flux-system`).MatchString(s) {
		t.Error("flux-system reconciles the Flux controllers; it must never be repointed")
	}
	for _, want := range []string{
		"OCIRepository",          // the root's source, applied to the bootstrap
		"confighub-flux-targets", // the gateway credential
		"imageupdateautomation",  // suspended: it commits to a Git nobody reads
		"arrived 'apps'",         // a layer the root takes over
		"same ",                  // the byte-equality check before any swap
	} {
		if !regexp.MustCompile(regexp.QuoteMeta(want)).MatchString(s) {
			t.Errorf("handover.sh is missing %q", want)
		}
	}
	// Layers are swapped in dependency order: infrastructure before apps.
	infra := regexp.MustCompile(`(?m)^arrived 'infrastructure'`).FindStringIndex(s)
	apps := regexp.MustCompile(`(?m)^arrived 'apps'`).FindStringIndex(s)
	if infra == nil || apps == nil || infra[0] > apps[0] {
		t.Error("infrastructure must be waited for before apps, which dependsOn it")
	}
}

// A HelmRelease pinned to a range is a governance gap, not a reason to refuse
// onboarding: the object is stored as it is, helm-controller goes on resolving
// it, and the handover does not change that. It would be fatal only if the
// chart were flattened, which needs one exact version.
func TestUnpinnedHelmReleaseIsNamedNotRefused(t *testing.T) {
	p := planOf(t, example, repoRoot(t))
	if len(p.Problems) > 0 {
		t.Errorf("the example pins edge-router to 2.4.x and must still onboard: %v", p.Problems)
	}
	joined := strings.Join(p.NotInGit, "\n")
	if !strings.Contains(joined, "edge-router") || !strings.Contains(joined, "2.4.x") {
		t.Errorf("the range should be named under what is not in Git:\n%s", joined)
	}
	if !strings.Contains(joined, "Pin one exact version") {
		t.Errorf("the note should say what would close it:\n%s", joined)
	}
}

// A layer's targetNamespace is where an object whose manifest names no
// namespace lands. Without it the check cannot tell such an object from one
// the layer does not apply at all, and would report every one of them as both
// deleted and added.
func TestTargetNamespaceIsCarriedToTheCheck(t *testing.T) {
	const dir = "../../../gitops/flux/beginner"
	if _, err := os.Stat(dir); err != nil {
		t.Skip("example not present")
	}
	in, err := Load(nil, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Build(in, Options{RepoRoot: repoRoot(t)})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range p.Components {
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				got[c.Name+"/"+v.Cluster] = v.TargetNamespace
			}
		}
	}
	if got["apps/dev"] != "apptique-dev" {
		t.Errorf("apps on dev sets targetNamespace apptique-dev, got %q", got["apps/dev"])
	}
	if got["infrastructure/dev"] != "" {
		t.Errorf("infrastructure sets none, got %q", got["infrastructure/dev"])
	}

	// And the handover has to pass it, or reading it was pointless.
	dirOut := t.TempDir()
	if _, err := WriteApply(p, "flux", dirOut); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dirOut, "handover.sh"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "--target-namespace 'apptique-dev'") {
		t.Error("handover.sh must pass --target-namespace for the apps layer")
	}
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "kustomization 'infrastructure'") && strings.Contains(line, "target-namespace") {
			t.Errorf("infrastructure sets none, so the flag should be absent: %s", line)
		}
	}
}

// check works the estate out from the same input plan does, so a person needs
// no per-Kustomization flags.
func TestChecksAreDerivedFromThePlan(t *testing.T) {
	p := planOf(t, example, repoRoot(t))
	checks := ChecksFor(p)
	if len(checks) == 0 {
		t.Fatal("the expert fleet has layers, so it should yield checks")
	}
	seen := map[string]bool{}
	for _, c := range checks {
		if c.Kustomization == "" || c.Space == "" || c.Unit == "" {
			t.Errorf("a check needs a Kustomization, Space and unit: %+v", c)
		}
		// A layer name reaches kubectl, so it must not still carry the
		// namespace the plan writes it with.
		if strings.Contains(c.Kustomization, "/") {
			t.Errorf("%q is a namespace/name, not the name kubectl takes", c.Kustomization)
		}
		key := c.Cluster + "|" + c.Kustomization
		if seen[key] {
			t.Errorf("two checks for %s on %s", c.Kustomization, c.Cluster)
		}
		seen[key] = true
	}
}

// A layer path with no kustomization is read as kustomize-controller reads it:
// every .yaml and .yml below it, a subdirectory with its own kustomization
// taken whole. The scripts render it the same way, through their own build().
// Before, the plan passed it and apply.sh failed in kustomize build.
func TestPlainLayerIsReadAsFluxReadsIt(t *testing.T) {
	root, dir := copyExample(t)
	dev := filepath.Join(dir, "apps", "dev")
	if err := os.Remove(filepath.Join(dev, "kustomization.yaml")); err != nil {
		t.Fatal(err)
	}
	for f, body := range map[string]string{
		"nested/cm.yaml":                 "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: nested\n  namespace: apptique-dev\n",
		"kustomized/kustomization.yaml":  "resources: [cm.yaml]\n",
		"kustomized/cm.yaml":             "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: whole\n  namespace: apptique-dev\n",
		"kustomized/deeper/ignored.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: not-listed\n",
		"README.md":                      "not a manifest\n",
		".hidden.yaml":                   "not: kubernetes\n",
	} {
		path := filepath.Join(dev, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := planOf(t, dir, root)
	for _, pr := range p.Problems {
		if strings.Contains(pr, "apps/dev") {
			t.Errorf("a plain layer of objects is read, not refused: %s", pr)
		}
	}
	files, dirs := fluxManifests(dev)
	if got := strings.Join(files, ","); got != "namespace.yaml,nested/cm.yaml" {
		t.Errorf("files: %s", got)
	}
	if got := strings.Join(dirs, ","); got != "kustomized" {
		t.Errorf("whole directories: %s", got)
	}

	// The script's own build() renders it, if bash and kustomize are here.
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("no kustomize")
	}
	script := ApplyScript(p, "flux", ".")
	start := strings.Index(script, "build() {")
	end := strings.Index(script[start:], "\n}\n")
	fn := script[start : start+end+3]
	out, err := exec.Command("bash", "-c", "set -euo pipefail\n"+fn+"\nbuild "+dev).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, want := range []string{"name: apptique-dev", "name: nested", "name: whole"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("render should hold %q:\n%s", want, out)
		}
	}
	if strings.Contains(string(out), "not-listed") {
		t.Errorf("a directory with its own kustomization is built as it says, not walked:\n%s", out)
	}

	// A file Flux cannot decode fails its build, so the plan says so.
	if err := os.WriteFile(filepath.Join(dev, "values.yaml"), []byte("replicas: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := planOf(t, dir, root); !hasProblemText(p, "values.yaml is not Kubernetes YAML") {
		t.Errorf("want the stray file named: %v", p.Problems)
	}
}

func hasProblemText(p *Plan, s string) bool {
	for _, pr := range p.Problems {
		if strings.Contains(pr, s) {
			return true
		}
	}
	return false
}
