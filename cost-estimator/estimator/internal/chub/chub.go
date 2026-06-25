// Package chub talks to the ConfigHub REST API directly — the same surface the
// cost-estimator web app uses — to read workload manifests from the fleet and
// write cost estimates back as data. No `cub` CLI shell-out.
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
	"strings"
	"time"
)

// Unit is one fleet Unit: its identity, raw config YAML, and ConfigHub labels
// (Environment / Region drive the cost model).
type Unit struct {
	Space   string
	SpaceID string
	Unit    string
	UnitID  string
	YAML    string
	Labels  map[string]string
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
// expression) and returns each one's config YAML + labels. Callers parse the
// YAML and skip Units that aren't workloads.
//
// cub's familiar "cost-demo-*" glob has no API equivalent, so a partial glob is
// translated to a Space.Slug LIKE filter and an exact slug to Space.Slug =.
func Inventory(spaceGlob, where string) ([]Unit, error) {
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

	var units []Unit
	for _, e := range entries {
		data, err := unitData(e.Unit.SpaceID, e.Unit.UnitID)
		if err != nil {
			return nil, err
		}
		units = append(units, Unit{
			Space: e.Space.Slug, SpaceID: e.Unit.SpaceID,
			Unit: e.Unit.Slug, UnitID: e.Unit.UnitID,
			YAML: data, Labels: e.Unit.Labels,
		})
	}
	return units, nil
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

// SetAnnotations writes estimator annotations onto the workload (Deployment or
// StatefulSet) in a Unit via a server-side yq-i function invocation, recording a
// change description. This is the data the within-budget guardrail gates on.
func SetAnnotations(u Unit, annos map[string]string, changeDesc string) error {
	var expr strings.Builder
	first := true
	for k, v := range annos {
		if !first {
			expr.WriteString(" | ")
		}
		first = false
		fmt.Fprintf(&expr,
			`(select(.kind == "Deployment" or .kind == "StatefulSet").metadata.annotations["%s"]) = "%s"`, k, v)
	}
	reqBody, _ := json.Marshal(map[string]any{
		"ChangeDescription": changeDesc,
		"FunctionInvocations": []map[string]any{{
			"FunctionName": "yq-i",
			"Arguments":    []map[string]string{{"ParameterName": "yq-expression", "Value": expr.String()}},
		}},
	})
	q := url.Values{}
	q.Set("where", fmt.Sprintf("UnitID = '%s'", u.UnitID))
	b, err := do("POST", fmt.Sprintf("/space/%s/function/invoke", u.SpaceID), q, reqBody)
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
// publish the AppConfig/YAML cost record and pricebook-status Units. Idempotent:
// if a Unit with the slug already exists it is PATCHed, otherwise created.
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
