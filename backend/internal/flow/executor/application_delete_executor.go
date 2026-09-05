// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// applicationDeleteExecutor deletes the application named in the trusted revocation plan. It runs last,
// after that application's artifacts have been denied and its session participation detached.
type applicationDeleteExecutor struct {
	providers.Executor
	provider providers.ApplicationAdminProvider
}

var _ providers.Executor = (*applicationDeleteExecutor)(nil)

// newApplicationDeleteExecutor creates an executor that performs the application deletion.
func newApplicationDeleteExecutor(factory core.FlowFactoryInterface) *applicationDeleteExecutor {
	base := factory.CreateExecutor(ExecutorNameApplicationDelete, providers.ExecutorTypeUtility,
		nil, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
		})
	return &applicationDeleteExecutor{Executor: base}
}

// Execute deletes the application identified by the trusted revocation plan.
func (e *applicationDeleteExecutor) Execute(ctx *providers.NodeContext) (
	*providers.ExecutorResponse, error) {
	provider, err := e.applicationProvider()
	if err != nil {
		return nil, err
	}
	appID, err := applicationTargetFromPlan(ctx.SharedRuntimeData, revocation.ReasonApplicationDeleted)
	if err != nil {
		return nil, err
	}

	if svcErr := provider.DeleteApplication(ctx.Context, appID); svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to delete application: %s", svcErr.Error.DefaultValue)
		}
		return &providers.ExecutorResponse{
			Status: providers.ExecFailure,
			Error:  &ErrApplicationDeletionFailed,
		}, nil
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}

// setApplicationProvider injects the application service. See ExecutorRegistryInterface.
func (e *applicationDeleteExecutor) setApplicationProvider(provider providers.ApplicationAdminProvider) {
	e.provider = provider
}

// applicationProvider returns the injected application service, or an error when the second-phase
// injection never ran.
func (e *applicationDeleteExecutor) applicationProvider() (providers.ApplicationAdminProvider, error) {
	if e.provider == nil {
		return nil, fmt.Errorf("application service is not configured")
	}
	return e.provider, nil
}
