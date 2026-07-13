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

package session

import (
	"encoding/json"
	"fmt"
)

// MaxSessionContextBytes bounds the serialized session context payload, keeping the sibling row
// small and preventing unbounded growth from accumulated step facts and claims.
const MaxSessionContextBytes = 16 * 1024

// SessionContext is a durable authenticated-context snapshot of a session at one checkpoint, stored
// in SSO_SESSION_CONTEXT keyed by (session_id, checkpoint_id) — 1:many, one row per checkpoint (join
// node) reached in the flow. It holds the runtime state the SSO skip replays at that join: the flow's
// RuntimeData (which carries the in-flow attribute manipulations and federated claims), the subject
// reference, and the completed steps keyed by node id. It is read only on the SSO load path — never
// touched by activity (last_active_at) updates.
//
// RuntimeData is persisted in full pending the flow-context data-classification revisit. Attributes
// are not materialized here: a local subject's attributes are re-resolved from the entity store via
// the subject reference on load, and a federated subject's authoritative claims are carried in
// RuntimeData. Aggregate facts used by hot-path policy checks (authenticated_at) live on SESSION,
// not here, so those checks never load this context.
type SessionContext struct {
	// SessionID is the owning session's internal id.
	SessionID string
	// CheckpointID identifies the checkpoint (join node) this snapshot belongs to; together with
	// SessionID it is the row's key. One session accumulates one snapshot per checkpoint it reaches.
	CheckpointID string
	// RuntimeData is the flow's runtime data captured at the save node. It is the durable carrier
	// of the effective attribute set — in-flow manipulations and federated claims included.
	RuntimeData map[string]string
	// AuthUser is the marshaled subject reference (resolved entity reference + a re-resolvable
	// attribute token), not materialized attributes; on load GetUserAttributes re-resolves the
	// subject's attributes fresh from the entity store.
	AuthUser json.RawMessage
	// CompletedSteps records the completed authentication steps keyed by node id.
	CompletedSteps map[string]StepFact
	// ContextVersion versions the context payload schema/content independently of the session.
	ContextVersion int
}

// StepFact is a per-node completed authentication-step fact.
type StepFact struct {
	Executor string `json:"executor,omitempty"`
	Status   string `json:"status,omitempty"`
	// CompletedAt is the Unix time (seconds) at which the authentication step completed.
	CompletedAt int64 `json:"completedAt,omitempty"`
}

// sessionContextPayload is the JSON form of the session context stored in the CONTEXT column. The
// context version is stored as its own column, not in the payload.
type sessionContextPayload struct {
	RuntimeData    map[string]string   `json:"runtimeData,omitempty"`
	AuthUser       json.RawMessage     `json:"authUser,omitempty"`
	CompletedSteps map[string]StepFact `json:"completedSteps,omitempty"`
}

// serializePayload renders the persistable portion of the session context to JSON.
func (c SessionContext) serializePayload() (string, error) {
	data, err := json.Marshal(sessionContextPayload{
		RuntimeData:    c.RuntimeData,
		AuthUser:       c.AuthUser,
		CompletedSteps: c.CompletedSteps,
	})
	if err != nil {
		return "", fmt.Errorf("failed to serialize session context: %w", err)
	}
	return string(data), nil
}

// parseSessionContextPayload parses the JSON payload of an session context.
func parseSessionContextPayload(raw string) (sessionContextPayload, error) {
	var payload sessionContextPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return sessionContextPayload{}, fmt.Errorf("failed to parse session context: %w", err)
	}
	return payload, nil
}
