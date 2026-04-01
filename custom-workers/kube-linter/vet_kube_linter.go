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

// kubeLinterBinary is the name or path of the kube-linter CLI binary.
// It can be overridden for testing.
var kubeLinterBinary = "kube-linter"

// GetVetKubeLinterSignature returns the function signature for vet-kube-linter.
func GetVetKubeLinterSignature() api.FunctionSignature {
	return api.FunctionSignature{
		FunctionName: "vet-kube-linter",
		Parameters: []api.FunctionParameter{
			{
				ParameterName: "score-threshold",
				Required:      true,
				Description:   `Score threshold for validation failure. If any finding has a score at or above this threshold, validation fails. Possible values: "Critical", "High", "Medium", "Low". All kube-linter findings are scored as Medium.`,
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
		Hermetic:              false, // execs kube-linter CLI
		Idempotent:            true,
		Description:           "Validates Kubernetes resources using kube-linter best-practice checks. See https://github.com/stackrox/kube-linter for details.",
		FunctionType:          api.FunctionTypeCustom,
		AffectedResourceTypes: []api.ResourceType{api.ResourceTypeAny},
	}
}

// VetKubeLinterFunction validates Kubernetes resources using the kube-linter CLI.
func VetKubeLinterFunction(fArgs handler.FunctionImplementationArguments) (gaby.Container, any, error) {
	return vetKubeLinter(k8skit.NewK8sResourceProvider(), fArgs.ParsedData, fArgs.Arguments)
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

// JSON types matching kube-linter JSON output.
type kubeLinterOutput struct {
	Checks  []kubeLinterCheck  `json:"Checks"`
	Reports []kubeLinterReport `json:"Reports"`
	Summary kubeLinterSummary  `json:"Summary"`
}

type kubeLinterCheck struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Remediation string `json:"remediation"`
}

type kubeLinterReport struct {
	Diagnostic  kubeLinterDiagnostic `json:"Diagnostic"`
	Check       string               `json:"Check"`
	Remediation string               `json:"Remediation"`
	Object      kubeLinterObject     `json:"Object"`
}

type kubeLinterDiagnostic struct {
	Message string `json:"Message"`
}

type kubeLinterObject struct {
	Metadata  kubeLinterObjectMetadata `json:"Metadata"`
	K8sObject kubeLinterK8sObjectInfo  `json:"K8sObject"`
}

type kubeLinterObjectMetadata struct {
	FilePath string `json:"FilePath"`
}

type kubeLinterK8sObjectInfo struct {
	Namespace        string                    `json:"Namespace"`
	Name             string                    `json:"Name"`
	GroupVersionKind kubeLinterGroupVersionKind `json:"GroupVersionKind"`
}

type kubeLinterGroupVersionKind struct {
	Group   string `json:"Group"`
	Version string `json:"Version"`
	Kind    string `json:"Kind"`
}

type kubeLinterSummary struct {
	ChecksStatus string `json:"ChecksStatus"`
}

func vetKubeLinter(rp *k8skit.K8sResourceProviderType, parsedData gaby.Container, args []api.FunctionArgument) (gaby.Container, any, error) {
	threshold, err := parseScoreThreshold(args[0].Value.(string))
	if err != nil {
		return parsedData, nil, err
	}

	// Write resources to a temp file.
	resourceFile, err := os.CreateTemp("", "kube-linter-resource-*.yaml")
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

	// Run kube-linter lint with JSON output.
	cmd := exec.Command(kubeLinterBinary, "lint", "--format", "json", resourceFile.Name())
	output, err := cmd.CombinedOutput()

	// kube-linter exits non-zero when lint violations are found,
	// but still produces valid JSON output. Only treat non-ExitError as real errors.
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return parsedData, nil, errors.Wrapf(err, "failed to execute kube-linter CLI (is %q in PATH?)", kubeLinterBinary)
		}
	}

	// Parse the JSON output. kube-linter may append error messages after
	// the JSON on stderr (via CombinedOutput).
	jsonBytes := extractJSON(output)
	if jsonBytes == nil {
		return parsedData, nil, fmt.Errorf("no JSON found in kube-linter output: %s", string(output))
	}

	var lintOutput kubeLinterOutput
	if err := json.Unmarshal(jsonBytes, &lintOutput); err != nil {
		return parsedData, nil, errors.Wrapf(err, "failed to parse kube-linter JSON output: %s", string(output))
	}

	// If no reports, pass.
	if len(lintOutput.Reports) == 0 {
		return parsedData, api.ValidationResultTrue, nil
	}

	// Build resource info map.
	resourceInfoMap := buildResourceInfoMap(parsedData, rp)

	var details []string
	var failedAttributes api.AttributeValueList

	for _, report := range lintOutput.Reports {
		ns := report.Object.K8sObject.Namespace
		if ns == "" {
			ns = "default"
		}
		kind := report.Object.K8sObject.GroupVersionKind.Kind
		name := report.Object.K8sObject.Name

		resourceKey := fmt.Sprintf("%s/%s/%s", ns, kind, name)
		resourceInfo := lookupResourceInfo(resourceInfoMap, resourceKey)

		detail := fmt.Sprintf("[%s] %s: %s", report.Check, name, report.Diagnostic.Message)
		details = append(details, detail)

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
			Score: api.ScoreMedium,
			Issues: []api.Issue{
				{
					Identifier: report.Check,
					Message:    report.Diagnostic.Message,
				},
			},
		})
	}

	// All kube-linter findings are scored as Medium.
	maxScore := api.ScoreMedium
	passed := api.ScoreMax(maxScore, threshold) != maxScore

	validationResult := api.ValidationResult{
		Passed:           passed,
		MaxScore:         maxScore,
		Details:          details,
		FailedAttributes: failedAttributes,
	}

	return parsedData, validationResult, nil
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
// cases where the CLI prints error messages after the JSON on stderr.
func extractJSON(output []byte) []byte {
	s := string(output)
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return nil
	}
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
