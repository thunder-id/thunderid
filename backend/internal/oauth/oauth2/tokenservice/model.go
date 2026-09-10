// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package tokenservice provides centralized token generation and validation services for OAuth2.
package tokenservice

import (
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// TokenType represents the type of token being processed.
type TokenType string

const (
	// TokenTypeAccess represents an access token.
	TokenTypeAccess TokenType = "access_token"
	// TokenTypeRefresh represents a refresh token.
	TokenTypeRefresh TokenType = "refresh_token"
	// TokenTypeID represents an ID token.
	TokenTypeID TokenType = "id_token"
)

// TokenConfig holds the configuration for token generation.
type TokenConfig struct {
	Issuer         string
	ValidityPeriod int64
	// SigningAlg is the client-configured JWS algorithm. Empty means the server's preferred
	// signing key algorithm is used.
	SigningAlg string
}

// AccessTokenBuildContext contains all the information needed to build an access token.
// The aud claim is serialized as a JSON array when Audiences has 2+ entries, and as a string
// when it has a single entry.
type AccessTokenBuildContext struct {
	Subject   string
	Audiences []string
	ClientID  string
	Scopes    []string
	// SubjectAttributes holds the token subject's attributes, already resolved and filtered by
	// the grant handler (user attributes for user-subject grants; OU claims plus own attributes
	// for client_credentials). The token builder embeds them as-is without any subject-specific
	// handling.
	SubjectAttributes map[string]interface{}
	AttributeCacheID  string
	GrantType         string
	OAuthApp          *providers.OAuthClient
	ActorClaims       *SubjectTokenClaims
	ClaimsRequest     *oauth2model.ClaimsRequest
	ClaimsLocales     string
	// ValidityPeriod is the subject's configured access-token validity in seconds (0 to use the
	// global default), resolved by the grant handler from the subject's access token sub-config.
	ValidityPeriod int64
	// DPoPJkt, when set, sender-constrains the access token to the supplied JWK thumbprint.
	// The token receives a `cnf.jkt` claim and is issued with `token_type=DPoP`.
	DPoPJkt string
	// SourceIDP, when set, records the issuer of the external identity provider that authenticated the
	// subject (used by the jwt-bearer/ID-JAG grant). It is emitted as the `idp` claim so downstream
	// consumers can distinguish a federated principal from a local one.
	SourceIDP string
	// TokenFamilyID, when set, is stamped as the `tfid` claim so the token can be revoked as part of
	// its authorization grant's family. It is constant across refresh rotation.
	TokenFamilyID string
	// SubjectEntityID is the resource ID of the entity Subject refers to, and SubjectCategory its
	// entity category. Grant handlers set them when an upstream layer already knows the answer (the
	// flow assertion for authorization_code), which spares the builder an entity read. Left empty the
	// builder resolves them itself, so a grant that does not set them still reports the subject.
	// Neither is emitted as a token claim; both are carried for observability.
	SubjectEntityID string
	SubjectCategory string
}

// RefreshTokenBuildContext contains all the information needed to build a refresh token.
type RefreshTokenBuildContext struct {
	ClientID             string
	Scopes               []string
	GrantType            string
	AccessTokenSubject   string
	AccessTokenAudiences []string
	AttributeCacheID     string
	OAuthApp             *providers.OAuthClient
	ClaimsRequest        *oauth2model.ClaimsRequest
	ClaimsLocales        string
	DPoPJkt              string
	ActorSub             string
	// TokenFamilyID, when set, is stamped as the `tfid` claim on the refresh token. It is copied
	// unchanged across rotation so every token of the grant shares one family id.
	TokenFamilyID string
	// ExpiresAt, when set, is the Unix expiry the rotated token inherits from the token it replaces,
	// so a grant cannot outlive its original issuance window. Zero starts a fresh validity period,
	// which is what first issuance does.
	ExpiresAt int64
}

// IDJAGBuildContext contains all the information needed to build an ID-JAG (Identity Assertion
// Authorization Grant, draft-ietf-oauth-identity-assertion-authz-grant).
type IDJAGBuildContext struct {
	Subject  string
	Audience string
	ClientID string
	Scopes   []string
	// Resources holds the RFC 8707 resource parameter values, when present on the request. Embedded
	// in the ID-JAG's `resource` claim so the resource AS can process them on the jwt-bearer leg.
	Resources []string
	OAuthApp  *providers.OAuthClient
}

// IDTokenBuildContext contains all the information needed to build an ID token (OIDC).
type IDTokenBuildContext struct {
	Subject        string
	Audience       string
	Scopes         []string
	UserAttributes map[string]interface{}
	AuthTime       int64
	OAuthApp       *providers.OAuthClient
	ClaimsRequest  *oauth2model.ClaimsRequest
	Nonce          string
	CompletedACR   string
}

// RefreshTokenClaims represents the validated claims from a refresh token.
type RefreshTokenClaims struct {
	Sub              string
	ClientID         string
	Audiences        []string
	GrantType        string
	Scopes           []string
	AttributeCacheID string
	Iat              int64
	ClaimsRequest    *oauth2model.ClaimsRequest
	ClaimsLocales    string
	DPoPJkt          string
	ActorSub         string
	// JTI is the refresh token's unique identifier, used for deny-list (revocation) enforcement.
	JTI string
	// Exp is the refresh token's expiry (exp claim); used to bound the deny-list entry when the token
	// is revoked on rotation.
	Exp int64
	// TokenFamilyID is the token family id (tfid) carried on the refresh token. It is copied onto the
	// tokens minted during rotation so the family stays intact, and used to revoke the whole family on
	// reuse. Empty for pre-rollout tokens.
	TokenFamilyID string
	Claims        map[string]interface{}
}

// SubjectTokenClaims represents the validated claims from a subject token (for token exchange).
type SubjectTokenClaims struct {
	Sub            string
	Iss            string
	Aud            []string
	Scopes         []string
	UserAttributes map[string]interface{}
	NestedAct      map[string]interface{}
	// CnfJkt is the JWK thumbprint extracted from the subject token's cnf.jkt claim.
	// Empty when the subject token is not DPoP-bound.
	CnfJkt string
	// JTI is the subject token's unique identifier, populated only for self-issued tokens and used
	// for deny-list (revocation) enforcement. Empty for externally-issued subject tokens.
	JTI string
	// TokenFamilyID is the subject token's token family id (tfid), if any. Token exchange may inherit
	// it onto the exchanged token so the two share a revocation family.
	TokenFamilyID string
}

// IDJAGAssertionClaims represents the validated claims from an ID-JAG assertion presented on the
// jwt-bearer grant (draft-ietf-oauth-identity-assertion-authz-grant).
type IDJAGAssertionClaims struct {
	Sub    string
	Iss    string
	Scopes []string
	// Resources holds the RFC 8707 `resource` claim values carried by the assertion, when present.
	// Empty when the assertion carries no resource claim.
	Resources []string
	// JTI is the assertion's unique identifier, required by the draft. Enforced as single-use via
	// the JTI replay cache; a replayed assertion is rejected.
	JTI string
}

// AccessTokenClaims represents the validated claims from an access token.
type AccessTokenClaims struct {
	Sub       string
	Iss       string
	Aud       []string
	GrantType string
	Scopes    []string
	ClientID  string
	Claims    map[string]interface{}
}
