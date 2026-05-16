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

package discovery

// OAuth2AuthorizationServerMetadata represents OAuth2 Authorization Server Metadata (RFC 8414)
type OAuth2AuthorizationServerMetadata struct {
	Issuer                                     string   `json:"issuer"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	UserInfoEndpoint                           string   `json:"userinfo_endpoint,omitempty"`
	JWKSUri                                    string   `json:"jwks_uri"`
	RegistrationEndpoint                       string   `json:"registration_endpoint,omitempty"`
	RevocationEndpoint                         string   `json:"revocation_endpoint,omitempty"`
	IntrospectionEndpoint                      string   `json:"introspection_endpoint,omitempty"`
	PushedAuthorizationRequestEndpoint         string   `json:"pushed_authorization_request_endpoint,omitempty"`
	RequirePushedAuthorizationRequests         bool     `json:"require_pushed_authorization_requests,omitempty"`
	ScopesSupported                            []string `json:"scopes_supported"`
	ResponseTypesSupported                     []string `json:"response_types_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported"`
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported,omitempty"`
	AuthorizationResponseIssParameterSupported bool     `json:"authorization_response_iss_parameter_supported"`
	DPoPSigningAlgValuesSupported              []string `json:"dpop_signing_alg_values_supported,omitempty"`
}

// OIDCProviderMetadata represents OpenID Connect Provider Metadata (OIDC Discovery 1.0)
type OIDCProviderMetadata struct {
	OAuth2AuthorizationServerMetadata
	SubjectTypesSupported                []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported     []string `json:"id_token_signing_alg_values_supported"`
	UserInfoSigningAlgValuesSupported    []string `json:"userinfo_signing_alg_values_supported,omitempty"`
	UserInfoEncryptionAlgValuesSupported []string `json:"userinfo_encryption_alg_values_supported,omitempty"`
	UserInfoEncryptionEncValuesSupported []string `json:"userinfo_encryption_enc_values_supported,omitempty"`
	IDTokenEncryptionAlgValuesSupported  []string `json:"id_token_encryption_alg_values_supported,omitempty"`
	IDTokenEncryptionEncValuesSupported  []string `json:"id_token_encryption_enc_values_supported,omitempty"`
	ClaimsSupported                      []string `json:"claims_supported"`
	ClaimsParameterSupported             bool     `json:"claims_parameter_supported"`
	EndSessionEndpoint                   string   `json:"end_session_endpoint,omitempty"`
	AcrValuesSupported                   []string `json:"acr_values_supported,omitempty"`
}
