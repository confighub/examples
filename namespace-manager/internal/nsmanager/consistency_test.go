// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package nsmanager

import "testing"

// resFull builds a FleetResource with Space + Space labels set, so consistency
// (which groups by the Component Space label) can be exercised.
func resFull(cluster, space, component string, doc map[string]any) FleetResource {
	return FleetResource{
		Origin: ResourceOrigin{
			Cluster:     cluster,
			Target:      cluster,
			Space:       space,
			SpaceLabels: map[string]string{ComponentLabel: component},
			UnitSlug:    space,
		},
		Doc: doc,
	}
}

func findComponent(cs []ComponentConsistency, comp string) *ComponentConsistency {
	for i := range cs {
		if cs[i].Component == comp {
			return &cs[i]
		}
	}
	return nil
}

func TestAnalyzeConsistency(t *testing.T) {
	psaBaseline := map[string]any{PodSecurityEnforceLabel: "baseline"}
	resources := []FleetResource{
		// component "web": consistent — namespace "web", baseline, across two variant Spaces.
		resFull("dev", "web-dev", "web", nsDoc("web", psaBaseline)),
		resFull("dev", "web-dev", "web", workloadDoc("Deployment", "web", "web")),
		resFull("prod", "web-prod", "web", nsDoc("web", psaBaseline)),
		resFull("prod", "web-prod", "web", workloadDoc("Deployment", "web", "web")),
		// component "api": namespace name differs across variants.
		resFull("dev", "api-dev", "api", nsDoc("api", psaBaseline)),
		resFull("prod", "api-prod", "api", nsDoc("api-prod", psaBaseline)),
		// component "cache": pod-security level differs across variants.
		resFull("dev", "cache-dev", "cache", nsDoc("cache", map[string]any{PodSecurityEnforceLabel: "baseline"})),
		resFull("prod", "cache-prod", "cache", nsDoc("cache", map[string]any{PodSecurityEnforceLabel: "restricted"})),
		// no Component label → skipped.
		{Origin: ResourceOrigin{Cluster: "dev", Space: "orphan", UnitSlug: "orphan"}, Doc: nsDoc("orphan", nil)},
	}
	got := AnalyzeConsistency(BuildFleet(resources))

	web := findComponent(got, "web")
	if web == nil || !web.Consistent {
		t.Fatalf("web should be consistent, got %+v", web)
	}
	if len(web.Variants) != 2 {
		t.Errorf("web variants = %d, want 2", len(web.Variants))
	}

	api := findComponent(got, "api")
	if api == nil || api.NamespaceConsistent {
		t.Fatalf("api namespace should be inconsistent, got %+v", api)
	}
	if len(api.Namespaces) != 2 {
		t.Errorf("api namespaces = %v, want 2 distinct", api.Namespaces)
	}

	cache := findComponent(got, "cache")
	if cache == nil || cache.PodSecurityConsistent {
		t.Fatalf("cache pod-security should be inconsistent, got %+v", cache)
	}

	if findComponent(got, "") != nil {
		t.Error("resources without a Component label should be skipped")
	}
}
