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
type CIBAAuthRequest struct {
	AuthReqID        string
	ExecutionID      string
	ClientID         string
	UserID           string
	Scopes           string
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
	// NotificationURL is an MVP-only testing affordance. It is non-standard and will be
	// removed once notification-channel delivery is implemented.
	NotificationURL string `json:"notification_url"`
}

// CallbackRequest represents the request body for the CIBA callback endpoint.
type CallbackRequest struct {
	AuthReqID string `json:"auth_req_id"`
	Assertion string `json:"assertion"`
}

// cibaError holds structured error information for CIBA backchannel and callback failures.
type cibaError struct {
	Code    string
	Message string
}
