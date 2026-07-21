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

package connection //nolint:dupl // google mirrors github's identical shape, kept distinct per vendor

import (
	"strconv"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// googleConnectionRequest is the create/update payload for a Google connection. Google's
// OAuth/OIDC endpoints are known to the executor, so only the client credentials are needed.
type googleConnectionRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret"`
	RedirectURI  string   `json:"redirectUri"`
	Scopes       []string `json:"scopes,omitempty"`
	Prompt       string   `json:"prompt,omitempty"`

	JwksEndpoint         string `json:"jwksEndpoint,omitempty"`
	Issuer               string `json:"issuer,omitempty"`
	TokenExchangeEnabled *bool  `json:"tokenExchangeEnabled,omitempty"`

	AttributeConfiguration *providers.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

// googleConnectionResponse is the detail payload for a Google connection (secret masked).
type googleConnectionResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Type         string   `json:"type"`
	ClientID     string   `json:"clientId,omitempty"`
	ClientSecret string   `json:"clientSecret,omitempty"`
	RedirectURI  string   `json:"redirectUri,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	Prompt       string   `json:"prompt,omitempty"`

	JwksEndpoint         string `json:"jwksEndpoint,omitempty"`
	Issuer               string `json:"issuer,omitempty"`
	TokenExchangeEnabled *bool  `json:"tokenExchangeEnabled,omitempty"`

	AttributeConfiguration *providers.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

func googleToIDPDTO(req googleConnectionRequest) (*providers.IDPDTO, error) {
	var props []cmodels.Property
	var err error
	if props, err = appendProperty(props, idp.PropClientID, req.ClientID, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, idp.PropClientSecret, req.ClientSecret, true); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, idp.PropRedirectURI, req.RedirectURI, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, idp.PropScopes, joinScopes(req.Scopes), false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, idp.PropPrompt, req.Prompt, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, idp.PropJwksEndpoint, req.JwksEndpoint, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, idp.PropIssuer, req.Issuer, false); err != nil {
		return nil, err
	}
	if req.TokenExchangeEnabled != nil {
		if props, err = appendProperty(props, idp.PropTokenExchangeEnabled,
			strconv.FormatBool(*req.TokenExchangeEnabled), false); err != nil {
			return nil, err
		}
	}
	return &providers.IDPDTO{
		Name:                   req.Name,
		Description:            req.Description,
		Type:                   providers.IDPTypeGoogle,
		Properties:             props,
		AttributeConfiguration: req.AttributeConfiguration,
	}, nil
}

func googleFromIDPDTO(dto providers.IDPDTO) (googleConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return googleConnectionResponse{}, err
	}
	resp := googleConnectionResponse{
		ID:                     dto.ID,
		Name:                   dto.Name,
		Description:            dto.Description,
		Type:                   connectionTypeName(dto.Type),
		ClientID:               values[idp.PropClientID],
		ClientSecret:           values[idp.PropClientSecret],
		RedirectURI:            values[idp.PropRedirectURI],
		Scopes:                 splitScopes(values[idp.PropScopes]),
		Prompt:                 values[idp.PropPrompt],
		JwksEndpoint:           values[idp.PropJwksEndpoint],
		Issuer:                 values[idp.PropIssuer],
		AttributeConfiguration: dto.AttributeConfiguration,
	}
	if raw, ok := values[idp.PropTokenExchangeEnabled]; ok {
		if enabled, parseErr := strconv.ParseBool(raw); parseErr == nil {
			resp.TokenExchangeEnabled = &enabled
		}
	}
	return resp, nil
}
