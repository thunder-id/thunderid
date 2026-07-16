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
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// oauthConnectionRequest is the create/update payload for a generic OAuth 2.0 connection.
// Unlike OIDC, there is no id_token: the user profile is always fetched from
// userInfoEndpoint, so that field is required here.
type oauthConnectionRequest struct {
	Name                  string   `json:"name"`
	Description           string   `json:"description,omitempty"`
	ClientID              string   `json:"clientId"`
	ClientSecret          string   `json:"clientSecret"`
	RedirectURI           string   `json:"redirectUri"`
	AuthorizationEndpoint string   `json:"authorizationEndpoint"`
	TokenEndpoint         string   `json:"tokenEndpoint"`
	UserInfoEndpoint      string   `json:"userInfoEndpoint"`
	LogoutEndpoint        string   `json:"logoutEndpoint,omitempty"`
	Scopes                []string `json:"scopes,omitempty"`
	Prompt                string   `json:"prompt,omitempty"`

	AttributeConfiguration *providers.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

// oauthConnectionResponse is the detail payload for an OAuth 2.0 connection (secret masked).
type oauthConnectionResponse struct {
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
	LogoutEndpoint        string   `json:"logoutEndpoint,omitempty"`
	Scopes                []string `json:"scopes,omitempty"`
	Prompt                string   `json:"prompt,omitempty"`

	AttributeConfiguration *providers.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

func oauthToIDPDTO(req oauthConnectionRequest) (*providers.IDPDTO, error) {
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
		{idp.PropLogoutEndpoint, req.LogoutEndpoint, false},
		{idp.PropScopes, joinScopes(req.Scopes), false},
		{idp.PropPrompt, req.Prompt, false},
	}
	for _, field := range fields {
		if props, err = appendProperty(props, field.name, field.value, field.isSecret); err != nil {
			return nil, err
		}
	}
	return &providers.IDPDTO{
		Name:                   req.Name,
		Description:            req.Description,
		Type:                   providers.IDPTypeOAuth,
		Properties:             props,
		AttributeConfiguration: req.AttributeConfiguration,
	}, nil
}

func oauthFromIDPDTO(dto providers.IDPDTO) (oauthConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return oauthConnectionResponse{}, err
	}
	resp := oauthConnectionResponse{
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
		LogoutEndpoint:        values[idp.PropLogoutEndpoint],
		Scopes:                splitScopes(values[idp.PropScopes]),
		Prompt:                values[idp.PropPrompt],
	}
	resp.AttributeConfiguration = dto.AttributeConfiguration
	return resp, nil
}
