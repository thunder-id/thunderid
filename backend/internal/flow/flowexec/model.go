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

package flowexec

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appmodel "github.com/asgardeo/thunder/internal/application/model"
	managerpkg "github.com/asgardeo/thunder/internal/authnprovider/manager"
	"github.com/asgardeo/thunder/internal/flow/common"
	"github.com/asgardeo/thunder/internal/flow/core"
	"github.com/asgardeo/thunder/internal/system/crypto"
	"github.com/asgardeo/thunder/internal/system/crypto/runtime"
)

// EngineContext holds the overall context used by the flow engine during execution.
type EngineContext struct {
	Context context.Context

	ExecutionID    string
	FlowType       common.FlowType
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
	Application appmodel.Application

	AuthUser         managerpkg.AuthUser
	Assertion        string
	ExecutionHistory map[string]*common.NodeExecutionRecord

	ChallengeTokenIn   string
	ChallengeTokenHash string
}

// FlowStep represents the outcome of a individual flow step
type FlowStep struct {
	ExecutionID    string
	StepID         string
	Type           common.FlowStepType
	Status         common.FlowStatus
	ChallengeToken string
	Data           FlowData
	Assertion      string
	FailureReason  string
}

// FlowData holds the data returned by a flow execution step
type FlowData struct {
	Inputs         []common.Input    `json:"inputs,omitempty"`
	RedirectURL    string            `json:"redirectURL,omitempty"`
	Actions        []common.Action   `json:"actions,omitempty"`
	Meta           interface{}       `json:"meta,omitempty"`
	AdditionalData map[string]string `json:"additionalData,omitempty"`
}

// FlowResponse represents the flow execution API response body
type FlowResponse struct {
	ExecutionID    string   `json:"executionId"`
	StepID         string   `json:"stepId,omitempty"`
	FlowStatus     string   `json:"flowStatus"`
	Type           string   `json:"type,omitempty"`
	ChallengeToken string   `json:"challengeToken,omitempty"`
	Data           FlowData `json:"data,omitempty"`
	Assertion      string   `json:"assertion,omitempty"`
	FailureReason  string   `json:"failureReason,omitempty"`
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
}

// FlowContextDB represents the database row for a flow context.
type FlowContextDB struct {
	ExecutionID string
	Context     string
	ExpiryTime  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// isEncrypted reports whether the Context field is still in encrypted form.
func (f *FlowContextDB) isEncrypted() bool {
	var encCheck struct {
		Algorithm string `json:"alg"`
	}
	return json.Unmarshal([]byte(f.Context), &encCheck) == nil && encCheck.Algorithm != ""
}

// decrypt decrypts the Context field in-place if it is still encrypted.
func (f *FlowContextDB) decrypt(ctx context.Context) error {
	if !f.isEncrypted() {
		return nil
	}
	decrypted, err := runtime.GetRuntimeCryptoService().Decrypt(
		ctx, crypto.KeyRef{}, crypto.AlgorithmParams{Algorithm: crypto.AlgorithmAESGCM}, []byte(f.Context))
	if err != nil {
		return err
	}
	f.Context = string(decrypted)
	return nil
}

// flowContextContent holds all flow state serialized into the CONTEXT JSON column.
type flowContextContent struct {
	AppID               string  `json:"appId"`
	Verbose             bool    `json:"verbose"`
	CurrentNodeID       *string `json:"currentNodeId,omitempty"`
	CurrentAction       *string `json:"currentAction,omitempty"`
	CurrentSegmentID    *string `json:"currentSegmentId,omitempty"`
	GraphID             string  `json:"graphId"`
	RuntimeData         *string `json:"runtimeData,omitempty"`
	ExecutionHistory    *string `json:"executionHistory,omitempty"`
	IsAuthenticated     bool    `json:"isAuthenticated"`
	UserID              *string `json:"userId,omitempty"`
	OUID                *string `json:"ouId,omitempty"`
	UserType            *string `json:"userType,omitempty"`
	UserInputs          *string `json:"userInputs,omitempty"`
	UserAttributes      *string `json:"userAttributes,omitempty"`
	Token               *string `json:"token,omitempty"`
	AvailableAttributes *string `json:"availableAttributes,omitempty"`
	AuthUser            *string `json:"authUser,omitempty"`
	ChallengeTokenHash  *string `json:"challengeTokenHash,omitempty"`
}

// encrypt marshals and encrypts the content, returning the encrypted string.
func (c *flowContextContent) encrypt(ctx context.Context) (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	encrypted, _, err := runtime.GetRuntimeCryptoService().Encrypt(
		ctx, crypto.KeyRef{}, crypto.AlgorithmParams{Algorithm: crypto.AlgorithmAESGCM}, data)
	if err != nil {
		return "", err
	}
	return string(encrypted), nil
}

// GetGraphID extracts the graph ID from the context JSON.
func (f *FlowContextDB) GetGraphID(ctx context.Context) (string, error) {
	if err := f.decrypt(ctx); err != nil {
		return "", err
	}
	var content flowContextContent
	if err := json.Unmarshal([]byte(f.Context), &content); err != nil {
		return "", err
	}
	return content.GraphID, nil
}

// ToEngineContext converts the database model to the flow engine context.
func (f *FlowContextDB) ToEngineContext(ctx context.Context, graph core.GraphInterface) (EngineContext, error) {
	// Ensure context is decrypted before parsing
	if err := f.decrypt(ctx); err != nil {
		return EngineContext{}, err
	}
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

	// Parse execution history
	var executionHistory map[string]*common.NodeExecutionRecord
	if content.ExecutionHistory != nil {
		if err := json.Unmarshal([]byte(*content.ExecutionHistory), &executionHistory); err != nil {
			return EngineContext{}, err
		}
	} else {
		executionHistory = make(map[string]*common.NodeExecutionRecord)
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
	var authUser managerpkg.AuthUser
	if content.AuthUser != nil {
		if err := json.Unmarshal([]byte(*content.AuthUser), &authUser); err != nil {
			return EngineContext{}, err
		}
	}

	// Get challenge token hash from JSON content
	challengeTokenHash := ""
	if content.ChallengeTokenHash != nil {
		challengeTokenHash = *content.ChallengeTokenHash
	}

	return EngineContext{
		Context:            ctx,
		ExecutionID:        f.ExecutionID,
		TraceID:            "", // TraceID is transient and set from request context
		FlowType:           graph.GetType(),
		AppID:              content.AppID,
		Verbose:            content.Verbose,
		UserInputs:         userInputs,
		RuntimeData:        runtimeData,
		CurrentNode:        currentNode,
		CurrentAction:      currentAction,
		CurrentSegmentID:   currentSegmentID,
		Graph:              graph,
		AuthUser:           authUser,
		ExecutionHistory:   executionHistory,
		ChallengeTokenHash: challengeTokenHash,
	}, nil
}

// FromEngineContext creates a database model from the flow engine context.
func FromEngineContext(ctx EngineContext) (*FlowContextDB, error) {
	// Serialize user inputs
	userInputsJSON, err := json.Marshal(ctx.UserInputs)
	if err != nil {
		return nil, err
	}
	userInputs := string(userInputsJSON)

	// Serialize runtime data
	runtimeDataJSON, err := json.Marshal(ctx.RuntimeData)
	if err != nil {
		return nil, err
	}
	runtimeData := string(runtimeDataJSON)

	// Serialize execution history
	executionHistoryJSON, err := json.Marshal(ctx.ExecutionHistory)
	if err != nil {
		return nil, err
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

	// Serialize AuthUser if present
	var authUserStr *string
	if ctx.AuthUser.IsSet() {
		authUserJSON, err := json.Marshal(&ctx.AuthUser)
		if err != nil {
			return nil, err
		}
		s := string(authUserJSON)
		authUserStr = &s
	}

	// Get graph ID
	if ctx.Graph == nil || ctx.Graph.GetID() == "" {
		return nil, fmt.Errorf("graph with a valid ID is required to persist engine context")
	}
	graphID := ctx.Graph.GetID()

	// Get challenge token hash
	var challengeTokenHash *string
	if ctx.ChallengeTokenHash != "" {
		challengeTokenHash = &ctx.ChallengeTokenHash
	}

	content := flowContextContent{
		AppID:              ctx.AppID,
		Verbose:            ctx.Verbose,
		CurrentNodeID:      currentNodeID,
		CurrentAction:      currentAction,
		CurrentSegmentID:   currentSegmentID,
		GraphID:            graphID,
		RuntimeData:        &runtimeData,
		ExecutionHistory:   &executionHistory,
		UserInputs:         &userInputs,
		AuthUser:           authUserStr,
		ChallengeTokenHash: challengeTokenHash,
	}

	encryptedContext, err := content.encrypt(ctx.Context)
	if err != nil {
		return nil, err
	}

	return &FlowContextDB{
		ExecutionID: ctx.ExecutionID,
		Context:     encryptedContext,
	}, nil
}
