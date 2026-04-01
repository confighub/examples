// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/confighub/sdk/configkit/k8skit"
	"github.com/confighub/sdk/core/configkit/yamlkit"
	"github.com/confighub/sdk/core/function/api"
	"github.com/confighub/sdk/core/function/handler"
	"github.com/confighub/sdk/core/third_party/gaby"
)

// kubepugBinary is the name or path of the kubepug CLI binary.
// It can be overridden for testing.
var kubepugBinary = "kubepug"

// kubepugDatabasePath, if set, is passed as --database to the kubepug CLI
// to use a pre-downloaded local copy of the deprecation database instead of
// downloading it on every invocation.
var kubepugDatabasePath string

// GetVetKubepugSignature returns the function signature for vet-kubepug.
func GetVetKubepugSignature() api.FunctionSignature {
	return api.FunctionSignature{
		FunctionName: "vet-kubepug",
		Parameters: []api.FunctionParameter{
			{
				ParameterName: "k8s-version",
				Required:      true,
				Description:   `Target Kubernetes version to check for deprecated and deleted APIs (e.g., "v1.22", "v1.25").`,
				DataType:      api.DataTypeString,
			},
			{
				ParameterName: "score-threshold",
				Required:      true,
				Description:   `Score threshold for validation failure. If any finding has a score at or above this threshold, validation fails. Possible values: "Critical", "High", "Medium", "Low".`,
				DataType:      api.DataTypeString,
			},
		},
		RequiredParameters: 2,
		OutputInfo: &api.FunctionOutput{
			ResultName:  "passed",
			Description: "True if all findings score below the threshold, false otherwise",
			OutputType:  api.OutputTypeValidationResult,
		},
		Mutating:              false,
		Validating:            true,
		Hermetic:              false, // execs kubepug CLI
		Idempotent:            true,
		Description:           "Checks Kubernetes resources for deprecated and deleted APIs using kubepug. See https://github.com/kubepug/kubepug for details.",
		FunctionType:          api.FunctionTypeCustom,
		AffectedResourceTypes: []api.ResourceType{api.ResourceTypeAny},
	}
}

// VetKubepugFunction validates Kubernetes resources using the kubepug CLI.
func VetKubepugFunction(fArgs handler.FunctionImplementationArguments) (gaby.Container, any, error) {
	return vetKubepug(k8skit.NewK8sResourceProvider(), fArgs.ParsedData, fArgs.Arguments)
}

// JSON types matching kubepug JSON output.
type kubepugResult struct {
	DeprecatedAPIs []kubepugResultItem `json:"deprecated_apis"`
	DeletedAPIs    []kubepugResultItem `json:"deleted_apis"`
}

type kubepugResultItem struct {
	Group       string                    `json:"group,omitempty"`
	Kind        string                    `json:"kind,omitempty"`
	Version     string                    `json:"version,omitempty"`
	K8sVersion  string                    `json:"k8sversion,omitempty"`
	Description string                    `json:"description,omitempty"`
	Replacement *kubepugGroupVersionKind  `json:"replacement,omitempty"`
	Items       []kubepugItem             `json:"deleted_items,omitempty"`
}

type kubepugGroupVersionKind struct {
	Group   string `json:"group,omitempty"`
	Version string `json:"version,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

type kubepugItem struct {
	Scope      string `json:"scope,omitempty"`
	ObjectName string `json:"objectname,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Location   string `json:"location,omitempty"`
}

// parseScoreThreshold converts a threshold string to an api.Score.
func parseScoreThreshold(s string) (api.Score, error) {
	switch s {
	case "Critical":
		return api.ScoreCritical, nil
	case "High":
		return api.ScoreHigh, nil
	case "Medium":
		return api.ScoreMedium, nil
	case "Low":
		return api.ScoreLow, nil
	default:
		return api.ScoreNone, fmt.Errorf("invalid score-threshold %q: must be Critical, High, Medium, or Low", s)
	}
}

func vetKubepug(rp *k8skit.K8sResourceProviderType, parsedData gaby.Container, args []api.FunctionArgument) (gaby.Container, any, error) {
	k8sVersion := args[0].Value.(string)
	threshold, err := parseScoreThreshold(args[1].Value.(string))
	if err != nil {
		return parsedData, nil, err
	}

	// Write resources to a temp file.
	resourceFile, err := os.CreateTemp("", "kubepug-resource-*.yaml")
	if err != nil {
		return parsedData, nil, errors.Wrap(err, "failed to create temp resource file")
	}
	defer os.Remove(resourceFile.Name())
	for i, doc := range parsedData {
		if i > 0 {
			resourceFile.WriteString("---\n")
		}
		resourceFile.Write(doc.Bytes())
		resourceFile.WriteString("\n")
	}
	resourceFile.Close()

	// Run kubepug with JSON output.
	cliArgs := []string{
		"--input-file=" + resourceFile.Name(),
		"--k8s-version=" + k8sVersion,
		"--format=json",
		"--error-on-deprecated",
		"--error-on-deleted",
	}
	if kubepugDatabasePath != "" {
		cliArgs = append(cliArgs, "--database="+kubepugDatabasePath)
	}
	cmd := exec.Command(kubepugBinary, cliArgs...)
	output, err := cmd.CombinedOutput()

	// kubepug exits non-zero when deprecated or deleted APIs are found,
	// but still produces valid JSON output. Only treat non-ExitError as real errors.
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return parsedData, nil, errors.Wrapf(err, "failed to execute kubepug CLI (is %q in PATH?)", kubepugBinary)
		}
	}

	// Parse the JSON output. kubepug may print warnings before the JSON
	// and error messages after it on stderr (via CombinedOutput).
	jsonBytes := extractJSON(output)
	if jsonBytes == nil {
		return parsedData, nil, fmt.Errorf("no JSON found in kubepug output: %s", string(output))
	}

	var result kubepugResult
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return parsedData, nil, errors.Wrapf(err, "failed to parse kubepug JSON output: %s", string(output))
	}

	// If no deprecated or deleted APIs found, pass.
	if len(result.DeprecatedAPIs) == 0 && len(result.DeletedAPIs) == 0 {
		return parsedData, api.ValidationResultTrue, nil
	}

	// Build resource info map.
	resourceInfoMap := buildResourceInfoMap(parsedData, rp)

	var maxScore api.Score
	var details []string
	var failedAttributes api.AttributeValueList

	// Process deleted APIs (Critical severity).
	for _, apiItem := range result.DeletedAPIs {
		maxScore = api.ScoreMax(maxScore, api.ScoreCritical)

		replacement := ""
		if apiItem.Replacement != nil {
			replacement = fmt.Sprintf(" (replacement: %s/%s/%s)", apiItem.Replacement.Group, apiItem.Replacement.Version, apiItem.Replacement.Kind)
		}

		for _, item := range apiItem.Items {
			resourceKey := kubepugResourceKey(apiItem, item)
			resourceInfo := lookupResourceInfo(resourceInfoMap, resourceKey)

			detail := fmt.Sprintf("[Critical] API %s/%s/%s deleted in k8s %s%s: %s/%s",
				apiItem.Group, apiItem.Version, apiItem.Kind,
				apiItem.K8sVersion, replacement,
				item.Namespace, item.ObjectName)
			details = append(details, detail)

			failedAttributes = append(failedAttributes, api.AttributeValue{
				AttributeInfo: api.AttributeInfo{
					AttributeIdentifier: api.AttributeIdentifier{
						ResourceInfo: resourceInfo,
						Path:         "apiVersion",
					},
					AttributeMetadata: api.AttributeMetadata{
						AttributeName: api.AttributeNameNone,
						Details: &api.AttributeDetails{
							Description: detail,
						},
					},
				},
				Score: api.ScoreCritical,
				Issues: []api.Issue{
					{
						Identifier: "deleted-api",
						Message:    fmt.Sprintf("API %s/%s deleted in Kubernetes %s%s", apiItem.Version, apiItem.Kind, apiItem.K8sVersion, replacement),
					},
				},
			})
		}
	}

	// Process deprecated APIs (High severity).
	for _, apiItem := range result.DeprecatedAPIs {
		maxScore = api.ScoreMax(maxScore, api.ScoreHigh)

		replacement := ""
		if apiItem.Replacement != nil {
			replacement = fmt.Sprintf(" (replacement: %s/%s/%s)", apiItem.Replacement.Group, apiItem.Replacement.Version, apiItem.Replacement.Kind)
		}

		for _, item := range apiItem.Items {
			resourceKey := kubepugResourceKey(apiItem, item)
			resourceInfo := lookupResourceInfo(resourceInfoMap, resourceKey)

			detail := fmt.Sprintf("[High] API %s/%s/%s deprecated in k8s %s%s: %s/%s",
				apiItem.Group, apiItem.Version, apiItem.Kind,
				apiItem.K8sVersion, replacement,
				item.Namespace, item.ObjectName)
			details = append(details, detail)

			failedAttributes = append(failedAttributes, api.AttributeValue{
				AttributeInfo: api.AttributeInfo{
					AttributeIdentifier: api.AttributeIdentifier{
						ResourceInfo: resourceInfo,
						Path:         "apiVersion",
					},
					AttributeMetadata: api.AttributeMetadata{
						AttributeName: api.AttributeNameNone,
						Details: &api.AttributeDetails{
							Description: detail,
						},
					},
				},
				Score: api.ScoreHigh,
				Issues: []api.Issue{
					{
						Identifier: "deprecated-api",
						Message:    fmt.Sprintf("API %s/%s deprecated in Kubernetes %s%s", apiItem.Version, apiItem.Kind, apiItem.K8sVersion, replacement),
					},
				},
			})
		}
	}

	// Determine pass/fail based on threshold.
	passed := maxScore == api.ScoreNone || api.ScoreMax(maxScore, threshold) != maxScore

	validationResult := api.ValidationResult{
		Passed:           passed,
		MaxScore:         maxScore,
		Details:          details,
		FailedAttributes: failedAttributes,
	}

	return parsedData, validationResult, nil
}

// kubepugResourceKey builds a lookup key from kubepug API and item info.
// Format matches buildResourceInfoMap: "namespace/Kind/name".
func kubepugResourceKey(apiItem kubepugResultItem, item kubepugItem) string {
	ns := item.Namespace
	if ns == "" {
		ns = "default"
	}
	return fmt.Sprintf("%s/%s/%s", ns, apiItem.Kind, item.ObjectName)
}

// buildResourceInfoMap creates a map from "namespace/Kind/name" to ResourceInfo.
func buildResourceInfoMap(parsedData gaby.Container, rp *k8skit.K8sResourceProviderType) map[string]api.ResourceInfo {
	infoMap := make(map[string]api.ResourceInfo)
	visitor := func(_ *gaby.YamlDoc, output any, _ int, resourceInfo *api.ResourceInfo) (any, []error) {
		rt := string(resourceInfo.ResourceType)
		kind := rt
		if idx := strings.LastIndex(rt, "/"); idx >= 0 {
			kind = rt[idx+1:]
		}
		apiVersion := ""
		if idx := strings.LastIndex(rt, "/"); idx >= 0 {
			apiVersion = rt[:idx]
		}

		rn := string(resourceInfo.ResourceName)
		var namespace, name string
		if ns, n, ok := strings.Cut(rn, "/"); ok {
			namespace = ns
			name = n
		} else {
			name = rn
		}

		if namespace == "" && !k8skit.IsClusterScoped(apiVersion, kind) {
			namespace = "default"
		}

		key := fmt.Sprintf("%s/%s/%s", namespace, kind, name)
		infoMap[key] = *resourceInfo
		return output, nil
	}
	yamlkit.VisitResources(parsedData, nil, rp, visitor)
	return infoMap
}

func lookupResourceInfo(infoMap map[string]api.ResourceInfo, key string) api.ResourceInfo {
	if info, ok := infoMap[key]; ok {
		return info
	}
	return api.ResourceInfo{}
}

// extractJSON finds the first top-level JSON object in output, handling
// cases where kubepug prints error messages after the JSON on stderr.
func extractJSON(output []byte) []byte {
	s := string(output)
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return nil
	}
	// Find the matching closing brace by counting nesting depth.
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		if escaped {
			escaped = false
			continue
		}
		c := s[i]
		if inString {
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return output[start : i+1]
			}
		}
	}
	return nil
}
