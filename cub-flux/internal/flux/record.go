package flux

import (
	"fmt"
	"strings"
)

// A check that passes is evidence, and evidence that stays in a terminal
// governs nothing. --record writes the verdict to ConfigHub as an attestation
// on the unit revision the checked release bundled: a Pass, or a rejection
// naming what differs. The type is LiveCheck, the one cub kubara records for
// the same claim, so a workflow can require one type whichever plugin
// checked.

// LiveCheckType is the attestation type a recorded check carries.
const LiveCheckType = "LiveCheck"

// RecordCheck records one check's verdict, and returns the attestation's ID.
// It needs a field comparison: a claim that the cluster runs this release
// rests on more than the object set. Health is part of the claim: a layer
// Flux reports stalled or not ready is a rejection, and one still reconciling
// records nothing and returns no ID, since it is neither yet. A Pass names
// the health it rests on, which is Unknown where Flux was not asked to check
// the workloads.
func RecordCheck(hub Hub, r Result) (string, error) {
	if r.Fields == nil {
		return "", fmt.Errorf("%s: --record needs --fields, so the claim rests on every field the release sets", r.Check.Kustomization)
	}
	if r.Release.UnitRevision == 0 || r.Release.ManifestDigest == "" {
		return "", fmt.Errorf("%s: no release was identified, so there is no revision to attest to", r.Check.Kustomization)
	}
	var problems []string
	problems = append(problems, r.Inventory.WouldPrune...)
	problems = append(problems, r.Inventory.WouldAdd...)
	problems = append(problems, r.Stale...)
	problems = append(problems, r.Fields.Unreadable...)
	for _, d := range r.Fields.Diffs {
		problems = append(problems, d.String())
	}
	if r.Fields.Compared < r.Fields.Total {
		problems = append(problems, fmt.Sprintf("fields compared on %d of %d objects", r.Fields.Compared, r.Fields.Total))
	}
	ns := checkNamespace
	if r.Unhealthy != "" {
		problems = append(problems, r.Unhealthy)
	}
	if len(problems) == 0 && r.NotYet != "" {
		return "", nil
	}
	note := fmt.Sprintf("cub flux check: %d objects match what the layer applied, every field the release sets matches on all %d, and Flux reports it ready", r.Inventory.Same, r.Fields.Total)
	if r.Health == "Unknown" {
		note += "; Flux was not asked to check its workloads, so that means applied, not healthy"
	}
	reject := len(problems) > 0
	if reject {
		note = "cub flux check: " + strings.Join(problems, "; ")
	}
	// A note is a reason, not a report: long enough to say what, short enough
	// to read in a list.
	if len(note) > 480 {
		note = note[:477] + "..."
	}
	id, err := hub.Attest(Attestation{
		Space: r.Check.Space, Unit: r.Check.Unit, Revision: r.Release.UnitRevision,
		Type: LiveCheckType,
		Claims: map[string]string{
			"kustomize.toolkit.fluxcd.io/kustomization": ns + "/" + r.Check.Kustomization,
			"kustomize.toolkit.fluxcd.io/health":        orSay(r.Health, "none"),
			"confighub.com/release":                     r.Release.ManifestDigest,
		},
		Reject: reject, Note: note,
	})
	if err != nil {
		return "", fmt.Errorf("recording the check of %s: %w", r.Check.Kustomization, err)
	}
	return id, nil
}
