package flux

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// A cluster joins a Flux fleet the way Flux fleets grow: someone adds its
// directory under clusters/ in the fleet repository. The watcher notices, and
// proposes it to ConfigHub the way `cub sveltos watch` does: it runs apply.sh
// with PROPOSE_ONLY=1, which makes the cluster's Target, variants and layers
// Units and promotes its first release, and approves nothing. A person
// approves in ConfigHub; on its next look the watcher runs apply.sh again,
// which publishes what was approved and, once every variant of the cluster is
// released, its layers Space. Then the cluster can join: join.sh puts the
// root on it. The watcher does not reach into clusters itself.

// JoinState is where a cluster in the plan stands in ConfigHub.
type JoinState string

const (
	// Joining clusters have no Target in ConfigHub yet: nothing is proposed.
	Joining JoinState = "joining"
	// Proposed clusters have their Target, and their layers Space has no
	// release yet: something waits for approval.
	Proposed JoinState = "proposed"
	// Ready clusters have a released layers Space: join.sh can bring them on,
	// or already has.
	Ready JoinState = "ready"
	// HandedOver clusters read ConfigHub through a root committed to Git.
	HandedOver JoinState = "handed over"
)

// ClusterStates reads where each cluster of the plan stands.
func ClusterStates(run Runner, p *Plan, prefix string) (map[string]JoinState, error) {
	out := map[string]JoinState{}
	for _, c := range p.Clusters {
		if c.LayersSpace != "" {
			out[c.Name] = HandedOver
			continue
		}
		if _, err := run("cub", "target", "get", "--space", prefix+"-targets", c.Name, "-o", "json"); err != nil {
			if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "404") {
				return nil, fmt.Errorf("reading the Target of %s: %w", c.Name, err)
			}
			out[c.Name] = Joining
			continue
		}
		if _, err := run("cub", "release", "get", "--space", DeliverySpace(prefix, c.Name), "--oci-reference", "latest", "-o", "json"); err != nil {
			out[c.Name] = Proposed
			continue
		}
		out[c.Name] = Ready
	}
	return out, nil
}

// Watcher proposes the clusters that join a fleet.
type Watcher struct {
	Run Runner
	// Script runs apply.sh in dir with PROPOSE_ONLY=1 and returns what it
	// printed.
	Script func(dir string) (string, error)
	// Plan re-reads the fleet repository.
	Plan func() (*Plan, error)
	// Mark patches a Space, as CubWriter does; it records when a cluster's
	// layers were released.
	Mark   Writer
	Prefix string
	Out    string
	// told keeps what was last said about each cluster, so a watch that sees
	// nothing new says nothing.
	told map[string]string
}

// NewWatcher is a Watcher that runs commands on this machine.
func NewWatcher(prefix, out string, plan func() (*Plan, error)) *Watcher {
	return &Watcher{Run: Run, Script: runProposeOnly, Plan: plan, Mark: CubWriter, Prefix: prefix, Out: out}
}

var waitsFor = regexp.MustCompile(`(?m)^(\S+) waits for approval: (.*)$`)

// Once looks once: it proposes what is new, publishes what was approved, and
// says what changed since it last looked.
func (w *Watcher) Once(say io.Writer) error {
	if w.told == nil {
		w.told = map[string]string{}
	}
	p, err := w.Plan()
	if err != nil {
		return err
	}
	if len(p.Problems) > 0 {
		return fmt.Errorf("the plan has problems to fix first: %s", strings.Join(p.Problems, "; "))
	}
	before, err := ClusterStates(w.Run, p, w.Prefix)
	if err != nil {
		return err
	}
	var waiting []string
	for _, c := range p.Clusters {
		if s := before[c.Name]; s == Joining || s == Proposed {
			waiting = append(waiting, c.Name)
		}
	}
	if len(waiting) == 0 {
		return nil
	}
	if _, err := WriteApply(p, w.Prefix, w.Out); err != nil {
		return err
	}
	printed, err := w.Script(w.Out)
	w.log(printed)
	if err != nil {
		return fmt.Errorf("apply.sh with PROPOSE_ONLY=1 failed; its output is in %s: %w", filepath.Join(w.Out, "watch.log"), err)
	}
	after, err := ClusterStates(w.Run, p, w.Prefix)
	if err != nil {
		return err
	}
	approvals := map[string][]string{}
	for _, m := range waitsFor.FindAllStringSubmatch(printed, -1) {
		for _, c := range waiting {
			if strings.HasSuffix(m[1], "-"+c) {
				approvals[c] = append(approvals[c], m[2])
			}
		}
	}
	for _, c := range waiting {
		var msg string
		switch after[c] {
		case Ready:
			msg = fmt.Sprintf("%s is ready: its layers Space %s is released. Bring it on with\n  FLUX_CONTEXT=<its context> CLUSTER=%s CONFIGHUB_OCI=<gateway> bash %s",
				c, DeliverySpace(w.Prefix, c), c, filepath.Join(w.Out, "join.sh"))
			w.markJoined(c)
		case Proposed:
			msg = fmt.Sprintf("%s is proposed: its variants are made, and its release waits for a person to approve it in ConfigHub", c)
			for _, a := range dedupe(approvals[c]) {
				msg += "\n  " + a
			}
		default:
			msg = fmt.Sprintf("%s is still %s", c, after[c])
		}
		if w.told[c] != msg {
			fmt.Fprintln(say, msg)
			w.told[c] = msg
		}
	}
	return nil
}

// markJoined records on the cluster's layers Space when and why it was
// proposed, as cub sveltos records on its variants.
func (w *Watcher) markJoined(cluster string) {
	if w.Mark == nil {
		return
	}
	note := fmt.Sprintf(`{"Annotations":{"flux.confighub.com/joined":"proposed by cub flux watch; layers released %s"}}`, time.Now().UTC().Format(time.RFC3339))
	_ = w.Mark(DeliverySpace(w.Prefix, cluster), []byte(note))
}

func (w *Watcher) log(printed string) {
	f, err := os.OpenFile(filepath.Join(w.Out, "watch.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "== %s PROPOSE_ONLY=1 bash apply.sh\n%s\n", time.Now().UTC().Format(time.RFC3339), printed)
}

func runProposeOnly(dir string) (string, error) {
	cmd := exec.Command("bash", filepath.Join(dir, "apply.sh"))
	cmd.Env = append(os.Environ(), "PROPOSE_ONLY=1")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
