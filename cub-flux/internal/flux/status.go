package flux

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// Live status is how ConfigHub hears what a cluster is running: a small JSON
// document in the confighub.com/live-status annotation of the variant's Space.
// argobot writes it for Argo CD and `cub sveltos status` for Sveltos; this
// writes it for Flux, in the same shape, so the Healthy gate, change order
// stages and the UI read all three alike.
//
// Two things make it more than a mirror of Flux's own conditions:
//
//   - ConfigHub's Healthy gate passes only on the exact words Synced,
//     Succeeded and Healthy, and nothing else checks what a reading claims. So
//     a reading says Synced only when the digest Flux applied is the newest
//     published release of that Space, and Healthy only when Flux actually
//     checked the workloads.
//   - ConfigHub advances a change order when a reading's revision equals one of
//     its releases' manifest digests. So the revision written is the digest Flux
//     reports it applied, never one inferred from times, which is what Sveltos
//     has to do.

// LiveStatusAnnotation is the Space annotation ConfigHub reads.
const LiveStatusAnnotation = "confighub.com/live-status"

// StatusSource names this reporter. The UI shows a Flux mark for any source
// containing "flux".
const StatusSource = "cub-flux"

// LiveStatus is ConfigHub's live-status document. The words are Argo CD's,
// because that is what ConfigHub's gate and UI read.
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

// Reading is what one layer says about itself, before anything is written.
type Reading struct {
	Check  Check      `json:"check"`
	Status LiveStatus `json:"status"`
	// Skip, when set, is why nothing is reported for this layer: it does not
	// read the Space the reading would be written to.
	Skip string `json:"skip,omitempty"`
}

// statusMessageLimit keeps a reading well inside the 1024 bytes ConfigHub
// allows an annotation value.
const statusMessageLimit = 200

// workloadKinds are what a layer runs, as opposed to what it merely declares.
// A layer applying none of them is healthy once it is applied; one applying
// any of them is healthy only if Flux checked them.
var workloadKinds = map[string]bool{
	"Deployment": true, "StatefulSet": true, "DaemonSet": true, "ReplicaSet": true,
	"Job": true, "CronJob": true, "Pod": true, "HelmRelease": true,
}

// ReadStatus reads one layer and says what ConfigHub should hear. A read that
// fails returns an error and no reading: a guess written here could open a
// gate or advance a change order.
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
		return r, nil
	}
	releases, err := publishedReleases(hub, c.Space)
	if err != nil {
		return r, err
	}
	r.Status = k.status(releases)
	r.Status.Source = StatusSource
	r.Status.App = c.Kustomization
	r.Status.ObservedAt = now.UTC().Format(time.RFC3339)
	if len(r.Status.Message) > statusMessageLimit {
		r.Status.Message = r.Status.Message[:statusMessageLimit-3] + "..."
	}
	return r, nil
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

// status maps a Kustomization onto ConfigHub's words.
func (k kustomization) status(releases []release) LiveStatus {
	if k.Spec.Suspend {
		return LiveStatus{SyncStatus: "Unknown", HealthStatus: "Suspended", Message: "the Kustomization is suspended, so Flux applies nothing"}
	}
	ready := k.condition("Ready")
	if k.Status.ObservedGeneration != k.Metadata.Generation || ready == nil || ready.ObservedGeneration != k.Metadata.Generation {
		return LiveStatus{SyncStatus: "Unknown", HealthStatus: "Progressing", OperationPhase: "Running", Message: "Flux has not reported on the current generation yet"}
	}
	if s := k.condition("Stalled"); s != nil && s.Status == "True" {
		return LiveStatus{SyncStatus: "OutOfSync", HealthStatus: "Degraded", OperationPhase: "Failed", Message: "stalled: " + s.Message}
	}
	if rc := k.condition("Reconciling"); rc != nil && rc.Status == "True" {
		return LiveStatus{SyncStatus: "OutOfSync", HealthStatus: "Progressing", OperationPhase: "Running", Message: rc.Message}
	}
	switch ready.Status {
	case "False":
		return LiveStatus{SyncStatus: "OutOfSync", HealthStatus: "Degraded", OperationPhase: "Failed", Message: ready.Message}
	case "True":
	default:
		return LiveStatus{SyncStatus: "OutOfSync", HealthStatus: "Progressing", OperationPhase: "Running", Message: ready.Message}
	}

	applied := digestOf(k.Status.LastAppliedRevision)
	if a := k.Status.LastAttemptedRevision; a != "" && digestOf(a) != applied {
		return LiveStatus{SyncStatus: "OutOfSync", HealthStatus: "Progressing", OperationPhase: "Running", Revision: applied,
			Message: "applying " + digestOf(a)}
	}
	health, why := "Healthy", ""
	if u := k.unchecked(); len(u) > 0 {
		// Ready then means applied, not running, for what Flux was not asked
		// to look at.
		health, why = "Unknown", "; Ready means applied, not healthy, for "+strings.Join(u, ", ")+": neither spec.wait nor a health check covers it"
	}
	var this, newest *release
	for i := range releases {
		if releases[i].Digest == applied {
			this = &releases[i]
		}
		if newest == nil || releases[i].Num > newest.Num {
			newest = &releases[i]
		}
	}
	switch {
	case this == nil:
		return LiveStatus{SyncStatus: "Unknown", HealthStatus: health, OperationPhase: "Succeeded", Revision: applied,
			Message: "Flux applied " + applied + ", which is no published release of this Space" + why}
	case this.Num != newest.Num:
		return LiveStatus{SyncStatus: "OutOfSync", HealthStatus: health, OperationPhase: "Succeeded", Revision: applied,
			Message: fmt.Sprintf("release %d is applied; release %d is published and not applied yet%s", this.Num, newest.Num, why)}
	}
	return LiveStatus{SyncStatus: "Synced", HealthStatus: health, OperationPhase: "Succeeded", Revision: applied,
		Message: fmt.Sprintf("release %d applied%s", this.Num, why)}
}

// digestOf takes the digest out of Flux's "<tag>@sha256:..." revision.
func digestOf(revision string) string {
	if i := strings.LastIndex(revision, "@"); i >= 0 {
		return revision[i+1:]
	}
	return revision
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

// Outcome is what reporting did for one layer.
type Outcome struct {
	Reading
	// Did is one of written, unchanged, dry-run or skipped.
	Did string `json:"did"`
}

// ReportStatus writes each reading that says something new, or that ConfigHub
// has held for longer than refresh: observedAt is the only sign a reporter is
// still running, so an unchanged reading is written again now and then.
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
		if r.Skip != "" {
			// A layer that has left this Space, handed back to Git say, leaves
			// behind the last reading written for it, and the Healthy gate
			// would go on passing on it. So a reading this reporter wrote is
			// replaced with one that says the layer no longer reads the Space.
			// Another reporter's reading is not ours to change.
			if !ok || held.Source != StatusSource {
				o.Did = "skipped"
				out = append(out, o)
				continue
			}
			r.Status = LiveStatus{Source: StatusSource, App: r.Check.Kustomization, SyncStatus: "Unknown", HealthStatus: "Unknown",
				Message: "the layer no longer reads this Space: it " + r.Skip, ObservedAt: now.UTC().Format(time.RFC3339)}
			if len(r.Status.Message) > statusMessageLimit {
				r.Status.Message = r.Status.Message[:statusMessageLimit-3] + "..."
			}
			o.Reading = r
		}
		if ok && held.Same(r.Status) {
			if at, err := time.Parse(time.RFC3339, held.ObservedAt); err == nil && now.Sub(at) < refresh {
				o.Did = "unchanged"
				out = append(out, o)
				continue
			}
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

// PrintOutcomes says what happened, one layer to a line.
func PrintOutcomes(w io.Writer, outs []Outcome) {
	for _, o := range outs {
		if o.Skip != "" && o.Did == "skipped" {
			fmt.Fprintf(w, "%s: not reported: %s\n", o.Check.Kustomization, o.Skip)
			continue
		}
		gate := "the Healthy gate would not pass"
		if o.Status.Gate() {
			gate = "the Healthy gate would pass"
		}
		fmt.Fprintf(w, "%s -> %s: %s (%s; %s)\n", o.Check.Kustomization, o.Check.Space, o.Status, o.Did, gate)
	}
}
