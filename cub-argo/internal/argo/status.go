package argo

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// Live status is how ConfigHub hears what a cluster is running: a small JSON
// document in the confighub.com/live-status annotation of the Space an
// Application reads. On a cluster `cub cluster up` made, argobot writes it. An
// estate onboarded here has no argobot, so without this nothing would, and
// ConfigHub's Healthy gate, its change order stages and its UI would have
// nothing to read.
//
// Argo CD's own words are close to what ConfigHub wants, but not the same:
//
//   - Argo says Synced when the cluster matches the revision Argo last
//     resolved for "latest", and it caches that digest until a hard refresh.
//     So after a publish Argo goes on saying Synced at the release before. A
//     reading here says Synced only when the digest Argo synced is the newest
//     published release of that Space.
//   - ConfigHub advances a change order when a reading's revision equals one of
//     its releases' manifest digests. For an oci:// source Argo records that
//     digest in status.sync.revision, and that is what is written, never a
//     digest inferred from anything else.

// LiveStatusAnnotation is the Space annotation ConfigHub reads.
const LiveStatusAnnotation = "confighub.com/live-status"

// StatusSource names this reporter. The UI shows an Argo mark for any source
// containing "argo".
const StatusSource = "cub-argo"

// LiveStatus is ConfigHub's live-status document, in Argo CD's words.
type LiveStatus struct {
	Source         string `json:"source"`
	App            string `json:"app,omitempty"`
	SyncStatus     string `json:"syncStatus,omitempty"`
	HealthStatus   string `json:"healthStatus,omitempty"`
	OperationPhase string `json:"operationPhase,omitempty"`
	Revision       string `json:"revision,omitempty"`
	Message        string `json:"message,omitempty"`
	ObservedAt     string `json:"observedAt"`
}

// Same reports whether two readings say the same thing, whenever each was
// taken.
func (s LiveStatus) Same(o LiveStatus) bool {
	s.ObservedAt, o.ObservedAt = "", ""
	return s == o
}

// Gate reports whether ConfigHub's Healthy prerequisite would pass on this
// reading.
func (s LiveStatus) Gate() bool {
	return s.SyncStatus == "Synced" && s.OperationPhase == "Succeeded" && s.HealthStatus == "Healthy"
}

func (s LiveStatus) String() string {
	parts := []string{s.SyncStatus, s.HealthStatus}
	if s.OperationPhase != "" {
		parts = append(parts, s.OperationPhase)
	}
	out := strings.Join(parts, "/")
	if s.Revision != "" {
		out += " at " + s.Revision
	}
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

// Reading is what one Application says about itself, before anything is
// written.
type Reading struct {
	Check  StatusCheck `json:"check"`
	Status LiveStatus  `json:"status"`
	// Skip, when set, is why nothing is reported for this Application: it
	// does not read the Space the reading would be written to.
	Skip string `json:"skip,omitempty"`
	// Unread is the digest of the newest published release when Argo CD has
	// synced an older one: what a hard refresh would make it read.
	Unread string `json:"unread,omitempty"`
}

// statusMessageLimit keeps a reading well inside the 1024 bytes ConfigHub
// allows an annotation value.
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

// ReadStatus reads one Application and says what ConfigHub should hear. A read
// that fails returns an error and no reading: a guess written here could open
// a gate or advance a change order.
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
		return r, nil
	}
	releases, err := publishedReleases(hub, c.Space)
	if err != nil {
		return r, err
	}
	r.Status = a.status(src, releases)
	if len(releases) > 0 && r.Status.Revision != "" {
		if newest := releases[len(releases)-1]; newest.Digest != r.Status.Revision {
			r.Unread = newest.Digest
		}
	}
	r.Status.Source = StatusSource
	r.Status.App = c.Application
	r.Status.ObservedAt = now.UTC().Format(time.RFC3339)
	if len(r.Status.Message) > statusMessageLimit {
		r.Status.Message = r.Status.Message[:statusMessageLimit-3] + "..."
	}
	return r, nil
}

// status maps an Application onto ConfigHub's words.
func (a application) status(src appSource, releases []release) LiveStatus {
	health := a.Status.Health.Status
	if health == "" {
		health = "Unknown"
	}
	phase := ""
	if op := a.Status.OperationState; op != nil {
		phase = op.Phase
	}
	if !a.comparedWith(src) {
		return LiveStatus{SyncStatus: "Unknown", HealthStatus: health, OperationPhase: phase,
			Message: "Argo CD has not compared the Application with its ConfigHub source yet"}
	}
	revision := a.Status.Sync.Revision
	if p := a.problem(); p != "" {
		// Argo can go on saying Synced from the last comparison that worked
		// while it reports an error now. That is not a reading to pass a gate
		// on.
		sync := orUnknown(a.Status.Sync.Status)
		if sync == "Synced" {
			sync = "Unknown"
		}
		return LiveStatus{SyncStatus: sync, HealthStatus: health, OperationPhase: phase, Revision: revision, Message: p}
	}
	if a.Status.Sync.Status != "Synced" {
		// The last operation's message describes that operation, which may
		// have succeeded before the source moved on, so it is the last resort.
		msg := a.outOfSync()
		if msg == "" {
			msg = a.Status.Health.Message
		}
		if op := a.Status.OperationState; msg == "" && op != nil {
			msg = op.Message
		}
		return LiveStatus{SyncStatus: orUnknown(a.Status.Sync.Status), HealthStatus: health, OperationPhase: phase, Revision: revision, Message: msg}
	}
	why := ""
	if phase == "" {
		why = "; Argo CD has run no sync operation, so none has succeeded"
	}
	var this, newest *release
	for i := range releases {
		if releases[i].Digest == revision {
			this = &releases[i]
		}
		if newest == nil || releases[i].Num > newest.Num {
			newest = &releases[i]
		}
	}
	switch {
	case this == nil:
		return LiveStatus{SyncStatus: "Unknown", HealthStatus: health, OperationPhase: phase, Revision: revision,
			Message: "Argo CD synced " + revision + ", which is no published release of this Space" + why}
	case this.Num != newest.Num:
		return LiveStatus{SyncStatus: "OutOfSync", HealthStatus: health, OperationPhase: phase, Revision: revision,
			Message: fmt.Sprintf("release %d is synced; release %d is published and Argo CD has not read it: it caches the digest behind latest until a hard refresh (status --hard-refresh asks for one)%s", this.Num, newest.Num, why)}
	}
	return LiveStatus{SyncStatus: "Synced", HealthStatus: health, OperationPhase: phase, Revision: revision,
		Message: fmt.Sprintf("release %d synced%s", this.Num, why)}
}

func orUnknown(s string) string {
	if s == "" {
		return "Unknown"
	}
	return s
}

type release struct {
	Num    int
	Digest string
}

func publishedReleases(hub Hub, space string) ([]release, error) {
	list, err := hub.Releases(space)
	if err != nil {
		return nil, fmt.Errorf("listing the releases of %s: %w", space, err)
	}
	var rs []release
	for _, l := range list {
		if l.Published {
			rs = append(rs, release{Num: l.Num, Digest: l.ManifestDigest})
		}
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].Num < rs[j].Num })
	return rs, nil
}

// HeldStatus is the reading ConfigHub holds for a Space now, if any.
func HeldStatus(hub Hub, space string) (LiveStatus, bool, error) {
	a, err := hub.SpaceAnnotations(space)
	if err != nil {
		return LiveStatus{}, false, fmt.Errorf("reading Space %s: %w", space, err)
	}
	raw, ok := a[LiveStatusAnnotation]
	if !ok {
		return LiveStatus{}, false, nil
	}
	var held LiveStatus
	if err := json.Unmarshal([]byte(raw), &held); err != nil {
		// Unreadable is as good as absent: it will be replaced.
		return LiveStatus{}, false, nil
	}
	return held, true, nil
}

// StatusPatch is the body that sets only the live-status annotation, leaving
// every other field and annotation of the Space as it is.
func StatusPatch(s LiveStatus) ([]byte, error) {
	doc, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"Annotations": map[string]string{LiveStatusAnnotation: string(doc)}})
}

// Outcome is what reporting did for one Application.
type Outcome struct {
	Reading
	// Did is one of written, unchanged, dry-run, skipped or left.
	Did string `json:"did"`
	// Why says, for left, whose reading was left alone.
	Why string `json:"why,omitempty"`
}

// ReportStatus writes each reading that says something new, or that ConfigHub
// has held for longer than refresh: observedAt is the only sign a reporter is
// still running, so an unchanged reading is written again now and then.
//
// A reading another reporter wrote, argobot say, is left alone while it is
// fresher than refresh: two reporters on one Space would overwrite each other
// on every pass. One older than that is from a reporter that has stopped, and
// is replaced.
func ReportStatus(hub Hub, readings []Reading, refresh time.Duration, dryRun bool, now time.Time) ([]Outcome, error) {
	var out []Outcome
	// A Space that cannot be read or written is reported, and the rest are
	// still reported: one deleted Space must not freeze every reading after it.
	var errs []string
	for _, r := range readings {
		o := Outcome{Reading: r}
		held, ok, err := HeldStatus(hub, r.Check.Space)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		fresh := false
		if ok {
			if at, err := time.Parse(time.RFC3339, held.ObservedAt); err == nil && now.Sub(at) < refresh {
				fresh = true
			}
		}
		if ok && held.Source != StatusSource && fresh {
			o.Did = "left"
			o.Why = fmt.Sprintf("%s reported this Space at %s; two reporters would overwrite each other", held.Source, held.ObservedAt)
			out = append(out, o)
			continue
		}
		if r.Skip != "" {
			// An Application that has left this Space, handed back to Git say,
			// leaves behind the last reading written for it, and the Healthy
			// gate would go on passing on it. So a reading this reporter wrote
			// is replaced with one that says the Application no longer reads
			// the Space. Another reporter's reading is not ours to change.
			if !ok || held.Source != StatusSource {
				o.Did = "skipped"
				out = append(out, o)
				continue
			}
			r.Status = LiveStatus{Source: StatusSource, App: r.Check.Application, SyncStatus: "Unknown", HealthStatus: "Unknown",
				Message: "the Application no longer reads this Space: it " + r.Skip, ObservedAt: now.UTC().Format(time.RFC3339)}
			if len(r.Status.Message) > statusMessageLimit {
				r.Status.Message = r.Status.Message[:statusMessageLimit-3] + "..."
			}
			o.Reading = r
		}
		if ok && held.Same(r.Status) && fresh {
			o.Did = "unchanged"
			out = append(out, o)
			continue
		}
		if dryRun {
			o.Did = "dry-run"
			out = append(out, o)
			continue
		}
		patch, err := StatusPatch(r.Status)
		if err != nil {
			return out, err
		}
		if err := hub.PatchSpace(r.Check.Space, patch); err != nil {
			errs = append(errs, fmt.Sprintf("writing the live status of %s: %v", r.Check.Space, err))
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
		case o.Skip != "" && o.Did == "skipped":
			fmt.Fprintf(w, "%s: not reported: %s\n", o.Check.Application, o.Skip)
			continue
		}
		gate := "the Healthy gate would not pass"
		if o.Status.Gate() {
			gate = "the Healthy gate would pass"
		}
		fmt.Fprintf(w, "%s -> %s: %s (%s; %s)\n", o.Check.Application, o.Check.Space, o.Status, o.Did, gate)
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
