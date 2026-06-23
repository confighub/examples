// Package chub talks to the ConfigHub REST API directly — the same surface the
// rbac-manager / sec-scanner web apps use — to read image references from the
// fleet and write scan results back as data. No `cub` CLI shell-out.
//
// Configuration mirrors the app's bearer-token mode:
//
//	CONFIGHUB_URL    server base URL (default https://hub.confighub.com)
//	CONFIGHUB_TOKEN  bearer token (get one with `cub auth get-token`)
package chub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Workload is one fleet Unit and the container images it references, plus the
// scan provenance recorded on it (when it was scanned, against which DB version).
type Workload struct {
	Space        string
	SpaceID      string
	Unit         string
	UnitID       string
	Images       []string
	ScannedAt    string // sec-scanner.confighub.com/scanned-at annotation
	CVEDBVersion string // sec-scanner.confighub.com/cvedb-version annotation
}

func baseURL() string {
	u := os.Getenv("CONFIGHUB_URL")
	if u == "" {
		u = "https://hub.confighub.com"
	}
	return strings.TrimRight(u, "/") + "/api"
}

func token() string { return os.Getenv("CONFIGHUB_TOKEN") }

var httpClient = &http.Client{Timeout: 60 * time.Second}

// do issues an authenticated request against the ConfigHub API.
func do(method, path string, query url.Values, body []byte) ([]byte, error) {
	if token() == "" {
		return nil, fmt.Errorf("CONFIGHUB_TOKEN is not set (get one with: cub auth get-token)")
	}
	u := baseURL() + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, u, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token())
	req.Header.Set("Accept", "application/json")
	if body != nil {
		ct := "application/json"
		if method == http.MethodPatch {
			ct = "application/merge-patch+json" // ConfigHub's patch content type
		}
		req.Header.Set("Content-Type", ct)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return b, nil
}

// unitListEntry matches the shape of GET /unit (the SDK's ListAllUnits).
type unitListEntry struct {
	Unit struct {
		Slug    string            `json:"Slug"`
		UnitID  string            `json:"UnitID"`
		SpaceID string            `json:"SpaceID"`
		Labels  map[string]string `json:"Labels"`
	} `json:"Unit"`
	Space struct {
		Slug string `json:"Slug"`
	} `json:"Space"`
}

// Inventory lists Units across the space glob (optionally filtered by a --where
// expression) and extracts the container images each references.
//
// cub's familiar "sec-demo-*" glob has no API equivalent, so a partial glob is
// translated to a Space.Slug LIKE filter and an exact slug to Space.Slug =.
func Inventory(spaceGlob, where string) ([]Workload, error) {
	if f := spaceFilter(spaceGlob); f != "" {
		if where == "" {
			where = f
		} else {
			where = "(" + where + ") AND " + f
		}
	}
	q := url.Values{}
	if where != "" {
		q.Set("where", where)
	}
	q.Set("select", "Slug,UnitID,SpaceID,Labels")
	q.Set("include", "SpaceID")
	b, err := do("GET", "/unit", q, nil)
	if err != nil {
		return nil, err
	}
	var entries []unitListEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, fmt.Errorf("parse unit list: %w", err)
	}

	var wls []Workload
	for _, e := range entries {
		data, err := unitData(e.Unit.SpaceID, e.Unit.UnitID)
		if err != nil {
			return nil, err
		}
		imgs := ExtractImages(data)
		if len(imgs) == 0 {
			continue
		}
		wls = append(wls, Workload{
			Space: e.Space.Slug, SpaceID: e.Unit.SpaceID,
			Unit: e.Unit.Slug, UnitID: e.Unit.UnitID, Images: imgs,
			ScannedAt:    annoValue(data, "sec-scanner.confighub.com/scanned-at"),
			CVEDBVersion: annoValue(data, "sec-scanner.confighub.com/cvedb-version"),
		})
	}
	return wls, nil
}

// annoValue extracts a single annotation value from a manifest's YAML text.
func annoValue(yaml, key string) string {
	re := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(key) + `:\s*["']?([^"'\n]+?)["']?\s*$`)
	if m := re.FindStringSubmatch(yaml); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// ResolveSpaceID returns the SpaceID for a space slug.
func ResolveSpaceID(slug string) (string, error) {
	q := url.Values{}
	q.Set("where", fmt.Sprintf("Slug = '%s'", slug))
	q.Set("select", "SpaceID,Slug")
	b, err := do("GET", "/space", q, nil)
	if err != nil {
		return "", err
	}
	var spaces []struct {
		Space struct {
			SpaceID string `json:"SpaceID"`
		} `json:"Space"`
	}
	if err := json.Unmarshal(b, &spaces); err != nil || len(spaces) == 0 {
		return "", fmt.Errorf("space %q not found", slug)
	}
	return spaces[0].Space.SpaceID, nil
}

func spaceFilter(glob string) string {
	if glob == "" || glob == "*" {
		return ""
	}
	if strings.Contains(glob, "*") {
		return fmt.Sprintf("Space.Slug LIKE '%s'", strings.ReplaceAll(glob, "*", "%"))
	}
	return fmt.Sprintf("Space.Slug = '%s'", glob)
}

// unitData fetches a Unit's config as text (the /data endpoint returns YAML).
func unitData(spaceID, unitID string) (string, error) {
	b, err := do("GET", fmt.Sprintf("/space/%s/unit/%s/data", spaceID, unitID), nil, nil)
	return string(b), err
}

var imageLine = regexp.MustCompile(`(?m)^\s*-?\s*image:\s*["']?([^"'\s#]+)`)

// ExtractImages pulls image references out of a Kubernetes manifest's YAML.
func ExtractImages(yaml string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range imageLine.FindAllStringSubmatch(yaml, -1) {
		ref := m[1]
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out
}

// SetAnnotations writes scanner annotations onto the Deployment in a Unit via a
// server-side yq-i function invocation (POST /space/{id}/function/invoke),
// recording a change description. This is the data the no-critical-cves
// guardrail gates on.
func SetAnnotations(w Workload, annos map[string]string, changeDesc string) error {
	var expr strings.Builder
	first := true
	for k, v := range annos {
		if !first {
			expr.WriteString(" | ")
		}
		first = false
		fmt.Fprintf(&expr, `(select(.kind == "Deployment").metadata.annotations["%s"]) = "%s"`, k, v)
	}
	reqBody, _ := json.Marshal(map[string]any{
		"ChangeDescription": changeDesc,
		"FunctionInvocations": []map[string]any{{
			"FunctionName": "yq-i",
			"Arguments":    []map[string]string{{"ParameterName": "yq-expression", "Value": expr.String()}},
		}},
	})
	q := url.Values{}
	q.Set("where", fmt.Sprintf("UnitID = '%s'", w.UnitID))
	b, err := do("POST", fmt.Sprintf("/space/%s/function/invoke", w.SpaceID), q, reqBody)
	if err != nil {
		return err
	}
	var resp []struct {
		Success      bool   `json:"Success"`
		ErrorMessage string `json:"ErrorMessage"`
	}
	if json.Unmarshal(b, &resp) == nil && len(resp) > 0 && !resp[0].Success {
		return fmt.Errorf("invoke failed: %s", resp[0].ErrorMessage)
	}
	return nil
}

// UpsertUnit creates or replaces a Unit's data in the given Space — used to
// publish the AppConfig/YAML scan reports and the cvedb-status Unit. Data is the
// raw config bytes (base64-encoded for the API). Idempotent: if a Unit with the
// slug already exists it is PATCHed, otherwise created.
func UpsertUnit(spaceID, slug, toolchain, displayName string, labels map[string]string, data []byte, changeDesc string) error {
	enc := base64.StdEncoding.EncodeToString(data)

	q := url.Values{}
	q.Set("where", fmt.Sprintf("Slug = '%s' AND SpaceID = '%s'", slug, spaceID))
	q.Set("select", "Slug,UnitID")
	b, err := do("GET", "/unit", q, nil)
	if err != nil {
		return err
	}
	var existing []unitListEntry
	_ = json.Unmarshal(b, &existing)

	if len(existing) > 0 && existing[0].Unit.UnitID != "" {
		patch, _ := json.Marshal(map[string]any{"Data": enc, "LastChangeDescription": changeDesc})
		_, err := do("PATCH", fmt.Sprintf("/space/%s/unit/%s", spaceID, existing[0].Unit.UnitID), nil, patch)
		return err
	}

	create, _ := json.Marshal(map[string]any{
		"Slug":                  slug,
		"ToolchainType":         toolchain,
		"DisplayName":           displayName,
		"Labels":                labels,
		"Data":                  enc,
		"LastChangeDescription": changeDesc,
	})
	_, err = do("POST", fmt.Sprintf("/space/%s/unit", spaceID), nil, create)
	return err
}
