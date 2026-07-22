// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package eks

import (
	"fmt"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"
)

// Placeholder is ConfigHub's string placeholder. vet-placeholders blocks apply
// while any remain, which is exactly the behavior we want for a value that
// genuinely cannot be known at authoring time (see AutoNodeRoleARN).
const Placeholder = "confighubplaceholder"

// NAT strategies for private-subnet egress.
const (
	NATSingle = "single"
	NATPerAZ  = "per-az"
	NATNone   = "none"
)

// Managed policy ARNs. These follow AWS's *current* guidance, which has moved:
// the node role now takes AmazonEC2ContainerRegistryPullOnly (not the older
// ReadOnly), and AmazonEKS_CNI_Policy is deliberately absent — AWS recommends
// attaching it to an IRSA/Pod-Identity role bound to the aws-node service
// account rather than to the node role.
var (
	autoModeClusterPolicies = []string{
		"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
		"arn:aws:iam::aws:policy/AmazonEKSComputePolicy",
		"arn:aws:iam::aws:policy/AmazonEKSBlockStoragePolicyV2",
		"arn:aws:iam::aws:policy/AmazonEKSLoadBalancingPolicy",
		"arn:aws:iam::aws:policy/AmazonEKSNetworkingPolicy",
	}
	classicClusterPolicies = []string{
		"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
	}
	autoModeNodePolicies = []string{
		"arn:aws:iam::aws:policy/AmazonEKSWorkerNodeMinimalPolicy",
		"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryPullOnly",
	}
	classicNodePolicies = []string{
		"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
		"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryPullOnly",
	}
)

// Trust policies. Auto Mode's cluster role additionally needs sts:TagSession.
const (
	eksTrustAutoMode = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"eks.amazonaws.com"},"Action":["sts:AssumeRole","sts:TagSession"]}]}`
	eksTrustClassic  = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"eks.amazonaws.com"},"Action":["sts:AssumeRole"]}]}`
	ec2Trust         = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":["sts:AssumeRole"]}]}`
)

// API versions. The object-shaped v1beta2 is the current storage version for the
// EKS kinds that have one; the rest are still v1beta1.
const (
	apiEKSv1beta2 = "eks.aws.upbound.io/v1beta2"
	apiEKSv1beta1 = "eks.aws.upbound.io/v1beta1"
	apiEC2v1beta1 = "ec2.aws.upbound.io/v1beta1"
	apiEC2v1beta2 = "ec2.aws.upbound.io/v1beta2"
	apiIAMv1beta1 = "iam.aws.upbound.io/v1beta1"
)

// NodeGroupSpec describes one managed node group to generate (classic mode).
type NodeGroupSpec struct {
	Name          string
	InstanceTypes []string
	CapacityType  string
	// AMIType is optional: empty lets EKS pick the default for the cluster's
	// version. It is ForceNew, so changing it means replacing the node group.
	AMIType     string
	MinSize     int64
	MaxSize     int64
	DesiredSize int64
	DiskSize    int64
}

// AddonSpec describes one EKS addon to generate (classic mode).
type AddonSpec struct {
	Name    string
	Version string
}

// ClusterSpec is the full input to Generate.
type ClusterSpec struct {
	Name        string
	Region      string
	Version     string
	Environment string

	// AutoMode emits the EKS Auto Mode tuple and no node groups. It is the
	// default because it removes the NodeGroup resource entirely, and with it
	// the desiredSize-vs-autoscaler conflict that is currently unfixable in the
	// provider.
	AutoMode  bool
	NodePools []string
	// AutoNodeRoleARN is the one value that cannot be expressed as a reference:
	// computeConfig.nodeRoleArn has no Ref/Selector in the provider. Empty
	// yields Placeholder, so vet-placeholders gates apply until it is supplied.
	AutoNodeRoleARN string

	Zones   []string
	VPCCIDR string
	NAT     string

	NodeGroups []NodeGroupSpec
	Addons     []AddonSpec

	// PublicEndpoint exposes the Kubernetes API endpoint publicly. Default is
	// private-only.
	PublicEndpoint bool
	// Orphan sets deletionPolicy: Orphan, so deleting the managed resource
	// leaves the AWS resource in place.
	Orphan bool

	ProviderConfig string
}

// GeneratedUnit is one ConfigHub Unit's worth of generated config: exactly one
// Crossplane managed resource, per the one-resource-per-Unit rule.
type GeneratedUnit struct {
	Slug   string            `json:"slug"`
	Kind   string            `json:"kind"`
	Group  string            `json:"group"`
	Labels map[string]string `json:"labels,omitempty"`
	YAML   string            `json:"yaml"`
}

// DefaultZones derives deterministic availability zones for a region.
//
// This is deliberately unlike eksctl, which randomizes AZ selection per
// invocation (rand.Perm seeded by wall clock). "Same config produces the same
// infrastructure" is not a property eksctl has; it is one of ours, so the zones
// are pinned into the generated Units.
func DefaultZones(region string, count int) []string {
	zones := make([]string, 0, count)
	for i := 0; i < count; i++ {
		zones = append(zones, region+string(rune('a'+i)))
	}
	return zones
}

// Generate produces the full envelope of Units for a new EKS cluster: IAM roles
// and policy attachments, the VPC and its subnets / gateways / route tables, the
// EKS control plane, and (in classic mode) node groups and addons.
//
// Every cross-resource reference is emitted as a Ref or Selector, never a
// literal identifier. That is what makes the resulting set safe to apply in any
// order: the provider's reference resolvers hold each managed resource at
// Synced=False until its dependency exists, so there is no ordering requirement
// for ConfigHub to encode.
func Generate(spec ClusterSpec) ([]GeneratedUnit, error) {
	if err := spec.validate(); err != nil {
		return nil, err
	}
	g := &generator{spec: spec}

	g.iamRoles()
	g.network()
	g.controlPlane()
	if !spec.AutoMode {
		g.nodeGroups()
		g.addons()
	}
	return g.units, g.err
}

func (s *ClusterSpec) validate() error {
	if s.Name == "" {
		return fmt.Errorf("cluster name is required")
	}
	if s.Region == "" {
		return fmt.Errorf("--region is required")
	}
	if s.Version == "" {
		return fmt.Errorf("--version is required")
	}
	if _, ok := ParseVersion(s.Version); !ok {
		return fmt.Errorf("--version %q is not a Kubernetes major.minor version (e.g. 1.34)", s.Version)
	}
	if len(s.Zones) == 0 {
		return fmt.Errorf("at least one availability zone is required")
	}
	if len(s.Zones) > 4 {
		return fmt.Errorf("at most 4 availability zones are supported (got %d); the default /16 splits into 8 x /19", len(s.Zones))
	}
	switch s.NAT {
	case NATSingle, NATPerAZ, NATNone:
	default:
		return fmt.Errorf("--nat %q: want %s | %s | %s", s.NAT, NATSingle, NATPerAZ, NATNone)
	}
	if _, _, err := parseVPCCIDR(s.VPCCIDR); err != nil {
		return err
	}
	if s.AutoMode && len(s.NodeGroups) > 0 {
		return fmt.Errorf("--node-group is not valid with Auto Mode: Auto Mode manages capacity itself (pass --auto-mode=false for managed node groups)")
	}
	return nil
}

type generator struct {
	spec  ClusterSpec
	units []GeneratedUnit
	err   error
}

// buildUnit marshals one managed resource into a Unit. Shared by the whole-
// cluster generator and the single-resource entry points so both emit
// byte-identical output.
func buildUnit(slug, group, apiVersion, kind, name string, labels map[string]string, spec map[string]any, clusterName string) (GeneratedUnit, error) {
	doc := map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   metadata(name, labels),
		"spec":       spec,
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return GeneratedUnit{}, fmt.Errorf("marshal %s/%s: %w", kind, name, err)
	}
	return GeneratedUnit{
		Slug: slug, Kind: kind, Group: group,
		Labels: map[string]string{"cluster": clusterName, "group": group},
		YAML:   string(out),
	}, nil
}

// add appends a resource to the generator. The first marshalling error wins and
// is returned from Generate.
func (g *generator) add(slug, group, apiVersion, kind, name string, labels map[string]string, spec map[string]any) {
	if g.err != nil {
		return
	}
	u, err := buildUnit(slug, group, apiVersion, kind, name, labels, spec, g.spec.Name)
	if err != nil {
		g.err = err
		return
	}
	g.units = append(g.units, u)
}

func metadata(name string, labels map[string]string) map[string]any {
	md := map[string]any{"name": name}
	if len(labels) > 0 {
		md["labels"] = toAny(labels)
	}
	return md
}

// providerSpec wraps forProvider with the common Crossplane spec fields. IAM
// resources are global, so they carry no region — the caller omits it.
func (g *generator) providerSpec(forProvider map[string]any) map[string]any {
	return g.spec.clusterContext().providerSpec(forProvider)
}

// --- IAM ---

func (g *generator) iamRoles() {
	name := g.spec.Name
	clusterTrust, clusterPolicies := eksTrustClassic, classicClusterPolicies
	nodePolicies := classicNodePolicies
	if g.spec.AutoMode {
		clusterTrust, clusterPolicies = eksTrustAutoMode, autoModeClusterPolicies
		nodePolicies = autoModeNodePolicies
	}

	clusterRole := name + "-cluster-role"
	g.add("cluster-role", "iam", apiIAMv1beta1, "Role", clusterRole, nil,
		g.providerSpec(map[string]any{"assumeRolePolicy": clusterTrust}))
	g.attachments("cluster-role", clusterRole, clusterPolicies)

	nodeRole := name + "-node-role"
	g.add("node-role", "iam", apiIAMv1beta1, "Role", nodeRole, nil,
		g.providerSpec(map[string]any{"assumeRolePolicy": ec2Trust}))
	g.attachments("node-role", nodeRole, nodePolicies)
}

// attachments emits one RolePolicyAttachment per managed policy. They are
// separate resources (rather than Role.managedPolicyArns) because the two
// mechanisms conflict, and one-per-Unit keeps a policy change to one revision.
func (g *generator) attachments(slugPrefix, roleName string, policies []string) {
	for _, arn := range policies {
		short := arn[strings.LastIndex(arn, "/")+1:]
		g.add(slugPrefix+"-"+strings.ToLower(short), "iam", apiIAMv1beta1, "RolePolicyAttachment",
			roleName+"-"+short, nil,
			g.providerSpec(map[string]any{
				"policyArn": arn,
				"roleRef":   map[string]any{"name": roleName},
			}))
	}
}

// --- Network ---

func (g *generator) network() {
	s := g.spec
	name := s.Name
	region := s.Region

	vpcName := name + "-vpc"
	g.add("vpc", "ec2", apiEC2v1beta1, "VPC", vpcName, map[string]string{"cluster": name},
		g.providerSpec(map[string]any{
			"region":    region,
			"cidrBlock": s.VPCCIDR,
			// Both are required by EKS.
			"enableDnsSupport":   true,
			"enableDnsHostnames": true,
			"tags":               toAny(map[string]string{"Name": vpcName}),
		}))

	igwName := name + "-igw"
	g.add("igw", "ec2", apiEC2v1beta1, "InternetGateway", igwName, nil,
		g.providerSpec(map[string]any{
			"region":   region,
			"vpcIdRef": map[string]any{"name": vpcName},
			"tags":     toAny(map[string]string{"Name": igwName}),
		}))

	// Subnets: public first, then private, so the /19 offsets are stable as AZs
	// are added. The tier label is what the Cluster and node groups select on.
	for i, az := range s.Zones {
		cidr, _ := subnetCIDR(s.VPCCIDR, i)
		sn := fmt.Sprintf("%s-public-%s", name, azSuffix(region, az))
		g.add("subnet-public-"+azSuffix(region, az), "ec2", apiEC2v1beta1, "Subnet", sn,
			map[string]string{"cluster": name, "tier": "public"},
			g.providerSpec(map[string]any{
				"region":              region,
				"availabilityZone":    az,
				"cidrBlock":           cidr,
				"mapPublicIpOnLaunch": true,
				"vpcIdRef":            map[string]any{"name": vpcName},
				// Required for internet-facing load balancer placement.
				"tags": toAny(map[string]string{"Name": sn, "kubernetes.io/role/elb": "1"}),
			}))
	}
	for i, az := range s.Zones {
		cidr, _ := subnetCIDR(s.VPCCIDR, len(s.Zones)+i)
		sn := fmt.Sprintf("%s-private-%s", name, azSuffix(region, az))
		g.add("subnet-private-"+azSuffix(region, az), "ec2", apiEC2v1beta1, "Subnet", sn,
			map[string]string{"cluster": name, "tier": "private"},
			g.providerSpec(map[string]any{
				"region":           region,
				"availabilityZone": az,
				"cidrBlock":        cidr,
				"vpcIdRef":         map[string]any{"name": vpcName},
				// Required for internal load balancer placement.
				"tags": toAny(map[string]string{"Name": sn, "kubernetes.io/role/internal-elb": "1"}),
			}))
	}

	// Public route table: one, shared, with the default route to the IGW.
	rtPublic := name + "-public-rt"
	g.add("rt-public", "ec2", apiEC2v1beta1, "RouteTable", rtPublic, nil,
		g.providerSpec(map[string]any{
			"region":   region,
			"vpcIdRef": map[string]any{"name": vpcName},
			"tags":     toAny(map[string]string{"Name": rtPublic}),
		}))
	g.add("route-public-igw", "ec2", apiEC2v1beta2, "Route", name+"-public-default", nil,
		g.providerSpec(map[string]any{
			"region":               region,
			"routeTableIdRef":      map[string]any{"name": rtPublic},
			"destinationCidrBlock": "0.0.0.0/0",
			"gatewayIdRef":         map[string]any{"name": igwName},
		}))
	for _, az := range s.Zones {
		suffix := azSuffix(region, az)
		g.add("rta-public-"+suffix, "ec2", apiEC2v1beta1, "RouteTableAssociation",
			fmt.Sprintf("%s-public-%s-assoc", name, suffix), nil,
			g.providerSpec(map[string]any{
				"region":          region,
				"routeTableIdRef": map[string]any{"name": rtPublic},
				"subnetIdRef":     map[string]any{"name": fmt.Sprintf("%s-public-%s", name, suffix)},
			}))
	}

	g.natGateways()

	// Private route tables: one per AZ regardless of NAT strategy, so switching
	// from single to per-az NAT later is a route edit, not a re-architecture.
	for _, az := range s.Zones {
		suffix := azSuffix(region, az)
		rt := fmt.Sprintf("%s-private-%s-rt", name, suffix)
		g.add("rt-private-"+suffix, "ec2", apiEC2v1beta1, "RouteTable", rt, nil,
			g.providerSpec(map[string]any{
				"region":   region,
				"vpcIdRef": map[string]any{"name": vpcName},
				"tags":     toAny(map[string]string{"Name": rt}),
			}))
		g.add("rta-private-"+suffix, "ec2", apiEC2v1beta1, "RouteTableAssociation",
			fmt.Sprintf("%s-private-%s-assoc", name, suffix), nil,
			g.providerSpec(map[string]any{
				"region":          region,
				"routeTableIdRef": map[string]any{"name": rt},
				"subnetIdRef":     map[string]any{"name": fmt.Sprintf("%s-private-%s", name, suffix)},
			}))

		if s.NAT == NATNone {
			continue
		}
		natRef := g.natNameFor(az)
		g.add("route-private-"+suffix+"-nat", "ec2", apiEC2v1beta2, "Route",
			fmt.Sprintf("%s-private-%s-default", name, suffix), nil,
			g.providerSpec(map[string]any{
				"region":               region,
				"routeTableIdRef":      map[string]any{"name": rt},
				"destinationCidrBlock": "0.0.0.0/0",
				"natGatewayIdRef":      map[string]any{"name": natRef},
			}))
	}
}

func (g *generator) natGateways() {
	s := g.spec
	if s.NAT == NATNone {
		return
	}
	zones := s.Zones
	if s.NAT == NATSingle {
		zones = s.Zones[:1]
	}
	for _, az := range zones {
		suffix := azSuffix(s.Region, az)
		eip := fmt.Sprintf("%s-nat-%s-eip", s.Name, suffix)
		g.add("eip-nat-"+suffix, "ec2", apiEC2v1beta1, "EIP", eip, nil,
			g.providerSpec(map[string]any{
				"region": s.Region,
				"domain": "vpc",
				"tags":   toAny(map[string]string{"Name": eip}),
			}))
		nat := fmt.Sprintf("%s-nat-%s", s.Name, suffix)
		g.add("nat-"+suffix, "ec2", apiEC2v1beta1, "NATGateway", nat, nil,
			g.providerSpec(map[string]any{
				"region":           s.Region,
				"connectivityType": "public",
				"allocationIdRef":  map[string]any{"name": eip},
				"subnetIdRef":      map[string]any{"name": fmt.Sprintf("%s-public-%s", s.Name, suffix)},
				"tags":             toAny(map[string]string{"Name": nat}),
			}))
	}
}

// natNameFor returns the NAT gateway a given AZ's private subnet egresses
// through: its own under per-az, the single shared one otherwise.
func (g *generator) natNameFor(az string) string {
	if g.spec.NAT == NATPerAZ {
		return fmt.Sprintf("%s-nat-%s", g.spec.Name, azSuffix(g.spec.Region, az))
	}
	return fmt.Sprintf("%s-nat-%s", g.spec.Name, azSuffix(g.spec.Region, g.spec.Zones[0]))
}

// --- EKS ---

func (g *generator) controlPlane() {
	s := g.spec
	fp := map[string]any{
		"region":  s.Region,
		"version": s.Version,
		// ForceNew, and Auto Mode requires it false. Emitting false from the
		// start keeps Auto Mode reachable later; getting it wrong makes the
		// switch impossible without replacing the cluster.
		"bootstrapSelfManagedAddons": false,
		"deletionProtection":         true,
		"enabledClusterLogTypes":     []any{"api", "audit", "authenticator"},
		"roleArnRef":                 map[string]any{"name": s.Name + "-cluster-role"},
		"accessConfig":               map[string]any{"authenticationMode": "API"},
		// Pin the version, so pair it with EXTENDED support: under STANDARD, AWS
		// auto-upgrades at end of standard support and then fights the pin.
		"upgradePolicy": map[string]any{"supportType": "EXTENDED"},
		"vpcConfig": map[string]any{
			"endpointPrivateAccess": true,
			"endpointPublicAccess":  s.PublicEndpoint,
			"subnetIdSelector": map[string]any{
				"matchLabels": toAny(map[string]string{"cluster": s.Name, "tier": "private"}),
			},
		},
		"tags": toAny(g.awsTags()),
	}
	if s.AutoMode {
		nodeRoleARN := s.AutoNodeRoleARN
		if nodeRoleARN == "" {
			// computeConfig.nodeRoleArn has no Ref/Selector in the provider, so
			// this cannot be wired declaratively. A placeholder is honest: it
			// blocks apply via vet-placeholders until the real ARN is supplied.
			nodeRoleARN = Placeholder
		}
		// All three toggles must be present and agree, or AWS rejects the update.
		fp["computeConfig"] = map[string]any{
			"enabled":     true,
			"nodePools":   toAnySlice(s.NodePools),
			"nodeRoleArn": nodeRoleARN,
		}
		fp["storageConfig"] = map[string]any{"blockStorage": map[string]any{"enabled": true}}
		fp["kubernetesNetworkConfig"] = map[string]any{
			"elasticLoadBalancing": map[string]any{"enabled": true},
		}
	}
	g.add("cluster", "eks", apiEKSv1beta2, "Cluster", s.Name,
		map[string]string{"cluster": s.Name}, g.providerSpec(fp))
}

func (g *generator) nodeGroups() {
	for _, ng := range g.spec.NodeGroups {
		u, err := GenerateNodeGroup(g.spec.clusterContext(), ng)
		if err != nil {
			if g.err == nil {
				g.err = err
			}
			return
		}
		g.units = append(g.units, u)
	}
}

func (g *generator) addons() {
	for _, a := range g.spec.Addons {
		u, err := GenerateAddon(g.spec.clusterContext(), a)
		if err != nil {
			if g.err == nil {
				g.err = err
			}
			return
		}
		g.units = append(g.units, u)
	}
}

// ClusterContext is the cluster-level information a single generated resource
// needs. It is derived either from a ClusterSpec (create cluster) or read back
// out of ConfigHub from an existing cluster (create nodegroup / create addon),
// so both paths emit byte-identical resources and cannot drift apart.
type ClusterContext struct {
	Name           string
	Region         string
	Version        string
	Environment    string
	ProviderConfig string
	Orphan         bool
}

func (s ClusterSpec) clusterContext() ClusterContext {
	return ClusterContext{
		Name: s.Name, Region: s.Region, Version: s.Version,
		Environment: s.Environment, ProviderConfig: s.ProviderConfig, Orphan: s.Orphan,
	}
}

func (cc ClusterContext) providerSpec(forProvider map[string]any) map[string]any {
	spec := map[string]any{"forProvider": forProvider}
	if cc.ProviderConfig != "" {
		spec["providerConfigRef"] = map[string]any{"name": cc.ProviderConfig}
	}
	if cc.Orphan {
		spec["deletionPolicy"] = "Orphan"
	}
	return spec
}

func (cc ClusterContext) tags() map[string]string {
	tags := map[string]string{"ManagedBy": "confighub", "Cluster": cc.Name}
	if cc.Environment != "" {
		tags["Environment"] = cc.Environment
	}
	return tags
}

// GenerateNodeGroup builds one managed node group for an existing cluster.
// Subnets are chosen by label selector rather than by id, and the cluster and
// node role by reference, so the Unit is valid regardless of apply order.
func GenerateNodeGroup(cc ClusterContext, ng NodeGroupSpec) (GeneratedUnit, error) {
	if ng.Name == "" {
		return GeneratedUnit{}, fmt.Errorf("node group name is required")
	}
	if cc.Name == "" || cc.Region == "" {
		return GeneratedUnit{}, fmt.Errorf("cluster name and region are required")
	}
	fp := map[string]any{
		"region":         cc.Region,
		"clusterNameRef": map[string]any{"name": cc.Name},
		"nodeRoleArnRef": map[string]any{"name": cc.Name + "-node-role"},
		"subnetIdSelector": map[string]any{
			"matchLabels": toAny(map[string]string{"cluster": cc.Name, "tier": "private"}),
		},
		"version":       cc.Version,
		"instanceTypes": toAnySlice(ng.InstanceTypes),
		"capacityType":  ng.CapacityType,
		"diskSize":      ng.DiskSize,
		"scalingConfig": map[string]any{
			"minSize":     ng.MinSize,
			"maxSize":     ng.MaxSize,
			"desiredSize": ng.DesiredSize,
		},
		"updateConfig": map[string]any{"maxUnavailablePercentage": 25},
		"tags":         toAny(cc.tags()),
	}
	if ng.AMIType != "" {
		fp["amiType"] = ng.AMIType
	}
	return buildUnit("nodegroup-"+ng.Name, "eks", apiEKSv1beta2, "NodeGroup", ng.Name,
		map[string]string{"cluster": cc.Name}, cc.providerSpec(fp), cc.Name)
}

// GenerateAddon builds one EKS addon for an existing cluster.
func GenerateAddon(cc ClusterContext, a AddonSpec) (GeneratedUnit, error) {
	if a.Name == "" {
		return GeneratedUnit{}, fmt.Errorf("addon name is required")
	}
	if cc.Name == "" || cc.Region == "" {
		return GeneratedUnit{}, fmt.Errorf("cluster name and region are required")
	}
	fp := map[string]any{
		"region":         cc.Region,
		"clusterNameRef": map[string]any{"name": cc.Name},
		"addonName":      a.Name,
		// ConfigHub is the source of record, so out-of-band edits to addon
		// configuration are drift to be overwritten, not state to preserve.
		"resolveConflictsOnCreate": "OVERWRITE",
		"resolveConflictsOnUpdate": "OVERWRITE",
		// Keep the addon's Kubernetes objects if the addon record is deleted —
		// removing CoreDNS or the CNI outright is an outage.
		"preserve": true,
	}
	if a.Version != "" {
		fp["addonVersion"] = a.Version
	}
	return buildUnit("addon-"+a.Name, "eks", apiEKSv1beta1, "Addon", a.Name,
		map[string]string{"cluster": cc.Name}, cc.providerSpec(fp), cc.Name)
}

func (g *generator) awsTags() map[string]string {
	return g.spec.clusterContext().tags()
}

// --- helpers ---

// parseVPCCIDR requires a /16 base, which is what the /19 subnet split assumes.
func parseVPCCIDR(cidr string) (base string, prefix int, err error) {
	i := strings.LastIndex(cidr, "/")
	if i < 0 {
		return "", 0, fmt.Errorf("--vpc-cidr %q is not a CIDR block (e.g. 10.20.0.0/16)", cidr)
	}
	prefix, err = strconv.Atoi(cidr[i+1:])
	if err != nil {
		return "", 0, fmt.Errorf("--vpc-cidr %q has a non-numeric prefix length", cidr)
	}
	if prefix != 16 {
		return "", 0, fmt.Errorf("--vpc-cidr %q must be a /16 (the subnet plan splits it into 8 x /19)", cidr)
	}
	octets := strings.Split(cidr[:i], ".")
	if len(octets) != 4 {
		return "", 0, fmt.Errorf("--vpc-cidr %q is not an IPv4 CIDR block", cidr)
	}
	for _, o := range octets {
		n, err := strconv.Atoi(o)
		if err != nil || n < 0 || n > 255 {
			return "", 0, fmt.Errorf("--vpc-cidr %q is not an IPv4 CIDR block", cidr)
		}
	}
	return cidr[:i], prefix, nil
}

// subnetCIDR carves the index'th /19 out of a /16 base. A /19 is 8192 addresses,
// i.e. 32 in the third octet, so a /16 yields exactly 8 of them.
func subnetCIDR(vpcCIDR string, index int) (string, error) {
	base, _, err := parseVPCCIDR(vpcCIDR)
	if err != nil {
		return "", err
	}
	if index < 0 || index > 7 {
		return "", fmt.Errorf("subnet index %d out of range for a /16 (0-7)", index)
	}
	octets := strings.Split(base, ".")
	return fmt.Sprintf("%s.%s.%d.0/19", octets[0], octets[1], index*32), nil
}

// azSuffix reduces an availability zone to its distinguishing suffix
// ("us-east-1a" -> "a") for use in resource names.
func azSuffix(region, az string) string {
	if s := strings.TrimPrefix(az, region); s != az && s != "" {
		return s
	}
	return az
}

func toAny(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func toAnySlice(s []string) []any {
	out := make([]any, 0, len(s))
	for _, v := range s {
		out = append(out, v)
	}
	return out
}
