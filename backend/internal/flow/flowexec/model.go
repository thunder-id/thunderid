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

package flowexec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
)

// frame captures the per-call execution state saved when a CALL node pushes execution
// into a callee flow and restored when the callee returns.
type frame struct {
	graph               core.GraphInterface
	flowType            providers.FlowType
	currentNode         core.NodeInterface
	currentNodeResponse *common.NodeResponse
	currentAction       string
	currentSegmentID    string
	runtimeData         map[string]string
	forwardedData       map[string]interface{}
	additionalData      map[string]string
	resumeCallNodeID    string
}

// EngineContext holds the overall context used by the flow engine during execution.
//
// TODO: fields on EngineContext are currently exposed directly. Convert to unexported
// fields accessed via getters and setters so that mutation can be encapsulated.
type EngineContext struct {
	Context context.Context

	ExecutionID    string
	FlowType       providers.FlowType
	AppID          string
	Verbose        bool
	UserInputs     map[string]string
	RuntimeData    map[string]string
	ForwardedData  map[string]interface{}
	AdditionalData map[string]string
	TraceID        string

	CurrentNode         core.NodeInterface
	CurrentNodeResponse *common.NodeResponse
	CurrentAction       string
	CurrentSegmentID    string

	Graph       core.GraphInterface
	Application providers.Application

	AuthenticatedUser authncm.AuthenticatedUser
	AuthUser          providers.AuthUser
	Assertion         string
	ExecutionHistory  map[string]*providers.NodeExecutionRecord

	InterceptorSharedData map[string]string
	// consumedInputs accumulates identifiers reported as consumed by executors and
	// interceptors within the current request
	consumedInputs []string
	// frameStack holds saved call frames. Top is the most recent caller.
	frameStack []*frame
	// sharedRuntimeData is a cross-frame key-value store available to executors that opt in.
	sharedRuntimeData map[string]string
	// SSOHandleIn carries the inbound SSO handle for this request. It is transient: read from
	// the transport at the start of execution and never persisted with the flow context.
	SSOHandleIn string
	// SSOFlowVersion is the current active version of this flow's definition, captured from the
	// flow fetched when the context is loaded. Transient; used by the SSO-Check node to reject
	// sessions established at an incompatible flow version.
	SSOFlowVersion int
}

// mergeRuntimeData merges the given data into RuntimeData.
func (ec *EngineContext) mergeRuntimeData(data map[string]string) {
	if ec.RuntimeData == nil {
		ec.RuntimeData = make(map[string]string)
	}
	for k, v := range data {
		ec.RuntimeData[k] = v
	}
}

// pushFrame saves the current execution state as a new frame. Call this before swapping context to the callee.
func (e *EngineContext) pushFrame(resumeCallNodeID string) {
	f := &frame{
		graph:               e.Graph,
		flowType:            e.FlowType,
		currentNode:         e.CurrentNode,
		currentNodeResponse: e.CurrentNodeResponse,
		currentAction:       e.CurrentAction,
		currentSegmentID:    e.CurrentSegmentID,
		runtimeData:         e.RuntimeData,
		forwardedData:       e.ForwardedData,
		additionalData:      e.AdditionalData,
		resumeCallNodeID:    resumeCallNodeID,
	}
	e.frameStack = append(e.frameStack, f)
}

// popFrame restores the most-recently-pushed frame and removes it from the stack.
// Returns nil when the stack is empty.
func (e *EngineContext) popFrame() *frame {
	if len(e.frameStack) == 0 {
		return nil
	}
	top := e.frameStack[len(e.frameStack)-1]
	e.frameStack = e.frameStack[:len(e.frameStack)-1]

	e.Graph = top.graph
	e.FlowType = top.flowType
	e.CurrentNode = top.currentNode
	e.CurrentNodeResponse = top.currentNodeResponse
	e.CurrentAction = top.currentAction
	e.CurrentSegmentID = top.currentSegmentID
	e.RuntimeData = top.runtimeData
	e.ForwardedData = top.forwardedData
	e.AdditionalData = top.additionalData
	return top
}

// frameDepth returns the number of saved frames (0 means root flow).
func (e *EngineContext) frameDepth() int {
	return len(e.frameStack)
}

// setSharedRuntimeData writes a value into the cross-frame shared runtime data bucket.
func (e *EngineContext) setSharedRuntimeData(key, value string) {
	if e.sharedRuntimeData == nil {
		e.sharedRuntimeData = make(map[string]string)
	}
	e.sharedRuntimeData[key] = value
}

// getSharedRuntimeData returns a value from the cross-frame shared runtime data bucket.
func (e *EngineContext) getSharedRuntimeData(key string) (string, bool) {
	if e.sharedRuntimeData == nil {
		return "", false
	}
	v, ok := e.sharedRuntimeData[key]
	return v, ok
}

// InterceptorRunnerContext is a self-contained, request-scoped context built by the engine
// for each RunInterceptors call. It carries everything the interceptor service needs without
// requiring access to the engine context itself.
//
// TODO: fields on EngineContext are currently exposed directly. Convert to unexported
// fields accessed via getters and setters so that mutation can be encapsulated.
type InterceptorRunnerContext struct {
	Ctx                  context.Context
	ExecutionID          string
	AppID                string
	FlowType             providers.FlowType
	FlowStatus           providers.FlowStatus
	CurrentNodeID        string
	NodeType             common.NodeType
	SkipInterceptors     []string
	ExecutionPolicy      *providers.ExecutionPolicy
	AllowSegmentRestart  bool
	UserInputs           map[string]string
	ForwardedData        map[string]interface{}
	AdditionalData       map[string]string
	CurrentNodeInputs    []providers.Input
	ResolvedInterceptors []core.InterceptorUnitInterface
	SharedData           map[string]string
	// consumedInputs accumulates identifiers reported as consumed by interceptors
	// during this RunInterceptors call
	consumedInputs []string
}

// AppendConsumedInputs appends the given keys to the accumulator of inputs consumed during
// this RunInterceptors call.
func (c *InterceptorRunnerContext) AppendConsumedInputs(keys []string) {
	if len(keys) == 0 {
		return
	}
	if c.consumedInputs == nil {
		c.consumedInputs = make([]string, 0, len(keys))
	}
	c.consumedInputs = append(c.consumedInputs, keys...)
}

// GetConsumedInputs returns the keys reported via ConsumeInput by interceptors during this
// RunInterceptors call.
func (c *InterceptorRunnerContext) GetConsumedInputs() []string {
	return c.consumedInputs
}

// FlowStep represents the outcome of a individual flow step
type FlowStep struct {
	ExecutionID    string
	StepID         string
	Type           common.FlowStepType
	Status         providers.FlowStatus
	ChallengeToken string
	Data           FlowData
	Assertion      string
	Error          *tidcommon.ServiceError

	// SSOHandleOut / SSOFlowID carry an SSO session handle minted during this step back to the
	// transport layer (the handler), which sets it as a per-flow cookie. They are not part of
	// the JSON response body.
	SSOHandleOut string
	SSOFlowID    string
}

// FlowData holds the data returned by a flow execution step
type FlowData struct {
	Inputs         []providers.Input   `json:"inputs,omitempty"`
	RedirectURL    string              `json:"redirectURL,omitempty"`
	Actions        []common.Action     `json:"actions,omitempty"`
	Meta           interface{}         `json:"meta,omitempty"`
	AdditionalData map[string]string   `json:"additionalData,omitempty"`
	FieldErrors    []common.FieldError `json:"fieldErrors,omitempty"`
}

// FlowResponse represents the flow execution API response body
type FlowResponse struct {
	ExecutionID    string                  `json:"executionId"`
	StepID         string                  `json:"stepId,omitempty"`
	FlowStatus     string                  `json:"flowStatus"`
	Type           string                  `json:"type,omitempty"`
	ChallengeToken string                  `json:"challengeToken,omitempty"`
	Data           FlowData                `json:"data,omitempty"`
	Assertion      string                  `json:"assertion,omitempty"`
	Error          *apierror.ErrorResponse `json:"error,omitempty"`
}

// FlowRequest represents the flow execution API request body
type FlowRequest struct {
	ApplicationID  string            `json:"applicationId"`
	FlowType       string            `json:"flowType"`
	Verbose        bool              `json:"verbose,omitempty"`
	ExecutionID    string            `json:"executionId"`
	ChallengeToken string            `json:"challengeToken,omitempty"`
	Action         string            `json:"action"`
	Inputs         map[string]string `json:"inputs"`
}

// FlowInitContext represents the context for initiating a new flow with runtime data
type FlowInitContext struct {
	ApplicationID string
	FlowType      string
	RuntimeData   map[string]string
	InitialInputs map[string]string
	ExpirySeconds int64
}

// FlowContextDB represents the database row for a flow context.
type FlowContextDB struct {
	ExecutionID string
	Context     string
	ExpiryTime  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// serializedFrame is the on-disk representation of a single call frame.
type serializedFrame struct {
	GraphID          string  `json:"graphId"`
	CurrentNodeID    *string `json:"currentNodeId,omitempty"`
	CurrentAction    *string `json:"currentAction,omitempty"`
	CurrentSegmentID *string `json:"currentSegmentId,omitempty"`
	RuntimeData      *string `json:"runtimeData,omitempty"`
	ResumeCallNodeID string  `json:"resumeCallNodeId,omitempty"`
}

// flowContextContent holds all flow state serialized into the CONTEXT JSON column.
type flowContextContent struct {
	AppID                 string  `json:"appId"`
	Verbose               bool    `json:"verbose"`
	CurrentNodeID         *string `json:"currentNodeId,omitempty"`
	CurrentAction         *string `json:"currentAction,omitempty"`
	CurrentSegmentID      *string `json:"currentSegmentId,omitempty"`
	GraphID               string  `json:"graphId"`
	RuntimeData           *string `json:"runtimeData,omitempty"`
	ExecutionHistory      *string `json:"executionHistory,omitempty"`
	IsAuthenticated       bool    `json:"isAuthenticated"`
	UserID                *string `json:"userId,omitempty"`
	OUID                  *string `json:"ouId,omitempty"`
	UserType              *string `json:"userType,omitempty"`
	UserInputs            *string `json:"userInputs,omitempty"`
	UserAttributes        *string `json:"userAttributes,omitempty"`
	Token                 *string `json:"token,omitempty"`
	AvailableAttributes   *string `json:"availableAttributes,omitempty"`
	AuthUser              *string `json:"authUser,omitempty"`
	InterceptorSharedData *string `json:"interceptorSharedData,omitempty"`
	FrameStack            *string `json:"frameStack,omitempty"`
	SharedRuntimeData     *string `json:"sharedRuntimeData,omitempty"`
}

// graphResolverFunc resolves a flow graph by its flow ID. Used during context deserialization to
// hydrate saved call frames.
type graphResolverFunc func(ctx context.Context, flowID string) (core.GraphInterface, error)

// GetGraphID extracts the graph ID from the context JSON.
func (f *FlowContextDB) GetGraphID(_ context.Context) (string, error) {
	var content flowContextContent
	if err := json.Unmarshal([]byte(f.Context), &content); err != nil {
		return "", err
	}
	return content.GraphID, nil
}

// ToEngineContext converts the database model to the flow engine context.
func (f *FlowContextDB) ToEngineContext(ctx context.Context,
	graph core.GraphInterface, resolveGraph graphResolverFunc) (EngineContext, error) {
	var content flowContextContent
	if err := json.Unmarshal([]byte(f.Context), &content); err != nil {
		return EngineContext{}, err
	}
	// Parse user inputs
	var userInputs map[string]string
	if content.UserInputs != nil {
		if err := json.Unmarshal([]byte(*content.UserInputs), &userInputs); err != nil {
			return EngineContext{}, err
		}
	} else {
		userInputs = make(map[string]string)
	}

	// Parse runtime data
	var runtimeData map[string]string
	if content.RuntimeData != nil {
		if err := json.Unmarshal([]byte(*content.RuntimeData), &runtimeData); err != nil {
			return EngineContext{}, err
		}
	} else {
		runtimeData = make(map[string]string)
	}

	// Parse authenticated user attributes
	var userAttributes map[string]interface{}
	if content.UserAttributes != nil {
		if err := json.Unmarshal([]byte(*content.UserAttributes), &userAttributes); err != nil {
			return EngineContext{}, err
		}
	} else {
		userAttributes = make(map[string]interface{})
	}

	var token string
	if content.Token != nil {
		token = *content.Token
	}

	// Parse available attributes
	var availableAttributes *providers.AttributesResponse
	if content.AvailableAttributes != nil && strings.TrimSpace(*content.AvailableAttributes) != "" {
		var attrs providers.AttributesResponse
		if err := json.Unmarshal([]byte(*content.AvailableAttributes), &attrs); err != nil {
			return EngineContext{}, err
		}
		availableAttributes = &attrs
	}

	// Build authenticated user
	authenticatedUser := authncm.AuthenticatedUser{
		IsAuthenticated:     content.IsAuthenticated,
		UserID:              "",
		Attributes:          userAttributes,
		Token:               token,
		AvailableAttributes: availableAttributes,
	}
	if content.UserID != nil {
		authenticatedUser.UserID = *content.UserID
	}
	if content.OUID != nil {
		authenticatedUser.OUID = *content.OUID
	}
	if content.UserType != nil {
		authenticatedUser.UserType = *content.UserType
	}

	// Parse execution history
	var executionHistory map[string]*providers.NodeExecutionRecord
	if content.ExecutionHistory != nil {
		if err := json.Unmarshal([]byte(*content.ExecutionHistory), &executionHistory); err != nil {
			return EngineContext{}, err
		}
	} else {
		executionHistory = make(map[string]*providers.NodeExecutionRecord)
	}

	// Get current node from graph if available
	var currentNode core.NodeInterface
	if content.CurrentNodeID != nil {
		if node, exists := graph.GetNode(*content.CurrentNodeID); exists {
			currentNode = node
		}
	}

	// Get current action
	currentAction := ""
	if content.CurrentAction != nil {
		currentAction = *content.CurrentAction
	}

	// Get current segment ID
	currentSegmentID := ""
	if content.CurrentSegmentID != nil {
		currentSegmentID = *content.CurrentSegmentID
	}

	// Deserialize AuthUser if present
	var authUser providers.AuthUser
	if content.AuthUser != nil {
		if err := json.Unmarshal([]byte(*content.AuthUser), &authUser); err != nil {
			return EngineContext{}, err
		}
	}

	// Parse interceptor shared data
	var interceptorSharedData map[string]string
	if content.InterceptorSharedData != nil {
		if err := json.Unmarshal([]byte(*content.InterceptorSharedData), &interceptorSharedData); err != nil {
			return EngineContext{}, err
		}
	} else {
		interceptorSharedData = make(map[string]string)
	}

	// Parse frame stack
	frameStack, err := f.deserializeFrameStack(ctx, content, resolveGraph)
	if err != nil {
		return EngineContext{}, err
	}

	// Parse shared runtime data
	var sharedRuntimeData map[string]string
	if content.SharedRuntimeData != nil {
		if err := json.Unmarshal([]byte(*content.SharedRuntimeData), &sharedRuntimeData); err != nil {
			return EngineContext{}, err
		}
	}

	return EngineContext{
		Context:               ctx,
		ExecutionID:           f.ExecutionID,
		TraceID:               "", // TraceID is transient and set from request context
		FlowType:              graph.GetType(),
		AppID:                 content.AppID,
		Verbose:               content.Verbose,
		UserInputs:            userInputs,
		RuntimeData:           runtimeData,
		CurrentNode:           currentNode,
		CurrentAction:         currentAction,
		CurrentSegmentID:      currentSegmentID,
		Graph:                 graph,
		AuthenticatedUser:     authenticatedUser,
		AuthUser:              authUser,
		ExecutionHistory:      executionHistory,
		InterceptorSharedData: interceptorSharedData,
		frameStack:            frameStack,
		sharedRuntimeData:     sharedRuntimeData,
	}, nil
}

// FromEngineContext converts an EngineContext to the database model for persistence.
func (f *FlowContextDB) FromEngineContext(ctx EngineContext) error {
	// Serialize user inputs
	userInputsJSON, err := json.Marshal(ctx.UserInputs)
	if err != nil {
		return err
	}
	userInputs := string(userInputsJSON)

	// Serialize runtime data
	runtimeDataJSON, err := json.Marshal(ctx.RuntimeData)
	if err != nil {
		return err
	}
	runtimeData := string(runtimeDataJSON)

	// Serialize authenticated user attributes
	userAttributesJSON, err := json.Marshal(ctx.AuthenticatedUser.Attributes)
	if err != nil {
		return err
	}
	userAttributes := string(userAttributesJSON)

	// Serialize execution history
	executionHistoryJSON, err := json.Marshal(ctx.ExecutionHistory)
	if err != nil {
		return err
	}
	executionHistory := string(executionHistoryJSON)

	// Get current node ID
	var currentNodeID *string
	if ctx.CurrentNode != nil {
		nodeID := ctx.CurrentNode.GetID()
		currentNodeID = &nodeID
	}

	// Get current action
	var currentAction *string
	if ctx.CurrentAction != "" {
		currentAction = &ctx.CurrentAction
	}

	// Get current segment ID
	var currentSegmentID *string
	if ctx.CurrentSegmentID != "" {
		currentSegmentID = &ctx.CurrentSegmentID
	}

	// Get authenticated user ID
	var authenticatedUserID *string
	if ctx.AuthenticatedUser.UserID != "" {
		authenticatedUserID = &ctx.AuthenticatedUser.UserID
	}

	// Get organization unit ID
	var oUID *string
	if ctx.AuthenticatedUser.OUID != "" {
		oUID = &ctx.AuthenticatedUser.OUID
	}

	// Get user type
	var userType *string
	if ctx.AuthenticatedUser.UserType != "" {
		userType = &ctx.AuthenticatedUser.UserType
	}

	var token *string
	if ctx.AuthenticatedUser.Token != "" {
		token = &ctx.AuthenticatedUser.Token
	}

	// Serialize available attributes
	var availableAttributes *string
	if ctx.AuthenticatedUser.AvailableAttributes != nil {
		availableAttrsJSON, err := json.Marshal(ctx.AuthenticatedUser.AvailableAttributes)
		if err != nil {
			return err
		}
		availableAttrsStr := string(availableAttrsJSON)
		availableAttributes = &availableAttrsStr
	}

	// Serialize AuthUser if present
	var authUserStr *string
	if ctx.AuthUser.IsAuthenticated() {
		authUserJSON, err := json.Marshal(&ctx.AuthUser)
		if err != nil {
			return err
		}
		s := string(authUserJSON)
		authUserStr = &s
	}

	// Get graph ID
	if ctx.Graph == nil || ctx.Graph.GetID() == "" {
		return fmt.Errorf("graph with a valid ID is required to persist engine context")
	}
	graphID := ctx.Graph.GetID()

	// Serialize interceptorSharedData
	interceptorSharedDataJSON, err := json.Marshal(ctx.InterceptorSharedData)
	if err != nil {
		return err
	}
	interceptorSharedData := string(interceptorSharedDataJSON)

	// Serialize frame stack
	frameStackStr, err := f.serializeFrameStack(ctx.frameStack)
	if err != nil {
		return err
	}

	// Serialize shared runtime data
	var sharedRuntimeDataStr *string
	if len(ctx.sharedRuntimeData) > 0 {
		srdJSON, err := json.Marshal(ctx.sharedRuntimeData)
		if err != nil {
			return err
		}
		s := string(srdJSON)
		sharedRuntimeDataStr = &s
	}

	content := flowContextContent{
		AppID:                 ctx.AppID,
		Verbose:               ctx.Verbose,
		CurrentNodeID:         currentNodeID,
		CurrentAction:         currentAction,
		CurrentSegmentID:      currentSegmentID,
		GraphID:               graphID,
		RuntimeData:           &runtimeData,
		ExecutionHistory:      &executionHistory,
		IsAuthenticated:       ctx.AuthenticatedUser.IsAuthenticated,
		UserID:                authenticatedUserID,
		OUID:                  oUID,
		UserType:              userType,
		UserInputs:            &userInputs,
		UserAttributes:        &userAttributes,
		Token:                 token,
		AvailableAttributes:   availableAttributes,
		AuthUser:              authUserStr,
		InterceptorSharedData: &interceptorSharedData,
		FrameStack:            frameStackStr,
		SharedRuntimeData:     sharedRuntimeDataStr,
	}

	contextJSON, err := json.Marshal(content)
	if err != nil {
		return err
	}

	f.ExecutionID = ctx.ExecutionID
	f.Context = string(contextJSON)
	return nil
}

// deserializeFrameStack reconstructs call frames from persisted content.
// Returns nil without error when FrameStack is absent or resolveGraph is nil.
func (f *FlowContextDB) deserializeFrameStack(ctx context.Context,
	content flowContextContent, resolveGraph graphResolverFunc) ([]*frame, error) {
	if content.FrameStack == nil || resolveGraph == nil {
		return nil, nil
	}

	var serializedFrames []serializedFrame
	if err := json.Unmarshal([]byte(*content.FrameStack), &serializedFrames); err != nil {
		return nil, err
	}

	frames := make([]*frame, 0, len(serializedFrames))
	for _, sf := range serializedFrames {
		frameGraph, err := resolveGraph(ctx, sf.GraphID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve frame graph %s: %w", sf.GraphID, err)
		}

		var currentNode core.NodeInterface
		if sf.CurrentNodeID != nil {
			if n, exists := frameGraph.GetNode(*sf.CurrentNodeID); exists {
				currentNode = n
			}
		}

		var currentAction, currentSegmentID string
		if sf.CurrentAction != nil {
			currentAction = *sf.CurrentAction
		}
		if sf.CurrentSegmentID != nil {
			currentSegmentID = *sf.CurrentSegmentID
		}

		var runtimeData map[string]string
		if sf.RuntimeData != nil {
			if err := json.Unmarshal([]byte(*sf.RuntimeData), &runtimeData); err != nil {
				return nil, err
			}
		}

		frames = append(frames, &frame{
			graph:            frameGraph,
			flowType:         frameGraph.GetType(),
			currentNode:      currentNode,
			currentAction:    currentAction,
			currentSegmentID: currentSegmentID,
			runtimeData:      runtimeData,
			resumeCallNodeID: sf.ResumeCallNodeID,
		})
	}

	return frames, nil
}

// serializeFrameStack converts in-memory call frames to a JSON string pointer.
// Returns nil when the stack is empty.
func (f *FlowContextDB) serializeFrameStack(frameStack []*frame) (*string, error) {
	if len(frameStack) == 0 {
		return nil, nil
	}

	serializedFrames := make([]serializedFrame, 0, len(frameStack))
	for _, f := range frameStack {
		if f.graph == nil || f.graph.GetID() == "" {
			return nil, fmt.Errorf("frame graph with a valid ID is required to persist frame stack")
		}

		sf := serializedFrame{
			GraphID:          f.graph.GetID(),
			ResumeCallNodeID: f.resumeCallNodeID,
		}

		if f.currentNode != nil {
			nodeID := f.currentNode.GetID()
			sf.CurrentNodeID = &nodeID
		}
		if f.currentAction != "" {
			sf.CurrentAction = &f.currentAction
		}
		if f.currentSegmentID != "" {
			sf.CurrentSegmentID = &f.currentSegmentID
		}

		if len(f.runtimeData) > 0 {
			b, err := json.Marshal(f.runtimeData)
			if err != nil {
				return nil, err
			}
			s := string(b)
			sf.RuntimeData = &s
		}

		serializedFrames = append(serializedFrames, sf)
	}

	b, err := json.Marshal(serializedFrames)
	if err != nil {
		return nil, err
	}

	s := string(b)
	return &s, nil
}
