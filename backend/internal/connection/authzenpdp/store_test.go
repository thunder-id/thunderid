// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeAuthZENPDPPropertiesStoresEmptyPropertiesAsArray(t *testing.T) {
	subjectProperties, mappings, err := encodeAuthZENPDPProperties(AuthZENPDPConnection{})

	require.NoError(t, err)
	require.JSONEq(t, `{"properties":[]}`, subjectProperties)
	require.JSONEq(t, `null`, mappings)
}

func TestDecodeSubjectPropertiesAcceptsNullPropertiesObject(t *testing.T) {
	var connection AuthZENPDPConnection

	decodeSubjectProperties(`{"properties":null}`, &connection)

	require.Nil(t, connection.SubjectProperties)
	require.Nil(t, connection.SubjectAttributeMappings)
}

func TestDecodeSubjectPropertiesAcceptsGroupedObject(t *testing.T) {
	var connection AuthZENPDPConnection

	decodeSubjectProperties(
		`{"properties":["email","groups"],"mappings":[{"userType":"TravelCustomer",`+
			`"attributes":[{"attribute":"groups"}]}]}`,
		&connection,
	)

	require.Equal(t, []string{"email", "groups"}, connection.SubjectProperties)
	require.Equal(t, []SubjectAttributeMapping{
		{
			UserType: "TravelCustomer",
			Attributes: []SubjectAttributeRow{
				{Attribute: "groups"},
			},
		},
	}, connection.SubjectAttributeMappings)
}

func TestDecodeSubjectPropertiesAcceptsLegacyArray(t *testing.T) {
	var connection AuthZENPDPConnection

	decodeSubjectProperties(`["email","groups"]`, &connection)

	require.Equal(t, []string{"email", "groups"}, connection.SubjectProperties)
}

func TestDecodeSubjectPropertiesAcceptsLegacyPlainText(t *testing.T) {
	var connection AuthZENPDPConnection

	decodeSubjectProperties(`email groups`, &connection)

	require.Equal(t, []string{"email", "groups"}, connection.SubjectProperties)
}

func TestNormalizedSubjectMappingDoesNotFlattenGroupedAttributes(t *testing.T) {
	subjectProperties, subjectPropertyMappings := NormalizedSubjectMapping(AuthZENPDPConnection{
		SubjectProperties:       []string{"email"},
		SubjectPropertyMappings: map[string]string{"email": "email"},
		SubjectAttributeMappings: []SubjectAttributeMapping{
			{
				UserType: "TravelCustomer",
				Attributes: []SubjectAttributeRow{
					{Attribute: "groups"},
					{Attribute: "accountStatus", PDPAttribute: "active"},
				},
			},
		},
	})

	require.Equal(t, []string{"email"}, subjectProperties)
	require.Equal(t, map[string]string{
		"email": "email",
	}, subjectPropertyMappings)
}

func TestBuildAuthZENPDPConnectionAppliesDefaults(t *testing.T) {
	connection := buildAuthZENPDPConnection(map[string]interface{}{
		"id":                 []byte("pdp-1"),
		"name":               "External PDP",
		"description":        "Test PDP",
		"endpoint":           "https://pdp.example.com/access/v1/evaluation",
		"subject_properties": `{"properties":[],"batchEndpoint":"https://pdp.example.com/access/v1/evaluations"}`,
	})

	require.Equal(t, "pdp-1", connection.ID)
	require.Equal(t, "External PDP", connection.Name)
	require.Equal(t, "https://pdp.example.com/access/v1/evaluation", connection.Endpoint)
	require.Equal(t, "https://pdp.example.com/access/v1/evaluations", connection.BatchEndpoint)
	require.Equal(t, DefaultTimeoutMS, connection.TimeoutMS)
	require.Equal(t, DefaultRetryCount, connection.RetryCount)
}

func TestBuildAuthZENPDPConnectionDefaultsNegativeRetryCount(t *testing.T) {
	connection := buildAuthZENPDPConnection(map[string]interface{}{
		"subject_properties": `{"properties":[],"retryCount":-1}`,
	})

	require.Equal(t, DefaultRetryCount, connection.RetryCount)
}

func TestAuthZENPDPConnectionSettingsRoundTripThroughStoredProperties(t *testing.T) {
	subjectProperties, subjectPropertyMappings, err := encodeAuthZENPDPProperties(AuthZENPDPConnection{
		SubjectProperties: []string{"email"},
		BatchEndpoint:     "https://pdp.example.com/access/v1/evaluations",
		TimeoutMS:         1500,
		RetryCount:        3,
		SubjectPropertyMappings: map[string]string{
			"email": "mail",
		},
		SubjectAttributeMappings: []SubjectAttributeMapping{{
			UserType:   "Customer",
			Attributes: []SubjectAttributeRow{{Attribute: "accountStatus", PDPAttribute: "status"}},
		}},
		FailOpen: true,
	})
	require.NoError(t, err)

	connection := buildAuthZENPDPConnection(map[string]interface{}{
		"id":                        "pdp-1",
		"name":                      "External PDP",
		"endpoint":                  "https://pdp.example.com/access/v1/evaluation",
		"subject_properties":        subjectProperties,
		"subject_property_mappings": subjectPropertyMappings,
	})

	require.Equal(t, []string{"email"}, connection.SubjectProperties)
	require.Equal(t, "https://pdp.example.com/access/v1/evaluation", connection.Endpoint)
	require.Equal(t, "https://pdp.example.com/access/v1/evaluations", connection.BatchEndpoint)
	require.Equal(t, 1500, connection.TimeoutMS)
	require.Equal(t, 3, connection.RetryCount)
	require.Equal(t, map[string]string{"email": "mail"}, connection.SubjectPropertyMappings)
	require.Equal(t, []SubjectAttributeMapping{{
		UserType:   "Customer",
		Attributes: []SubjectAttributeRow{{Attribute: "accountStatus", PDPAttribute: "status"}},
	}}, connection.SubjectAttributeMappings)
	require.True(t, connection.FailOpen)
}

func TestStringValueSupportsDatabaseValueTypes(t *testing.T) {
	require.Equal(t, "value", stringValue("value"))
	require.Equal(t, "value", stringValue([]byte("value")))
	require.Empty(t, stringValue(42))
}
