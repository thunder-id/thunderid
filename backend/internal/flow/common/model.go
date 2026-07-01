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
	"fmt"
	"regexp"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// PrepareValidationRules compiles the regex pattern of every regex rule in place.
// An empty or non-string regex value is treated as a no-op.
func PrepareValidationRules(rules []providers.ValidationRule) error {
	for i := range rules {
		if rules[i].Type != providers.ValidationTypeRegex {
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

// Action represents an action to be executed in a flow step
type Action struct {
	Ref      string `json:"ref,omitempty"`
	Type     string `json:"type,omitempty"`
	NextNode string `json:"nextNode,omitempty"`
}

// Prompt groups inputs with an action for prompt nodes.
type Prompt struct {
	Inputs []providers.Input `json:"inputs,omitempty"`
	Action *Action           `json:"action,omitempty"`
}

// NodeResponse represents the response from a node execution
type NodeResponse struct {
	Status         NodeStatus              `json:"status"`
	Type           NodeResponseType        `json:"type"`
	Error          *tidcommon.ServiceError `json:"error,omitempty"`
	Inputs         []providers.Input       `json:"inputs,omitempty"`
	AdditionalData map[string]string       `json:"additionalData,omitempty"`
	RedirectURL    string                  `json:"redirectUrl,omitempty"`
	Actions        []Action                `json:"actions,omitempty"`
	Meta           interface{}             `json:"meta,omitempty"`
	NextNodeID     string                  `json:"nextNodeId,omitempty"`
	RuntimeData    map[string]string       `json:"runtimeData,omitempty"`
	ForwardedData  map[string]interface{}  `json:"forwardedData,omitempty"`
	Assertion      string                  `json:"assertion,omitempty"`
	FieldErrors    []FieldError            `json:"fieldErrors,omitempty"`
	AuthUser       providers.AuthUser      `json:"-"`
}

// InterceptorResponse represents the response from an interceptor execution
type InterceptorResponse struct {
	Status InterceptorStatus       `json:"status"`
	Error  *tidcommon.ServiceError `json:"error,omitempty"`

	// EngineOutputs passes data from interceptors back to the engine.
	EngineOutputs map[string]string `json:"engineOutputs,omitempty"`

	// Flow step data returned to the client.
	ChallengeToken string       `json:"challengeToken,omitempty"`
	FieldErrors    []FieldError `json:"fieldErrors,omitempty"`
}
