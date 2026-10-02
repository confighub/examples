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

// RecordCheck records one check's verdict. It needs a field comparison: a
// claim that the cluster runs this release rests on more than the object set.
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
	note := fmt.Sprintf("cub flux check: %d objects match what the layer applied, and every field the release sets matches on all %d", r.Inventory.Same, r.Fields.Total)
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
			"confighub.com/release":                     r.Release.ManifestDigest,
		},
		Reject: reject, Note: note,
	})
	if err != nil {
		return "", fmt.Errorf("recording the check of %s: %w", r.Check.Kustomization, err)
	}
	return id, nil
}
