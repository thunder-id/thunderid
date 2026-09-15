// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"

	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// clientSecretExecutor acts on the client secret of the application named in the trusted revocation
// plan, and runs after the artifacts issued under the old secret have been denied.
type clientSecretExecutor struct {
	providers.Executor
	provider applicationAdminProvider
}

var _ providers.Executor = (*clientSecretExecutor)(nil)

// newClientSecretExecutor creates an executor that regenerates an application's client secret.
func newClientSecretExecutor(factory core.FlowFactoryInterface) *clientSecretExecutor {
	base := factory.CreateExecutor(ExecutorNameClientSecret, providers.ExecutorTypeUtility,
		nil, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
		})
	return &clientSecretExecutor{Executor: base}
}

// Execute regenerates the secret and returns the new value to the caller.
//
// The value goes out on AdditionalData, the only executor output the engine serializes, and this is the
// single moment it is readable: the entity layer hashes it on write and no read path returns it. It is
// deliberately not placed on RuntimeData, which persists with the flow context.
func (e *clientSecretExecutor) Execute(ctx *providers.NodeContext) (
	*providers.ExecutorResponse, error) {
	if e.provider == nil {
		return nil, fmt.Errorf("application service is not configured")
	}
	appID, err := applicationTargetFromPlan(
		ctx.SharedRuntimeData, revocation.ReasonApplicationSecretRegenerated)
	if err != nil {
		return nil, err
	}

	secret, svcErr := e.provider.ApplyCredentialAction(
		ctx.Context, appID, appmodel.CredentialActionRegenerate)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to regenerate client secret: %s", svcErr.Error.DefaultValue)
		}
		return &providers.ExecutorResponse{
			Status: providers.ExecFailure,
			Error:  &ErrSecretRegenerationFailed,
		}, nil
	}
	return &providers.ExecutorResponse{
		Status:         providers.ExecComplete,
		AdditionalData: map[string]string{common.DataClientSecret: secret},
	}, nil
}

// setApplicationProvider injects the application service. See ExecutorRegistryInterface.
func (e *clientSecretExecutor) setApplicationProvider(provider applicationAdminProvider) {
	e.provider = provider
}
