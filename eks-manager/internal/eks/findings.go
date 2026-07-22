// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package eks

import (
	"fmt"
	"sort"
	"strings"
)

// Severity ranks a finding for triage. It mirrors ConfigHub's Score vocabulary
// so findings and disruption gradings speak the same language.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

func severityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 3
	case SeverityHigh:
		return 2
	case SeverityMedium:
		return 1
	}
	return 0
}

// ValidSeverity reports whether s names a severity.
func ValidSeverity(s string) bool {
	switch Severity(s) {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow:
		return true
	}
	return false
}

// Analyzer names. The analyzer is a finding's identity — it is what --analyzer
// filters on — so these are stable kebab-case rule names.
const (
	AnalyzerAutoscalerConflict = "autoscaler-conflict"
	AnalyzerVersionSkew        = "version-skew"
	AnalyzerSupportPolicy      = "support-policy"
	AnalyzerAutoModeInvariant  = "automode-invariant"
	AnalyzerExposure           = "exposure"
	AnalyzerDanglingRef        = "dangling-ref"
	AnalyzerInconsistent       = "inconsistent"
	AnalyzerBootstrapAddons    = "bootstrap-addons"
)

// AllAnalyzers lists every analyzer, for --help and validation.
var AllAnalyzers = []string{
	AnalyzerAutoscalerConflict, AnalyzerVersionSkew, AnalyzerSupportPolicy,
	AnalyzerAutoModeInvariant, AnalyzerExposure, AnalyzerDanglingRef,
	AnalyzerInconsistent, AnalyzerBootstrapAddons,
}

// Finding is one ranked issue on one resource, with enough origin to deep-link
// and to fix the offending Unit.
type Finding struct {
	Severity Severity `json:"severity"`
	Analyzer string   `json:"analyzer"`
	Cluster  string   `json:"cluster"`
	Kind     string   `json:"kind"`
	Name     string   `json:"name"`
	Space    string   `json:"space,omitempty"`
	UnitSlug string   `json:"unitSlug,omitempty"`
	Message  string   `json:"message"`
	Fix      string   `json:"fix,omitempty"`
}

// Findings analyzes a fleet and returns severity-ranked findings, worst first.
func Findings(clusters map[string]*ClusterSet) []Finding {
	var out []Finding
	for _, cs := range clusters {
		out = append(out, analyzeCluster(cs)...)
	}
	out = append(out, analyzeConsistency(clusters)...)

	sort.Slice(out, func(i, j int) bool {
		if severityRank(out[i].Severity) != severityRank(out[j].Severity) {
			return severityRank(out[i].Severity) > severityRank(out[j].Severity)
		}
		if out[i].Cluster != out[j].Cluster {
			return out[i].Cluster < out[j].Cluster
		}
		if out[i].Analyzer != out[j].Analyzer {
			return out[i].Analyzer < out[j].Analyzer
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func analyzeCluster(cs *ClusterSet) []Finding {
	var out []Finding
	c := cs.Control

	if c != nil {
		origin := func(f *Finding) {
			f.Cluster = cs.Cluster
			f.Kind = "Cluster"
			f.Name = c.Name
			f.Space = c.Origin.Space
			f.UnitSlug = c.Origin.UnitSlug
		}

		// The Auto Mode tuple must be complete and agree, or AWS rejects the
		// update outright and the managed resource never syncs.
		if !c.AutoMode.Consistent() {
			f := Finding{
				Severity: SeverityHigh, Analyzer: AnalyzerAutoModeInvariant,
				Message: "Auto Mode toggles disagree or are partially specified: computeConfig.enabled, " +
					"kubernetesNetworkConfig.elasticLoadBalancing.enabled and storageConfig.blockStorage.enabled " +
					"must all be present and all agree",
				Fix: "set all three toggles to the same value in one change",
			}
			origin(&f)
			out = append(out, f)
		}

		// bootstrapSelfManagedAddons is ForceNew and Auto Mode requires it false,
		// so leaving it true forecloses Auto Mode for the cluster's whole life.
		if c.AutoMode.Enabled() && c.BootstrapSelfManagedAddons != nil && *c.BootstrapSelfManagedAddons {
			f := Finding{
				Severity: SeverityHigh, Analyzer: AnalyzerBootstrapAddons,
				Message: "Auto Mode is enabled but bootstrapSelfManagedAddons is true; the field is immutable, " +
					"so this combination cannot be reconciled",
				Fix: "the cluster must be recreated with bootstrapSelfManagedAddons: false",
			}
			origin(&f)
			out = append(out, f)
		}

		// A pinned version under STANDARD support is auto-upgraded by AWS at end
		// of standard support, which then fights the pin forever.
		if c.Version != "" && strings.EqualFold(c.UpgradeSupportType, "STANDARD") {
			f := Finding{
				Severity: SeverityMedium, Analyzer: AnalyzerSupportPolicy,
				Message: fmt.Sprintf("version is pinned to %s but upgradePolicy.supportType is STANDARD; "+
					"AWS will auto-upgrade at end of standard support and drift from the pin", c.Version),
				Fix: "set upgradePolicy.supportType: EXTENDED, or stop pinning the version",
			}
			origin(&f)
			out = append(out, f)
		}

		// Prod posture.
		if isTrue(c.EndpointPublicAccess) {
			sev, extra := SeverityMedium, ""
			for _, cidr := range c.PublicAccessCIDRs {
				if cidr == "0.0.0.0/0" {
					sev, extra = SeverityHigh, " with publicAccessCidrs 0.0.0.0/0"
				}
			}
			if len(c.PublicAccessCIDRs) == 0 {
				sev, extra = SeverityHigh, " with no publicAccessCidrs restriction (defaults to 0.0.0.0/0)"
			}
			f := Finding{
				Severity: sev, Analyzer: AnalyzerExposure,
				Message: "the Kubernetes API endpoint is publicly accessible" + extra,
				Fix:     "set vpcConfig.endpointPublicAccess: false, or restrict publicAccessCidrs",
			}
			origin(&f)
			out = append(out, f)
		}
		if !c.EncryptionConfigured {
			f := Finding{
				Severity: SeverityMedium, Analyzer: AnalyzerExposure,
				Message: "no encryptionConfig: Kubernetes Secrets are not encrypted with a KMS key",
				Fix:     "add encryptionConfig with a KMS key ARN (note: removing it later replaces the cluster)",
			}
			origin(&f)
			out = append(out, f)
		}
		if missing := missingLogTypes(c.LogTypes); len(missing) > 0 {
			f := Finding{
				Severity: SeverityLow, Analyzer: AnalyzerExposure,
				Message: "control-plane logging is missing: " + strings.Join(missing, ", "),
				Fix:     "add the missing types to enabledClusterLogTypes",
			}
			origin(&f)
			out = append(out, f)
		}
		if c.DeletionProtection != nil && !*c.DeletionProtection {
			f := Finding{
				Severity: SeverityLow, Analyzer: AnalyzerExposure,
				Message: "deletionProtection is false",
				Fix:     "set deletionProtection: true, and add a ConfigHub Destroy Gate on the Unit",
			}
			origin(&f)
			out = append(out, f)
		}
	}

	cpVersion, cpOK := Version{}, false
	if c != nil {
		cpVersion, cpOK = ParseVersion(c.Version)
	}

	for _, n := range cs.NodeGroups {
		origin := func(f *Finding) {
			f.Cluster = cs.Cluster
			f.Kind = "NodeGroup"
			f.Name = n.Name
			f.Space = n.Origin.Space
			f.UnitSlug = n.Origin.UnitSlug
		}

		// The infrastructure analog of a PodDisruptionBudget fighting an HPA:
		// a desiredSize in forProvider is reconciled on every pass, so it fights
		// any external autoscaler. The documented fix (spec.initProvider) is
		// currently broken for the object-shaped API versions, so the honest
		// recommendation is Auto Mode.
		if n.DesiredSize != nil && !n.DesiredSizeInInitProvider && managesUpdates(n.ManagementPolicies) {
			f := Finding{
				Severity: SeverityMedium, Analyzer: AnalyzerAutoscalerConflict,
				Message: fmt.Sprintf("scalingConfig.desiredSize (%d) is set under forProvider with Update "+
					"management: Crossplane will reconcile it on every pass, fighting Cluster Autoscaler or Karpenter",
					*n.DesiredSize),
				Fix: "prefer EKS Auto Mode (no node group at all); moving desiredSize to spec.initProvider is the " +
					"documented fix but does not currently work on the object-shaped API versions",
			}
			origin(&f)
			out = append(out, f)
		}

		// LateInitialize copies the observed value back into spec, which defeats
		// every workaround for the above.
		for _, p := range n.ManagementPolicies {
			if p == "LateInitialize" || p == "*" {
				f := Finding{
					Severity: SeverityMedium, Analyzer: AnalyzerAutoscalerConflict,
					Message: "managementPolicies includes LateInitialize, which copies observed values into " +
						"spec and defeats any attempt to leave desiredSize externally managed",
					Fix: "list the policies explicitly without LateInitialize",
				}
				origin(&f)
				out = append(out, f)
				break
			}
		}

		// desiredSize must sit inside the min/max window.
		if n.DesiredSize != nil {
			if n.MinSize != nil && *n.DesiredSize < *n.MinSize {
				f := Finding{
					Severity: SeverityHigh, Analyzer: AnalyzerAutoscalerConflict,
					Message: fmt.Sprintf("desiredSize %d is below minSize %d", *n.DesiredSize, *n.MinSize),
					Fix:     "raise desiredSize or lower minSize",
				}
				origin(&f)
				out = append(out, f)
			}
			if n.MaxSize != nil && *n.DesiredSize > *n.MaxSize {
				f := Finding{
					Severity: SeverityHigh, Analyzer: AnalyzerAutoscalerConflict,
					Message: fmt.Sprintf("desiredSize %d is above maxSize %d", *n.DesiredSize, *n.MaxSize),
					Fix:     "lower desiredSize or raise maxSize",
				}
				origin(&f)
				out = append(out, f)
			}
		}

		// Version skew against the control plane.
		if cpOK && n.Version != "" {
			if v, ok := ParseVersion(n.Version); ok {
				if skew := MinorSkew(cpVersion, v); skew > 0 {
					sev := SeverityLow
					if skew >= 3 {
						sev = SeverityHigh
					} else if skew == 2 {
						sev = SeverityMedium
					}
					f := Finding{
						Severity: sev, Analyzer: AnalyzerVersionSkew,
						Message: fmt.Sprintf("node group is on %s, %d minor version(s) behind the control plane (%s)",
							n.Version, skew, c.Version),
						Fix: "upgrade the node group one minor at a time (a version change rolls every node)",
					}
					origin(&f)
					out = append(out, f)
				}
				if v.Compare(cpVersion) > 0 {
					f := Finding{
						Severity: SeverityHigh, Analyzer: AnalyzerVersionSkew,
						Message: fmt.Sprintf("node group is on %s, ahead of the control plane (%s); "+
							"EKS does not support nodes newer than the control plane", n.Version, c.Version),
						Fix: "upgrade the control plane first, or pin the node group back",
					}
					origin(&f)
					out = append(out, f)
				}
			}
		}

		// A node group whose cluster reference does not resolve to a Cluster in
		// scope will block at Synced=False forever, invisibly.
		if n.ClusterName != "" && c != nil && n.ClusterName != c.Name {
			f := Finding{
				Severity: SeverityHigh, Analyzer: AnalyzerDanglingRef,
				Message: fmt.Sprintf("references cluster %q, but the Cluster in this Space is %q",
					n.ClusterName, c.Name),
				Fix: "correct clusterNameRef; an unresolvable reference blocks reconciliation indefinitely",
			}
			origin(&f)
			out = append(out, f)
		}
	}

	for _, a := range cs.Addons {
		if a.ClusterName != "" && c != nil && a.ClusterName != c.Name {
			out = append(out, Finding{
				Severity: SeverityHigh, Analyzer: AnalyzerDanglingRef,
				Cluster: cs.Cluster, Kind: "Addon", Name: a.Name,
				Space: a.Origin.Space, UnitSlug: a.Origin.UnitSlug,
				Message: fmt.Sprintf("references cluster %q, but the Cluster in this Space is %q",
					a.ClusterName, c.Name),
				Fix: "correct clusterNameRef; an unresolvable reference blocks reconciliation indefinitely",
			})
		}
	}
	return out
}

// analyzeConsistency is the cross-cluster reduce: clusters that share an
// Environment label should not diverge in version or addon set.
func analyzeConsistency(clusters map[string]*ClusterSet) []Finding {
	byEnv := map[string][]*ClusterSet{}
	for _, cs := range clusters {
		if cs.Control == nil {
			continue
		}
		env := cs.Control.Origin.SpaceLabels["Environment"]
		if env == "" {
			continue
		}
		byEnv[env] = append(byEnv[env], cs)
	}

	var out []Finding
	for env, group := range byEnv {
		if len(group) < 2 {
			continue
		}
		versions := map[string][]string{}
		for _, cs := range group {
			versions[cs.Control.Version] = append(versions[cs.Control.Version], cs.Cluster)
		}
		if len(versions) > 1 {
			var parts []string
			for v, names := range versions {
				sort.Strings(names)
				parts = append(parts, fmt.Sprintf("%s: %s", dashIfEmpty(v), strings.Join(names, ",")))
			}
			sort.Strings(parts)
			for _, cs := range group {
				out = append(out, Finding{
					Severity: SeverityMedium, Analyzer: AnalyzerInconsistent,
					Cluster: cs.Cluster, Kind: "Cluster", Name: cs.Control.Name,
					Space: cs.Control.Origin.Space, UnitSlug: cs.Control.Origin.UnitSlug,
					Message: fmt.Sprintf("clusters in environment %q run different Kubernetes versions (%s)",
						env, strings.Join(parts, "; ")),
					Fix: "promote the intended version across the environment",
				})
			}
		}
	}
	return out
}

func managesUpdates(policies []string) bool {
	if len(policies) == 0 {
		return true // the default is full management
	}
	for _, p := range policies {
		if p == "Update" || p == "*" {
			return true
		}
	}
	return false
}

func missingLogTypes(have []string) []string {
	want := []string{"api", "audit", "authenticator"}
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	var missing []string
	for _, w := range want {
		if !set[w] {
			missing = append(missing, w)
		}
	}
	return missing
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}
