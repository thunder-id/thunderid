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
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// PromptNodeInterface extends NodeInterface for nodes that require user interaction.
type PromptNodeInterface interface {
	NodeInterface
	GetPrompts() []common.Prompt
	SetPrompts(prompts []common.Prompt)
	GetMeta() interface{}
	SetMeta(meta interface{})
	GetNextNode() string
	SetNextNode(nextNode string)
	GetMessage() string
	SetMessage(message string)
	IsDisplayOnly() bool
	GetVariant() common.NodeVariant
	SetVariant(variant common.NodeVariant)
}

// promptNode represents a node that prompts for user input/ action in the flow execution.
type promptNode struct {
	*node
	prompts  []common.Prompt
	meta     interface{}
	nextNode string
	message  string
	variant  common.NodeVariant
	logger   *log.Logger
}

// newPromptNode creates a new instance of PromptNode with the given details.
func newPromptNode(id string, properties map[string]interface{},
	isStartNode bool, isFinalNode bool) NodeInterface {
	return &promptNode{
		node: &node{
			id:               id,
			_type:            common.NodeTypePrompt,
			properties:       properties,
			isStartNode:      isStartNode,
			isFinalNode:      isFinalNode,
			nextNodeList:     []string{},
			previousNodeList: []string{},
		},
		prompts: []common.Prompt{},
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "PromptNode"),
			log.String(log.LoggerKeyNodeID, id)),
	}
}

// Execute executes the prompt node logic based on the current context.
func (n *promptNode) Execute(ctx *NodeContext) (*common.NodeResponse, *serviceerror.ServiceError) {
	logger := n.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing prompt node")

	nodeResp := &common.NodeResponse{
		Inputs:         make([]common.Input, 0),
		AdditionalData: make(map[string]string),
		Actions:        make([]common.Action, 0),
		RuntimeData:    make(map[string]string),
	}

	// Check if this prompt is handling a failure
	if ctx.RuntimeData != nil {
		if jsonStr, exists := ctx.RuntimeData["failureReasonJSON"]; exists && jsonStr != "" {
			var errResp serviceerror.ServiceError
			if err := json.Unmarshal([]byte(jsonStr), &errResp); err == nil {
				nodeResp.Error = &errResp
				logger.Debug(ctx.Context, "Prompt node is handling a failure",
					log.String("errorCode", errResp.Code))
			}
			delete(ctx.RuntimeData, "failureReasonJSON")
			// Clear this prompt's inputs and current action
			for _, input := range n.getAllInputs() {
				delete(ctx.UserInputs, input.Identifier)
			}
			ctx.CurrentAction = ""
		}
	}

	// Check if this is a display-only prompt node
	if n.IsDisplayOnly() {
		logger.Debug(ctx.Context, "Display-only prompt node, returning display content")

		if ctx.Verbose && n.GetMeta() != nil {
			nodeResp.Meta = n.GetMeta()
		}

		if n.message != "" {
			if nodeResp.AdditionalData == nil {
				nodeResp.AdditionalData = make(map[string]string)
			}
			nodeResp.AdditionalData[common.DataPromptMessage] = n.message
		}

		nodeResp.Status = common.NodeStatusComplete
		nodeResp.Type = common.NodeResponseTypeView
		return nodeResp, nil
	}

	if n.variant == common.NodeVariantLoginOptions {
		return n.executeLoginOptions(ctx, nodeResp)
	}

	if n.resolvePromptInputs(ctx, nodeResp) {
		logger.Debug(ctx.Context, "All required inputs and actions are available")
		if n.applyValidationFailureRePrompt(ctx, nodeResp) {
			return nodeResp, nil
		}

		if ctx.CurrentAction != "" {
			if nextNode := n.getNextNodeForActionRef(ctx.Context, ctx.CurrentAction); nextNode != "" {
				nodeResp.NextNodeID = nextNode
			} else {
				logger.Debug(ctx.Context, ErrInvalidActionProvided.Error.DefaultValue,
					log.String("actionRef", ctx.CurrentAction))
				nodeResp.Status = common.NodeStatusFailure
				nodeResp.Error = &ErrInvalidActionProvided
				return nodeResp, nil
			}
		}

		// Forward the action type to the next node
		if actionType := n.getActionTypeForRef(ctx.CurrentAction); actionType != "" {
			if nodeResp.ForwardedData == nil {
				nodeResp.ForwardedData = make(map[string]interface{})
			}
			nodeResp.ForwardedData[common.ForwardedDataKeyActionType] = actionType
		}

		nodeResp.Status = common.NodeStatusComplete
		nodeResp.Type = ""
		return nodeResp, nil
	}

	// If required inputs or action is not yet available, prompt for user interaction
	logger.Debug(ctx.Context, "Required inputs or action not available, prompting user",
		log.Any("inputs", nodeResp.Inputs), log.Any("actions", nodeResp.Actions))

	// Include meta in the response if verbose mode is enabled
	if ctx.Verbose && n.GetMeta() != nil {
		trimmed := n.trimMetaToRequestedInputs(n.meta, nodeResp.Inputs, nodeResp.Actions)
		nodeResp.Meta = n.appendSyntheticMetaComponents(trimmed, nodeResp.Inputs)
	}

	nodeResp.Status = common.NodeStatusIncomplete
	nodeResp.Type = common.NodeResponseTypeView
	return nodeResp, nil
}

// applyValidationFailureRePrompt re-validates the current submission. When any rule
// fails, it populates nodeResp with the initial-prompt field set and returns true;
// otherwise it returns false and leaves nodeResp unchanged.
func (n *promptNode) applyValidationFailureRePrompt(ctx *NodeContext, nodeResp *common.NodeResponse) bool {
	actionInputs := n.getInputsForCurrentAction(ctx.CurrentAction)
	fieldErrors := validateInputValues(actionInputs, ctx.UserInputs)
	if len(fieldErrors) == 0 {
		return false
	}
	n.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID)).
		Debug(ctx.Context, "Input validation failed", log.Int("errorCount", len(fieldErrors)))

	matchingAction := n.findActionByRef(ctx.CurrentAction)

	// Clear failing values from every source resolvePromptInputs reads, so the engine
	// cannot fall back to a stale value on the next iteration.
	for _, fe := range fieldErrors {
		delete(ctx.UserInputs, fe.Identifier)
		delete(ctx.RuntimeData, fe.Identifier)
		delete(ctx.ForwardedData, fe.Identifier)
	}
	ctx.CurrentAction = ""

	// Rebuild the initial-prompt set: the action's inputs minus those pre-satisfied
	// via RuntimeData or ForwardedData. UserInputs holds the current submission and
	// is excluded here because it was empty when the initial prompt was rendered.
	rePromptInputs := make([]common.Input, 0, len(actionInputs))
	for _, input := range actionInputs {
		if _, inRuntime := ctx.RuntimeData[input.Identifier]; inRuntime {
			continue
		}
		if val, inForwarded := ctx.ForwardedData[input.Identifier]; inForwarded {
			if _, isString := val.(string); isString {
				continue
			}
		}
		rePromptInputs = append(rePromptInputs, input)
	}
	nodeResp.Inputs = rePromptInputs

	if matchingAction != nil {
		nodeResp.Actions = []common.Action{*matchingAction}
	} else {
		nodeResp.Actions = n.getAllActions()
	}

	nodeResp.FieldErrors = fieldErrors
	nodeResp.Status = common.NodeStatusIncomplete
	nodeResp.Type = common.NodeResponseTypeView
	if ctx.Verbose && n.GetMeta() != nil {
		trimmed := n.trimMetaToRequestedInputs(n.meta, nodeResp.Inputs, nodeResp.Actions)
		nodeResp.Meta = n.appendSyntheticMetaComponents(trimmed, nodeResp.Inputs)
	}
	return true
}

// findActionByRef returns a copy of the action with the given ref, or nil if no match exists.
func (n *promptNode) findActionByRef(ref string) *common.Action {
	if ref == "" {
		return nil
	}
	for _, p := range n.prompts {
		if p.Action != nil && p.Action.Ref == ref {
			action := *p.Action
			return &action
		}
	}
	return nil
}

// executeLoginOptions handles the LOGIN_OPTIONS variant.
func (n *promptNode) executeLoginOptions(ctx *NodeContext,
	nodeResp *common.NodeResponse) (*common.NodeResponse, *serviceerror.ServiceError) {
	logger := n.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	authClassToAction := n.authClassToActionMapping()

	if ctx.CurrentAction != "" {
		if allowedRaw := ctx.RuntimeData[common.RuntimeKeyAllowedLoginOptions]; allowedRaw != "" {
			if !slices.Contains(strings.Fields(allowedRaw), ctx.CurrentAction) {
				logger.Debug(ctx.Context, "Selected action is not in allowed login options",
					log.String("actionRef", ctx.CurrentAction))
				nodeResp.Status = common.NodeStatusFailure
				nodeResp.Error = &ErrInvalidActionProvided
				return nodeResp, nil
			}
		}
		if n.resolvePromptInputs(ctx, nodeResp) {
			if n.applyValidationFailureRePrompt(ctx, nodeResp) {
				return nodeResp, nil
			}
			return n.finalizeLoginOptionsAction(ctx, nodeResp, authClassToAction)
		}
		n.applyMetaForLoginOptions(ctx, nodeResp, nil)
		nodeResp.Status = common.NodeStatusIncomplete
		nodeResp.Type = common.NodeResponseTypeView
		return nodeResp, nil
	}

	requestedAuthClasses := parseAuthClasses(ctx.RuntimeData[common.RuntimeKeyRequestedAuthClasses])
	effectivePrompts := n.filterAndOrderPrompts(requestedAuthClasses, authClassToAction)
	actions := make([]common.Action, 0)
	for _, p := range effectivePrompts {
		if p.Action != nil {
			actions = append(actions, *p.Action)
		}
	}

	// Auto-select the sole remaining option so the user skips a single-choice chooser.
	if len(actions) == 1 && len(requestedAuthClasses) > 0 {
		ctx.CurrentAction = actions[0].Ref
		logger.Debug(ctx.Context, "Auto-selected single login option",
			log.String("actionRef", ctx.CurrentAction))
		if n.resolvePromptInputs(ctx, nodeResp) {
			if n.applyValidationFailureRePrompt(ctx, nodeResp) {
				return nodeResp, nil
			}
			return n.finalizeLoginOptionsAction(ctx, nodeResp, authClassToAction)
		}
		nodeResp.RuntimeData[common.RuntimeKeyAllowedLoginOptions] = ctx.CurrentAction
		n.applyMetaForLoginOptions(ctx, nodeResp, nil)
		nodeResp.Status = common.NodeStatusIncomplete
		nodeResp.Type = common.NodeResponseTypeView
		return nodeResp, nil
	}

	nodeResp.Actions = append(nodeResp.Actions, actions...)
	nodeResp.RuntimeData[common.RuntimeKeyAllowedLoginOptions] = joinActionRefs(effectivePrompts)
	n.applyMetaForLoginOptions(ctx, nodeResp, effectivePrompts)
	nodeResp.Status = common.NodeStatusIncomplete
	nodeResp.Type = common.NodeResponseTypeView
	return nodeResp, nil
}

// finalizeLoginOptionsAction completes the chooser once an action is selected and inputs are satisfied.
func (n *promptNode) finalizeLoginOptionsAction(ctx *NodeContext, nodeResp *common.NodeResponse,
	authClassToAction map[string]string) (*common.NodeResponse, *serviceerror.ServiceError) {
	logger := n.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	nextNode := n.getNextNodeForActionRef(ctx.Context, ctx.CurrentAction)
	if nextNode == "" {
		logger.Debug(ctx.Context, ErrInvalidActionProvided.Error.DefaultValue,
			log.String("actionRef", ctx.CurrentAction))
		nodeResp.Status = common.NodeStatusFailure
		nodeResp.Error = &ErrInvalidActionProvided
		return nodeResp, nil
	}
	nodeResp.NextNodeID = nextNode
	for authClass, ref := range authClassToAction {
		if ref == ctx.CurrentAction {
			nodeResp.RuntimeData[common.RuntimeKeySelectedAuthClass] = authClass
			break
		}
	}
	if actionType := n.getActionTypeForRef(ctx.CurrentAction); actionType != "" {
		if nodeResp.ForwardedData == nil {
			nodeResp.ForwardedData = make(map[string]interface{})
		}
		nodeResp.ForwardedData[common.ForwardedDataKeyActionType] = actionType
	}
	nodeResp.Status = common.NodeStatusComplete
	nodeResp.Type = ""
	return nodeResp, nil
}

// applyMetaForLoginOptions sets verbose-mode meta on the response, reordering it to match
// effectivePrompts when provided.
func (n *promptNode) applyMetaForLoginOptions(ctx *NodeContext, nodeResp *common.NodeResponse,
	effectivePrompts []common.Prompt) {
	if !ctx.Verbose || n.GetMeta() == nil {
		return
	}
	meta := n.meta
	if effectivePrompts != nil {
		meta = n.filteredMeta(effectivePrompts)
	}
	trimmed := n.trimMetaToRequestedInputs(meta, nodeResp.Inputs, nodeResp.Actions)
	nodeResp.Meta = n.appendSyntheticMetaComponents(trimmed, nodeResp.Inputs)
}

// GetPrompts returns the prompts for the prompt node
func (n *promptNode) GetPrompts() []common.Prompt {
	return n.prompts
}

// SetPrompts sets the prompts for the prompt node
func (n *promptNode) SetPrompts(prompts []common.Prompt) {
	n.prompts = prompts
}

// GetMeta returns the meta object for the prompt node
func (n *promptNode) GetMeta() interface{} {
	return n.meta
}

// SetMeta sets the meta object for the prompt node
func (n *promptNode) SetMeta(meta interface{}) {
	n.meta = meta
}

// GetNextNode returns the next node ID for display-only prompt nodes.
func (n *promptNode) GetNextNode() string {
	return n.nextNode
}

// SetNextNode sets the next node ID for display-only prompt nodes.
func (n *promptNode) SetNextNode(nextNode string) {
	n.nextNode = nextNode
}

// GetMessage returns the display message for display-only prompt nodes.
func (n *promptNode) GetMessage() string {
	return n.message
}

// SetMessage sets the display message for display-only prompt nodes.
func (n *promptNode) SetMessage(message string) {
	n.message = message
}

// IsDisplayOnly returns true if this is a display-only prompt node.
// A prompt node is considered display-only if it has a next node, but no prompts (inputs or actions).
func (n *promptNode) IsDisplayOnly() bool {
	return n.nextNode != "" && len(n.prompts) == 0
}

// GetVariant returns the variant of the prompt node
func (n *promptNode) GetVariant() common.NodeVariant {
	return n.variant
}

// SetVariant sets the variant of the prompt node
func (n *promptNode) SetVariant(variant common.NodeVariant) {
	n.variant = variant
}

// resolvePromptInputs resolves the inputs and actions for the prompt node.
// It checks for missing required inputs, validates action selection, attempts auto-selection
// if applicable, and enriches inputs with dynamic data from ForwardedData.
// Returns true if all required inputs are available and a valid action is selected, otherwise false.
func (n *promptNode) resolvePromptInputs(ctx *NodeContext, nodeResp *common.NodeResponse) bool {
	// Check for required inputs and collect missing ones
	hasAllInputs := n.hasRequiredInputs(ctx, nodeResp)

	// Enrich inputs from ForwardedData — may append dynamically derived inputs not in node prompts.
	// If any new inputs are added they are unsatisfied by definition, so the node is incomplete.
	prevCount := len(nodeResp.Inputs)
	n.enrichInputsFromForwardedData(ctx, nodeResp)
	if len(nodeResp.Inputs) > prevCount {
		hasAllInputs = false
	}

	// Check for action selection
	hasAction := n.hasSelectedAction(ctx, nodeResp)

	// If inputs are satisfied but no action selected, try to auto-select single action
	if hasAllInputs && !hasAction && n.tryAutoSelectSingleAction(ctx) {
		hasAction = true
		// Clear actions from response since we auto-selected
		nodeResp.Actions = make([]common.Action, 0)
	}

	return hasAllInputs && hasAction
}

// hasRequiredInputs checks if all required inputs are available in the context. Adds missing
// inputs to the node response. Returns true if all required inputs are available, otherwise false.
func (n *promptNode) hasRequiredInputs(ctx *NodeContext, nodeResp *common.NodeResponse) bool {
	logger := n.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if nodeResp.Inputs == nil {
		nodeResp.Inputs = make([]common.Input, 0)
	}

	// Check if an action is selected
	if ctx.CurrentAction != "" {
		// If the selected action matches a prompt, validate inputs for that prompt only
		for _, prompt := range n.prompts {
			if prompt.Action != nil && prompt.Action.Ref == ctx.CurrentAction {
				return !n.appendMissingInputs(ctx, nodeResp, prompt.Inputs)
			}
		}
		logger.Debug(ctx.Context, "Selected action not found in prompts, treating as no action selected",
			log.String("action", ctx.CurrentAction))
	} else {
		logger.Debug(ctx.Context, "No action selected, checking inputs from all prompts")
	}

	// If no action selected or action not found, validate inputs from all prompts
	return !n.appendMissingInputs(ctx, nodeResp, n.getAllInputs())
}

// appendMissingInputs appends the missing prompt inputs to the node response.
// Returns true when the prompt should pause for user input.
func (n *promptNode) appendMissingInputs(ctx *NodeContext, nodeResp *common.NodeResponse,
	requiredInputs []common.Input) bool {
	logger := log.GetLogger().With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	missing := collectMissingInputs(ctx, GetPresentedOptionalInputs(ctx.RuntimeData), requiredInputs, logger)
	nodeResp.Inputs = append(nodeResp.Inputs, missing...)
	return len(missing) > 0
}

// enrichInputsFromForwardedData enriches the inputs in the node response with dynamic data
// from ForwardedData. Inputs present in ForwardedData but absent from the node response are
// appended (dynamically derived inputs). Options are propagated for all matched inputs.
func (n *promptNode) enrichInputsFromForwardedData(ctx *NodeContext, nodeResp *common.NodeResponse) {
	if ctx.ForwardedData == nil {
		return
	}

	// Check if ForwardedData contains inputs.
	forwardedInputsData, ok := ctx.ForwardedData[common.ForwardedDataKeyInputs]
	if !ok {
		return
	}

	// Type assert to []common.Input.
	forwardedInputs, ok := forwardedInputsData.([]common.Input)
	if !ok {
		n.logger.Debug(ctx.Context,
			"ForwardedData contains 'inputs' key but value is not []common.Input, skipping enrichment")
		return
	}

	// Build an index map of identifiers already in the response for O(1) lookup and in-place update.
	existingIndexMap := make(map[string]int, len(nodeResp.Inputs))
	for i, inp := range nodeResp.Inputs {
		existingIndexMap[inp.Identifier] = i
	}

	// Single pass: upsert forwarded inputs — replace existing entries (updating required/options)
	// or append dynamically derived inputs not yet satisfied by the user.
	for _, fwdInput := range forwardedInputs {
		if idx, exists := existingIndexMap[fwdInput.Identifier]; exists {
			if fwdInput.Required && !nodeResp.Inputs[idx].Required {
				nodeResp.Inputs[idx].Required = true
				n.logger.Debug(ctx.Context, "Updated input required flag from ForwardedData",
					log.String("identifier", fwdInput.Identifier))
			}
			if fwdInput.Type == common.InputTypePassword &&
				nodeResp.Inputs[idx].Type != common.InputTypePassword {
				nodeResp.Inputs[idx].Type = common.InputTypePassword
				n.logger.Debug(ctx.Context, "Updated input type to password from ForwardedData",
					log.String("identifier", fwdInput.Identifier))
			}
			if fwdInput.Type == common.InputTypeSelect &&
				nodeResp.Inputs[idx].Type == common.InputTypeSelect &&
				len(fwdInput.Options) > 0 {
				nodeResp.Inputs[idx].Options = fwdInput.Options
				n.logger.Debug(ctx.Context, "Enriched input with options from ForwardedData",
					log.String("identifier", fwdInput.Identifier),
					log.Int("optionsCount", len(fwdInput.Options)))
			}
			continue
		}
		if _, ok := ctx.UserInputs[fwdInput.Identifier]; ok {
			continue
		}
		if _, ok := ctx.RuntimeData[fwdInput.Identifier]; ok {
			continue
		}
		if value, ok := ctx.ForwardedData[fwdInput.Identifier]; ok {
			if _, isString := value.(string); isString {
				continue
			}
		}
		nodeResp.Inputs = append(nodeResp.Inputs, fwdInput)
		existingIndexMap[fwdInput.Identifier] = len(nodeResp.Inputs) - 1
		n.logger.Debug(ctx.Context, "Added dynamically-derived input from ForwardedData",
			log.String("identifier", fwdInput.Identifier))
	}
}

// hasSelectedAction checks if a valid action has been selected when actions are defined. Adds actions
// to the response if they haven't been selected yet.
// Returns true if an action is already selected or no actions are defined, otherwise false.
func (n *promptNode) hasSelectedAction(ctx *NodeContext, nodeResp *common.NodeResponse) bool {
	actions := n.getAllActions()
	if len(actions) == 0 {
		return true
	}

	// Check if a valid action is selected
	if ctx.CurrentAction != "" {
		for _, action := range actions {
			if action.Ref == ctx.CurrentAction {
				return true
			}
		}
	}

	// If no action selected or invalid action, add actions to response
	nodeResp.Actions = append(nodeResp.Actions, actions...)
	return false
}

// tryAutoSelectSingleAction attempts to auto-select the action when there's exactly one action
// defined, no action has been selected, and inputs are defined. If no inputs are defined
// (confirmation-only prompts), we should not auto-select as the prompt is meant to wait for
// explicit user action.
// Returns true if an action was auto-selected, otherwise false.
func (n *promptNode) tryAutoSelectSingleAction(ctx *NodeContext) bool {
	actions := n.getAllActions()
	allInputs := n.getAllInputs()

	// Auto-select only when: single action, no action selected, and has inputs defined
	// Skip auto-select for confirmation prompts (no inputs) - they should wait for explicit action
	if len(actions) == 1 && ctx.CurrentAction == "" && len(allInputs) > 0 {
		ctx.CurrentAction = actions[0].Ref
		n.logger.Debug(ctx.Context, "Auto-selected single action",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID),
			log.String("actionRef", actions[0].Ref))
		return true
	}
	return false
}

// getInputsForCurrentAction returns the inputs of the prompt whose action matches
// the given actionRef. When no action is selected, or no prompt matches the given
// ref, it falls back to getAllInputs() — preserving the historical behavior for
// single-prompt nodes and the no-action-selected path.
func (n *promptNode) getInputsForCurrentAction(actionRef string) []common.Input {
	if actionRef == "" {
		return n.getAllInputs()
	}
	for _, prompt := range n.prompts {
		if prompt.Action != nil && prompt.Action.Ref == actionRef {
			return prompt.Inputs
		}
	}
	return n.getAllInputs()
}

// getAllInputs returns all unique inputs from prompts, deduplicated by Identifier.
func (n *promptNode) getAllInputs() []common.Input {
	seen := make(map[string]struct{})
	inputs := make([]common.Input, 0)
	for _, prompt := range n.prompts {
		for _, input := range prompt.Inputs {
			if _, exists := seen[input.Identifier]; !exists {
				seen[input.Identifier] = struct{}{}
				inputs = append(inputs, input)
			}
		}
	}

	return inputs
}

// getAllActions returns all actions from prompts.
func (n *promptNode) getAllActions() []common.Action {
	actions := make([]common.Action, 0)
	for _, prompt := range n.prompts {
		if prompt.Action != nil {
			actions = append(actions, *prompt.Action)
		}
	}
	return actions
}

// getNextNodeForActionRef finds the next node for the given action reference.
func (n *promptNode) getNextNodeForActionRef(ctx context.Context, actionRef string) string {
	actions := n.getAllActions()
	for i := range actions {
		if actions[i].Ref == actionRef {
			n.logger.Debug(ctx, "Action selected successfully", log.String("actionRef", actions[i].Ref),
				log.String("nextNode", actions[i].NextNode))
			return actions[i].NextNode
		}
	}
	return ""
}

// getActionTypeForRef finds the action type for the given action reference.
func (n *promptNode) getActionTypeForRef(actionRef string) string {
	for _, prompt := range n.prompts {
		if prompt.Action != nil && prompt.Action.Ref == actionRef {
			return prompt.Action.Type
		}
	}
	return ""
}

// trimMetaToRequestedInputs returns a copy of meta with the "components" list trimmed to only
// include components matching the given inputs and actions (plus structural components like TEXT
// and BLOCK containers that are not themselves inputs or actions).
func (n *promptNode) trimMetaToRequestedInputs(meta interface{}, inputs []common.Input,
	actions []common.Action) interface{} {
	metaMap, ok := meta.(map[string]interface{})
	if !ok {
		return meta
	}

	allowedRefs := make(map[string]struct{})
	for _, input := range inputs {
		if input.Ref != "" {
			allowedRefs[input.Ref] = struct{}{}
		}
	}
	for _, action := range actions {
		if action.Ref != "" {
			allowedRefs[action.Ref] = struct{}{}
		}
	}

	knownInputActionRefs := make(map[string]struct{})
	for _, input := range n.getAllInputs() {
		if input.Ref != "" {
			knownInputActionRefs[input.Ref] = struct{}{}
		}
	}
	for _, action := range n.getAllActions() {
		if action.Ref != "" {
			knownInputActionRefs[action.Ref] = struct{}{}
		}
	}

	trimmed := make(map[string]interface{}, len(metaMap))
	for k, v := range metaMap {
		trimmed[k] = v
	}
	if comps, ok := metaMap["components"]; ok {
		if compSlice, ok := comps.([]interface{}); ok {
			trimmed["components"] = filterMetaComponents(compSlice, allowedRefs, knownInputActionRefs)
		}
	}
	return trimmed
}

// filterMetaComponents filters a meta components slice, dropping satisfied input/action components
// while keeping structural components (TEXT, BLOCK containers, etc.) and recursively trimming
// their children.
func filterMetaComponents(comps []interface{}, allowedRefs, knownInputActionRefs map[string]struct{}) []interface{} {
	result := make([]interface{}, 0, len(comps))
	for _, comp := range comps {
		compMap, ok := comp.(map[string]interface{})
		if !ok {
			result = append(result, comp)
			continue
		}

		id, _ := compMap["id"].(string)
		if _, isKnown := knownInputActionRefs[id]; isKnown {
			if _, isAllowed := allowedRefs[id]; isAllowed {
				result = append(result, comp)
			}
			continue
		}

		// Structural component — always keep; recurse into children if present.
		if childComps, hasChildren := compMap["components"]; hasChildren {
			if childSlice, ok := childComps.([]interface{}); ok {
				trimmedComp := make(map[string]interface{}, len(compMap))
				for k, v := range compMap {
					trimmedComp[k] = v
				}
				trimmedComp["components"] = filterMetaComponents(childSlice, allowedRefs, knownInputActionRefs)
				result = append(result, trimmedComp)
				continue
			}
		}
		result = append(result, comp)
	}
	return result
}

// cloneBlockWithChildren returns a shallow copy of compMap with its "components"
// field replaced by the provided children slice.
func cloneBlockWithChildren(compMap map[string]interface{}, children []interface{}) map[string]interface{} {
	newBlock := make(map[string]interface{}, len(compMap))
	for k, v := range compMap {
		newBlock[k] = v
	}
	newBlock["components"] = children
	return newBlock
}

// buildSyntheticComponentList separates missing inputs into promoted meta
// components and newly synthesized component definitions for inputs absent from
// the meta tree.
func (n *promptNode) buildSyntheticComponentList(
	inputs []common.Input,
	metaCompByRef map[string]map[string]interface{},
	nodeInputRefs map[string]struct{},
) (synthetic []interface{}, promotions map[string]map[string]interface{}) {
	synthetic = make([]interface{}, 0, len(inputs))
	promotions = make(map[string]map[string]interface{})
	for _, input := range inputs {
		ref := input.Identifier
		comp, inMeta := metaCompByRef[ref]
		if !inMeta && input.Ref != "" {
			ref = input.Ref
			comp, inMeta = metaCompByRef[ref]
		}
		if inMeta {
			needsRequired := input.Required && comp["required"] != true
			needsPassword := input.Type == common.InputTypePassword && comp["type"] != common.InputTypePassword
			if needsRequired || needsPassword {
				cloned := make(map[string]interface{}, len(comp))
				for k, v := range comp {
					cloned[k] = v
				}
				if needsRequired {
					cloned["required"] = true
				}
				if needsPassword {
					cloned["type"] = common.InputTypePassword
				}
				promotions[ref] = cloned
			}
			continue
		}
		if _, isNodeInput := nodeInputRefs[input.Identifier]; isNodeInput {
			continue
		}
		label := input.DisplayName
		if label == "" {
			label = input.Identifier
		}
		inputType := input.Type
		if inputType == "" {
			inputType = common.InputTypeText
		}
		synthetic = append(synthetic, map[string]interface{}{
			"id":       input.Identifier,
			"ref":      input.Identifier,
			"type":     inputType,
			"label":    label,
			"required": input.Required,
		})
	}
	return synthetic, promotions
}

// applyComponentPromotions recursively walks a components slice and replaces any component
// whose ref or id matches a key in promotions with the corresponding cloned+promoted map.
// Parent nodes are cloned only when a descendant is actually replaced.
// Returns the updated slice and whether any replacement was made.
func applyComponentPromotions(comps []interface{}, promotions map[string]map[string]interface{}) ([]interface{}, bool) {
	result := make([]interface{}, len(comps))
	copy(result, comps)
	changed := false
	for i, comp := range result {
		compMap, ok := comp.(map[string]interface{})
		if !ok {
			continue
		}
		if ref, _ := compMap["ref"].(string); ref != "" {
			if promoted, ok := promotions[ref]; ok {
				result[i] = promoted
				changed = true
				continue
			}
		}
		if id, _ := compMap["id"].(string); id != "" {
			if promoted, ok := promotions[id]; ok {
				result[i] = promoted
				changed = true
				continue
			}
		}
		children, hasChildren := compMap["components"].([]interface{})
		if !hasChildren {
			continue
		}
		newChildren, childChanged := applyComponentPromotions(children, promotions)
		if childChanged {
			cloned := make(map[string]interface{}, len(compMap))
			for k, v := range compMap {
				cloned[k] = v
			}
			cloned["components"] = newChildren
			result[i] = cloned
			changed = true
		}
	}
	return result, changed
}

// appendSyntheticMetaComponents ensures every input in the list has a corresponding meta
// component. For inputs whose component already exists (matched by ref or id), a cloned
// component with the promoted fields (required, type) is swapped in — the original shared
// metadata is never mutated. For inputs with no existing component, a minimal synthetic
// component is created and inserted into the first BLOCK before any ACTION. If no BLOCK
// exists, a new one is appended.
// The label uses DisplayName when set, falling back to Identifier.
func (n *promptNode) appendSyntheticMetaComponents(trimmedMeta interface{}, inputs []common.Input) interface{} {
	metaMap, ok := trimmedMeta.(map[string]interface{})
	if !ok {
		return trimmedMeta
	}

	// Build a set of refs/ids from the node's own configured prompt inputs —
	// used to suppress synthesis for node-defined inputs with no meta component.
	nodeInputRefs := make(map[string]struct{})
	for _, inp := range n.getAllInputs() {
		if inp.Ref != "" {
			nodeInputRefs[inp.Ref] = struct{}{}
		}
		nodeInputRefs[inp.Identifier] = struct{}{}
	}

	metaCompByRef := make(map[string]map[string]interface{})
	collectMetaComponentMap(metaMap["components"], metaCompByRef)

	synthetic, promotions := n.buildSyntheticComponentList(inputs, metaCompByRef, nodeInputRefs)

	// Walk the meta tree and either replace a DYNAMIC_INPUT_PLACEHOLDER with synthetic
	// inputs (preferred — exact insertion point), or insert before the first ACTION in
	// the first BLOCK (fallback). The placeholder is always stripped even when there are
	// no synthetic inputs, so it never leaks to the client.
	// Input components are only rendered inside a BLOCK by the UI renderer.
	result := make(map[string]interface{}, len(metaMap))
	for k, v := range metaMap {
		result[k] = v
	}
	existing, _ := metaMap["components"].([]interface{})
	updatedComponents := make([]interface{}, len(existing))
	copy(updatedComponents, existing)

	if len(promotions) > 0 {
		updatedComponents, _ = applyComponentPromotions(updatedComponents, promotions)
	}

	placeholderStripped := false
	inserted := false
	for i, comp := range updatedComponents {
		compMap, ok := comp.(map[string]interface{})
		if !ok || compMap["type"] != common.MetaComponentTypeBlock {
			continue
		}
		children, _ := compMap["components"].([]interface{})

		// Preferred path: find and replace DYNAMIC_INPUT_PLACEHOLDER.
		// Always strip it, inserting synthetic inputs in its place (may be empty).
		for j, child := range children {
			childMap, ok := child.(map[string]interface{})
			if !ok || childMap["type"] != common.MetaComponentTypeDynamicInputPlaceholder {
				continue
			}
			newChildren := make([]interface{}, 0, len(children)-1+len(synthetic))
			newChildren = append(newChildren, children[:j]...)
			newChildren = append(newChildren, synthetic...)
			newChildren = append(newChildren, children[j+1:]...)
			updatedComponents[i] = cloneBlockWithChildren(compMap, newChildren)
			placeholderStripped = true
			inserted = true
			break
		}
		if placeholderStripped {
			break
		}

		// Fallback (no placeholder): insert before the first ACTION, only when there
		// are synthetic inputs to add.
		if len(synthetic) == 0 {
			break
		}
		insertIdx := len(children)
		for j, child := range children {
			childMap, ok := child.(map[string]interface{})
			if ok && childMap["type"] == common.MetaComponentTypeAction {
				insertIdx = j
				break
			}
		}
		newChildren := make([]interface{}, 0, len(children)+len(synthetic))
		newChildren = append(newChildren, children[:insertIdx]...)
		newChildren = append(newChildren, synthetic...)
		newChildren = append(newChildren, children[insertIdx:]...)
		updatedComponents[i] = cloneBlockWithChildren(compMap, newChildren)
		inserted = true
		break
	}

	if len(synthetic) == 0 && !placeholderStripped && len(promotions) == 0 {
		return trimmedMeta
	}

	if !inserted && len(synthetic) > 0 {
		// No BLOCK found — wrap synthetic inputs in a new BLOCK so the UI renderer
		// can display them (input components are only supported inside a BLOCK).
		updatedComponents = append(updatedComponents, map[string]interface{}{
			"id":         "block_schema_dynamic",
			"type":       common.MetaComponentTypeBlock,
			"components": synthetic,
		})
	}

	result["components"] = updatedComponents
	return result
}

// collectMetaComponentMap recursively walks the meta components tree and builds a
// ref/id → component map. ref takes priority over id for the same component so that
// identifier-based lookups match the data-binding key rather than the element id.
func collectMetaComponentMap(comps interface{}, compMap map[string]map[string]interface{}) {
	compSlice, ok := comps.([]interface{})
	if !ok {
		return
	}
	for _, comp := range compSlice {
		cm, ok := comp.(map[string]interface{})
		if !ok {
			continue
		}
		if ref, ok := cm["ref"].(string); ok && ref != "" {
			compMap[ref] = cm
		}
		if id, ok := cm["id"].(string); ok && id != "" {
			if _, exists := compMap[id]; !exists {
				compMap[id] = cm
			}
		}
		collectMetaComponentMap(cm["components"], compMap)
	}
}

// filterAndOrderPrompts returns prompts whose action matches a requested auth class (in
// preference order), followed by non-gated prompts. Falls back to all prompts if nothing matches.
func (n *promptNode) filterAndOrderPrompts(requestedAuthClasses []string,
	authClassToAction map[string]string) []common.Prompt {
	if len(requestedAuthClasses) == 0 {
		return n.prompts
	}

	actionToPrompt := make(map[string]common.Prompt)
	gatedActions := make(map[string]struct{})
	for _, p := range n.prompts {
		if p.Action != nil && p.Action.Ref != "" {
			actionToPrompt[p.Action.Ref] = p
		}
	}
	for _, ref := range authClassToAction {
		gatedActions[ref] = struct{}{}
	}

	result := make([]common.Prompt, 0)
	for _, authClass := range requestedAuthClasses {
		ref, ok := authClassToAction[authClass]
		if !ok {
			continue
		}
		if p, ok := actionToPrompt[ref]; ok {
			result = append(result, p)
		}
	}

	for _, p := range n.prompts {
		if p.Action == nil || p.Action.Ref == "" {
			result = append(result, p)
			continue
		}
		if _, gated := gatedActions[p.Action.Ref]; !gated {
			result = append(result, p)
		}
	}

	if len(result) == 0 {
		return n.prompts
	}
	return result
}

// authClassToActionMapping returns the authMethodMapping property as an auth class → action ref map.
func (n *promptNode) authClassToActionMapping() map[string]string {
	result := make(map[string]string)
	props := n.GetProperties()
	if props == nil {
		return result
	}
	raw, ok := props[common.NodePropertyAuthMethodMapping]
	if !ok {
		return result
	}
	mapping, ok := raw.(map[string]interface{})
	if !ok {
		return result
	}
	for authClass, refVal := range mapping {
		if ref, ok := refVal.(string); ok {
			result[authClass] = ref
		}
	}
	return result
}

// joinActionRefs returns a space-separated list of action refs from the given prompts.
func joinActionRefs(prompts []common.Prompt) string {
	refs := make([]string, 0, len(prompts))
	for _, p := range prompts {
		if p.Action != nil && p.Action.Ref != "" {
			refs = append(refs, p.Action.Ref)
		}
	}
	return strings.Join(refs, " ")
}

// filteredMeta returns a copy of n.meta with ACTION components reordered to match prompts.
// Non-ACTION components keep their original positions; filtered-out actions are dropped.
func (n *promptNode) filteredMeta(prompts []common.Prompt) interface{} {
	metaMap, ok := n.meta.(map[string]interface{})
	if !ok {
		return n.meta
	}
	components, ok := metaMap["components"].([]interface{})
	if !ok {
		return n.meta
	}

	actionCompMap := make(map[string]interface{}, len(prompts))
	for _, comp := range components {
		compMap, ok := comp.(map[string]interface{})
		if !ok {
			continue
		}
		if compMap["type"] == "ACTION" {
			if id, ok := compMap["id"].(string); ok {
				actionCompMap[id] = comp
			}
		}
	}

	orderedActions := make([]interface{}, 0, len(prompts))
	for _, p := range prompts {
		if p.Action != nil && p.Action.Ref != "" {
			if comp, ok := actionCompMap[p.Action.Ref]; ok {
				orderedActions = append(orderedActions, comp)
			}
		}
	}

	actionIdx := 0
	result := make([]interface{}, 0, len(components))
	for _, comp := range components {
		compMap, ok := comp.(map[string]interface{})
		if !ok || compMap["type"] != "ACTION" {
			result = append(result, comp)
			continue
		}
		if actionIdx < len(orderedActions) {
			result = append(result, orderedActions[actionIdx])
			actionIdx++
		}
	}

	resultMap := make(map[string]interface{}, len(metaMap))
	for k, v := range metaMap {
		resultMap[k] = v
	}
	resultMap["components"] = result
	return resultMap
}

// parseAuthClasses splits the space-separated auth class values string from RuntimeData into an ordered slice.
func parseAuthClasses(raw string) []string {
	if raw == "" {
		return nil
	}
	return strings.Fields(raw)
}
