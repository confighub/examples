// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package eks is the EKS analysis engine: it parses Crossplane managed resources
// (eks / ec2 / iam on *.aws.upbound.io) drawn from ConfigHub Units into a typed
// domain model, grouped per EKS cluster, and — in later milestones — grades
// pending changes by disruption and scores each cluster's posture over it.
//
// Parsing is lenient: malformed documents are skipped, never errored on — a bad
// resource in one Unit must not take down fleet-wide analysis. It is also
// deliberately shape-tolerant: upjet converted several singleton lists to
// embedded objects between v1beta1 and v1beta2, so `vpcConfig` and
// `scalingConfig` are objects on the current API and one-element lists on the
// deprecated one. See singleton.
package eks

import (
	"strconv"
	"strings"
)

// Crossplane API group suffixes. Managed resources on *.upbound.io are
// cluster-scoped; the .m. variants are namespaced (Crossplane v2 topology).
const (
	GroupSuffixUpbound    = ".upbound.io"
	GroupSuffixCrossplane = ".crossplane.io"
	namespacedInfix       = ".m."
)

// ResourceOrigin records where a resource came from in ConfigHub.
//
// Note the deliberate departure from the sibling managers: there, a cluster is a
// *Target* (the cluster a Unit deploys to). Here the Units *describe* an EKS
// cluster rather than deploy to one, so Cluster is the EKS cluster the resource
// belongs to — taken from the Space's Cluster label, falling back to the Space
// slug — while Target is the Crossplane *management* cluster the managed
// resources get applied to. The two are different clusters and must not be
// conflated.
type ResourceOrigin struct {
	Cluster      string            `json:"cluster"`
	Space        string            `json:"space"`
	SpaceID      string            `json:"spaceId"`
	SpaceLabels  map[string]string `json:"spaceLabels,omitempty"`
	Target       string            `json:"target,omitempty"`
	UnitID       string            `json:"unitId"`
	UnitSlug     string            `json:"unitSlug"`
	ResourceName string            `json:"resourceName"`
	// Canonical is true for definitions in base/policy Spaces that aren't
	// deployed anywhere — shown in the explorer but excluded from analysis.
	Canonical bool `json:"canonical,omitempty"`
}

// FleetResource is a parsed resource document plus its ConfigHub origin. Doc is
// the decoded JSON body (typically a map[string]any).
type FleetResource struct {
	Origin ResourceOrigin
	Doc    any
}

// AutoMode is the EKS Auto Mode tuple. AWS requires computeConfig.enabled,
// kubernetesNetworkConfig.elasticLoadBalancing.enabled, and
// storageConfig.blockStorage.enabled to all be present and all agree on any
// update touching Auto Mode; a partial or disagreeing set wedges the resource.
type AutoMode struct {
	ComputeEnabled       *bool    `json:"computeEnabled,omitempty"`
	LoadBalancingEnabled *bool    `json:"loadBalancingEnabled,omitempty"`
	BlockStorageEnabled  *bool    `json:"blockStorageEnabled,omitempty"`
	NodePools            []string `json:"nodePools,omitempty"`
	NodeRoleARN          string   `json:"nodeRoleArn,omitempty"`
}

// Declared reports whether any Auto Mode field is set at all.
func (a AutoMode) Declared() bool {
	return a.ComputeEnabled != nil || a.LoadBalancingEnabled != nil || a.BlockStorageEnabled != nil
}

// Enabled reports whether Auto Mode is fully and consistently enabled.
func (a AutoMode) Enabled() bool {
	return isTrue(a.ComputeEnabled) && isTrue(a.LoadBalancingEnabled) && isTrue(a.BlockStorageEnabled)
}

// Consistent reports whether the three toggles are all present and all agree.
// An inconsistent tuple is a hard AWS validation failure, not a warning.
func (a AutoMode) Consistent() bool {
	if !a.Declared() {
		return true // absent entirely is fine; that's a classic cluster
	}
	if a.ComputeEnabled == nil || a.LoadBalancingEnabled == nil || a.BlockStorageEnabled == nil {
		return false
	}
	return *a.ComputeEnabled == *a.LoadBalancingEnabled && *a.ComputeEnabled == *a.BlockStorageEnabled
}

// ClusterEntity is a parsed eks Cluster managed resource.
type ClusterEntity struct {
	Name    string `json:"name"`
	Region  string `json:"region,omitempty"`
	Version string `json:"version,omitempty"`
	// ExternalName is the actual AWS cluster name: the crossplane.io/external-name
	// annotation when set, else metadata.name (Cluster uses NameAsIdentifier).
	ExternalName string `json:"externalName,omitempty"`

	EndpointPublicAccess  *bool    `json:"endpointPublicAccess,omitempty"`
	EndpointPrivateAccess *bool    `json:"endpointPrivateAccess,omitempty"`
	PublicAccessCIDRs     []string `json:"publicAccessCidrs,omitempty"`
	LogTypes              []string `json:"enabledClusterLogTypes,omitempty"`
	EncryptionConfigured  bool     `json:"encryptionConfigured"`
	DeletionProtection    *bool    `json:"deletionProtection,omitempty"`
	AuthenticationMode    string   `json:"authenticationMode,omitempty"`
	UpgradeSupportType    string   `json:"upgradeSupportType,omitempty"`
	// BootstrapSelfManagedAddons is ForceNew and must be false for Auto Mode, so
	// it is effectively a creation-time decision.
	BootstrapSelfManagedAddons *bool    `json:"bootstrapSelfManagedAddons,omitempty"`
	AutoMode                   AutoMode `json:"autoMode"`

	APIVersion         string         `json:"apiVersion"`
	DeletionPolicy     string         `json:"deletionPolicy,omitempty"`
	ManagementPolicies []string       `json:"managementPolicies,omitempty"`
	Origin             ResourceOrigin `json:"origin"`
}

// NodeGroupEntity is a parsed eks NodeGroup managed resource.
type NodeGroupEntity struct {
	Name        string `json:"name"`
	Region      string `json:"region,omitempty"`
	ClusterName string `json:"clusterName,omitempty"`

	Version        string `json:"version,omitempty"`
	ReleaseVersion string `json:"releaseVersion,omitempty"`

	// ForceNew fields — identity, not configuration. Changing one cannot be
	// reconciled in place; Crossplane refuses the update permanently.
	AMIType       string   `json:"amiType,omitempty"`
	CapacityType  string   `json:"capacityType,omitempty"`
	InstanceTypes []string `json:"instanceTypes,omitempty"`
	DiskSize      *int64   `json:"diskSize,omitempty"`

	MinSize     *int64 `json:"minSize,omitempty"`
	MaxSize     *int64 `json:"maxSize,omitempty"`
	DesiredSize *int64 `json:"desiredSize,omitempty"`
	// DesiredSizeInInitProvider reports whether desiredSize is declared under
	// spec.initProvider (the intended external-scaling pattern) rather than
	// spec.forProvider. Note upjet's ignore-path bug makes this ineffective on
	// the object-shaped API versions; the analyzer in M2 reports it.
	DesiredSizeInInitProvider bool `json:"desiredSizeInInitProvider"`

	LaunchTemplateID      string `json:"launchTemplateId,omitempty"`
	LaunchTemplateName    string `json:"launchTemplateName,omitempty"`
	LaunchTemplateVersion string `json:"launchTemplateVersion,omitempty"`

	APIVersion         string         `json:"apiVersion"`
	DeletionPolicy     string         `json:"deletionPolicy,omitempty"`
	ManagementPolicies []string       `json:"managementPolicies,omitempty"`
	Origin             ResourceOrigin `json:"origin"`
}

// AddonEntity is a parsed eks Addon managed resource.
type AddonEntity struct {
	Name         string `json:"name"`
	Region       string `json:"region,omitempty"`
	ClusterName  string `json:"clusterName,omitempty"`
	AddonName    string `json:"addonName,omitempty"`
	AddonVersion string `json:"addonVersion,omitempty"`

	ResolveConflictsOnCreate string `json:"resolveConflictsOnCreate,omitempty"`
	ResolveConflictsOnUpdate string `json:"resolveConflictsOnUpdate,omitempty"`
	Preserve                 *bool  `json:"preserve,omitempty"`

	APIVersion string         `json:"apiVersion"`
	Origin     ResourceOrigin `json:"origin"`
}

// ResourceEntity is the generic representation for managed resources the model
// does not (yet) parse in detail — ec2 networking, iam, and the less common eks
// kinds. Enough for inventory, reference integrity, and the explorer.
type ResourceEntity struct {
	Group      string         `json:"group"`
	Version    string         `json:"version"`
	Kind       string         `json:"kind"`
	Name       string         `json:"name"`
	Region     string         `json:"region,omitempty"`
	APIVersion string         `json:"apiVersion"`
	Origin     ResourceOrigin `json:"origin"`
}

// ClusterSet holds every managed resource belonging to one EKS cluster.
type ClusterSet struct {
	Cluster    string             `json:"cluster"`
	Control    *ClusterEntity     `json:"control,omitempty"`
	NodeGroups []*NodeGroupEntity `json:"nodeGroups,omitempty"`
	Addons     []*AddonEntity     `json:"addons,omitempty"`
	Network    []*ResourceEntity  `json:"network,omitempty"`
	IAM        []*ResourceEntity  `json:"iam,omitempty"`
	OtherEKS   []*ResourceEntity  `json:"otherEks,omitempty"`
}

// BuildFleet indexes parsed fleet resources into per-cluster entity sets.
// Unrecognized groups and unparseable docs are ignored. Entities within a
// cluster preserve input order.
func BuildFleet(resources []FleetResource) map[string]*ClusterSet {
	out := map[string]*ClusterSet{}
	get := func(key string) *ClusterSet {
		if cs, ok := out[key]; ok {
			return cs
		}
		cs := &ClusterSet{Cluster: key}
		out[key] = cs
		return cs
	}

	for _, r := range resources {
		rec, ok := asRecord(r.Doc)
		if !ok {
			continue
		}
		apiVersion, _ := asString(rec["apiVersion"])
		kind, _ := asString(rec["kind"])
		group, version := SplitAPIVersion(apiVersion)
		if group == "" || kind == "" {
			continue
		}
		md, _ := asRecord(rec["metadata"])
		name, _ := asString(md["name"])
		if name == "" {
			continue
		}
		cs := get(r.Origin.Cluster)

		switch {
		case isEKSGroup(group) && kind == "Cluster":
			cs.Control = parseCluster(rec, apiVersion, name, r.Origin)
		case isEKSGroup(group) && kind == "NodeGroup":
			cs.NodeGroups = append(cs.NodeGroups, parseNodeGroup(rec, apiVersion, name, r.Origin))
		case isEKSGroup(group) && kind == "Addon":
			cs.Addons = append(cs.Addons, parseAddon(rec, apiVersion, name, r.Origin))
		case isEKSGroup(group):
			cs.OtherEKS = append(cs.OtherEKS, generic(rec, group, version, kind, name, apiVersion, r.Origin))
		case strings.HasPrefix(group, "ec2."):
			cs.Network = append(cs.Network, generic(rec, group, version, kind, name, apiVersion, r.Origin))
		case strings.HasPrefix(group, "iam."):
			cs.IAM = append(cs.IAM, generic(rec, group, version, kind, name, apiVersion, r.Origin))
		}
	}
	return out
}

func isEKSGroup(group string) bool { return strings.HasPrefix(group, "eks.") }

func generic(rec map[string]any, group, version, kind, name, apiVersion string, origin ResourceOrigin) *ResourceEntity {
	fp := forProvider(rec)
	region, _ := asString(fp["region"])
	return &ResourceEntity{
		Group: group, Version: version, Kind: kind, Name: name,
		Region: region, APIVersion: apiVersion, Origin: origin,
	}
}

func parseCluster(rec map[string]any, apiVersion, name string, origin ResourceOrigin) *ClusterEntity {
	spec, _ := asRecord(rec["spec"])
	fp := forProvider(rec)
	c := &ClusterEntity{
		Name:         name,
		ExternalName: externalName(rec, name),
		APIVersion:   apiVersion,
		Origin:       origin,
	}
	c.Region, _ = asString(fp["region"])
	c.Version, _ = asString(fp["version"])
	c.LogTypes = asStringArray(fp["enabledClusterLogTypes"])
	c.DeletionProtection = boolFrom(fp["deletionProtection"])
	c.BootstrapSelfManagedAddons = boolFrom(fp["bootstrapSelfManagedAddons"])
	c.DeletionPolicy, _ = asString(spec["deletionPolicy"])
	c.ManagementPolicies = asStringArray(spec["managementPolicies"])

	if vpc := singleton(fp["vpcConfig"]); vpc != nil {
		c.EndpointPublicAccess = boolFrom(vpc["endpointPublicAccess"])
		c.EndpointPrivateAccess = boolFrom(vpc["endpointPrivateAccess"])
		c.PublicAccessCIDRs = asStringArray(vpc["publicAccessCidrs"])
	}
	if ac := singleton(fp["accessConfig"]); ac != nil {
		c.AuthenticationMode, _ = asString(ac["authenticationMode"])
	}
	if up := singleton(fp["upgradePolicy"]); up != nil {
		c.UpgradeSupportType, _ = asString(up["supportType"])
	}
	// encryptionConfig is a list of {provider,resources} entries in every version.
	if enc := singleton(fp["encryptionConfig"]); enc != nil {
		c.EncryptionConfigured = true
	}

	if cc := singleton(fp["computeConfig"]); cc != nil {
		c.AutoMode.ComputeEnabled = boolFrom(cc["enabled"])
		c.AutoMode.NodePools = asStringArray(cc["nodePools"])
		c.AutoMode.NodeRoleARN, _ = asString(cc["nodeRoleArn"])
	}
	if sc := singleton(fp["storageConfig"]); sc != nil {
		if bs := singleton(sc["blockStorage"]); bs != nil {
			c.AutoMode.BlockStorageEnabled = boolFrom(bs["enabled"])
		}
	}
	if knc := singleton(fp["kubernetesNetworkConfig"]); knc != nil {
		if elb := singleton(knc["elasticLoadBalancing"]); elb != nil {
			c.AutoMode.LoadBalancingEnabled = boolFrom(elb["enabled"])
		}
	}
	return c
}

func parseNodeGroup(rec map[string]any, apiVersion, name string, origin ResourceOrigin) *NodeGroupEntity {
	spec, _ := asRecord(rec["spec"])
	fp := forProvider(rec)
	n := &NodeGroupEntity{Name: name, APIVersion: apiVersion, Origin: origin}
	n.Region, _ = asString(fp["region"])
	n.ClusterName = refName(fp, "clusterName")
	n.Version, _ = asString(fp["version"])
	n.ReleaseVersion, _ = asString(fp["releaseVersion"])
	n.AMIType, _ = asString(fp["amiType"])
	n.CapacityType, _ = asString(fp["capacityType"])
	n.InstanceTypes = asStringArray(fp["instanceTypes"])
	n.DiskSize = intFrom(fp["diskSize"])
	n.DeletionPolicy, _ = asString(spec["deletionPolicy"])
	n.ManagementPolicies = asStringArray(spec["managementPolicies"])

	if sc := singleton(fp["scalingConfig"]); sc != nil {
		n.MinSize = intFrom(sc["minSize"])
		n.MaxSize = intFrom(sc["maxSize"])
		n.DesiredSize = intFrom(sc["desiredSize"])
	}
	// desiredSize under initProvider is the documented external-scaling pattern.
	if ip, ok := asRecord(spec["initProvider"]); ok {
		if sc := singleton(ip["scalingConfig"]); sc != nil {
			if d := intFrom(sc["desiredSize"]); d != nil {
				n.DesiredSizeInInitProvider = true
				if n.DesiredSize == nil {
					n.DesiredSize = d
				}
			}
		}
	}
	if lt := singleton(fp["launchTemplate"]); lt != nil {
		n.LaunchTemplateID, _ = asString(lt["id"])
		n.LaunchTemplateName, _ = asString(lt["name"])
		n.LaunchTemplateVersion = asScalarString(lt["version"])
	}
	return n
}

func parseAddon(rec map[string]any, apiVersion, name string, origin ResourceOrigin) *AddonEntity {
	fp := forProvider(rec)
	a := &AddonEntity{Name: name, APIVersion: apiVersion, Origin: origin}
	a.Region, _ = asString(fp["region"])
	a.ClusterName = refName(fp, "clusterName")
	a.AddonName, _ = asString(fp["addonName"])
	a.AddonVersion, _ = asString(fp["addonVersion"])
	a.ResolveConflictsOnCreate, _ = asString(fp["resolveConflictsOnCreate"])
	a.ResolveConflictsOnUpdate, _ = asString(fp["resolveConflictsOnUpdate"])
	a.Preserve = boolFrom(fp["preserve"])
	return a
}

// forProvider returns spec.forProvider as a record (empty when absent).
func forProvider(rec map[string]any) map[string]any {
	spec, _ := asRecord(rec["spec"])
	fp, ok := asRecord(spec["forProvider"])
	if !ok {
		return map[string]any{}
	}
	return fp
}

// refName resolves a Crossplane cross-resource reference to the referenced
// object's name, preferring the literal field when already resolved. For base
// name "clusterName" it checks clusterName, then clusterNameRef.name. A
// *Selector cannot be resolved without the whole set, so it yields "".
func refName(fp map[string]any, base string) string {
	if s, ok := asString(fp[base]); ok && s != "" {
		return s
	}
	if ref := singleton(fp[base+"Ref"]); ref != nil {
		if s, ok := asString(ref["name"]); ok {
			return s
		}
	}
	return ""
}

// externalName returns the crossplane.io/external-name annotation when set,
// else the fallback (metadata.name). The annotation is authoritative: it is what
// the provider uses to address the resource in AWS.
func externalName(rec map[string]any, fallback string) string {
	md, _ := asRecord(rec["metadata"])
	ann, _ := asRecord(md["annotations"])
	if s, ok := asString(ann["crossplane.io/external-name"]); ok && s != "" {
		return s
	}
	return fallback
}

// SplitAPIVersion splits "group/version" into its parts. A core-style
// apiVersion with no slash ("v1") yields an empty group.
func SplitAPIVersion(apiVersion string) (group, version string) {
	i := strings.LastIndex(apiVersion, "/")
	if i < 0 {
		return "", apiVersion
	}
	return apiVersion[:i], apiVersion[i+1:]
}

// IsClusterScopedGroup reports whether a Crossplane API group is cluster-scoped,
// by the group-suffix rule rather than by enumerating thousands of managed
// resource types: groups under *.upbound.io / *.crossplane.io are cluster-scoped
// unless they carry the ".m." namespaced infix (Crossplane v2 topology).
//
// It does not classify user-defined composite resources, whose groups are
// arbitrary — those need an explicit where-expression.
func IsClusterScopedGroup(group string) bool {
	if !strings.HasSuffix(group, GroupSuffixUpbound) && !strings.HasSuffix(group, GroupSuffixCrossplane) {
		return false
	}
	return !strings.Contains(group, namespacedInfix)
}

// ResourceMeta extracts the apiVersion, kind, and name from a decoded resource
// document. ok is false when the doc is not an object or lacks a kind/name.
func ResourceMeta(doc any) (apiVersion, kind, name string, ok bool) {
	rec, isRec := asRecord(doc)
	if !isRec {
		return "", "", "", false
	}
	apiVersion, _ = asString(rec["apiVersion"])
	kind, _ = asString(rec["kind"])
	md, _ := asRecord(rec["metadata"])
	name, _ = asString(md["name"])
	return apiVersion, kind, name, kind != "" && name != ""
}

// --- lenient decoding helpers ---

// singleton returns v as a record, accepting both shapes upjet produces for a
// MaxItems:1 Terraform block: the embedded object of eks.aws.upbound.io/v1beta2
// (and the namespaced v1beta1), and the one-element list of the deprecated
// cluster-scoped v1beta1. Returns nil when v is neither, or is an empty list.
func singleton(v any) map[string]any {
	if rec, ok := asRecord(v); ok {
		return rec
	}
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return nil
	}
	rec, ok := asRecord(arr[0])
	if !ok {
		return nil
	}
	return rec
}

func asRecord(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func asString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func asBool(v any) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}

// asInt coerces a JSON number (always float64 from encoding/json) or integer to
// int64.
func asInt(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
}

// asScalarString renders a scalar (string or number) as a string, "" otherwise.
// Used for values AWS accepts either way, such as launchTemplate.version
// ("$Latest" or 3).
func asScalarString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	}
	return ""
}

func asStringArray(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// boolFrom returns the first set bool among the given values, or nil if none is
// a bool.
func boolFrom(vals ...any) *bool {
	for _, v := range vals {
		if b, ok := asBool(v); ok {
			return &b
		}
	}
	return nil
}

// intFrom returns the first set int among the given values, or nil.
func intFrom(vals ...any) *int64 {
	for _, v := range vals {
		if n, ok := asInt(v); ok {
			return &n
		}
	}
	return nil
}

func isTrue(b *bool) bool { return b != nil && *b }
