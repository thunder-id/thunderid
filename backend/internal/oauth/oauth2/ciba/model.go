/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package ciba implements the OpenID Connect CIBA (Client-Initiated Backchannel Authentication) grant.
package ciba

import "time"

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
	State            CIBARequestState
	AttributeCacheID string
	CompletedACR     string
	AuthTime         time.Time
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
	Scope           string
	BindingMessage  string
	RequestedExpiry string
	ACRValues       string
}

// assertionClaims represents the claims extracted from the flow assertion JWT.
type assertionClaims struct {
	userID                string
	attributeCacheID      string
	completedACR          string
	cibaAuthReqID         string
	authorizedPermissions string
}
