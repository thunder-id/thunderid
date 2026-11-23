// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
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

// applicationUsableFromOU reports whether the application of a bounded sign-in can be used from the picked OU. The
// OU provider is asked as before sign-in, whoever has signed in, so the pick is checked against the application
// only and completion checks the user. A flow without a boundary or application has nothing to check, and every
// pick is usable. An OU that does not exist is not usable, and any other failure of the check is returned.
func (e *ouResolverExecutor) applicationUsableFromOU(ctx *providers.NodeContext, ouID string) (bool, error) {
	if ctx.Context == nil {
		return true, nil
	}
	boundary, bounded := authnprovidercm.SignInBoundaryFrom(ctx.Context)
	if !bounded || boundary.ApplicationID == "" {
		return true, nil
	}

	boundary.EntityID = ""
	usable, svcErr := e.ouService.IsEligibleOU(
		security.WithRuntimeContext(authnprovidercm.WithSignInBoundary(ctx.Context, boundary)), ouID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return false, nil
		}
		return false, errors.New("failed to check application access for the selected organization unit: " +
			svcErr.Error.DefaultValue)
	}
	return usable, nil
}

// Execute resolves the organization unit for the user being onboarded.
// It reads the "resolveFrom" node property to determine the OU resolution strategy.
// Supported strategies:
//   - "caller": overrides the default OU with the caller's OU from the security context.
//   - "prompt": asks a bounded sign-in's signed-in user to pick from the OUs they are eligible for, otherwise
//     checks for child OUs and prompts the user to select one if applicable.
//   - "promptAll": shows the full OU tree from root, independent of UserTypeResolver.
//
// Inside a sign-in boundary, an OU the flow already decided is kept whatever the strategy, and so is the unit
// onboarding pins for a caller confined to their token's unit.
//
// Each read of the OU provider runs with the server's authority at its call site. Reading the units to choose
// from grants nothing, and the entity the flow places there is authorized as the caller; the rest of the
// resolution keeps the caller's context.
func (e *ouResolverExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &providers.ExecutorResponse{
		Status:      providers.ExecComplete,
		RuntimeData: make(map[string]string),
	}

	bounded := false
	if ctx.Context != nil {
		_, bounded = authnprovidercm.SignInBoundaryFrom(ctx.Context)
	}
	if bounded && ctx.RuntimeData[ouIDKey] != "" {
		logger.Debug(ctx.Context, "OU already decided for the sign-in, skipping OU resolution")
		return execResp, nil
	}
	// Onboarding by a caller confined to their token's unit starts with the unit pinned; no strategy may move it.
	if ctx.FlowType == providers.FlowTypeUserOnboarding && ctx.RuntimeData[ouIDKey] != "" &&
		security.IsConfinedToOrganizationUnit(ctx.Context) {
		logger.Debug(ctx.Context, "OU pinned to the caller's unit, skipping OU resolution")
		return execResp, nil
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
	}

	logger.Error(ctx.Context, "Unsupported resolveFrom value", log.String("resolveFrom", resolveFrom))
	execResp.Status = providers.ExecFailure
	execResp.Error = tidcommon.CustomServiceError(ErrOUResolutionFailed, tidcommon.I18nMessage{
		Key:          ErrOUResolutionFailed.ErrorDescription.Key,
		DefaultValue: "Unsupported OU resolution strategy: {{param(strategy)}}",
		Params:       map[string]string{"strategy": resolveFrom},
	})
	return execResp, nil
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

// resolveFromPrompt asks a bounded sign-in's signed-in user to pick from the OUs they are eligible for. Otherwise
// it checks whether the parent OU has child OUs and, if so, prompts the user to select one.
func (e *ouResolverExecutor) resolveFromPrompt(ctx *providers.NodeContext,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	eligible, known, err := e.eligibleOUs(ctx)
	if err != nil {
		return nil, err
	}
	if known {
		return e.selectEligibleOU(ctx, eligible, logger)
	}

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
		isDescendant, svcErr := e.ouService.IsParent(security.WithRuntimeContext(ctx.Context), parentOUID, selectedOUID)
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
		usable, err := e.applicationUsableFromOU(ctx, selectedOUID)
		if err != nil {
			return nil, err
		}
		if !usable {
			logger.Debug(ctx.Context, "Application is not usable from the selected OU",
				log.String(ouIDKey, selectedOUID))
			execResp.Status = providers.ExecUserInputRequired
			execResp.Inputs = e.GetDefaultInputs()
			execResp.Error = &ErrApplicationNotInOU
			return execResp, nil
		}
		logger.Debug(ctx.Context, "OU selected by user", log.String(ouIDKey, selectedOUID))
		execResp.RuntimeData[ouIDKey] = selectedOUID
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	// Check if the parent OU has child OUs.
	children, svcErr := e.ouService.GetOrganizationUnitChildren(
		security.WithRuntimeContext(ctx.Context), parentOUID, 1, 0, nil)
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
		matches, svcErr := e.ouService.GetOrganizationUnitChildren(
			security.WithRuntimeContext(ctx.Context), parentOUID, 1, 0, withHandle)
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
		selectedOU, svcErr := e.ouService.GetOrganizationUnit(security.WithRuntimeContext(ctx.Context), selection)
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

	usable, err := e.applicationUsableFromOU(ctx, selectedOUID)
	if err != nil {
		return nil, err
	}
	if !usable {
		logger.Debug(ctx.Context, "Application is not usable from the selected OU",
			log.String(ouIDKey, selectedOUID))
		return askAgain(&ErrApplicationNotInOU)
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
		exists, svcErr := e.ouService.IsOrganizationUnitExists(security.WithRuntimeContext(ctx.Context), selectedOUID)
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

// eligibleOUs returns the OUs a bounded sign-in's signed-in user is eligible for. known is false outside a sign-in
// boundary, and before a user the OU provider can list them for has signed in.
func (e *ouResolverExecutor) eligibleOUs(ctx *providers.NodeContext) ([]string, bool, error) {
	if ctx.Context == nil {
		return nil, false, nil
	}
	if boundary, bounded := authnprovidercm.SignInBoundaryFrom(ctx.Context); !bounded || boundary.EntityID == "" {
		return nil, false, nil
	}
	// A signed-in user the OU provider cannot list OUs for, such as one the entity store does not hold, picks as
	// before sign-in.
	eligible, svcErr := e.ouService.GetEligibleOUs(security.WithRuntimeContext(ctx.Context))
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return nil, false, nil
		}
		return nil, false, errors.New("failed to read eligible organization units: " + svcErr.Error.DefaultValue)
	}
	return eligible, true, nil
}

// selectEligibleOU decides the OU among the signed-in user's eligible ones, asking the user when several remain.
func (e *ouResolverExecutor) selectEligibleOU(ctx *providers.NodeContext, eligible []string,
	logger *log.Logger) (*providers.ExecutorResponse, error) {
	execResp := &providers.ExecutorResponse{
		RuntimeData:   make(map[string]string),
		ForwardedData: make(map[string]interface{}),
	}

	read := make(map[string]providers.OrganizationUnit)
	candidates, err := e.withinParentOU(ctx, eligible, read)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		logger.Debug(ctx.Context, "User is in no organization unit the application can be used from")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrNoAccessToApplication
		return execResp, nil
	}
	if len(candidates) == 1 {
		logger.Debug(ctx.Context, "Only one eligible OU, deciding it", log.String(ouIDKey, candidates[0]))
		execResp.RuntimeData[ouIDKey] = candidates[0]
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	// Direct children of one parent are told apart by their own handles.
	byHandle := e.isDirectChildrenOnly(ctx) && e.getParentOU(ctx) != ""
	type option struct{ ouID, label string }
	options := make([]option, 0, len(candidates))
	for _, ouID := range candidates {
		label, err := e.optionLabel(ctx.Context, ouID, byHandle, read)
		if err != nil {
			return nil, err
		}
		options = append(options, option{ouID: ouID, label: label})
	}
	slices.SortFunc(options, func(a, b option) int {
		return cmp.Or(cmp.Compare(a.label, b.label), cmp.Compare(a.ouID, b.ouID))
	})

	if selection := ctx.UserInputs[ouIDKey]; selection != "" {
		if i := slices.IndexFunc(options, func(o option) bool { return o.label == selection }); i >= 0 {
			logger.Debug(ctx.Context, "OU selected by user", log.String(ouIDKey, options[i].ouID))
			execResp.RuntimeData[ouIDKey] = options[i].ouID
			execResp.Status = providers.ExecComplete
			return execResp, nil
		}
		logger.Debug(ctx.Context, "Selected OU is not one the user is eligible for")
		execResp.Error = &ErrInvalidOU
	}

	input := e.GetDefaultInputs()[0]
	input.Type = providers.InputTypeSelect
	for _, o := range options {
		input.Options = append(input.Options, o.label)
	}
	execResp.Status = providers.ExecUserInputRequired
	execResp.Inputs = []providers.Input{input}
	execResp.ForwardedData[common.ForwardedDataKeyInputs] = execResp.Inputs
	return execResp, nil
}

// withinParentOU keeps parentOuId and the OUs below it, or only its direct children with directChildrenOnly.
func (e *ouResolverExecutor) withinParentOU(ctx *providers.NodeContext, ouIDs []string,
	read map[string]providers.OrganizationUnit) ([]string, error) {
	parentOUID := e.getParentOU(ctx)
	if parentOUID == "" {
		return ouIDs, nil
	}
	directChildrenOnly := e.isDirectChildrenOnly(ctx)

	kept := make([]string, 0, len(ouIDs))
	for _, ouID := range ouIDs {
		if ouID == parentOUID {
			if !directChildrenOnly {
				kept = append(kept, ouID)
			}
			continue
		}
		var below bool
		if directChildrenOnly {
			ou, err := e.readOU(ctx.Context, ouID, read)
			if err != nil {
				return nil, err
			}
			below = ou.Parent == parentOUID
		} else {
			isDescendant, svcErr := e.ouService.IsParent(security.WithRuntimeContext(ctx.Context), parentOUID, ouID)
			if svcErr != nil {
				return nil, errors.New("failed to check organization unit hierarchy: " + svcErr.Error.DefaultValue)
			}
			below = isDescendant
		}
		if below {
			kept = append(kept, ouID)
		}
	}
	return kept, nil
}

// optionLabel returns how an eligible OU is offered: its handle when byHandle is set, otherwise its handle path.
func (e *ouResolverExecutor) optionLabel(ctx context.Context, ouID string, byHandle bool,
	read map[string]providers.OrganizationUnit) (string, error) {
	if !byHandle {
		return e.handlePath(ctx, ouID, read)
	}
	ou, err := e.readOU(ctx, ouID, read)
	if err != nil {
		return "", err
	}
	return ou.Handle, nil
}

// handlePath returns the OU's handles joined from the root with "/", such as "acme/engineering".
func (e *ouResolverExecutor) handlePath(ctx context.Context, ouID string,
	read map[string]providers.OrganizationUnit) (string, error) {
	var handles []string
	visited := make(map[string]struct{})
	for current := ouID; current != ""; {
		if _, seen := visited[current]; seen {
			return "", errors.New("cyclic organization unit parent chain at " + current)
		}
		visited[current] = struct{}{}

		ou, err := e.readOU(ctx, current, read)
		if err != nil {
			return "", err
		}
		handles = append(handles, ou.Handle)
		current = ou.Parent
	}
	slices.Reverse(handles)
	return strings.Join(handles, "/"), nil
}

// readOU returns the OU with the given ID, reading each one through the OU service once per resolution.
func (e *ouResolverExecutor) readOU(ctx context.Context, ouID string,
	read map[string]providers.OrganizationUnit) (providers.OrganizationUnit, error) {
	if ou, ok := read[ouID]; ok {
		return ou, nil
	}
	ou, svcErr := e.ouService.GetOrganizationUnit(security.WithRuntimeContext(ctx), ouID)
	if svcErr != nil {
		return providers.OrganizationUnit{},
			errors.New("failed to read organization unit: " + svcErr.Error.DefaultValue)
	}
	read[ouID] = ou
	return ou, nil
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
