/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package scim

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const testBaseURL = "https://thunder.example.com"

// TestHandleServiceProviderConfigGetRequest_Success verifies that the handler
// returns HTTP 200 with application/scim+json content type and a well-formed
// ServiceProviderConfig body.
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
			MaxResults: 200,
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
			Created:      scimServiceProviderConfigCreated,
			LastModified: scimServiceProviderConfigCreated,
		},
	}

	mockSvc := NewSCIMServiceInterfaceMock(t)
	mockSvc.On("GetServiceProviderConfig", mock.Anything, testBaseURL).
		Return(expectedConfig)

	h := newSCIMHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	rr := httptest.NewRecorder()

	h.HandleServiceProviderConfigGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, scimContentType, rr.Header().Get("Content-Type"))

	var got SCIMServiceProviderConfig
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, expectedConfig, got)
}

// TestHandleServiceProviderConfigGetRequest_PassesBaseURL verifies that the
// handler forwards the configured baseURL to the service layer unchanged.
func TestHandleServiceProviderConfigGetRequest_PassesBaseURL(t *testing.T) {
	var capturedURL string

	mockSvc := NewSCIMServiceInterfaceMock(t)
	mockSvc.On("GetServiceProviderConfig", mock.Anything, testBaseURL).
		Return(SCIMServiceProviderConfig{
			Schemas: []string{SCIMServiceProviderConfigSchemaURN},
			Meta:    SCIMMeta{Location: testBaseURL + "/scim/v2/ServiceProviderConfig"},
		}).
		Run(func(args mock.Arguments) {
			capturedURL = args.String(1)
		})

	h := newSCIMHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	rr := httptest.NewRecorder()

	h.HandleServiceProviderConfigGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, testBaseURL, capturedURL)
}

// TestHandleServiceProviderConfigGetRequest_ResponseContainsCorrectSchema verifies
// the schemas array contains the ServiceProviderConfig URN per RFC 7643 §5.
func TestHandleServiceProviderConfigGetRequest_ResponseContainsCorrectSchema(t *testing.T) {
	mockSvc := NewSCIMServiceInterfaceMock(t)
	mockSvc.On("GetServiceProviderConfig", mock.Anything, testBaseURL).
		Return(SCIMServiceProviderConfig{
			Schemas: []string{SCIMServiceProviderConfigSchemaURN},
		})

	h := newSCIMHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	rr := httptest.NewRecorder()

	h.HandleServiceProviderConfigGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var got SCIMServiceProviderConfig
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Contains(t, got.Schemas, SCIMServiceProviderConfigSchemaURN)
}

// TestHandleUnsupportedRequest_Returns501 verifies that HandleUnsupportedRequest
// responds with HTTP 501 and the SCIM notImplemented scimType per RFC 7644 §3.12.
func TestHandleUnsupportedRequest_Returns501(t *testing.T) {
	mockSvc := NewSCIMServiceInterfaceMock(t)
	h := newSCIMHandler(mockSvc, testBaseURL)

	req := httptest.NewRequest(http.MethodPost, "/scim/v2/SomeUnimplementedEndpoint", nil)
	rr := httptest.NewRecorder()

	h.HandleUnsupportedRequest(rr, req)

	require.Equal(t, http.StatusNotImplemented, rr.Code)
	require.Equal(t, scimContentType, rr.Header().Get("Content-Type"))

	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, []string{SCIMErrorSchemaURN}, errResp.Schemas)
	require.Equal(t, "501", errResp.Status)
	require.Equal(t, "notImplemented", errResp.ScimType)
}

// TestHandleSCIMError_ErrorMapping verifies that handleSCIMError maps internal
// ThunderID service error codes to the correct SCIM HTTP status and scimType
// per RFC 7644 §3.12. Internal codes must never appear in the response.
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
			// Internal error codes must never be sent to the client.
			require.NotContains(t, errResp.Detail, tc.svcErr.Code)
		})
	}
}

// TestHandleSchemaListRequest_Success verifies the handler returns HTTP 200
// with a well-formed ListResponse body for GET /scim/v2/Schemas.
func TestHandleSchemaListRequest_Success(t *testing.T) {
	expectedResp := SCIMSchemaListResponse{
		Schemas:      []string{SCIMListResponseSchemaURN},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: 1,
		Resources: []SCIMSchema{
			{
				ID:   SCIMCoreUserSchemaURN,
				Name: "User",
			},
		},
	}

	mockSvc := NewSCIMServiceInterfaceMock(t)
	mockSvc.On("ListSchemas", mock.Anything, testBaseURL).
		Return(expectedResp, (*tidcommon.ServiceError)(nil))

	h := newSCIMHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas", nil)
	rr := httptest.NewRecorder()

	h.HandleSchemaListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, scimContentType, rr.Header().Get("Content-Type"))

	var got SCIMSchemaListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, []string{SCIMListResponseSchemaURN}, got.Schemas)
	require.Equal(t, 1, got.TotalResults)
}

// TestHandleSchemaListRequest_ServiceError verifies that a service error
// is correctly mapped to a SCIM error response.
func TestHandleSchemaListRequest_ServiceError(t *testing.T) {
	mockSvc := NewSCIMServiceInterfaceMock(t)
	mockSvc.On("ListSchemas", mock.Anything, testBaseURL).
		Return(SCIMSchemaListResponse{}, &ErrorSchemaNotFound)

	h := newSCIMHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas", nil)
	rr := httptest.NewRecorder()

	h.HandleSchemaListRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleSchemaGetRequest_Success verifies the handler returns HTTP 200
// with the correct schema body for GET /scim/v2/Schemas/{id}.
func TestHandleSchemaGetRequest_Success(t *testing.T) {
	schemaURN := SCIMCoreUserSchemaURN
	expectedSchema := &SCIMSchema{
		Schemas: []string{SCIMSchemaSchemaURN},
		ID:      schemaURN,
		Name:    "User",
	}

	mockSvc := NewSCIMServiceInterfaceMock(t)
	mockSvc.On("GetSchema", mock.Anything, schemaURN, testBaseURL).
		Return(expectedSchema, (*tidcommon.ServiceError)(nil))

	h := newSCIMHandler(mockSvc, testBaseURL)

	// Use SetPathValue to simulate Go 1.22 router path parameter injection.
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas/"+schemaURN, nil)
	req.SetPathValue("id", schemaURN)
	rr := httptest.NewRecorder()

	h.HandleSchemaGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, scimContentType, rr.Header().Get("Content-Type"))

	var got SCIMSchema
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, schemaURN, got.ID)
}

// TestHandleSchemaGetRequest_NotFound verifies that an unknown URN returns 404.
func TestHandleSchemaGetRequest_NotFound(t *testing.T) {
	mockSvc := NewSCIMServiceInterfaceMock(t)
	mockSvc.On("GetSchema", mock.Anything, "urn:unknown", testBaseURL).
		Return((*SCIMSchema)(nil), &ErrorSchemaNotFound)

	h := newSCIMHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas/urn:unknown", nil)
	req.SetPathValue("id", "urn:unknown")
	rr := httptest.NewRecorder()

	h.HandleSchemaGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleSchemaGetRequest_MissingID verifies that a missing path value returns 404
// without calling the service layer.
func TestHandleSchemaGetRequest_MissingID(t *testing.T) {
	mockSvc := NewSCIMServiceInterfaceMock(t)
	// No mock.On — service must NOT be called when id is empty.

	h := newSCIMHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas/", nil)
	// Intentionally do NOT set path value — simulates empty {id}.
	rr := httptest.NewRecorder()

	h.HandleSchemaGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}
