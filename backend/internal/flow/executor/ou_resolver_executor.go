/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package executor

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// OU resolve from strategy values.
const (
	// ouResolveFromCaller indicates that the caller's OU should be used when creating the user.
	ouResolveFromCaller = "caller"
	// ouResolveFromPrompt indicates that the user should be prompted to select an OU.
	ouResolveFromPrompt = "prompt"
	// ouResolveFromPromptAll shows the full OU tree without depending on UserTypeResolver.
	ouResolveFromPromptAll = "promptAll"
)

// ouResolverExecutor resolves the organization unit for a user being onboarded.
type ouResolverExecutor struct {
	core.ExecutorInterface
	ouService ou.OrganizationUnitServiceInterface
	logger    *log.Logger
}

// newOUResolverExecutor creates a new OU resolver executor.
func newOUResolverExecutor(
	flowFactory core.FlowFactoryInterface,
	ouService ou.OrganizationUnitServiceInterface,
) *ouResolverExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OUResolverExecutor"))

	defaultInputs := []common.Input{
		{
			Ref:        "ou_selection_input",
			Identifier: ouIDKey,
			Type:       "OU_SELECT",
			Required:   true,
		},
	}

	base := flowFactory.CreateExecutor(
		ExecutorNameOUResolver,
		common.ExecutorTypeUtility,
		defaultInputs,
		[]common.Input{},
	)
	return &ouResolverExecutor{
		ExecutorInterface: base,
		ouService:         ouService,
		logger:            logger,
	}
}

// Execute resolves the organization unit for the user being onboarded.
// It reads the "resolveFrom" node property to determine the OU resolution strategy.
// Supported strategies:
//   - "caller": overrides the default OU with the caller's OU from the security context.
//   - "prompt": checks for child OUs and prompts the user to select one if applicable.
//   - "promptAll": shows the full OU tree from root, independent of UserTypeResolver.
func (e *ouResolverExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &common.ExecutorResponse{
		Status:      common.ExecComplete,
		RuntimeData: make(map[string]string),
	}

	resolveFrom := e.getResolveFrom(ctx)
	if resolveFrom == "" {
		logger.Debug("resolveFrom not configured, skipping OU override")
		return execResp, nil
	}

	switch resolveFrom {
	case ouResolveFromCaller:
		return e.resolveFromCaller(ctx, execResp, logger)
	case ouResolveFromPrompt:
		return e.resolveFromPrompt(ctx, logger)
	case ouResolveFromPromptAll:
		return e.resolveFromPromptAll(ctx, logger)
	default:
		logger.Error("Unsupported resolveFrom value", log.String("resolveFrom", resolveFrom))
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Unsupported OU resolution strategy: " + resolveFrom
		return execResp, nil
	}
}

// resolveFromCaller resolves the OU from the caller's security context.
func (e *ouResolverExecutor) resolveFromCaller(ctx *core.NodeContext,
	execResp *common.ExecutorResponse, logger *log.Logger) (*common.ExecutorResponse, error) {
	callerOUID := security.GetOUID(ctx.Context)
	if callerOUID == "" {
		logger.Error("Caller OU not found in security context")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Unable to determine caller organization unit"
		return execResp, nil
	}

	logger.Debug("Overriding user OU with caller's OU", log.String("callerOUID", callerOUID))
	execResp.RuntimeData[ouIDKey] = callerOUID

	return execResp, nil
}

// resolveFromPrompt checks whether the user type's OU has child OUs and,
// if so, prompts the admin to select one during the onboarding flow.
func (e *ouResolverExecutor) resolveFromPrompt(ctx *core.NodeContext,
	logger *log.Logger) (*common.ExecutorResponse, error) {
	execResp := &common.ExecutorResponse{
		RuntimeData:    make(map[string]string),
		AdditionalData: make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
	}

	// Read the default OU set by UserTypeResolver.
	// The "prompt" strategy requires UserTypeResolver to have run first and set the defaultOUID.
	parentOUID := ctx.RuntimeData[defaultOUIDKey]
	if parentOUID == "" {
		return nil, errors.New(
			"no defaultOUID in runtime data; UserTypeResolver must run before OUResolver with prompt strategy",
		)
	}

	// If the user already provided an OU selection, validate and accept it.
	if selectedOUID, ok := ctx.UserInputs[ouIDKey]; ok && selectedOUID != "" {
		// Validate that the selected OU belongs to the parent OU's subtree.
		isDescendant, svcErr := e.ouService.IsParent(ctx.Context, parentOUID, selectedOUID)
		if svcErr != nil {
			if svcErr.Type == serviceerror.ClientErrorType {
				execResp.Status = common.ExecUserInputRequired
				execResp.Inputs = e.GetDefaultInputs()
				execResp.FailureReason = "The selected organization unit is not valid."
				return execResp, nil
			}

			return nil, errors.New("failed to validate selected organization unit: " + svcErr.Error.DefaultValue)
		}
		if !isDescendant {
			logger.Debug("Selected OU is not a descendant of the parent OU",
				log.String(ouIDKey, selectedOUID),
				log.String("parentOUID", parentOUID))
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = e.GetDefaultInputs()
			execResp.FailureReason = "The selected organization unit is not valid for the chosen user type."
			return execResp, nil
		}

		logger.Debug("OU selected by user", log.String(ouIDKey, selectedOUID))
		execResp.RuntimeData[ouIDKey] = selectedOUID
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	// Check if the parent OU has child OUs.
	children, svcErr := e.ouService.GetOrganizationUnitChildren(ctx.Context, parentOUID, 1, 0, nil)
	if svcErr != nil {
		return nil, errors.New("failed to check child organization units: " + svcErr.Error.DefaultValue)
	}

	if children.TotalResults == 0 {
		logger.Debug("No child OUs found, skipping OU selection")
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	// Child OUs exist — prompt the user to select one.
	logger.Debug("Child OUs found, requesting OU selection",
		log.String("parentOUID", parentOUID),
		log.Int("totalChildren", children.TotalResults))

	execResp.Status = common.ExecUserInputRequired

	inputs := e.GetDefaultInputs()
	if len(inputs) > 0 {
		input := inputs[0]
		execResp.Inputs = []common.Input{input}
		// Forward the root OU ID so the frontend knows where to start the tree picker.
		execResp.AdditionalData[common.DataRootOUID] = parentOUID
		execResp.ForwardedData[common.ForwardedDataKeyInputs] = execResp.Inputs
	}

	return execResp, nil
}

// resolveFromPromptAll shows the full OU tree from root, allowing selection of any OU.
// Unlike "prompt", this strategy does not depend on UserTypeResolver having run first.
func (e *ouResolverExecutor) resolveFromPromptAll(ctx *core.NodeContext,
	logger *log.Logger) (*common.ExecutorResponse, error) {
	execResp := &common.ExecutorResponse{
		RuntimeData:    make(map[string]string),
		AdditionalData: make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
	}

	// If the user already provided an OU selection, validate and accept it.
	if selectedOUID, ok := ctx.UserInputs[ouIDKey]; ok && selectedOUID != "" {
		exists, svcErr := e.ouService.IsOrganizationUnitExists(ctx.Context, selectedOUID)
		if svcErr != nil {
			return nil, errors.New("failed to validate selected organization unit: " + svcErr.Error.DefaultValue)
		}
		if !exists {
			execResp.Status = common.ExecUserInputRequired
			execResp.Inputs = e.GetDefaultInputs()
			execResp.FailureReason = "The selected organization unit does not exist."
			return execResp, nil
		}

		logger.Debug("OU selected by user", log.String(ouIDKey, selectedOUID))
		execResp.RuntimeData[ouIDKey] = selectedOUID
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	// No selection yet — prompt the user with the full OU tree.
	logger.Debug("Requesting OU selection from full tree")
	execResp.Status = common.ExecUserInputRequired

	inputs := e.GetDefaultInputs()
	if len(inputs) > 0 {
		input := inputs[0]
		execResp.Inputs = []common.Input{input}
		execResp.ForwardedData[common.ForwardedDataKeyInputs] = execResp.Inputs
	}

	return execResp, nil
}

// getResolveFrom retrieves the resolveFrom strategy from the node properties.
func (e *ouResolverExecutor) getResolveFrom(ctx *core.NodeContext) string {
	if ctx.NodeProperties == nil {
		return ""
	}
	val, ok := ctx.NodeProperties[common.NodePropertyOUResolveFrom]
	if !ok {
		return ""
	}
	strVal, ok := val.(string)
	if !ok {
		return ""
	}
	return strVal
}
