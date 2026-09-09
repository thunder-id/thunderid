// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	ouDeleteExecLoggerComponentName = "OUDeleteExecutor"
)

// ouDeleteExecutor deletes the organization unit recorded in RuntimeData[ouIDKey] by an earlier
// OUExecutor node in the same flow execution. A flow author wires it into their own
// onIncomplete/failure branches as an opt-in compensation step for when a later node fails after
// an OU has already been created — it only ever acts on the OU this same execution created, never
// an arbitrary caller-supplied target, so it carries none of the privilege-escalation risk
// UserDeleteExecutor's trusted-revocation-plan pairing guards against.
type ouDeleteExecutor struct {
	providers.Executor
	ouService ou.OrganizationUnitServiceInterface
	logger    *log.Logger
}

var _ providers.Executor = (*ouDeleteExecutor)(nil)

// newOUDeleteExecutor creates a new instance of OUDeleteExecutor.
func newOUDeleteExecutor(
	flowFactory core.FlowFactoryInterface,
	ouService ou.OrganizationUnitServiceInterface,
) *ouDeleteExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, ouDeleteExecLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameOUDelete))

	base := flowFactory.CreateExecutor(ExecutorNameOUDelete, providers.ExecutorTypeUtility,
		[]providers.Input{}, []providers.Input{}, &providers.ExecutorMeta{})

	return &ouDeleteExecutor{
		Executor:  base,
		ouService: ouService,
		logger:    logger,
	}
}

// Execute deletes the organization unit recorded in RuntimeData[ouIDKey], if any. Completes
// successfully as a no-op when no OU was recorded, since that means there is nothing to roll back.
func (o *ouDeleteExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	ouID := ctx.RuntimeData[ouIDKey]
	if ouID == "" {
		logger.Debug(ctx.Context, "No organization unit recorded in runtime data, nothing to roll back")
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	svcErr := o.ouService.DeleteOrganizationUnit(ctx.Context, ouID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			logger.Debug(ctx.Context, "Failed to delete organization unit",
				log.String(ouIDKey, ouID), log.String("errorCode", svcErr.Code))
			execResp.Status = providers.ExecFailure
			execResp.Error = &ErrOUDeletionFailed
			return execResp, nil
		}

		logger.Error(ctx.Context, "Error occurred while deleting organization unit",
			log.String(ouIDKey, ouID), log.String("errorCode", svcErr.Code))
		return nil, errors.New("failed to delete organization unit")
	}

	logger.Debug(ctx.Context, "Organization unit rolled back successfully", log.String(ouIDKey, ouID))
	execResp.Status = providers.ExecComplete
	return execResp, nil
}
