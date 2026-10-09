package argo

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/confighub/sdk/core/livestatus"
)

// Live status is how ConfigHub hears what a cluster is running. It is
// recorded on the Release: what the tool deploying that Release says about it
// running. ConfigHub's Healthy gate reads the newest published Release of a
// Space and nothing else, so a Release no tool has reported on is not healthy,
// however the one before it was. On a cluster `cub cluster up` made, argobot
// records it. An estate onboarded here may run no argobot, and without this
// nothing would record it: the gate, the change order stages and the UI would
// have nothing to read.
//
// Which Release a reading is recorded on is decided by one thing only: the
// digest Argo CD says it synced, status.sync.revision, which for an oci://
// source is the Release's manifest digest. It is never inferred from anything
// else. Argo caches the digest it resolved for "latest" until a hard refresh,
// so after a publish it goes on saying Synced at the release before: that
// reading is recorded on the release before, where it is true, and the newest
// Release stays without one until Argo reads it.

// StatusReporter names this reporter on each Release it reports on.
const StatusReporter = "cub-argo"

// LiveStatus is what is recorded on a Release. Sync, Health and Operation are
// ConfigHub's normalized words, which its gates read; the Reporter fields
// keep Argo CD's own beside them.
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

// StatusCheck is one Application whose status is reported, and the Space it
// is reported to.
type StatusCheck struct {
	Application string `json:"application"`
	Space       string `json:"space"`
	Cluster     string `json:"cluster,omitempty"`
}

// StatusChecksFor is every Application a plan governs that reads a Space of
// its own: each variant's Application, and each app of apps whose children
// moved into a control Space.
func StatusChecksFor(p *Plan, prefix string) []StatusCheck {
	var out []StatusCheck
	for _, cs := range p.controlSpaces(prefix) {
		out = append(out, StatusCheck{Application: cs.Parent, Space: cs.Space})
	}
	for _, ck := range ChecksFor(p) {
		out = append(out, StatusCheck{Application: ck.Application, Space: ck.Space, Cluster: ck.Cluster})
	}
	return out
}

// Reading is what one Application says about itself, and the Release it says
// it about, before anything is written.
type Reading struct {
	Check  StatusCheck `json:"check"`
	Status LiveStatus  `json:"status"`
	// Revision is the digest Argo CD says it synced, which names the Release
	// the reading is about.
	Revision string `json:"revision,omitempty"`
	// Release is the published release with that digest, where the reading is
	// recorded; 0 when there is none. Newest is the Space's newest published
	// release, the one the Healthy gate reads.
	Release int `json:"release,omitempty"`
	Newest  int `json:"newest,omitempty"`
	// Held is what that Release holds now, if a tool has reported on it.
	Held *LiveStatus `json:"held,omitempty"`
	// Unrecorded, when set, is why the reading names no Release to record it
	// on.
	Unrecorded string `json:"unrecorded,omitempty"`
	// Skip, when set, is why nothing is reported for this Application: it
	// does not read the Space the reading would be written to.
	Skip string `json:"skip,omitempty"`
	// Unread is the digest of the newest published release when Argo CD has
	// synced an older one: what a hard refresh would make it read.
	Unread string `json:"unread,omitempty"`
}

// statusMessageLimit keeps a message short enough to read in a list; ConfigHub
// allows 1024 bytes.
const statusMessageLimit = 200

type appSource struct {
	RepoURL        string `json:"repoURL"`
	TargetRevision string `json:"targetRevision"`
	Path           string `json:"path"`
}

type application struct {
	Spec struct {
		Source  *appSource  `json:"source"`
		Sources []appSource `json:"sources"`
	} `json:"spec"`
	Status struct {
		Sync struct {
			Status     string `json:"status"`
			Revision   string `json:"revision"`
			ComparedTo struct {
				Source  *appSource  `json:"source"`
				Sources []appSource `json:"sources"`
			} `json:"comparedTo"`
		} `json:"sync"`
		Health struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"health"`
		OperationState *struct {
			Phase   string `json:"phase"`
			Message string `json:"message"`
		} `json:"operationState"`
		Conditions []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"conditions"`
		Resources []struct {
			Kind      string `json:"kind"`
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
			Status    string `json:"status"`
		} `json:"resources"`
	} `json:"status"`
}

// outOfSync names what Argo says differs from the source, a few at most.
func (a application) outOfSync() string {
	var names []string
	for _, r := range a.Status.Resources {
		if r.Status != "OutOfSync" {
			continue
		}
		o := Owned{Kind: r.Kind, Namespace: r.Namespace, Name: r.Name}
		names = append(names, o.String())
	}
	if len(names) == 0 {
		return ""
	}
	total := len(names)
	const most = 3
	more := ""
	if total > most {
		more = fmt.Sprintf(" and %d more", total-most)
		names = names[:most]
	}
	return fmt.Sprintf("%d out of sync: %s%s", total, strings.Join(names, ", "), more)
}

// spaceSource is the source that reads the Space, if any does.
func (a application) spaceSource(space string) (appSource, bool) {
	var all []appSource
	if a.Spec.Source != nil {
		all = append(all, *a.Spec.Source)
	}
	all = append(all, a.Spec.Sources...)
	for _, s := range all {
		if readsSpace(s.RepoURL, space) {
			return s, true
		}
	}
	return appSource{}, false
}

// readsSpace says whether a repoURL is the gateway's repository for a Space,
// which it serves at /space/<space>.
func readsSpace(url, space string) bool {
	return strings.HasSuffix(strings.TrimSuffix(url, "/"), "/space/"+space)
}

func (a application) sourceURLs() string {
	var urls []string
	if a.Spec.Source != nil {
		urls = append(urls, a.Spec.Source.RepoURL)
	}
	for _, s := range a.Spec.Sources {
		urls = append(urls, s.RepoURL)
	}
	if len(urls) == 0 {
		return "no source"
	}
	return strings.Join(urls, ", ")
}

// comparedWith says whether Argo's last comparison was against this source.
// Right after a repoint, status.sync still describes the old one.
func (a application) comparedWith(s appSource) bool {
	var all []appSource
	if c := a.Status.Sync.ComparedTo.Source; c != nil {
		all = append(all, *c)
	}
	all = append(all, a.Status.Sync.ComparedTo.Sources...)
	for _, c := range all {
		if c.RepoURL == s.RepoURL && c.TargetRevision == s.TargetRevision && c.Path == s.Path {
			return true
		}
	}
	return false
}

// problem is the first thing Argo says is wrong with the Application, if
// anything: an error condition, or a failed operation.
func (a application) problem() string {
	for _, c := range a.Status.Conditions {
		if strings.HasSuffix(c.Type, "Error") {
			return c.Message
		}
	}
	if op := a.Status.OperationState; op != nil && (op.Phase == "Failed" || op.Phase == "Error") {
		return op.Message
	}
	return ""
}

// ReadStatus reads one Application and says what ConfigHub should hear, and
// about which Release. A read that fails returns an error and no reading: a
// guess written here could open a gate or advance a change order.
func ReadStatus(run Runner, hub Hub, c StatusCheck, now time.Time) (Reading, error) {
	r := Reading{Check: c}
	out, err := run("kubectl", "-n", checkNamespace, "get", "application", c.Application, "-o", "json")
	if err != nil {
		return r, fmt.Errorf("reading Application %s: %w", c.Application, err)
	}
	var a application
	if err := json.Unmarshal(out, &a); err != nil {
		return r, fmt.Errorf("reading Application %s: %w", c.Application, err)
	}
	src, ok := a.spaceSource(c.Space)
	if !ok {
		r.Skip = fmt.Sprintf("reads %s, not Space %s, so there is nothing of ConfigHub's to report", a.sourceURLs(), c.Space)
		r.leftBehind(hub)
		return r, nil
	}
	releases, err := hub.Releases(c.Space)
	if err != nil {
		return r, fmt.Errorf("listing the releases of %s: %w", c.Space, err)
	}
	r.Status, r.Revision = a.status(src)
	r.Status.Reporter = StatusReporter
	r.Status.DataSource = c.Application
	r.Status.ObservedAt = now.UTC().Format(time.RFC3339)
	r.place(releases)
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
		r.Release, r.Newest, r.Held = newest.Num, newest.Num, newest.Live
	}
}

func clip(msg string) string {
	if len(msg) > statusMessageLimit {
		return msg[:statusMessageLimit-3] + "..."
	}
	return msg
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
// the digest Argo CD synced.
func (r *Reading) place(releases []HubRelease) {
	newest := newestPublished(releases)
	if newest != nil {
		r.Newest = newest.Num
	}
	if r.Revision == "" {
		r.Unrecorded = r.Status.Message
		if r.Unrecorded == "" {
			r.Unrecorded = "Argo CD reports no synced revision"
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
		r.Unrecorded = "Argo CD synced " + r.Revision + ", which is no published release of this Space"
		return
	}
	r.Release, r.Held = this.Num, this.Live
	behind := ""
	if this.Num != newest.Num {
		r.Unread = newest.ManifestDigest
		behind = fmt.Sprintf("; release %d is published and Argo CD has not read it: it caches the digest behind latest until a hard refresh (status --hard-refresh asks for one)", newest.Num)
	}
	if r.Status.Message == "" {
		r.Status.Message = fmt.Sprintf("release %d synced%s", this.Num, behind)
	}
}

// status maps an Application onto ConfigHub's words, and names the digest it
// is about: empty when Argo has not said.
func (a application) status(src appSource) (LiveStatus, string) {
	phase := ""
	if op := a.Status.OperationState; op != nil {
		phase = op.Phase
	}
	n := livestatus.FromArgoCD(a.Status.Sync.Status, a.Status.Health.Status, phase)
	st := LiveStatus{
		Sync: string(n.Sync), Health: string(n.Health), Operation: string(n.Operation),
		ReporterSync: n.ReporterSync, ReporterHealth: n.ReporterHealth, ReporterOperation: n.ReporterOperation,
	}
	if !a.comparedWith(src) {
		st.Sync = "Unknown"
		st.Message = "Argo CD has not compared the Application with its ConfigHub source yet"
		return st, ""
	}
	revision := a.Status.Sync.Revision
	if p := a.problem(); p != "" {
		// Argo can go on saying Synced from the last comparison that worked
		// while it reports an error now. That is not a reading to pass a gate
		// on.
		if st.Sync == "Synced" {
			st.Sync = "Unknown"
		}
		st.Message = p
		return st, revision
	}
	if a.Status.Sync.Status != "Synced" {
		// The last operation's message describes that operation, which may
		// have succeeded before the source moved on, so it is the last resort.
		st.Message = a.outOfSync()
		if st.Message == "" {
			st.Message = a.Status.Health.Message
		}
		if op := a.Status.OperationState; st.Message == "" && op != nil {
			st.Message = op.Message
		}
	}
	return st, revision
}

// Outcome is what reporting did for one Application.
type Outcome struct {
	Reading
	// Did is one of written, unchanged, dry-run, skipped, left or unrecorded.
	Did string `json:"did"`
	// Why says, for left, whose reading was left alone.
	Why string `json:"why,omitempty"`
}

// ReportStatus records each reading that says something new on its Release, or
// that the Release has held for longer than refresh: observedAt is the only
// sign a reporter is still running, so an unchanged reading is written again
// now and then.
//
// A reading another reporter wrote, argobot say, is left alone while it is
// fresher than refresh: two reporters on one Release would overwrite each
// other on every pass. One older than that is from a reporter that has
// stopped, and is replaced.
func ReportStatus(hub Hub, readings []Reading, refresh time.Duration, dryRun bool, now time.Time) ([]Outcome, error) {
	var out []Outcome
	// A Release that cannot be written is reported, and the rest are still
	// reported: one deleted Space must not freeze every reading after it.
	var errs []string
	for _, r := range readings {
		o := Outcome{Reading: r}
		held := r.Held
		fresh := false
		if held != nil {
			if at, err := time.Parse(time.RFC3339, held.ObservedAt); err == nil && now.Sub(at) < refresh {
				fresh = true
			}
		}
		if r.Skip != "" {
			// An Application that has left this Space, handed back to Git say,
			// leaves behind the last reading recorded for it, and the Healthy
			// gate would go on passing on it. So a reading this reporter wrote
			// on the newest Release is replaced with one that says the
			// Application no longer reads the Space. Another reporter's reading
			// is not ours to change.
			if held == nil || held.Reporter != StatusReporter {
				o.Did = "skipped"
				out = append(out, o)
				continue
			}
			r.Status = LiveStatus{Reporter: StatusReporter, DataSource: r.Check.Application, Sync: "Unknown", Health: "Unknown",
				Message: clip("the Application no longer reads this Space: it " + r.Skip), ObservedAt: now.UTC().Format(time.RFC3339)}
			o.Reading = r
		}
		if r.Release == 0 {
			o.Did = "unrecorded"
			out = append(out, o)
			continue
		}
		if held != nil && held.Reporter != StatusReporter && fresh {
			o.Did = "left"
			o.Why = fmt.Sprintf("%s reported on release %d at %s; two reporters would overwrite each other", held.Reporter, r.Release, held.ObservedAt)
			out = append(out, o)
			continue
		}
		if held != nil && held.Same(r.Status) && fresh {
			o.Did = "unchanged"
			out = append(out, o)
			continue
		}
		if dryRun {
			o.Did = "dry-run"
			out = append(out, o)
			continue
		}
		if err := hub.SetLiveStatus(r.Check.Space, r.Release, r.Status); err != nil {
			errs = append(errs, fmt.Sprintf("recording the live status of release %d of %s: %v", r.Release, r.Check.Space, err))
			continue
		}
		o.Did = "written"
		out = append(out, o)
	}
	if len(errs) > 0 {
		return out, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return out, nil
}

// PrintOutcomes says what happened, one Application to a line.
func PrintOutcomes(w io.Writer, outs []Outcome) {
	for _, o := range outs {
		switch {
		case o.Did == "left":
			fmt.Fprintf(w, "%s -> %s: left alone: %s\n", o.Check.Application, o.Check.Space, o.Why)
			continue
		case o.Did == "skipped":
			fmt.Fprintf(w, "%s: not reported: %s\n", o.Check.Application, o.Skip)
			continue
		case o.Did == "unrecorded":
			fmt.Fprintf(w, "%s -> %s: not recorded, there is no release to record it on: %s (the Healthy gate would not pass)\n", o.Check.Application, o.Check.Space, o.Unrecorded)
			continue
		}
		gate := "the Healthy gate would not pass"
		switch {
		case o.Release != o.Newest:
			gate = fmt.Sprintf("the Healthy gate would not pass: it reads release %d, the newest, which nothing has reported on", o.Newest)
		case o.Status.Gate():
			gate = "the Healthy gate would pass"
		}
		fmt.Fprintf(w, "%s -> %s release %d: %s (%s; %s)\n", o.Check.Application, o.Check.Space, o.Release, o.Status, o.Did, gate)
	}
}

// Argo CD caches the digest it resolved for "latest", so a release published
// after it is not read until something asks Argo to resolve the tag again.
// argobot asks on every publish; an estate without argobot needs this.

// HardRefreshAnnotation is Argo CD's own request to resolve the source again.
const HardRefreshAnnotation = "argocd.argoproj.io/refresh"

// HardRefresh asks Argo CD to resolve an Application's source again, which is
// what makes it read a newly published release.
func HardRefresh(run Runner, app string) error {
	_, err := run("kubectl", "-n", checkNamespace, "annotate", "application", app, HardRefreshAnnotation+"=hard", "--overwrite")
	if err != nil {
		return fmt.Errorf("asking Argo CD to refresh %s: %w", app, err)
	}
	return nil
}

// Refresher asks for a hard refresh once per Application and release: Argo
// removes the annotation when it has refreshed, and a release it still has
// not synced after that is not helped by asking again every pass.
type Refresher struct {
	Run   Runner
	asked map[string]string
}

// Refresh asks for each reading whose newest release Argo has not read, and
// returns the Applications it asked for.
func (f *Refresher) Refresh(readings []Reading) ([]string, error) {
	if f.asked == nil {
		f.asked = map[string]string{}
	}
	var did, errs []string
	for _, r := range readings {
		if r.Unread == "" || r.Skip != "" || f.asked[r.Check.Application] == r.Unread {
			continue
		}
		if err := HardRefresh(f.Run, r.Check.Application); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		f.asked[r.Check.Application] = r.Unread
		did = append(did, r.Check.Application)
	}
	if len(errs) > 0 {
		return did, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return did, nil
}
