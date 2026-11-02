// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cli

// This tool's guardrail pack. The mechanism -- policy Space, Triggers, the group
// selection on each in-scope Space, and reporting the Units a Trigger marked
// -- is managerkit/guardrails'. What is this tool's is the rules, and which
// Units the annotate command has something to say about.

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/confighub/sdk/core/cubapi"
	"github.com/confighub/sdk/core/function/api"
	"github.com/confighub/sdk/managerkit/guardrails"

	"github.com/confighub/examples/workload-manager/internal/cub"
	"github.com/confighub/examples/workload-manager/internal/snapshot"
	"github.com/confighub/examples/workload-manager/internal/workload"
)

// pdbCoverageAnnotation is written by `guardrails annotate` onto each uncovered
// multi-replica workload Unit, and read by the pdb-coverage rule
// (annotate-then-validate -- the one finding a per-Unit CEL check cannot see).
const pdbCoverageAnnotation = "workload.confighub.com/pdb-coverage"

const controllerKinds = "['Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet']"

const (
	celHasLimits = "!(r.kind in " + controllerKinds + ") || (has(r.spec.template.spec.containers) && r.spec.template.spec.containers.all(c, has(c.resources) && has(c.resources.limits) && 'memory' in c.resources.limits))"

	celRunsNonRoot = "!(r.kind in " + controllerKinds + ") || (has(r.spec.template.spec.securityContext) && has(r.spec.template.spec.securityContext.runAsNonRoot) && r.spec.template.spec.securityContext.runAsNonRoot) || (has(r.spec.template.spec.containers) && r.spec.template.spec.containers.all(c, has(c.securityContext) && has(c.securityContext.runAsNonRoot) && c.securityContext.runAsNonRoot))"

	celTerminationMsg = "!(r.kind in " + controllerKinds + ") || (has(r.spec.template.spec.containers) && r.spec.template.spec.containers.all(c, has(c.terminationMessagePolicy) && c.terminationMessagePolicy == 'FallbackToLogsOnError'))"

	// Annotate-then-validate: warn while a pdb-coverage finding annotation is
	// present. The null check is required: a resource written with a bare
	// `annotations:` key has the key present with a null value, so has() is true
	// but `in` against null raises "no such overload" and the rule reports a
	// spurious failure.
	celNoPDBFinding = "!has(r.metadata.annotations) || r.metadata.annotations == null || !('" + pdbCoverageAnnotation + "' in r.metadata.annotations)"
)

// databaseImage matches an image of a common database server by its repository
// name, the last path segment before any image tag or digest, so registry
// mirrors (bitnami/postgresql, docker.io/library/mysql) match too.
const databaseImage = `^([^/]+/)*(postgres|postgresql|postgis|timescaledb|timescaledb-ha|mysql|mariadb|percona-server|mongo|mongodb|cassandra|scylla|cockroach|couchdb|neo4j|clickhouse-server)([:@].*)?$`

// celDatabaseStateful fails a Deployment or ReplicaSet that runs a database
// image. Their Pods are interchangeable, so a rollout can run two database
// Pods on one volume, or start a replacement before the old one lets go of it;
// a StatefulSet gives each replica a stable identity and its own volume.
const celDatabaseStateful = "!(r.kind in ['Deployment', 'ReplicaSet']) || !has(r.spec.template.spec.containers) || !r.spec.template.spec.containers.exists(c, has(c.image) && c.image.matches('" + databaseImage + "'))"

// untaggedOrLatestImage matches an image reference whose image tag is latest, or
// that has neither an image tag nor a digest, where the runtime pulls latest. The
// last path segment is checked alone, so a registry host's port is not mistaken
// for an image tag.
const untaggedOrLatestImage = `(:latest$)|^([^/]+/)*[^/:@]+$`

var pack = guardrails.Pack{
	Group: "workload",
	Rules: []guardrails.Rule{
		{Slug: "workload-has-limits", Description: "Warns on a controller whose containers don't all set resources.limits.memory. Fix: `cub-workload set-resources <space>/<unit>`.", Expression: celHasLimits},
		{Slug: "workload-runs-nonroot", Description: "Warns on a controller not running as non-root (pod or all containers). Fix: `cub-workload harden <space>/<unit>`.", Expression: celRunsNonRoot},
		{Slug: "workload-termination-message-policy", Description: "Warns on a controller whose containers don't all set terminationMessagePolicy: FallbackToLogsOnError. Fix: the termination-message-policy profile.", Expression: celTerminationMsg},
		{Slug: "workload-explicit-image-tag", Description: "Warns on a container image with no image tag or digest, or with the latest image tag. Fix: set a version with `cub function set set-image-reference-by-uri <repository> :<version>`.",
			Function: "vet-images", Arguments: []api.FunctionArgument{{ParameterName: "image-filter", Value: `{"DenyRegexp": "` + untaggedOrLatestImage + `"}`}}},
		{Slug: "workload-database-statefulset", Description: "Warns on a Deployment or ReplicaSet running a database image (postgres, mysql, mariadb, mongo, cassandra, ...). Fix: run it as a StatefulSet, with volumeClaimTemplates for its data.", Expression: celDatabaseStateful},
		{Slug: "workload-pdb-coverage", Description: "Warns while a workload.confighub.com/pdb-coverage annotation is present (set by `cub-workload guardrails annotate`). Fix: `cub-workload ensure-pdb`, then re-run annotate.", Expression: celNoPDBFinding},
	},
}

func preflight(ctx context.Context) (*cubapi.Client, error) { return cub.Preflight(ctx) }

func newGuardrailsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardrails",
		Short: "Install, inspect, and feed the workload-readiness guardrail policy pack",
		Long: `guardrails manages a pack of workload-readiness validation policies, defined once
in a policy Space and enforced in each Space that opts into the group:

  workload-has-limits                controller containers set a memory limit
  workload-runs-nonroot              controller runs as non-root
  workload-termination-message-policy  containers set FallbackToLogsOnError
  workload-explicit-image-tag        images name an image tag other than latest, or a digest
  workload-database-statefulset      database images run in a StatefulSet or DaemonSet
  workload-pdb-coverage              warns while a PDB-coverage finding is annotated

The first five are plain per-resource checks (vet-cel, and vet-images for image
tags) — a single Unit answers them, no annotation needed. The last realizes
annotate-then-validate for the one property vet-cel can't see under
one-resource-per-Unit: whether a *matching* PDB exists in some other Unit. 'guardrails annotate' writes that finding onto each
uncovered workload Unit and this Trigger turns it into a warning.

Triggers are created with Warn=true (advisory ValidationWarnings, never blocking).
Promote one to blocking later with:
  cub trigger update <slug> --space <policy-space> --unwarn`,
	}
	cmd.AddCommand(pack.InstallCmd(preflight), pack.StatusCmd(preflight), pack.AnnotateCmd(preflight, annotateSpec))
	return cmd
}

// annotateSpec is the workload half of `guardrails annotate`: which Units have
// a coverage finding, and what to write on them. The rest of the command is
// managerkit/guardrails'.
var annotateSpec = guardrails.AnnotateSpec{
	Key:   pdbCoverageAnnotation,
	Noun:  "workload Unit",
	Short: "Annotate each uncovered multi-replica workload Unit with a PDB-coverage finding (dry-run unless --commit)",
	Long: `annotate runs the availability analysis and writes a workload.confighub.com/
pdb-coverage annotation onto each multi-replica workload Unit that has no matching
PodDisruptionBudget. Paired with the workload-pdb-coverage rule from
'guardrails install', this turns the cross-Unit coverage finding — the one a
per-Unit rule can't compute — into an advisory ValidationWarning.

Re-run after adding PDBs. Dry run unless --commit --change-desc.`,
	Targets: func(ctx context.Context, c *cubapi.Client, where, cluster string) ([]guardrails.Target, error) {
		snap, err := snapshot.Load(ctx, c, where)
		if err != nil {
			return nil, err
		}
		var out []guardrails.Target
		for _, r := range workload.AnalyzeAvailability(snap.Clusters) {
			if r.HasPDB || r.SpaceID == "" || r.UnitSlug == "" {
				continue // covered, or no Unit to annotate
			}
			if cluster != "" && r.Cluster != cluster {
				continue
			}
			out = append(out, guardrails.Target{
				SpaceID: r.SpaceID, UnitSlug: r.UnitSlug, Value: "uncovered",
				Cluster: r.Cluster, Namespace: r.Namespace,
			})
		}
		return out, nil
	},
}
