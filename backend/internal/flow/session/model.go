// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"time"

	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/eventlistener"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
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

// Type is the kind of session.
type Type string

const (
	// TypeSSO is a session a Session node established, which later logins can reuse.
	TypeSSO Type = "SSO"
	// TypeDirect is the session of one login through a flow without a Session node.
	TypeDirect Type = "DIRECT"
)

// TerminationReason names the path that ended a session.
type TerminationReason string

const (
	// TerminationReasonSignOut is a sign-out of one session.
	TerminationReasonSignOut TerminationReason = "sign_out"
	// TerminationReasonSubjectRevocation is an administrative termination of all the subject's sessions.
	TerminationReasonSubjectRevocation TerminationReason = "subject_revocation"
)

// TerminatedSession describes a session the service has ended. Participants is captured before the
// rows are deleted, so it is the only remaining record of which applications shared the session.
type TerminatedSession struct {
	SessionID    string
	SubjectID    string
	Participants []Participant
	Reason       TerminationReason
}

// TerminationListener is told about every terminated session after the terminating transaction
// commits. The contract is the eventlistener package's.
type TerminationListener = eventlistener.Listener[TerminatedSession]

// TerminationHook registers termination listeners. Initialize hands it to the composition root.
type TerminationHook = eventlistener.Hook[TerminatedSession]

// Session is the lean, hot-path SSO session entity. It carries only operational fields plus
// authenticated_at (used by the max_age policy check), so the resolve, SSO-check, and
// activity-touch paths never load the durable session context.
//
// The write-once auth-event facts (completed steps + sanitized claim snapshot) live in the
// sibling SessionContext (SESSION_AUTH_CONTEXT, 1:1 by session id), loaded only on the SSO path.
type Session struct {
	// SessionID is the primary key and the session's public identifier: every application in the
	// session receives the same value. Unlike HandleID it is not a credential. It is a UUIDv7, so it
	// carries the session's creation time, and changing what is published would need a migration.
	SessionID string
	// Type is the kind of session. A direct session has no flow execution or handle.
	Type Type
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

	// Properties is the client metadata recorded on the session, stored as JSON in the PROPERTIES column.
	Properties SessionProperties

	// State is the lifecycle state of the session.
	State State
	// Version is the optimistic-lock token, incremented on every successful update.
	Version int
}

// idleSlide is one refresh of a session's idle deadline: where it moves the deadline, and the bounds a
// session must be within to be moved.
type idleSlide struct {
	// now is the session's new last-active time.
	now time.Time
	// idleExpiresAt is the session's new idle deadline.
	idleExpiresAt time.Time
	// refreshedBefore throttles writes: a session refreshed after it is not moved.
	refreshedBefore time.Time
	// activeAfter and createdAfter keep the idle and absolute timeouts in effect now: a session last
	// active, or created, before them has ended.
	activeAfter  time.Time
	createdAfter time.Time
}

// Participant records an application that has used (joined) an SSO session. A session is shared
// across the applications that authenticate through its flow; each such application is tracked so
// the session's audience is known — the basis for logout and subject-scoped revocation. An application
// signed in under several organization units is one participant per unit, each with its own token family.
type Participant struct {
	// SessionID is the owning session's internal id.
	SessionID string
	// AppID is the participating application's id.
	AppID string
	// OUID is the organization unit the application signed in under, or empty when the sign-in had none.
	OUID string
	// TokenFamilyID is the token family id (tfid) the application's grants in the session share under this
	// organization unit, until it is revoked. Empty for participants recorded before tfid was introduced.
	TokenFamilyID string
	// SubjectID is the subject of the session the participation belongs to. Only ListByAppID fills it, and it
	// is empty when the session row is gone.
	SubjectID string
	// FirstJoinedAt is when the application first joined the session (write-once).
	FirstJoinedAt time.Time
	// LastActiveAt is refreshed each time the application reuses the session.
	LastActiveAt time.Time
}

// SSOInputs are the transient, request-scoped inputs the SSO-Check, Session and auth assert nodes need to
// resolve or record a session: the inbound handle for the current flow and the flow's identity/version
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

// SessionProperties is the client metadata recorded on a session so it can be recognized later. It is
// stored as one JSON document, so a new field needs no schema change.
type SessionProperties struct {
	// UserAgent is the User-Agent of the client that established the session, recorded once at creation.
	UserAgent string `json:"userAgent,omitempty"`
	// CreationIP is the client IP address the session was established from, recorded once at creation.
	CreationIP string `json:"creationIp,omitempty"`
	// LastActiveIP is the client IP address the session was last used from: recorded at creation, and
	// refreshed when a login flow reuses the session or the subject authenticates again inside it.
	LastActiveIP string `json:"lastActiveIp,omitempty"`
}

// ClientInfo is the request-scoped client metadata recorded on a session so it can be recognized
// later (the device and the network location it was used from).
type ClientInfo struct {
	// IP is the client IP address.
	IP string
	// UserAgent is the client's User-Agent header.
	UserAgent string
}

// sessionDetail is a live session together with the applications that have joined it.
type sessionDetail struct {
	Session
	Participants []Participant
}

// sessionPage is one page of a session listing, with the total number of live sessions that match.
type sessionPage struct {
	Sessions     []sessionDetail
	TotalResults int
}

// participantResponse is an application that has joined a session under an organization unit. OUID is
// omitted when the sign-in had none, and AppName and AppLogoURL when they can't be resolved.
type participantResponse struct {
	AppID         string `json:"appId"`
	OUID          string `json:"ouId,omitempty"`
	AppName       string `json:"appName,omitempty"`
	AppLogoURL    string `json:"appLogoUrl,omitempty"`
	FirstJoinedAt string `json:"firstJoinedAt"`
	LastActiveAt  string `json:"lastActiveAt"`
}

// sessionResponse is the API view of a live session. It never carries the session handle, the
// checkpoint contexts or the token family ids. UserDisplayName is omitted when it can't be resolved, and
// IsCurrent is set only on the caller's own sessions.
type sessionResponse struct {
	ID                string                `json:"id"`
	UserID            string                `json:"userId"`
	UserDisplayName   string                `json:"userDisplayName,omitempty"`
	UserAgent         string                `json:"userAgent,omitempty"`
	CreationIP        string                `json:"creationIp,omitempty"`
	LastActiveIP      string                `json:"lastActiveIp,omitempty"`
	AuthenticatedAt   string                `json:"authenticatedAt"`
	CreatedAt         string                `json:"createdAt"`
	LastActiveAt      string                `json:"lastActiveAt"`
	IdleExpiresAt     string                `json:"idleExpiresAt,omitempty"`
	AbsoluteExpiresAt string                `json:"absoluteExpiresAt,omitempty"`
	IsCurrent         *bool                 `json:"isCurrent,omitempty"`
	Participants      []participantResponse `json:"participants"`
}

// sessionListResponse is the paginated body of GET /sessions and GET /users/me/sessions.
type sessionListResponse struct {
	TotalResults int               `json:"totalResults"`
	StartIndex   int               `json:"startIndex"`
	Count        int               `json:"count"`
	Sessions     []sessionResponse `json:"sessions"`
	Links        []sysutils.Link   `json:"links"`
}

// appDisplay is the name and logo of a participating application, shown with the session.
type appDisplay struct {
	name    string
	logoURL string
}

// sessionEvent is an audited session event: its name and the action it records, chosen together.
type sessionEvent struct {
	name   string
	action audit.Action
}

var (
	eventSessionCreate = sessionEvent{name: audit.EventSessionCreate, action: audit.ActionCreate}
	eventSessionReuse  = sessionEvent{name: audit.EventSessionReuse, action: audit.ActionReuse}
	eventSessionEnd    = sessionEvent{name: audit.EventSessionEnd, action: audit.ActionEnd}
)

// sessionAudit is what one audit event of the session service records. The target is always the user. A zero
// actor leaves the actor to the security context of the call.
type sessionAudit struct {
	event       sessionEvent
	userID      string
	outcome     audit.Outcome
	actor       audit.Actor
	sessionID   string
	application string
	endReason   TerminationReason
}
