// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package snapshot

import (
	"github.com/confighub/sdk/core/cubapi"
	"testing"
)

// The generic reading of the fleet is cubapi.SnapshotLoader's, and tested there. What
// is this tool's own is which resource types it asks for and what it turns them
// into.

func TestLoaderCoversTheRBACModel(t *testing.T) {
	want := map[string]bool{
		"rbac.authorization.k8s.io/v1/Role":               false,
		"rbac.authorization.k8s.io/v1/ClusterRole":        false,
		"rbac.authorization.k8s.io/v1/RoleBinding":        false,
		"rbac.authorization.k8s.io/v1/ClusterRoleBinding": false,
		"v1/ServiceAccount":                               false,
	}
	for _, got := range loader.ResourceTypes {
		if _, ok := want[got]; !ok {
			t.Errorf("unexpected resource type %q", got)
			continue
		}
		want[got] = true
	}
	for rt, seen := range want {
		if !seen {
			t.Errorf("%s is analyzed but never fetched", rt)
		}
	}
}

// A field dropped from the mapping is invisible: the resource still parses, and
// the analyzers just see an empty cluster or space.
func TestLoaderNewCarriesTheWholeOrigin(t *testing.T) {
	origin := cubapi.Origin{
		Cluster:      "prod-oci",
		Target:       "prod-target",
		Space:        "prod-rbac",
		SpaceID:      "11111111-1111-1111-1111-111111111111",
		UnitID:       "22222222-2222-2222-2222-222222222222",
		UnitSlug:     "argocd-rbac",
		ResourceName: "argocd/viewer",
		ResourceType: "rbac.authorization.k8s.io/v1/Role",
		Canonical:    true,
	}
	doc := map[string]any{"kind": "Role"}
	got := loader.New(origin, doc)

	for _, tc := range []struct{ field, got, want string }{
		{"Cluster", got.Origin.Cluster, origin.Cluster},
		{"Target", got.Origin.Target, origin.Target},
		{"Space", got.Origin.Space, origin.Space},
		{"SpaceID", got.Origin.SpaceID, origin.SpaceID},
		{"UnitID", got.Origin.UnitID, origin.UnitID},
		{"UnitSlug", got.Origin.UnitSlug, origin.UnitSlug},
		{"ResourceName", got.Origin.ResourceName, origin.ResourceName},
	} {
		if tc.got != tc.want {
			t.Errorf("Origin.%s = %q, want %q", tc.field, tc.got, tc.want)
		}
	}
	if !got.Origin.Canonical {
		t.Error("Origin.Canonical was not carried through")
	}
	if got.Doc == nil {
		t.Error("Doc was not carried through")
	}
}
