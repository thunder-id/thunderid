// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package event

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// DataKey provides standardized keys for Event.Data map.
// Using these constants prevents typos and makes refactoring easier.
//
// Usage:
//
//	evt.WithData(event.DataKey.ClientID, "client123")
//	evt.WithData(event.DataKey.Subject, "019d3279-78bc-7af0-8ea8-979a9c9a8cb7")
var DataKey = struct {
	// Identity & User Keys
	Username string
	ClientID string
	EntityID string

	// Principal & Delegation Keys.
	// The subject and actor axes are named after the token claims they mirror, so one vocabulary
	// covers both tokens and events: sub/sub_type for the principal a token is about, act_sub/act_type
	// for the principal acting on its behalf.
	ActorType   string
	ActorSub    string
	Subject     string
	SubjectType string
	IsDelegated string

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
	Message       string
	Error         string
	DurationMs    string
	LatencyUs     string
	TraceParent   string
	CorrelationID string

	// Testing Keys
	Key   string
	Value string
}{
	// Identity & User Keys
	Username: "username",
	ClientID: "client_id",
	EntityID: "app_id",

	// Principal & Delegation Keys
	ActorType:   "act_type",
	ActorSub:    "act_sub",
	Subject:     "sub",
	SubjectType: "sub_type",
	IsDelegated: "is_delegated",

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
	Message:       "message",
	Error:         "error",
	DurationMs:    "duration_ms",
	LatencyUs:     "latency_us",
	TraceParent:   "trace_parent",
	CorrelationID: "correlation_id",

	// Testing Keys
	Key:   "key",
	Value: "value",
}

// Principal type values reported by the act_type and sub_type keys. They match the wire values of
// the token's sub_type claim so one vocabulary describes a principal in both places.
const (
	PrincipalTypeUser        = "user"
	PrincipalTypeAgent       = "agent"
	PrincipalTypeApplication = "application"
)

// PrincipalType maps an entity category to the value reported for act_type and sub_type. The entity
// vocabulary spells an application "app" while the token claim spells it "application"; this is the
// single place that difference is reconciled. An unrecognized category returns an empty string, so
// the caller omits the field rather than reporting a category consumers cannot interpret.
func PrincipalType(entityCategory string) string {
	switch providers.EntityCategory(entityCategory) {
	case providers.EntityCategoryUser:
		return PrincipalTypeUser
	case providers.EntityCategoryAgent:
		return PrincipalTypeAgent
	case providers.EntityCategoryApp:
		return PrincipalTypeApplication
	default:
		return ""
	}
}
