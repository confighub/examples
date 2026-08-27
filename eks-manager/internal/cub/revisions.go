// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cub

import (
	"context"
	"fmt"
	"net/http"

	"github.com/confighub/sdk/core/cubapi"
	"github.com/google/uuid"
	"sigs.k8s.io/yaml"
)

// RevisionDocs fetches one revision of a Unit and returns its resources decoded
// as generic documents, keyed by "<apiVersion>/<kind>/<name>" so two revisions
// can be joined resource-by-resource.
//
// The baseline for a disruption check is LastReleasedRevisionNum: publishing a
// Release advances it, so it is "what the cluster was last told". A check that
// runs *before* the next publish wants exactly that comparison point.
func RevisionDocs(ctx context.Context, c *cubapi.Client, spaceID, unitID string, revisionNum int64) (map[string]any, error) {
	rev, err := cubapi.GetRevisionByNum(ctx, c.API, spaceID, unitID, revisionNum)
	if err != nil {
		return nil, err
	}
	if rev == nil || rev.Revision == nil {
		return nil, fmt.Errorf("revision %d has no data", revisionNum)
	}
	sid, err := uuid.Parse(spaceID)
	if err != nil {
		return nil, fmt.Errorf("parse space id %q: %w", spaceID, err)
	}
	uid, err := uuid.Parse(unitID)
	if err != nil {
		return nil, fmt.Errorf("parse unit id %q: %w", unitID, err)
	}
	// Configuration is not a field of a Revision: it is read through the Revision's
	// own data endpoint, as text.
	res, dlErr := c.API.DownloadRevisionDataWithResponse(ctx, sid, uid, rev.Revision.RevisionID)
	if dlErr != nil {
		return nil, dlErr
	}
	if res == nil {
		return nil, fmt.Errorf("get data for revision %d: no response from server", revisionNum)
	}
	if res.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("get data for revision %d: %s", revisionNum, res.Status())
	}
	return decodeDocs(res.Body)
}

// decodeDocs splits a multi-document YAML payload into resources keyed by
// identity. Malformed documents are skipped rather than failing the whole Unit —
// one bad resource must not blind the classifier to the rest.
func decodeDocs(raw []byte) (map[string]any, error) {
	out := map[string]any{}
	for _, chunk := range splitYAMLDocs(raw) {
		var doc any
		if err := yaml.Unmarshal(chunk, &doc); err != nil {
			continue
		}
		rec, ok := doc.(map[string]any)
		if !ok {
			continue
		}
		apiVersion, _ := rec["apiVersion"].(string)
		kind, _ := rec["kind"].(string)
		name := ""
		if md, ok := rec["metadata"].(map[string]any); ok {
			name, _ = md["name"].(string)
		}
		if kind == "" || name == "" {
			continue
		}
		out[apiVersion+"/"+kind+"/"+name] = doc
	}
	return out, nil
}

// splitYAMLDocs splits on document separators at the start of a line.
func splitYAMLDocs(raw []byte) [][]byte {
	var docs [][]byte
	start := 0
	lines := 0
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\n' && i != len(raw)-1 {
			continue
		}
		lines++
		_ = lines
		// Look for a line that is exactly "---".
		lineStart := start
		for j := i; j >= start; j-- {
			if raw[j] == '\n' {
				lineStart = j + 1
				break
			}
		}
		line := raw[lineStart : i+1]
		if isSeparator(line) {
			if lineStart > start {
				docs = append(docs, raw[start:lineStart])
			}
			start = i + 1
		}
	}
	if start < len(raw) {
		docs = append(docs, raw[start:])
	}
	if len(docs) == 0 {
		docs = append(docs, raw)
	}
	return docs
}

func isSeparator(line []byte) bool {
	s := string(line)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s == "---"
}
