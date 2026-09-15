// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzen

// Subject identifies the principal in an AuthZEN access evaluation request.
type Subject struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// Resource identifies the protected resource in an AuthZEN access evaluation request.
// Type is the ThunderID resource server identifier. ID identifies the resource instance.
type Resource struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// Action identifies the operation in an AuthZEN access evaluation request.
type Action struct {
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// AccessEvaluationRequest represents a single AuthZEN access evaluation request.
type AccessEvaluationRequest struct {
	Subject  Subject                `json:"subject"`
	Resource Resource               `json:"resource"`
	Action   Action                 `json:"action"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// AccessEvaluationResponse represents a single AuthZEN access evaluation response.
type AccessEvaluationResponse struct {
	Decision bool                   `json:"decision"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// AccessEvaluationsRequest represents a batched AuthZEN access evaluations request.
type AccessEvaluationsRequest struct {
	Evaluations []AccessEvaluationRequest `json:"evaluations"`
}

// AccessEvaluationsResponse represents a batched AuthZEN access evaluations response.
type AccessEvaluationsResponse struct {
	Evaluations []AccessEvaluationResponse `json:"evaluations"`
}

// AccessActionSearchRequest represents an AuthZEN action search request.
type AccessActionSearchRequest struct {
	Subject  Subject                `json:"subject"`
	Resource Resource               `json:"resource"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// AccessSearchResponse represents an AuthZEN search response.
type AccessSearchResponse struct {
	Results []Action `json:"results"`
}

// MetadataResponse represents AuthZEN PDP metadata.
type MetadataResponse struct {
	PolicyDecisionPoint       string `json:"policy_decision_point"`
	AccessEvaluationEndpoint  string `json:"access_evaluation_endpoint"`
	AccessEvaluationsEndpoint string `json:"access_evaluations_endpoint,omitempty"`
	SearchActionEndpoint      string `json:"search_action_endpoint,omitempty"`
}
