// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package snapshot loads the fleet-wide view of the Kubernetes config the namespace envelope covers and assembles it into the
// nsmanager analysis model.
//
// Reading the fleet is cubapi.SnapshotLoader's job. What is tool-specific is which
// resource types the model needs and what a resource becomes once it arrives.
package snapshot

import (
	"context"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/namespace-manager/internal/nsmanager"
)

// UnitMeta is the per-Unit metadata joined onto resources.
type UnitMeta = cubapi.UnitMeta

// ClusterNone is the cluster key for Units the fleet view cannot attribute to a
// cluster.
const ClusterNone = cubapi.ClusterNone

// Snapshot is a fleet-wide namespace-envelope view.
type Snapshot struct {
	// Clusters holds the analysis entities per cluster, excluding canonical
	// (base/policy) definitions.
	Clusters map[string]*nsmanager.ClusterNamespaces
	// Resources is every parsed resource, including canonical ones.
	Resources []nsmanager.FleetResource
	// Units is in-scope Unit metadata by UnitID.
	Units map[string]UnitMeta
	// Filter is the ConfigHub Unit `where` predicate the snapshot was scoped by
	// (empty = the whole fleet the user can view).
	Filter string `json:"filter,omitempty"`
}

// loader names the resource types the envelope model needs: the namespace itself and
// everything the envelope is meant to contain.
var loader = cubapi.SnapshotLoader[nsmanager.FleetResource]{
	ResourceTypes: []string{
		"v1/Namespace",
		"apps/v1/Deployment",
		"apps/v1/StatefulSet",
		"apps/v1/DaemonSet",
		"apps/v1/ReplicaSet",
		"batch/v1/Job",
		"batch/v1/CronJob",
		"batch/v1beta1/CronJob",
		"v1/Pod",
	},
	New: func(o cubapi.Origin, doc map[string]any) nsmanager.FleetResource {
		return nsmanager.FleetResource{
			Origin: nsmanager.ResourceOrigin{
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
	var forAnalysis []nsmanager.FleetResource
	for _, r := range snap.Resources {
		if !r.Origin.Canonical {
			forAnalysis = append(forAnalysis, r)
		}
	}
	return &Snapshot{
		Clusters:  nsmanager.BuildFleet(forAnalysis),
		Resources: snap.Resources,
		Units:     snap.Units,
		Filter:    snap.Filter,
	}, nil
}
