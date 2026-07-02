// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

// Package localexec runs ConfigHub functions in-process using an embedded
// function executor — no worker connector, no server round-trip. The CLI fetches
// a Unit's data, runs a function on it here, and writes the result back. This is
// how cub-autoscale ships convert-hpa-to-keda before it exists as a server-side
// function: the same handler.FunctionImplementation, invoked locally.
package localexec

import (
	"context"
	"fmt"
	"strings"

	"github.com/confighub/sdk/configkit/k8skit"
	api "github.com/confighub/sdk/core/function/api"
	"github.com/confighub/sdk/core/function/executor"
	"github.com/confighub/sdk/core/function/handler"
	"github.com/confighub/sdk/core/workerapi"

	"github.com/confighub/examples/autoscale-manager/internal/keda"
)

// newExecutor builds an executor with the Kubernetes/YAML toolchain and the
// autoscale custom functions registered.
func newExecutor() (*executor.ConcreteFunctionExecutor, error) {
	exec := executor.NewEmptyExecutor()
	exec.RegisterToolchain(k8skit.NewK8sResourceProvider(), true)
	if err := exec.RegisterFunction(workerapi.ToolchainKubernetesYAML, handler.FunctionRegistration{
		FunctionSignature: keda.Signature(),
		Function:          keda.Convert,
	}); err != nil {
		return nil, fmt.Errorf("register %s: %w", keda.FunctionName, err)
	}
	return exec, nil
}

// ConvertHPAToKEDA runs convert-hpa-to-keda over the given Kubernetes/YAML config
// data and returns the (possibly rewritten) data plus whether anything changed.
func ConvertHPAToKEDA(ctx context.Context, configYAML []byte) (out []byte, changed bool, err error) {
	exec, err := newExecutor()
	if err != nil {
		return nil, false, err
	}
	req := &api.FunctionInvocationRequest{
		FunctionContext:     api.FunctionContext{ToolchainType: workerapi.ToolchainKubernetesYAML},
		ConfigData:          configYAML,
		FunctionInvocations: api.FunctionInvocationList{{FunctionName: keda.FunctionName}},
	}
	resp, err := exec.Invoke(ctx, req)
	if err != nil {
		return nil, false, err
	}
	if !resp.Success {
		return nil, false, fmt.Errorf("%s: %s", keda.FunctionName, strings.Join(resp.ErrorMessages, "; "))
	}
	return resp.ConfigData, resp.HasNewMutations, nil
}
