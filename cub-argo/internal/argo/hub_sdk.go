package argo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/confighub/sdk/core/cubapi"
	goclientnew "github.com/confighub/sdk/core/openapi/goclient-new"
)

// SDKHub answers Hub through the ConfigHub SDK, as the user cub is logged in
// as: the context and token cub passes a plugin, or the active context.
type SDKHub struct {
	userAgent string
}

// NewHub is the Hub the commands use. It connects when asked something, so a
// command that asks ConfigHub nothing needs no login.
func NewHub(version string) *SDKHub {
	return &SDKHub{userAgent: "cub-argo/" + version}
}

// client reads the login again for each question, as running cub did: that
// is a file read, not a request, and a status or watch left running for days
// then picks up a token that was renewed meanwhile.
func (h *SDKHub) client(ctx context.Context) (*cubapi.Client, error) {
	c, err := cubapi.ResolveClient(ctx, cubapi.ClientOptions{UserAgent: h.userAgent})
	if err != nil {
		return nil, fmt.Errorf("connecting to ConfigHub: %w", err)
	}
	return c, nil
}

func (h *SDKHub) space(ctx context.Context, c *cubapi.Client, slug string) (goclientnew.UUID, error) {
	s, err := cubapi.ResolveSpace(ctx, c, cubapi.ParseRef(slug), cubapi.ResolveOpts{})
	if err != nil {
		return goclientnew.UUID{}, err
	}
	return s.Space.SpaceID, nil
}

func (h *SDKHub) unit(ctx context.Context, c *cubapi.Client, space, slug string) (*goclientnew.Unit, error) {
	u, err := cubapi.ResolveUnit(ctx, c, cubapi.NewRef(space, slug), cubapi.ResolveOpts{})
	if err != nil {
		return nil, err
	}
	return u.Unit, nil
}

func (h *SDKHub) releases(ctx context.Context, where string, space string) ([]HubRelease, error) {
	c, err := h.client(ctx)
	if err != nil {
		return nil, err
	}
	id, err := h.space(ctx, c, space)
	if err != nil {
		return nil, err
	}
	// Named fields only: a release's bundle is large, and none of it is
	// wanted here.
	fields := "ReleaseID,ReleaseNum,SpaceID,OrganizationID,Published,ManifestDigest,TagID,LiveStatus"
	params := &goclientnew.ListExtendedReleasesParams{Select: &fields}
	if where != "" {
		params.Where = &where
	}
	res, err := c.API.ListExtendedReleasesWithResponse(ctx, id, params)
	if cubapi.IsAPIError(err, res) {
		return nil, cubapi.InterpretErrorGeneric(err, res)
	}
	var out []HubRelease
	if res.JSON200 == nil {
		return out, nil
	}
	for _, er := range *res.JSON200 {
		if er.Release == nil {
			continue
		}
		r := HubRelease{Num: int(er.Release.ReleaseNum), ManifestDigest: er.Release.ManifestDigest, Published: er.Release.Published}
		if ls := er.Release.LiveStatus; ls != nil {
			r.Live = &LiveStatus{
				Reporter: ls.Reporter, DataSource: ls.DataSource,
				Sync: string(ls.Sync), Health: string(ls.Health), Operation: string(ls.Operation),
				ReporterSync: ls.ReporterSync, ReporterHealth: ls.ReporterHealth, ReporterOperation: ls.ReporterOperation,
				Message: ls.Message,
			}
			if !ls.ObservedAt.IsZero() {
				r.Live.ObservedAt = ls.ObservedAt.UTC().Format(time.RFC3339)
			}
		}
		if er.Release.TagID != nil {
			r.TagID = er.Release.TagID.String()
		}
		out = append(out, r)
	}
	return out, nil
}

// Release finds a release the way `cub release get --oci-reference` does: by
// its manifest digest, or the newest published one for "latest".
func (h *SDKHub) Release(space, ref string) (HubRelease, error) {
	where := "Published = true"
	if ref != "" && ref != "latest" {
		if !strings.HasPrefix(strings.ToLower(ref), "sha256:") {
			return HubRelease{}, fmt.Errorf("release %q is neither \"latest\" nor a manifest digest (sha256:...)", ref)
		}
		w := cubapi.Where{}.Eq("ManifestDigest", ref)
		if err := w.Err(); err != nil {
			return HubRelease{}, err
		}
		where = w.String()
	}
	rs, err := h.releases(context.Background(), where, space)
	if err != nil {
		return HubRelease{}, err
	}
	var newest *HubRelease
	for i := range rs {
		if newest == nil || rs[i].Num > newest.Num {
			newest = &rs[i]
		}
	}
	if newest == nil {
		return HubRelease{}, fmt.Errorf("no release %s found in space %s", ref, space)
	}
	return *newest, nil
}

func (h *SDKHub) Releases(space string) ([]HubRelease, error) {
	return h.releases(context.Background(), "", space)
}

func (h *SDKHub) revisions(ctx context.Context, c *cubapi.Client, u *goclientnew.Unit, where string) ([]goclientnew.ExtendedRevision, error) {
	res, err := c.API.ListExtendedRevisionsWithResponse(ctx, u.SpaceID, u.UnitID, &goclientnew.ListExtendedRevisionsParams{Where: &where})
	if cubapi.IsAPIError(err, res) {
		return nil, cubapi.InterpretErrorGeneric(err, res)
	}
	if res.JSON200 == nil {
		return nil, nil
	}
	return *res.JSON200, nil
}

func (h *SDKHub) TaggedRevision(space, unit, tagID string) (int, bool, error) {
	ctx := context.Background()
	c, err := h.client(ctx)
	if err != nil {
		return 0, false, err
	}
	u, err := h.unit(ctx, c, space, unit)
	if err != nil {
		return 0, false, err
	}
	if strings.ContainsAny(tagID, "'\\") {
		return 0, false, fmt.Errorf("tag %q cannot be named in a filter", tagID)
	}
	revs, err := h.revisions(ctx, c, u, "Tags ? '"+tagID+"'")
	if err != nil {
		return 0, false, err
	}
	// The newest revision carrying the tag, whatever order they come in.
	newest, found := 0, false
	for _, r := range revs {
		if r.Revision != nil && (!found || int(r.Revision.RevisionNum) > newest) {
			newest, found = int(r.Revision.RevisionNum), true
		}
	}
	return newest, found, nil
}

func (h *SDKHub) RevisionData(space, unit string, revision int) ([]byte, error) {
	ctx := context.Background()
	c, err := h.client(ctx)
	if err != nil {
		return nil, err
	}
	u, err := h.unit(ctx, c, space, unit)
	if err != nil {
		return nil, err
	}
	revs, err := h.revisions(ctx, c, u, "RevisionNum = "+strconv.Itoa(revision))
	if err != nil {
		return nil, err
	}
	for _, r := range revs {
		if r.Revision == nil || int(r.Revision.RevisionNum) != revision {
			continue
		}
		// The body is the configuration itself, not a JSON envelope.
		res, err := c.API.DownloadRevisionDataWithResponse(ctx, u.SpaceID, u.UnitID, r.Revision.RevisionID)
		if err != nil {
			return nil, err
		}
		if res.StatusCode() != http.StatusOK {
			if apiErr := cubapi.InterpretErrorGeneric(nil, res); apiErr != nil {
				return nil, apiErr
			}
			return nil, fmt.Errorf("reading revision %d of %s: %s", revision, unit, res.Status())
		}
		return res.Body, nil
	}
	return nil, fmt.Errorf("revision %d of %s not found in space %s", revision, unit, space)
}

func (h *SDKHub) UnitHead(space, unit string) (int, error) {
	ctx := context.Background()
	c, err := h.client(ctx)
	if err != nil {
		return 0, err
	}
	u, err := h.unit(ctx, c, space, unit)
	if err != nil {
		return 0, err
	}
	return int(u.HeadRevisionNum), nil
}

// SetLiveStatus patches the Release with the whole status. Every field is
// sent, the empty ones as null: a merge patch leaves out what it does not
// name, and what an earlier reporter wrote must not stay beside this.
func (h *SDKHub) SetLiveStatus(space string, release int, st LiveStatus) error {
	ctx := context.Background()
	c, err := h.client(ctx)
	if err != nil {
		return err
	}
	id, err := h.space(ctx, c, space)
	if err != nil {
		return err
	}
	where := "ReleaseNum = " + strconv.Itoa(release)
	only := "ReleaseID,ReleaseNum,SpaceID,OrganizationID"
	res, err := c.API.ListExtendedReleasesWithResponse(ctx, id, &goclientnew.ListExtendedReleasesParams{Where: &where, Select: &only})
	if cubapi.IsAPIError(err, res) {
		return cubapi.InterpretErrorGeneric(err, res)
	}
	var releaseID *goclientnew.UUID
	if res.JSON200 != nil {
		for _, er := range *res.JSON200 {
			if er.Release != nil && int(er.Release.ReleaseNum) == release {
				releaseID = &er.Release.ReleaseID
			}
		}
	}
	if releaseID == nil {
		return fmt.Errorf("release %d not found in space %s", release, space)
	}
	if _, err := time.Parse(time.RFC3339, st.ObservedAt); err != nil {
		return fmt.Errorf("the status names no time it was observed: %w", err)
	}
	fields := map[string]any{}
	for name, value := range map[string]string{
		"Reporter": st.Reporter, "DataSource": st.DataSource,
		"Sync": st.Sync, "Health": st.Health, "Operation": st.Operation,
		"ReporterSync": st.ReporterSync, "ReporterHealth": st.ReporterHealth, "ReporterOperation": st.ReporterOperation,
		"Message": st.Message, "ObservedAt": st.ObservedAt,
	} {
		if value == "" {
			fields[name] = nil
		} else {
			fields[name] = value
		}
	}
	patch, err := json.Marshal(map[string]any{"LiveStatus": fields})
	if err != nil {
		return err
	}
	pres, err := c.API.PatchReleaseWithBodyWithResponse(ctx, id, *releaseID, &goclientnew.PatchReleaseParams{}, "application/merge-patch+json", bytes.NewReader(patch))
	if cubapi.IsAPIError(err, pres) {
		return cubapi.InterpretErrorGeneric(err, pres)
	}
	return nil
}

func (h *SDKHub) Attest(a Attestation) (string, error) {
	ctx := context.Background()
	c, err := h.client(ctx)
	if err != nil {
		return "", err
	}
	id, err := h.space(ctx, c, a.Space)
	if err != nil {
		return "", err
	}
	unit := cubapi.Where{}.Eq("Slug", a.Unit)
	if err := unit.Err(); err != nil {
		return "", err
	}
	req := goclientnew.AttestationCreateRequest{
		Type:      a.Type,
		Note:      a.Note,
		Claims:    a.Claims,
		WhereUnit: unit.String(),
		Revision:  strconv.Itoa(a.Revision),
	}
	if a.Reject {
		req.Result = "Fail"
	}
	out, err := cubapi.CreateAttestation(ctx, c, id, req, false)
	if err != nil {
		return "", err
	}
	if out.Attestation == nil {
		return "", fmt.Errorf("no attestation was recorded: no revision %d of %s in %s", a.Revision, a.Unit, a.Space)
	}
	return out.Attestation.AttestationID.String(), nil
}
