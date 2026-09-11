// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"errors"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
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
	ouService ou.OrganizationUnitServiceInterface
	logger    *log.Logger
}

// newOUResolverExecutor creates a new OU resolver executor.
func newOUResolverExecutor(
	flowFactory core.FlowFactoryInterface,
	ouService ou.OrganizationUnitServiceInterface,
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
				{Property: common.NodePropertyOUPromptUseHandle},
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

// resolveFromPrompt checks whether the user type's OU has child OUs and,
// if so, prompts the admin to select one during the onboarding flow.
func (e *ouResolverExecutor) resolveFromPrompt(ctx *providers.NodeContext,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	execResp := &providers.ExecutorResponse{
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

	// The user may submit either a literal OU ID (ouId) or a handle scoped to the default OU's
	// children (ouHandle), but not both — each identifies the same selection unambiguously on its
	// own, so accepting both at once would leave which one takes precedence undefined.
	selectedID, hasID := ctx.UserInputs[ouIDKey]
	hasID = hasID && selectedID != ""
	selectedHandle, hasHandle := ctx.UserInputs[ouHandleKey]
	hasHandle = hasHandle && selectedHandle != ""

	if hasID && hasHandle {
		logger.Debug(ctx.Context, "Both ouId and ouHandle were submitted; exactly one is expected")
		execResp.Status = providers.ExecUserInputRequired
		execResp.Inputs = e.promptInputs(ctx)
		execResp.Error = &ErrInvalidOU
		return execResp, nil
	}

	if hasID || hasHandle {
		resolvedOUID := selectedID
		if hasHandle {
			handleOUID, svcErr := e.ouService.GetOrganizationUnitIDByHandle(ctx.Context, selectedHandle, &parentOUID)
			if svcErr != nil {
				if svcErr.Type == tidcommon.ClientErrorType {
					logger.Debug(ctx.Context, "Selected OU handle could not be resolved",
						log.String(ouHandleKey, selectedHandle))
					execResp.Status = providers.ExecUserInputRequired
					execResp.Inputs = e.promptInputs(ctx)
					execResp.Error = &ErrInvalidOU
					return execResp, nil
				}
				return nil, errors.New("failed to resolve organization unit by handle: " + svcErr.Error.DefaultValue)
			}
			resolvedOUID = handleOUID
		}

		// Validate that the resolved OU belongs to the parent OU's subtree.
		isDescendant, svcErr := e.ouService.IsParent(ctx.Context, parentOUID, resolvedOUID)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ClientErrorType {
				execResp.Status = providers.ExecUserInputRequired
				execResp.Inputs = e.promptInputs(ctx)
				execResp.Error = &ErrInvalidOU
				return execResp, nil
			}

			return nil, errors.New("failed to validate selected organization unit: " + svcErr.Error.DefaultValue)
		}
		if !isDescendant {
			logger.Debug(ctx.Context, "Selected OU is not a descendant of the parent OU",
				log.String(ouIDKey, resolvedOUID),
				log.String("parentOUID", parentOUID))
			execResp.Status = providers.ExecUserInputRequired
			execResp.Inputs = e.promptInputs(ctx)
			execResp.Error = &ErrOUNotValidForUserType
			return execResp, nil
		}

		logger.Debug(ctx.Context, "OU selected by user", log.String(ouIDKey, resolvedOUID))
		execResp.RuntimeData[ouIDKey] = resolvedOUID
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	// Check if the parent OU has child OUs. In handle mode every child is fetched so its handle can
	// be offered as a selectable option; otherwise only the count is needed.
	useHandle := e.isPromptUseHandle(ctx)
	childrenLimit := 1
	if useHandle {
		childrenLimit = serverconst.MaxPageSize
	}
	children, svcErr := e.ouService.GetOrganizationUnitChildren(ctx.Context, parentOUID, childrenLimit, 0, nil)
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

	inputs := e.promptInputs(ctx)
	if len(inputs) > 0 {
		input := inputs[0]
		if useHandle {
			// Offer each child's handle as a selectable option; a caller that already knows the
			// target OU's ID may still submit it directly under ouId instead.
			input.Options = organizationUnitHandles(children.OrganizationUnits)
		}
		execResp.Inputs = []providers.Input{input}
		// Forward the root OU ID so the frontend knows where to start the tree picker.
		execResp.AdditionalData[common.DataRootOUID] = parentOUID
		execResp.ForwardedData[common.ForwardedDataKeyInputs] = execResp.Inputs
	}

	return execResp, nil
}

// isPromptUseHandle returns the value of the promptUseHandle node property, defaulting to false
// (offer a literal OU ID) if absent or not a bool.
func (e *ouResolverExecutor) isPromptUseHandle(ctx *providers.NodeContext) bool {
	if ctx.NodeProperties == nil {
		return false
	}
	if val, ok := ctx.NodeProperties[common.NodePropertyOUPromptUseHandle]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

// promptInputs returns the "prompt" strategy's OU-selection input. When promptUseHandle is
// enabled it is keyed by ouHandleKey and typed as a plain SELECT — a flat list of handles with no
// tree data, the same shape a generic SELECT input already renders, so no dedicated OU_SELECT
// frontend support is needed. Otherwise it falls back to the default input (ouIDKey, OU_SELECT).
func (e *ouResolverExecutor) promptInputs(ctx *providers.NodeContext) []providers.Input {
	defaults := e.GetDefaultInputs()
	if !e.isPromptUseHandle(ctx) || len(defaults) == 0 {
		return defaults
	}
	input := defaults[0]
	input.Identifier = ouHandleKey
	input.Type = providers.InputTypeSelect
	return []providers.Input{input}
}

// organizationUnitHandles extracts the handle of each organization unit, in order, for use as a
// selectable input's options.
func organizationUnitHandles(units []providers.OrganizationUnitBasic) []string {
	handles := make([]string, 0, len(units))
	for _, unit := range units {
		handles = append(handles, unit.Handle)
	}
	return handles
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
