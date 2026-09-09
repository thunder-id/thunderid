// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzen

type subject struct {
	Type       string                 `json:"type,omitempty"`
	ID         string                 `json:"id,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

type resource struct {
	Type       string                 `json:"type,omitempty"`
	ID         string                 `json:"id,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

type action struct {
	Name       string                 `json:"name,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

type evaluationRequest struct {
	Subject  subject                `json:"subject"`
	Resource resource               `json:"resource"`
	Action   action                 `json:"action"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

type evaluationResponse struct {
	Decision bool                   `json:"decision"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

type evaluationsRequest struct {
	Evaluations []evaluationRequest `json:"evaluations,omitempty"`
}

type evaluationsResponse struct {
	Evaluations []evaluationResponse `json:"evaluations"`
}

type searchActionRequest struct {
	Subject  subject  `json:"subject"`
	Resource resource `json:"resource"`
}

type searchActionResponse struct {
	Results []action `json:"results"`
}

type metadataResponse struct {
	PolicyDecisionPoint       string `json:"policy_decision_point"`
	AccessEvaluationEndpoint  string `json:"access_evaluation_endpoint"`
	AccessEvaluationsEndpoint string `json:"access_evaluations_endpoint"`
	SearchActionEndpoint      string `json:"search_action_endpoint"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type createResourceRequest struct {
	Name        string  `json:"name"`
	Handle      string  `json:"handle"`
	Description string  `json:"description,omitempty"`
	Parent      *string `json:"parent"`
}

type resourceResponse struct {
	ID         string `json:"id"`
	Handle     string `json:"handle"`
	Permission string `json:"permission"`
}
