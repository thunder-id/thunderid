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
	"context"

	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/flow/common"
)

// NodeContext holds the context for a specific node in the flow execution.
type NodeContext struct {
	Context context.Context

	ExecutionID   string
	FlowType      common.FlowType
	EntityID      string
	Verbose       bool
	CurrentAction string
	CurrentNodeID string
	ExecutorMode  string

	NodeProperties map[string]interface{}
	NodeInputs     []common.Input
	UserInputs     map[string]string
	RuntimeData    map[string]string
	ForwardedData  map[string]interface{}

	Application       appmodel.Application
	AuthenticatedUser authncm.AuthenticatedUser
	AuthUser          manager.AuthUser
	ExecutionHistory  map[string]*common.NodeExecutionRecord
}

// NodeCondition represents a condition that must be met for a node to execute.
// If specified, the node will only execute when the resolved value of key matches value.
// OnSkip specifies which node to skip to if the condition is not met.
type NodeCondition struct {
	Key    string
	Value  string
	OnSkip string
}

// Segment represents a contiguous section of a flow graph bounded by display-only prompt nodes.
type Segment struct {
	ID          string
	StartNodeID string
}

// ExecutionPolicy defines behavioral policies for node execution.
type ExecutionPolicy struct {
	SkipChallengeValidation bool
	AllowSegmentRestart     bool
}
