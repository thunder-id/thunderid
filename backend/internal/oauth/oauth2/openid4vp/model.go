// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package openid4vp implements an OpenID4VP verifier and the authentication service for authentication via OpenID4VP.
package openid4vp

import (
	"crypto/ecdsa"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// VerifiedPresentation is the result of a successful presentation verification.
type VerifiedPresentation struct {
	Format               string
	Type                 []string
	Subject              string
	Claims               map[string]interface{}
	KeyBindingThumbprint string
}

// requestConfig is the static (config-driven) part of the OpenID4VP request.
type requestConfig struct {
	ClientID          string
	ResponseURI       string
	ResponseMode      string
	Audience          string
	Validity          time.Duration
	DCQL              providers.DCQLQuery
	ResponseEncValues []string
	// VerifierInfo is passed through opaquely; obtaining it is part of RP onboarding.
	VerifierInfo []interface{}
}

// requestParams is the per-request dynamic input to the request object builder.
type requestParams struct {
	Nonce        string
	State        string
	EphemeralKey *ecdsa.PublicKey
	IssuedAt     time.Time
}

// authorizationResponse is the decrypted direct_post.jwt body returned by the wallet.
type authorizationResponse struct {
	State         string
	Presentations map[string][]string
}

// Initiation is what the client needs to render the QR / deep link.
type Initiation struct {
	State      string
	ClientID   string
	RequestURI string
	// WalletURI is the pre-computed openid4vp:// deep link for the wallet.
	WalletURI string
	// ExpiresAt is when the request state expires.
	ExpiresAt time.Time
}

// Status is the lifecycle status of an OpenID4VP request.
type Status string

// Status values reported by the engine and consumed by callers (flow executor,
// RP-facing status endpoint).
const (
	StatusPending   Status = "PENDING"
	StatusStarted   Status = "STARTED"
	StatusVerified  Status = "VERIFIED"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
	StatusExpired   Status = "EXPIRED"
)

// RequestState is the short-lived per-request state correlated by State.
type RequestState struct {
	State         string
	VerifierID    string
	Nonce         string
	EphemeralKey  *ecdsa.PrivateKey
	ClientID      string
	RequestURI    string
	Status        Status
	Result        *VerifiedPresentation
	FailureReason string
	ExpiresAt     time.Time
	// ResultToken is a signed JWT result token, set by GetResult when Status==StatusCompleted.
	ResultToken string
	// ValidatedVP is the validated presentation, set by SubmitResponse after successful validation of the presentation.
	ValidatedVP string
}
