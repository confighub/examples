// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/confighub/sdk/cliutil"
	"github.com/confighub/sdk/core/cubapi"
)

// commitChange converts the shared dry-run/commit flags (cliutil.CommitFlags)
// into a cubapi.Change: empty on dry-run, the change description on commit. It
// enforces that a commit carries a description (summary is suggested in the error).
func commitChange(c cliutil.CommitFlags, summary string) (cubapi.Change, error) {
	desc, _, err := c.Validate(summary)
	if err != nil {
		return cubapi.Change{}, err
	}
	return cubapi.Change{Description: desc}, nil
}

// changedUnits returns the "space/unit" labels of the Units an invocation
// actually mutated. It returns an error if any Unit failed, surfacing the first
// failure with its message.
func changedUnits(res *cubapi.Result) ([]string, error) {
	if failed := res.Failed(); len(failed) > 0 {
		f := failed[0]
		ref := strings.Trim(f.SpaceSlug+"/"+f.UnitSlug, "/")
		if ref == "" {
			ref = "unit"
		}
		return nil, fmt.Errorf("invocation failed on %s: %s", ref, f.Error)
	}
	var changed []string
	for _, o := range res.Outcomes {
		if o.Success && o.HasMutations {
			changed = append(changed, o.SpaceSlug+"/"+o.UnitSlug)
		}
	}
	sort.Strings(changed)
	return changed, nil
}

// editParams turns the edit Invocation's "name=value" parameter strings into the
// parameter map a stored Invocation expects.
func editParams(params []string) map[string]any {
	m := make(map[string]any, len(params))
	for _, p := range params {
		if k, v, ok := strings.Cut(p, "="); ok {
			m[k] = v
		}
	}
	return m
}

func parseUnitRef(ref string) (space, unit string, err error) {
	space, unit, ok := strings.Cut(ref, "/")
	if !ok || space == "" || unit == "" {
		return "", "", fmt.Errorf("invalid unit reference %q: expected <space>/<unit>", ref)
	}
	return space, unit, nil
}
