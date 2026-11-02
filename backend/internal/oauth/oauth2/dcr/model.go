// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dcr

import (
	"encoding/json"
	"strings"

	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Default values for DCR
const (
	ClientSecretExpiresAtNever   = 0 // Never expires
	maxLocalizedVariantsPerField = 20

	//nolint:gosec // WWW-Authenticate challenge value, not a credential
	wwwAuthenticateInvalidToken = `Bearer error="invalid_token"`
)

// DCRRegistrationRequest represents the RFC 7591 Dynamic Client Registration request. It carries
// client metadata only: the client identifier and secret are issued by the server, so they are not
// part of a registration. An update supplies them through DCRUpdateRequest.
type DCRRegistrationRequest struct {
	OUID                    string                            `json:"ou_id,omitempty"`
	RedirectURIs            []string                          `json:"redirect_uris"`
	PostLogoutRedirectURIs  []string                          `json:"post_logout_redirect_uris,omitempty"`
	BackchannelLogoutURI    string                            `json:"backchannel_logout_uri,omitempty"`
	GrantTypes              []providers.GrantType             `json:"grant_types,omitempty"`
	ResponseTypes           []providers.ResponseType          `json:"response_types,omitempty"`
	ClientName              string                            `json:"client_name,omitempty"`
	ClientURI               string                            `json:"client_uri,omitempty"`
	LogoURI                 string                            `json:"logo_uri,omitempty"`
	TokenEndpointAuthMethod providers.TokenEndpointAuthMethod `json:"token_endpoint_auth_method,omitempty"`
	JWKSUri                 string                            `json:"jwks_uri,omitempty"`
	JWKS                    map[string]interface{}            `json:"jwks,omitempty"`
	Scope                   string                            `json:"scope,omitempty"`
	Contacts                []string                          `json:"contacts,omitempty"`
	TosURI                  string                            `json:"tos_uri,omitempty"`
	PolicyURI               string                            `json:"policy_uri,omitempty"`

	RequirePushedAuthorizationRequests bool   `json:"require_pushed_authorization_requests,omitempty"`
	DPoPBoundAccessTokens              bool   `json:"dpop_bound_access_tokens,omitempty"`
	UserInfoSignedResponseAlg          string `json:"userinfo_signed_response_alg,omitempty"`
	UserInfoEncryptedResponseAlg       string `json:"userinfo_encrypted_response_alg,omitempty"`
	UserInfoEncryptedResponseEnc       string `json:"userinfo_encrypted_response_enc,omitempty"`
	IDTokenSignedResponseAlg           string `json:"id_token_signed_response_alg,omitempty"`
	IDTokenEncryptedResponseAlg        string `json:"id_token_encrypted_response_alg,omitempty"`
	IDTokenEncryptedResponseEnc        string `json:"id_token_encrypted_response_enc,omitempty"`
	// Localized variant maps — populated from #-keyed JSON fields (e.g. "client_name#fr").
	LocalizedClientName map[string]string `json:"-"`
	LocalizedLogoURI    map[string]string `json:"-"`
	LocalizedTosURI     map[string]string `json:"-"`
	LocalizedPolicyURI  map[string]string `json:"-"`
}

// UnmarshalJSON decodes DCRRegistrationRequest from JSON, extracting OIDC language-tagged fields
// (e.g. "client_name#fr") into the localized variant maps.
func (r *DCRRegistrationRequest) UnmarshalJSON(data []byte) error {
	type Alias DCRRegistrationRequest
	if err := json.Unmarshal(data, (*Alias)(r)); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	return parseLocalizedFields(raw, r)
}

// parseLocalizedFields extracts language-tagged fields (e.g. "client_name#fr") from a raw JSON map
// and populates the localized variant maps on r.
func parseLocalizedFields(raw map[string]json.RawMessage, r *DCRRegistrationRequest) error {
	for key, val := range raw {
		field, tag, ok := strings.Cut(key, "#")
		if !ok {
			continue
		}
		canonical, valid := i18nmgt.NormaliseBCP47Tag(tag)
		if !valid {
			return &errInvalidBCP47Tag{key: key}
		}
		var s string
		if err := json.Unmarshal(val, &s); err != nil {
			continue
		}
		var target *map[string]string
		switch field {
		case "client_name":
			target = &r.LocalizedClientName
		case "logo_uri":
			target = &r.LocalizedLogoURI
		case "tos_uri":
			target = &r.LocalizedTosURI
		case "policy_uri":
			target = &r.LocalizedPolicyURI
		}
		if target == nil {
			continue
		}
		if err := setLocalizedVariant(target, field, canonical, s); err != nil {
			return err
		}
	}
	return nil
}

// DCRUpdateRequest represents the request body of an RFC 7592 client configuration update. It is the
// registration metadata plus the two fields that only an update carries: RFC 7592 section 2.2
// requires the request to identify the client it updates, and lets it present the currently issued
// secret. Registration has neither, because the server issues both.
type DCRUpdateRequest struct {
	DCRRegistrationRequest
	ClientID string `json:"client_id,omitempty"`
	// ClientSecret is optional. When present it must match the secret currently issued to the
	// client. It is verified and never written: a client may not choose its own secret.
	ClientSecret string `json:"client_secret,omitempty"`
}

// UnmarshalJSON decodes DCRUpdateRequest from JSON. Aliasing the outer type alone is not enough:
// the embedded registration request keeps its own UnmarshalJSON, which the alias promotes, so the
// decoder would hand the whole object to the embedded value and never populate the update fields.
// Embedding an alias of the registration request instead strips that method, letting one pass fill
// both levels, after which the language tagged fields are extracted for the embedded value.
func (r *DCRUpdateRequest) UnmarshalJSON(data []byte) error {
	type registrationAlias DCRRegistrationRequest
	type updateAlias struct {
		registrationAlias
		ClientID     string `json:"client_id,omitempty"`
		ClientSecret string `json:"client_secret,omitempty"`
	}
	var decoded updateAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	r.DCRRegistrationRequest = DCRRegistrationRequest(decoded.registrationAlias)
	r.ClientID = decoded.ClientID
	r.ClientSecret = decoded.ClientSecret

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	return parseLocalizedFields(raw, &r.DCRRegistrationRequest)
}

// setLocalizedVariant initializes the map if needed, stores the value, and enforces the variant limit.
func setLocalizedVariant(m *map[string]string, field, tag, val string) error {
	if *m == nil {
		*m = make(map[string]string)
	}
	(*m)[tag] = val
	if len(*m) > maxLocalizedVariantsPerField {
		return &errTooManyLocalizedVariants{field: field}
	}
	return nil
}

// DCRRegistrationResponse represents the RFC 7591 Dynamic Client Registration response. The same
// shape is returned by the RFC 7592 client configuration endpoint.
type DCRRegistrationResponse struct {
	ClientID                string                            `json:"client_id"`
	ClientSecret            string                            `json:"client_secret,omitempty"`
	ClientSecretExpiresAt   int64                             `json:"client_secret_expires_at"`
	RedirectURIs            []string                          `json:"redirect_uris,omitempty"`
	PostLogoutRedirectURIs  []string                          `json:"post_logout_redirect_uris,omitempty"`
	BackchannelLogoutURI    string                            `json:"backchannel_logout_uri,omitempty"`
	GrantTypes              []providers.GrantType             `json:"grant_types,omitempty"`
	ResponseTypes           []providers.ResponseType          `json:"response_types,omitempty"`
	ClientName              string                            `json:"client_name,omitempty"`
	ClientURI               string                            `json:"client_uri,omitempty"`
	LogoURI                 string                            `json:"logo_uri,omitempty"`
	TokenEndpointAuthMethod providers.TokenEndpointAuthMethod `json:"token_endpoint_auth_method,omitempty"`
	JWKSUri                 string                            `json:"jwks_uri,omitempty"`
	JWKS                    map[string]interface{}            `json:"jwks,omitempty"`
	Scope                   string                            `json:"scope,omitempty"`
	Contacts                []string                          `json:"contacts,omitempty"`
	TosURI                  string                            `json:"tos_uri,omitempty"`
	PolicyURI               string                            `json:"policy_uri,omitempty"`
	AppID                   string                            `json:"app_id,omitempty"`

	RequirePushedAuthorizationRequests bool   `json:"require_pushed_authorization_requests,omitempty"`
	DPoPBoundAccessTokens              bool   `json:"dpop_bound_access_tokens,omitempty"`
	UserInfoSignedResponseAlg          string `json:"userinfo_signed_response_alg,omitempty"`
	UserInfoEncryptedResponseAlg       string `json:"userinfo_encrypted_response_alg,omitempty"`
	UserInfoEncryptedResponseEnc       string `json:"userinfo_encrypted_response_enc,omitempty"`
	IDTokenSignedResponseAlg           string `json:"id_token_signed_response_alg,omitempty"`
	IDTokenEncryptedResponseAlg        string `json:"id_token_encrypted_response_alg,omitempty"`
	IDTokenEncryptedResponseEnc        string `json:"id_token_encrypted_response_enc,omitempty"`
	// Localized variant maps — injected as #-keyed top-level fields during serialization.
	LocalizedClientName map[string]string `json:"-"`
	LocalizedLogoURI    map[string]string `json:"-"`
	LocalizedTosURI     map[string]string `json:"-"`
	LocalizedPolicyURI  map[string]string `json:"-"`
}

// MarshalJSON serializes DCRRegistrationResponse to JSON, injecting OIDC language-tagged
// fields (e.g. "client_name#fr") as top-level keys.
func (r DCRRegistrationResponse) MarshalJSON() ([]byte, error) {
	type Alias DCRRegistrationResponse
	base, err := json.Marshal(Alias(r))
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, err
	}
	appendLocalizedFields(m, r)
	return json.Marshal(m)
}

// appendLocalizedFields injects localized variant maps from r into m as #-keyed top-level entries.
func appendLocalizedFields(m map[string]interface{}, r DCRRegistrationResponse) {
	for tag, val := range r.LocalizedClientName {
		m["client_name#"+tag] = val
	}
	for tag, val := range r.LocalizedLogoURI {
		m["logo_uri#"+tag] = val
	}
	for tag, val := range r.LocalizedTosURI {
		m["tos_uri#"+tag] = val
	}
	for tag, val := range r.LocalizedPolicyURI {
		m["policy_uri#"+tag] = val
	}
}

// DCRErrorResponse represents the RFC 7591 Dynamic Client Registration error response.
type DCRErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}
