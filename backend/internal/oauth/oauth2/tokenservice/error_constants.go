// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package tokenservice

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
)

// Reasons a token failed validation, discriminated by callers with errors.Is to pick a specific error_description.
var (
	// ErrTokenExpired indicates the token's exp claim is in the past.
	ErrTokenExpired = errors.New("token has expired")

	// ErrIssuerNotTrusted indicates the token's iss claim resolves to neither this server nor an
	// identity provider registered as a trusted issuer for the grant being used.
	ErrIssuerNotTrusted = errors.New("token issuer is not trusted")

	// ErrAudienceNotAccepted indicates the token's aud claim names none of the audiences the grant
	// accepts, such as this server's issuer or an identity provider's trusted token audience.
	ErrAudienceNotAccepted = errors.New("token audience is not accepted")

	// ErrAssertionReplayed indicates the assertion's jti has already been recorded in the replay cache.
	// Aliased to the shared sentinel so that every redemption path — this one and the authorization
	// callback — reports replay as the same error value.
	ErrAssertionReplayed = utils.ErrAssertionReplayed
)
