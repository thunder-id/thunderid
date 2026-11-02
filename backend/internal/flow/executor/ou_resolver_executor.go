// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"errors"
	"strings"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
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
	providers.Executor
	ouService providers.OrganizationUnitProvider
	logger    *log.Logger
}

// newOUResolverExecutor creates a new OU resolver executor.
func newOUResolverExecutor(
	flowFactory core.FlowFactoryInterface,
	ouService providers.OrganizationUnitProvider,
) *ouResolverExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OUResolverExecutor"))

	defaultInputs := []providers.Input{
		{
			Ref:        "ou_selection_input",
			Identifier: ouIDKey,
			Type:       providers.InputTypeOUSelect,
			Required:   true,
		},
	}

	base := flowFactory.CreateExecutor(
		ExecutorNameOUResolver,
		providers.ExecutorTypeUtility,
		defaultInputs,
		[]providers.Input{},
		&providers.ExecutorMeta{
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: common.NodePropertyOUResolveFrom},
				{Property: common.NodePropertyOUParent},
				{Property: common.NodePropertyOUDirectChildrenOnly},
			},
		},
	)
	return &ouResolverExecutor{
		Executor:  base,
		ouService: ouService,
		logger:    logger,
	}
}

// Execute resolves the organization unit for the user being onboarded.
// It reads the "resolveFrom" node property to determine the OU resolution strategy.
// Supported strategies:
//   - "caller": overrides the default OU with the caller's OU from the security context.
//   - "prompt": checks for child OUs and prompts the user to select one if applicable.
//   - "promptAll": shows the full OU tree from root, independent of UserTypeResolver.
func (e *ouResolverExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &providers.ExecutorResponse{
		Status:      providers.ExecComplete,
		RuntimeData: make(map[string]string),
	}

	resolveFrom := e.getResolveFrom(ctx)
	if resolveFrom == "" {
		logger.Debug(ctx.Context, "resolveFrom not configured, skipping OU override")
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
		logger.Error(ctx.Context, "Unsupported resolveFrom value", log.String("resolveFrom", resolveFrom))
		execResp.Status = providers.ExecFailure
		execResp.Error = tidcommon.CustomServiceError(ErrOUResolutionFailed, tidcommon.I18nMessage{
			Key:          ErrOUResolutionFailed.ErrorDescription.Key,
			DefaultValue: "Unsupported OU resolution strategy: {{param(strategy)}}",
			Params:       map[string]string{"strategy": resolveFrom},
		})
		return execResp, nil
	}
}

// resolveFromCaller resolves the OU from the caller's security context.
func (e *ouResolverExecutor) resolveFromCaller(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, logger *log.Logger) (*providers.ExecutorResponse, error) {
	callerOUID := security.GetOUID(ctx.Context)
	if callerOUID == "" {
		logger.Error(ctx.Context, "Caller OU not found in security context")
		execResp.Status = providers.ExecFailure
		execResp.Error = tidcommon.CustomServiceError(ErrOUResolutionFailed, tidcommon.I18nMessage{
			Key:          ErrOUResolutionFailed.ErrorDescription.Key,
			DefaultValue: "Unable to resolve caller organization unit from  context",
		})
		return execResp, nil
	}

	logger.Debug(ctx.Context, "Overriding user OU with caller's OU", log.String("callerOUID", callerOUID))
	execResp.RuntimeData[ouIDKey] = callerOUID

	return execResp, nil
}

// resolveFromPrompt checks whether the parent OU has child OUs and, if so, prompts the user to select one.
func (e *ouResolverExecutor) resolveFromPrompt(ctx *providers.NodeContext,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	execResp := &providers.ExecutorResponse{
		RuntimeData:    make(map[string]string),
		AdditionalData: make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
	}

	// The parent OU is the node's parentOuId (authentication flows only), else the default OU from UserTypeResolver.
	configuredParentOUID := e.getParentOU(ctx)
	parentOUID := configuredParentOUID
	if parentOUID == "" {
		parentOUID = ctx.RuntimeData[defaultOUIDKey]
	}
	if parentOUID == "" {
		if ctx.FlowType == providers.FlowTypeAuthentication {
			return nil, errors.New("no parentOuId configured on the OUResolver node with prompt strategy")
		}

		return nil, errors.New(
			"no defaultOUID in runtime data; UserTypeResolver must run before OUResolver with prompt strategy",
		)
	}
	directChildrenOnly := e.isDirectChildrenOnly(ctx)
	selectionKey := ouIDKey
	if directChildrenOnly {
		selectionKey = e.directChildrenInput(ctx).Identifier
	}

	// If the user already provided an OU selection, validate and accept it.
	if selection, ok := ctx.UserInputs[selectionKey]; ok && selection != "" {
		if directChildrenOnly {
			return e.selectDirectChild(ctx, parentOUID, selection, selectionKey == ouHandleKey, execResp, logger)
		}
		selectedOUID := selection

		// Validate that the selected OU belongs to the parent OU's subtree.
		isDescendant, svcErr := e.ouService.IsParent(ctx.Context, parentOUID, selectedOUID)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ClientErrorType {
				execResp.Status = providers.ExecUserInputRequired
				execResp.Inputs = e.GetDefaultInputs()
				execResp.Error = &ErrInvalidOU
				return execResp, nil
			}

			return nil, errors.New("failed to validate selected organization unit: " + svcErr.Error.DefaultValue)
		}
		if !isDescendant {
			logger.Debug(ctx.Context, "Selected OU is not a descendant of the parent OU",
				log.String(ouIDKey, selectedOUID),
				log.String("parentOUID", parentOUID))
			execResp.Status = providers.ExecUserInputRequired
			execResp.Inputs = e.GetDefaultInputs()
			execResp.Error = &ErrOUNotValidForUserType
			return execResp, nil
		}
		logger.Debug(ctx.Context, "OU selected by user", log.String(ouIDKey, selectedOUID))
		execResp.RuntimeData[ouIDKey] = selectedOUID
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	// Check if the parent OU has child OUs.
	children, svcErr := e.ouService.GetOrganizationUnitChildren(ctx.Context, parentOUID, 1, 0, nil)
	if svcErr != nil {
		return nil, errors.New("failed to check child organization units: " + svcErr.Error.DefaultValue)
	}

	if children.TotalResults == 0 {
		logger.Debug(ctx.Context, "No child OUs found, skipping OU selection")
		// The default OU is already in runtime data, but a parentOuId is not, so record it as the resolved OU.
		if configuredParentOUID != "" {
			execResp.RuntimeData[ouIDKey] = configuredParentOUID
		}
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	// Child OUs exist — prompt the user to select one.
	logger.Debug(ctx.Context, "Child OUs found, requesting OU selection",
		log.String("parentOUID", parentOUID),
		log.Int("totalChildren", children.TotalResults))

	execResp.Status = providers.ExecUserInputRequired

	inputs := e.promptInputs(ctx)
	if len(inputs) > 0 {
		input := inputs[0]
		execResp.Inputs = []providers.Input{input}
		// Forward the root OU ID so the frontend knows where to start the tree picker.
		execResp.AdditionalData[common.DataRootOUID] = parentOUID
		execResp.ForwardedData[common.ForwardedDataKeyInputs] = execResp.Inputs
	}

	return execResp, nil
}

// selectDirectChild accepts the user's selection when it names a direct child of the parent OU, by its handle
// when byHandle is set and by its ID otherwise, and records the child's ID as the resolved OU.
func (e *ouResolverExecutor) selectDirectChild(ctx *providers.NodeContext, parentOUID, selection string,
	byHandle bool, execResp *providers.ExecutorResponse, logger *log.Logger) (*providers.ExecutorResponse, error) {
	askAgain := func(svcErr *tidcommon.ServiceError) (*providers.ExecutorResponse, error) {
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = e.promptInputs(ctx)
		execResp.AdditionalData[common.DataRootOUID] = parentOUID
		execResp.Error = svcErr
		return execResp, nil
	}
	failed := func(svcErr *tidcommon.ServiceError) (*providers.ExecutorResponse, error) {
		return nil, errors.New("failed to validate selected organization unit: " + svcErr.Error.DefaultValue)
	}

	var selectedOUID string
	if byHandle {
		withHandle := &tidcommon.FilterGroup{Clauses: []tidcommon.FilterClause{{
			Expr: tidcommon.FilterExpression{Attribute: "handle", Operator: tidcommon.OperatorEq, Value: selection},
		}}}
		matches, svcErr := e.ouService.GetOrganizationUnitChildren(ctx.Context, parentOUID, 1, 0, withHandle)
		if svcErr != nil {
			return failed(svcErr)
		}
		if matches == nil || len(matches.OrganizationUnits) == 0 {
			logger.Debug(ctx.Context, "No direct child of the parent OU has the selected handle",
				log.String(ouHandleKey, selection), log.String("parentOUID", parentOUID))
			return askAgain(&ErrInvalidOU)
		}
		selectedOUID = matches.OrganizationUnits[0].ID
	} else {
		selectedOU, svcErr := e.ouService.GetOrganizationUnit(ctx.Context, selection)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ClientErrorType {
				return askAgain(&ErrInvalidOU)
			}
			return failed(svcErr)
		}
		if selectedOU.Parent != parentOUID {
			logger.Debug(ctx.Context, "Selected OU is not a direct child of the parent OU",
				log.String(ouIDKey, selectedOU.ID),
				log.String("parentOUID", parentOUID))
			return askAgain(&ErrOUNotDirectChild)
		}
		selectedOUID = selectedOU.ID
	}

	logger.Debug(ctx.Context, "Direct child OU selected by user", log.String(ouIDKey, selectedOUID))
	execResp.RuntimeData[ouIDKey] = selectedOUID
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// directChildrenInput returns the input the "prompt" strategy asks for in direct children mode. It is the ouId or
// ouHandle input the flow author declares on the node, and the default ouId input when none is declared.
func (e *ouResolverExecutor) directChildrenInput(ctx *providers.NodeContext) providers.Input {
	inputs := e.GetRequiredInputs(ctx)
	for _, input := range inputs {
		if input.Identifier == ouIDKey || input.Identifier == ouHandleKey {
			return input
		}
	}

	return e.GetDefaultInputs()[0]
}

// promptInputs returns the input the "prompt" strategy asks the user for: the one declared on the node in direct
// children mode, otherwise the default OU selection keyed by ouId.
func (e *ouResolverExecutor) promptInputs(ctx *providers.NodeContext) []providers.Input {
	if !e.isDirectChildrenOnly(ctx) {
		return e.GetDefaultInputs()
	}

	return []providers.Input{e.directChildrenInput(ctx)}
}

// resolveFromPromptAll shows the full OU tree from root, allowing selection of any OU.
// Unlike "prompt", this strategy does not depend on UserTypeResolver having run first.
func (e *ouResolverExecutor) resolveFromPromptAll(ctx *providers.NodeContext,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	execResp := &providers.ExecutorResponse{
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
			execResp.Status = providers.ExecUserInputRequired
			execResp.Inputs = e.GetDefaultInputs()
			execResp.Error = &ErrOUNotFound
			return execResp, nil
		}

		logger.Debug(ctx.Context, "OU selected by user", log.String(ouIDKey, selectedOUID))
		execResp.RuntimeData[ouIDKey] = selectedOUID
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	// No selection yet — prompt the user with the full OU tree.
	logger.Debug(ctx.Context, "Requesting OU selection from full tree")
	execResp.Status = providers.ExecUserInputRequired

	inputs := e.GetDefaultInputs()
	if len(inputs) > 0 {
		input := inputs[0]
		execResp.Inputs = []providers.Input{input}
		execResp.ForwardedData[common.ForwardedDataKeyInputs] = execResp.Inputs
	}

	return execResp, nil
}

// getParentOU retrieves the parentOuId property, which names the OU the "prompt" strategy selects below. It is
// read in authentication flows only, where UserTypeResolver does not set a default OU.
func (e *ouResolverExecutor) getParentOU(ctx *providers.NodeContext) string {
	if ctx.FlowType != providers.FlowTypeAuthentication {
		return ""
	}
	val, _ := ctx.NodeProperties[common.NodePropertyOUParent].(string)

	return strings.TrimSpace(val)
}

// isDirectChildrenOnly reports whether the node limits the "prompt" strategy to direct children.
func (e *ouResolverExecutor) isDirectChildrenOnly(ctx *providers.NodeContext) bool {
	val, _ := ctx.NodeProperties[common.NodePropertyOUDirectChildrenOnly].(bool)

	return val
}

// getResolveFrom retrieves the resolveFrom strategy from the node properties.
func (e *ouResolverExecutor) getResolveFrom(ctx *providers.NodeContext) string {
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
