// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package cub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/confighub/sdk/core/cubapi"
	api "github.com/confighub/sdk/core/function/api"

	"github.com/confighub/examples/namespace-manager/internal/nsmanager"
)

// VetPodSecurityFunction is the server function that checks workloads against a
// Pod Security Standard, with the same checks the PodSecurity admission
// controller runs.
const VetPodSecurityFunction = "vet-pod-security-standard"

// unitsPerInvocation bounds the UnitID IN (...) list of one invocation.
const unitsPerInvocation = 100

// VetPodSecurity runs vet-pod-security-standard at level (and version, "" for
// latest) over the Units with the given IDs, and returns the workloads that fail,
// keyed by Unit and resource, with the checks each fails. A Unit the function
// could not check is added to unchecked, keyed by ID, with the reason, rather
// than failing the rest. It is read-only: the function validates and every
// invocation is a dry run.
func VetPodSecurity(ctx context.Context, c *cubapi.Client, level, version string, unitIDs []string, unchecked map[string]string) (map[nsmanager.WorkloadKey][]nsmanager.PodSecurityCheck, error) {
	args := []api.FunctionArgument{{ParameterName: "level", Value: level}}
	if version != "" {
		args = append(args, api.FunctionArgument{ParameterName: "version", Value: version})
	}
	failing := map[nsmanager.WorkloadKey][]nsmanager.PodSecurityCheck{}
	for start := 0; start < len(unitIDs); start += unitsPerInvocation {
		chunk := unitIDs[start:min(start+unitsPerInvocation, len(unitIDs))]
		where := cubapi.Where{}.InStrings("UnitID", chunk)
		if err := where.Err(); err != nil {
			return nil, err
		}
		res, err := cubapi.InvokeFunction(ctx, c,
			api.FunctionInvocation{FunctionName: VetPodSecurityFunction, Arguments: args},
			cubapi.Selector{Where: where.String()}, cubapi.Change{})
		if err != nil {
			return nil, explainMissingFunction(err)
		}
		for _, o := range res.Outcomes {
			if !o.Success {
				if isMissingFunction(o.Error) {
					return nil, explainMissingFunction(fmt.Errorf("%s on %s/%s: %s", VetPodSecurityFunction, o.SpaceSlug, o.UnitSlug, o.Error))
				}
				unchecked[o.UnitID] = o.Error
				continue
			}
			results, err := validationResults(o.Outputs)
			if err != nil {
				return nil, fmt.Errorf("%s on %s/%s: %w", VetPodSecurityFunction, o.SpaceSlug, o.UnitSlug, err)
			}
			for _, vr := range results {
				for _, fa := range vr.FailedAttributes {
					key := nsmanager.WorkloadKey{UnitID: o.UnitID, ResourceName: string(fa.ResourceName)}
					for _, issue := range fa.Issues {
						failing[key] = append(failing[key], nsmanager.PodSecurityCheck{Reason: issue.Identifier, Message: issue.Message})
					}
				}
			}
		}
	}
	return failing, nil
}

// validationResults decodes a validating function's output, which the server
// returns base64-encoded under the ValidationResult or ValidationResultList key,
// as one result or a list of them.
func validationResults(outputs map[string]string) ([]api.ValidationResult, error) {
	var out []api.ValidationResult
	for _, key := range []api.OutputType{api.OutputTypeValidationResultList, api.OutputTypeValidationResult} {
		encoded := outputs[string(key)]
		if encoded == "" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode function output: %w", err)
		}
		var list api.ValidationResultList
		if err := json.Unmarshal(raw, &list); err == nil {
			out = append(out, list...)
			continue
		}
		var vr api.ValidationResult
		if err := json.Unmarshal(raw, &vr); err != nil {
			return nil, fmt.Errorf("decode function output: %w", err)
		}
		out = append(out, vr)
	}
	return out, nil
}

// isMissingFunction reports whether a server error says it has no
// vet-pod-security-standard function.
func isMissingFunction(msg string) bool {
	msg = strings.ToLower(msg)
	return strings.Contains(msg, VetPodSecurityFunction) && (strings.Contains(msg, "does not exist") || strings.Contains(msg, "not found"))
}

// explainMissingFunction says what to do when the server predates the function,
// which otherwise surfaces as a bare "does not exist".
func explainMissingFunction(err error) error {
	if isMissingFunction(err.Error()) {
		return fmt.Errorf("%w\nthis ConfigHub server has no %s function; pod-security needs a server that does", err, VetPodSecurityFunction)
	}
	return err
}
