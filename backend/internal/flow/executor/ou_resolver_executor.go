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
//   - "prompt": checks for child OUs and prompts the user to select one if applicable. The OUs to select from
//     are those below the default OU set by UserTypeResolver or, in authentication flows only, the node's
//     "parentOuId" property. With "directChildrenOnly", only direct children of that OU can be selected, by handle
//     or by ID.
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

// resolveFromPrompt checks whether the default OU has child OUs and, if so, prompts the user to select one.
func (e *ouResolverExecutor) resolveFromPrompt(ctx *providers.NodeContext,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	execResp := &providers.ExecutorResponse{
		RuntimeData:    make(map[string]string),
		AdditionalData: make(map[string]string),
		ForwardedData:  make(map[string]interface{}),
	}

	// The OUs are selected below the node's parentOuId property, which only authentication flows may use, or else
	// the default OU set by UserTypeResolver. Without the property, the "prompt" strategy requires
	// UserTypeResolver to have run first.
	parentOUID := e.getParentOU(ctx)
	if parentOUID == "" {
		parentOUID = ctx.RuntimeData[defaultOUIDKey]
	}
	if parentOUID == "" {
		message := "no defaultOUID in runtime data; UserTypeResolver must run before OUResolver with prompt strategy"
		if ctx.FlowType == providers.FlowTypeAuthentication {
			message += " unless the node sets parentOuId"
		}

		return nil, errors.New(message)
	}
	directChildrenOnly := e.isDirectChildrenOnly(ctx)

	// If the user already provided an OU selection, validate and accept it.
	if selectedOUID, ok := ctx.UserInputs[ouIDKey]; ok && selectedOUID != "" {
		if directChildrenOnly {
			return e.selectDirectChild(ctx, parentOUID, selectedOUID, execResp, logger)
		}

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
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	// Child OUs exist — prompt the user to select one.
	logger.Debug(ctx.Context, "Child OUs found, requesting OU selection",
		log.String("parentOUID", parentOUID),
		log.Int("totalChildren", children.TotalResults))

	execResp.Status = providers.ExecUserInputRequired

	inputs := e.GetDefaultInputs()
	if len(inputs) > 0 {
		input := inputs[0]
		execResp.Inputs = []providers.Input{input}
		// Forward the root OU ID so the frontend knows where to start the tree picker.
		execResp.AdditionalData[common.DataRootOUID] = parentOUID
		execResp.ForwardedData[common.ForwardedDataKeyInputs] = execResp.Inputs
	}

	return execResp, nil
}

// selectDirectChild accepts the user's selection when it names a direct child of the parent OU, either by
// its handle or by its ID, and records the child's ID as the resolved OU.
func (e *ouResolverExecutor) selectDirectChild(ctx *providers.NodeContext, parentOUID, selection string,
	execResp *providers.ExecutorResponse, logger *log.Logger) (*providers.ExecutorResponse, error) {
	askAgain := func(svcErr *tidcommon.ServiceError) (*providers.ExecutorResponse, error) {
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = e.GetDefaultInputs()
		execResp.Error = svcErr
		return execResp, nil
	}
	failed := func(svcErr *tidcommon.ServiceError) (*providers.ExecutorResponse, error) {
		return nil, errors.New("failed to validate selected organization unit: " + svcErr.Error.DefaultValue)
	}

	byHandle := &tidcommon.FilterGroup{Clauses: []tidcommon.FilterClause{{
		Expr: tidcommon.FilterExpression{Attribute: "handle", Operator: tidcommon.OperatorEq, Value: selection},
	}}}
	matches, svcErr := e.ouService.GetOrganizationUnitChildren(ctx.Context, parentOUID, 1, 0, byHandle)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return askAgain(&ErrInvalidOU)
		}
		return failed(svcErr)
	}

	var selectedOUID string
	if matches != nil && len(matches.OrganizationUnits) > 0 {
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
	switch val := ctx.NodeProperties[common.NodePropertyOUDirectChildrenOnly].(type) {
	case bool:
		return val
	case string:
		return val == dataValueTrue
	default:
		return false
	}
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
