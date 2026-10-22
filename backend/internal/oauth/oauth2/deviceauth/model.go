// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package deviceauth

import "time"

// DeviceRequestState represents the lifecycle state of a device authorization request.
type DeviceRequestState string

const (
	// DeviceStatePending indicates that the user has not completed authorization.
	DeviceStatePending DeviceRequestState = "PENDING"
	// DeviceStateRedeeming indicates that one browser has claimed the user code and is completing the flow.
	DeviceStateRedeeming DeviceRequestState = "REDEEMING"
	// DeviceStateApproved indicates that the user approved the request and tokens may be issued.
	DeviceStateApproved DeviceRequestState = "APPROVED"
	// DeviceStateConsumed indicates that tokens have already been issued for the request.
	DeviceStateConsumed DeviceRequestState = "CONSUMED"
	// DeviceStateDenied indicates that the user denied the request.
	DeviceStateDenied DeviceRequestState = "DENIED"
	// DeviceStateFailed indicates that the authorization flow failed.
	DeviceStateFailed DeviceRequestState = "FAILED"
	// DeviceStateExpired indicates that the request expired before authorization completed.
	DeviceStateExpired DeviceRequestState = "EXPIRED"
)

// DeviceAuthRequest represents a persisted device authorization request.
type DeviceAuthRequest struct {
	RequestID                  string
	ApplicationID              string
	ApplicationAuthFlowID      string
	DeviceCodeHash             string
	UserCode                   string
	ClientID                   string
	UserID                     string
	StandardScopes             string
	RequestedPermissions       string
	RequiredOptionalAttributes string
	AuthorizedScopes           string
	Resources                  []string
	State                      DeviceRequestState
	AttributeCacheID           string
	CompletedACR               string
	AuthTime                   time.Time
	SessionID                  string
	LastPolledAt               time.Time
	PollInterval               int64
	ExpiryTime                 time.Time
}

// DeviceAuthorizationRequest contains parameters from a device authorization request.
type DeviceAuthorizationRequest struct {
	Scope     string
	Resources []string
}

// DeviceAuthorizationResponse represents a successful device authorization response.
type DeviceAuthorizationResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}

// DeviceVerificationResponse identifies the claimed request and its application.
type DeviceVerificationResponse struct {
	ApplicationID              string `json:"applicationId"`
	ApplicationAuthFlowID      string `json:"applicationAuthenticationFlowId"`
	AuthID                     string `json:"authId"`
	RequestedPermissions       string `json:"requestedPermissions,omitempty"`
	ResourceServerIdentifier   string `json:"resourceServerIdentifier,omitempty"`
	RequiredOptionalAttributes string `json:"requiredOptionalAttributes,omitempty"`
}

// assertionClaims represents the claims extracted from a Device Grant flow assertion.
type assertionClaims struct {
	userID                string
	attributeCacheID      string
	completedACR          string
	sessionID             string
	requestID             string
	consentID             string
	consentApproved       bool
	authorizedPermissions string
	flowErrorType         string
}
