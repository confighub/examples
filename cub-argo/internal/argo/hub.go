package argo

// The plugin asks ConfigHub what it holds through the SDK, in this process,
// rather than by running cub and reading what it prints. Hub is everything it
// asks; the SDK-backed answer is in hub_sdk.go, and tests stand in for it.

// Hub is what the plugin asks ConfigHub at run time.
type Hub interface {
	// Release is the Space's published release named by ref: "latest", the
	// newest, or a manifest digest naming one exactly.
	Release(space, ref string) (HubRelease, error)
	// Releases lists the Space's releases, published or not.
	Releases(space string) ([]HubRelease, error)
	// TaggedRevision is the revision of unit that carries the tag, which is
	// how a release names the revisions it bundled.
	TaggedRevision(space, unit, tagID string) (revision int, found bool, err error)
	// RevisionData is unit's configuration at that revision.
	RevisionData(space, unit string, revision int) ([]byte, error)
	// UnitHead is unit's newest revision.
	UnitHead(space, unit string) (int, error)
	// SetLiveStatus records what a tool says about a release of the Space
	// running, on that Release, replacing what it held.
	SetLiveStatus(space string, release int, s LiveStatus) error
	// Attest records an attestation and returns its ID.
	Attest(a Attestation) (string, error)
}

// HubRelease is one release of a Space.
type HubRelease struct {
	Num            int
	ManifestDigest string
	TagID          string
	Published      bool
	// Live is what a tool has reported about the release running, if any has.
	Live *LiveStatus
}

// Attestation is a verdict recorded on one revision of one Unit.
type Attestation struct {
	Space, Unit string
	Revision    int
	Type        string
	Claims      map[string]string
	Reject      bool
	Note        string
}
