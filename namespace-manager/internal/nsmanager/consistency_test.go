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
		// component "api": its server runs in a different namespace in prod.
		resFull("dev", "api-dev", "api", nsDoc("api", psaBaseline)),
		resFull("dev", "api-dev", "api", workloadDoc("Deployment", "server", "api")),
		resFull("prod", "api-prod", "api", nsDoc("api-prod", psaBaseline)),
		resFull("prod", "api-prod", "api", workloadDoc("Deployment", "server", "api-prod")),
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

// A workload's namespace is compared only across clusters: a component may span
// several namespaces, a variant may run parts another does not, and variants
// sharing a cluster must use different namespaces.
func TestAnalyzeConsistencyComparesWorkloadsAcrossClusters(t *testing.T) {
	resources := []FleetResource{
		// platform spans several namespaces, and prod runs a part dev does not.
		resFull("dev", "platform-dev", "platform", workloadDoc("Deployment", "cert-manager", "cert-manager")),
		resFull("dev", "platform-dev", "platform", workloadDoc("Deployment", "traefik", "traefik")),
		resFull("prod", "platform-prod", "platform", workloadDoc("Deployment", "cert-manager", "cert-manager")),
		resFull("prod", "platform-prod", "platform", workloadDoc("Deployment", "traefik", "traefik")),
		resFull("prod", "platform-prod", "platform", workloadDoc("Deployment", "ack-s3", "ack-system")),
		// app's staging and prod share a cluster, so their namespaces must differ.
		resFull("prod", "app-prod", "app", workloadDoc("Deployment", "server", "app")),
		resFull("prod", "app-staging", "app", workloadDoc("Deployment", "server", "app-staging")),
		// preview has one variant per pull request on one cluster, and a base
		// whose namespace is still a placeholder.
		resFull("dev", "preview-pr-1", "preview", workloadDoc("Deployment", "ui", "pr-1")),
		resFull("dev", "preview-pr-2", "preview", workloadDoc("Deployment", "ui", "pr-2")),
		resFull("None", "preview-base", "preview", workloadDoc("Deployment", "ui", PlaceholderNamespace)),
		// site moves its server to another namespace in prod.
		resFull("dev", "site-dev", "site", workloadDoc("Deployment", "server", "site")),
		resFull("dev", "site-dev", "site", workloadDoc("Deployment", "worker", "site")),
		resFull("prod", "site-prod", "site", workloadDoc("Deployment", "server", "site-prod")),
		resFull("prod", "site-prod", "site", workloadDoc("Deployment", "worker", "site")),
	}
	got := AnalyzeConsistency(BuildFleet(resources))

	for _, comp := range []string{"platform", "app", "preview"} {
		if cc := findComponent(got, comp); cc == nil || !cc.NamespaceConsistent {
			t.Errorf("%s should be namespace-consistent, got %+v", comp, cc)
		}
	}
	site := findComponent(got, "site")
	if site == nil || site.NamespaceConsistent {
		t.Fatalf("site should be namespace-inconsistent, got %+v", site)
	}
	if len(site.NamespaceMismatches) != 1 || site.NamespaceMismatches[0].Workload != "Deployment/server" {
		t.Errorf("site mismatches = %+v, want only Deployment/server", site.NamespaceMismatches)
	}
	want := `workloads run in different namespaces on different clusters: Deployment/server is in "site" in site-dev, "site-prod" in site-prod`
	if len(site.Issues) != 1 || site.Issues[0] != want {
		t.Errorf("site issues = %q, want [%q]", site.Issues, want)
	}
}
