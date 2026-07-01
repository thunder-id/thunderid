/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package authzen

type subject struct {
	Type string `json:"type,omitempty"`
	ID   string `json:"id,omitempty"`
}

type resource struct {
	Type string `json:"type,omitempty"`
	ID   string `json:"id,omitempty"`
}

type action struct {
	Name string `json:"name,omitempty"`
}

type evaluationRequest struct {
	Subject  subject  `json:"subject"`
	Resource resource `json:"resource"`
	Action   action   `json:"action"`
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
