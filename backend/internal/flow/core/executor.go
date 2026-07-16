/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package core

import (
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	userAttributeUserID = "userID"
)

// executor represents the basic implementation of an executor.
type executor struct {
	Name          string
	Type          providers.ExecutorType
	DefaultInputs []providers.Input
	Prerequisites []providers.Input
	Meta          *providers.ExecutorMeta
}

var _ providers.Executor = (*executor)(nil)

// newExecutor creates a new instance of Executor with the given properties.
func newExecutor(name string, executorType providers.ExecutorType, defaultInputs []providers.Input,
	prerequisites []providers.Input, meta *providers.ExecutorMeta) providers.Executor {
	return &executor{
		Name:          name,
		Type:          executorType,
		DefaultInputs: defaultInputs,
		Prerequisites: prerequisites,
		Meta:          meta,
	}
}

// GetMeta returns the executor metadata describing its capabilities.
func (e *executor) GetMeta() *providers.ExecutorMeta {
	return e.Meta
}

// GetName returns the name of the executor.
func (e *executor) GetName() string {
	return e.Name
}

// GetType returns the type of the executor.
func (e *executor) GetType() providers.ExecutorType {
	return e.Type
}

// Execute executes the executor logic.
func (e *executor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	// Implement the logic for executing the executor here.
	// This is just a placeholder implementation
	return nil, nil
}

// GetDefaultInputs returns the default required inputs for the executor.
func (e *executor) GetDefaultInputs() []providers.Input {
	return e.DefaultInputs
}

// GetPrerequisites returns the prerequisites for the executor.
func (e *executor) GetPrerequisites() []providers.Input {
	return e.Prerequisites
}

// HasRequiredInputs checks if the required inputs are provided in the context and appends any
// missing inputs to the executor response. Returns true if required inputs are found, otherwise false.
func (e *executor) HasRequiredInputs(ctx *providers.NodeContext, execResp *providers.ExecutorResponse) bool {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "Executor"),
		log.String(log.LoggerKeyExecutorName, e.GetName()),
		log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Checking inputs for the executor")

	requiredData := e.GetRequiredInputs(ctx)

	if execResp.Inputs == nil {
		execResp.Inputs = make([]providers.Input, 0)
	}
	if len(ctx.UserInputs) == 0 && len(ctx.RuntimeData) == 0 && len(ctx.ForwardedData) == 0 {
		execResp.Inputs = append(execResp.Inputs, requiredData...)
		return false
	}

	return !e.appendMissingInputs(ctx, execResp, requiredData)
}

// ValidatePrerequisites validates whether the prerequisites for the executor are met.
// Returns true if all prerequisites are met, otherwise returns false and updates the executor response.
func (e *executor) ValidatePrerequisites(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	authnProvider providers.AuthnProviderManager) bool {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "Executor"),
		log.String(log.LoggerKeyExecutorName, e.GetName()),
		log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	prerequisites := e.GetPrerequisites()
	if len(prerequisites) == 0 {
		return true
	}

	authenticatedUserAttributes := make(map[string]string)
	if authnProvider != nil && ctx.AuthUser.IsAuthenticated() {
		authUser := ctx.AuthUser
		providerAuthUser, entityRef, err := authnProvider.GetEntityReference(ctx.Context, authUser)
		if err != nil {
			logger.Debug(ctx.Context,
				"Failed to get entity reference for authenticated user, proceeding without user id")
		} else {
			authUser = providerAuthUser
			if entityRef.EntityID != "" {
				authenticatedUserAttributes[userAttributeUserID] = entityRef.EntityID
			}
		}
		providerAuthUser, authAttributes, err := authnProvider.GetUserAttributes(ctx.Context, nil, nil, authUser)
		if err != nil {
			logger.Debug(ctx.Context,
				"Failed to get attributes for authenticated user, proceeding without user attributes")
		} else {
			authUser = providerAuthUser
			for key, attribute := range authAttributes.Attributes {
				authenticatedUserAttributes[key] = systemutils.ConvertInterfaceValueToString(attribute.Value)
			}
		}
		execResp.AuthUser = authUser
	}

	for _, prerequisite := range prerequisites {
		// Skip optional prerequisites
		if !prerequisite.Required {
			continue
		}

		if _, ok := ctx.UserInputs[prerequisite.Identifier]; ok {
			continue
		}

		if _, ok := ctx.RuntimeData[prerequisite.Identifier]; ok {
			continue
		}

		if value, ok := ctx.ForwardedData[prerequisite.Identifier]; ok {
			if _, isString := value.(string); isString {
				continue
			}
		}

		if value, ok := authenticatedUserAttributes[prerequisite.Identifier]; ok && value != "" {
			continue
		}

		logger.Debug(ctx.Context, "Prerequisite not met for the executor",
			log.String("identifier", prerequisite.Identifier))
		execResp.Status = providers.ExecFailure
		execResp.Error = tidcommon.CustomServiceError(ErrExecutorPrerequisiteNotMet,
			tidcommon.I18nMessage{
				Key:          ErrExecutorPrerequisiteNotMet.ErrorDescription.Key,
				DefaultValue: "Prerequisite not met: {{param(identifier)}}",
				Params:       map[string]string{"identifier": prerequisite.Identifier},
			})
		return false
	}
	return true
}

// GetUserIDFromContext retrieves the user ID from the context.
func (e *executor) GetUserIDFromContext(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	authnProvider providers.AuthnProviderManager) string {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "Executor"),
		log.String(log.LoggerKeyExecutorName, e.GetName()),
		log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if val, ok := ctx.RuntimeData[userAttributeUserID]; ok && val != "" {
		return val
	}

	if authnProvider != nil && ctx.AuthUser.IsAuthenticated() {
		authUser, entityRef, err := authnProvider.GetEntityReference(ctx.Context, ctx.AuthUser)
		if err != nil {
			logger.Debug(ctx.Context,
				"Failed to get entity reference for authenticated user, proceeding without user id")
		} else {
			if entityRef.EntityID != "" {
				return entityRef.EntityID
			}
		}
		execResp.AuthUser = authUser
	}

	return ""
}

// GetRequiredInputs returns the required inputs for the executor.
// If node inputs are defined, they replace the defaults; otherwise defaults are used.
func (e *executor) GetRequiredInputs(ctx *providers.NodeContext) []providers.Input {
	if len(ctx.NodeInputs) > 0 {
		return ctx.NodeInputs
	}

	return e.GetDefaultInputs()
}

// GetExecutionPolicy returns the execution policy for the given mode. By default, it returns nil,
// indicating no special execution policy. Executors that need per-mode policies should override this method.
func (e *executor) GetExecutionPolicy(mode string) *providers.ExecutionPolicy {
	return nil
}

// appendMissingInputs appends the missing executor inputs to the response.
// Returns true when execution should pause for user input.
func (e *executor) appendMissingInputs(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	requiredInputs []providers.Input) bool {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "Executor"),
		log.String(log.LoggerKeyExecutorName, e.GetName()),
		log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	missing := collectMissingInputs(ctx, GetPresentedOptionalInputs(ctx.RuntimeData), requiredInputs, logger)
	execResp.Inputs = append(execResp.Inputs, missing...)
	return len(missing) > 0
}
