// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/security"
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

// testURNPrefix is the custom schema URN prefix used by the tests.
const testURNPrefix = scimconfig.DefaultSchemaURNPrefix

// testHandlerConfig is the handler configuration used by the tests.
var testHandlerConfig = scimconfig.SCIMConfig{PublicURL: testBaseURL, SchemaURNPrefix: testURNPrefix}

// testSCIMConfig carries the custom schema URN prefix the URN-building code requires.
var testSCIMConfig = scimconfig.SCIMConfig{SchemaURNPrefix: testURNPrefix}

const scimTestPayloadBody = `{
		"schemas": ["urn:thunderid:params:scim:schemas:person:2.0:User"],
		"urn:thunderid:params:scim:schemas:person:2.0:User": {
			"given_name": "Test"
		}
	}`

// TestHandleUsersGetRequest_Success tests Handle Users Get Request for Success.
func (suite *HandlerTestSuite) TestHandleUsersGetRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	expectedUser := &SCIMUser{
		Schemas: []string{scim.SCIMCoreUserSchemaURN},
		ID:      "user-123",
		Meta: scim.SCIMMeta{
			ResourceType: "User",
			Location:     testBaseURL + "/scim/v2/Users/user-123",
		},
	}
	mockSvc.On("GetUser", mock.Anything, "user-123", testBaseURL).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/user-123", nil)
	req.SetPathValue("id", "user-123")
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, constants.SCIMContentType, rr.Header().Get("Content-Type"))

	var got SCIMUser
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, expectedUser.ID, got.ID)
}

// TestHandleUsersGetRequest_NotFound tests Handle Users Get Request for Not Found.
func (suite *HandlerTestSuite) TestHandleUsersGetRequest_NotFound() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("GetUser", mock.Anything, "unknown", testBaseURL).Return((*SCIMUser)(nil), &scim.ErrorUserNotFound)

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/unknown", nil)
	req.SetPathValue("id", "unknown")
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleUsersDeleteRequest_Success tests Handle Users Delete Request for Success.
func (suite *HandlerTestSuite) TestHandleUsersDeleteRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("DeleteUser", mock.Anything, "user-123").Return((*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/user-123", nil)
	req.SetPathValue("id", "user-123")
	rr := httptest.NewRecorder()

	h.HandleUsersDeleteRequest(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
}

// TestHandleUsersCreateRequest_Success tests Handle Users Create Request for Success.
func (suite *HandlerTestSuite) TestHandleUsersCreateRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))

	expectedUser := &SCIMUser{
		Schemas: []string{scim.SCIMCoreUserSchemaURN, "urn:thunderid:params:scim:schemas:person:2.0:User"},
		ID:      "user-123",
		Meta: scim.SCIMMeta{
			ResourceType: "User",
			Location:     testBaseURL + "/scim/v2/Users/user-123",
		},
	}
	mockSvc.On(
		"CreateUser", mock.Anything, mock.AnythingOfType("*users.SCIMUserPayload"), testBaseURL,
	).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", bytes.NewBufferString(scimTestPayloadBody))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	require.Equal(t, expectedUser.Meta.Location, rr.Header().Get("Location"))
}

// TestHandleUsersListRequest_Success tests Handle Users List Request for Success.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	expectedResp := SCIMUserListResponse{
		Schemas:      []string{scim.SCIMListResponseSchemaURN},
		TotalResults: 0,
		StartIndex:   1,
		ItemsPerPage: constants.DefaultPageSize,
		Resources:    []SCIMUser{},
	}
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, mock.Anything,
		testBaseURL).Return(expectedResp, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersReplaceRequest_Success tests Handle Users Replace Request for Success.
func (suite *HandlerTestSuite) TestHandleUsersReplaceRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))

	expectedUser := &SCIMUser{
		Schemas: []string{scim.SCIMCoreUserSchemaURN, "urn:thunderid:params:scim:schemas:person:2.0:User"},
		ID:      "user-123",
		Meta: scim.SCIMMeta{
			ResourceType: "User",
			Location:     testBaseURL + "/scim/v2/Users/user-123",
		},
	}
	mockSvc.On(
		"ReplaceUser", mock.Anything, "user-123", mock.AnythingOfType("*users.SCIMUserPayload"), testBaseURL, false,
	).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123", bytes.NewBufferString(scimTestPayloadBody))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
}

// --- GET /scim/v2/Me (RFC 7644 §3.11) ---

// TestHandleMeGetRequest_Success tests Handle Me Get Request for Success.
func (suite *HandlerTestSuite) TestHandleMeGetRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	expectedUser := &SCIMUser{
		Schemas: []string{scim.SCIMCoreUserSchemaURN},
		ID:      "user-123",
		Meta: scim.SCIMMeta{
			ResourceType: "User",
			Location:     testBaseURL + "/scim/v2/Users/user-123",
		},
	}
	mockSvc.On("GetUser", mock.Anything, "user-123", testBaseURL).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Me", nil)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, expectedUser.Meta.Location, rr.Header().Get("Location"))

	var got SCIMUser
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, expectedUser.ID, got.ID)
}

// TestHandleMeGetRequest_NoSubject_Returns401 tests Handle Me Get Request for No Subject Returns 401.
func (suite *HandlerTestSuite) TestHandleMeGetRequest_NoSubject_Returns401() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Me", nil)
	rr := httptest.NewRecorder()

	h.HandleMeGetRequest(rr, req)

	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestHandleMeGetRequest_AppliesAttributeProjection tests Handle Me Get Request for Applies Attribute Projection.
func (suite *HandlerTestSuite) TestHandleMeGetRequest_AppliesAttributeProjection() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	expectedUser := &SCIMUser{
		ID:      "user-123",
		Schemas: []string{scim.SCIMCoreUserSchemaURN},
		Meta:    scim.SCIMMeta{ResourceType: "User"},
		CoreAttrs: map[string]json.RawMessage{
			"userName": json.RawMessage(`"alice"`),
			"active":   json.RawMessage(`true`),
		},
	}
	mockSvc.On("GetUser", mock.Anything, "user-123", testBaseURL).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Me?attributes=userName", nil)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, "alice", got["userName"])
	require.NotContains(t, got, "active")
}

// TestHandleMeGetRequest_ConflictingAttributesParams_Returns400 tests Handle Me Get Request for Conflicting
// Attributes Params Returns 400.
func (suite *HandlerTestSuite) TestHandleMeGetRequest_ConflictingAttributesParams_Returns400() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, []string{"userName"}, []string{"active"}).
		Return(&scim.ErrorConflictingAttributesParams)
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Me?attributes=userName&excludedAttributes=active", nil)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeGetRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleMeGetRequest_ServiceError_Returns404 tests Handle Me Get Request for Service Error Returns 404.
func (suite *HandlerTestSuite) TestHandleMeGetRequest_ServiceError_Returns404() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("GetUser", mock.Anything, "user-123", testBaseURL).
		Return((*SCIMUser)(nil), &scim.ErrorUserNotFound)

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Me", nil)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// --- PUT /scim/v2/Me (RFC 7644 §3.11) ---

// TestHandleMeReplaceRequest_Success tests Handle Me Replace Request for Success.
func (suite *HandlerTestSuite) TestHandleMeReplaceRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	expectedUser := &SCIMUser{
		Schemas: []string{scim.SCIMCoreUserSchemaURN, "urn:thunderid:params:scim:schemas:person:2.0:User"},
		ID:      "user-123",
		Meta: scim.SCIMMeta{
			ResourceType: "User",
			Location:     testBaseURL + "/scim/v2/Users/user-123",
		},
	}
	mockSvc.On(
		"ReplaceUser", mock.Anything, "user-123", mock.AnythingOfType("*users.SCIMUserPayload"), testBaseURL, true,
	).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", bytes.NewBufferString(scimTestPayloadBody))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeReplaceRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, expectedUser.Meta.Location, rr.Header().Get("Location"))
}

// TestHandleMeReplaceRequest_NoSubject_Returns401 tests Handle Me Replace Request for No Subject Returns 401.
func (suite *HandlerTestSuite) TestHandleMeReplaceRequest_NoSubject_Returns401() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleMeReplaceRequest(rr, req)

	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestHandleMeReplaceRequest_WrongContentType_Returns400 tests Handle Me Replace Request for Wrong Content
// Type Returns 400.
func (suite *HandlerTestSuite) TestHandleMeReplaceRequest_WrongContentType_Returns400() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleMeReplaceRequest_EmptyBody_Returns400 tests Handle Me Replace Request for Empty Body Returns 400.
func (suite *HandlerTestSuite) TestHandleMeReplaceRequest_EmptyBody_Returns400() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", http.NoBody)
	req.Header.Set("Content-Type", constants.SCIMContentType)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleMeReplaceRequest_InvalidJSON_Returns400 tests Handle Me Replace Request for Invalid JSON Returns 400.
func (suite *HandlerTestSuite) TestHandleMeReplaceRequest_InvalidJSON_Returns400() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleMeReplaceRequest_ServiceError_Returns404 tests Handle Me Replace Request for Service Error Returns 404.
func (suite *HandlerTestSuite) TestHandleMeReplaceRequest_ServiceError_Returns404() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("ReplaceUser", mock.Anything, "user-123",
		mock.AnythingOfType("*users.SCIMUserPayload"), testBaseURL, true).
		Return((*SCIMUser)(nil), &scim.ErrorUserNotFound)

	body := `{"schemas":["urn:thunderid:params:scim:schemas:person:2.0:User"],` +
		`"urn:thunderid:params:scim:schemas:person:2.0:User":{}}`
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
	req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
	rr := httptest.NewRecorder()

	h.HandleMeReplaceRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// --- GET /scim/v2/Users/{id} error paths ---

// TestHandleUsersGetRequest_MissingID_Returns404 tests Handle Users Get Request for Missing ID Returns 404.
func (suite *HandlerTestSuite) TestHandleUsersGetRequest_MissingID_Returns404() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleUsersGetRequest_ServerError_Returns500 tests Handle Users Get Request for Server Error Returns 500.
func (suite *HandlerTestSuite) TestHandleUsersGetRequest_ServerError_Returns500() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("GetUser", mock.Anything, "user-123", testBaseURL).
		Return((*SCIMUser)(nil), &tidcommon.InternalServerError)

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/user-123", nil)
	req.SetPathValue("id", "user-123")
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Empty(t, errResp.ScimType)
}

// --- DELETE /scim/v2/Users/{id} error paths ---

// TestHandleUsersDeleteRequest_MissingID_Returns404 tests Handle Users Delete Request for Missing ID Returns 404.
func (suite *HandlerTestSuite) TestHandleUsersDeleteRequest_MissingID_Returns404() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersDeleteRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleUsersDeleteRequest_NotFound_Returns404 tests Handle Users Delete Request for Not Found Returns 404.
func (suite *HandlerTestSuite) TestHandleUsersDeleteRequest_NotFound_Returns404() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("DeleteUser", mock.Anything, "no-such").Return(&scim.ErrorUserNotFound)

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/no-such", nil)
	req.SetPathValue("id", "no-such")
	rr := httptest.NewRecorder()

	h.HandleUsersDeleteRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleUsersDeleteRequest_MutabilityViolation_Returns400 tests Handle Users Delete Request for
// Mutability Violation Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersDeleteRequest_MutabilityViolation_Returns400() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("DeleteUser", mock.Anything, "readonly").Return(&scim.ErrorMutabilityViolation)

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodDelete, "/scim/v2/Users/readonly", nil)
	req.SetPathValue("id", "readonly")
	rr := httptest.NewRecorder()

	h.HandleUsersDeleteRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scim.ScimErrorTypeMutability, errResp.ScimType)
}

// --- POST /scim/v2/Users error paths ---

// TestHandleUsersCreateRequest_WrongContentType_Returns400 tests Handle Users Create Request for Wrong
// Content Type Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersCreateRequest_WrongContentType_Returns400() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users",
		bytes.NewBufferString(`{"schemas":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scim.ScimErrorTypeInvalidSyntax, errResp.ScimType)
}

// TestHandleUsersCreateRequest_EmptyBody_Returns400 tests Handle Users Create Request for Empty Body Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersCreateRequest_EmptyBody_Returns400() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", http.NoBody)
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersCreateRequest_InvalidJSON_Returns400 tests Handle Users Create Request for Invalid JSON Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersCreateRequest_InvalidJSON_Returns400() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users",
		bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersCreateRequest_ServiceErrors tests Handle Users Create Request for service-layer
// errors surfaced as SCIM error responses (uniqueness conflict, schema validation failure).
func (suite *HandlerTestSuite) TestHandleUsersCreateRequest_ServiceErrors() {
	t := suite.T()
	tests := []struct {
		name         string
		svcErr       tidcommon.ServiceError
		body         string
		wantStatus   int
		wantScimType scim.ScimErrorType
	}{
		{
			name:   "UniquenessConflict_Returns409",
			svcErr: scim.ErrorUniquenessConflict,
			body: `{"schemas":["urn:thunderid:params:scim:schemas:person:2.0:User"],` +
				`"urn:thunderid:params:scim:schemas:person:2.0:User":{"email":"x@x.com"}}`,
			wantStatus:   http.StatusConflict,
			wantScimType: scim.ScimErrorTypeUniqueness,
		},
		{
			name:   "SchemaValidationFailed_Returns400",
			svcErr: scim.ErrorSchemaValidationFailed,
			body: `{"schemas":["urn:thunderid:params:scim:schemas:person:2.0:User"],` +
				`"urn:thunderid:params:scim:schemas:person:2.0:User":{}}`,
			wantStatus:   http.StatusBadRequest,
			wantScimType: scim.ScimErrorTypeInvalidValue,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSvc := NewSCIMUsersServiceInterfaceMock(t)
			mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
				Return((*tidcommon.ServiceError)(nil))
			mockSvc.On("CreateUser", mock.Anything,
				mock.AnythingOfType("*users.SCIMUserPayload"), testBaseURL).
				Return((*SCIMUser)(nil), &tc.svcErr)

			h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
			req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users",
				bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", constants.SCIMContentType)
			rr := httptest.NewRecorder()

			h.HandleUsersCreateRequest(rr, req)

			require.Equal(t, tc.wantStatus, rr.Code)
			var errResp scim.SCIMErrorResponse
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
			require.Equal(t, tc.wantScimType, errResp.ScimType)
		})
	}
}

// TestHandleUsersCreateRequest_ServerError_Returns500 tests Handle Users Create Request for Server Error Returns 500.
func (suite *HandlerTestSuite) TestHandleUsersCreateRequest_ServerError_Returns500() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("CreateUser", mock.Anything,
		mock.AnythingOfType("*users.SCIMUserPayload"), testBaseURL).
		Return((*SCIMUser)(nil), &tidcommon.InternalServerError)

	body := `{"schemas":["urn:thunderid:params:scim:schemas:person:2.0:User"],` +
		`"urn:thunderid:params:scim:schemas:person:2.0:User":{}}`
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

// --- PUT /scim/v2/Users/{id} error paths ---

// TestHandleUsersReplaceRequest_MissingID_Returns404 tests Handle Users Replace Request for Missing ID Returns 404.
func (suite *HandlerTestSuite) TestHandleUsersReplaceRequest_MissingID_Returns404() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/", http.NoBody)
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleUsersReplaceRequest_WrongContentType_Returns400 tests Handle Users Replace Request for Wrong
// Content Type Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersReplaceRequest_WrongContentType_Returns400() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123",
		bytes.NewBufferString(`{}`))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersReplaceRequest_EmptyBody_Returns400 tests Handle Users Replace Request for Empty Body Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersReplaceRequest_EmptyBody_Returns400() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123", http.NoBody)
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersReplaceRequest_InvalidJSON_Returns400 tests Handle Users Replace Request for Invalid JSON Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersReplaceRequest_InvalidJSON_Returns400() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123",
		bytes.NewBufferString(`not json`))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersReplaceRequest_NotFound_Returns404 tests Handle Users Replace Request for Not Found Returns 404.
func (suite *HandlerTestSuite) TestHandleUsersReplaceRequest_NotFound_Returns404() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("ReplaceUser", mock.Anything, "no-such",
		mock.AnythingOfType("*users.SCIMUserPayload"), testBaseURL, false).
		Return((*SCIMUser)(nil), &scim.ErrorUserNotFound)

	body := `{"schemas":["urn:thunderid:params:scim:schemas:person:2.0:User"],` +
		`"urn:thunderid:params:scim:schemas:person:2.0:User":{}}`
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/no-such",
		bytes.NewBufferString(body))
	req.SetPathValue("id", "no-such")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleUsersReplaceRequest_MutabilityViolation_Returns400 tests Handle Users Replace Request for
// Mutability Violation Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersReplaceRequest_MutabilityViolation_Returns400() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("ReplaceUser", mock.Anything, "readonly",
		mock.AnythingOfType("*users.SCIMUserPayload"), testBaseURL, false).
		Return((*SCIMUser)(nil), &scim.ErrorMutabilityViolation)

	body := `{"schemas":["urn:thunderid:params:scim:schemas:person:2.0:User"],` +
		`"urn:thunderid:params:scim:schemas:person:2.0:User":{}}`
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/readonly",
		bytes.NewBufferString(body))
	req.SetPathValue("id", "readonly")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scim.ScimErrorTypeMutability, errResp.ScimType)
}

// --- GET /scim/v2/Users list error paths ---

// TestHandleUsersListRequest_FilterNotSupported_Returns400 tests Handle Users List Request for Filter Not
// Supported Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_FilterNotSupported_Returns400() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	// "or" expressions are unsupported; only "eq" clauses joined by "and" are allowed.
	req := httptest.NewRequest(http.MethodGet,
		`/scim/v2/Users?filter=userName+eq+"alice"+or+active+eq+true`, nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scim.ScimErrorTypeInvalidFilter, errResp.ScimType)
}

// TestHandleUsersListRequest_FilterOnMultiValueUnsupportedSubAttr_Returns400 tests Handle Users List Request
// for Filter On Multi Value Unsupported Sub Attr Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_FilterOnMultiValueUnsupportedSubAttr_Returns400() {
	t := suite.T()
	// "emails.type"/"emails.primary" have no matching flat ThunderID attribute
	// — ThunderID only stores the value, never a per-entry type or primary
	// flag — so they're rejected explicitly instead of silently matching
	// nothing.
	tests := []string{
		`id+eq+"user-1"`,
		`emails.type+eq+"work"`,
		`emails.primary+eq+true`,
		`active+eq+true`,
		`externalId+eq+"123"`,
		`userType+eq+"employee"`,
	}
	for _, filter := range tests {
		t.Run(filter, func(t *testing.T) {
			h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
			req := httptest.NewRequest(http.MethodGet,
				"/scim/v2/Users?filter="+filter, nil)
			rr := httptest.NewRecorder()

			h.HandleUsersListRequest(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)
			var errResp scim.SCIMErrorResponse
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
			require.Equal(t, scim.ScimErrorTypeInvalidFilter, errResp.ScimType)
		})
	}
}

// TestHandleUsersListRequest_FilterOnHyphenatedAttr_Returns400 tests Handle Users List Request for Filter On
// Hyphenated Attr Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_FilterOnHyphenatedAttr_Returns400() {
	t := suite.T()
	// "-" is valid in an attrPath per RFC 7643 but rejected by the store-layer
	// key charset; must be caught here as invalidFilter, not surface as a 500.
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet,
		"/scim/v2/Users?filter="+neturl.QueryEscape(`custom-attr eq "x"`), nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scim.ScimErrorTypeInvalidFilter, errResp.ScimType)
}

// TestHandleUsersListRequest_AttributesCustomWithoutURN_Returns400 tests Handle Users List Request for
// Attributes Custom Without URN Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_AttributesCustomWithoutURN_Returns400() {
	t := suite.T()
	tests := []struct {
		name       string
		query      string
		attributes []string
		excluded   []string
		badAttr    string
	}{
		{name: "attributes", query: "attributes=department", attributes: []string{"department"}, badAttr: "department"},
		{
			name: "excludedAttributes", query: "excludedAttributes=employee_id",
			excluded: []string{"employee_id"}, badAttr: "employee_id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := NewSCIMUsersServiceInterfaceMock(t)
			mockSvc.On("ValidateAttributePaths", mock.Anything, tt.attributes, tt.excluded).
				Return(scim.NewCustomAttributeRequiresURNError(tt.badAttr))
			h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
			req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?"+tt.query, nil)
			rr := httptest.NewRecorder()

			h.HandleUsersListRequest(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)
			var errResp scim.SCIMErrorResponse
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
			require.Equal(t, scim.ScimErrorTypeInvalidPath, errResp.ScimType)
		})
	}
}

// TestHandleUsersListRequest_AttributesCustomWithURN_Accepted tests Handle Users List Request for
// Attributes Custom With URN Accepted.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_AttributesCustomWithURN_Accepted() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything,
		[]string{"urn:thunderid:params:scim:schemas:employee:2.0:User:department"}, []string(nil)).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize,
		map[string]interface{}(nil), testBaseURL).
		Return(SCIMUserListResponse{Schemas: []string{scim.SCIMListResponseSchemaURN}}, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet,
		"/scim/v2/Users?"+neturl.QueryEscape("attributes")+"="+
			neturl.QueryEscape("urn:thunderid:params:scim:schemas:employee:2.0:User:department"), nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersListRequest_FilterTranslatesCoreAttributes tests Handle Users List Request for Filter
// Translates Core Attributes.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_FilterTranslatesCoreAttributes() {
	t := suite.T()
	tests := []struct {
		name           string
		filter         string
		expectedFilter map[string]interface{}
	}{
		{
			name:           "simple string attribute",
			filter:         `userName eq "alice"`,
			expectedFilter: map[string]interface{}{"username": "alice"},
		},
		{
			name:           "sub-attribute of complex object",
			filter:         `name.givenName eq "Alice"`,
			expectedFilter: map[string]interface{}{"given_name": "Alice"},
		},
		{
			name:           "multi-valued complex attribute value",
			filter:         `emails.value eq "alice@example.com"`,
			expectedFilter: map[string]interface{}{"email": "alice@example.com"},
		},
		{
			name:           "address sub-attribute",
			filter:         `addresses.streetAddress eq "Main St"`,
			expectedFilter: map[string]interface{}{"street_address": "Main St"},
		},
		{
			name:           "unmapped attribute with URN passes through unchanged",
			filter:         `urn:thunderid:params:scim:schemas:employee_hier:2.0:User:department eq "Engineering"`,
			expectedFilter: map[string]interface{}{"department": "Engineering"},
		},
		{
			name:           "URN-prefixed attribute with numeric version segment",
			filter:         `urn:thunderid:params:scim:schemas:employee_hier:2.0:User:custom_code eq "code1"`,
			expectedFilter: map[string]interface{}{"custom_code": "code1"},
		},
		{
			name:           "compound AND with two clauses",
			filter:         `userName eq "alice" and title eq "Engineer"`,
			expectedFilter: map[string]interface{}{"username": "alice", "title": "Engineer"},
		},
		{
			name:           "compound AND with case-insensitive keyword",
			filter:         `userName eq "alice" AND title eq "Engineer"`,
			expectedFilter: map[string]interface{}{"username": "alice", "title": "Engineer"},
		},
		{
			name:   "compound AND with three clauses",
			filter: `userName eq "alice" and title eq "Engineer" and name.givenName eq "Alice"`,
			expectedFilter: map[string]interface{}{
				"username": "alice", "title": "Engineer", "given_name": "Alice",
			},
		},
		{
			name:           "quoted value containing literal 'and' text is not split",
			filter:         `title eq "call and response"`,
			expectedFilter: map[string]interface{}{"title": "call and response"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := NewSCIMUsersServiceInterfaceMock(t)
			mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
				Return((*tidcommon.ServiceError)(nil))
			mockSvc.On("ValidateFilterSchemaAttribute", mock.Anything, mock.Anything, mock.Anything).
				Return((*tidcommon.ServiceError)(nil)).Maybe()
			mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, tt.expectedFilter, testBaseURL).
				Return(SCIMUserListResponse{}, (*tidcommon.ServiceError)(nil))

			h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
			req := httptest.NewRequest(http.MethodGet,
				"/scim/v2/Users?filter="+neturl.QueryEscape(tt.filter), nil)
			rr := httptest.NewRecorder()

			h.HandleUsersListRequest(rr, req)

			require.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

// TestHandleUsersListRequest_FilterUnrecognizedSchemaURN_Returns400 tests that a filter whose URN
// prefix fails schema validation is rejected with the service error before any user query.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_FilterUnrecognizedSchemaURN_Returns400() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateFilterSchemaAttribute", mock.Anything, "urn:x:y:2.0:User:", "password").
		Return(scim.NewUnrecognizedSchemaURNError("urn:x:y:2.0:User:password"))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet,
		"/scim/v2/Users?filter="+neturl.QueryEscape(`urn:x:y:2.0:User:password eq "x"`), nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), `"scimType":"invalidPath"`)
}

// TestHandleUsersListRequest_ServiceError_Returns500 tests Handle Users List Request for Service Error Returns 500.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_ServiceError_Returns500() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{}, &tidcommon.InternalServerError)

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

// TestHandleUsersListRequest_CustomPagination tests Handle Users List Request for Custom Pagination.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_CustomPagination() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("ListUsers", mock.Anything, 5, 10, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{
			Schemas:      []string{scim.SCIMListResponseSchemaURN},
			TotalResults: 0,
			StartIndex:   5,
			ItemsPerPage: 10,
			Resources:    []SCIMUser{},
		}, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?startIndex=5&count=10", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersListRequest_CapsMaxCount tests Handle Users List Request for Caps Max Count.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_CapsMaxCount() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("ListUsers", mock.Anything, 1, constants.MaxPageSize, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{}, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?count=100000", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersListRequest_SortNotSupported_Returns400 tests Handle Users List Request for Sort Not
// Supported Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_SortNotSupported_Returns400() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?sortBy=userName", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scim.ScimErrorTypeInvalidValue, errResp.ScimType)
}

// TestHandleUsersSearchRequest_Success tests Handle Users Search Request for Success.
func (suite *HandlerTestSuite) TestHandleUsersSearchRequest_Success() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	expectedResp := SCIMUserListResponse{
		Schemas:      []string{scim.SCIMListResponseSchemaURN},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: constants.DefaultPageSize,
		Resources:    []SCIMUser{{ID: "user-123"}},
	}
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize,
		map[string]interface{}{"username": "alice"}, testBaseURL).
		Return(expectedResp, (*tidcommon.ServiceError)(nil))

	body := `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:SearchRequest"],
		"filter": "userName eq \"alice\""
	}`
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got SCIMUserListResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, 1, got.TotalResults)
}

// TestHandleUsersSearchRequest_NoFilter_DefaultsPagination tests Handle Users Search Request for No Filter
// Defaults Pagination.
func (suite *HandlerTestSuite) TestHandleUsersSearchRequest_NoFilter_DefaultsPagination() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{}, (*tidcommon.ServiceError)(nil))

	body := `{"schemas": ["urn:ietf:params:scim:api:messages:2.0:SearchRequest"]}`
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersSearchRequest_CustomPagination tests Handle Users Search Request for Custom Pagination.
func (suite *HandlerTestSuite) TestHandleUsersSearchRequest_CustomPagination() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("ListUsers", mock.Anything, 5, 10, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{
			Schemas: []string{scim.SCIMListResponseSchemaURN}, StartIndex: 5, ItemsPerPage: 10,
			Resources: []SCIMUser{},
		}, (*tidcommon.ServiceError)(nil))

	body := `{"schemas": ["` + scim.SCIMSearchSchemaURN + `"], "startIndex": 5, "count": 10}`
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersSearchRequest_ExplicitZeroCount tests Handle Users Search Request for Explicit Zero Count.
func (suite *HandlerTestSuite) TestHandleUsersSearchRequest_ExplicitZeroCount() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("ListUsers", mock.Anything, 1, 0, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{
			Schemas: []string{scim.SCIMListResponseSchemaURN}, StartIndex: 1, ItemsPerPage: 0, TotalResults: 5,
			Resources: []SCIMUser{},
		}, (*tidcommon.ServiceError)(nil))

	body := `{"schemas": ["` + scim.SCIMSearchSchemaURN + `"], "count": 0}`
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersSearchRequest_CapsMaxCount tests Handle Users Search Request for Caps Max Count.
func (suite *HandlerTestSuite) TestHandleUsersSearchRequest_CapsMaxCount() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("ListUsers", mock.Anything, 1, constants.MaxPageSize, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{}, (*tidcommon.ServiceError)(nil))

	body := `{"schemas": ["` + scim.SCIMSearchSchemaURN + `"], "count": 100000}`
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleUsersSearchRequest_SortNotSupported_Returns400 tests Handle Users Search Request for Sort Not
// Supported Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersSearchRequest_SortNotSupported_Returns400() {
	t := suite.T()
	h := newSCIMUsersHandler(NewSCIMUsersServiceInterfaceMock(t), testHandlerConfig)

	body := `{"schemas": ["` + scim.SCIMSearchSchemaURN + `"], "sortBy": "userName"}`
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scim.ScimErrorTypeInvalidValue, errResp.ScimType)
}

// TestHandleUsersSearchRequest_InvalidFilter_Returns400 tests Handle Users Search Request for Invalid Filter
// Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersSearchRequest_InvalidFilter_Returns400() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)

	body := `{"schemas": ["` + scim.SCIMSearchSchemaURN + `"], "filter": "userName co \"ali\""}`
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp scim.SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, scim.ScimErrorTypeInvalidFilter, errResp.ScimType)
}

// TestHandleUsersSearchRequest_MalformedJSON_Returns400 tests Handle Users Search Request for Malformed JSON
// Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersSearchRequest_MalformedJSON_Returns400() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)

	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(`{not-json`))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersSearchRequest_EmptyBody_Returns400 tests Handle Users Search Request for Empty Body Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersSearchRequest_EmptyBody_Returns400() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)

	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(``))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersSearchRequest_WrongContentType_Returns400 tests Handle Users Search Request for Wrong
// Content Type Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersSearchRequest_WrongContentType_Returns400() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)

	body := `{"filter": "userName eq \"alice\""}`
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersSearchRequest_ServiceError_Returns500 tests Handle Users Search Request for Service Error Returns 500.
func (suite *HandlerTestSuite) TestHandleUsersSearchRequest_ServiceError_Returns500() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, mock.Anything, testBaseURL).
		Return(SCIMUserListResponse{}, &tidcommon.InternalServerError)

	body := `{"schemas": ["` + scim.SCIMSearchSchemaURN + `"]}`
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

// TestParseCSVQueryParam tests Parse CSV Query Param.
func (suite *HandlerTestSuite) TestParseCSVQueryParam() {
	t := suite.T()
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty string returns nil", "", nil},
		{"single value", "userName", []string{"userName"}},
		{"multiple values", "userName,emails", []string{"userName", "emails"}},
		{"trims whitespace", " userName , emails ", []string{"userName", "emails"}},
		{"drops empty entries", "userName,,emails,", []string{"userName", "emails"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, parseCSVQueryParam(tt.input))
		})
	}
}

// TestHandleUsersListRequest_AttributesProjection tests Handle Users List Request for Attributes Projection.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_AttributesProjection() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	listResp := SCIMUserListResponse{
		Schemas:      []string{scim.SCIMListResponseSchemaURN},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: constants.DefaultPageSize,
		Resources: []SCIMUser{
			{
				ID:      "user-123",
				Schemas: []string{scim.SCIMCoreUserSchemaURN},
				Meta:    scim.SCIMMeta{ResourceType: "User"},
				CoreAttrs: map[string]json.RawMessage{
					"userName": json.RawMessage(`"alice"`),
					"active":   json.RawMessage(`true`),
				},
			},
		},
	}
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, mock.Anything, testBaseURL).
		Return(listResp, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?attributes=userName", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	resources, ok := got["Resources"].([]interface{})
	require.True(t, ok)
	require.Len(t, resources, 1)
	resource, ok := resources[0].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "alice", resource["userName"])
	require.NotContains(t, resource, "active")
}

// TestHandleUsersSearchRequest_ExcludedAttributesProjection tests Handle Users Search Request for Excluded
// Attributes Projection.
func (suite *HandlerTestSuite) TestHandleUsersSearchRequest_ExcludedAttributesProjection() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	listResp := SCIMUserListResponse{
		Schemas:      []string{scim.SCIMListResponseSchemaURN},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: constants.DefaultPageSize,
		Resources: []SCIMUser{
			{
				ID:      "user-123",
				Schemas: []string{scim.SCIMCoreUserSchemaURN},
				Meta:    scim.SCIMMeta{ResourceType: "User"},
				CoreAttrs: map[string]json.RawMessage{
					"userName":    json.RawMessage(`"alice"`),
					"displayName": json.RawMessage(`"Alice"`),
				},
			},
		},
	}
	mockSvc.On("ListUsers", mock.Anything, 1, constants.DefaultPageSize, mock.Anything, testBaseURL).
		Return(listResp, (*tidcommon.ServiceError)(nil))

	body := `{"schemas": ["` + scim.SCIMSearchSchemaURN + `"], "excludedAttributes": ["displayName"]}`
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	resources := got["Resources"].([]interface{})
	resource := resources[0].(map[string]interface{})
	require.Equal(t, "alice", resource["userName"])
	require.NotContains(t, resource, "displayName")
}

// --- RFC 7644 §3.9 attributes/excludedAttributes mutual exclusivity ---

// TestHandleUsersListRequest_ConflictingAttributesParams_Returns400 tests Handle Users List Request for
// Conflicting Attributes Params Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersListRequest_ConflictingAttributesParams_Returns400() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, []string{"userName"}, []string{"active"}).
		Return(&scim.ErrorConflictingAttributesParams)

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?attributes=userName&excludedAttributes=active", nil)
	rr := httptest.NewRecorder()

	h.HandleUsersListRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersSearchRequest_ConflictingAttributesParams_Returns400 tests Handle Users Search Request for
// Conflicting Attributes Params Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersSearchRequest_ConflictingAttributesParams_Returns400() {
	t := suite.T()
	body := `{"schemas": ["` + scim.SCIMSearchSchemaURN + `"], "attributes": ["userName"], ` +
		`"excludedAttributes": ["active"]}`
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, []string{"userName"}, []string{"active"}).
		Return(&scim.ErrorConflictingAttributesParams)
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersSearchRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersGetRequest_ConflictingAttributesParams_Returns400 tests Handle Users Get Request for
// Conflicting Attributes Params Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersGetRequest_ConflictingAttributesParams_Returns400() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, []string{"userName"}, []string{"active"}).
		Return(&scim.ErrorConflictingAttributesParams)
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(
		http.MethodGet, "/scim/v2/Users/user-123?attributes=userName&excludedAttributes=active", nil)
	req.SetPathValue("id", "user-123")
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersCreateRequest_ConflictingAttributesParams_Returns400 tests Handle Users Create Request for
// Conflicting Attributes Params Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersCreateRequest_ConflictingAttributesParams_Returns400() {
	t := suite.T()
	payloadBody := scimTestPayloadBody
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, []string{"userName"}, []string{"active"}).
		Return(&scim.ErrorConflictingAttributesParams)
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(
		http.MethodPost, "/scim/v2/Users?attributes=userName&excludedAttributes=active",
		bytes.NewBufferString(payloadBody))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestHandleUsersReplaceRequest_ConflictingAttributesParams_Returns400 tests Handle Users Replace Request for
// Conflicting Attributes Params Returns 400.
func (suite *HandlerTestSuite) TestHandleUsersReplaceRequest_ConflictingAttributesParams_Returns400() {
	t := suite.T()
	payloadBody := scimTestPayloadBody
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, []string{"userName"}, []string{"active"}).
		Return(&scim.ErrorConflictingAttributesParams)
	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(
		http.MethodPut, "/scim/v2/Users/user-123?attributes=userName&excludedAttributes=active",
		bytes.NewBufferString(payloadBody))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- RFC 7644 §3.9 projection wired into single-resource responses ---

// TestHandleUsersGetRequest_AppliesAttributeProjection tests Handle Users Get Request for Applies Attribute Projection.
func (suite *HandlerTestSuite) TestHandleUsersGetRequest_AppliesAttributeProjection() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	expectedUser := &SCIMUser{
		ID:      "user-123",
		Schemas: []string{scim.SCIMCoreUserSchemaURN},
		Meta:    scim.SCIMMeta{ResourceType: "User"},
		CoreAttrs: map[string]json.RawMessage{
			"userName": json.RawMessage(`"alice"`),
			"active":   json.RawMessage(`true`),
		},
	}
	mockSvc.On("GetUser", mock.Anything, "user-123", testBaseURL).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users/user-123?attributes=userName", nil)
	req.SetPathValue("id", "user-123")
	rr := httptest.NewRecorder()

	h.HandleUsersGetRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, "alice", got["userName"])
	require.NotContains(t, got, "active")
}

// TestHandleUsersCreateRequest_AppliesAttributeProjection tests Handle Users Create Request for Applies
// Attribute Projection.
func (suite *HandlerTestSuite) TestHandleUsersCreateRequest_AppliesAttributeProjection() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	payloadBody := scimTestPayloadBody
	expectedUser := &SCIMUser{
		ID:      "user-123",
		Schemas: []string{scim.SCIMCoreUserSchemaURN},
		Meta:    scim.SCIMMeta{ResourceType: "User"},
		CoreAttrs: map[string]json.RawMessage{
			"userName": json.RawMessage(`"alice"`),
			"active":   json.RawMessage(`true`),
		},
	}
	mockSvc.On(
		"CreateUser", mock.Anything, mock.AnythingOfType("*users.SCIMUserPayload"), testBaseURL,
	).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(
		http.MethodPost, "/scim/v2/Users?attributes=userName", bytes.NewBufferString(payloadBody))
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersCreateRequest(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var got map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, "alice", got["userName"])
	require.NotContains(t, got, "active")
}

// TestHandleUsersReplaceRequest_AppliesAttributeProjection tests Handle Users Replace Request for Applies
// Attribute Projection.
func (suite *HandlerTestSuite) TestHandleUsersReplaceRequest_AppliesAttributeProjection() {
	t := suite.T()
	mockSvc := NewSCIMUsersServiceInterfaceMock(t)
	mockSvc.On("ValidateAttributePaths", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	payloadBody := scimTestPayloadBody
	expectedUser := &SCIMUser{
		ID:      "user-123",
		Schemas: []string{scim.SCIMCoreUserSchemaURN},
		Meta:    scim.SCIMMeta{ResourceType: "User"},
		CoreAttrs: map[string]json.RawMessage{
			"userName":    json.RawMessage(`"alice"`),
			"displayName": json.RawMessage(`"Alice"`),
		},
	}
	mockSvc.On(
		"ReplaceUser", mock.Anything, "user-123", mock.AnythingOfType("*users.SCIMUserPayload"), testBaseURL, false,
	).Return(expectedUser, (*tidcommon.ServiceError)(nil))

	h := newSCIMUsersHandler(mockSvc, testHandlerConfig)
	req := httptest.NewRequest(
		http.MethodPut, "/scim/v2/Users/user-123?excludedAttributes=displayName", bytes.NewBufferString(payloadBody))
	req.SetPathValue("id", "user-123")
	req.Header.Set("Content-Type", constants.SCIMContentType)
	rr := httptest.NewRecorder()

	h.HandleUsersReplaceRequest(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, "alice", got["userName"])
	require.NotContains(t, got, "displayName")
}

// TestHandleUsers_BodyExceedsLimit tests Handle Users for Body Exceeds Limit.
func (suite *HandlerTestSuite) TestHandleUsers_BodyExceedsLimit() {
	t := suite.T()
	h := newSCIMUsersHandler(nil, testHandlerConfig)

	t.Run("Search", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, scim.MaxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users/.search", oversizedBody)
		req.Header.Set("Content-Type", constants.SCIMContentType)
		rr := httptest.NewRecorder()
		h.HandleUsersSearchRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Create", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, scim.MaxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", oversizedBody)
		req.Header.Set("Content-Type", constants.SCIMContentType)
		rr := httptest.NewRecorder()
		h.HandleUsersCreateRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Replace", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, scim.MaxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Users/user-123", oversizedBody)
		req.SetPathValue("id", "user-123")
		req.Header.Set("Content-Type", constants.SCIMContentType)
		rr := httptest.NewRecorder()
		h.HandleUsersReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("MeReplace", func(t *testing.T) {
		oversizedBody := bytes.NewBuffer(make([]byte, scim.MaxRequestBodyBytes+10))
		req := httptest.NewRequest(http.MethodPut, "/scim/v2/Me", oversizedBody)
		req.Header.Set("Content-Type", constants.SCIMContentType)
		authCtx := security.NewSecurityContextForTest("user-123", "", "", nil, nil)
		req = req.WithContext(security.WithSecurityContextTest(req.Context(), authCtx))
		rr := httptest.NewRecorder()
		h.HandleMeReplaceRequest(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}
