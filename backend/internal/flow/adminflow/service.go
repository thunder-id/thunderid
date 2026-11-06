// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package adminflow runs the administration flow a deployment configures for a management action, the
// way the Console does, so a change made through another client applies the same steps.
package adminflow

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// The flow configuration entries the runner reads.
const (
	rolePermissionRemovalFlowKey  = "rolePermissionRemovalFlow"
	groupMembershipRemovalFlowKey = "groupMembershipRemovalFlow"
)

// configEntries picks each entry the runner reads out of the flow configuration section.
var configEntries = map[string]func(flowconfig.FlowSectionConfig) flowconfig.FlowTypeConfig{
	rolePermissionRemovalFlowKey: func(c flowconfig.FlowSectionConfig) flowconfig.FlowTypeConfig {
		return c.RolePermissionRemovalFlow
	},
	groupMembershipRemovalFlowKey: func(c flowconfig.FlowSectionConfig) flowconfig.FlowTypeConfig {
		return c.GroupMembershipRemovalFlow
	},
}

// AdministrationFlowRunnerInterface runs the administration flow configured for an action.
type AdministrationFlowRunnerInterface interface {
	// RunAdministrationFlow runs the flow the server configuration names under configKey, such as
	// rolePermissionRemovalFlow, with every input given up front and as the caller. It reports false when
	// no flow is configured, so the caller performs the action directly.
	RunAdministrationFlow(ctx context.Context, configKey string, inputs map[string]string) (
		bool, *tidcommon.ServiceError)
}

// serverConfigReader reads the effective value of a server configuration section.
type serverConfigReader interface {
	GetMergedConfig(ctx context.Context, name string) (any, *tidcommon.ServiceError)
}

type administrationFlowRunner struct {
	logger       *log.Logger
	serverConfig serverConfigReader
	flows        providers.FlowProvider
	executor     flowexec.FlowExecServiceInterface
}

func newAdministrationFlowRunner(serverConfig serverConfigReader, flows providers.FlowProvider,
	executor flowexec.FlowExecServiceInterface) AdministrationFlowRunnerInterface {
	return &administrationFlowRunner{
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AdministrationFlowRunner")),
		serverConfig: serverConfig,
		flows:        flows,
		executor:     executor,
	}
}

// RunAdministrationFlow runs the administration flow configured for an action. A read failure, a
// configured flow that no longer exists, or a run that doesn't complete refuses the action rather than
// falling back to performing it directly, since that would skip the steps the configuration asks for.
func (r *administrationFlowRunner) RunAdministrationFlow(ctx context.Context, configKey string,
	inputs map[string]string) (bool, *tidcommon.ServiceError) {
	handle, svcErr := r.configuredHandle(ctx, configKey)
	if svcErr != nil || handle == "" {
		return false, svcErr
	}

	flow, svcErr := r.flows.GetFlowByHandle(ctx, handle, providers.FlowTypeAdministration)
	if svcErr != nil {
		if svcErr.Code == flowmgt.ErrorFlowNotFound.Code {
			return false, &ErrorConfiguredFlowMissing
		}
		return false, svcErr
	}

	step, svcErr := r.executor.ExecuteByID(ctx, flow.ID, "", false, "", inputs, "", nil)
	if svcErr != nil {
		return false, svcErr
	}
	switch step.Status {
	case providers.FlowStatusComplete:
		return true, nil
	case providers.FlowStatusIncomplete:
		return false, &ErrorFlowNeedsInput
	case providers.FlowStatusError:
		if step.Error != nil {
			return false, step.Error
		}
	}
	r.logger.Error(ctx, "Administration flow did not complete", log.String("status", string(step.Status)))
	return false, &tidcommon.InternalServerError
}

// configuredHandle returns the handle of the flow the server configuration names under configKey, or
// an empty string when none is configured.
func (r *administrationFlowRunner) configuredHandle(ctx context.Context, configKey string) (
	string, *tidcommon.ServiceError) {
	merged, svcErr := r.serverConfig.GetMergedConfig(ctx, string(serverconfig.ConfigNameFlow))
	if svcErr != nil {
		return "", svcErr
	}
	section, ok := merged.(flowconfig.FlowSectionConfig)
	entry, known := configEntries[configKey]
	if !ok || !known {
		r.logger.Error(ctx, "Failed to read the flow configuration entry", log.String("configKey", configKey))
		return "", &tidcommon.InternalServerError
	}
	return entry(section).DefaultHandle, nil
}
