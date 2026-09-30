package argo

import (
	"encoding/json"
	"fmt"
	"strconv"
)

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
func ReleasedData(run Runner, space, unit, ref string) (Release, []byte, error) {
	if ref == "" {
		ref = "latest"
	}
	out, err := run("cub", "release", "get", "--space", space, "--oci-reference", ref, "-o", "json")
	if err != nil {
		return Release{}, nil, fmt.Errorf("reading the release of %s (%s): %w. A handover delivers a published release, so without one there is nothing to check", space, ref, err)
	}
	var rel struct {
		Release struct {
			ReleaseNum     int     `json:"ReleaseNum"`
			ManifestDigest string  `json:"ManifestDigest"`
			TagID          *string `json:"TagID"`
		} `json:"Release"`
	}
	if err := json.Unmarshal(out, &rel); err != nil {
		return Release{}, nil, fmt.Errorf("reading the release of %s: %w", space, err)
	}
	if rel.Release.TagID == nil || *rel.Release.TagID == "" {
		return Release{}, nil, fmt.Errorf("release %d of %s names no tag, so which revision of %s it bundled cannot be told", rel.Release.ReleaseNum, space, unit)
	}
	r := Release{Num: rel.Release.ReleaseNum, ManifestDigest: rel.Release.ManifestDigest}

	out, err = run("cub", "revision", "list", "--space", space, unit, "--where", "Tags ? '"+*rel.Release.TagID+"'", "-o", "json")
	if err != nil {
		return Release{}, nil, fmt.Errorf("finding the revision of %s in release %d: %w", unit, r.Num, err)
	}
	var revs []struct {
		Revision struct {
			RevisionNum int `json:"RevisionNum"`
		} `json:"Revision"`
	}
	if err := json.Unmarshal(out, &revs); err != nil {
		return Release{}, nil, fmt.Errorf("finding the revision of %s in release %d: %w", unit, r.Num, err)
	}
	if len(revs) == 0 {
		return Release{}, nil, fmt.Errorf("release %d of %s does not hold %s, so a handover would deliver none of its objects", r.Num, space, unit)
	}
	r.UnitRevision = revs[0].Revision.RevisionNum

	data, err := run("cub", "revision", "data", "--space", space, unit, strconv.Itoa(r.UnitRevision))
	if err != nil {
		return Release{}, nil, fmt.Errorf("reading %s at revision %d: %w", unit, r.UnitRevision, err)
	}

	out, err = run("cub", "unit", "get", "--space", space, unit, "-o", "json")
	if err == nil {
		var u struct {
			Unit struct {
				HeadRevisionNum int `json:"HeadRevisionNum"`
			} `json:"Unit"`
		}
		if json.Unmarshal(out, &u) == nil {
			r.HeadRevision = u.Unit.HeadRevisionNum
		}
	}
	return r, data, nil
}
