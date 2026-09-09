// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthZENPDPConnectionSettingsRoundTripThroughProperties(t *testing.T) {
	expected := AuthZENPDPConnection{
		ID: "pdp-1", Name: "PDP", Description: "Test connection",
		Endpoint:      "https://pdp.example.com/evaluation",
		BatchEndpoint: "https://pdp.example.com/evaluations",
		TimeoutMS:     1500, RetryCount: 0,
		SubjectProperties:       []string{"email"},
		SubjectPropertyMappings: map[string]string{"email": "mail"},
		SubjectAttributeMappings: []SubjectAttributeMapping{{
			UserType:   "Customer",
			Attributes: []SubjectAttributeRow{{Attribute: "status", PDPAttribute: "account_status"}},
		}},
	}
	properties, err := encodeAuthZENPDPProperties(expected)
	require.NoError(t, err)
	actual, err := buildAuthZENPDPConnection(map[string]interface{}{
		"id": []byte(expected.ID), "name": expected.Name, "description": expected.Description,
		"properties": []byte(properties),
	})
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestBuildAuthZENPDPConnectionDefaults(t *testing.T) {
	for _, raw := range []string{`{}`, `{"timeoutMs":-1,"retryCount":-1}`} {
		t.Run(raw, func(t *testing.T) {
			connection, err := buildAuthZENPDPConnection(map[string]interface{}{"properties": raw})
			require.NoError(t, err)
			require.Equal(t, DefaultTimeoutMS(), connection.TimeoutMS)
			require.Equal(t, DefaultRetryCount(), connection.RetryCount)
		})
	}
}

func TestBuildAuthZENPDPConnectionRejectsMalformedProperties(t *testing.T) {
	for _, raw := range []string{"", "{", `{"timeoutMs":"invalid"}`} {
		_, err := buildAuthZENPDPConnection(map[string]interface{}{"properties": raw})
		require.Error(t, err)
	}
}

func TestStringValueSupportsDatabaseValueTypes(t *testing.T) {
	require.Equal(t, "value", stringValue("value"))
	require.Equal(t, "value", stringValue([]byte("value")))
	require.Empty(t, stringValue(42))
}
