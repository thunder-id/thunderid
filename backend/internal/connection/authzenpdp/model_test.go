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

func TestFromRequestPreservesNativeSubjectMappings(t *testing.T) {
	retries := 2
	service := &AuthZENPDPService{defaults: config.AuthZENPDPConfig{TimeoutMS: 1200, RetryCount: &retries}}
	connection := service.fromRequest(ConnectionRequest{
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
