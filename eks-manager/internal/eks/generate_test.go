// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package eks

import (
	"encoding/json"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func baseSpec() ClusterSpec {
	return ClusterSpec{
		Name:           "demo",
		Region:         "us-east-1",
		Version:        "1.34",
		Environment:    "dev",
		AutoMode:       true,
		NodePools:      []string{"general-purpose", "system"},
		Zones:          DefaultZones("us-east-1", 3),
		VPCCIDR:        "10.20.0.0/16",
		NAT:            NATSingle,
		ProviderConfig: "default",
	}
}

// parse decodes a generated Unit's YAML back into the model, so tests assert on
// what the parser will actually see rather than on string matching.
func parseUnits(t *testing.T, units []GeneratedUnit) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, u := range units {
		var doc map[string]any
		if err := yaml.Unmarshal([]byte(u.YAML), &doc); err != nil {
			t.Fatalf("unit %s: generated invalid YAML: %v\n%s", u.Slug, err, u.YAML)
		}
		out[u.Slug] = doc
	}
	return out
}

func TestGenerate_AutoModeEnvelope(t *testing.T) {
	units, err := Generate(baseSpec())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	docs := parseUnits(t, units)

	// Auto Mode emits no node groups and no addons — that is the point of it.
	for slug := range docs {
		if strings.HasPrefix(slug, "nodegroup-") || strings.HasPrefix(slug, "addon-") {
			t.Errorf("Auto Mode generated %s, want none", slug)
		}
	}

	cluster, ok := docs["cluster"]
	if !ok {
		t.Fatal("no cluster Unit generated")
	}
	fp := cluster["spec"].(map[string]any)["forProvider"].(map[string]any)

	// The Auto Mode tuple must be complete and agree, or AWS rejects the update.
	cc := fp["computeConfig"].(map[string]any)
	sc := fp["storageConfig"].(map[string]any)["blockStorage"].(map[string]any)
	elb := fp["kubernetesNetworkConfig"].(map[string]any)["elasticLoadBalancing"].(map[string]any)
	if cc["enabled"] != true || sc["enabled"] != true || elb["enabled"] != true {
		t.Errorf("Auto Mode tuple not all true: compute=%v storage=%v elb=%v",
			cc["enabled"], sc["enabled"], elb["enabled"])
	}
	// Round-tripping through the model must agree.
	var doc any
	b, _ := json.Marshal(cluster)
	_ = json.Unmarshal(b, &doc)
	fleet := BuildFleet([]FleetResource{{Origin: ResourceOrigin{Cluster: "demo"}, Doc: doc}})
	if am := fleet["demo"].Control.AutoMode; !am.Enabled() || !am.Consistent() {
		t.Errorf("model reads generated Auto Mode as enabled=%v consistent=%v", am.Enabled(), am.Consistent())
	}

	// ForceNew, and Auto Mode requires it false; getting it wrong at creation
	// makes Auto Mode permanently unreachable.
	if fp["bootstrapSelfManagedAddons"] != false {
		t.Errorf("bootstrapSelfManagedAddons = %v, want false", fp["bootstrapSelfManagedAddons"])
	}
	// A pinned version must be paired with EXTENDED support or AWS auto-upgrades
	// and fights the pin.
	if up := fp["upgradePolicy"].(map[string]any); up["supportType"] != "EXTENDED" {
		t.Errorf("supportType = %v, want EXTENDED alongside a pinned version", up["supportType"])
	}
	if fp["version"] != "1.34" {
		t.Errorf("version = %v", fp["version"])
	}
}

// The one value that cannot be a reference must be a placeholder, so
// vet-placeholders blocks apply rather than the cluster silently coming up wrong.
func TestGenerate_AutoNodeRoleARNPlaceholder(t *testing.T) {
	units, err := Generate(baseSpec())
	if err != nil {
		t.Fatal(err)
	}
	docs := parseUnits(t, units)
	cc := docs["cluster"]["spec"].(map[string]any)["forProvider"].(map[string]any)["computeConfig"].(map[string]any)
	if cc["nodeRoleArn"] != Placeholder {
		t.Errorf("nodeRoleArn = %v, want %s", cc["nodeRoleArn"], Placeholder)
	}

	s := baseSpec()
	s.AutoNodeRoleARN = "arn:aws:iam::111122223333:role/Real"
	units, _ = Generate(s)
	docs = parseUnits(t, units)
	cc = docs["cluster"]["spec"].(map[string]any)["forProvider"].(map[string]any)["computeConfig"].(map[string]any)
	if cc["nodeRoleArn"] != "arn:aws:iam::111122223333:role/Real" {
		t.Errorf("explicit ARN not used: %v", cc["nodeRoleArn"])
	}
}

// This is the invariant that makes the generated set safe to apply in any order.
func TestGenerate_NoLiteralIdentifiers(t *testing.T) {
	units, err := Generate(baseSpec())
	if err != nil {
		t.Fatal(err)
	}
	// Fields that must never carry a literal value: they identify another
	// resource and must be expressed as a Ref or Selector.
	banned := []string{
		"\n    vpcId:", "\n    subnetIds:", "\n    roleArn:", "\n    clusterName:",
		"\n    nodeRoleArn:", "\n    routeTableId:", "\n    subnetId:",
		"\n    allocationId:", "\n    natGatewayId:", "\n    gatewayId:", "\n    policyArn:",
	}
	for _, u := range units {
		for _, b := range banned {
			// policyArn is legitimately a literal: AWS managed policy ARNs are
			// well-known constants, not references to resources we manage.
			if strings.Contains(b, "policyArn") {
				continue
			}
			if strings.Contains(u.YAML, b) {
				t.Errorf("unit %s emits literal %q; must use a Ref or Selector\n%s",
					u.Slug, strings.TrimSpace(b), u.YAML)
			}
		}
	}
}

func TestGenerate_SubnetCIDRPlan(t *testing.T) {
	units, err := Generate(baseSpec())
	if err != nil {
		t.Fatal(err)
	}
	docs := parseUnits(t, units)
	want := map[string]string{
		"subnet-public-a":  "10.20.0.0/19",
		"subnet-public-b":  "10.20.32.0/19",
		"subnet-public-c":  "10.20.64.0/19",
		"subnet-private-a": "10.20.96.0/19",
		"subnet-private-b": "10.20.128.0/19",
		"subnet-private-c": "10.20.160.0/19",
	}
	for slug, cidr := range want {
		d, ok := docs[slug]
		if !ok {
			t.Errorf("missing %s", slug)
			continue
		}
		fp := d["spec"].(map[string]any)["forProvider"].(map[string]any)
		if fp["cidrBlock"] != cidr {
			t.Errorf("%s cidrBlock = %v, want %s", slug, fp["cidrBlock"], cidr)
		}
	}
	// Public and private subnets must not overlap; the private block starts
	// after all the public ones so adding an AZ never renumbers existing subnets.
	if docs["subnet-private-a"]["spec"].(map[string]any)["forProvider"].(map[string]any)["cidrBlock"] == "10.20.64.0/19" {
		t.Error("private subnet overlaps the public range")
	}
}

func TestSubnetCIDR(t *testing.T) {
	for i, want := range []string{
		"10.20.0.0/19", "10.20.32.0/19", "10.20.64.0/19", "10.20.96.0/19",
		"10.20.128.0/19", "10.20.160.0/19", "10.20.192.0/19", "10.20.224.0/19",
	} {
		got, err := subnetCIDR("10.20.0.0/16", i)
		if err != nil || got != want {
			t.Errorf("subnetCIDR(%d) = %q, %v; want %q", i, got, err, want)
		}
	}
	if _, err := subnetCIDR("10.20.0.0/16", 8); err == nil {
		t.Error("index 8 should overflow a /16")
	}
	if _, err := subnetCIDR("10.20.0.0/24", 0); err == nil {
		t.Error("a /24 base should be rejected")
	}
	if _, err := subnetCIDR("not-a-cidr", 0); err == nil {
		t.Error("garbage should be rejected")
	}
}

func TestGenerate_NATStrategies(t *testing.T) {
	count := func(units []GeneratedUnit, prefix string) int {
		n := 0
		for _, u := range units {
			if strings.HasPrefix(u.Slug, prefix) {
				n++
			}
		}
		return n
	}

	s := baseSpec()
	s.NAT = NATSingle
	single, _ := Generate(s)
	if got := count(single, "nat-"); got != 1 {
		t.Errorf("single: %d NAT gateways, want 1", got)
	}
	if got := count(single, "eip-nat-"); got != 1 {
		t.Errorf("single: %d EIPs, want 1", got)
	}
	// Every private subnet still gets its own route table and a default route.
	if got := count(single, "rt-private-"); got != 3 {
		t.Errorf("single: %d private route tables, want 3", got)
	}
	if got := count(single, "route-private-"); got != 3 {
		t.Errorf("single: %d private default routes, want 3", got)
	}

	s.NAT = NATPerAZ
	perAZ, _ := Generate(s)
	if got := count(perAZ, "nat-"); got != 3 {
		t.Errorf("per-az: %d NAT gateways, want 3", got)
	}

	s.NAT = NATNone
	none, _ := Generate(s)
	if got := count(none, "nat-"); got != 0 {
		t.Errorf("none: %d NAT gateways, want 0", got)
	}
	if got := count(none, "route-private-"); got != 0 {
		t.Errorf("none: %d private default routes, want 0", got)
	}
	// Route tables still exist, so adding NAT later is a route edit.
	if got := count(none, "rt-private-"); got != 3 {
		t.Errorf("none: %d private route tables, want 3", got)
	}
}

// Under single NAT every AZ egresses through the one gateway; under per-az each
// routes to its own.
func TestGenerate_PrivateRoutesTargetCorrectNAT(t *testing.T) {
	s := baseSpec()
	s.NAT = NATSingle
	units, _ := Generate(s)
	docs := parseUnits(t, units)
	for _, az := range []string{"a", "b", "c"} {
		fp := docs["route-private-"+az+"-nat"]["spec"].(map[string]any)["forProvider"].(map[string]any)
		ref := fp["natGatewayIdRef"].(map[string]any)
		if ref["name"] != "demo-nat-a" {
			t.Errorf("single NAT: az %s routes to %v, want demo-nat-a", az, ref["name"])
		}
	}

	s.NAT = NATPerAZ
	units, _ = Generate(s)
	docs = parseUnits(t, units)
	for _, az := range []string{"a", "b", "c"} {
		fp := docs["route-private-"+az+"-nat"]["spec"].(map[string]any)["forProvider"].(map[string]any)
		ref := fp["natGatewayIdRef"].(map[string]any)
		if ref["name"] != "demo-nat-"+az {
			t.Errorf("per-az NAT: az %s routes to %v, want demo-nat-%s", az, ref["name"], az)
		}
	}
}

func TestGenerate_ClassicMode(t *testing.T) {
	s := baseSpec()
	s.AutoMode = false
	s.NodeGroups = []NodeGroupSpec{{
		Name: "system", InstanceTypes: []string{"m6i.large"}, CapacityType: "ON_DEMAND",
		MinSize: 2, MaxSize: 6, DesiredSize: 2, DiskSize: 80,
	}}
	s.Addons = []AddonSpec{{Name: "vpc-cni"}, {Name: "coredns", Version: "v1.11.1-eksbuild.1"}}

	units, err := Generate(s)
	if err != nil {
		t.Fatal(err)
	}
	docs := parseUnits(t, units)

	if _, ok := docs["nodegroup-system"]; !ok {
		t.Fatal("no node group generated in classic mode")
	}
	cluster := docs["cluster"]["spec"].(map[string]any)["forProvider"].(map[string]any)
	if _, ok := cluster["computeConfig"]; ok {
		t.Error("classic mode emitted computeConfig")
	}

	ng := docs["nodegroup-system"]["spec"].(map[string]any)["forProvider"].(map[string]any)
	scaling := ng["scalingConfig"].(map[string]any)
	if scaling["minSize"] != float64(2) || scaling["maxSize"] != float64(6) {
		t.Errorf("scalingConfig = %v", scaling)
	}
	// Node groups select their subnets by label rather than listing IDs.
	sel := ng["subnetIdSelector"].(map[string]any)["matchLabels"].(map[string]any)
	if sel["tier"] != "private" || sel["cluster"] != "demo" {
		t.Errorf("subnetIdSelector = %v", sel)
	}

	addon := docs["addon-coredns"]["spec"].(map[string]any)["forProvider"].(map[string]any)
	if addon["addonVersion"] != "v1.11.1-eksbuild.1" {
		t.Errorf("addonVersion = %v", addon["addonVersion"])
	}
	// ConfigHub is the source of record, so out-of-band addon edits are drift.
	if addon["resolveConflictsOnUpdate"] != "OVERWRITE" {
		t.Errorf("resolveConflictsOnUpdate = %v, want OVERWRITE", addon["resolveConflictsOnUpdate"])
	}
	// Removing CoreDNS or the CNI outright is an outage.
	if addon["preserve"] != true {
		t.Errorf("preserve = %v, want true", addon["preserve"])
	}
	// An addon with no explicit version must omit the field, not send "".
	if _, ok := docs["addon-vpc-cni"]["spec"].(map[string]any)["forProvider"].(map[string]any)["addonVersion"]; ok {
		t.Error("unversioned addon emitted an addonVersion key")
	}
}

func TestGenerate_IAMRoles(t *testing.T) {
	// IAM is global: these resources must not carry a region.
	units, _ := Generate(baseSpec())
	docs := parseUnits(t, units)
	for slug, d := range docs {
		u := findUnit(units, slug)
		if u.Group != "iam" {
			continue
		}
		fp := d["spec"].(map[string]any)["forProvider"].(map[string]any)
		if _, ok := fp["region"]; ok {
			t.Errorf("%s carries a region, but IAM is global", slug)
		}
	}

	// Auto Mode needs sts:TagSession in the cluster role's trust policy.
	trust := docs["cluster-role"]["spec"].(map[string]any)["forProvider"].(map[string]any)["assumeRolePolicy"].(string)
	if !strings.Contains(trust, "sts:TagSession") {
		t.Errorf("Auto Mode cluster trust policy lacks sts:TagSession: %s", trust)
	}

	s := baseSpec()
	s.AutoMode = false
	classic, _ := Generate(s)
	cdocs := parseUnits(t, classic)
	ctrust := cdocs["cluster-role"]["spec"].(map[string]any)["forProvider"].(map[string]any)["assumeRolePolicy"].(string)
	if strings.Contains(ctrust, "sts:TagSession") {
		t.Errorf("classic cluster trust policy should not need sts:TagSession")
	}

	// Policy attachments differ by mode.
	autoAttach := countPrefix(units, "cluster-role-")
	classicAttach := countPrefix(classic, "cluster-role-")
	if autoAttach != len(autoModeClusterPolicies) {
		t.Errorf("auto mode cluster attachments = %d, want %d", autoAttach, len(autoModeClusterPolicies))
	}
	if classicAttach != len(classicClusterPolicies) {
		t.Errorf("classic cluster attachments = %d, want %d", classicAttach, len(classicClusterPolicies))
	}

	// AWS moved off ReadOnly, and the CNI policy belongs on an IRSA role.
	for _, u := range classic {
		if strings.Contains(u.YAML, "AmazonEC2ContainerRegistryReadOnly") {
			t.Errorf("%s uses the superseded ReadOnly ECR policy", u.Slug)
		}
		if strings.Contains(u.YAML, "AmazonEKS_CNI_Policy") {
			t.Errorf("%s attaches AmazonEKS_CNI_Policy to a node role; it belongs on an IRSA role", u.Slug)
		}
	}
}

// Same input, same output — the property eksctl does not have, since it
// randomizes AZ selection per invocation.
func TestGenerate_Deterministic(t *testing.T) {
	a, err := Generate(baseSpec())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(baseSpec())
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) {
		t.Fatalf("unit counts differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Slug != b[i].Slug || a[i].YAML != b[i].YAML {
			t.Errorf("unit %d differs between runs", i)
		}
	}
}

func TestDefaultZones(t *testing.T) {
	got := DefaultZones("us-east-1", 3)
	want := []string{"us-east-1a", "us-east-1b", "us-east-1c"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DefaultZones = %v, want %v", got, want)
		}
	}
	if len(DefaultZones("eu-west-2", 2)) != 2 {
		t.Error("zone count not honored")
	}
}

func TestGenerate_Validation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ClusterSpec)
	}{
		{"no name", func(s *ClusterSpec) { s.Name = "" }},
		{"no region", func(s *ClusterSpec) { s.Region = "" }},
		{"no version", func(s *ClusterSpec) { s.Version = "" }},
		{"bad version", func(s *ClusterSpec) { s.Version = "latest" }},
		{"no zones", func(s *ClusterSpec) { s.Zones = nil }},
		{"too many zones", func(s *ClusterSpec) { s.Zones = DefaultZones("us-east-1", 5) }},
		{"bad nat", func(s *ClusterSpec) { s.NAT = "sometimes" }},
		{"non-/16 cidr", func(s *ClusterSpec) { s.VPCCIDR = "10.20.0.0/20" }},
		// Auto Mode manages capacity itself; a node group alongside it is a
		// contradiction worth rejecting rather than silently ignoring.
		{"auto mode with node groups", func(s *ClusterSpec) {
			s.NodeGroups = []NodeGroupSpec{{Name: "x"}}
		}},
	}
	for _, tt := range tests {
		s := baseSpec()
		tt.mutate(&s)
		if _, err := Generate(s); err == nil {
			t.Errorf("%s: expected an error", tt.name)
		}
	}
	// The base spec itself must be valid.
	if _, err := Generate(baseSpec()); err != nil {
		t.Errorf("base spec rejected: %v", err)
	}
}

// Every generated Unit must hold exactly one resource, per one-resource-per-Unit.
func TestGenerate_OneResourcePerUnit(t *testing.T) {
	units, _ := Generate(baseSpec())
	seen := map[string]bool{}
	for _, u := range units {
		if strings.Contains(u.YAML, "\n---") {
			t.Errorf("unit %s contains multiple documents", u.Slug)
		}
		if seen[u.Slug] {
			t.Errorf("duplicate unit slug %s", u.Slug)
		}
		seen[u.Slug] = true
		if u.Slug == "" || u.Kind == "" || u.Group == "" {
			t.Errorf("unit with empty metadata: %+v", u)
		}
	}
	if len(units) < 15 {
		t.Errorf("only %d units generated; expected a full envelope", len(units))
	}
}

func findUnit(units []GeneratedUnit, slug string) GeneratedUnit {
	for _, u := range units {
		if u.Slug == slug {
			return u
		}
	}
	return GeneratedUnit{}
}

func countPrefix(units []GeneratedUnit, prefix string) int {
	n := 0
	for _, u := range units {
		if strings.HasPrefix(u.Slug, prefix) && u.Kind == "RolePolicyAttachment" {
			n++
		}
	}
	return n
}

// create cluster and create nodegroup must emit byte-identical resources for the
// same inputs. They are separate entry points, so without a shared code path
// they would drift — a node group added later would differ from one created with
// the cluster in ways nobody would notice until reconciliation behaved oddly.
func TestGenerate_SingleResourceMatchesWholeCluster(t *testing.T) {
	ngSpec := NodeGroupSpec{
		Name: "system", InstanceTypes: []string{"m6i.large"}, CapacityType: "ON_DEMAND",
		MinSize: 2, MaxSize: 6, DesiredSize: 2, DiskSize: 80,
	}
	addonSpec := AddonSpec{Name: "coredns", Version: "v1.11.1-eksbuild.1"}

	s := baseSpec()
	s.AutoMode = false
	s.NodeGroups = []NodeGroupSpec{ngSpec}
	s.Addons = []AddonSpec{addonSpec}
	whole, err := Generate(s)
	if err != nil {
		t.Fatal(err)
	}

	cc := s.clusterContext()
	standaloneNG, err := GenerateNodeGroup(cc, ngSpec)
	if err != nil {
		t.Fatal(err)
	}
	standaloneAddon, err := GenerateAddon(cc, addonSpec)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []GeneratedUnit{standaloneNG, standaloneAddon} {
		got := findUnit(whole, want.Slug)
		if got.Slug == "" {
			t.Errorf("%s not present in the whole-cluster output", want.Slug)
			continue
		}
		if got.YAML != want.YAML {
			t.Errorf("%s differs between create-cluster and single-resource generation:\n--- cluster ---\n%s\n--- standalone ---\n%s",
				want.Slug, got.YAML, want.YAML)
		}
		if got.Kind != want.Kind || got.Group != want.Group {
			t.Errorf("%s metadata differs: %+v vs %+v", want.Slug, got, want)
		}
	}
}

func TestGenerateNodeGroup_Validation(t *testing.T) {
	cc := ClusterContext{Name: "c", Region: "us-east-1", Version: "1.34"}
	if _, err := GenerateNodeGroup(cc, NodeGroupSpec{}); err == nil {
		t.Error("empty node group name accepted")
	}
	if _, err := GenerateNodeGroup(ClusterContext{Region: "us-east-1"}, NodeGroupSpec{Name: "x"}); err == nil {
		t.Error("missing cluster name accepted")
	}
	if _, err := GenerateNodeGroup(ClusterContext{Name: "c"}, NodeGroupSpec{Name: "x"}); err == nil {
		t.Error("missing region accepted")
	}
	if _, err := GenerateAddon(cc, AddonSpec{}); err == nil {
		t.Error("empty addon name accepted")
	}
}

// An addon with no explicit version must omit the field entirely, letting EKS
// choose the default compatible with the cluster's Kubernetes version.
func TestGenerateAddon_VersionOptional(t *testing.T) {
	cc := ClusterContext{Name: "c", Region: "us-east-1", Version: "1.34"}
	u, err := GenerateAddon(cc, AddonSpec{Name: "vpc-cni"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(u.YAML, "addonVersion") {
		t.Errorf("unversioned addon emitted addonVersion:\n%s", u.YAML)
	}
	u, err = GenerateAddon(cc, AddonSpec{Name: "vpc-cni", Version: "v1.19.5"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u.YAML, "addonVersion: v1.19.5") {
		t.Errorf("versioned addon missing addonVersion:\n%s", u.YAML)
	}
}
