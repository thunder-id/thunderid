// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/httpauth"
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
	cfg.Crypto.Encryption.Key = "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"
	if err := config.InitializeServerRuntime("", &cfg); err != nil {
		panic(err)
	}
	_, cryptoProvider, err := defaultkm.Initialize(nil)
	if err != nil {
		panic(err)
	}
	cmodels.SetConfigCryptoProvider(cryptoProvider)
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
			name: "basic authenticated remote HTTPS", endpoint: "https://pdp.example.com/evaluation",
			scheme: "BASIC", valid: true,
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
		{name: "basic authenticated remote HTTP", endpoint: "http://pdp.example.com/evaluation", scheme: "BASIC"},
		{
			name: "authenticated remote HTTP batch endpoint", endpoint: "https://pdp.example.com/evaluation",
			batch: "http://pdp.example.com/evaluations", scheme: "API_KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typeProperty, err := cmodels.NewProperty(outboundauth.PropertyKeyType, strings.ToLower(tt.scheme), false)
			require.NoError(t, err)
			err = validateConnection(AuthZENPDPConnection{
				Endpoint:                 tt.endpoint,
				BatchEndpoint:            tt.batch,
				AuthenticationProperties: []cmodels.Property{*typeProperty},
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
	connection, err := ConnectionFromRequest(ConnectionRequest{
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
	}, config.AuthZENPDPConfig{TimeoutMS: 1200, RetryCount: &retries})
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
		header string
		value  string
	}{
		{name: "invalid name", header: "X Key", value: "secret"},
		{name: "line break in value", header: "X-API-Key", value: "secret\nvalue"},
		{name: "reserved header", header: "Content-Type", value: "text/plain"},
		{name: "masked value", header: "X-API-Key", value: "******"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ConnectionFromRequest(ConnectionRequest{Authentication: &outboundauth.Authentication{
				Type: string(outboundauth.TypeAPIKey), Properties: map[string]string{test.header: test.value},
			}}, config.AuthZENPDPConfig{})
			require.Error(t, err)
		})
	}
}

func TestAuthenticationPropertiesRequiresSelectedCredentials(t *testing.T) {
	tests := []struct {
		name    string
		request outboundauth.Authentication
	}{
		{name: "none with bearer", request: outboundauth.Authentication{
			Type: string(outboundauth.TypeNone), Properties: map[string]string{"token": "token"},
		}},
		{name: "bearer without bearer credentials", request: outboundauth.Authentication{
			Type: string(outboundauth.TypeBearer),
		}},
		{name: "bearer with masked token", request: outboundauth.Authentication{
			Type: string(outboundauth.TypeBearer), Properties: map[string]string{"token": "******"},
		}},
		{name: "basic with masked password", request: outboundauth.Authentication{
			Type: string(outboundauth.TypeBasic), Properties: map[string]string{
				"username": "client", "password": "******",
			},
		}},
		{name: "basic with API key credentials", request: outboundauth.Authentication{
			Type: string(outboundauth.TypeBasic), Properties: map[string]string{"X-API-Key": "secret"},
		}},
		{name: "API key with invalid header", request: outboundauth.Authentication{
			Type:       string(outboundauth.TypeAPIKey),
			Properties: map[string]string{"X-API-Key": "secret", "Bad Header": "token"},
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ConnectionFromRequest(ConnectionRequest{Authentication: &test.request}, config.AuthZENPDPConfig{})
			require.Error(t, err)
		})
	}
}

func TestUpdateAuthenticationRetainsOnlySubmittedMaskedHeaders(t *testing.T) {
	current := AuthZENPDPConnection{
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
	}
	current.AuthenticationProperties = testAuthProperties(t, outboundauth.Authentication{
		Type: string(outboundauth.TypeAPIKey),
		Properties: map[string]string{
			"X-First-Key": "first-secret", "X-Second-Key": "second-secret",
		},
	})
	request := &outboundauth.Authentication{Type: string(outboundauth.TypeAPIKey),
		Properties: map[string]string{"X-Second-Key": "******", "X-Third-Key": "third-secret"}}
	merged, err := outboundauth.UpdateProperties(request, current.AuthenticationProperties,
		[]string{current.Endpoint, current.BatchEndpoint}, []string{current.Endpoint, current.BatchEndpoint},
		httpauth.SupportedTypes(), httpauth.CanonicalizeForUpdate, httpauth.Canonicalize)
	require.NoError(t, err)
	config, err := outboundauth.FromProperties(merged)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"X-Second-Key": "second-secret", "X-Third-Key": "third-secret",
	}, config.Properties)

	_, err = outboundauth.UpdateProperties(request, current.AuthenticationProperties,
		[]string{current.Endpoint, current.BatchEndpoint},
		[]string{"https://other.example.com/evaluation", current.BatchEndpoint},
		httpauth.SupportedTypes(), httpauth.CanonicalizeForUpdate, httpauth.Canonicalize)
	require.Error(t, err)
}

func TestUpdateAuthenticationRetainsMaskedBearerAndBasicCredentials(t *testing.T) {
	tests := []struct {
		name       string
		authType   outboundauth.Type
		stored     map[string]string
		request    map[string]string
		secretName string
	}{
		{"bearer", outboundauth.TypeBearer,
			map[string]string{"token": "bearer-secret"}, map[string]string{"token": "******"}, "token"},
		{"basic", outboundauth.TypeBasic,
			map[string]string{"username": "client", "password": "basic-secret"},
			map[string]string{"username": "client", "password": "******"}, "password"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := AuthZENPDPConnection{Endpoint: "https://pdp.example.com/evaluation"}
			current.AuthenticationProperties = testAuthProperties(t, outboundauth.Authentication{
				Type: string(test.authType), Properties: test.stored,
			})
			request := &outboundauth.Authentication{Type: string(test.authType), Properties: test.request}
			merged, err := outboundauth.UpdateProperties(request, current.AuthenticationProperties,
				[]string{current.Endpoint}, []string{current.Endpoint},
				httpauth.SupportedTypes(), httpauth.CanonicalizeForUpdate, httpauth.Canonicalize)
			require.NoError(t, err)
			config, err := outboundauth.FromProperties(merged)
			require.NoError(t, err)
			require.Equal(t, test.stored[test.secretName], config.Get(test.secretName))

			_, err = outboundauth.UpdateProperties(request, current.AuthenticationProperties,
				[]string{current.Endpoint}, []string{"https://other.example.com/evaluation"},
				httpauth.SupportedTypes(), httpauth.CanonicalizeForUpdate, httpauth.Canonicalize)
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

func testAuthProperties(t *testing.T, authentication outboundauth.Authentication) []cmodels.Property {
	t.Helper()
	config, err := httpauth.Canonicalize(authentication.Config())
	require.NoError(t, err)
	properties, err := outboundauth.ToProperties(config)
	require.NoError(t, err)
	return properties
}
