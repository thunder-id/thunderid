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

package common

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"

	"gopkg.in/yaml.v3"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// Input represents the inputs required for a node
type Input struct {
	Ref         string           `json:"ref,omitempty"`
	Identifier  string           `json:"identifier"`
	Type        string           `json:"type"`
	Required    bool             `json:"required"`
	Options     []string         `json:"options,omitempty"`
	DisplayName string           `json:"-"`
	Validation  []ValidationRule `json:"validation,omitempty"`
}

// ValidationRule defines a single constraint on a flow input. CompiledRegex is
// populated by PrepareValidationRules at graph-build time and excluded from JSON.
type ValidationRule struct {
	Type          ValidationType `json:"type"`
	Value         interface{}    `json:"value"`
	Message       string         `json:"message,omitempty"`
	CompiledRegex *regexp.Regexp `json:"-"`
}

// PrepareValidationRules compiles the regex pattern of every regex rule in place.
// An empty or non-string regex value is treated as a no-op.
func PrepareValidationRules(rules []ValidationRule) error {
	for i := range rules {
		if rules[i].Type != ValidationTypeRegex {
			continue
		}
		pattern, ok := rules[i].Value.(string)
		if !ok || pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid validation regex %q: %w", pattern, err)
		}
		rules[i].CompiledRegex = re
	}
	return nil
}

// FieldError represents a single validation rule failure for a specific input field.
type FieldError struct {
	Identifier string `json:"identifier"`
	Message    string `json:"message"`
}

// IsSensitive checks whether this input's type is considered sensitive.
func (i Input) IsSensitive() bool {
	return slices.Contains(sensitiveInputTypes, i.Type)
}

// Action represents an action to be executed in a flow step
type Action struct {
	Ref      string `json:"ref,omitempty"`
	Type     string `json:"type,omitempty"`
	NextNode string `json:"nextNode,omitempty"`
}

// Prompt groups inputs with an action for prompt nodes.
type Prompt struct {
	Inputs []Input `json:"inputs,omitempty"`
	Action *Action `json:"action,omitempty"`
}

// NodeResponse represents the response from a node execution
type NodeResponse struct {
	Status            NodeStatus                 `json:"status"`
	Type              NodeResponseType           `json:"type"`
	Error             *serviceerror.ServiceError `json:"error,omitempty"`
	Inputs            []Input                    `json:"inputs,omitempty"`
	AdditionalData    map[string]string          `json:"additionalData,omitempty"`
	RedirectURL       string                     `json:"redirectUrl,omitempty"`
	Actions           []Action                   `json:"actions,omitempty"`
	Meta              interface{}                `json:"meta,omitempty"`
	NextNodeID        string                     `json:"nextNodeId,omitempty"`
	RuntimeData       map[string]string          `json:"runtimeData,omitempty"`
	ForwardedData     map[string]interface{}     `json:"forwardedData,omitempty"`
	AuthenticatedUser authncm.AuthenticatedUser  `json:"authenticatedUser,omitempty"`
	Assertion         string                     `json:"assertion,omitempty"`
	FieldErrors       []FieldError               `json:"fieldErrors,omitempty"`
	AuthUser          authnprovidermgr.AuthUser  `json:"-"`
}

// ExecutorResponse represents the response from an executor
type ExecutorResponse struct {
	Status            ExecutorStatus             `json:"status"`
	Inputs            []Input                    `json:"inputs,omitempty"`
	AdditionalData    map[string]string          `json:"additionalData,omitempty"`
	RedirectURL       string                     `json:"redirectUrl,omitempty"`
	RuntimeData       map[string]string          `json:"runtimeData,omitempty"`
	ForwardedData     map[string]interface{}     `json:"forwardedData,omitempty"`
	AuthenticatedUser authncm.AuthenticatedUser  `json:"authenticatedUser,omitempty"`
	Assertion         string                     `json:"assertion,omitempty"`
	Error             *serviceerror.ServiceError `json:"error,omitempty"`
	AuthUser          authnprovidermgr.AuthUser  `json:"-"`
}

// NodeExecutionRecord represents a record of a node execution in the flow.
type NodeExecutionRecord struct {
	NodeID       string             `json:"nodeId"`
	NodeType     string             `json:"nodeType"`
	ExecutorName string             `json:"executorName,omitempty"`
	ExecutorType ExecutorType       `json:"executorType,omitempty"`
	ExecutorMode string             `json:"executorMode,omitempty"`
	Step         int                `json:"step"`
	Status       FlowStatus         `json:"status"`
	Executions   []ExecutionAttempt `json:"executions"`
	StartTime    int64              `json:"startTime,omitempty"`
	EndTime      int64              `json:"endTime,omitempty"`
}

// GetDuration calculates the duration of the execution in milliseconds.
func (n *NodeExecutionRecord) GetDuration() int64 {
	return getDuration(n.StartTime, n.EndTime)
}

// ExecutionAttempt represents a single execution attempt of a node.
type ExecutionAttempt struct {
	Attempt   int        `json:"attempt"`
	Timestamp int64      `json:"timestamp"`
	Status    FlowStatus `json:"status"`
	StartTime int64      `json:"startTime"`
	EndTime   int64      `json:"endTime"`
}

// GetDuration calculates the duration of the execution attempt in milliseconds.
func (e *ExecutionAttempt) GetDuration() int64 {
	return getDuration(e.StartTime, e.EndTime)
}

// getDuration calculates the duration between startTime and endTime in milliseconds.
func getDuration(startTime int64, endTime int64) int64 {
	if startTime == 0 || endTime == 0 {
		return 0
	}
	return (endTime - startTime) * 1000
}

// CompleteFlowDefinition represents a complete flow definition with all details.
//
//nolint:lll // Schema tags are intentionally verbose.
type CompleteFlowDefinition struct {
	ID            string           `json:"id" yaml:"id" jsonschema:"Unique identifier of the flow. UUID format."`
	Handle        string           `json:"handle" yaml:"handle" jsonschema:"URL-friendly handle for the flow."`
	Name          string           `json:"name" yaml:"name" jsonschema:"Display name of the flow."`
	FlowType      FlowType         `json:"flowType" yaml:"flowType" jsonschema:"Type of flow (AUTHENTICATION or REGISTRATION)."`
	ActiveVersion int              `json:"activeVersion,omitempty" yaml:"activeVersion" jsonschema:"Current active version number of the flow."`
	Nodes         []NodeDefinition `json:"nodes,omitempty" yaml:"nodes" jsonschema:"List of nodes defining the flow logic."`
	CreatedAt     string           `json:"createdAt,omitempty" yaml:"createdAt" jsonschema:"Timestamp when the flow was created."`
	UpdatedAt     string           `json:"updatedAt,omitempty" yaml:"updatedAt" jsonschema:"Timestamp when the flow was last updated."`
	IsReadOnly    bool             `json:"isReadOnly" yaml:"isReadOnly" jsonschema:"Whether the flow is immutable (declarative)."`
}

// NodeLayout represents the layout information for a node in the flow composer UI.
//
//nolint:lll // Schema tags are intentionally verbose.
type NodeLayout struct {
	Size     *NodeSize     `json:"size,omitempty" yaml:"size,omitempty" jsonschema:"Dimensions of the node."`
	Position *NodePosition `json:"position,omitempty" yaml:"position,omitempty" jsonschema:"Coordinates of the node on the canvas."`
}

// NodeSize represents the dimensions of a node.
type NodeSize struct {
	Width  float64 `json:"width" yaml:"width" jsonschema:"Width of the node in pixels."`
	Height float64 `json:"height" yaml:"height" jsonschema:"Height of the node in pixels."`
}

// NodePosition represents the position of a node on the canvas.
type NodePosition struct {
	X float64 `json:"x" yaml:"x" jsonschema:"X-coordinate of the node."`
	Y float64 `json:"y" yaml:"y" jsonschema:"Y-coordinate of the node."`
}

// NodeDefinition represents a single node in a flow definition.
//
//nolint:lll // Schema tags are intentionally verbose.
type NodeDefinition struct {
	ID           string                 `json:"id" yaml:"id" jsonschema:"Unique node identifier within the flow. Example: 'start', 'username-password', 'end'"`
	Type         string                 `json:"type" yaml:"type" jsonschema:"Node type: 'START' (entry point), 'END' (exit point), 'TASK_EXECUTION' (backend logic), or 'PROMPT' (user input)"`
	Layout       *NodeLayout            `json:"layout,omitempty" yaml:"layout,omitempty" jsonschema:"Optional UI layout information for flow composer (position and size on canvas)"`
	Meta         interface{}            `json:"meta,omitempty" yaml:"meta,omitempty" jsonschema:"Optional metadata. For PROMPT nodes, must include 'components' array for UI rendering. See existing flows for examples."`
	Prompts      []PromptDefinition     `json:"prompts,omitempty" yaml:"prompts,omitempty" jsonschema:"For PROMPT nodes: defines user inputs and actions. Each prompt has inputs (form fields) and an action (what happens on submit)."`
	Variant      NodeVariant            `json:"variant,omitempty" yaml:"variant,omitempty" jsonschema:"Optional PROMPT node variant. Use 'LOGIN_OPTIONS' to enable login option filtering on this node."`
	Next         string                 `json:"next,omitempty" yaml:"next,omitempty" jsonschema:"For display-only PROMPT nodes: ID of the next node. Mutually exclusive with 'prompts'."`
	Message      string                 `json:"message,omitempty" yaml:"message,omitempty" jsonschema:"For display-only PROMPT nodes: textual message for non-verbose mode."`
	Properties   map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty" jsonschema:"Optional node-specific properties for configuration"`
	Executor     *ExecutorDefinition    `json:"executor,omitempty" yaml:"executor,omitempty" jsonschema:"For TASK_EXECUTION nodes: defines which executor to run (e.g., 'UsernamePasswordAuthenticator', 'OTPGenerator')"`
	OnSuccess    string                 `json:"onSuccess,omitempty" yaml:"onSuccess,omitempty" jsonschema:"ID of the next node to execute on successful completion"`
	OnFailure    string                 `json:"onFailure,omitempty" yaml:"onFailure,omitempty" jsonschema:"ID of the next node to execute on failure"`
	OnIncomplete string                 `json:"onIncomplete,omitempty" yaml:"onIncomplete,omitempty" jsonschema:"For TASK_EXECUTION nodes: ID of the PROMPT node to forward to when user input is required."`
	Condition    *ConditionDefinition   `json:"condition,omitempty" yaml:"condition,omitempty" jsonschema:"Optional condition to determine if this node should execute"`
}

// InputDefinition represents an input parameter for a node.
//
//nolint:lll // Schema tags are intentionally verbose.
type InputDefinition struct {
	Ref        string                     `json:"ref,omitempty" yaml:"ref,omitempty" jsonschema:"Reference ID for the input."`
	Type       string                     `json:"type" yaml:"type" jsonschema:"Input type (e.g., 'text', 'password', 'email')."`
	Identifier string                     `json:"identifier" yaml:"identifier" jsonschema:"Field identifier or name."`
	Required   bool                       `json:"required" yaml:"required" jsonschema:"Whether this input is mandatory."`
	Validation []ValidationRuleDefinition `json:"validation,omitempty" yaml:"validation,omitempty" jsonschema:"Server-enforced validation rules applied to the submitted value."`
}

// ValidationRuleDefinition represents a single validation constraint on an input.
// Type is one of "regex", "minLength", or "maxLength"; Value holds the constraint
// parameter (string for regex, number for length types); Message is an i18n key or
// literal string returned in fieldErrors when the rule fails.
type ValidationRuleDefinition struct {
	Type    string      `json:"type" yaml:"type" jsonschema:"Rule type: regex, minLength, or maxLength."`
	Value   interface{} `json:"value" yaml:"value" jsonschema:"Constraint value: regex pattern or length."`
	Message string      `json:"message,omitempty" yaml:"message,omitempty" jsonschema:"Message when rule fails."`
}

// ActionDefinition represents an action to be executed by a node.
type ActionDefinition struct {
	Ref      string `json:"ref" yaml:"ref" jsonschema:"Reference ID for the action."`
	Type     string `json:"type,omitempty" yaml:"type,omitempty" jsonschema:"Action type for the next executor."`
	NextNode string `json:"nextNode" yaml:"nextNode" jsonschema:"Target node ID when action is taken."`
}

// PromptDefinition groups inputs with an action for prompt nodes.
type PromptDefinition struct {
	Inputs []InputDefinition `json:"inputs,omitempty" yaml:"inputs,omitempty" jsonschema:"User input fields."`
	Action *ActionDefinition `json:"action,omitempty" yaml:"action,omitempty" jsonschema:"Submission action."`
}

// ExecutorDefinition represents the executor configuration for a node.
type ExecutorDefinition struct {
	Name   string            `json:"name" yaml:"name" jsonschema:"Executor name."`
	Mode   string            `json:"mode,omitempty" yaml:"mode,omitempty" jsonschema:"Execution mode."`
	Inputs []InputDefinition `json:"inputs,omitempty" yaml:"inputs,omitempty" jsonschema:"Static executor inputs."`
}

// ConditionDefinition represents a condition for node execution.
type ConditionDefinition struct {
	Key    string `json:"key" yaml:"key" jsonschema:"Attribute key to check."`
	Value  string `json:"value" yaml:"value" jsonschema:"Value to match."`
	OnSkip string `json:"onSkip" yaml:"onSkip" jsonschema:"Node ID to skip to if condition is not met."`
}

// nodeDefinitionAlias avoids infinite recursion when NodeDefinition custom YAML methods delegate to the standard codec.
type nodeDefinitionAlias NodeDefinition

// MarshalYAML implements custom YAML marshaling for NodeDefinition.
// It converts the Meta interface{} field to a JSON-encoded string for proper serialization.
func (nd *NodeDefinition) MarshalYAML() (interface{}, error) {
	alias := nodeDefinitionAlias(*nd)

	if alias.Meta == nil {
		return alias, nil
	}

	metaJSON, err := json.Marshal(alias.Meta)
	if err != nil {
		return nil, err
	}

	alias.Meta = string(metaJSON)

	return alias, nil
}

// UnmarshalYAML implements custom YAML unmarshaling for NodeDefinition.
// It parses the Meta field from a JSON-encoded string back to interface{}.
func (nd *NodeDefinition) UnmarshalYAML(value *yaml.Node) error {
	var alias nodeDefinitionAlias

	if err := value.Decode(&alias); err != nil {
		return err
	}

	*nd = NodeDefinition(alias)

	if metaStr, ok := nd.Meta.(string); ok && metaStr != "" {
		var metaData interface{}
		if err := json.Unmarshal([]byte(metaStr), &metaData); err != nil {
			return nil
		}
		nd.Meta = metaData
	}

	return nil
}
