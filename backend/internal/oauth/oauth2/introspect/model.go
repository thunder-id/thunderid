// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package introspect

// IntrospectRequest represents the request to the token introspection endpoint
type IntrospectRequest struct {
	Token         string `json:"token" form:"token"`
	TokenTypeHint string `json:"token_type_hint,omitempty" form:"token_type_hint,omitempty"`
}

// IntrospectResponse represents the response from the token introspection endpoint
type IntrospectResponse struct {
	Active    bool      `json:"active"`
	Scope     string    `json:"scope,omitempty"`
	ClientID  string    `json:"client_id,omitempty"`
	Username  string    `json:"username,omitempty"`
	TokenType string    `json:"token_type,omitempty"`
	Exp       int64     `json:"exp,omitempty"`
	Iat       int64     `json:"iat,omitempty"`
	Nbf       int64     `json:"nbf,omitempty"`
	Sub       string    `json:"sub,omitempty"`
	SubType   string    `json:"sub_type,omitempty"`
	Aud       any       `json:"aud,omitempty"`
	Iss       string    `json:"iss,omitempty"`
	Jti       string    `json:"jti,omitempty"`
	Cnf       *CnfClaim `json:"cnf,omitempty"`
}

// CnfClaim represents the confirmation claim. For DPoP-bound tokens this carries
// the JWK SHA-256 thumbprint.
type CnfClaim struct {
	Jkt string `json:"jkt,omitempty"`
}
