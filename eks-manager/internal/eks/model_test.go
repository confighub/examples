// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package eks

import (
	"encoding/json"
	"testing"
)

// doc parses a JSON literal into the shape BuildFleet receives (get-resources
// returns JSON bodies, so numbers arrive as float64).
func doc(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return v
}

func res(t *testing.T, cluster, s string) FleetResource {
	t.Helper()
	return FleetResource{Origin: ResourceOrigin{Cluster: cluster}, Doc: doc(t, s)}
}

func TestIsClusterScopedGroup(t *testing.T) {
	tests := []struct {
		group string
		want  bool
	}{
		{"eks.aws.upbound.io", true},
		{"ec2.aws.upbound.io", true},
		{"aws.upbound.io", true},
		{"gcp.upbound.io", true},
		{"pkg.crossplane.io", true},
		{"apiextensions.crossplane.io", true},
		// The .m. infix marks the Crossplane v2 namespaced variants.
		{"eks.aws.m.upbound.io", false},
		{"ec2.aws.m.upbound.io", false},
		{"pkg.m.crossplane.io", false},
		// Not Crossplane at all.
		{"apps", false},
		{"", false},
		{"eks.services.k8s.aws", false},
		// User-defined composite resources are not classified by the rule.
		{"platform.acme.io", false},
	}
	for _, tt := range tests {
		if got := IsClusterScopedGroup(tt.group); got != tt.want {
			t.Errorf("IsClusterScopedGroup(%q) = %v, want %v", tt.group, got, tt.want)
		}
	}
}

func TestSplitAPIVersion(t *testing.T) {
	tests := []struct{ in, group, version string }{
		{"eks.aws.upbound.io/v1beta2", "eks.aws.upbound.io", "v1beta2"},
		{"v1", "", "v1"},
		{"apps/v1", "apps", "v1"},
		{"", "", ""},
	}
	for _, tt := range tests {
		g, v := SplitAPIVersion(tt.in)
		if g != tt.group || v != tt.version {
			t.Errorf("SplitAPIVersion(%q) = (%q, %q), want (%q, %q)", tt.in, g, v, tt.group, tt.version)
		}
	}
}

// singleton must accept both shapes upjet emits for a MaxItems:1 block: the
// embedded object of v1beta2 and the one-element list of the deprecated v1beta1.
func TestSingleton(t *testing.T) {
	if got := singleton(map[string]any{"a": "b"}); got == nil || got["a"] != "b" {
		t.Errorf("object shape not accepted: %v", got)
	}
	if got := singleton([]any{map[string]any{"a": "b"}}); got == nil || got["a"] != "b" {
		t.Errorf("list shape not accepted: %v", got)
	}
	for name, v := range map[string]any{
		"nil":         nil,
		"empty list":  []any{},
		"scalar":      "x",
		"list scalar": []any{"x"},
	} {
		if got := singleton(v); got != nil {
			t.Errorf("singleton(%s) = %v, want nil", name, got)
		}
	}
}

// The same cluster expressed on both API versions must parse identically. This
// is the shape divergence upjet introduced with the singleton-list conversion.
func TestParseCluster_BothAPIShapes(t *testing.T) {
	v1beta2 := `{
      "apiVersion": "eks.aws.upbound.io/v1beta2", "kind": "Cluster",
      "metadata": {"name": "prod-use1"},
      "spec": {
        "deletionPolicy": "Orphan",
        "managementPolicies": ["Observe","Create","Update","Delete"],
        "forProvider": {
          "region": "us-east-1", "version": "1.34",
          "enabledClusterLogTypes": ["api","audit"],
          "vpcConfig": {"endpointPublicAccess": false, "endpointPrivateAccess": true,
                        "publicAccessCidrs": ["10.0.0.0/8"]},
          "accessConfig": {"authenticationMode": "API"},
          "upgradePolicy": {"supportType": "EXTENDED"},
          "encryptionConfig": {"provider": {"keyArn": "arn:aws:kms:::key/abc"}}
        }
      }}`
	// Deprecated cluster-scoped v1beta1: the same blocks are one-element lists.
	v1beta1 := `{
      "apiVersion": "eks.aws.upbound.io/v1beta1", "kind": "Cluster",
      "metadata": {"name": "prod-use1"},
      "spec": {
        "deletionPolicy": "Orphan",
        "managementPolicies": ["Observe","Create","Update","Delete"],
        "forProvider": {
          "region": "us-east-1", "version": "1.34",
          "enabledClusterLogTypes": ["api","audit"],
          "vpcConfig": [{"endpointPublicAccess": false, "endpointPrivateAccess": true,
                         "publicAccessCidrs": ["10.0.0.0/8"]}],
          "accessConfig": [{"authenticationMode": "API"}],
          "upgradePolicy": [{"supportType": "EXTENDED"}],
          "encryptionConfig": [{"provider": [{"keyArn": "arn:aws:kms:::key/abc"}]}]
        }
      }}`

	for _, src := range []struct{ name, body string }{{"v1beta2", v1beta2}, {"v1beta1", v1beta1}} {
		fleet := BuildFleet([]FleetResource{res(t, "prod-use1", src.body)})
		c := fleet["prod-use1"].Control
		if c == nil {
			t.Fatalf("%s: no control plane parsed", src.name)
		}
		if c.Version != "1.34" || c.Region != "us-east-1" {
			t.Errorf("%s: version/region = %q/%q", src.name, c.Version, c.Region)
		}
		if c.EndpointPublicAccess == nil || *c.EndpointPublicAccess {
			t.Errorf("%s: endpointPublicAccess = %v, want false", src.name, c.EndpointPublicAccess)
		}
		if c.EndpointPrivateAccess == nil || !*c.EndpointPrivateAccess {
			t.Errorf("%s: endpointPrivateAccess = %v, want true", src.name, c.EndpointPrivateAccess)
		}
		if got := len(c.PublicAccessCIDRs); got != 1 {
			t.Errorf("%s: publicAccessCidrs len = %d, want 1", src.name, got)
		}
		if c.AuthenticationMode != "API" {
			t.Errorf("%s: authenticationMode = %q", src.name, c.AuthenticationMode)
		}
		if c.UpgradeSupportType != "EXTENDED" {
			t.Errorf("%s: upgradeSupportType = %q", src.name, c.UpgradeSupportType)
		}
		if !c.EncryptionConfigured {
			t.Errorf("%s: encryptionConfigured = false, want true", src.name)
		}
		if c.DeletionPolicy != "Orphan" {
			t.Errorf("%s: deletionPolicy = %q", src.name, c.DeletionPolicy)
		}
		if len(c.ManagementPolicies) != 4 {
			t.Errorf("%s: managementPolicies = %v", src.name, c.ManagementPolicies)
		}
	}
}

func TestParseCluster_ExternalNameAnnotationWins(t *testing.T) {
	body := `{"apiVersion":"eks.aws.upbound.io/v1beta2","kind":"Cluster",
	  "metadata":{"name":"unit-slug","annotations":{"crossplane.io/external-name":"real-aws-name"}},
	  "spec":{"forProvider":{"region":"us-east-1"}}}`
	c := BuildFleet([]FleetResource{res(t, "c", body)})["c"].Control
	if c.Name != "unit-slug" {
		t.Errorf("Name = %q, want the Kubernetes object name", c.Name)
	}
	if c.ExternalName != "real-aws-name" {
		t.Errorf("ExternalName = %q, want the annotation value", c.ExternalName)
	}

	// Without the annotation, external-name defaults to metadata.name.
	body = `{"apiVersion":"eks.aws.upbound.io/v1beta2","kind":"Cluster",
	  "metadata":{"name":"prod"},"spec":{"forProvider":{}}}`
	c = BuildFleet([]FleetResource{res(t, "c", body)})["c"].Control
	if c.ExternalName != "prod" {
		t.Errorf("ExternalName = %q, want %q", c.ExternalName, "prod")
	}
}

func TestAutoMode(t *testing.T) {
	tests := []struct {
		name                              string
		compute, elb, storage             *bool
		wantDeclared, wantOK, wantEnabled bool
	}{
		{"absent entirely", nil, nil, nil, false, true, false},
		{"all true", ptr(true), ptr(true), ptr(true), true, true, true},
		{"all false", ptr(false), ptr(false), ptr(false), true, true, false},
		// AWS rejects a partial or disagreeing tuple outright.
		{"partial", ptr(true), nil, nil, true, false, false},
		{"disagreeing", ptr(true), ptr(true), ptr(false), true, false, false},
		{"two of three", ptr(true), ptr(true), nil, true, false, false},
	}
	for _, tt := range tests {
		a := AutoMode{ComputeEnabled: tt.compute, LoadBalancingEnabled: tt.elb, BlockStorageEnabled: tt.storage}
		if got := a.Declared(); got != tt.wantDeclared {
			t.Errorf("%s: Declared() = %v, want %v", tt.name, got, tt.wantDeclared)
		}
		if got := a.Consistent(); got != tt.wantOK {
			t.Errorf("%s: Consistent() = %v, want %v", tt.name, got, tt.wantOK)
		}
		if got := a.Enabled(); got != tt.wantEnabled {
			t.Errorf("%s: Enabled() = %v, want %v", tt.name, got, tt.wantEnabled)
		}
	}
}

func TestParseCluster_AutoMode(t *testing.T) {
	body := `{"apiVersion":"eks.aws.upbound.io/v1beta2","kind":"Cluster",
	  "metadata":{"name":"auto"},
	  "spec":{"forProvider":{
	    "bootstrapSelfManagedAddons": false,
	    "computeConfig":{"enabled":true,"nodePools":["general-purpose","system"],
	                     "nodeRoleArn":"arn:aws:iam::1:role/AutoNode"},
	    "storageConfig":{"blockStorage":{"enabled":true}},
	    "kubernetesNetworkConfig":{"elasticLoadBalancing":{"enabled":true}}}}}`
	c := BuildFleet([]FleetResource{res(t, "c", body)})["c"].Control
	if !c.AutoMode.Enabled() {
		t.Fatalf("AutoMode.Enabled() = false, want true (%+v)", c.AutoMode)
	}
	if !c.AutoMode.Consistent() {
		t.Errorf("AutoMode.Consistent() = false")
	}
	if len(c.AutoMode.NodePools) != 2 {
		t.Errorf("NodePools = %v", c.AutoMode.NodePools)
	}
	if c.AutoMode.NodeRoleARN == "" {
		t.Errorf("NodeRoleARN empty")
	}
	if c.BootstrapSelfManagedAddons == nil || *c.BootstrapSelfManagedAddons {
		t.Errorf("bootstrapSelfManagedAddons = %v, want false", c.BootstrapSelfManagedAddons)
	}
}

func TestParseNodeGroup(t *testing.T) {
	body := `{"apiVersion":"eks.aws.upbound.io/v1beta2","kind":"NodeGroup",
	  "metadata":{"name":"system-2026a"},
	  "spec":{"forProvider":{
	    "region":"us-east-1",
	    "clusterNameRef":{"name":"prod-use1"},
	    "version":"1.34","releaseVersion":"1.34.0-20260701",
	    "amiType":"AL2023_x86_64_STANDARD","capacityType":"ON_DEMAND",
	    "instanceTypes":["m6i.large"],"diskSize":100,
	    "scalingConfig":{"minSize":3,"maxSize":10,"desiredSize":3},
	    "launchTemplate":{"id":"lt-0abc","version":"3"}}}}`
	n := BuildFleet([]FleetResource{res(t, "c", body)})["c"].NodeGroups[0]

	if n.ClusterName != "prod-use1" {
		t.Errorf("ClusterName = %q, want resolution through clusterNameRef", n.ClusterName)
	}
	if n.AMIType != "AL2023_x86_64_STANDARD" || n.CapacityType != "ON_DEMAND" {
		t.Errorf("amiType/capacityType = %q/%q", n.AMIType, n.CapacityType)
	}
	if len(n.InstanceTypes) != 1 || n.InstanceTypes[0] != "m6i.large" {
		t.Errorf("InstanceTypes = %v", n.InstanceTypes)
	}
	if n.DiskSize == nil || *n.DiskSize != 100 {
		t.Errorf("DiskSize = %v", n.DiskSize)
	}
	if n.MinSize == nil || *n.MinSize != 3 || n.MaxSize == nil || *n.MaxSize != 10 {
		t.Errorf("scaling = %v..%v", n.MinSize, n.MaxSize)
	}
	if n.DesiredSize == nil || *n.DesiredSize != 3 {
		t.Errorf("DesiredSize = %v", n.DesiredSize)
	}
	if n.DesiredSizeInInitProvider {
		t.Errorf("DesiredSizeInInitProvider = true, want false (it is under forProvider)")
	}
	if n.LaunchTemplateID != "lt-0abc" || n.LaunchTemplateVersion != "3" {
		t.Errorf("launchTemplate = %q/%q", n.LaunchTemplateID, n.LaunchTemplateVersion)
	}
}

// desiredSize under initProvider is the documented pattern for externally-scaled
// node groups; the model must distinguish it from forProvider.
func TestParseNodeGroup_DesiredSizeInInitProvider(t *testing.T) {
	body := `{"apiVersion":"eks.aws.upbound.io/v1beta2","kind":"NodeGroup",
	  "metadata":{"name":"ng"},
	  "spec":{
	    "managementPolicies":["Observe","Create","Update","Delete"],
	    "initProvider":{"scalingConfig":{"desiredSize":3}},
	    "forProvider":{"scalingConfig":{"minSize":3,"maxSize":10}}}}`
	n := BuildFleet([]FleetResource{res(t, "c", body)})["c"].NodeGroups[0]
	if !n.DesiredSizeInInitProvider {
		t.Errorf("DesiredSizeInInitProvider = false, want true")
	}
	if n.DesiredSize == nil || *n.DesiredSize != 3 {
		t.Errorf("DesiredSize = %v, want 3 carried from initProvider", n.DesiredSize)
	}
	if n.MinSize == nil || *n.MinSize != 3 {
		t.Errorf("MinSize = %v", n.MinSize)
	}
}

// launchTemplate.version is legitimately a string ("$Latest") or a number.
func TestParseNodeGroup_LaunchTemplateVersionScalar(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{`"$Latest"`, "$Latest"},
		{`3`, "3"},
	} {
		body := `{"apiVersion":"eks.aws.upbound.io/v1beta2","kind":"NodeGroup",
		  "metadata":{"name":"ng"},
		  "spec":{"forProvider":{"launchTemplate":{"name":"lt","version":` + tc.raw + `}}}}`
		n := BuildFleet([]FleetResource{res(t, "c", body)})["c"].NodeGroups[0]
		if n.LaunchTemplateVersion != tc.want {
			t.Errorf("version %s parsed as %q, want %q", tc.raw, n.LaunchTemplateVersion, tc.want)
		}
	}
}

func TestRefName(t *testing.T) {
	// A literal value wins when the reference has already been resolved.
	fp := map[string]any{"clusterName": "literal", "clusterNameRef": map[string]any{"name": "viaref"}}
	if got := refName(fp, "clusterName"); got != "literal" {
		t.Errorf("literal: got %q", got)
	}
	// Otherwise fall back to the *Ref.
	fp = map[string]any{"clusterNameRef": map[string]any{"name": "viaref"}}
	if got := refName(fp, "clusterName"); got != "viaref" {
		t.Errorf("ref: got %q", got)
	}
	// A *Selector cannot be resolved from one resource.
	fp = map[string]any{"clusterNameSelector": map[string]any{"matchLabels": map[string]any{"a": "b"}}}
	if got := refName(fp, "clusterName"); got != "" {
		t.Errorf("selector: got %q, want empty", got)
	}
	if got := refName(map[string]any{}, "clusterName"); got != "" {
		t.Errorf("absent: got %q", got)
	}
}

func TestParseAddon(t *testing.T) {
	body := `{"apiVersion":"eks.aws.upbound.io/v1beta1","kind":"Addon",
	  "metadata":{"name":"vpc-cni"},
	  "spec":{"forProvider":{"region":"us-east-1","clusterNameRef":{"name":"prod-use1"},
	    "addonName":"vpc-cni","addonVersion":"v1.19.5-eksbuild.1",
	    "resolveConflictsOnUpdate":"OVERWRITE","preserve":true}}}`
	a := BuildFleet([]FleetResource{res(t, "c", body)})["c"].Addons[0]
	if a.AddonName != "vpc-cni" || a.AddonVersion != "v1.19.5-eksbuild.1" {
		t.Errorf("addon = %q/%q", a.AddonName, a.AddonVersion)
	}
	if a.ClusterName != "prod-use1" {
		t.Errorf("ClusterName = %q", a.ClusterName)
	}
	if a.ResolveConflictsOnUpdate != "OVERWRITE" {
		t.Errorf("resolveConflictsOnUpdate = %q", a.ResolveConflictsOnUpdate)
	}
	if a.Preserve == nil || !*a.Preserve {
		t.Errorf("preserve = %v", a.Preserve)
	}
}

func TestBuildFleet_Bucketing(t *testing.T) {
	fleet := BuildFleet([]FleetResource{
		res(t, "prod", `{"apiVersion":"eks.aws.upbound.io/v1beta2","kind":"Cluster","metadata":{"name":"prod"},"spec":{"forProvider":{"version":"1.34"}}}`),
		res(t, "prod", `{"apiVersion":"eks.aws.upbound.io/v1beta2","kind":"NodeGroup","metadata":{"name":"ng1"},"spec":{"forProvider":{}}}`),
		res(t, "prod", `{"apiVersion":"eks.aws.upbound.io/v1beta1","kind":"Addon","metadata":{"name":"cni"},"spec":{"forProvider":{}}}`),
		res(t, "prod", `{"apiVersion":"eks.aws.upbound.io/v1beta1","kind":"PodIdentityAssociation","metadata":{"name":"pia"},"spec":{"forProvider":{}}}`),
		res(t, "prod", `{"apiVersion":"ec2.aws.upbound.io/v1beta1","kind":"VPC","metadata":{"name":"vpc"},"spec":{"forProvider":{"region":"us-east-1"}}}`),
		res(t, "prod", `{"apiVersion":"iam.aws.upbound.io/v1beta1","kind":"Role","metadata":{"name":"role"},"spec":{"forProvider":{}}}`),
		res(t, "dev", `{"apiVersion":"eks.aws.upbound.io/v1beta2","kind":"Cluster","metadata":{"name":"dev"},"spec":{"forProvider":{"version":"1.33"}}}`),
		// Ignored: not a group the model handles.
		res(t, "prod", `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"web"},"spec":{}}`),
	})

	if len(fleet) != 2 {
		t.Fatalf("clusters = %d, want 2", len(fleet))
	}
	p := fleet["prod"]
	if p.Control == nil || p.Control.Version != "1.34" {
		t.Errorf("prod control = %+v", p.Control)
	}
	if len(p.NodeGroups) != 1 || len(p.Addons) != 1 {
		t.Errorf("nodegroups=%d addons=%d", len(p.NodeGroups), len(p.Addons))
	}
	if len(p.OtherEKS) != 1 || p.OtherEKS[0].Kind != "PodIdentityAssociation" {
		t.Errorf("otherEks = %+v", p.OtherEKS)
	}
	if len(p.Network) != 1 || p.Network[0].Kind != "VPC" || p.Network[0].Region != "us-east-1" {
		t.Errorf("network = %+v", p.Network)
	}
	if len(p.IAM) != 1 || p.IAM[0].Kind != "Role" {
		t.Errorf("iam = %+v", p.IAM)
	}
	if fleet["dev"].Control.Version != "1.33" {
		t.Errorf("dev version = %q", fleet["dev"].Control.Version)
	}
}

// A malformed or incomplete resource must be skipped, never fatal — one bad
// Unit cannot be allowed to take down fleet-wide analysis.
func TestBuildFleet_LenientParsing(t *testing.T) {
	fleet := BuildFleet([]FleetResource{
		{Origin: ResourceOrigin{Cluster: "c"}, Doc: "not an object"},
		{Origin: ResourceOrigin{Cluster: "c"}, Doc: nil},
		{Origin: ResourceOrigin{Cluster: "c"}, Doc: []any{1, 2}},
		res(t, "c", `{"kind":"Cluster","metadata":{"name":"x"}}`),                          // no apiVersion
		res(t, "c", `{"apiVersion":"eks.aws.upbound.io/v1beta2","kind":"Cluster"}`),        // no name
		res(t, "c", `{"apiVersion":"eks.aws.upbound.io/v1beta2","metadata":{"name":"x"}}`), // no kind
		// A resource with no spec at all must still parse.
		res(t, "c", `{"apiVersion":"eks.aws.upbound.io/v1beta2","kind":"Cluster","metadata":{"name":"ok"}}`),
	})
	c := fleet["c"]
	if c == nil || c.Control == nil {
		t.Fatalf("the one valid resource was not parsed: %+v", c)
	}
	if c.Control.Name != "ok" {
		t.Errorf("Name = %q, want %q", c.Control.Name, "ok")
	}
	if c.Control.Version != "" {
		t.Errorf("Version = %q, want empty", c.Control.Version)
	}
}

func TestResourceMeta(t *testing.T) {
	av, kind, name, ok := ResourceMeta(doc(t, `{"apiVersion":"eks.aws.upbound.io/v1beta2","kind":"Cluster","metadata":{"name":"p"}}`))
	if !ok || av != "eks.aws.upbound.io/v1beta2" || kind != "Cluster" || name != "p" {
		t.Errorf("got (%q,%q,%q,%v)", av, kind, name, ok)
	}
	if _, _, _, ok := ResourceMeta("nope"); ok {
		t.Errorf("non-object reported ok")
	}
	if _, _, _, ok := ResourceMeta(doc(t, `{"kind":"Cluster"}`)); ok {
		t.Errorf("missing name reported ok")
	}
}

func ptr[T any](v T) *T { return &v }
