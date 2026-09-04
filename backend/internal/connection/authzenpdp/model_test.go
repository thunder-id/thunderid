// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnectionRequestJSONRoundTrip(t *testing.T) {
	request := ConnectionRequest{
		Name:                    "External PDP",
		Description:             "Test connection",
		Endpoint:                "https://pdp.example.com/access/v1/evaluation",
		BatchEndpoint:           "https://pdp.example.com/access/v1/evaluations",
		TimeoutMS:               1000,
		RetryCount:              2,
		SubjectProperties:       "email groups",
		SubjectPropertyMappings: "email: mail, groups: roles",
		SubjectAttributeMappings: []SubjectAttributeMapping{{
			UserType:   "Customer",
			Attributes: []SubjectAttributeRow{{Attribute: "email", PDPAttribute: "mail"}},
		}},
		FailOpen: true,
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
			err := ValidateEndpoint(tt.endpoint)
			if tt.valid {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, "endpoint must be an absolute URL")
		})
	}
}

func TestFromRequestNormalizesConnection(t *testing.T) {
	connection := FromRequest(ConnectionRequest{
		Name:                    "PDP",
		Endpoint:                "https://pdp.example.com",
		BatchEndpoint:           "https://pdp.example.com/batch",
		SubjectProperties:       "email, groups email",
		SubjectPropertyMappings: "email: mail, invalid, groups: roles",
		SubjectAttributeMappings: []SubjectAttributeMapping{{
			UserType: " Customer ",
			Attributes: []SubjectAttributeRow{
				{Attribute: " email ", PDPAttribute: " mail "},
				{Attribute: ""},
			},
		}},
	})

	require.Equal(t, []string{"email", "groups"}, connection.SubjectProperties)
	require.Equal(t, "https://pdp.example.com/batch", connection.BatchEndpoint)
	require.Equal(t, map[string]string{"email": "mail", "groups": "roles"}, connection.SubjectPropertyMappings)
	require.Equal(t, DefaultTimeoutMS, connection.TimeoutMS)
	require.Equal(t, DefaultRetryCount, connection.RetryCount)
	require.Equal(t, "Customer", connection.SubjectAttributeMappings[0].UserType)
}

func TestToResponseSerializesConnection(t *testing.T) {
	response := ToResponse(AuthZENPDPConnection{
		ID:                       "pdp-1",
		Name:                     "PDP",
		Endpoint:                 "https://pdp.example.com/access/v1/evaluation",
		BatchEndpoint:            "https://pdp.example.com/access/v1/evaluations",
		TimeoutMS:                1000,
		RetryCount:               2,
		SubjectProperties:        []string{"groups", "email"},
		SubjectPropertyMappings:  map[string]string{"groups": "roles", "email": "mail"},
		SubjectAttributeMappings: []SubjectAttributeMapping{{UserType: "Customer"}},
		FailOpen:                 true,
	})

	require.Equal(t, "pdp-1", response.ID)
	require.Equal(t, VendorName, response.Type)
	require.Equal(t, "https://pdp.example.com/access/v1/evaluation", response.Endpoint)
	require.Equal(t, "https://pdp.example.com/access/v1/evaluations", response.BatchEndpoint)
	require.Equal(t, "groups email", response.SubjectProperties)
	require.Equal(t, "email: mail, groups: roles", response.SubjectPropertyMappings)
	require.True(t, response.FailOpen)
}

func TestSubjectPropertyMappingsParsingAndJoining(t *testing.T) {
	require.Nil(t, SplitSubjectPropertyMappings(" , invalid, : missing "))
	require.Equal(t, map[string]string{"email": "mail", "groups": "roles"},
		SplitSubjectPropertyMappings("email: mail, groups: roles, invalid"))
	require.Equal(t, "email: mail, groups: roles", JoinSubjectPropertyMappings(map[string]string{
		"groups": "roles", "email": "mail", "": "ignored",
	}))
	require.Empty(t, JoinSubjectPropertyMappings(nil))
}

func TestRuntimeConfigPreservesSubjectAttributeMappings(t *testing.T) {
	connection := AuthZENPDPConnection{
		SubjectAttributeMappings: []SubjectAttributeMapping{
			{
				UserType:   "Agent",
				Attributes: []SubjectAttributeRow{{Attribute: "status", PDPAttribute: "agent_status"}},
			},
			{
				UserType:   "Customer",
				Attributes: []SubjectAttributeRow{{Attribute: "status", PDPAttribute: "customer_status"}},
			},
		},
	}

	groups := cloneSubjectAttributeMappings(connection.SubjectAttributeMappings)
	groups[0].Attributes[0].PDPAttribute = "changed"

	require.Equal(t, "agent_status", connection.SubjectAttributeMappings[0].Attributes[0].PDPAttribute)
	require.Equal(t, "customer_status", connection.SubjectAttributeMappings[1].Attributes[0].PDPAttribute)
}
