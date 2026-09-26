// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
)

func TestMain(m *testing.M) {
	data, err := os.ReadFile("../../../cmd/server/config/default.json")
	if err != nil {
		panic(err)
	}
	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		panic(err)
	}
	if err := config.InitializeServerRuntime("", &cfg); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestConnectionRequestJSONRoundTrip(t *testing.T) {
	retries := 2
	request := ConnectionRequest{
		Name:          "AuthZEN PDP",
		Description:   "Test connection",
		Endpoint:      "https://pdp.example.com/access/v1/evaluation",
		BatchEndpoint: "https://pdp.example.com/access/v1/evaluations",
		TimeoutMS:     1000,
		RetryCount:    &retries,
		SubjectAttributeMappings: []SubjectAttributeMapping{{
			EntityType: "Customer",
			Attributes: []SubjectAttributeRow{{Attribute: "email", PDPAttribute: "mail"}},
		}},
	}

	data, err := json.Marshal(request)
	require.NoError(t, err)
	require.Contains(t, string(data), `"endpoint":"https://pdp.example.com/access/v1/evaluation"`)
	require.Contains(t, string(data), `"batchEndpoint":"https://pdp.example.com/access/v1/evaluations"`)

	var decoded ConnectionRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, request, decoded)
}

func TestConnectionRequestUnmarshalRejectsInvalidEndpoint(t *testing.T) {
	var request ConnectionRequest

	err := json.Unmarshal([]byte(`{"endpoint":42}`), &request)
	require.Error(t, err)
}

func TestConnectionRequestUnmarshalAllowsNullEndpoint(t *testing.T) {
	var request ConnectionRequest

	require.NoError(t, json.Unmarshal([]byte(`{"name":"PDP","endpoint":null}`), &request))
	require.Equal(t, "PDP", request.Name)
	require.Empty(t, request.Endpoint)
}

func TestValidateEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		valid    bool
	}{
		{name: "absolute", endpoint: "http://localhost:3592/access/v1/evaluation", valid: true},
		{name: "unsupported scheme", endpoint: "ftp://pdp.example.com/access/v1/evaluation"},
		{name: "missing host", endpoint: "http:///access/v1/evaluation"},
		{name: "relative", endpoint: "/access/v1/evaluation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEndpoint(tt.endpoint)
			if tt.valid {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, "endpoint must be an absolute URL")
		})
	}
}

func TestValidateConnectionRequiresHTTPSForRemoteAuthenticatedEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		batch    string
		scheme   string
		valid    bool
	}{
		{
			name: "unauthenticated remote HTTP", endpoint: "http://pdp.example.com/evaluation",
			scheme: "NONE", valid: true,
		},
		{
			name: "authenticated remote HTTPS", endpoint: "https://pdp.example.com/evaluation",
			scheme: "BEARER", valid: true,
		},
		{
			name: "authenticated localhost HTTP", endpoint: "http://localhost:3592/evaluation",
			scheme: "API_KEY", valid: true,
		},
		{
			name: "authenticated IPv4 loopback HTTP", endpoint: "http://127.0.0.1:3592/evaluation",
			scheme: "API_KEY", valid: true,
		},
		{
			name: "authenticated IPv6 loopback HTTP", endpoint: "http://[::1]:3592/evaluation",
			scheme: "API_KEY", valid: true,
		},
		{name: "authenticated remote HTTP", endpoint: "http://pdp.example.com/evaluation", scheme: "BEARER"},
		{
			name: "authenticated remote HTTP batch endpoint", endpoint: "https://pdp.example.com/evaluation",
			batch: "http://pdp.example.com/evaluations", scheme: "API_KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConnection(AuthZENPDPConnection{
				Endpoint:             tt.endpoint,
				BatchEndpoint:        tt.batch,
				AuthenticationScheme: tt.scheme,
			})
			if tt.valid {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, "HTTPS is required when authentication is configured")
		})
	}
}

func TestFromRequestPreservesNativeSubjectMappings(t *testing.T) {
	retries := 2
	service := &AuthZENPDPService{defaults: config.AuthZENPDPConfig{TimeoutMS: 1200, RetryCount: &retries}}
	connection, err := service.fromRequest(ConnectionRequest{
		Name:          "PDP",
		Endpoint:      "https://pdp.example.com",
		BatchEndpoint: "https://pdp.example.com/batch",
		SubjectAttributeMappings: []SubjectAttributeMapping{{
			EntityType: " Customer ",
			Attributes: []SubjectAttributeRow{
				{Attribute: " email ", PDPAttribute: " mail "},
				{Attribute: ""},
			},
		}},
	})
	require.NoError(t, err)

	require.Equal(t, "https://pdp.example.com/batch", connection.BatchEndpoint)
	require.Equal(t, 1200, connection.TimeoutMS)
	require.Equal(t, 2, connection.RetryCount)
	require.Equal(t, "Customer", connection.SubjectAttributeMappings[0].EntityType)
}

func TestToResponseSerializesConnection(t *testing.T) {
	response := ToResponse(AuthZENPDPConnection{
		ID:                       "pdp-1",
		Name:                     "PDP",
		Endpoint:                 "https://pdp.example.com/access/v1/evaluation",
		BatchEndpoint:            "https://pdp.example.com/access/v1/evaluations",
		TimeoutMS:                1000,
		RetryCount:               2,
		SubjectAttributeMappings: []SubjectAttributeMapping{{EntityType: "Customer"}},
	})

	require.Equal(t, "pdp-1", response.ID)
	require.Equal(t, "authzen-pdp", response.Type)
	require.Equal(t, "https://pdp.example.com/access/v1/evaluation", response.Endpoint)
	require.Equal(t, "https://pdp.example.com/access/v1/evaluations", response.BatchEndpoint)
}

func TestAuthenticationPropertiesRejectsInvalidAPIKeyHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header APIHeader
	}{
		{name: "invalid name", header: APIHeader{Name: "X Key", Value: "secret"}},
		{name: "line break in value", header: APIHeader{Name: "X-API-Key", Value: "secret\nvalue"}},
		{name: "reserved header", header: APIHeader{Name: "Content-Type", Value: "text/plain"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := authenticationProperties(&AuthenticationRequest{
				Scheme:        "API_KEY",
				APIKeyHeaders: []APIHeader{test.header},
			})
			require.Error(t, err)
		})
	}
}

func TestRuntimeConfigPreservesSubjectAttributeMappings(t *testing.T) {
	connection := AuthZENPDPConnection{
		SubjectAttributeMappings: []SubjectAttributeMapping{
			{
				EntityType: "Agent",
				Attributes: []SubjectAttributeRow{{Attribute: "status", PDPAttribute: "agent_status"}},
			},
			{
				EntityType: "Customer",
				Attributes: []SubjectAttributeRow{{Attribute: "status", PDPAttribute: "customer_status"}},
			},
		},
	}

	groups := cloneSubjectAttributeMappings(connection.SubjectAttributeMappings)
	groups[0].Attributes[0].PDPAttribute = "changed"

	require.Equal(t, "agent_status", connection.SubjectAttributeMappings[0].Attributes[0].PDPAttribute)
	require.Equal(t, "customer_status", connection.SubjectAttributeMappings[1].Attributes[0].PDPAttribute)
}
