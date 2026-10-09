package flux

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// Live status is how ConfigHub hears what a cluster is running. It is
// recorded on the Release: what the tool deploying that Release says about it
// running. ConfigHub's Healthy gate reads the newest published Release of a
// Space and nothing else, so a Release no tool has reported on is not healthy,
// however the one before it was. argobot records it for Argo CD; this records
// it for Flux, in the same normalized words, so the gate, change order stages
// and the UI read both alike.
//
// Two things make it more than a mirror of Flux's own conditions:
//
//   - Which Release a reading is recorded on is decided by the digest Flux
//     reports: the one it applied, or the one it is trying to apply when the
//     reading is about that attempt. It is never inferred from times, which is
//     what Sveltos has to do.
//   - Nothing else checks what a reading claims, so it says Healthy only when
//     Flux actually checked the workloads.

// StatusReporter names this reporter on each Release it reports on.
const StatusReporter = "cub-flux"

// LiveStatus is what is recorded on a Release. Sync, Health and Operation are
// ConfigHub's normalized words, which its gates read; the Reporter fields
// keep Flux's own beside them.
type LiveStatus struct {
	Reporter          string `json:"reporter"`
	DataSource        string `json:"dataSource,omitempty"`
	Sync              string `json:"sync"`
	Health            string `json:"health"`
	Operation         string `json:"operation,omitempty"`
	ReporterSync      string `json:"reporterSync,omitempty"`
	ReporterHealth    string `json:"reporterHealth,omitempty"`
	ReporterOperation string `json:"reporterOperation,omitempty"`
	Message           string `json:"message,omitempty"`
	ObservedAt        string `json:"observedAt"`
}

// Same reports whether two readings say the same thing, whenever each was
// taken.
func (s LiveStatus) Same(o LiveStatus) bool {
	s.ObservedAt, o.ObservedAt = "", ""
	return s == o
}

// Gate reports whether ConfigHub's Healthy prerequisite would pass on this
// reading, were it on the Space's newest published Release: synced, healthy,
// and no operation running or failed.
func (s LiveStatus) Gate() bool {
	return s.Sync == "Synced" && s.Health == "Healthy" && s.Operation != "Running" && s.Operation != "Failed"
}

func (s LiveStatus) String() string {
	parts := []string{s.Sync, s.Health}
	if s.Operation != "" {
		parts = append(parts, s.Operation)
	}
	out := strings.Join(parts, "/")
	if s.Message != "" {
		out += ": " + s.Message
	}
	return out
}

// Reading is what one layer says about itself, and the Release it says it
// about, before anything is written.
type Reading struct {
	Check  Check      `json:"check"`
	Status LiveStatus `json:"status"`
	// Revision is the digest Flux reports, which names the Release the
	// reading is about.
	Revision string `json:"revision,omitempty"`
	// Release is the published release with that digest, where the reading is
	// recorded; 0 when there is none. Newest is the Space's newest published
	// release, the one the Healthy gate reads.
	Release int `json:"release,omitempty"`
	Newest  int `json:"newest,omitempty"`
	// Held is what that Release holds now, if a tool has reported on it.
	Held *LiveStatus `json:"held,omitempty"`
	// NewestHeld is what the newest Release holds, which is what the gate
	// would find.
	NewestHeld *LiveStatus `json:"newestHeld,omitempty"`
	// Unrecorded, when set, is why the reading names no Release to record it
	// on.
	Unrecorded string `json:"unrecorded,omitempty"`
	// Pending, when set, is why this pass has nothing new to say and what is
	// recorded stands: Flux is reconciling the release it already applied,
	// which it does on every interval.
	Pending string `json:"pending,omitempty"`
	// Skip, when set, is why nothing is reported for this layer: it does not
	// read the Space the reading would be written to.
	Skip string `json:"skip,omitempty"`
}

// statusMessageLimit keeps a message short enough to read in a list; ConfigHub
// allows 1024 bytes.
const statusMessageLimit = 200

// workloadKinds are what a layer runs, as opposed to what it merely declares.
// A layer applying none of them is healthy once it is applied; one applying
// any of them is healthy only if Flux checked them.
var workloadKinds = map[string]bool{
	"Deployment": true, "StatefulSet": true, "DaemonSet": true, "ReplicaSet": true,
	"Job": true, "CronJob": true, "Pod": true, "HelmRelease": true,
}

// ReadStatus reads one layer and says what ConfigHub should hear, and about
// which Release. A read that fails returns an error and no reading: a guess
// written here could open a gate or advance a change order.
func ReadStatus(run Runner, hub Hub, c Check, now time.Time) (Reading, error) {
	r := Reading{Check: c}
	out, err := run("kubectl", "-n", checkNamespace, "get", "kustomization", c.Kustomization, "-o", "json")
	if err != nil {
		return r, fmt.Errorf("reading Kustomization %s: %w", c.Kustomization, err)
	}
	var k kustomization
	if err := json.Unmarshal(out, &k); err != nil {
		return r, fmt.Errorf("reading Kustomization %s: %w", c.Kustomization, err)
	}
	if k.Spec.SourceRef.Kind != "OCIRepository" {
		r.Skip = fmt.Sprintf("reads %s %s, not ConfigHub, so there is nothing of ConfigHub's to report", k.Spec.SourceRef.Kind, k.Spec.SourceRef.Name)
		r.leftBehind(hub)
		return r, nil
	}
	srcNS := k.Spec.SourceRef.Namespace
	if srcNS == "" {
		srcNS = checkNamespace
	}
	out, err = run("kubectl", "-n", srcNS, "get", "ocirepository", k.Spec.SourceRef.Name, "-o", "jsonpath={.spec.url}")
	if err != nil {
		return r, fmt.Errorf("reading OCIRepository %s: %w", k.Spec.SourceRef.Name, err)
	}
	// The gateway serves one repository per Space, at /space/<space>. A
	// layer reading another Space's release is not this variant's to report.
	if url := strings.TrimSpace(string(out)); !strings.HasSuffix(strings.TrimSuffix(url, "/"), "/space/"+c.Space) {
		r.Skip = fmt.Sprintf("reads %s, not Space %s", url, c.Space)
		r.leftBehind(hub)
		return r, nil
	}
	releases, err := hub.Releases(c.Space)
	if err != nil {
		return r, fmt.Errorf("listing the releases of %s: %w", c.Space, err)
	}
	var why string
	r.Status, r.Revision, why, r.Pending = k.status()
	if r.Pending != "" {
		return r, nil
	}
	r.Status.Reporter = StatusReporter
	r.Status.DataSource = c.Kustomization
	r.Status.ObservedAt = now.UTC().Format(time.RFC3339)
	r.place(releases, why)
	r.Status.Message = clip(r.Status.Message)
	return r, nil
}

// leftBehind notes what the Space's newest Release holds, for a reading that
// is skipped: that decides whether a reading of ours is left behind on it. A
// Space that cannot be read has nothing of ours to replace.
func (r *Reading) leftBehind(hub Hub) {
	releases, err := hub.Releases(r.Check.Space)
	if err != nil {
		return
	}
	if newest := newestPublished(releases); newest != nil {
		r.Release, r.Newest, r.Held, r.NewestHeld = newest.Num, newest.Num, newest.Live, newest.Live
	}
}

func clip(msg string) string {
	if len(msg) <= statusMessageLimit {
		return msg
	}
	// Cut on a character, not inside one: half a character reads back as a
	// different one, and the reading would never match what was written.
	cut := statusMessageLimit - 3
	for cut > 0 && !utf8.RuneStart(msg[cut]) {
		cut--
	}
	return msg[:cut] + "..."
}

func newestPublished(releases []HubRelease) *HubRelease {
	var newest *HubRelease
	for i := range releases {
		if releases[i].Published && (newest == nil || releases[i].Num > newest.Num) {
			newest = &releases[i]
		}
	}
	return newest
}

// place finds the Release a reading is about: the newest published one with
// the digest Flux reports. why is what an applied release's message should
// add about its health.
func (r *Reading) place(releases []HubRelease, why string) {
	newest := newestPublished(releases)
	if newest != nil {
		r.Newest, r.NewestHeld = newest.Num, newest.Live
	}
	if r.Revision == "" {
		r.Unrecorded = r.Status.Message
		if r.Unrecorded == "" {
			r.Unrecorded = "Flux reports no revision"
		}
		return
	}
	var this *HubRelease
	for i := range releases {
		if releases[i].Published && releases[i].ManifestDigest == r.Revision && (this == nil || releases[i].Num > this.Num) {
			this = &releases[i]
		}
	}
	if this == nil {
		r.Unrecorded = "Flux reports " + r.Revision + ", which is no published release of this Space"
		return
	}
	r.Release, r.Held = this.Num, this.Live
	if r.Status.Message == "" {
		behind := ""
		if this.Num != newest.Num {
			behind = fmt.Sprintf("; release %d is published and not applied yet", newest.Num)
		}
		r.Status.Message = fmt.Sprintf("release %d applied%s%s", this.Num, behind, why)
	}
}

type condition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Message            string `json:"message"`
	ObservedGeneration int64  `json:"observedGeneration"`
}

type kustomization struct {
	Metadata struct {
		Generation int64 `json:"generation"`
	} `json:"metadata"`
	Spec struct {
		Suspend      bool `json:"suspend"`
		Wait         bool `json:"wait"`
		HealthChecks []struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"healthChecks"`
		TargetNamespace string `json:"targetNamespace"`
		SourceRef       struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"sourceRef"`
	} `json:"spec"`
	Status struct {
		ObservedGeneration    int64       `json:"observedGeneration"`
		Conditions            []condition `json:"conditions"`
		LastAppliedRevision   string      `json:"lastAppliedRevision"`
		LastAttemptedRevision string      `json:"lastAttemptedRevision"`
		Inventory             struct {
			Entries []struct {
				ID string `json:"id"`
			} `json:"entries"`
		} `json:"inventory"`
	} `json:"status"`
}

func (k kustomization) condition(t string) *condition {
	for i := range k.Status.Conditions {
		if k.Status.Conditions[i].Type == t {
			return &k.Status.Conditions[i]
		}
	}
	return nil
}

// unchecked is every workload the layer applied that Flux was not asked to
// check. With spec.wait, Flux checks everything it applied. Without it, only
// what healthChecks names is checked, and a list naming one Deployment says
// nothing about the next.
func (k kustomization) unchecked() []string {
	if k.Spec.Wait {
		return nil
	}
	checked := map[string]bool{}
	for _, h := range k.Spec.HealthChecks {
		ns := h.Namespace
		if ns == "" {
			ns = k.Spec.TargetNamespace
		}
		checked[h.Kind+"|"+ns+"|"+h.Name] = true
	}
	var out []string
	for _, e := range k.Status.Inventory.Entries {
		o, err := parseID(e.ID)
		if err != nil || !workloadKinds[o.Kind] {
			continue
		}
		if !checked[o.Kind+"|"+o.Namespace+"|"+o.Name] {
			out = append(out, o.String())
		}
	}
	return out
}

// status maps a Kustomization onto ConfigHub's words, and names the digest
// the reading is about: the one Flux applied, or the one it is trying to apply
// when the reading is about that attempt. why is what to add about health once
// the release is known.
func (k kustomization) status() (st LiveStatus, revision, why, pending string) {
	applied := digestOf(k.Status.LastAppliedRevision)
	attempted := digestOf(k.Status.LastAttemptedRevision)
	if attempted == "" {
		attempted = applied
	}
	if k.Spec.Suspend {
		return LiveStatus{Sync: "Unknown", Health: "Suspended", Message: "the Kustomization is suspended, so Flux applies nothing"}, applied, "", ""
	}
	ready := k.condition("Ready")
	if k.Status.ObservedGeneration != k.Metadata.Generation || ready == nil || ready.ObservedGeneration != k.Metadata.Generation {
		return LiveStatus{Sync: "Unknown", Health: "Progressing", Operation: "Running", Message: "Flux has not reported on the current generation yet"}, attempted, "", ""
	}
	if s := k.condition("Stalled"); s != nil && s.Status == "True" {
		return LiveStatus{Sync: "OutOfSync", Health: "Degraded", Operation: "Failed", Message: "stalled: " + s.Message}, attempted, "", ""
	}
	// Flux marks a layer Reconciling, and its readiness unknown, at the start
	// of every pass, including the one it makes each interval over a release
	// it applied long ago. That pass says nothing new about the release, and
	// reporting it would close the gate for a few seconds every interval.
	again := applied != "" && attempted == applied
	const stands = "Flux is reconciling the release it already applied; what is recorded stands until it finishes"
	if rc := k.condition("Reconciling"); rc != nil && rc.Status == "True" {
		if again {
			return LiveStatus{}, applied, "", stands
		}
		return LiveStatus{Sync: "OutOfSync", Health: "Progressing", Operation: "Running", Message: orSay(rc.Message, "reconciling")}, attempted, "", ""
	}
	switch ready.Status {
	case "False":
		return LiveStatus{Sync: "OutOfSync", Health: "Degraded", Operation: "Failed", Message: orSay(ready.Message, "not ready")}, attempted, "", ""
	case "True":
	default:
		if again {
			return LiveStatus{}, applied, "", stands
		}
		return LiveStatus{Sync: "OutOfSync", Health: "Progressing", Operation: "Running", Message: orSay(ready.Message, "readiness unknown")}, attempted, "", ""
	}
	if attempted != applied {
		return LiveStatus{Sync: "OutOfSync", Health: "Progressing", Operation: "Running", Message: "Flux is applying this release; it last applied " + orSay(applied, "none")}, attempted, "", ""
	}
	health := "Healthy"
	if u := k.unchecked(); len(u) > 0 {
		// Ready then means applied, not running, for what Flux was not asked
		// to look at.
		health, why = "Unknown", "; Ready means applied, not healthy, for "+strings.Join(u, ", ")+": neither spec.wait nor a health check covers it"
	}
	return LiveStatus{Sync: "Synced", Health: health, Operation: "Succeeded"}, applied, why, ""
}

// health is what Flux says of the layer for a recorded check: its health in
// ConfigHub's words, what is wrong if it is Degraded, and why it is neither
// healthy nor wrong yet if it is still on its way. A layer whose workloads
// Flux was not asked to check is Unknown, and is not held back for it: Ready
// is all Flux will ever say of it.
func (k kustomization) health(name string) (health, unhealthy, notYet string) {
	if k.Spec.Suspend {
		return "Suspended", "", fmt.Sprintf("Kustomization %s is suspended, so Flux applies nothing", name)
	}
	ready := k.condition("Ready")
	if k.Status.ObservedGeneration != k.Metadata.Generation || ready == nil || ready.ObservedGeneration != k.Metadata.Generation {
		return "Progressing", "", fmt.Sprintf("Flux has not reported on the current generation of %s yet", name)
	}
	if s := k.condition("Stalled"); s != nil && s.Status == "True" {
		return "Degraded", fmt.Sprintf("Flux reports %s stalled: %s", name, orSay(s.Message, "no reason given")), ""
	}
	if rc := k.condition("Reconciling"); rc != nil && rc.Status == "True" {
		return "Progressing", "", fmt.Sprintf("Flux is reconciling %s", name)
	}
	switch ready.Status {
	case "False":
		return "Degraded", fmt.Sprintf("Flux reports %s not ready: %s", name, orSay(ready.Message, "no reason given")), ""
	case "True":
	default:
		return "Progressing", "", fmt.Sprintf("Flux does not say yet whether %s is ready", name)
	}
	if len(k.unchecked()) > 0 {
		return "Unknown", "", ""
	}
	return "Healthy", "", ""
}

// orSay is msg, or what to say when Flux gave none: an empty message would be
// taken for an applied release's.
func orSay(msg, otherwise string) string {
	if msg == "" {
		return otherwise
	}
	return msg
}

// digestOf takes the digest out of Flux's "<tag>@sha256:..." revision.
func digestOf(revision string) string {
	if i := strings.LastIndex(revision, "@"); i >= 0 {
		return revision[i+1:]
	}
	return revision
}

// Outcome is what reporting did for one layer.
type Outcome struct {
	Reading
	// Did is one of written, unchanged, dry-run, skipped, left, unrecorded or
	// pending.
	Did string `json:"did"`
	// Why says, for left, whose reading was left alone.
	Why string `json:"why,omitempty"`
	// Withdrew is what was done about a passing reading of ours left on the
	// newest Release when the reading is now of another: written, unchanged,
	// left or dry-run.
	Withdrew string `json:"withdrew,omitempty"`
}

// ReportStatus records each reading that says something new on its Release, or
// that the Release has held for longer than refresh: observedAt is the only
// sign this reporter is still running, so an unchanged reading is written
// again now and then.
//
// A reading another reporter wrote, argobot say, is left alone while it is
// fresher than refresh, and for as long as it says the same: argobot writes
// only when something changes, so an old reading of its is not a stopped
// reporter. It is replaced only when it is both old and different.
//
// A reading is true of one Release. When the newest Release holds a passing
// reading of ours and Flux now reports another release, or none, that
// reading is no longer true and the Healthy gate would go on passing on it,
// so it is withdrawn.
func ReportStatus(hub Hub, readings []Reading, refresh time.Duration, dryRun bool, now time.Time) ([]Outcome, error) {
	var out []Outcome
	// A Release that cannot be written is reported, and the rest are still
	// reported: one deleted Space must not freeze every reading after it.
	var errs []string
	for _, r := range readings {
		o := Outcome{Reading: r}
		source := r.Check.Kustomization
		ours := func(h *LiveStatus) bool {
			return h != nil && h.Reporter == StatusReporter && h.DataSource == source
		}
		record := func(release int, held *LiveStatus, st LiveStatus) (did, why string) {
			fresh := false
			if held != nil {
				if at, err := time.Parse(time.RFC3339, held.ObservedAt); err == nil && now.Sub(at) < refresh {
					fresh = true
				}
			}
			if held != nil && held.Reporter != StatusReporter {
				same := held.Sync == st.Sync && held.Health == st.Health && held.Operation == st.Operation
				switch {
				case fresh:
					return "left", fmt.Sprintf("%s reported on release %d at %s; two reporters would overwrite each other", held.Reporter, release, held.ObservedAt)
				case same:
					return "left", fmt.Sprintf("%s reported the same on release %d at %s", held.Reporter, release, held.ObservedAt)
				}
			}
			if ours(held) && held.Same(st) && fresh {
				return "unchanged", ""
			}
			if dryRun {
				return "dry-run", ""
			}
			if err := hub.SetLiveStatus(r.Check.Space, release, st); err != nil {
				errs = append(errs, fmt.Sprintf("recording the live status of release %d of %s: %v", release, r.Check.Space, err))
				return "", ""
			}
			return "written", ""
		}
		if r.Pending != "" {
			// Nothing new is known, so what is recorded stands.
			o.Did = "pending"
			out = append(out, o)
			continue
		}
		if r.Skip != "" {
			// A layer that has left this Space, handed back to Git say, leaves
			// behind the last reading recorded for it, and the Healthy gate
			// would go on passing on it. So a reading this reporter wrote on
			// the newest Release is replaced with one that says the layer no
			// longer reads the Space. Another reporter's reading is not ours
			// to change.
			if !ours(r.Held) {
				o.Did = "skipped"
				out = append(out, o)
				continue
			}
			r.Status = LiveStatus{Reporter: StatusReporter, DataSource: source, Sync: "Unknown", Health: "Unknown",
				Message: clip("the layer no longer reads this Space: it " + r.Skip), ObservedAt: now.UTC().Format(time.RFC3339)}
			o.Reading = r
		}
		if r.Release == 0 {
			o.Did = "unrecorded"
		} else if o.Did, o.Why = record(r.Release, r.Held, r.Status); o.Did == "" {
			continue
		}
		if r.Skip == "" && r.Newest != 0 && r.Release != r.Newest && ours(r.NewestHeld) && r.NewestHeld.Gate() {
			w := LiveStatus{Reporter: StatusReporter, DataSource: source, Sync: "Unknown", Health: "Unknown", ObservedAt: now.UTC().Format(time.RFC3339)}
			if r.Release != 0 {
				w.Sync = "OutOfSync"
				w.Message = fmt.Sprintf("not what is running: Flux reports release %d", r.Release)
			} else {
				w.Message = clip("not known to be running: " + r.Unrecorded)
			}
			if o.Withdrew, _ = record(r.Newest, r.NewestHeld, w); o.Withdrew == "" {
				continue
			}
			o.NewestHeld = &w
		}
		out = append(out, o)
	}
	if len(errs) > 0 {
		return out, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return out, nil
}

// PrintOutcomes says what happened, one layer to a line.
func PrintOutcomes(w io.Writer, outs []Outcome) {
	for _, o := range outs {
		name := o.Check.Kustomization
		// newest is what the gate would find on the release it reads.
		newest := "nothing has reported on"
		if o.NewestHeld != nil {
			newest = fmt.Sprintf("holds %s from %s", o.NewestHeld, o.NewestHeld.Reporter)
		}
		if o.Withdrew != "" {
			newest += fmt.Sprintf(" (%s: the passing reading this reporter left there is withdrawn)", o.Withdrew)
		}
		switch {
		case o.Did == "pending":
			fmt.Fprintf(w, "%s -> %s: %s\n", name, o.Check.Space, o.Pending)
			continue
		case o.Did == "left":
			fmt.Fprintf(w, "%s -> %s: left alone: %s\n", name, o.Check.Space, o.Why)
			continue
		case o.Did == "skipped":
			fmt.Fprintf(w, "%s: not reported: %s\n", name, o.Skip)
			continue
		case o.Did == "unrecorded" && o.Newest == 0:
			fmt.Fprintf(w, "%s -> %s: not recorded, there is no release to record it on: %s (the Healthy gate would not pass)\n", name, o.Check.Space, o.Unrecorded)
			continue
		case o.Did == "unrecorded":
			fmt.Fprintf(w, "%s -> %s: not recorded, there is no release to record it on: %s (the Healthy gate reads release %d, the newest, which %s)\n", name, o.Check.Space, o.Unrecorded, o.Newest, newest)
			continue
		}
		gate := "the Healthy gate would not pass"
		switch {
		case o.Release != o.Newest:
			gate = fmt.Sprintf("the Healthy gate would not pass: it reads release %d, the newest, which %s", o.Newest, newest)
		case o.Status.Gate():
			gate = "the Healthy gate would pass"
		}
		fmt.Fprintf(w, "%s -> %s release %d: %s (%s; %s)\n", name, o.Check.Space, o.Release, o.Status, o.Did, gate)
	}
}
