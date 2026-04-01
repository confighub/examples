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

// kubescapeBinary is the name or path of the kubescape CLI binary.
// It can be overridden for testing.
var kubescapeBinary = "kubescape"


// GetVetKubescapeSignature returns the function signature for vet-kubescape.
func GetVetKubescapeSignature() api.FunctionSignature {
	return api.FunctionSignature{
		FunctionName: "vet-kubescape",
		Parameters: []api.FunctionParameter{
			{
				ParameterName: "score-threshold",
				Required:      true,
				Description:   `Score threshold for validation failure. If any finding has a score at or above this threshold, validation fails. Possible values: "Critical", "High", "Medium", "Low".`,
				DataType:      api.DataTypeString,
			},
		},
		RequiredParameters: 1,
		OutputInfo: &api.FunctionOutput{
			ResultName:  "passed",
			Description: "True if all findings score below the threshold, false otherwise",
			OutputType:  api.OutputTypeValidationResult,
		},
		Mutating:              false,
		Validating:            true,
		Hermetic:              false, // execs kubescape CLI
		Idempotent:            true,
		Description:           "Validates Kubernetes resources against kubescape security controls. See https://kubescape.io for details.",
		FunctionType:          api.FunctionTypeCustom,
		AffectedResourceTypes: []api.ResourceType{api.ResourceTypeAny},
	}
}

// VetKubescapeFunction validates Kubernetes resources using the kubescape CLI.
func VetKubescapeFunction(fArgs handler.FunctionImplementationArguments) (gaby.Container, any, error) {
	return vetKubescape(k8skit.NewK8sResourceProvider(), fArgs.ParsedData, fArgs.Arguments)
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

// JSON types matching kubescape JSON v2 output.
type kubescapeReport struct {
	SummaryDetails kubescapeSummaryDetails `json:"summaryDetails"`
	Results        []kubescapeResult       `json:"results"`
}

type kubescapeSummaryDetails struct {
	Status         string                                 `json:"status"`
	Score          float32                                `json:"score"`
	StatusCounters kubescapeStatusCounters                `json:"ResourceCounters"`
	Controls       map[string]kubescapeControlSummary     `json:"controls"`
}

type kubescapeStatusCounters struct {
	Passed  int `json:"passedResources"`
	Failed  int `json:"failedResources"`
	Skipped int `json:"skippedResources"`
}

type kubescapeControlSummary struct {
	Name     string `json:"name"`
	ID       string `json:"controlID"`
	Severity string `json:"severity"`
}

type kubescapeResult struct {
	ResourceID         string                            `json:"resourceID"`
	AssociatedControls []kubescapeAssociatedControl       `json:"controls"`
}

type kubescapeAssociatedControl struct {
	ControlID string                    `json:"controlID"`
	Name      string                    `json:"name"`
	Severity  string                    `json:"severity"`
	Status    kubescapeControlStatus    `json:"status"`
	Rules     []kubescapeRuleResponse   `json:"rules"`
}

type kubescapeControlStatus struct {
	Status string `json:"status"`
}

type kubescapeRuleResponse struct {
	Name  string                  `json:"name"`
	Paths []kubescapePosturePath  `json:"paths"`
}

type kubescapePosturePath struct {
	FailedPath string              `json:"failedPath"`
	FixPath    kubescapeFixPath    `json:"fixPath"`
}

type kubescapeFixPath struct {
	Path  string `json:"path"`
	Value string `json:"value"`
}

// kubescapeSeverityToScore maps kubescape severity strings to ConfigHub Score.
func kubescapeSeverityToScore(severity string) api.Score {
	switch strings.ToLower(severity) {
	case "critical":
		return api.ScoreCritical
	case "high":
		return api.ScoreHigh
	case "medium":
		return api.ScoreMedium
	case "low":
		return api.ScoreLow
	default:
		return api.ScoreLow
	}
}

func vetKubescape(rp *k8skit.K8sResourceProviderType, parsedData gaby.Container, args []api.FunctionArgument) (gaby.Container, any, error) {
	threshold, err := parseScoreThreshold(args[0].Value.(string))
	if err != nil {
		return parsedData, nil, err
	}

	// Write resources to a temp file.
	resourceFile, err := os.CreateTemp("", "kubescape-resource-*.yaml")
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

	// Write JSON output to a temp file since kubescape writes to file with --format json.
	outputFile, err := os.CreateTemp("", "kubescape-output-*.json")
	if err != nil {
		return parsedData, nil, errors.Wrap(err, "failed to create temp output file")
	}
	defer os.Remove(outputFile.Name())
	outputFile.Close()

	// Run kubescape scan with JSON output.
	cmd := exec.Command(kubescapeBinary, "scan",
		resourceFile.Name(),
		"--format", "json",
		"--output", outputFile.Name(),
	)
	_, err = cmd.CombinedOutput()

	// kubescape exits non-zero when controls fail,
	// but still produces valid JSON output. Only treat non-ExitError as real errors.
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return parsedData, nil, errors.Wrapf(err, "failed to execute kubescape CLI (is %q in PATH?)", kubescapeBinary)
		}
	}

	// Read the JSON output file.
	output, err := os.ReadFile(outputFile.Name())
	if err != nil {
		return parsedData, nil, errors.Wrap(err, "failed to read kubescape output file")
	}

	// Parse the JSON output.
	var report kubescapeReport
	if err := json.Unmarshal(output, &report); err != nil {
		return parsedData, nil, errors.Wrapf(err, "failed to parse kubescape JSON output: %s", string(output))
	}

	// Check if everything passed.
	if report.SummaryDetails.StatusCounters.Failed == 0 {
		return parsedData, api.ValidationResultTrue, nil
	}

	// Build resource info map.
	resourceInfoMap := buildResourceInfoMap(parsedData, rp)

	var maxScore api.Score
	var details []string
	var failedAttributes api.AttributeValueList

	for _, result := range report.Results {
		for _, control := range result.AssociatedControls {
			if strings.ToLower(control.Status.Status) != "failed" {
				continue
			}

			score := kubescapeSeverityToScore(control.Severity)
			maxScore = api.ScoreMax(maxScore, score)

			// Parse resourceID to find the resource info.
			// kubescape resourceID format varies, try to match by the last parts.
			resourceInfo := lookupResourceInfoByKubescapeID(resourceInfoMap, result.ResourceID)

			// Collect failed paths from rules. Use failedPath when available,
			// fall back to fixPath.Path (which indicates what needs to be set).
			var failedPaths []string
			for _, rule := range control.Rules {
				for _, path := range rule.Paths {
					if path.FailedPath != "" {
						failedPaths = append(failedPaths, path.FailedPath)
					} else if path.FixPath.Path != "" {
						failedPaths = append(failedPaths, path.FixPath.Path)
					}
				}
			}

			detail := fmt.Sprintf("[%s] %s (%s): %s", control.Severity, control.Name, control.ControlID, result.ResourceID)
			details = append(details, detail)

			if len(failedPaths) > 0 {
				for _, fp := range failedPaths {
					dotPath := normalizePath(fp)
					failedAttributes = append(failedAttributes, api.AttributeValue{
						AttributeInfo: api.AttributeInfo{
							AttributeIdentifier: api.AttributeIdentifier{
								ResourceInfo: resourceInfo,
								Path:         api.ResolvedPath(dotPath),
							},
							AttributeMetadata: api.AttributeMetadata{
								AttributeName: api.AttributeNameNone,
								Details: &api.AttributeDetails{
									Description: detail,
								},
							},
						},
						Score: score,
						Issues: []api.Issue{
							{
								Identifier: control.ControlID,
								Message:    fmt.Sprintf("%s: %s", control.Name, result.ResourceID),
							},
						},
					})
				}
			} else {
				failedAttributes = append(failedAttributes, api.AttributeValue{
					AttributeInfo: api.AttributeInfo{
						AttributeIdentifier: api.AttributeIdentifier{
							ResourceInfo: resourceInfo,
						},
						AttributeMetadata: api.AttributeMetadata{
							AttributeName: api.AttributeNameNone,
							Details: &api.AttributeDetails{
								Description: detail,
							},
						},
					},
					Score: score,
					Issues: []api.Issue{
						{
							Identifier: control.ControlID,
							Message:    fmt.Sprintf("%s: %s", control.Name, result.ResourceID),
						},
					},
				})
			}
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

// normalizePath converts a path to dot notation. Handles both JSON Pointer
// form (e.g., "spec/template/spec/containers/0/securityContext") and
// kubescape's bracket notation (e.g., "spec.template.spec.containers[0].image").
func normalizePath(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	// If it already uses dots, it's likely kubescape bracket notation.
	if strings.Contains(path, ".") {
		return path
	}
	// JSON Pointer: convert slashes to dots.
	return strings.ReplaceAll(path, "/", ".")
}

// lookupResourceInfoByKubescapeID attempts to match a kubescape resourceID
// to the resource info map. Kubescape resourceIDs use the format:
// "path=HASH/api=GROUP/VERSION/NAMESPACE/KIND/NAME"
// e.g., "path=123/api=apps/v1/default/Deployment/bad-deployment"
func lookupResourceInfoByKubescapeID(infoMap map[string]api.ResourceInfo, resourceID string) api.ResourceInfo {
	// Strip "path=HASH/" prefix if present.
	if idx := strings.Index(resourceID, "/api="); idx >= 0 {
		resourceID = resourceID[idx+len("/api="):]
	}

	// Now resourceID is like "apps/v1/default/Deployment/bad-deployment"
	// or "v1/default/Pod/my-pod". Walk backwards: name, kind, namespace.
	parts := strings.Split(resourceID, "/")
	if len(parts) >= 3 {
		name := parts[len(parts)-1]
		kind := parts[len(parts)-2]
		namespace := parts[len(parts)-3]
		key := fmt.Sprintf("%s/%s/%s", namespace, kind, name)
		if info, ok := infoMap[key]; ok {
			return info
		}
	}

	return api.ResourceInfo{}
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

