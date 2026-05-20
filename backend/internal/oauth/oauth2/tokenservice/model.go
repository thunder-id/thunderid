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

// Package tokenservice provides centralized token generation and validation services for OAuth2.
package tokenservice

import (
	"context"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
)

// TokenType represents the type of token being processed.
type TokenType string

const (
	// TokenTypeAccess represents an access token.
	TokenTypeAccess TokenType = "access_token"
	// TokenTypeRefresh represents a refresh token.
	TokenTypeRefresh TokenType = "refresh_token"
	// TokenTypeID represents an ID token.
	TokenTypeID TokenType = "id_token"
)

// TokenConfig holds the configuration for token generation.
type TokenConfig struct {
	Issuer         string
	ValidityPeriod int64
}

// AccessTokenBuildContext contains all the information needed to build an access token.
// The aud claim is serialized as a JSON array when Audiences has 2+ entries, and as a string
// when it has a single entry.
type AccessTokenBuildContext struct {
	Context          context.Context
	Subject          string
	Audiences        []string
	ClientID         string
	Scopes           []string
	UserAttributes   map[string]interface{}
	AttributeCacheID string
	GrantType        string
	OAuthApp         *inboundmodel.OAuthClient
	ActorClaims      *SubjectTokenClaims
	ClaimsRequest    *oauth2model.ClaimsRequest
	ClaimsLocales    string
	ClientAttributes map[string]interface{}
}

// RefreshTokenBuildContext contains all the information needed to build a refresh token.
type RefreshTokenBuildContext struct {
	Context              context.Context
	ClientID             string
	Scopes               []string
	GrantType            string
	AccessTokenSubject   string
	AccessTokenAudiences []string
	AttributeCacheID     string
	OAuthApp             *inboundmodel.OAuthClient
	ClaimsRequest        *oauth2model.ClaimsRequest
	ClaimsLocales        string
}

// IDTokenBuildContext contains all the information needed to build an ID token (OIDC).
type IDTokenBuildContext struct {
	Context        context.Context
	Subject        string
	Audience       string
	Scopes         []string
	UserAttributes map[string]interface{}
	AuthTime       int64
	OAuthApp       *inboundmodel.OAuthClient
	ClaimsRequest  *oauth2model.ClaimsRequest
	Nonce          string
	CompletedACR   string
}

// RefreshTokenClaims represents the validated claims from a refresh token.
type RefreshTokenClaims struct {
	Sub              string
	Audiences        []string
	GrantType        string
	Scopes           []string
	AttributeCacheID string
	Iat              int64
	ClaimsRequest    *oauth2model.ClaimsRequest
	ClaimsLocales    string
}

// SubjectTokenClaims represents the validated claims from a subject token (for token exchange).
type SubjectTokenClaims struct {
	Sub            string
	Iss            string
	Aud            []string
	Scopes         []string
	UserAttributes map[string]interface{}
	NestedAct      map[string]interface{}
}

// AccessTokenClaims represents the validated claims from an access token.
type AccessTokenClaims struct {
	Sub       string
	Iss       string
	Aud       []string
	GrantType string
	Scopes    []string
	ClientID  string
	Claims    map[string]interface{}
}
