// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"time"

	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
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
}

// AuthZPostResponse represents the response body for the authorization POST request.
type AuthZPostResponse struct {
	RedirectURI string `json:"redirect_uri"`
}

// AuthorizationInitResult holds the result of a successful initial authorization request processing.
type AuthorizationInitResult struct {
	QueryParams map[string]string
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
	flowErrorType          string
}
