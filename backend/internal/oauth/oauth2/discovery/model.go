// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

// OAuth2AuthorizationServerMetadata represents OAuth2 Authorization Server Metadata (RFC 8414)
type OAuth2AuthorizationServerMetadata struct {
	Issuer                                     string   `json:"issuer"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	JWKSUri                                    string   `json:"jwks_uri"`
	RegistrationEndpoint                       string   `json:"registration_endpoint,omitempty"`
	RevocationEndpoint                         string   `json:"revocation_endpoint,omitempty"`
	IntrospectionEndpoint                      string   `json:"introspection_endpoint,omitempty"`
	PushedAuthorizationRequestEndpoint         string   `json:"pushed_authorization_request_endpoint,omitempty"`
	RequirePushedAuthorizationRequests         bool     `json:"require_pushed_authorization_requests,omitempty"`
	BackchannelAuthenticationEndpoint          string   `json:"backchannel_authentication_endpoint,omitempty"`
	BackchannelTokenDeliveryModesSupported     []string `json:"backchannel_token_delivery_modes_supported,omitempty"`
	BackchannelUserCodeParameterSupported      bool     `json:"backchannel_user_code_parameter_supported"`
	ResponseTypesSupported                     []string `json:"response_types_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported"`
	TokenEndpointAuthSigningAlgValuesSupported []string `json:"token_endpoint_auth_signing_alg_values_supported,omitempty"` //nolint:lll
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported,omitempty"`
	AuthorizationResponseIssParameterSupported bool     `json:"authorization_response_iss_parameter_supported"`
	DPoPSigningAlgValuesSupported              []string `json:"dpop_signing_alg_values_supported,omitempty"`
	AuthorizationGrantProfilesSupported        []string `json:"authorization_grant_profiles_supported,omitempty"`
}

// OIDCProviderMetadata represents OpenID Connect Provider Metadata (OIDC Discovery 1.0)
type OIDCProviderMetadata struct {
	OAuth2AuthorizationServerMetadata
	UserInfoEndpoint                     string   `json:"userinfo_endpoint"`
	ScopesSupported                      []string `json:"scopes_supported"`
	SubjectTypesSupported                []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported     []string `json:"id_token_signing_alg_values_supported"`
	UserInfoSigningAlgValuesSupported    []string `json:"userinfo_signing_alg_values_supported,omitempty"`
	UserInfoEncryptionAlgValuesSupported []string `json:"userinfo_encryption_alg_values_supported,omitempty"`
	UserInfoEncryptionEncValuesSupported []string `json:"userinfo_encryption_enc_values_supported,omitempty"`
	IDTokenEncryptionAlgValuesSupported  []string `json:"id_token_encryption_alg_values_supported,omitempty"`
	IDTokenEncryptionEncValuesSupported  []string `json:"id_token_encryption_enc_values_supported,omitempty"`
	ClaimsSupported                      []string `json:"claims_supported"`
	ClaimsParameterSupported             bool     `json:"claims_parameter_supported"`
	RequestParameterSupported            bool     `json:"request_parameter_supported"`
	RequestURIParameterSupported         bool     `json:"request_uri_parameter_supported"`
	EndSessionEndpoint                   string   `json:"end_session_endpoint,omitempty"`
	AcrValuesSupported                   []string `json:"acr_values_supported,omitempty"`
}
