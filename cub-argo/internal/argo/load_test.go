package argo

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestLoadLeavesOutWhatGitIgnores is the var/ an example's setup.sh renders
// into: copies of the estate's own Applications, which git ignores and Argo CD
// never reads. Read as estate, they doubled every cluster and variant in the
// plan. A force-added file is tracked, so it is read, and a directory named on
// purpose is read even when git ignores it.
func TestLoadLeavesOutWhatGitIgnores(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	app := func(name string) string {
		return "apiVersion: argoproj.io/v1alpha1\nkind: Application\nmetadata:\n  name: " + name + "\n  namespace: argocd\n"
	}
	write := func(rel, data string) {
		t.Helper()
		p := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write(".gitignore", "var/\n*.local.yaml\n")
	write("estate/root.yaml", app("root"))
	write("estate/var/rendered-root.yaml", app("root")+"---\n"+app("rendered-only"))
	write("estate/forced.local.yaml", app("forced"))
	git("add", "-f", "estate/forced.local.yaml")

	names := func(in *Input) []string {
		var out []string
		for _, d := range in.Docs {
			out = append(out, str(get(d.Value, "metadata", "name")))
		}
		sort.Strings(out)
		return out
	}

	dir := filepath.Join(repo, "estate")
	in, err := Load(nil, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names(in), ","); got != "forced,root" {
		t.Errorf("read %s; want forced,root: var/ is ignored, forced.local.yaml is tracked", got)
	}
	if got := strings.Join(in.Ignored, ","); got != filepath.Join("var", "rendered-root.yaml") {
		t.Errorf("ignored = %q, want var/rendered-root.yaml", got)
	}
	p, err := Build(in, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if out := Render(p); !strings.Contains(out, "Left out 1 file(s) git ignores") || !strings.Contains(out, "rendered-root.yaml") {
		t.Errorf("the plan should name the file it left out, got\n%s", out)
	}

	in, err = Load(nil, []string{filepath.Join(dir, "var")})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names(in), ","); got != "rendered-only,root" || len(in.Ignored) != 0 {
		t.Errorf("a directory named on purpose should be read whole, got %s, ignored %v", got, in.Ignored)
	}
}
