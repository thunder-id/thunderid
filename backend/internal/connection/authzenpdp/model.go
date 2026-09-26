// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"fmt"
	"strings"

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
type AuthenticationRequest struct {
	Scheme        string      `json:"scheme" yaml:"scheme"`
	BearerToken   string      `json:"bearerToken,omitempty" yaml:"bearerToken,omitempty"`
	APIKeyHeaders []APIHeader `json:"apiKeyHeaders,omitempty" yaml:"apiKeyHeaders,omitempty"`
}

// AuthenticationResponse returns the configured scheme and header names, never credential values.
type AuthenticationResponse struct {
	Scheme        string      `json:"scheme"`
	APIKeyHeaders []APIHeader `json:"apiKeyHeaders,omitempty"`
}

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
	config.APIKeyHeaders = make(map[string]string)
	for _, property := range connection.AuthenticationProperties {
		value, err := property.GetValue()
		if err != nil {
			return outboundauthn.Config{}, fmt.Errorf("failed to decrypt outbound authentication credential: %w", err)
		}
		if property.GetName() == bearerTokenProperty {
			config.BearerToken = value
		} else {
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

const bearerTokenProperty = "__bearerToken"

func (connection AuthZENPDPConnection) authenticationResponse() AuthenticationResponse {
	response := AuthenticationResponse{Scheme: connection.AuthenticationScheme}
	if response.Scheme == "" {
		response.Scheme = outboundauthn.SchemeNone
	}
	if response.Scheme != outboundauthn.SchemeAPIKey {
		return response
	}
	for _, property := range connection.AuthenticationProperties {
		if property.GetName() != bearerTokenProperty {
			response.APIKeyHeaders = append(response.APIKeyHeaders, APIHeader{
				Name: property.GetName(), Value: "******",
			})
		}
	}
	return response
}

func authenticationProperties(request *AuthenticationRequest) (string, []cmodels.Property, error) {
	if request == nil {
		return outboundauthn.SchemeNone, nil, nil
	}
	scheme := strings.ToUpper(strings.TrimSpace(request.Scheme))
	if scheme == "" {
		scheme = outboundauthn.SchemeNone
	}
	if scheme != outboundauthn.SchemeNone &&
		scheme != outboundauthn.SchemeBearer && scheme != outboundauthn.SchemeAPIKey {
		return "", nil, fmt.Errorf("unsupported outbound authentication scheme %q", request.Scheme)
	}
	if scheme == outboundauthn.SchemeBearer {
		if strings.TrimSpace(request.BearerToken) == "" {
			return "", nil, fmt.Errorf("bearer token is required")
		}
		property, err := cmodels.NewProperty(bearerTokenProperty, request.BearerToken, true)
		if err != nil {
			return "", nil, err
		}
		return scheme, []cmodels.Property{*property}, nil
	}
	if scheme == outboundauthn.SchemeAPIKey {
		if len(request.APIKeyHeaders) == 0 {
			return "", nil, fmt.Errorf("at least one API key header is required")
		}
		properties := make([]cmodels.Property, 0, len(request.APIKeyHeaders))
		seen := make(map[string]struct{}, len(request.APIKeyHeaders))
		for _, header := range request.APIKeyHeaders {
			name, err := outboundauthn.ValidateAPIKeyHeader(header.Name, header.Value)
			if err != nil {
				return "", nil, err
			}
			if name == bearerTokenProperty {
				return "", nil, fmt.Errorf("API key header name is reserved")
			}
			if _, found := seen[name]; found {
				return "", nil, fmt.Errorf("duplicate API key header %q", name)
			}
			seen[name] = struct{}{}
			property, err := cmodels.NewProperty(name, header.Value, true)
			if err != nil {
				return "", nil, err
			}
			properties = append(properties, *property)
		}
		return scheme, properties, nil
	}
	return scheme, nil, nil
}
