// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package snapshot loads the fleet-wide view of the config autoscaling analysis depends on and assembles it into the
// autoscale analysis model.
//
// Reading the fleet is cubapi.SnapshotLoader's job. What is tool-specific is which
// resource types the model needs and what a resource becomes once it arrives.
package snapshot

import (
	"context"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/autoscale-manager/internal/autoscale"
)

// UnitMeta is the per-Unit metadata joined onto resources.
type UnitMeta = cubapi.UnitMeta

// ClusterNone is the cluster key for Units the fleet view cannot attribute to a
// cluster.
const ClusterNone = cubapi.ClusterNone

// Snapshot is a fleet-wide autoscaling view.
type Snapshot struct {
	// Clusters holds the analysis entities per cluster, excluding canonical
	// (base/policy) definitions.
	Clusters map[string]*autoscale.ClusterAutoscale
	// Resources is every parsed resource, including canonical ones.
	Resources []autoscale.FleetResource
	// Units is in-scope Unit metadata by UnitID.
	Units map[string]UnitMeta
	// Filter is the ConfigHub Unit `where` predicate the snapshot was scoped by
	// (empty = the whole fleet the user can view).
	Filter string `json:"filter,omitempty"`
}

// loader names the resource types the autoscaling model needs: the autoscalers, their scale
// targets, and the PodDisruptionBudgets that can block a scale-down.
var loader = cubapi.SnapshotLoader[autoscale.FleetResource]{
	ResourceTypes: []string{
		"autoscaling/v1/HorizontalPodAutoscaler",
		"autoscaling/v2/HorizontalPodAutoscaler",
		"autoscaling/v2beta1/HorizontalPodAutoscaler",
		"autoscaling/v2beta2/HorizontalPodAutoscaler",
		"keda.sh/v1alpha1/ScaledObject",
		"apps/v1/Deployment",
		"apps/v1/StatefulSet",
		"policy/v1/PodDisruptionBudget",
		"policy/v1beta1/PodDisruptionBudget",
	},
	New: func(o cubapi.Origin, doc map[string]any) autoscale.FleetResource {
		return autoscale.FleetResource{
			Origin: autoscale.ResourceOrigin{
				Cluster:      o.Cluster,
				Target:       o.Target,
				Space:        o.Space,
				SpaceID:      o.SpaceID,
				SpaceLabels:  o.SpaceLabels,
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
	var forAnalysis []autoscale.FleetResource
	for _, r := range snap.Resources {
		if !r.Origin.Canonical {
			forAnalysis = append(forAnalysis, r)
		}
	}
	return &Snapshot{
		Clusters:  autoscale.BuildFleet(forAnalysis),
		Resources: snap.Resources,
		Units:     snap.Units,
		Filter:    snap.Filter,
	}, nil
}
