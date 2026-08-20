// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package model defines the data structures used in the OAuth2 module.
package model

// TokenRequest represents the OAuth2 token request.
type TokenRequest struct {
	GrantType          string   `json:"grant_type"`
	ClientID           string   `json:"client_id"`
	ClientSecret       string   `json:"client_secret"`
	Scope              string   `json:"scope,omitempty"`
	Username           string   `json:"username,omitempty"`
	Password           string   `json:"password,omitempty"`
	RefreshToken       string   `json:"refresh_token,omitempty"`
	CodeVerifier       string   `json:"code_verifier,omitempty"`
	Code               string   `json:"code,omitempty"`
	RedirectURI        string   `json:"redirect_uri,omitempty"`
	Resources          []string `json:"resources,omitempty"`
	SubjectToken       string   `json:"subject_token,omitempty"`
	SubjectTokenType   string   `json:"subject_token_type,omitempty"`
	ActorToken         string   `json:"actor_token,omitempty"`
	ActorTokenType     string   `json:"actor_token_type,omitempty"`
	RequestedTokenType string   `json:"requested_token_type,omitempty"`
	Audiences          []string `json:"audiences,omitempty"`
	AuthReqID          string   `json:"auth_req_id,omitempty"`
	Assertion          string   `json:"assertion,omitempty"`
}

// TokenResponse represents the OAuth2 token response.
type TokenResponse struct {
	AccessToken     string `json:"access_token"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int64  `json:"expires_in"`
	RefreshToken    string `json:"refresh_token,omitempty"`
	Scope           string `json:"scope,omitempty"`
	IDToken         string `json:"id_token,omitempty"`
	IssuedTokenType string `json:"issued_token_type,omitempty"`
}

// TokenDTO represents the data transfer object for tokens.
type TokenDTO struct {
	Token             string
	TokenType         string
	IssuedAt          int64
	ExpiresIn         int64
	Scopes            []string
	ClientID          string
	UserAttributes    map[string]interface{}
	AttributeCacheID  string
	Subject           string
	Audiences         []string
	OriginalAudiences []string
	ClaimsRequest     *ClaimsRequest
	ClaimsLocales     string
	// TokenFamilyID is the token family id (tfid) stamped on the token, carried here so the refresh
	// token issued alongside an access token can be stamped with the same family id.
	TokenFamilyID string
	// ActorSub is the resource ID of the principal acting for Subject, mirroring the token's act.sub
	// claim. Set only for delegated (on-behalf-of) issuance; empty when the subject acts for itself.
	ActorSub string
	// SubjectID is the resource ID of the entity the token was issued for. It differs from Subject
	// whenever the application maps a subject attribute, where Subject is that attribute's value: the
	// resource ID is opaque and stable across applications, so it is what observability reports.
	// Empty when the subject could not be resolved to an entity, as on an exchange whose subject token
	// carries a mapped attribute.
	SubjectID string
	// SubjectCategory is the entity category of SubjectID (user, agent or app), resolved while the
	// token is built. Empty when it could not be determined, so consumers omit it rather than assume.
	SubjectCategory string
}

// TokenResponseDTO represents the data transfer object for token responses.
type TokenResponseDTO struct {
	AccessToken  TokenDTO
	RefreshToken TokenDTO
	IDToken      TokenDTO
	// CorrelationID is the correlation identifier of the authorization grant these tokens were issued
	// against, when the grant carries one (the login flow's execution id, arriving via the
	// authorization code). Reported on the token issuance event so it stitches to the flow's own
	// events; never returned to the client.
	CorrelationID string
}
