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

// Package model defines the data structures used in the OAuth2 module.
package model

// TokenRequest represents the OAuth2 token request.
type TokenRequest struct {
	GrantType          string   `json:"grant_type"`
	ClientID           string   `json:"client_id"`
	ClientSecret       string   `json:"client_secret"`
	Scope              string   `json:"scope,omitempty"`
	Username           string   `json:"username,omitempty"`
	Password           string   `json:"password,omitempty"`
	RefreshToken       string   `json:"refresh_token,omitempty"`
	CodeVerifier       string   `json:"code_verifier,omitempty"`
	Code               string   `json:"code,omitempty"`
	RedirectURI        string   `json:"redirect_uri,omitempty"`
	Resources          []string `json:"resources,omitempty"`
	SubjectToken       string   `json:"subject_token,omitempty"`
	SubjectTokenType   string   `json:"subject_token_type,omitempty"`
	ActorToken         string   `json:"actor_token,omitempty"`
	ActorTokenType     string   `json:"actor_token_type,omitempty"`
	RequestedTokenType string   `json:"requested_token_type,omitempty"`
	Audiences          []string `json:"audiences,omitempty"`
	AuthReqID          string   `json:"auth_req_id,omitempty"`
}

// TokenResponse represents the OAuth2 token response.
type TokenResponse struct {
	AccessToken     string `json:"access_token"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int64  `json:"expires_in"`
	RefreshToken    string `json:"refresh_token,omitempty"`
	Scope           string `json:"scope,omitempty"`
	IDToken         string `json:"id_token,omitempty"`
	IssuedTokenType string `json:"issued_token_type,omitempty"`
}

// TokenDTO represents the data transfer object for tokens.
type TokenDTO struct {
	Token             string
	TokenType         string
	IssuedAt          int64
	ExpiresIn         int64
	Scopes            []string
	ClientID          string
	UserAttributes    map[string]interface{}
	AttributeCacheID  string
	Subject           string
	Audiences         []string
	OriginalAudiences []string
	ClaimsRequest     *ClaimsRequest
	ClaimsLocales     string
}

// TokenResponseDTO represents the data transfer object for token responses.
type TokenResponseDTO struct {
	AccessToken  TokenDTO
	RefreshToken TokenDTO
	IDToken      TokenDTO
}
