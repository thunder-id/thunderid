// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"time"
)

// DefaultHTTPTimeout is the default timeout duration for HTTP federated IDP requests.
const DefaultHTTPTimeout = 5 * time.Second

// IDPRequestTimeout bounds a request to a federated identity provider's token or user info endpoint.
const IDPRequestTimeout = 30 * time.Second

// Authenticator name constants.
const (
	AuthenticatorCredentials = "CredentialsAuthenticator"
	AuthenticatorSMSOTP      = "SMSOTPAuthenticator"
	AuthenticatorOTP         = "OTPAuthenticator"
	AuthenticatorMagicLink   = "MagicLinkAuthenticator"
	AuthenticatorGoogle      = "GoogleOIDCAuthenticator"
	AuthenticatorGithub      = "GithubOAuthAuthenticator"
	AuthenticatorOAuth       = "OAuthAuthenticator"
	AuthenticatorOIDC        = "OIDCAuthenticator"
	AuthenticatorPasskey     = "Passkey"
	AuthenticatorOpenID4VP   = "OpenID4VPAuthenticator"
)

// AuthenticationFactor represents the type of authentication factor.
type AuthenticationFactor string

const (
	// FactorKnowledge represents "something you know" (e.g., password, PIN).
	FactorKnowledge AuthenticationFactor = "KNOWLEDGE"
	// FactorPossession represents "something you have" (e.g., OTP device, SMS).
	FactorPossession AuthenticationFactor = "POSSESSION"
	// FactorInherence represents "something you are" (e.g., biometrics).
	FactorInherence AuthenticationFactor = "INHERENCE"
)

// UserAttributeUserID is the user attribute key for user ID in authentication results.
const UserAttributeUserID = "userID"
