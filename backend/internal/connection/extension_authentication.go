// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"strconv"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// extensionAuthenticationConnectionRequest is the create/update payload for an authentication endpoint
// connection: where the organization's service answers an authentication, and how an identity it verifies
// maps to and links with a local user.
type extensionAuthenticationConnectionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	OUID        string `json:"ouId,omitempty"`
	URL         string `json:"url"`
	TimeoutMS   int    `json:"timeoutMs,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`

	AttributeConfiguration *providers.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

// extensionAuthenticationConnectionResponse is the detail payload for an authentication endpoint
// connection (password masked).
type extensionAuthenticationConnectionResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	OUID        string `json:"ouId,omitempty"`
	Type        string `json:"type"`
	URL         string `json:"url,omitempty"`
	TimeoutMS   int    `json:"timeoutMs,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`

	AttributeConfiguration *providers.AttributeConfiguration `json:"attributeConfiguration,omitempty"`

	connectionOwnership
}

func extensionAuthenticationToIDPDTO(req extensionAuthenticationConnectionRequest) (*providers.IDPDTO, error) {
	var props []cmodels.Property
	var err error
	if props, err = appendProperty(props, idp.PropEndpointURL, req.URL, false); err != nil {
		return nil, err
	}
	if req.TimeoutMS != 0 {
		if props, err = appendProperty(props, idp.PropTimeoutMS, strconv.Itoa(req.TimeoutMS), false); err != nil {
			return nil, err
		}
	}
	if props, err = appendProperty(props, idp.PropAuthUsername, req.Username, false); err != nil {
		return nil, err
	}
	if props, err = appendProperty(props, idp.PropAuthPassword, req.Password, true); err != nil {
		return nil, err
	}
	return &providers.IDPDTO{
		Name:                   req.Name,
		Description:            req.Description,
		OUID:                   req.OUID,
		Type:                   providers.IDPTypeExtensionAuthentication,
		Properties:             props,
		AttributeConfiguration: req.AttributeConfiguration,
	}, nil
}

func extensionAuthenticationFromIDPDTO(dto providers.IDPDTO) (extensionAuthenticationConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return extensionAuthenticationConnectionResponse{}, err
	}
	timeoutMS, _ := strconv.Atoi(values[idp.PropTimeoutMS])
	return extensionAuthenticationConnectionResponse{
		ID:                     dto.ID,
		Name:                   dto.Name,
		Description:            dto.Description,
		OUID:                   dto.OUID,
		Type:                   authenticationEndpointVendorName,
		URL:                    values[idp.PropEndpointURL],
		TimeoutMS:              timeoutMS,
		Username:               values[idp.PropAuthUsername],
		Password:               values[idp.PropAuthPassword],
		AttributeConfiguration: dto.AttributeConfiguration,
	}, nil
}
