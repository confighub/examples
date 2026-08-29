// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package snapshot loads the fleet-wide view of the Kubernetes config relevant
// to NetworkPolicy and assembles it into the netpol analysis model.
//
// Reading the fleet is cubapi.SnapshotLoader's job. What is netpol-specific is which
// resource types the coverage model needs and what a resource becomes once it
// arrives: coverage is a join across types -- a NetworkPolicy means nothing
// without the pods its podSelector matches -- so all four families are pulled.
package snapshot

import (
	"context"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/network-policy-manager/internal/netpol"
)

// UnitMeta is the per-Unit metadata joined onto resources.
type UnitMeta = cubapi.UnitMeta

// ClusterNone is the cluster key for Units whose Space has no release Target.
const ClusterNone = cubapi.ClusterNone

// Snapshot is a fleet-wide NetworkPolicy view.
type Snapshot struct {
	// Clusters holds NetworkPolicy-relevant entities per cluster, excluding
	// canonical (base/policy) definitions.
	Clusters map[string]*netpol.ClusterNetpol
	// Resources is every parsed resource, including canonical ones, for the
	// explorer.
	Resources []netpol.FleetResource
	// Units is in-scope Unit metadata by UnitID.
	Units map[string]UnitMeta
	// Filter is the ConfigHub Unit `where` predicate the snapshot was scoped by
	// (empty = the whole fleet the user can view).
	Filter string `json:"filter,omitempty"`
}

// loader names the resource types the coverage model needs: the policies
// themselves, the namespaces they live in, everything that carries a pod
// template for their selectors to match, and the Services that name it.
var loader = cubapi.SnapshotLoader[netpol.FleetResource]{
	ResourceTypes: []string{
		"networking.k8s.io/v1/NetworkPolicy",
		"v1/Namespace",
		"apps/v1/Deployment",
		"apps/v1/StatefulSet",
		"apps/v1/DaemonSet",
		"apps/v1/ReplicaSet",
		"batch/v1/Job",
		"batch/v1/CronJob",
		"batch/v1beta1/CronJob",
		"v1/Pod",
		"v1/Service",
	},
	New: func(o cubapi.Origin, doc map[string]any) netpol.FleetResource {
		return netpol.FleetResource{
			Origin: netpol.ResourceOrigin{
				Cluster:      o.Cluster,
				Target:       o.Target,
				Space:        o.Space,
				SpaceID:      o.SpaceID,
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
	var forAnalysis []netpol.FleetResource
	for _, r := range snap.Resources {
		if !r.Origin.Canonical {
			forAnalysis = append(forAnalysis, r)
		}
	}
	return &Snapshot{
		Clusters:  netpol.BuildFleet(forAnalysis),
		Resources: snap.Resources,
		Units:     snap.Units,
		Filter:    snap.Filter,
	}, nil
}
