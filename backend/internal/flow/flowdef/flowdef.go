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

// Package flowdef holds the flow-definition model types shared between the flow
// management service and the flow execution engine. It is a driver-free leaf
// package so that the engine (flowexec) can reference the flow definition without
// importing the DB-backed flowmgt package.
//
//nolint:lll
package flowdef

import (
	"encoding/json"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/flow/common"
)

// CompleteFlowDefinition represents a complete flow definition with all details.
type CompleteFlowDefinition struct {
	ID            string                  `json:"id" yaml:"id" jsonschema:"Unique identifier of the flow. UUID format."`
	Handle        string                  `json:"handle" yaml:"handle" jsonschema:"URL-friendly handle for the flow."`
	Name          string                  `json:"name" yaml:"name" jsonschema:"Display name of the flow."`
	FlowType      common.FlowType         `json:"flowType" yaml:"flowType" jsonschema:"Type of flow (AUTHENTICATION or REGISTRATION)."`
	ActiveVersion int                     `json:"activeVersion,omitempty" yaml:"activeVersion" jsonschema:"Current active version number of the flow."`
	Interceptors  []InterceptorDefinition `json:"interceptors,omitempty" yaml:"interceptors,omitempty" jsonschema:"Interceptor declarations for cross-cutting concerns."`
	Nodes         []NodeDefinition        `json:"nodes,omitempty" yaml:"nodes" jsonschema:"List of nodes defining the flow logic."`
	CreatedAt     string                  `json:"createdAt,omitempty" yaml:"createdAt" jsonschema:"Timestamp when the flow was created."`
	UpdatedAt     string                  `json:"updatedAt,omitempty" yaml:"updatedAt" jsonschema:"Timestamp when the flow was last updated."`
	IsReadOnly    bool                    `json:"isReadOnly" yaml:"isReadOnly" jsonschema:"Whether the flow is immutable (declarative)."`
}

// InterceptorDefinition describes how an interceptor is declared in the flow definition.
type InterceptorDefinition struct {
	Name       string                  `json:"name" yaml:"name"`
	Mode       common.InterceptorMode  `json:"mode" yaml:"mode"`
	Scope      common.InterceptorScope `json:"scope,omitempty" yaml:"scope,omitempty"`
	ApplyTo    []string                `json:"applyTo,omitempty" yaml:"applyTo,omitempty"`
	Properties map[string]interface{}  `json:"properties,omitempty" yaml:"properties,omitempty"`
}

// NodeLayout represents the layout information for a node in the flow composer UI.
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
type NodeDefinition struct {
	ID           string                 `json:"id" yaml:"id" jsonschema:"Unique node identifier within the flow. Example: 'start', 'username-password', 'end'"`
	Type         string                 `json:"type" yaml:"type" jsonschema:"Node type: 'START' (entry point), 'END' (exit point), 'TASK_EXECUTION' (backend logic), or 'PROMPT' (user input)"`
	Layout       *NodeLayout            `json:"layout,omitempty" yaml:"layout,omitempty" jsonschema:"Optional UI layout information for flow composer (position and size on canvas)"`
	Meta         interface{}            `json:"meta,omitempty" yaml:"meta,omitempty" jsonschema:"Optional metadata. For PROMPT nodes, must include 'components' array for UI rendering. See existing flows for examples."`
	Prompts      []PromptDefinition     `json:"prompts,omitempty" yaml:"prompts,omitempty" jsonschema:"For PROMPT nodes: defines user inputs and actions. Each prompt has inputs (form fields) and an action (what happens on submit)."`
	Variant      common.NodeVariant     `json:"variant,omitempty" yaml:"variant,omitempty" jsonschema:"Optional PROMPT node variant. Use 'LOGIN_OPTIONS' to enable login option filtering on this node."`
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
	Type    string      `json:"type" yaml:"type" jsonschema:"Rule type: 'regex', 'minLength', or 'maxLength'."`
	Value   interface{} `json:"value" yaml:"value" jsonschema:"Constraint value: regex pattern (string) or length (number)."`
	Message string      `json:"message,omitempty" yaml:"message,omitempty" jsonschema:"i18n key or literal message returned when the rule fails."`
}

// ActionDefinition represents an action to be executed by a node.
type ActionDefinition struct {
	Ref      string `json:"ref" yaml:"ref" jsonschema:"Reference ID for the action."`
	Type     string `json:"type,omitempty" yaml:"type,omitempty" jsonschema:"Action type. Forwarded to next executor to determine the action to take."`
	NextNode string `json:"nextNode" yaml:"nextNode" jsonschema:"ID of the node to transition to when this action is taken."`
}

// PromptDefinition groups inputs with an action for prompt nodes.
type PromptDefinition struct {
	Inputs []InputDefinition `json:"inputs,omitempty" yaml:"inputs,omitempty" jsonschema:"List of input fields shown to the user."`
	Action *ActionDefinition `json:"action,omitempty" yaml:"action,omitempty" jsonschema:"Action to take upon submission."`
}

// ExecutorDefinition represents the executor configuration for a node.
type ExecutorDefinition struct {
	Name   string            `json:"name" yaml:"name" jsonschema:"Name of the executor (e.g., 'UsernamePasswordAuthenticator')."`
	Mode   string            `json:"mode,omitempty" yaml:"mode,omitempty" jsonschema:"Execution mode or configuration."`
	Inputs []InputDefinition `json:"inputs,omitempty" yaml:"inputs,omitempty" jsonschema:"Static inputs or configuration parameters for the executor."`
}

// ConditionDefinition represents a condition for node execution.
type ConditionDefinition struct {
	Key    string `json:"key" yaml:"key" jsonschema:"Attribute key to check."`
	Value  string `json:"value" yaml:"value" jsonschema:"Value to match."`
	OnSkip string `json:"onSkip" yaml:"onSkip" jsonschema:"Node ID to skip to if condition is not met."`
}

// nodeDefinitionAlias is used to avoid infinite recursion during marshaling/unmarshaling.
type nodeDefinitionAlias NodeDefinition

// MarshalYAML implements custom YAML marshaling for NodeDefinition.
// It converts the Meta interface{} field to a JSON-encoded string for proper serialization.
func (nd *NodeDefinition) MarshalYAML() (interface{}, error) {
	// Create an alias to avoid infinite recursion
	alias := nodeDefinitionAlias(*nd)

	// If Meta is nil or empty, marshal as-is
	if alias.Meta == nil {
		return alias, nil
	}

	// JSON-encode the Meta field to preserve its structure
	metaJSON, err := json.Marshal(alias.Meta)
	if err != nil {
		return nil, err
	}

	// Replace Meta with the JSON string
	alias.Meta = string(metaJSON)

	return alias, nil
}

// UnmarshalYAML implements custom YAML unmarshaling for NodeDefinition.
// It parses the Meta field from a JSON-encoded string back to interface{}.
func (nd *NodeDefinition) UnmarshalYAML(value *yaml.Node) error {
	// Create an alias to avoid infinite recursion
	var alias nodeDefinitionAlias

	// Unmarshal into the alias
	if err := value.Decode(&alias); err != nil {
		return err
	}

	// Copy all fields from alias to nd
	*nd = NodeDefinition(alias)

	// If Meta is a string, try to parse it as JSON
	if metaStr, ok := nd.Meta.(string); ok && metaStr != "" {
		var metaData interface{}
		if err := json.Unmarshal([]byte(metaStr), &metaData); err != nil {
			// If JSON parsing fails, keep the string value
			// This allows backward compatibility with non-JSON Meta values
			return nil
		}
		nd.Meta = metaData
	}

	return nil
}
