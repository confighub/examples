// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package eks

import (
	"fmt"
	"sort"
	"strings"
)

// Disruption grades what applying a change will actually do to the running
// infrastructure.
//
// The grading exists because of a specific Crossplane behavior. Terraform
// destroys and recreates a resource when a ForceNew field changes; Crossplane
// refuses — upjet's assertNoForceNew returns "refuse to update the external
// resource because the following update requires replacing it", the managed
// resource goes Synced=False, and it is retried forever while AWS is never
// touched. So under a source of record the Unit is edited, a revision is
// committed with a clean diff, the apply succeeds, and *nothing happens,
// permanently*.
//
// That means DisruptionReplace and DisruptionReplaceCluster are not "risky
// changes" — they are changes that cannot be applied at all in place, and whose
// only symptom is a condition message on a managed resource in a management
// cluster nobody is watching.
type Disruption string

const (
	// DisruptionNone is an ordinary in-place update.
	DisruptionNone Disruption = ""
	// DisruptionLow is in-place but service-affecting.
	DisruptionLow Disruption = "in-place-disruptive"
	// DisruptionRolling is in-place, but EKS rolls every node in the group.
	DisruptionRolling Disruption = "rolling"
	// DisruptionReplace means the resource must be destroyed and recreated.
	// Crossplane will not do this; the edit wedges instead.
	//
	// Note this generalizes the design's "replace-nodegroup": Addons, subnets,
	// and IAM roles have exactly the same failure mode, so the tier is named for
	// the disruption rather than for one resource kind. The finding names which
	// resource is affected.
	DisruptionReplace Disruption = "replace"
	// DisruptionReplaceCluster means the EKS control plane itself must be
	// replaced — a new cluster, and everything running on it.
	DisruptionReplaceCluster Disruption = "replace-cluster"
)

// Score maps a disruption tier onto ConfigHub's validation Score vocabulary, so
// a vet-disruption function can gate on it with a score-threshold and the
// Trigger's Warn field decides gate-versus-warning.
func (d Disruption) Score() string {
	switch d {
	case DisruptionReplaceCluster:
		return "Critical"
	case DisruptionReplace:
		return "High"
	case DisruptionRolling:
		return "Medium"
	case DisruptionLow:
		return "Low"
	}
	return ""
}

// Rank orders tiers for aggregation. Higher is worse.
func (d Disruption) Rank() int {
	switch d {
	case DisruptionReplaceCluster:
		return 4
	case DisruptionReplace:
		return 3
	case DisruptionRolling:
		return 2
	case DisruptionLow:
		return 1
	}
	return 0
}

// Blocks reports whether the change cannot be reconciled in place at all, and
// will therefore wedge rather than apply.
func (d Disruption) Blocks() bool {
	return d == DisruptionReplace || d == DisruptionReplaceCluster
}

// MaxDisruption returns the worse of two tiers.
func MaxDisruption(a, b Disruption) Disruption {
	if b.Rank() > a.Rank() {
		return b
	}
	return a
}

// disruptionRule maps a config path prefix to what changing it does.
type disruptionRule struct {
	path   string
	level  Disruption
	reason string
}

// disruptionTable is keyed by "<group>/<Kind>" — deliberately without the
// version, so the same rules apply to the object-shaped v1beta2 and the
// deprecated list-shaped v1beta1.
//
// Paths are prefixes matched against the changed path, with list indices
// normalized away (see normalizePath), so "spec.forProvider.scalingConfig" also
// covers "spec.forProvider.scalingConfig.0.desiredSize".
//
// Derived from ForceNew markers in hashicorp/terraform-provider-aws, which is
// what upjet's diff — and therefore assertNoForceNew — actually consults.
var disruptionTable = map[string][]disruptionRule{
	"eks.aws.upbound.io/Cluster": {
		{"metadata.name", DisruptionReplaceCluster, "the cluster name is its identity in AWS"},
		{"spec.forProvider.roleArn", DisruptionReplaceCluster, "the cluster IAM role is immutable"},
		{"spec.forProvider.bootstrapSelfManagedAddons", DisruptionReplaceCluster, "immutable; and Auto Mode requires it false, so it is a creation-time decision"},
		{"spec.forProvider.accessConfig.bootstrapClusterCreatorAdminPermissions", DisruptionReplaceCluster, "immutable"},
		{"spec.forProvider.kubernetesNetworkConfig.ipFamily", DisruptionReplaceCluster, "the cluster IP family is immutable"},
		{"spec.forProvider.kubernetesNetworkConfig.serviceIpv4Cidr", DisruptionReplaceCluster, "the service CIDR is immutable"},
		{"spec.forProvider.outpostConfig", DisruptionReplaceCluster, "Outpost placement is immutable"},
		{"spec.forProvider.encryptionConfig", DisruptionReplaceCluster, "removing encryptionConfig replaces the cluster (adding it is in-place)"},
		// In-place, but a control-plane update is a multi-minute operation and
		// EKS permits only one at a time.
		{"spec.forProvider.version", DisruptionLow, "control-plane upgrade: multi-minute, and EKS allows one cluster update at a time"},
		{"spec.forProvider.computeConfig", DisruptionLow, "changes Auto Mode compute; all three Auto Mode toggles must be sent together"},
		{"spec.forProvider.storageConfig", DisruptionLow, "changes Auto Mode storage; all three Auto Mode toggles must be sent together"},
		{"spec.forProvider.kubernetesNetworkConfig.elasticLoadBalancing", DisruptionLow, "changes Auto Mode load balancing; all three Auto Mode toggles must be sent together"},
	},
	"eks.aws.upbound.io/NodeGroup": {
		{"metadata.name", DisruptionReplace, "the node group name is its identity in AWS"},
		{"spec.forProvider.amiType", DisruptionReplace, "amiType is immutable"},
		{"spec.forProvider.capacityType", DisruptionReplace, "capacityType is immutable"},
		{"spec.forProvider.instanceTypes", DisruptionReplace, "instanceTypes is immutable"},
		{"spec.forProvider.diskSize", DisruptionReplace, "diskSize is immutable (use a launch template to change it in place)"},
		{"spec.forProvider.clusterName", DisruptionReplace, "a node group cannot move between clusters"},
		{"spec.forProvider.nodeRoleArn", DisruptionReplace, "the node IAM role is immutable"},
		{"spec.forProvider.subnetId", DisruptionReplace, "node group subnets are immutable"},
		{"spec.forProvider.remoteAccess", DisruptionReplace, "remoteAccess is immutable"},
		{"spec.forProvider.launchTemplate.id", DisruptionReplace, "switching launch template is immutable (bump launchTemplate.version instead)"},
		{"spec.forProvider.launchTemplate.name", DisruptionReplace, "switching launch template is immutable (bump launchTemplate.version instead)"},
		// In-place, but EKS drains and replaces every node in the group.
		{"spec.forProvider.version", DisruptionRolling, "EKS rolls every node in the group"},
		{"spec.forProvider.releaseVersion", DisruptionRolling, "EKS rolls every node in the group"},
		{"spec.forProvider.launchTemplate.version", DisruptionRolling, "EKS rolls every node in the group"},
		{"spec.forProvider.forceUpdateVersion", DisruptionRolling, "forces the roll past PodDisruptionBudgets"},
	},
	"eks.aws.upbound.io/Addon": {
		{"metadata.name", DisruptionReplace, "the addon name is its identity"},
		{"spec.forProvider.addonName", DisruptionReplace, "addonName is immutable"},
		{"spec.forProvider.clusterName", DisruptionReplace, "an addon cannot move between clusters"},
		{"spec.forProvider.namespaceConfig", DisruptionReplace, "namespaceConfig is immutable"},
		{"spec.forProvider.addonVersion", DisruptionLow, "addon upgrade restarts the addon's pods"},
		{"spec.forProvider.configurationValues", DisruptionLow, "reconfigures the addon in place"},
	},
	"eks.aws.upbound.io/FargateProfile": {
		{"metadata.name", DisruptionReplace, "Fargate profiles are immutable once created"},
		{"spec.forProvider", DisruptionReplace, "Fargate profiles are immutable once created"},
	},
	"eks.aws.upbound.io/PodIdentityAssociation": {
		{"spec.forProvider.clusterName", DisruptionReplace, "immutable"},
		{"spec.forProvider.namespace", DisruptionReplace, "immutable"},
		{"spec.forProvider.serviceAccount", DisruptionReplace, "immutable"},
	},
	// Networking. Replacing any of these detaches or recreates infrastructure
	// the cluster is actively using.
	"ec2.aws.upbound.io/VPC": {
		{"spec.forProvider.cidrBlock", DisruptionReplace, "the primary VPC CIDR is immutable"},
		{"spec.forProvider.instanceTenancy", DisruptionReplace, "instanceTenancy is immutable"},
	},
	"ec2.aws.upbound.io/Subnet": {
		{"spec.forProvider.cidrBlock", DisruptionReplace, "subnet CIDR is immutable"},
		{"spec.forProvider.availabilityZone", DisruptionReplace, "a subnet cannot move between availability zones"},
		{"spec.forProvider.vpcId", DisruptionReplace, "a subnet cannot move between VPCs"},
	},
	"ec2.aws.upbound.io/NATGateway": {
		{"spec.forProvider.subnetId", DisruptionReplace, "immutable"},
		{"spec.forProvider.allocationId", DisruptionReplace, "immutable"},
		{"spec.forProvider.connectivityType", DisruptionReplace, "immutable"},
	},
	"ec2.aws.upbound.io/InternetGateway": {
		{"spec.forProvider.vpcId", DisruptionReplace, "immutable"},
	},
	"ec2.aws.upbound.io/RouteTable": {
		{"spec.forProvider.vpcId", DisruptionReplace, "immutable"},
	},
	"ec2.aws.upbound.io/Route": {
		{"spec.forProvider.routeTableId", DisruptionReplace, "immutable"},
		{"spec.forProvider.destinationCidrBlock", DisruptionReplace, "immutable"},
	},
	"ec2.aws.upbound.io/RouteTableAssociation": {
		{"spec.forProvider.subnetId", DisruptionReplace, "immutable"},
		{"spec.forProvider.routeTableId", DisruptionReplace, "immutable"},
	},
	"ec2.aws.upbound.io/EIP": {
		{"spec.forProvider.domain", DisruptionReplace, "immutable"},
	},
	"iam.aws.upbound.io/Role": {
		{"metadata.name", DisruptionReplace, "the role name is its identity in AWS"},
		{"spec.forProvider.namePrefix", DisruptionReplace, "immutable"},
	},
	"iam.aws.upbound.io/RolePolicyAttachment": {
		{"spec.forProvider.policyArn", DisruptionReplace, "an attachment is recreated to point at a different policy"},
		{"spec.forProvider.roleArn", DisruptionReplace, "an attachment is recreated to point at a different role"},
	},
}

// Classification is the disruption verdict for one changed path.
type Classification struct {
	Path       string     `json:"path"`
	Disruption Disruption `json:"disruption"`
	Reason     string     `json:"reason,omitempty"`
	Score      string     `json:"score,omitempty"`
}

// ClassifyPath grades a single changed path on a resource type. resourceType is
// the full "group/version/Kind"; the version is ignored.
func ClassifyPath(resourceType, path string) Classification {
	key := disruptionKey(resourceType)
	norm := normalizePath(path)

	best := Classification{Path: path, Disruption: DisruptionNone}
	// Longest matching prefix wins, so launchTemplate.version (rolling) beats
	// launchTemplate.id (replace) rather than either shadowing the other.
	bestLen := -1
	for _, r := range disruptionTable[key] {
		if !pathMatches(norm, r.path) {
			continue
		}
		if len(r.path) > bestLen {
			bestLen = len(r.path)
			best = Classification{
				Path: path, Disruption: r.level, Reason: r.reason, Score: r.level.Score(),
			}
		}
	}
	return best
}

// pathMatches reports whether a changed path falls under a rule prefix. A rule
// matches the exact path or anything beneath it, but never a sibling with a
// shared string prefix ("versionRef" must not match "version").
func pathMatches(path, rule string) bool {
	if path == rule {
		return true
	}
	// The changed path is inside the rule's subtree.
	if strings.HasPrefix(path, rule+".") {
		return true
	}
	// A reference field is the same logical field: nodeRoleArnRef.name changes
	// nodeRoleArn, and subnetIdRefs/subnetIdSelector change subnetId.
	for _, suffix := range []string{"Ref", "Refs", "Selector"} {
		if strings.HasPrefix(path, rule+suffix) {
			return true
		}
	}
	// Plural/singular: the rule says subnetId, the field is subnetIds.
	if strings.HasPrefix(path, rule+"s.") || path == rule+"s" {
		return true
	}
	return false
}

// disruptionKey reduces "group/version/Kind" to "group/Kind", so rules apply
// across API versions.
//
// It also collapses the Crossplane v2 namespaced group onto its cluster-scoped
// twin (eks.aws.m.upbound.io -> eks.aws.upbound.io). The two carry identical
// fields and identical AWS immutability; only the Kubernetes scoping differs.
// Without this, a namespaced managed resource would match no rule and grade as
// "no disruption" — silently reporting a cluster replacement as safe, which is
// the one failure this classifier exists to prevent.
func disruptionKey(resourceType string) string {
	parts := strings.Split(resourceType, "/")
	if len(parts) < 2 {
		return resourceType
	}
	group := strings.Replace(parts[0], namespacedInfix, ".", 1)
	return group + "/" + parts[len(parts)-1]
}

// normalizePath strips numeric list indices, so a path into the deprecated
// list-shaped v1beta1 ("scalingConfig.0.desiredSize") matches the same rule as
// the object-shaped v1beta2 ("scalingConfig.desiredSize").
func normalizePath(path string) string {
	segs := strings.Split(path, ".")
	out := segs[:0]
	for _, s := range segs {
		if isNumeric(s) {
			continue
		}
		out = append(out, s)
	}
	return strings.Join(out, ".")
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ResourceChange is the disruption verdict for one resource in one Unit.
type ResourceChange struct {
	ResourceType  string           `json:"resourceType"`
	ResourceName  string           `json:"resourceName"`
	Changes       []Classification `json:"changes"`
	MaxDisruption Disruption       `json:"maxDisruption"`
	MaxScore      string           `json:"maxScore,omitempty"`
	Blocks        bool             `json:"blocks"`
}

// ClassifyResource grades every changed path on one resource.
func ClassifyResource(resourceType, resourceName string, changedPaths []string) ResourceChange {
	rc := ResourceChange{ResourceType: resourceType, ResourceName: resourceName}
	for _, p := range changedPaths {
		c := ClassifyPath(resourceType, p)
		rc.Changes = append(rc.Changes, c)
		rc.MaxDisruption = MaxDisruption(rc.MaxDisruption, c.Disruption)
	}
	sort.Slice(rc.Changes, func(i, j int) bool {
		if rc.Changes[i].Disruption.Rank() != rc.Changes[j].Disruption.Rank() {
			return rc.Changes[i].Disruption.Rank() > rc.Changes[j].Disruption.Rank()
		}
		return rc.Changes[i].Path < rc.Changes[j].Path
	})
	rc.MaxScore = rc.MaxDisruption.Score()
	rc.Blocks = rc.MaxDisruption.Blocks()
	return rc
}

// Remediation returns the action a blocking change requires.
func Remediation(d Disruption, kind string) string {
	switch d {
	case DisruptionReplaceCluster:
		return "this replaces the EKS control plane — it is a new cluster, not an edit; revert the field or create a new cluster deliberately"
	case DisruptionReplace:
		if kind == "NodeGroup" {
			return "this replaces the node group — use `replace-nodegroup` for a blue/green swap, or revert the field"
		}
		return fmt.Sprintf("this replaces the %s — create a replacement resource and retire the old one, or revert the field", kind)
	case DisruptionRolling:
		return "EKS will drain and replace every node in the group; check updateConfig.maxUnavailable and PodDisruptionBudgets first"
	}
	return ""
}

// DiffPaths returns the leaf paths whose values differ between two decoded
// resource documents, in dotted notation with list indices included.
func DiffPaths(oldDoc, newDoc any) []string {
	seen := map[string]bool{}
	var paths []string
	walkDiff("", oldDoc, newDoc, seen, &paths)
	sort.Strings(paths)
	return paths
}

func walkDiff(prefix string, a, b any, seen map[string]bool, out *[]string) {
	record := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			*out = append(*out, p)
		}
	}

	am, aIsMap := a.(map[string]any)
	bm, bIsMap := b.(map[string]any)
	if aIsMap && bIsMap {
		keys := map[string]bool{}
		for k := range am {
			keys[k] = true
		}
		for k := range bm {
			keys[k] = true
		}
		for k := range keys {
			walkDiff(join(prefix, k), am[k], bm[k], seen, out)
		}
		return
	}

	as, aIsSlice := a.([]any)
	bs, bIsSlice := b.([]any)
	if aIsSlice && bIsSlice {
		n := len(as)
		if len(bs) > n {
			n = len(bs)
		}
		for i := 0; i < n; i++ {
			var av, bv any
			if i < len(as) {
				av = as[i]
			}
			if i < len(bs) {
				bv = bs[i]
			}
			walkDiff(fmt.Sprintf("%s.%d", prefix, i), av, bv, seen, out)
		}
		return
	}

	// Scalars, or a type change (map -> scalar), or presence change.
	if !equalScalar(a, b) {
		record(prefix)
	}
}

func equalScalar(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Compare structurally when either side is composite; a type change counts
	// as a difference.
	switch a.(type) {
	case map[string]any, []any:
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
	switch b.(type) {
	case map[string]any, []any:
		return false
	}
	return a == b
}

func join(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// DisruptionPaths returns, for one disruption tier, the resource types and config paths registered
// at that severity. It is the same table `plan` grades against, exposed so the paths can be
// registered as ConfigHub Attributes and enforced server-side by vet-disruption — the client-side
// classifier and the server-side gate then cannot disagree, because there is one table.
//
// Paths are returned in their canonical (object-shaped) form. Rule matching normalizes list indices
// away, so a v1beta1 Unit still matches, but a registered path is a literal lookup — see the note in
// the eks-manager design doc about registering both shapes if a fleet is mid-migration.
func DisruptionPaths(level Disruption) map[string][]string {
	out := map[string][]string{}
	for key, rules := range disruptionTable {
		var paths []string
		for _, r := range rules {
			if r.level == level {
				paths = append(paths, r.path)
			}
		}
		if len(paths) == 0 {
			continue
		}
		sort.Strings(paths)
		out[key] = paths
	}
	return out
}

// DisruptionTiers lists the graded tiers, worst first.
func DisruptionTiers() []Disruption {
	return []Disruption{DisruptionReplaceCluster, DisruptionReplace, DisruptionRolling, DisruptionLow}
}

// AttributeName is the ConfigHub Attribute a tier's paths register under. It must match the
// suffixes vet-disruption looks for: <prefix>-critical / -high / -medium / -low.
func (d Disruption) AttributeName(prefix string) string {
	switch d {
	case DisruptionReplaceCluster:
		return prefix + "-critical"
	case DisruptionReplace:
		return prefix + "-high"
	case DisruptionRolling:
		return prefix + "-medium"
	case DisruptionLow:
		return prefix + "-low"
	}
	return ""
}
