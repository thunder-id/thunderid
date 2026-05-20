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

package common

import (
	"slices"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
)

// Input represents the inputs required for a node
type Input struct {
	Ref         string   `json:"ref,omitempty"`
	Identifier  string   `json:"identifier"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Options     []string `json:"options,omitempty"`
	DisplayName string   `json:"-"`
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
	Status            NodeStatus                `json:"status"`
	Type              NodeResponseType          `json:"type"`
	FailureReason     string                    `json:"failureReason,omitempty"`
	Inputs            []Input                   `json:"inputs,omitempty"`
	AdditionalData    map[string]string         `json:"additionalData,omitempty"`
	RedirectURL       string                    `json:"redirectUrl,omitempty"`
	Actions           []Action                  `json:"actions,omitempty"`
	Meta              interface{}               `json:"meta,omitempty"`
	NextNodeID        string                    `json:"nextNodeId,omitempty"`
	RuntimeData       map[string]string         `json:"runtimeData,omitempty"`
	ForwardedData     map[string]interface{}    `json:"forwardedData,omitempty"`
	AuthenticatedUser authncm.AuthenticatedUser `json:"authenticatedUser,omitempty"`
	Assertion         string                    `json:"assertion,omitempty"`
	AuthUser          authnprovidermgr.AuthUser `json:"-"`
}

// ExecutorResponse represents the response from an executor
type ExecutorResponse struct {
	Status            ExecutorStatus            `json:"status"`
	Inputs            []Input                   `json:"inputs,omitempty"`
	AdditionalData    map[string]string         `json:"additionalData,omitempty"`
	RedirectURL       string                    `json:"redirectUrl,omitempty"`
	RuntimeData       map[string]string         `json:"runtimeData,omitempty"`
	ForwardedData     map[string]interface{}    `json:"forwardedData,omitempty"`
	AuthenticatedUser authncm.AuthenticatedUser `json:"authenticatedUser,omitempty"`
	Assertion         string                    `json:"assertion,omitempty"`
	FailureReason     string                    `json:"failureReason,omitempty"`
	AuthUser          authnprovidermgr.AuthUser `json:"-"`
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
