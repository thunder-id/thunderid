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
	"context"
	"time"
)

// State represents the lifecycle state of a session.
type State string

const (
	// StateActive indicates the session is live and may back an SSO decision.
	StateActive State = "ACTIVE"
	// StateRevoked indicates the session was explicitly revoked and must not be resumed.
	StateRevoked State = "REVOKED"
	// StateEnded indicates the session ended (e.g. logout) and must not be resumed.
	StateEnded State = "ENDED"
)

// Session is the lean, hot-path SSO session entity. It carries only operational fields plus
// authenticated_at (used by the max_age policy check), so the resolve, SSO-check, and
// activity-touch paths never load the durable session context.
//
// The write-once auth-event facts (completed steps + sanitized claim snapshot) live in the
// sibling SessionContext (SESSION_AUTH_CONTEXT, 1:1 by session id), loaded only on the SSO path.
type Session struct {
	// SessionID is the internal primary key, never exposed to clients.
	SessionID string
	// SubjectID is the authenticated subject (user) the session belongs to.
	SubjectID string
	// FlowID is the flow this session is grouped under (the SSO group key).
	FlowID string
	// FlowVersion is the flow definition version the session was established at.
	FlowVersion int
	// FlowExecutionID is the id of the flow execution that established this session. It is unique per
	// session (enforced by a DB constraint), so concurrent joins within one execution converge on a
	// single session instead of minting duplicates. It is set once at establishment and never changes
	// on reuse by later executions.
	FlowExecutionID string

	// HandleID is the opaque handle that references this session (the cookie value). It has no
	// expiry of its own; session lifetime is governed by the idle and absolute deadlines.
	HandleID string

	// AuthenticatedAt is when the subject most recently authenticated for this session.
	AuthenticatedAt time.Time
	// CreatedAt is when the session row was created.
	CreatedAt time.Time
	// LastActiveAt is refreshed each time the session backs a flow execution.
	LastActiveAt time.Time

	// IdleExpiresAt slides forward on each activity touch; AbsoluteExpiresAt is fixed at creation.
	// Both are enforced by the resolver, which rejects a session past either deadline.
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time

	// State is the lifecycle state of the session.
	State State
	// Version is the optimistic-lock token, incremented on every successful update.
	Version int
}

// Participant records an application that has used (joined) an SSO session. A session is shared
// across the applications that authenticate through its flow; each such application is tracked so
// the session's audience is known — the basis for logout and subject-scoped revocation.
type Participant struct {
	// SessionID is the owning session's internal id.
	SessionID string
	// AppID is the participating application's id.
	AppID string
	// TokenFamilyID is the token family id (tfid) minted for this application's most recent grant in
	// the session. It links the session to the grant's tokens so logout can revoke the whole family.
	// Refreshed on each re-authorization (latest grant wins); empty for participants recorded before
	// tfid was introduced.
	TokenFamilyID string
	// FirstJoinedAt is when the application first joined the session (write-once).
	FirstJoinedAt time.Time
	// LastActiveAt is refreshed each time the application reuses the session.
	LastActiveAt time.Time
}

// SSOInputs are the transient, request-scoped inputs the SSO-Check and Session nodes need to resolve
// or establish a session: the inbound handle for the current flow and the flow's identity/version
// (the SSO group key). They are carried on the Go context.Context rather than a NodeContext field, so
// they never persist with the flow context and never enter the reusable engine's public contract.
type SSOInputs struct {
	// Handle is the inbound session handle carried for the current flow (empty if none).
	Handle string
	// FlowID is the current flow's id (the SSO group key).
	FlowID string
	// FlowVersion is the current active version of the flow definition.
	FlowVersion int
}

type ssoInputsContextKey struct{}

// WithSSOInputs returns a context carrying the SSO inputs for the current flow execution.
func WithSSOInputs(ctx context.Context, in SSOInputs) context.Context {
	return context.WithValue(ctx, ssoInputsContextKey{}, in)
}

// SSOInputsFrom returns the SSO inputs carried on the context, or the zero value if none were set.
func SSOInputsFrom(ctx context.Context) SSOInputs {
	if in, ok := ctx.Value(ssoInputsContextKey{}).(SSOInputs); ok {
		return in
	}
	return SSOInputs{}
}
