// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"

	"github.com/thunder-id/thunderid/internal/flow/common"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ExternalIdentity is what an external party (a federated identity provider or a credential issuer)
// asserted, as stored under common.RuntimeKeyExternalIdentity. IdpID and Sub are set only for a
// federated connection.
type ExternalIdentity struct {
	IdpID  string                 `json:"idpId,omitempty"`
	Sub    string                 `json:"sub,omitempty"`
	Claims map[string]interface{} `json:"claims,omitempty"`
}

// Claim returns one claim as a string, and whether it is present. It is safe on a nil identity.
func (e *ExternalIdentity) Claim(name string) (string, bool) {
	if e == nil {
		return "", false
	}
	value, ok := e.Claims[name]
	if !ok {
		return "", false
	}
	return systemutils.ConvertInterfaceValueToString(value), true
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
