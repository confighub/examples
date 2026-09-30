package flux

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hub stands in for ConfigHub as a join goes through it: which Targets exist
// and which layers Spaces have a release.
type hub struct {
	targets, released map[string]bool
}

func (h *hub) run(name string, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	switch {
	case strings.HasPrefix(key, "target get"):
		if h.targets[args[4]] {
			return []byte(`{}`), nil
		}
		return nil, fmt.Errorf("target %s not found", args[4])
	case strings.HasPrefix(key, "release get"):
		if h.released[args[3]] {
			return []byte(`{}`), nil
		}
		return nil, fmt.Errorf("latest: not found")
	}
	return nil, fmt.Errorf("unexpected %s", key)
}

// A cluster joins: the watcher proposes it, says what waits for approval, and
// once its layers Space is released, says it can join. It never approves.
func TestWatcherProposesAJoiningCluster(t *testing.T) {
	p := planOf(t, example, repoRoot(t))
	h := &hub{targets: map[string]bool{}, released: map[string]bool{}}
	for _, c := range p.Clusters {
		if c.Name != "prod-1" {
			h.targets[c.Name] = true
			h.released[DeliverySpace("flux", c.Name)] = true
		}
	}
	var scripts int
	var marked []string
	w := &Watcher{Run: h.run, Plan: func() (*Plan, error) { return p, nil }, Prefix: "flux", Out: t.TempDir(),
		Mark: func(space string, patch []byte) error { marked = append(marked, space+" "+string(patch)); return nil }}
	w.Script = func(dir string) (string, error) {
		scripts++
		if _, err := os.Stat(filepath.Join(dir, "apply.sh")); err != nil {
			t.Errorf("the plan should be written to --out before apply.sh runs: %v", err)
		}
		h.targets["prod-1"] = true // PROPOSE_ONLY makes the Target and the variants
		return "flux-apps-prod-1 waits for approval: cub variant approve --change-order flux-apps-base/onboard-x --stage prod-1\n", nil
	}

	var say bytes.Buffer
	if err := w.Once(&say); err != nil {
		t.Fatal(err)
	}
	if scripts != 1 || !strings.Contains(say.String(), "prod-1 is proposed") ||
		!strings.Contains(say.String(), "cub variant approve --change-order flux-apps-base/onboard-x --stage prod-1") {
		t.Fatalf("want one proposal naming the approval, got %d runs:\n%s", scripts, say.String())
	}

	// Approved in ConfigHub; the next run publishes, and the layers Space with it.
	say.Reset()
	w.Script = func(string) (string, error) {
		scripts++
		h.released[DeliverySpace("flux", "prod-1")] = true
		return "", nil
	}
	if err := w.Once(&say); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(say.String(), "prod-1 is ready") || !strings.Contains(say.String(), "CLUSTER=prod-1") {
		t.Fatalf("want the join command:\n%s", say.String())
	}
	if len(marked) != 1 || !strings.Contains(marked[0], "flux-prod-1-layers") || !strings.Contains(marked[0], "flux.confighub.com/joined") {
		t.Errorf("the layers Space should record the join: %v", marked)
	}

	// Nothing waits: no script, nothing said.
	say.Reset()
	before := scripts
	if err := w.Once(&say); err != nil {
		t.Fatal(err)
	}
	if scripts != before || say.Len() != 0 {
		t.Errorf("with nothing waiting the watcher runs nothing and says nothing: %d runs, %q", scripts-before, say.String())
	}
}

// PROPOSE_ONLY=1 approves nothing, and a release refused for want of an
// approval waits rather than failing; without it, the script approves as
// before and a refusal is an error.
func TestProposeOnlyApprovesNothing(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	s := ApplyScript(planOf(t, example, repoRoot(t)), "flux", ".")
	var fns []string
	for _, name := range []string{"holds() {", "approve() {", "publish() {"} {
		i := strings.Index(s, "\n"+name)
		if i < 0 {
			t.Fatalf("apply.sh has no %s", name)
		}
		end := strings.Index(s[i+1:], "\n}\n")
		if strings.HasPrefix(name, "approve") {
			end = strings.Index(s[i+1:], "\n")
		} else {
			end += 2
		}
		fns = append(fns, s[i+1:i+1+end])
	}
	bin := t.TempDir()
	calls := filepath.Join(bin, "calls")
	stub := `#!/usr/bin/env bash
echo "$*" >> "` + calls + `"
case " $* " in
  *" unit list "*) echo 1 ;;
  *" release publish "*) echo "Failed: requires approval: 1 Approval attestation(s) from eligible attesters" >&2; exit 1 ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "cub"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join(fns, "\n") + "\napprove base/order prod-1\npublish flux-apps-prod-1 base/order 1 prod-1\n"
	for _, tc := range []struct {
		env      string
		fails    bool
		approves bool
	}{{"PROPOSE_ONLY=1", false, false}, {"", true, true}} {
		os.Remove(calls)
		cmd := exec.Command("bash", "-c", body)
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		if tc.env != "" {
			cmd.Env = append(cmd.Env, tc.env)
		}
		out, err := cmd.CombinedOutput()
		log, _ := os.ReadFile(calls)
		if (err != nil) != tc.fails {
			t.Errorf("%q: failed=%v, want %v:\n%s", tc.env, err != nil, tc.fails, out)
		}
		if strings.Contains(string(log), "variant approve") != tc.approves {
			t.Errorf("%q: approved=%v, want %v:\n%s", tc.env, !tc.approves, tc.approves, log)
		}
		if tc.env != "" && !strings.Contains(string(out), "flux-apps-prod-1 waits for approval: cub variant approve --change-order base/order --stage prod-1") {
			t.Errorf("PROPOSE_ONLY should name the approval:\n%s", out)
		}
	}
}
