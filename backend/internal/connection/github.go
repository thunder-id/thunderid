// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection //nolint:dupl // github mirrors google's identical shape, kept distinct per vendor

import (
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// githubConnectionRequest is the create/update payload for a GitHub connection. GitHub's
// OAuth endpoints are known to the executor, so only the client credentials are needed.
type githubConnectionRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	OUID         string   `json:"ouId,omitempty"`
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret"`
	RedirectURI  string   `json:"redirectUri"`
	Scopes       []string `json:"scopes,omitempty"`
	Prompt       string   `json:"prompt,omitempty"`

	AttributeConfiguration *providers.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

// githubConnectionResponse is the detail payload for a GitHub connection (secret masked).
type githubConnectionResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	OUID         string   `json:"ouId,omitempty"`
	Type         string   `json:"type"`
	ClientID     string   `json:"clientId,omitempty"`
	ClientSecret string   `json:"clientSecret,omitempty"`
	RedirectURI  string   `json:"redirectUri,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	Prompt       string   `json:"prompt,omitempty"`

	AttributeConfiguration *providers.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
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
		Name:                   req.Name,
		Description:            req.Description,
		OUID:                   req.OUID,
		Type:                   providers.IDPTypeGitHub,
		Properties:             props,
		AttributeConfiguration: req.AttributeConfiguration,
	}, nil
}

func githubFromIDPDTO(dto providers.IDPDTO) (githubConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return githubConnectionResponse{}, err
	}
	return githubConnectionResponse{
		ID:                     dto.ID,
		Name:                   dto.Name,
		Description:            dto.Description,
		OUID:                   dto.OUID,
		Type:                   connectionTypeName(dto.Type),
		ClientID:               values[idp.PropClientID],
		ClientSecret:           values[idp.PropClientSecret],
		RedirectURI:            values[idp.PropRedirectURI],
		Scopes:                 splitScopes(values[idp.PropScopes]),
		Prompt:                 values[idp.PropPrompt],
		AttributeConfiguration: dto.AttributeConfiguration,
	}, nil
}
