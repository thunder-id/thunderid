// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package model defines OAuth-related types for inbound client configuration.
//
//nolint:lll
package model

import (
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// OAuthConfig is the wire output shape (GET responses). ClientSecret is structurally absent.
// Empty slice/map fields are omitted; booleans are always serialized in both JSON and YAML for
// explicit semantics.
type OAuthConfig struct {
	ClientID                           string                            `json:"clientId,omitempty"                 yaml:"clientId,omitempty"`
	RedirectURIs                       []string                          `json:"redirectUris,omitempty"             yaml:"redirectUris,omitempty"`
	PostLogoutRedirectURIs             []string                          `json:"postLogoutRedirectUris,omitempty"   yaml:"postLogoutRedirectUris,omitempty"`
	BackchannelLogoutURI               string                            `json:"backchannelLogoutUri,omitempty"     yaml:"backchannelLogoutUri,omitempty"`
	BackchannelLogoutSessionRequired   bool                              `json:"backchannelLogoutSessionRequired"   yaml:"backchannelLogoutSessionRequired"`
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
