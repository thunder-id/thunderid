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

package connection //nolint:dupl // github mirrors google's identical shape, kept distinct per vendor

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// githubConnectionRequest is the create/update payload for a GitHub connection. GitHub's
// OAuth endpoints are known to the executor, so only the client credentials are needed.
type githubConnectionRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret"`
	RedirectURI  string   `json:"redirectUri"`
	Scopes       []string `json:"scopes,omitempty"`
	Prompt       string   `json:"prompt,omitempty"`
}

// githubConnectionResponse is the detail payload for a GitHub connection (secret masked).
type githubConnectionResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Type         string   `json:"type"`
	ClientID     string   `json:"clientId,omitempty"`
	ClientSecret string   `json:"clientSecret,omitempty"`
	RedirectURI  string   `json:"redirectUri,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	Prompt       string   `json:"prompt,omitempty"`
}

func githubToIDPDTO(req githubConnectionRequest) (*providers.IDPDTO, error) {
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
	return &providers.IDPDTO{
		Name:        req.Name,
		Description: req.Description,
		Type:        providers.IDPTypeGitHub,
		Properties:  props,
	}, nil
}

func githubFromIDPDTO(dto providers.IDPDTO) (githubConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return githubConnectionResponse{}, err
	}
	return githubConnectionResponse{
		ID:           dto.ID,
		Name:         dto.Name,
		Description:  dto.Description,
		Type:         connectionTypeName(dto.Type),
		ClientID:     values[idp.PropClientID],
		ClientSecret: values[idp.PropClientSecret],
		RedirectURI:  values[idp.PropRedirectURI],
		Scopes:       splitScopes(values[idp.PropScopes]),
		Prompt:       values[idp.PropPrompt],
	}, nil
}

func (h *handler) handleGitHubCreate(w http.ResponseWriter, r *http.Request) {
	createConnection(h, w, r, githubToIDPDTO, githubFromIDPDTO)
}

func (h *handler) handleGitHubGet(w http.ResponseWriter, r *http.Request) {
	getConnection(h, w, r, providers.IDPTypeGitHub, githubFromIDPDTO)
}

func (h *handler) handleGitHubUpdate(w http.ResponseWriter, r *http.Request) {
	updateConnection(h, w, r, providers.IDPTypeGitHub, githubToIDPDTO, githubFromIDPDTO)
}
