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

package event

// DataKey provides standardized keys for Event.Data map.
// Using these constants prevents typos and makes refactoring easier.
//
// Usage:
//
//	evt.WithData(event.DataKey.ClientID, "client123")
//	evt.WithData(event.DataKey.UserID, "user456")
var DataKey = struct {
	// Identity & User Keys
	UserID   string
	Username string
	ClientID string
	EntityID string

	// Flow Execution Keys
	ExecutionID   string
	FlowType      string
	NodeID        string
	NodeType      string
	NodeStatus    string
	ExecutorName  string
	ExecutorType  string
	StepNumber    string
	AttemptNumber string
	AuthMethod    string
	RedirectTo    string
	FailedStep    string

	// OAuth/Token Keys
	Scope            string
	GrantType        string
	JTI              string
	RevocationReason string

	// Event Metadata Keys
	Message     string
	Error       string
	DurationMs  string
	LatencyUs   string
	TraceParent string

	// Testing Keys
	Key   string
	Value string
}{
	// Identity & User Keys
	UserID:   "user_id",
	Username: "username",
	ClientID: "client_id",
	EntityID: "app_id",

	// Flow Execution Keys
	ExecutionID:   "execution_id",
	FlowType:      "flow_type",
	NodeID:        "node_id",
	NodeType:      "node_type",
	NodeStatus:    "node_status",
	ExecutorName:  "executor_name",
	ExecutorType:  "executor_type",
	StepNumber:    "step_number",
	AttemptNumber: "attempt_number",
	AuthMethod:    "auth_method",
	RedirectTo:    "redirect_to",
	FailedStep:    "failed_step",

	// OAuth/Token Keys
	Scope:            "scope",
	GrantType:        "grant_type",
	JTI:              "jti",
	RevocationReason: "revocation_reason",

	// Event Metadata Keys
	Message:     "message",
	Error:       "error",
	DurationMs:  "duration_ms",
	LatencyUs:   "latency_us",
	TraceParent: "trace_parent",

	// Testing Keys
	Key:   "key",
	Value: "value",
}
