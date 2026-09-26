// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package jwt

const (
	// TokenTypeJWT is the standard JWT type header value used for general-purpose JWTs.
	TokenTypeJWT = "JWT"

	// TokenTypeAccessToken is the JWT type header value for access tokens as defined in RFC 9068.
	TokenTypeAccessToken = "at+jwt"

	// TokenTypeAccessTokenWithPrefix is the media-type form of the access token typ header. RFC 9068
	// requires validators to accept both this and TokenTypeAccessToken.
	TokenTypeAccessTokenWithPrefix = "application/at+jwt"

	// TokenTypeRefreshToken is the JWT type header value for refresh tokens. Refresh tokens
	// historically shared the generic "JWT" typ with ID tokens and were told apart only by their
	// access_token_sub claim; an explicit type makes them self-identifying, so a validator that
	// whitelists the types it accepts rejects a refresh token by default rather than by remembering
	// to check a claim. Validators accept both during the migration window.
	TokenTypeRefreshToken = "rt+jwt" //nolint:gosec // JWT typ header value, not a credential

	// TokenTypeIDJAG is the JWT type header value for an Identity Assertion Authorization Grant
	// (draft-ietf-oauth-identity-assertion-authz-grant).
	TokenTypeIDJAG = "oauth-id-jag+jwt" //nolint:gosec // JWT typ header value, not a credential
)
