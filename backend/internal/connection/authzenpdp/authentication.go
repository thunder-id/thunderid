// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/httpauth"
)

// OutboundAuthenticationConfig decrypts credentials for an outbound PDP request.
func (connection AuthZENPDPConnection) OutboundAuthenticationConfig() (outboundauth.Config, error) {
	if len(connection.AuthenticationProperties) == 0 {
		return outboundauth.Config{Type: outboundauth.TypeNone}, nil
	}
	config, err := outboundauth.FromProperties(connection.AuthenticationProperties)
	if err != nil {
		return outboundauth.Config{}, fmt.Errorf("failed to decrypt outbound authentication: %w", err)
	}
	return httpauth.Canonicalize(config)
}

// SetAuthentication validates and stores outbound authentication credentials.
func (connection *AuthZENPDPConnection) SetAuthentication(request *outboundauth.Authentication) error {
	if connection == nil {
		return fmt.Errorf("connection is required")
	}
	config, err := httpauth.Canonicalize(request.Config())
	if err != nil {
		return err
	}
	properties, err := outboundauth.ToProperties(config)
	if err != nil {
		return err
	}
	connection.AuthenticationScheme = string(config.Type)
	connection.AuthenticationProperties = properties
	return nil
}

func (connection AuthZENPDPConnection) authenticationResponse() outboundauth.Authentication {
	values := make(map[string]string, len(connection.AuthenticationProperties))
	for _, property := range connection.AuthenticationProperties {
		if property.IsSecret() {
			values[property.GetName()] = "******"
			continue
		}
		value, err := property.GetValue()
		if err == nil {
			values[property.GetName()] = value
		}
	}
	return outboundauth.AuthenticationFromValues(values)
}
