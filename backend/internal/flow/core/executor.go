/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"github.com/asgardeo/thunder/internal/flow/common"
	"github.com/asgardeo/thunder/internal/system/log"
)

const (
	userAttributeUserID = "userID"
)

// ExecutorInterface defines the interface for executors.
type ExecutorInterface interface {
	Execute(ctx *NodeContext) (*common.ExecutorResponse, error)
	GetName() string
	GetType() common.ExecutorType
	GetDefaultInputs() []common.Input
	GetPrerequisites() []common.Input
	HasRequiredInputs(ctx *NodeContext, execResp *common.ExecutorResponse) bool
	ValidatePrerequisites(ctx *NodeContext, execResp *common.ExecutorResponse) bool
	GetUserIDFromContext(ctx *NodeContext) string
	GetRequiredInputs(ctx *NodeContext) []common.Input
	GetExecutionPolicy(mode string) *ExecutionPolicy
}

// executor represents the basic implementation of an executor.
type executor struct {
	Name          string
	Type          common.ExecutorType
	DefaultInputs []common.Input
	Prerequisites []common.Input
}

var _ ExecutorInterface = (*executor)(nil)

// newExecutor creates a new instance of Executor with the given properties.
func newExecutor(name string, executorType common.ExecutorType, defaultInputs []common.Input,
	prerequisites []common.Input) ExecutorInterface {
	return &executor{
		Name:          name,
		Type:          executorType,
		DefaultInputs: defaultInputs,
		Prerequisites: prerequisites,
	}
}

// GetName returns the name of the executor.
func (e *executor) GetName() string {
	return e.Name
}

// GetType returns the type of the executor.
func (e *executor) GetType() common.ExecutorType {
	return e.Type
}

// Execute executes the executor logic.
func (e *executor) Execute(ctx *NodeContext) (*common.ExecutorResponse, error) {
	// Implement the logic for executing the executor here.
	// This is just a placeholder implementation
	return nil, nil
}

// GetDefaultInputs returns the default required inputs for the executor.
func (e *executor) GetDefaultInputs() []common.Input {
	return e.DefaultInputs
}

// GetPrerequisites returns the prerequisites for the executor.
func (e *executor) GetPrerequisites() []common.Input {
	return e.Prerequisites
}

// HasRequiredInputs checks if the required inputs are provided in the context and appends any
// missing inputs to the executor response. Returns true if required inputs are found, otherwise false.
func (e *executor) HasRequiredInputs(ctx *NodeContext, execResp *common.ExecutorResponse) bool {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "Executor"),
		log.String(log.LoggerKeyExecutorName, e.GetName()),
		log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Checking inputs for the executor")

	requiredData := e.GetRequiredInputs(ctx)

	if execResp.Inputs == nil {
		execResp.Inputs = make([]common.Input, 0)
	}
	if len(ctx.UserInputs) == 0 && len(ctx.RuntimeData) == 0 && len(ctx.ForwardedData) == 0 {
		execResp.Inputs = append(execResp.Inputs, requiredData...)
		return false
	}

	return !e.appendMissingInputs(ctx, execResp, requiredData)
}

// ValidatePrerequisites validates whether the prerequisites for the executor are met.
// Returns true if all prerequisites are met, otherwise returns false and updates the executor response.
func (e *executor) ValidatePrerequisites(ctx *NodeContext, execResp *common.ExecutorResponse) bool {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "Executor"),
		log.String(log.LoggerKeyExecutorName, e.GetName()),
		log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	prerequisites := e.GetPrerequisites()
	if len(prerequisites) == 0 {
		return true
	}

	for _, prerequisite := range prerequisites {
		// Skip optional prerequisites
		if !prerequisite.Required {
			continue
		}

		if prerequisite.Identifier == userAttributeUserID {
			userID := ctx.AuthUser.GetUserID()
			if userID != "" {
				continue
			}
		}

		if _, ok := ctx.UserInputs[prerequisite.Identifier]; !ok {
			if _, ok := ctx.RuntimeData[prerequisite.Identifier]; !ok {
				if value, ok := ctx.ForwardedData[prerequisite.Identifier]; !ok {
					logger.Debug("Prerequisite not met for the executor",
						log.String("identifier", prerequisite.Identifier))
					execResp.Status = common.ExecFailure
					execResp.FailureReason = "Prerequisite not met: " + prerequisite.Identifier
					return false
				} else {
					// ForwardedData found but verify it's a string value
					if _, isString := value.(string); !isString {
						logger.Debug("Prerequisite not met for the executor (non-string in ForwardedData)",
							log.String("identifier", prerequisite.Identifier))
						execResp.Status = common.ExecFailure
						execResp.FailureReason = "Prerequisite not met: " + prerequisite.Identifier
						return false
					}
				}
			}
		}
	}

	return true
}

// GetUserIDFromContext retrieves the user ID from the context.
func (e *executor) GetUserIDFromContext(ctx *NodeContext) string {
	userID := ctx.AuthUser.GetUserID()
	if userID == "" {
		userID = ctx.RuntimeData[userAttributeUserID]
	}

	return userID
}

// GetRequiredInputs returns the required inputs for the executor.
// If node inputs are defined, they replace the defaults; otherwise defaults are used.
func (e *executor) GetRequiredInputs(ctx *NodeContext) []common.Input {
	if len(ctx.NodeInputs) > 0 {
		return ctx.NodeInputs
	}

	return e.GetDefaultInputs()
}

// GetExecutionPolicy returns the execution policy for the given mode. By default, it returns nil,
// indicating no special execution policy. Executors that need per-mode policies should override this method.
func (e *executor) GetExecutionPolicy(mode string) *ExecutionPolicy {
	return nil
}

// appendMissingInputs appends the missing required inputs to the executor response.
// Returns true if any required input is found missing, otherwise false.
func (e *executor) appendMissingInputs(ctx *NodeContext, execResp *common.ExecutorResponse,
	requiredInputs []common.Input) bool {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "Executor"),
		log.String(log.LoggerKeyExecutorName, e.GetName()),
		log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	requireData := false
	for _, input := range requiredInputs {
		if _, ok := ctx.UserInputs[input.Identifier]; !ok {
			if _, ok := ctx.RuntimeData[input.Identifier]; ok {
				logger.Debug("Input available in runtime data, skipping",
					log.String("identifier", input.Identifier), log.Bool("isRequired", input.Required))
				continue
			}

			if value, ok := ctx.ForwardedData[input.Identifier]; ok {
				if _, isString := value.(string); isString {
					logger.Debug("Input available in forwarded data, skipping",
						log.String("identifier", input.Identifier), log.Bool("isRequired", input.Required))
					continue
				}
			}

			requireData = true
			execResp.Inputs = append(execResp.Inputs, input)
			logger.Debug("Input not available in the context",
				log.String("identifier", input.Identifier), log.Bool("isRequired", input.Required))
		}
	}

	return requireData
}
