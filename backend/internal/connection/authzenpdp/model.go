// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/outboundauthn"
)

// ConnectionRequest is the API representation of an AuthZEN PDP connection request.
type ConnectionRequest struct {
	ID                       string                    `json:"-" yaml:"-"`
	Name                     string                    `json:"name"`
	Description              string                    `json:"description,omitempty"`
	Endpoint                 string                    `json:"endpoint"`
	BatchEndpoint            string                    `json:"batchEndpoint,omitempty"`
	TimeoutMS                int                       `json:"timeoutMs,omitempty"`
	RetryCount               *int                      `json:"retryCount,omitempty"`
	Authentication           *AuthenticationRequest    `json:"authentication,omitempty"`
	SubjectAttributeMappings []SubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
}

// ConnectionResponse is the API representation of an AuthZEN PDP connection.
type ConnectionResponse struct {
	ID                       string                    `json:"id"`
	Name                     string                    `json:"name"`
	Description              string                    `json:"description,omitempty"`
	Type                     string                    `json:"type"`
	Endpoint                 string                    `json:"endpoint"`
	BatchEndpoint            string                    `json:"batchEndpoint,omitempty"`
	TimeoutMS                int                       `json:"timeoutMs"`
	RetryCount               int                       `json:"retryCount"`
	Authentication           AuthenticationResponse    `json:"authentication"`
	SubjectAttributeMappings []SubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
}

// ToResponse converts an internal connection into an API response.
func ToResponse(connection AuthZENPDPConnection) ConnectionResponse {
	return ConnectionResponse{
		ID:                       connection.ID,
		Name:                     connection.Name,
		Description:              connection.Description,
		Type:                     "authzen-pdp",
		Endpoint:                 connection.Endpoint,
		BatchEndpoint:            connection.BatchEndpoint,
		TimeoutMS:                connection.TimeoutMS,
		RetryCount:               connection.RetryCount,
		Authentication:           connection.authenticationResponse(),
		SubjectAttributeMappings: connection.SubjectAttributeMappings,
	}
}

// AuthZENPDPConnection is the internal representation of an AuthZEN PDP connection.
type AuthZENPDPConnection struct {
	ID                       string
	Name                     string
	Description              string
	IsReadOnly               bool
	Endpoint                 string
	BatchEndpoint            string
	TimeoutMS                int
	RetryCount               int
	AuthenticationScheme     string
	AuthenticationProperties []cmodels.Property
	SubjectAttributeMappings []SubjectAttributeMapping
}

// AuthenticationRequest configures the credentials used for PDP HTTP requests.
type AuthenticationRequest = outboundauthn.Authentication

// AuthenticationResponse returns configured credentials with values masked.
type AuthenticationResponse = outboundauthn.Authentication

// BearerAuthentication configures an OAuth bearer token.
type BearerAuthentication = outboundauthn.BearerCredentials

// BasicAuthentication configures HTTP Basic credentials.
type BasicAuthentication = outboundauthn.BasicCredentials

// APIKeyAuthentication configures API-key HTTP headers.
type APIKeyAuthentication = outboundauthn.APIKeyCredentials

// APIHeader is one API-key HTTP header. Values are write-only credentials.
type APIHeader = outboundauthn.APIKeyHeader

// SubjectAttributeMapping maps attributes from a ThunderID entity type to PDP subject attributes.
type SubjectAttributeMapping struct {
	EntityType string                `json:"entityType" yaml:"entityType"`
	Attributes []SubjectAttributeRow `json:"attributes" yaml:"attributes"`
}

// SubjectAttributeRow identifies one subject attribute mapping.
type SubjectAttributeRow struct {
	Attribute    string `json:"attribute" yaml:"attribute"`
	PDPAttribute string `json:"pdpAttribute,omitempty" yaml:"pdpAttribute,omitempty"`
}

// OutboundAuthenticationConfig decrypts credentials for use in an outbound PDP request.
func (connection AuthZENPDPConnection) OutboundAuthenticationConfig() (outboundauthn.Config, error) {
	config := outboundauthn.Config{Scheme: connection.AuthenticationScheme}
	if config.Scheme == "" {
		config.Scheme = outboundauthn.SchemeNone
	}
	if config.Scheme == outboundauthn.SchemeNone {
		return config, nil
	}
	for _, property := range connection.AuthenticationProperties {
		value, err := property.GetValue()
		if err != nil {
			return outboundauthn.Config{}, fmt.Errorf("failed to decrypt outbound authentication credential: %w", err)
		}
		if property.GetName() == bearerTokenProperty {
			config.BearerToken = value
			continue
		}
		if property.GetName() == basicUsernameProperty {
			config.BasicUsername = value
			continue
		}
		if property.GetName() == basicPasswordProperty {
			config.BasicPassword = value
			continue
		}
		if property.GetName() == apiKeyHeadersProperty {
			headers, err := outboundauthn.APIKeyHeadersFromProperty(property, false)
			if err != nil {
				return outboundauthn.Config{}, err
			}
			config.APIKeyHeaders = make(map[string]string, len(headers))
			for _, header := range headers {
				config.APIKeyHeaders[header.Name] = header.Value
			}
		} else {
			if config.APIKeyHeaders == nil {
				config.APIKeyHeaders = make(map[string]string)
			}
			config.APIKeyHeaders[property.GetName()] = value
		}
	}
	return config, nil
}

// SetAuthentication validates and stores outbound authentication credentials on the connection.
func (connection *AuthZENPDPConnection) SetAuthentication(request *AuthenticationRequest) error {
	if connection == nil {
		return fmt.Errorf("connection is required")
	}
	scheme, properties, err := authenticationProperties(request)
	if err != nil {
		return err
	}
	connection.AuthenticationScheme = scheme
	connection.AuthenticationProperties = properties
	return nil
}

const (
	bearerTokenProperty   = "__bearerToken"
	basicUsernameProperty = "__basicUsername"
	basicPasswordProperty = "__basicPassword"
	apiKeyHeadersProperty = "__apiKeyHeaders"
)

func (connection AuthZENPDPConnection) authenticationResponse() AuthenticationResponse {
	response := AuthenticationResponse{Scheme: connection.AuthenticationScheme}
	if response.Scheme == "" {
		response.Scheme = outboundauthn.SchemeNone
	}
	switch response.Scheme {
	case outboundauthn.SchemeBearer:
		response.Bearer = &BearerAuthentication{Token: outboundauthn.MaskedSecretValue}
	case outboundauthn.SchemeBasic:
		response.Basic = &BasicAuthentication{
			Username: outboundauthn.MaskedSecretValue,
			Password: outboundauthn.MaskedSecretValue,
		}
	case outboundauthn.SchemeAPIKey:
		response.APIKey = &APIKeyAuthentication{}
		for _, property := range connection.AuthenticationProperties {
			if property.GetName() == apiKeyHeadersProperty {
				headers, err := outboundauthn.APIKeyHeadersFromProperty(property, true)
				if err == nil {
					response.APIKey.Headers = append(response.APIKey.Headers, headers...)
				}
				continue
			}
			if property.GetName() != bearerTokenProperty && property.GetName() != basicUsernameProperty &&
				property.GetName() != basicPasswordProperty {
				response.APIKey.Headers = append(response.APIKey.Headers, APIHeader{
					Name: property.GetName(), Value: outboundauthn.MaskedSecretValue,
				})
			}
		}
	}
	return response
}

func authenticationProperties(request *AuthenticationRequest) (string, []cmodels.Property, error) {
	config, err := outboundauthn.ConfigFromAuthentication(request)
	if err != nil {
		return "", nil, err
	}
	scheme := config.Scheme
	if scheme == outboundauthn.SchemeBearer {
		property, err := cmodels.NewProperty(bearerTokenProperty, config.BearerToken, true)
		if err != nil {
			return "", nil, err
		}
		return scheme, []cmodels.Property{*property}, nil
	}
	if scheme == outboundauthn.SchemeBasic {
		usernameProperty, err := cmodels.NewProperty(basicUsernameProperty, config.BasicUsername, true)
		if err != nil {
			return "", nil, err
		}
		passwordProperty, err := cmodels.NewProperty(basicPasswordProperty, config.BasicPassword, true)
		if err != nil {
			return "", nil, err
		}
		return scheme, []cmodels.Property{*usernameProperty, *passwordProperty}, nil
	}
	if scheme == outboundauthn.SchemeAPIKey {
		property, err := outboundauthn.NewAPIKeyHeadersProperty(apiKeyHeadersProperty, request.APIKey.Headers)
		if err != nil {
			return "", nil, err
		}
		return scheme, []cmodels.Property{*property}, nil
	}
	return scheme, nil, nil
}
