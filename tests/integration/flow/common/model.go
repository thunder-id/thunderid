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

type I18nMessage struct {
	Key          string `json:"key,omitempty"`
	DefaultValue string `json:"defaultValue,omitempty"`
}

// TestSuiteConfig holds common configuration for test suites
type TestSuiteConfig struct {
	CreatedUserIDs    []string
	CreatedFlowIDs    []string
	CreatedIdpIDs     []string
	CreatedSenderIDs  []string
	CreatedGroupIDs   []string
	CreatedRoleIDs    []string
	OriginalAppConfig map[string]interface{}
	MockServer        interface{} // Can be cast to specific mock server type
}

type FlowStep struct {
	ExecutionID    string   `json:"executionId"`
	FlowStatus     string   `json:"flowStatus"`
	Type           string   `json:"type,omitempty"`
	Data           FlowData `json:"data,omitempty"`
	Assertion      string   `json:"assertion,omitempty"`
	Error          *ErrorResponse `json:"error,omitempty"`
	ChallengeToken string   `json:"challengeToken,omitempty"`
}

type FlowData struct {
	Inputs         []Inputs          `json:"inputs,omitempty"`
	Actions        []Action          `json:"actions,omitempty"`
	RedirectURL    string            `json:"redirectURL,omitempty"`
	AdditionalData map[string]string `json:"additionalData,omitempty"`
	Meta           interface{}       `json:"meta,omitempty"`
	FieldErrors    []FieldError      `json:"fieldErrors,omitempty"`
}

type Inputs struct {
	Ref        string           `json:"ref,omitempty"`
	Identifier string           `json:"identifier"`
	Type       string           `json:"type"`
	Required   bool             `json:"required"`
	Options    []string         `json:"options,omitempty"`
	Validation []ValidationRule `json:"validation,omitempty"`
}

type ValidationRule struct {
	Type    string      `json:"type"`
	Value   interface{} `json:"value"`
	Message string      `json:"message,omitempty"`
}

type FieldError struct {
	Identifier string `json:"identifier"`
	Message    string `json:"message"`
}

type Action struct {
	Ref      string `json:"ref"`
	NextNode string `json:"nextNode,omitempty"`
}

type ErrorResponse struct {
	Code        string      `json:"code"`
	Message     I18nMessage `json:"message"`
	Description I18nMessage `json:"description"`
}
