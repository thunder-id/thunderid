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
//
// TODO: fields on InterceptorContext are currently exposed directly. Convert to unexported
// fields accessed via getters and setters so that mutation can be encapsulated.
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
	// consumedInputs accumulates identifiers of inputs the interceptor has used
	// up during this call
	consumedInputs []string
	// SharedData is interceptor-layer state shared across interceptors and preserved across
	// the requests of a single flow instance. Interceptors may read and write this map directly.
	// Each interceptor is responsible for reading any information it needs from SharedData and
	// populating relevant values into EngineOutputs.
	SharedData map[string]string
}

// ConsumeInput returns the value for key from UserInputs and records key on the consumed
// inputs list. Interceptors should prefer this over direct UserInputs access so the engine
// has a full audit trail of what was used.
func (ic *InterceptorContext) ConsumeInput(key string) (string, bool) {
	v, ok := ic.UserInputs[key]
	if ok {
		ic.consumedInputs = append(ic.consumedInputs, key)
	}
	return v, ok
}

// AppendConsumedInputs records the given keys on the consumed inputs list without
// reading from UserInputs.
func (ic *InterceptorContext) AppendConsumedInputs(keys []string) {
	if len(keys) == 0 {
		return
	}
	if ic.consumedInputs == nil {
		ic.consumedInputs = make([]string, 0, len(keys))
	}
	ic.consumedInputs = append(ic.consumedInputs, keys...)
}

// GetConsumedInputs returns the list of input keys that have been consumed by the interceptor
// during this call.
func (ic *InterceptorContext) GetConsumedInputs() []string {
	return ic.consumedInputs
}
