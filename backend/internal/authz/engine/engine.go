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

package engine

import "context"

// AuthorizationEngine is the internal interface for authorization engines.
// This interface is NOT exported and is used internally by the authorization service.
// Different engines can be plugged in (RBAC, ABAC, ReBAC, Custom) by implementing this interface.
type AuthorizationEngine interface {
	// EvaluateAccess evaluates a single fine-grained access request.
	EvaluateAccess(
		ctx context.Context,
		request AccessEvaluationRequest,
	) (*AccessEvaluationResponse, error)

	// EvaluateAccessBatch evaluates multiple fine-grained access requests.
	EvaluateAccessBatch(
		ctx context.Context,
		request AccessEvaluationsRequest,
	) (*AccessEvaluationsResponse, error)
}

// Subject identifies the principal for an access evaluation.
type Subject struct {
	Type       string
	ID         string
	GroupIDs   []string
	Properties map[string]interface{}
}

// ResourceServer identifies the resource server for an access evaluation.
type ResourceServer struct {
	ID         string
	Properties map[string]interface{}
}

// Permission identifies the permission string being evaluated.
type Permission struct {
	Name       string
	Properties map[string]interface{}
}

// AccessEvaluationRequest represents a single fine-grained access evaluation request.
type AccessEvaluationRequest struct {
	Subject        Subject
	ResourceServer ResourceServer
	Permission     Permission
	Context        map[string]interface{}
}

// AccessEvaluationResponse represents a single fine-grained access evaluation response.
type AccessEvaluationResponse struct {
	Decision bool
	Context  map[string]interface{}
}

// AccessEvaluationsRequest represents a batched fine-grained access evaluation request.
type AccessEvaluationsRequest struct {
	Evaluations []AccessEvaluationRequest
}

// AccessEvaluationsResponse represents a batched fine-grained access evaluation response.
type AccessEvaluationsResponse struct {
	Evaluations []AccessEvaluationResponse
}
