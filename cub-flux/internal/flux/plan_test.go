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
	for _, name := range []string{"apply.sh", "handover.sh"} {
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
		"OCIRepository",            // one per layer, applied to the bootstrap
		"confighub-flux-targets",   // the gateway credential
		"imageupdateautomation",    // suspended: it commits to a Git nobody reads
		"patch kustomization apps", // a layer that does move
		"same ",                    // the byte-equality check before any swap
	} {
		if !regexp.MustCompile(regexp.QuoteMeta(want)).MatchString(s) {
			t.Errorf("handover.sh is missing %q", want)
		}
	}
	// Layers are swapped in dependency order: infrastructure before apps.
	infra := regexp.MustCompile(`patch kustomization infrastructure`).FindStringIndex(s)
	apps := regexp.MustCompile(`patch kustomization apps`).FindStringIndex(s)
	if infra == nil || apps == nil || infra[0] > apps[0] {
		t.Error("infrastructure must be swapped before apps, which dependsOn it")
	}
}
