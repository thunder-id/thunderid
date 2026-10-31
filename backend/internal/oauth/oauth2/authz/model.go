// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"time"

	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// OAuthMessage represents the OAuth message.
type OAuthMessage struct {
	RequestType        string
	AuthID             string
	Resources          []string
	RequestHeaders     map[string][]string
	RequestQueryParams map[string][]string
	RequestBodyParams  map[string]string
}

// AuthorizationCode represents the authorization code.
type AuthorizationCode struct {
	CodeID              string
	Code                string
	ClientID            string
	RedirectURI         string
	RedirectURIProvided bool
	AuthorizedUserID    string
	AttributeCacheID    string
	TimeCreated         time.Time
	// AuthTime is when the subject actually authenticated, which on the SSO path predates
	// TimeCreated: a reused session authenticated once and every later authorization mints a fresh
	// code. It is kept separate from TimeCreated so the id_token's auth_time claim can report the
	// authentication while the code's own lifetime is still measured from its creation.
	AuthTime            time.Time
	ExpiryTime          time.Time
	Scopes              string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resources           []string
	ClaimsRequest       *oauth2model.ClaimsRequest
	ClaimsLocales       string
	Nonce               string
	CompletedACR        string
	DPoPJkt             string
	// TokenFamilyID is the token family id (tfid) minted during the login flow and carried on the flow
	// assertion. It is stamped onto the access and refresh tokens issued for this code so revocation
	// can target the whole family. Empty when the login flow issued no tfid (e.g. pre-rollout tokens).
	TokenFamilyID string
	// CorrelationID is the login flow's execution id, carried on the flow assertion. It is reported
	// on the token issuance events for this code so they correlate with the flow's own events.
	// Empty when the code was not minted from a flow assertion.
	CorrelationID string
	// SubjectID is the resource ID of the entity the flow authenticated, and SubjectCategory its
	// entity category. Both are carried on the flow assertion so the token issued for this code can
	// report the subject without resolving it again, and so the reported subject is the opaque
	// resource ID rather than the possibly-mapped token subject.
	SubjectID       string
	SubjectCategory string
	// SessionID is the SSO session id carried on the flow assertion, emitted in the ID token as the
	// sid claim. Empty when the flow established no session; unlike TokenFamilyID it is never minted here.
	SessionID string
	// SessionData is the SSO sessions the caller posted with the callback, kept when the client requested
	// the device_sso scope. The code exchange returns them in the device secret.
	SessionData []oauth2model.SessionData
	// AuthorizationDetails holds the RFC 9396 authorization details the user approved.
	AuthorizationDetails []providers.AuthorizationDetail `json:",omitempty"`
}

// AuthZPostResponse represents the response body for the authorization POST request.
type AuthZPostResponse struct {
	RedirectURI string `json:"redirect_uri"`
}

// SSOSessionAuthorization is a silent authorization request a client makes with an SSO session it
// presents, in place of a browser redirect.
type SSOSessionAuthorization struct {
	// SessionData is the SSO sessions the client presents; the one for its authentication flow is used.
	SessionData []oauth2model.SessionData
	Scope       string
	Resources   []string
}

// AuthorizationInitResult holds the result of a successful initial authorization request processing.
type AuthorizationInitResult struct {
	QueryParams map[string]string
	// RedirectURI, when set, is the client redirect with the code; the login page is skipped.
	RedirectURI string
}

// AuthorizationError holds structured error info for authorization failures.
type AuthorizationError struct {
	Code              string
	Message           string
	SendErrorToClient bool   // if true, redirect error to client's redirect_uri rather than the error page
	ClientRedirectURI string // populated when SendErrorToClient is true
	State             string // from the original request
}

// assertionClaims represents the claims extracted from the flow assertion JWT.
type assertionClaims struct {
	userID                 string
	authorizedPermissions  string
	attributeCacheID       string
	completedACR           string
	authorizationRequestID string
	tokenFamilyID          string
	sessionID              string
	correlationID          string
	subjectID              string
	subjectCategory        string
	flowErrorType          string
	jti                    string
	expiresAt              time.Time
	// grantedAuthorizationDetails is nil when the assertion carries no decision on them.
	grantedAuthorizationDetails []providers.AuthorizationDetail
}
