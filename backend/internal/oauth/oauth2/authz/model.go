/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package authz

import (
	"time"

	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
)

// OAuthMessage represents the OAuth message.
type OAuthMessage struct {
	RequestType        string
	AuthID             string
	RequestQueryParams map[string]string
	Resources          []string
	RequestBodyParams  map[string]string
}

// AuthorizationCode represents the authorization code.
type AuthorizationCode struct {
	CodeID              string
	Code                string
	ClientID            string
	RedirectURI         string
	AuthorizedUserID    string
	AttributeCacheID    string
	TimeCreated         time.Time
	ExpiryTime          time.Time
	Scopes              string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resources           []string
	ClaimsRequest       *oauth2model.ClaimsRequest
	ClaimsLocales       string
	Nonce               string
	CompletedACR        string
}

// AuthZPostRequest represents the request body for the authorization POST request.
type AuthZPostRequest struct {
	AuthID    string `json:"authId"`
	Assertion string `json:"assertion"`
}

// AuthZPostResponse represents the response body for the authorization POST request.
type AuthZPostResponse struct {
	RedirectURI string `json:"redirect_uri"`
}

// AuthorizationInitResult holds the result of a successful initial authorization request processing.
type AuthorizationInitResult struct {
	QueryParams map[string]string
}

// AuthorizationError holds structured error info for authorization failures.
type AuthorizationError struct {
	Code              string
	Message           string
	SendErrorToClient bool   // if true, redirect error to client's redirect_uri rather than the error page
	ClientRedirectURI string // populated when SendErrorToClient is true
	State             string // from the original request
}

// assertionClaims represents the claims extracted from the flow assertion JWT.
type assertionClaims struct {
	userID                string
	authorizedPermissions string
	attributeCacheID      string
	completedACR          string
}
