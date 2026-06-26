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

package connection

import (
	"net/http"
	"strconv"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// oidcConnectionRequest is the create/update payload for a generic OpenID Connect connection.
type oidcConnectionRequest struct {
	Name                  string   `json:"name"`
	Description           string   `json:"description,omitempty"`
	ClientID              string   `json:"clientId"`
	ClientSecret          string   `json:"clientSecret"`
	RedirectURI           string   `json:"redirectUri"`
	AuthorizationEndpoint string   `json:"authorizationEndpoint"`
	TokenEndpoint         string   `json:"tokenEndpoint"`
	UserInfoEndpoint      string   `json:"userInfoEndpoint,omitempty"`
	JwksEndpoint          string   `json:"jwksEndpoint,omitempty"`
	LogoutEndpoint        string   `json:"logoutEndpoint,omitempty"`
	Issuer                string   `json:"issuer,omitempty"`
	Scopes                []string `json:"scopes,omitempty"`
	Prompt                string   `json:"prompt,omitempty"`
	TokenExchangeEnabled  *bool    `json:"tokenExchangeEnabled,omitempty"`
}

// oidcConnectionResponse is the detail payload for an OIDC connection (secret masked).
type oidcConnectionResponse struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Description           string   `json:"description,omitempty"`
	Type                  string   `json:"type"`
	ClientID              string   `json:"clientId,omitempty"`
	ClientSecret          string   `json:"clientSecret,omitempty"`
	RedirectURI           string   `json:"redirectUri,omitempty"`
	AuthorizationEndpoint string   `json:"authorizationEndpoint,omitempty"`
	TokenEndpoint         string   `json:"tokenEndpoint,omitempty"`
	UserInfoEndpoint      string   `json:"userInfoEndpoint,omitempty"`
	JwksEndpoint          string   `json:"jwksEndpoint,omitempty"`
	LogoutEndpoint        string   `json:"logoutEndpoint,omitempty"`
	Issuer                string   `json:"issuer,omitempty"`
	Scopes                []string `json:"scopes,omitempty"`
	Prompt                string   `json:"prompt,omitempty"`
	TokenExchangeEnabled  *bool    `json:"tokenExchangeEnabled,omitempty"`
}

func oidcToIDPDTO(req oidcConnectionRequest) (*providers.IDPDTO, error) {
	var props []cmodels.Property
	var err error
	fields := []struct {
		name     string
		value    string
		isSecret bool
	}{
		{idp.PropClientID, req.ClientID, false},
		{idp.PropClientSecret, req.ClientSecret, true},
		{idp.PropRedirectURI, req.RedirectURI, false},
		{idp.PropAuthorizationEndpoint, req.AuthorizationEndpoint, false},
		{idp.PropTokenEndpoint, req.TokenEndpoint, false},
		{idp.PropUserInfoEndpoint, req.UserInfoEndpoint, false},
		{idp.PropJwksEndpoint, req.JwksEndpoint, false},
		{idp.PropLogoutEndpoint, req.LogoutEndpoint, false},
		{idp.PropIssuer, req.Issuer, false},
		{idp.PropScopes, joinScopes(req.Scopes), false},
		{idp.PropPrompt, req.Prompt, false},
	}
	if req.TokenExchangeEnabled != nil {
		fields = append(fields, struct {
			name     string
			value    string
			isSecret bool
		}{idp.PropTokenExchangeEnabled, strconv.FormatBool(*req.TokenExchangeEnabled), false})
	}
	for _, field := range fields {
		if props, err = appendProperty(props, field.name, field.value, field.isSecret); err != nil {
			return nil, err
		}
	}
	return &providers.IDPDTO{
		Name:        req.Name,
		Description: req.Description,
		Type:        providers.IDPTypeOIDC,
		Properties:  props,
	}, nil
}

func oidcFromIDPDTO(dto providers.IDPDTO) (oidcConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return oidcConnectionResponse{}, err
	}
	resp := oidcConnectionResponse{
		ID:                    dto.ID,
		Name:                  dto.Name,
		Description:           dto.Description,
		Type:                  connectionTypeName(dto.Type),
		ClientID:              values[idp.PropClientID],
		ClientSecret:          values[idp.PropClientSecret],
		RedirectURI:           values[idp.PropRedirectURI],
		AuthorizationEndpoint: values[idp.PropAuthorizationEndpoint],
		TokenEndpoint:         values[idp.PropTokenEndpoint],
		UserInfoEndpoint:      values[idp.PropUserInfoEndpoint],
		JwksEndpoint:          values[idp.PropJwksEndpoint],
		LogoutEndpoint:        values[idp.PropLogoutEndpoint],
		Issuer:                values[idp.PropIssuer],
		Scopes:                splitScopes(values[idp.PropScopes]),
		Prompt:                values[idp.PropPrompt],
	}
	if raw, ok := values[idp.PropTokenExchangeEnabled]; ok {
		if enabled, parseErr := strconv.ParseBool(raw); parseErr == nil {
			resp.TokenExchangeEnabled = &enabled
		}
	}
	return resp, nil
}

func (h *handler) handleOIDCCreate(w http.ResponseWriter, r *http.Request) {
	createConnection(h, w, r, oidcToIDPDTO, oidcFromIDPDTO)
}

func (h *handler) handleOIDCGet(w http.ResponseWriter, r *http.Request) {
	getConnection(h, w, r, providers.IDPTypeOIDC, oidcFromIDPDTO)
}

func (h *handler) handleOIDCUpdate(w http.ResponseWriter, r *http.Request) {
	updateConnection(h, w, r, providers.IDPTypeOIDC, oidcToIDPDTO, oidcFromIDPDTO)
}
