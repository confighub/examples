package argo

import "fmt"

// A handover delivers a published Release, not a unit's head. The two differ
// whenever someone has changed the unit since the last publish, so a check
// that read the head could pass while the cluster is handed something else.
// So the check reads the unit exactly as the Release bundled it, and names
// the Release by its manifest digest: the digest the gateway serves, and the
// one the controller records once it has fetched it.

// Release is the published Release a check compared against.
type Release struct {
	Num            int    `json:"num"`
	ManifestDigest string `json:"manifestDigest"`
	// UnitRevision is the revision of the unit the Release bundled;
	// HeadRevision is the unit's head now.
	UnitRevision int `json:"unitRevision"`
	HeadRevision int `json:"headRevision"`
}

// HeadAhead says, when the unit has changed since the Release, what that
// means. It is not a failure: the Release is what the handover delivers.
func (r Release) HeadAhead() string {
	if r.HeadRevision <= r.UnitRevision {
		return ""
	}
	return fmt.Sprintf("the unit's head is revision %d, and release %d holds revision %d; the handover delivers the release, so that is what was checked. Publish again if the head is what you mean to deliver", r.HeadRevision, r.Num, r.UnitRevision)
}

// ReleasedData reads a unit as the Space's Release bundled it. ref is
// "latest", the newest published Release, or a manifest digest naming one
// exactly.
func ReleasedData(hub Hub, space, unit, ref string) (Release, []byte, error) {
	if ref == "" {
		ref = "latest"
	}
	rel, err := hub.Release(space, ref)
	if err != nil {
		return Release{}, nil, fmt.Errorf("reading the release of %s (%s): %w. A handover delivers a published release, so without one there is nothing to check", space, ref, err)
	}
	if rel.TagID == "" {
		return Release{}, nil, fmt.Errorf("release %d of %s names no tag, so which revision of %s it bundled cannot be told", rel.Num, space, unit)
	}
	r := Release{Num: rel.Num, ManifestDigest: rel.ManifestDigest}

	rev, found, err := hub.TaggedRevision(space, unit, rel.TagID)
	if err != nil {
		return Release{}, nil, fmt.Errorf("finding the revision of %s in release %d: %w", unit, r.Num, err)
	}
	if !found {
		return Release{}, nil, fmt.Errorf("release %d of %s does not hold %s, so a handover would deliver none of its objects", r.Num, space, unit)
	}
	r.UnitRevision = rev

	data, err := hub.RevisionData(space, unit, r.UnitRevision)
	if err != nil {
		return Release{}, nil, fmt.Errorf("reading %s at revision %d: %w", unit, r.UnitRevision, err)
	}

	if head, err := hub.UnitHead(space, unit); err == nil {
		r.HeadRevision = head
	}
	return r, data, nil
}
