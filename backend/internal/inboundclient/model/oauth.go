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

// Package model defines OAuth-related types for inbound client configuration.
//
//nolint:lll
package model

import (
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Supported JOSE algorithms for userinfo responses.
var (
	SupportedUserInfoSigningAlgs = []string{
		string(jws.RS256), string(jws.RS512), string(jws.PS256),
		string(jws.ES256), string(jws.ES384), string(jws.ES512),
		string(jws.EdDSA),
		string(jws.MLDSA44), string(jws.MLDSA65), string(jws.MLDSA87),
	}
	SupportedUserInfoEncryptionAlgs = []string{string(jwe.RSAOAEP), string(jwe.RSAOAEP256)}
	SupportedUserInfoEncryptionEncs = []string{string(jwe.A128CBCHS256), string(jwe.A256GCM)}
)

// OAuthConfig is the wire output shape (GET responses). ClientSecret is structurally absent.
// Empty slice/map fields are omitted; booleans are always serialized in both JSON and YAML for
// explicit semantics.
type OAuthConfig struct {
	ClientID                           string                            `json:"clientId,omitempty"                 yaml:"clientId,omitempty"`
	RedirectURIs                       []string                          `json:"redirectUris,omitempty"             yaml:"redirectUris,omitempty"`
	GrantTypes                         []providers.GrantType             `json:"grantTypes,omitempty"               yaml:"grantTypes,omitempty"`
	ResponseTypes                      []providers.ResponseType          `json:"responseTypes,omitempty"            yaml:"responseTypes,omitempty"`
	TokenEndpointAuthMethod            providers.TokenEndpointAuthMethod `json:"tokenEndpointAuthMethod,omitempty"  yaml:"tokenEndpointAuthMethod,omitempty"`
	PKCERequired                       bool                              `json:"pkceRequired"                       yaml:"pkceRequired"`
	PublicClient                       bool                              `json:"publicClient"                       yaml:"publicClient"`
	RequirePushedAuthorizationRequests bool                              `json:"requirePushedAuthorizationRequests" yaml:"requirePushedAuthorizationRequests"`
	DPoPBoundAccessTokens              bool                              `json:"dpopBoundAccessTokens"              yaml:"dpopBoundAccessTokens"`
	IncludeActClaim                    bool                              `json:"includeActClaim"                    yaml:"includeActClaim"`
	Token                              *providers.OAuthTokenConfig       `json:"token,omitempty"                    yaml:"token,omitempty"`
	Scopes                             []string                          `json:"scopes,omitempty"                   yaml:"scopes,omitempty"`
	UserInfo                           *providers.UserInfoConfig         `json:"userInfo,omitempty"                 yaml:"userInfo,omitempty"`
	ScopeClaims                        map[string][]string               `json:"scopeClaims,omitempty"              yaml:"scopeClaims,omitempty"`
	Certificate                        *providers.Certificate            `json:"certificate,omitempty"              yaml:"certificate,omitempty"`
	AcrValues                          []string                          `json:"acrValues,omitempty"                yaml:"acrValues,omitempty"`
}

// SupportedIDTokenEncryptionAlgs lists JWE key-management algorithms supported for ID token encryption.
var SupportedIDTokenEncryptionAlgs = []string{string(jwe.RSAOAEP), string(jwe.RSAOAEP256)}

// SupportedIDTokenEncryptionEncs lists JWE content-encryption algorithms supported for ID token encryption.
var SupportedIDTokenEncryptionEncs = []string{string(jwe.A128CBCHS256), string(jwe.A256GCM)}

// InboundAuthConfig is the wire output wrapper (GET responses).
type InboundAuthConfig struct {
	Type        providers.InboundAuthType `json:"type"             yaml:"type"`
	OAuthConfig *OAuthConfig              `json:"config,omitempty" yaml:"config,omitempty"`
}

// InboundAuthConfigProcessed is the runtime wrapper.
type InboundAuthConfigProcessed struct {
	Type        providers.InboundAuthType `json:"type"             yaml:"type,omitempty"`
	OAuthConfig *providers.OAuthClient    `json:"config,omitempty" yaml:"config,omitempty"`
}
