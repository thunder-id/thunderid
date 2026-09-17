// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"strings"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// esignetConnectionRequest is the create/update payload for a MOSIP eSignet connection.
// eSignet authenticates its clients with private_key_jwt rather than a client secret, so the
// payload carries signingKey, the PEM-encoded RSA private key that signs the client assertion,
// and signingKeyId, the identifier of the matching JWK registered with eSignet. signingKey is
// stored encrypted and is write-only, exactly as a client secret would be.
type esignetConnectionRequest struct {
	Name                  string   `json:"name"`
	Description           string   `json:"description,omitempty"`
	ClientID              string   `json:"clientId"`
	RedirectURI           string   `json:"redirectUri"`
	AuthorizationEndpoint string   `json:"authorizationEndpoint"`
	TokenEndpoint         string   `json:"tokenEndpoint"`
	UserInfoEndpoint      string   `json:"userInfoEndpoint"`
	JwksEndpoint          string   `json:"jwksEndpoint"`
	SigningKey            string   `json:"signingKey"`
	SigningKeyID          string   `json:"signingKeyId"`
	Scopes                []string `json:"scopes,omitempty"`
	ACRValues             string   `json:"acrValues,omitempty"`
	UsernamePrefix        string   `json:"usernamePrefix,omitempty"`

	AttributeConfiguration *providers.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

// esignetConnectionResponse is the detail payload for an eSignet connection (signing key masked).
type esignetConnectionResponse struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Description           string   `json:"description,omitempty"`
	Type                  string   `json:"type"`
	ClientID              string   `json:"clientId,omitempty"`
	RedirectURI           string   `json:"redirectUri,omitempty"`
	AuthorizationEndpoint string   `json:"authorizationEndpoint,omitempty"`
	TokenEndpoint         string   `json:"tokenEndpoint,omitempty"`
	UserInfoEndpoint      string   `json:"userInfoEndpoint,omitempty"`
	JwksEndpoint          string   `json:"jwksEndpoint,omitempty"`
	SigningKey            string   `json:"signingKey,omitempty"`
	SigningKeyID          string   `json:"signingKeyId,omitempty"`
	Scopes                []string `json:"scopes,omitempty"`
	ACRValues             string   `json:"acrValues,omitempty"`
	UsernamePrefix        string   `json:"usernamePrefix,omitempty"`

	AttributeConfiguration *providers.AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

// singleLineSigningKey writes a PEM key's newlines as the two-character sequence \n. The key is
// a secret, so configuration export externalizes it to the generated .env file, where a value
// must occupy exactly one line; keeping the stored form single-line means the value travels
// through export, the .env file and a re-import unchanged. parseRSAPrivateKey restores the
// newlines where the key is used. Already single-line input passes through untouched.
func singleLineSigningKey(key string) string {
	return strings.NewReplacer("\r\n", `\n`, "\n", `\n`, "\r", `\n`).Replace(key)
}

func esignetToIDPDTO(req esignetConnectionRequest) (*providers.IDPDTO, error) {
	var props []cmodels.Property
	var err error
	// The signing key is the only secret: it is the private key that authenticates ThunderID to
	// eSignet, so it is encrypted at rest and masked on read, like a client secret elsewhere.
	fields := []struct {
		name     string
		value    string
		isSecret bool
	}{
		{idp.PropClientID, req.ClientID, false},
		{idp.PropRedirectURI, req.RedirectURI, false},
		{idp.PropAuthorizationEndpoint, req.AuthorizationEndpoint, false},
		{idp.PropTokenEndpoint, req.TokenEndpoint, false},
		{idp.PropUserInfoEndpoint, req.UserInfoEndpoint, false},
		{idp.PropJwksEndpoint, req.JwksEndpoint, false},
		{idp.PropSigningKey, singleLineSigningKey(req.SigningKey), true},
		{idp.PropSigningKeyID, req.SigningKeyID, false},
		{idp.PropScopes, joinScopes(req.Scopes), false},
		{idp.PropACRValues, req.ACRValues, false},
		{idp.PropUsernamePrefix, req.UsernamePrefix, false},
	}
	for _, field := range fields {
		if props, err = appendProperty(props, field.name, field.value, field.isSecret); err != nil {
			return nil, err
		}
	}
	return &providers.IDPDTO{
		Name:                   req.Name,
		Description:            req.Description,
		Type:                   providers.IDPTypeESignet,
		Properties:             props,
		AttributeConfiguration: req.AttributeConfiguration,
	}, nil
}

func esignetFromIDPDTO(dto providers.IDPDTO) (esignetConnectionResponse, error) {
	values, err := propertyValues(dto.Properties)
	if err != nil {
		return esignetConnectionResponse{}, err
	}
	return esignetConnectionResponse{
		ID:                     dto.ID,
		Name:                   dto.Name,
		Description:            dto.Description,
		Type:                   connectionTypeName(dto.Type),
		ClientID:               values[idp.PropClientID],
		RedirectURI:            values[idp.PropRedirectURI],
		AuthorizationEndpoint:  values[idp.PropAuthorizationEndpoint],
		TokenEndpoint:          values[idp.PropTokenEndpoint],
		UserInfoEndpoint:       values[idp.PropUserInfoEndpoint],
		JwksEndpoint:           values[idp.PropJwksEndpoint],
		SigningKey:             values[idp.PropSigningKey],
		SigningKeyID:           values[idp.PropSigningKeyID],
		Scopes:                 splitScopes(values[idp.PropScopes]),
		ACRValues:              values[idp.PropACRValues],
		UsernamePrefix:         values[idp.PropUsernamePrefix],
		AttributeConfiguration: dto.AttributeConfiguration,
	}, nil
}
