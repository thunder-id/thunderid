// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// HandlerTestSuite groups the tests in handler_test.go.
type HandlerTestSuite struct {
	suite.Suite
}

// TestHandlerTestSuite runs HandlerTestSuite.
func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

const testBaseURL = "https://thunderid.example.com"

// TestHandleServiceProviderConfigGetRequest_Success tests Handle Service Provider Config Get Request for Success.
func (suite *HandlerTestSuite) TestHandleServiceProviderConfigGetRequest_Success() {
	t := suite.T()
	expectedConfig := SCIMServiceProviderConfig{
		Schemas: []string{scimServiceProviderConfigSchemaURN},
		Patch:   scimSupportedFeature{Supported: true},
		Bulk: scimBulkConfig{
			Supported:      false,
			MaxOperations:  0,
			MaxPayloadSize: 0,
		},
		Filter: scimFilterConfig{
			Supported:  true,
			MaxResults: constants.MaxPageSize,
		},
		ChangePassword: scimSupportedFeature{Supported: false},
		Sort:           scimSupportedFeature{Supported: false},
		ETag:           scimSupportedFeature{Supported: false},
		AuthenticationSchemes: []scimAuthenticationScheme{
			{
				Type:        "oauthbearertoken",
				Name:        "OAuth Bearer Token",
				Description: "Authentication using an OAuth 2.0 Bearer Token",
			},
		},
		Meta: scim.SCIMMeta{
			ResourceType: "ServiceProviderConfig",
			Location:     testBaseURL + "/scim/v2/ServiceProviderConfig",
			Created:      testServerStartTime,
			LastModified: testServerStartTime,
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
func (suite *HandlerTestSuite) TestHandleServiceProviderConfigGetRequest_PassesBaseURL() {
	t := suite.T()
	var capturedURL string

	mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
	mockSvc.On("GetServiceProviderConfig", mock.Anything, testBaseURL).
		Return(SCIMServiceProviderConfig{
			Schemas: []string{scimServiceProviderConfigSchemaURN},
			Meta:    scim.SCIMMeta{Location: testBaseURL + "/scim/v2/ServiceProviderConfig"},
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
func (suite *HandlerTestSuite) TestHandleServiceProviderConfigGetRequest_ResponseContainsCorrectSchema() {
	t := suite.T()
	mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
	mockSvc.On("GetServiceProviderConfig", mock.Anything, testBaseURL).
		Return(SCIMServiceProviderConfig{
			Schemas: []string{scimServiceProviderConfigSchemaURN},
		})

	h := newSCIMDiscoveryHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/ServiceProviderConfig", nil)
	rr := httptest.NewRecorder()

	h.HandleServiceProviderConfigGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var got SCIMServiceProviderConfig
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Contains(t, got.Schemas, scimServiceProviderConfigSchemaURN)
}

// TestHandleUnsupportedRequest_Returns501 tests Handle Unsupported Request for Returns 501.
func (suite *HandlerTestSuite) TestHandleUnsupportedRequest_Returns501() {
	t := suite.T()
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/SomeUnimplementedEndpoint", nil)
	rr := httptest.NewRecorder()

	scim.HandleUnsupportedRequest(rr, req, *log.GetLogger())

	require.Equal(t, http.StatusNotImplemented, rr.Code)
	require.Equal(t, constants.SCIMContentType, rr.Header().Get("Content-Type"))

	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, []string{scim.SCIMErrorSchemaURN}, errResp.Schemas)
	require.Equal(t, "501", errResp.Status)
	require.Empty(t, errResp.ScimType)
}

// TestHandleSCIMError_ErrorMapping tests Handle SCIM Error for Error Mapping.
func (suite *HandlerTestSuite) TestHandleSCIMError_ErrorMapping() {
	t := suite.T()
	tests := []struct {
		name           string
		svcErr         *tidcommon.ServiceError
		wantHTTPStatus int
		wantScimType   scim.ScimErrorType
	}{
		{
			name:           "UnsupportedOperation_Returns501_NoScimType",
			svcErr:         &scim.ErrorUnsupportedOperation,
			wantHTTPStatus: http.StatusNotImplemented,
			wantScimType:   "",
		},
		{
			name:           "InvalidRequestBody_Returns400_InvalidSyntax",
			svcErr:         &scim.ErrorInvalidRequestBody,
			wantHTTPStatus: http.StatusBadRequest,
			wantScimType:   scim.ScimErrorTypeInvalidSyntax,
		},
		{
			name:           "MissingSchemas_Returns400_InvalidValue",
			svcErr:         &scim.ErrorMissingSchemas,
			wantHTTPStatus: http.StatusBadRequest,
			wantScimType:   scim.ScimErrorTypeInvalidValue,
		},
		{
			name:           "UserNotFound_Returns404_NoScimType",
			svcErr:         &scim.ErrorUserNotFound,
			wantHTTPStatus: http.StatusNotFound,
			wantScimType:   "",
		},
		{
			name:           "SchemaNotFound_Returns404_NoScimType",
			svcErr:         &scim.ErrorSchemaNotFound,
			wantHTTPStatus: http.StatusNotFound,
			wantScimType:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/scim/v2/test", nil)
			rr := httptest.NewRecorder()

			scim.HandleSCIMError(rr, req, tc.svcErr, *log.GetLogger())

			require.Equal(t, tc.wantHTTPStatus, rr.Code)

			var errResp scim.SCIMErrorResponse
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
			require.Equal(t, []string{scim.SCIMErrorSchemaURN}, errResp.Schemas)
			require.Equal(t, tc.wantScimType, errResp.ScimType)
			require.NotContains(t, errResp.Detail, tc.svcErr.Code)
		})
	}
}

// TestHandleSchemaListRequest_Success tests Handle Schema List Request for Success.
func (suite *HandlerTestSuite) TestHandleSchemaListRequest_Success() {
	t := suite.T()
	expectedResp := SCIMSchemaListResponse{
		Schemas:      []string{scim.SCIMListResponseSchemaURN},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: 1,
		Resources:    []SCIMSchema{{ID: scim.SCIMCoreUserSchemaURN, Name: "User"}},
	}

	mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
	mockSvc.On("ListSchemas", mock.Anything, testBaseURL, 1, constants.DefaultPageSize).
		Return(expectedResp, (*tidcommon.ServiceError)(nil))

	h := newSCIMDiscoveryHandler(mockSvc, testBaseURL)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas", nil)
	rr := httptest.NewRecorder()

	h.HandleSchemaListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, constants.SCIMContentType, rr.Header().Get("Content-Type"))

	var got SCIMSchemaListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, []string{scim.SCIMListResponseSchemaURN}, got.Schemas)
	require.Equal(t, 1, got.TotalResults)
}

// TestHandleSchemaListRequest_ErrorCases tests Handle Schema List Request for Error Cases.
func (suite *HandlerTestSuite) TestHandleSchemaListRequest_ErrorCases() {
	t := suite.T()
	t.Run("ServiceError_Returns404", func(t *testing.T) {
		mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
		mockSvc.On("ListSchemas", mock.Anything, testBaseURL, 1, constants.DefaultPageSize).
			Return(SCIMSchemaListResponse{}, &scim.ErrorSchemaNotFound)

		h := newSCIMDiscoveryHandler(mockSvc, testBaseURL)
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas", nil)
		rr := httptest.NewRecorder()

		h.HandleSchemaListRequest(rr, req)

		require.Equal(t, http.StatusNotFound, rr.Code)
	})
}

// TestHandleSchemaGetRequest_Success tests Handle Schema Get Request for Success.
func (suite *HandlerTestSuite) TestHandleSchemaGetRequest_Success() {
	t := suite.T()
	schemaURN := scim.SCIMCoreUserSchemaURN
	expectedSchema := &SCIMSchema{
		Schemas: []string{scimSchemaSchemaURN},
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
func (suite *HandlerTestSuite) TestHandleSchemaGetRequest_ErrorCases() {
	t := suite.T()
	t.Run("NotFound_UnknownURN", func(t *testing.T) {
		mockSvc := NewSCIMDiscoveryServiceInterfaceMock(t)
		mockSvc.On("GetSchema", mock.Anything, "urn:unknown", testBaseURL).
			Return((*SCIMSchema)(nil), &scim.ErrorSchemaNotFound)

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
func (suite *HandlerTestSuite) TestHandleSCIMError_ServerErrorType() {
	t := suite.T()
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Schemas", nil)
	rr := httptest.NewRecorder()

	svcErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		ErrorDescription: tidcommon.I18nMessage{
			DefaultValue: "something went wrong internally",
		},
	}
	scim.HandleSCIMError(rr, req, svcErr, *log.GetLogger())

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, "500", errResp.Status)
	require.Equal(t, "something went wrong internally", errResp.Detail)
}
