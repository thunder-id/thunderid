// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const testBaseURL = "https://thunderid.example.com"

// TestHandleServiceProviderConfigGetRequest_Success tests Handle Service Provider Config Get Request for Success.
func TestHandleServiceProviderConfigGetRequest_Success(t *testing.T) {
	expectedConfig := SCIMServiceProviderConfig{
		Schemas: []string{SCIMServiceProviderConfigSchemaURN},
		Patch:   SCIMSupportedFeature{Supported: true},
		Bulk: SCIMBulkConfig{
			Supported:      false,
			MaxOperations:  0,
			MaxPayloadSize: 0,
		},
		Filter: SCIMFilterConfig{
			Supported:  true,
			MaxResults: constants.MaxPageSize,
		},
		ChangePassword: SCIMSupportedFeature{Supported: false},
		Sort:           SCIMSupportedFeature{Supported: false},
		ETag:           SCIMSupportedFeature{Supported: true},
		AuthenticationSchemes: []SCIMAuthenticationScheme{
			{
				Type:        "oauthbearertoken",
				Name:        "OAuth Bearer Token",
				Description: "Authentication using an OAuth 2.0 Bearer Token",
			},
		},
		Meta: SCIMMeta{
			ResourceType: "ServiceProviderConfig",
			Location:     testBaseURL + "/scim/v2/ServiceProviderConfig",
			Created:      scimServerStartTime,
			LastModified: scimServerStartTime,
		},
	}

	mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
	mockSvc.On("GetServiceProviderConfig", mock.Anything, testBaseURL).
		Return(expectedConfig)

	h := newSCIMDiscoveryHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	rr := httptest.NewRecorder()

	h.HandleServiceProviderConfigGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, constants.SCIMContentType, rr.Header().Get("Content-Type"))

	var got SCIMServiceProviderConfig
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, expectedConfig, got)
}

// TestHandleServiceProviderConfigGetRequest_PassesBaseURL tests Handle Service Provider Config Get Request
// for Passes Base URL.
func TestHandleServiceProviderConfigGetRequest_PassesBaseURL(t *testing.T) {
	var capturedURL string

	mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
	mockSvc.On("GetServiceProviderConfig", mock.Anything, testBaseURL).
		Return(SCIMServiceProviderConfig{
			Schemas: []string{SCIMServiceProviderConfigSchemaURN},
			Meta:    SCIMMeta{Location: testBaseURL + "/scim/v2/ServiceProviderConfig"},
		}).
		Run(func(args mock.Arguments) {
			capturedURL = args.String(1)
		})

	h := newSCIMDiscoveryHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	rr := httptest.NewRecorder()

	h.HandleServiceProviderConfigGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, testBaseURL, capturedURL)
}

// TestHandleServiceProviderConfigGetRequest_ResponseContainsCorrectSchema tests Handle Service Provider
// Config Get Request for Response Contains Correct Schema.
func TestHandleServiceProviderConfigGetRequest_ResponseContainsCorrectSchema(t *testing.T) {
	mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
	mockSvc.On("GetServiceProviderConfig", mock.Anything, testBaseURL).
		Return(SCIMServiceProviderConfig{
			Schemas: []string{SCIMServiceProviderConfigSchemaURN},
		})

	h := newSCIMDiscoveryHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	rr := httptest.NewRecorder()

	h.HandleServiceProviderConfigGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var got SCIMServiceProviderConfig
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Contains(t, got.Schemas, SCIMServiceProviderConfigSchemaURN)
}

// TestHandleUnsupportedRequest_Returns501 tests Handle Unsupported Request for Returns 501.
func TestHandleUnsupportedRequest_Returns501(t *testing.T) {
	h := newSCIMDiscoveryHandler(NewSCIMDiscoveryServiceInterfaceMock(t), testBaseURL)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/SomeUnimplementedEndpoint", nil)
	rr := httptest.NewRecorder()

	h.handleUnsupportedRequest(rr, req)

	require.Equal(t, http.StatusNotImplemented, rr.Code)
	require.Equal(t, constants.SCIMContentType, rr.Header().Get("Content-Type"))

	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, []string{SCIMErrorSchemaURN}, errResp.Schemas)
	require.Equal(t, "501", errResp.Status)
	require.Equal(t, "notImplemented", errResp.ScimType)
}

// TestHandleSCIMError_ErrorMapping tests Handle SCIM Error for Error Mapping.
func TestHandleSCIMError_ErrorMapping(t *testing.T) {
	tests := []struct {
		name           string
		svcErr         *tidcommon.ServiceError
		wantHTTPStatus int
		wantScimType   string
	}{
		{
			name:           "UnsupportedOperation_Returns501_NotImplemented",
			svcErr:         &ErrorUnsupportedOperation,
			wantHTTPStatus: http.StatusNotImplemented,
			wantScimType:   "notImplemented",
		},
		{
			name:           "InvalidRequestBody_Returns400_InvalidSyntax",
			svcErr:         &ErrorInvalidRequestBody,
			wantHTTPStatus: http.StatusBadRequest,
			wantScimType:   "invalidSyntax",
		},
		{
			name:           "MissingSchemas_Returns400_InvalidValue",
			svcErr:         &ErrorMissingSchemas,
			wantHTTPStatus: http.StatusBadRequest,
			wantScimType:   "invalidValue",
		},
		{
			name:           "UserNotFound_Returns404_NoScimType",
			svcErr:         &ErrorUserNotFound,
			wantHTTPStatus: http.StatusNotFound,
			wantScimType:   "",
		},
		{
			name:           "SchemaNotFound_Returns404_NoScimType",
			svcErr:         &ErrorSchemaNotFound,
			wantHTTPStatus: http.StatusNotFound,
			wantScimType:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/scim/v2/test", nil)
			rr := httptest.NewRecorder()

			handleSCIMError(rr, req, tc.svcErr)

			require.Equal(t, tc.wantHTTPStatus, rr.Code)

			var errResp SCIMErrorResponse
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
			require.Equal(t, []string{SCIMErrorSchemaURN}, errResp.Schemas)
			require.Equal(t, tc.wantScimType, errResp.ScimType)
			require.NotContains(t, errResp.Detail, tc.svcErr.Code)
		})
	}
}

// TestHandleSchemaListRequest_Success tests Handle Schema List Request for Success.
func TestHandleSchemaListRequest_Success(t *testing.T) {
	expectedResp := SCIMSchemaListResponse{
		Schemas:      []string{SCIMListResponseSchemaURN},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: 1,
		Resources:    []SCIMSchema{{ID: SCIMCoreUserSchemaURN, Name: "User"}},
	}

	mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
	mockSvc.On("ListSchemas", mock.Anything, testBaseURL).
		Return(expectedResp, (*tidcommon.ServiceError)(nil))

	h := newSCIMDiscoveryHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas", nil)
	rr := httptest.NewRecorder()

	h.HandleSchemaListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, constants.SCIMContentType, rr.Header().Get("Content-Type"))

	var got SCIMSchemaListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, []string{SCIMListResponseSchemaURN}, got.Schemas)
	require.Equal(t, 1, got.TotalResults)
}

// TestHandleSchemaListRequest_ErrorCases tests Handle Schema List Request for Error Cases.
func TestHandleSchemaListRequest_ErrorCases(t *testing.T) {
	t.Run("ServiceError_Returns404", func(t *testing.T) {
		mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
		mockSvc.On("ListSchemas", mock.Anything, testBaseURL).
			Return(SCIMSchemaListResponse{}, &ErrorSchemaNotFound)

		h := newSCIMDiscoveryHandler(mockSvc, testBaseURL)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas", nil)
		rr := httptest.NewRecorder()

		h.HandleSchemaListRequest(rr, req)

		require.Equal(t, http.StatusNotFound, rr.Code)
	})
}

// TestHandleSchemaGetRequest_Success tests Handle Schema Get Request for Success.
func TestHandleSchemaGetRequest_Success(t *testing.T) {
	schemaURN := SCIMCoreUserSchemaURN
	expectedSchema := &SCIMSchema{
		Schemas: []string{SCIMSchemaSchemaURN},
		ID:      schemaURN,
		Name:    "User",
	}

	mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
	mockSvc.On("GetSchema", mock.Anything, schemaURN, testBaseURL).
		Return(expectedSchema, (*tidcommon.ServiceError)(nil))

	h := newSCIMDiscoveryHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas/"+schemaURN, nil)
	req.SetPathValue("id", schemaURN)
	rr := httptest.NewRecorder()

	h.HandleSchemaGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, constants.SCIMContentType, rr.Header().Get("Content-Type"))

	var got SCIMSchema
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, schemaURN, got.ID)
}

// TestHandleSchemaGetRequest_ErrorCases tests Handle Schema Get Request for Error Cases.
func TestHandleSchemaGetRequest_ErrorCases(t *testing.T) {
	t.Run("NotFound_UnknownURN", func(t *testing.T) {
		mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
		mockSvc.On("GetSchema", mock.Anything, "urn:unknown", testBaseURL).
			Return((*SCIMSchema)(nil), &ErrorSchemaNotFound)

		h := newSCIMDiscoveryHandler(mockSvc, testBaseURL)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas/urn:unknown", nil)
		req.SetPathValue("id", "urn:unknown")
		rr := httptest.NewRecorder()

		h.HandleSchemaGetRequest(rr, req)

		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("MissingID_NoServiceCall", func(t *testing.T) {
		mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)

		h := newSCIMDiscoveryHandler(mockSvc, testBaseURL)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas/", nil)
		rr := httptest.NewRecorder()

		h.HandleSchemaGetRequest(rr, req)

		require.Equal(t, http.StatusNotFound, rr.Code)
	})
}

// TestHandleSCIMError_ServerErrorType tests Handle SCIM Error for Server Error Type.
func TestHandleSCIMError_ServerErrorType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas", nil)
	rr := httptest.NewRecorder()

	svcErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		ErrorDescription: tidcommon.I18nMessage{
			DefaultValue: "something went wrong internally",
		},
	}
	handleSCIMError(rr, req, svcErr)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "500", errResp.Status)
	require.Equal(t, "something went wrong internally", errResp.Detail)
}
