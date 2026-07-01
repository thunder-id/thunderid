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

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

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

// InterceptorContext is the per-invocation context built by the InterceptorService for each
// interceptor call. It is assembled from the EngineContext plus the matched interceptor
// definition's properties and the cross-request SharedData.
type InterceptorContext struct {
	Context context.Context

	// Flow identity
	ExecutionID string
	AppID       string
	FlowType    providers.FlowType

	// Mode is the lifecycle point at which this interceptor is executing.
	Mode providers.InterceptorMode

	// Engine state
	FlowStatus          providers.FlowStatus
	UserInputs          map[string]string
	CurrentNodeID       string
	NodeType            common.NodeType
	ExecutionPolicy     *providers.ExecutionPolicy
	AllowSegmentRestart bool
	CurrentNodeInputs   []providers.Input
	ForwardedData       map[string]interface{}
	AdditionalData      map[string]string

	// SharedData is interceptor-layer state shared across interceptors and preserved across
	// the requests of a single flow instance. Interceptors may read and write this map directly.
	// Each interceptor is responsible for reading any information it needs from SharedData and
	// populating relevant values into EngineOutputs.
	SharedData map[string]string
}
