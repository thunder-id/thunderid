// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"encoding/json"
	"errors"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authzdetail "github.com/thunder-id/thunderid/internal/authzdetail/common"
	authzdetailexec "github.com/thunder-id/thunderid/internal/authzdetail/exec"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// authorizationDetailExecutor runs the type processors of the requested RFC 9396 authorization details.
// It must run after authentication and before consent.
type authorizationDetailExecutor struct {
	providers.Executor
	authzDetails  providers.AuthorizationDetailProvider
	authnProvider providers.AuthnProviderManager
	logger        *log.Logger
}

var _ providers.Executor = (*authorizationDetailExecutor)(nil)

// newAuthorizationDetailExecutor creates a new instance of authorizationDetailExecutor.
func newAuthorizationDetailExecutor(
	flowFactory core.FlowFactoryInterface,
	authzDetails providers.AuthorizationDetailProvider,
	authnProvider providers.AuthnProviderManager,
) *authorizationDetailExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthorizationDetailExecutor"),
		log.String(log.LoggerKeyExecutorName, ExecutorNameAuthorizationDetail))

	base := flowFactory.CreateExecutor(ExecutorNameAuthorizationDetail, providers.ExecutorTypeUtility,
		[]providers.Input{}, []providers.Input{}, &providers.ExecutorMeta{})

	return &authorizationDetailExecutor{
		Executor:      base,
		authzDetails:  authzDetails,
		authnProvider: authnProvider,
		logger:        logger,
	}
}

// Execute processes the requested authorization details and stores the admitted details.
func (e *authorizationDetailExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
		AuthUser:    ctx.AuthUser,
	}

	if ctx.RuntimeData[common.RuntimeKeyRequestedAuthorizationDetails] == "" {
		logger.Debug(ctx.Context, "No authorization details requested")
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	if !execResp.AuthUser.IsAuthenticated() {
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrUserNotAuthenticated
		return execResp, nil
	}

	authUser, entityRef, svcErr := e.authnProvider.GetEntityReference(ctx.Context, execResp.AuthUser)
	execResp.AuthUser = authUser
	if svcErr != nil {
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, categoryUnscoped)
		return execResp, nil
	}

	details, err := requestedAuthorizationDetails(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read the requested authorization details: %w", err)
	}
	admitted, svcErr := e.authzDetails.ProcessAuthorizationDetails(ctx.Context, details, &ctx.Application,
		*entityRef, ctx.RuntimeData[common.RuntimeKeyResourceServerIdentifier])
	if svcErr != nil {
		switch {
		case svcErr.Type == tidcommon.ClientErrorType:
			logger.Debug(ctx.Context, "Authorization details refused", log.String("code", svcErr.Code))
			execResp.Status = providers.ExecFailure
			execResp.Error = refusedError(svcErr.ErrorDescription.Params["reason"])
			return execResp, nil
		case svcErr.Code == authzdetailexec.ErrorProcessorUnavailable.Code,
			svcErr.Code == authzdetail.ErrorAuthorizationDetailTypeChanged.Code:
			execResp.Status = providers.ExecFailure
			execResp.Error = &ErrAuthorizationDetailsUnavailable
			return execResp, nil
		}
		return nil, errors.New("failed to process authorization details")
	}

	admittedJSON, err := json.Marshal(admitted)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the admitted authorization details: %w", err)
	}
	execResp.RuntimeData[common.RuntimeKeyRequestedAuthorizationDetails] = string(admittedJSON)
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// refusedError returns the refusal error, with the provider's reason when it gave one that is safe to show,
// since a provider supplied through the engine may not have filtered it.
func refusedError(reason string) *tidcommon.ServiceError {
	reason = authzdetail.DisplayableReason(reason)
	if reason == "" {
		return &ErrAuthorizationDetailsRefused
	}
	desc := errAuthorizationDetailsRefusedReason
	desc.Params = map[string]string{"reason": reason}
	return tidcommon.CustomServiceError(ErrAuthorizationDetailsRefused, desc)
}
