// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	userRollbackExecLoggerComponentName = "UserRollbackExecutor"
)

// userRollbackDeleter exposes the user operation the user rollback executor needs.
type userRollbackDeleter interface {
	DeleteUser(ctx context.Context, userID string) *tidcommon.ServiceError
}

// userRollbackExecutor deletes the user provisioned earlier in RuntimeData[common.RuntimeKeyProvisionedUserID] by
// an earlier ProvisioningExecutor node in the same flow execution. Deleting a user runs the user
// deletion system flow, which revokes every token issued to the user and terminates their sessions
// first. A flow author wires it into their own onFailure branches as an opt-in compensation step for
// when a later node fails after a user has already been provisioned. It only ever acts on the user
// this same execution created, never an arbitrary caller-supplied target.
type userRollbackExecutor struct {
	providers.Executor
	userProvider userRollbackDeleter
	logger       *log.Logger
}

var _ providers.Executor = (*userRollbackExecutor)(nil)

// newUserRollbackExecutor creates a new instance of UserRollbackExecutor.
func newUserRollbackExecutor(
	flowFactory core.FlowFactoryInterface,
	userProvider userRollbackDeleter,
) *userRollbackExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, userRollbackExecLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameUserRollback))

	base := flowFactory.CreateExecutor(ExecutorNameUserRollback, providers.ExecutorTypeUtility,
		[]providers.Input{}, []providers.Input{}, &providers.ExecutorMeta{
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyForwardFailure},
			},
		})

	return &userRollbackExecutor{
		Executor:     base,
		userProvider: userProvider,
		logger:       logger,
	}
}

// Execute deletes the user recorded in RuntimeData[common.RuntimeKeyProvisionedUserID]. Completes successfully as a no-op when
// no user was recorded, since that means there is nothing to roll back.
func (u *userRollbackExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := u.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	userID := ctx.RuntimeData[common.RuntimeKeyProvisionedUserID]
	if userID == "" {
		logger.Debug(ctx.Context, "No user recorded in runtime data, nothing to roll back")
		return completeOrForwardFailure(ctx, execResp), nil
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
	return completeOrForwardFailure(ctx, execResp), nil
}
