package flux

import (
	"fmt"
	"regexp"
	"strconv"
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

var attestationID = regexp.MustCompile(`attestation ([0-9a-f-]{36})`)

// RecordCheck records one check's verdict. It needs a field comparison: a
// claim that the cluster runs this release rests on more than the object set.
func RecordCheck(run Runner, r Result) (string, error) {
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
	args := []string{"attestation", "create", "--space", r.Check.Space,
		"--where", "Slug = '" + r.Check.Unit + "'",
		"--revision", strconv.Itoa(r.Release.UnitRevision),
		"--type", LiveCheckType,
		"--claim", "kustomize.toolkit.fluxcd.io/kustomization=" + ns + "/" + r.Check.Kustomization,
		"--claim", "confighub.com/release=" + r.Release.ManifestDigest}
	note := fmt.Sprintf("cub flux check: %d objects match what the layer applied, and every field the release sets matches on all %d", r.Inventory.Same, r.Fields.Total)
	if len(problems) > 0 {
		args = append(args, "--reject")
		note = "cub flux check: " + strings.Join(problems, "; ")
	}
	// A note is a reason, not a report: long enough to say what, short enough
	// to read in a list.
	if len(note) > 480 {
		note = note[:477] + "..."
	}
	out, err := run("cub", append(args, "--note", note)...)
	if err != nil {
		return "", fmt.Errorf("recording the check of %s: %w", r.Check.Kustomization, err)
	}
	if m := attestationID.FindStringSubmatch(string(out)); m != nil {
		return m[1], nil
	}
	return strings.TrimSpace(string(out)), nil
}
