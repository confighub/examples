// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cub

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/confighub/sdk/core/cubapi"
	"sigs.k8s.io/yaml"
)

// RevisionDocs fetches one revision of a Unit and returns its resources decoded
// as generic documents, keyed by "<apiVersion>/<kind>/<name>" so two revisions
// can be joined resource-by-resource.
//
// The baseline for a disruption check is LastAppliedRevisionNum, not
// LiveRevisionNum: the former is set at apply time and is "what the cluster was
// last told", while the latter only advances once an apply completes. For a
// check that runs *before* apply, the former is the correct comparison point.
func RevisionDocs(ctx context.Context, c *cubapi.Client, spaceID, unitID string, revisionNum int64) (map[string]any, error) {
	rev, err := cubapi.GetRevisionByNum(ctx, c.API, spaceID, unitID, revisionNum)
	if err != nil {
		return nil, err
	}
	if rev == nil || rev.Revision == nil {
		return nil, fmt.Errorf("revision %d has no data", revisionNum)
	}
	raw, err := base64.StdEncoding.DecodeString(rev.Revision.Data)
	if err != nil {
		// Data may already be plain text depending on the transport.
		raw = []byte(rev.Revision.Data)
	}
	return decodeDocs(raw)
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
