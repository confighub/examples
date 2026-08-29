// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package snapshot loads the fleet-wide view of Kubernetes RBAC config and assembles it into the
// rbac analysis model.
//
// Reading the fleet is cubapi.SnapshotLoader's job. What is tool-specific is which
// resource types the model needs and what a resource becomes once it arrives.
package snapshot

import (
	"context"

	"github.com/confighub/sdk/core/cubapi"

	"github.com/confighub/examples/rbac-manager-for-agents/internal/rbac"
)

// UnitMeta is the per-Unit metadata joined onto resources.
type UnitMeta = cubapi.UnitMeta

// ClusterNone is the cluster key for Units the fleet view cannot attribute to a
// cluster.
const ClusterNone = cubapi.ClusterNone

// Snapshot is a fleet-wide RBAC view.
type Snapshot struct {
	// Clusters holds the analysis entities per cluster, excluding canonical
	// (base/policy) definitions.
	Clusters map[string]*rbac.ClusterRbac
	// Resources is every parsed resource, including canonical ones.
	Resources []rbac.FleetResource
	// Units is in-scope Unit metadata by UnitID.
	Units map[string]UnitMeta
	// Filter is the ConfigHub Unit `where` predicate the snapshot was scoped by
	// (empty = the whole fleet the user can view).
	Filter string `json:"filter,omitempty"`
}

// loader names the resource types the RBAC model needs: the roles and bindings themselves,
// and the ServiceAccounts a binding can name.
var loader = cubapi.SnapshotLoader[rbac.FleetResource]{
	ResourceTypes: []string{
		"rbac.authorization.k8s.io/v1/Role",
		"rbac.authorization.k8s.io/v1/ClusterRole",
		"rbac.authorization.k8s.io/v1/RoleBinding",
		"rbac.authorization.k8s.io/v1/ClusterRoleBinding",
		"v1/ServiceAccount",
	},
	New: func(o cubapi.Origin, doc map[string]any) rbac.FleetResource {
		return rbac.FleetResource{
			Origin: rbac.ResourceOrigin{
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
	var forAnalysis []rbac.FleetResource
	for _, r := range snap.Resources {
		if !r.Origin.Canonical {
			forAnalysis = append(forAnalysis, r)
		}
	}
	return &Snapshot{
		Clusters:  rbac.BuildClusterRbac(forAnalysis),
		Resources: snap.Resources,
		Units:     snap.Units,
		Filter:    snap.Filter,
	}, nil
}
