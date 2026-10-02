package flux

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
	// SpaceAnnotations are the Space's annotations.
	SpaceAnnotations(space string) (map[string]string, error)
	// PatchSpace merges a JSON merge patch into the Space.
	PatchSpace(space string, patch []byte) error
	// Attest records an attestation and returns its ID.
	Attest(a Attestation) (string, error)
	// UnitSlugs names the Units of a Space.
	UnitSlugs(space string) ([]string, error)
	// HasTarget says whether the Space holds a Target of that name.
	HasTarget(space, target string) (bool, error)
}

// HubRelease is one release of a Space.
type HubRelease struct {
	Num            int
	ManifestDigest string
	TagID          string
	Published      bool
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
