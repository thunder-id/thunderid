// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	userRollbackExecLoggerComponentName = "UserRollbackExecutor"
)

// userRollbackExecutor deletes the user provisioned earlier in RuntimeData[userAttributeUserID] by
// an earlier ProvisioningExecutor node in the same flow execution, after revoking every token issued
// to that user and terminating their sessions. A flow author wires it into their own onIncomplete/failure
// branches as an opt-in compensation step for when a later node fails after a user has already been
// provisioned — it only ever acts on the user this same execution created, never an arbitrary
// caller-supplied target, so it carries none of the privilege-escalation risk UserDeleteExecutor's
// trusted-revocation-plan pairing guards against. Unlike that administration-only pipeline, the
// revoke-then-terminate-then-delete sequence here runs unconditionally against a single subject this
// execution already trusts, with no separate validation/planning step required beforehand.
type userRollbackExecutor struct {
	providers.Executor
	userProvider userDeletionProvider
	revoker      revocation.CriteriaRevoker
	sessionSvc   session.Service
	logger       *log.Logger
}

var _ providers.Executor = (*userRollbackExecutor)(nil)

// newUserRollbackExecutor creates a new instance of UserRollbackExecutor.
func newUserRollbackExecutor(
	flowFactory core.FlowFactoryInterface,
	userProvider userDeletionProvider,
	revoker revocation.CriteriaRevoker,
	sessionSvc session.Service,
) *userRollbackExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, userRollbackExecLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameUserRollback))

	base := flowFactory.CreateExecutor(ExecutorNameUserRollback, providers.ExecutorTypeUtility,
		[]providers.Input{}, []providers.Input{}, &providers.ExecutorMeta{})

	return &userRollbackExecutor{
		Executor:     base,
		userProvider: userProvider,
		revoker:      revoker,
		sessionSvc:   sessionSvc,
		logger:       logger,
	}
}

// Execute revokes every token and session belonging to the user recorded in
// RuntimeData[userAttributeUserID], then deletes that user. Completes successfully as a no-op when
// no user was recorded, since that means there is nothing to roll back.
func (u *userRollbackExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := u.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	userID := ctx.RuntimeData[userAttributeUserID]
	if userID == "" {
		logger.Debug(ctx.Context, "No user recorded in runtime data, nothing to roll back")
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	if err := u.revoker.RevokeByCriteria(ctx.Context, revocation.CriteriaRevocation{
		Criterion: revocation.Criterion{Type: revocation.CriterionTypeSubject, Value: userID},
		Mode:      revocation.ModeAll,
		Reason:    revocation.ReasonUserDeleted,
	}); err != nil {
		logger.Error(ctx.Context, "Failed to revoke tokens for the rolled-back user", log.Error(err))
		return nil, errors.New("failed to revoke tokens for the user")
	}

	if err := u.sessionSvc.TerminateBySubject(ctx.Context, userID); err != nil {
		logger.Error(ctx.Context, "Failed to terminate sessions for the rolled-back user", log.Error(err))
		return nil, errors.New("failed to terminate sessions for the user")
	}

	svcErr := u.userProvider.DeleteUser(ctx.Context, userID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			logger.Debug(ctx.Context, "Failed to delete user", log.String("errorCode", svcErr.Code))
			execResp.Status = providers.ExecFailure
			execResp.Error = &ErrUserRollbackFailed
			return execResp, nil
		}

		logger.Error(ctx.Context, "Error occurred while deleting user", log.String("errorCode", svcErr.Code))
		return nil, errors.New("failed to delete user")
	}

	logger.Debug(ctx.Context, "User rolled back successfully")
	execResp.Status = providers.ExecComplete
	return execResp, nil
}
