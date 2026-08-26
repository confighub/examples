// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package snapshot loads the fleet-wide view of the Crossplane managed
// resources that make up EKS clusters and assembles it into the eks analysis
// model.
//
// Reading the fleet is managerkit/fleet's job. What is EKS-specific is which
// resources count as EKS config and which cluster each one belongs to -- and for
// this tool that is not the Target the config is delivered to.
package snapshot

import (
	"context"
	"regexp"
	"strings"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/eks-manager/internal/eks"
	"github.com/confighub/examples/managerkit/fleet"
)

// UnitMeta is the per-Unit metadata joined onto resources.
type UnitMeta = fleet.UnitMeta

// ClusterNone is the cluster key for Units this tool cannot attribute to a
// cluster.
const ClusterNone = fleet.ClusterNone

// SpaceLabelCluster is the Space label naming the EKS cluster a Space describes.
const SpaceLabelCluster = "Cluster"

// Snapshot is a fleet-wide EKS view.
type Snapshot struct {
	// Clusters holds the EKS entities per cluster, excluding canonical
	// (base/policy) definitions.
	Clusters map[string]*eks.ClusterSet
	// Resources is every parsed resource, including canonical ones.
	Resources []eks.FleetResource
	// Units is in-scope Unit metadata by UnitID.
	Units map[string]UnitMeta
	// Filter is the ConfigHub Unit `where` predicate the snapshot was scoped by
	// (empty = the whole fleet the user can view).
	Filter string `json:"filter,omitempty"`
}

// resourceTypePatterns match the ResourceTypes the EKS model needs, one per
// Crossplane API group.
//
// They key off the API group rather than an enumeration of kinds. That matters:
// a Crossplane provider ships hundreds of kinds per group and adds more every
// release, so a list of exact types would silently go stale -- whereas the group
// prefix picks up a new EKS kind for free. The model buckets whatever comes
// back, and unrecognized kinds land in the generic inventory rather than being
// dropped.
//
// Adding a group (say rds, for a sibling tool) is one line here.
var resourceTypePatterns = []string{
	`eks\.aws\.upbound\.io/.+`,
	`ec2\.aws\.upbound\.io/.+`,
	`iam\.aws\.upbound\.io/.+`,
}

// resourceTypeMatch is the exact test, applied to what comes back.
var resourceTypeMatch = regexp.MustCompile(`(?i)^(` + strings.Join(resourceTypePatterns, "|") + `)$`)

// resourceTypeWhere asks the server for the same union in one clause. A filter
// literal cannot carry a backslash, so the escapes are dropped -- an unescaped
// `.` matches any character, which makes the clause broader than the patterns,
// never narrower, and resourceTypeMatch narrows it again on the way out.
var resourceTypeWhere = "ResourceType ~* '^(" +
	strings.ReplaceAll(strings.Join(resourceTypePatterns, "|"), `\`, "") + ")$'"

// clusterKey names the EKS cluster a Unit's resources belong to: the Space's
// Cluster label when set, else ClusterNone. Deliberately NOT the Target slug --
// the Target is the Crossplane management cluster these resources are applied
// to, which is a different cluster from the one they describe.
//
// A Space carrying no Cluster label describes no cluster this tool can name, so
// its Units group under ClusterNone rather than the Space slug standing in for a
// cluster of its own.
func clusterKey(meta UnitMeta) string {
	if v := meta.SpaceLabels[SpaceLabelCluster]; v != "" {
		return v
	}
	return ClusterNone
}

var loader = fleet.Loader[eks.FleetResource]{
	ResourceWhere: resourceTypeWhere,
	Keep:          func(o fleet.Origin) bool { return resourceTypeMatch.MatchString(o.ResourceType) },
	ClusterKey:    clusterKey,
	New: func(o fleet.Origin, doc map[string]any) eks.FleetResource {
		return eks.FleetResource{
			Origin: eks.ResourceOrigin{
				Cluster:      o.Cluster,
				Space:        o.Space,
				SpaceID:      o.SpaceID,
				SpaceLabels:  o.SpaceLabels,
				Target:       o.Target,
				UnitID:       o.UnitID,
				UnitSlug:     o.UnitSlug,
				ResourceName: o.ResourceName,
				Canonical:    o.Canonical,
			},
			Doc: doc,
		}
	},
}

// Load fetches and assembles the fleet snapshot, scoped by a single ConfigHub
// Unit `where` predicate (empty = everything the user can view).
func Load(ctx context.Context, c *cubapi.Client, where string) (*Snapshot, error) {
	snap, err := loader.Load(ctx, c, where)
	if err != nil {
		return nil, err
	}
	// Canonical definitions stay out of cluster analysis.
	var forAnalysis []eks.FleetResource
	for _, r := range snap.Resources {
		if !r.Origin.Canonical {
			forAnalysis = append(forAnalysis, r)
		}
	}
	return &Snapshot{
		Clusters:  eks.BuildFleet(forAnalysis),
		Resources: snap.Resources,
		Units:     snap.Units,
		Filter:    snap.Filter,
	}, nil
}
