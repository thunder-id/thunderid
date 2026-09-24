// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package ciba implements the OpenID Connect CIBA (Client-Initiated Backchannel Authentication) grant.
package ciba

import (
	"time"
)

// CIBARequestState represents the lifecycle state of a CIBA authentication request.
type CIBARequestState string

const (
	// CIBAStatePending indicates the user has not yet completed authentication.
	CIBAStatePending CIBARequestState = "PENDING"
	// CIBAStateAuthenticated indicates the user has authenticated and tokens may be issued.
	CIBAStateAuthenticated CIBARequestState = "AUTHENTICATED"
	// CIBAStateConsumed indicates the request has already been exchanged for tokens.
	CIBAStateConsumed CIBARequestState = "CONSUMED"
	// CIBAStateDenied indicates the user denied the authentication request.
	CIBAStateDenied CIBARequestState = "DENIED"
	// CIBAStateFailed indicates the authentication flow terminated due to a server-side error.
	CIBAStateFailed CIBARequestState = "FAILED"
	// CIBAStateExpired indicates the request expired before completion.
	CIBAStateExpired CIBARequestState = "EXPIRED"
)

// CIBAAuthRequest represents a persisted CIBA authentication request.
// UserID is empty at creation and populated by MarkAuthenticated once the user completes
// authentication and the callback verifies the assertion.
type CIBAAuthRequest struct {
	AuthReqID        string
	ClientID         string
	UserID           string
	StandardScopes   string
	AuthorizedScopes string
	Resources        []string
	State            CIBARequestState
	AttributeCacheID string
	CompletedACR     string
	AuthTime         time.Time
	SessionID        string
	LastPolledAt     time.Time
	ExpiryTime       time.Time
}

// BackchannelAuthResponse represents the response body for a successful backchannel authentication request.
type BackchannelAuthResponse struct {
	AuthReqID string `json:"auth_req_id"`
	ExpiresIn int64  `json:"expires_in"`
	Interval  int64  `json:"interval"`
}

// CIBAError holds structured error information for CIBA backchannel and callback failures.
type CIBAError struct {
	Code    string
	Message string
}

// BackchannelAuthRequest carries the parsed parameters of a backchannel authentication request.
type BackchannelAuthRequest struct {
	LoginHint       string
	IDTokenHint     string
	Scope           string
	Resources       []string
	BindingMessage  string
	RequestedExpiry string
	ACRValues       string
	Headers         map[string][]string
	QueryParams     map[string][]string
}

// assertionClaims represents the claims extracted from the flow assertion JWT.
type assertionClaims struct {
	userID                string
	attributeCacheID      string
	completedACR          string
	sessionID             string
	authReqID             string
	authorizedPermissions string
	flowErrorType         string
}
