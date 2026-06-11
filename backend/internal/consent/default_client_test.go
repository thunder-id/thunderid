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

package consent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/httpmock"
)

const testBaseURL = "http://consent.example.com"

type DefaultClientTestSuite struct {
	suite.Suite
}

func TestDefaultClientTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultClientTestSuite))
}

// initClientRuntime initializes a server runtime for default client tests.
func initClientRuntime(t *testing.T) {
	t.Helper()
	cfg := &config.Config{
		Consent: config.ConsentConfig{
			Enabled: true,
			BaseURL: testBaseURL,
		},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", cfg))
	t.Cleanup(config.ResetServerRuntime)
}

// buildHTTPResponse returns a mock *http.Response with the given status code and JSON body.
func buildHTTPResponse(t *testing.T, statusCode int, body interface{}) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err, "failed to marshal response body")
		r = bytes.NewReader(b)
	} else {
		r = bytes.NewReader([]byte{})
	}
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(r),
	}
}

// newTestClient creates a defaultClient backed by the http mock.
func newTestClient(t *testing.T, httpMock *httpmock.HTTPClientInterfaceMock) *defaultClient {
	t.Helper()
	initClientRuntime(t)
	return newDefaultClient(httpMock).(*defaultClient)
}

// ----- newDefaultClient -----

func (s *DefaultClientTestSuite) TestNewDefaultClient_ReadsBaseURLFromConfig() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	s.Equal(testBaseURL, c.clientConfig.baseURL)
}

func (s *DefaultClientTestSuite) TestNewDefaultClient_TrailingSlashTrimmed() {
	cfg := &config.Config{
		Consent: config.ConsentConfig{
			Enabled: true,
			BaseURL: testBaseURL + "/",
		},
	}
	config.ResetServerRuntime()
	require.NoError(s.T(), config.InitializeServerRuntime("/tmp/test", cfg))
	s.T().Cleanup(config.ResetServerRuntime)

	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newDefaultClient(httpMock).(*defaultClient)

	s.Equal(testBaseURL, c.clientConfig.baseURL)
}

// ----- createConsentElements -----

func (s *DefaultClientTestSuite) TestCreateConsentElements_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := elementsCreateResponseDTO{
		Data: []elementResponseDTO{{ID: "e1", Name: "email", Type: "basic"}},
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	inputs := []ConsentElementInput{{Name: "email", Namespace: NamespaceAttribute}}
	result, svcErr := c.createConsentElements(context.Background(), "ou1", inputs)

	s.Nil(svcErr)
	s.Len(result, 1)
	s.Equal("email", result[0].Name)
}

func (s *DefaultClientTestSuite) TestCreateConsentElements_BadRequest() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-400", Message: "bad request"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusBadRequest, errBody), nil)

	result, svcErr := c.createConsentElements(context.Background(), "ou1", []ConsentElementInput{{Name: "x"}})

	s.Nil(result)
	s.Equal(&ErrorInvalidConsentElementRequest, svcErr)
}

func (s *DefaultClientTestSuite) TestCreateConsentElements_Conflict() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-409", Message: "conflict"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusConflict, errBody), nil)

	result, svcErr := c.createConsentElements(context.Background(), "ou1", []ConsentElementInput{{Name: "email"}})

	s.Nil(result)
	s.Equal(&ErrorConsentElementAlreadyExists, svcErr)
}

func (s *DefaultClientTestSuite) TestCreateConsentElements_HTTPClientError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("connection refused"))

	result, svcErr := c.createConsentElements(context.Background(), "ou1", []ConsentElementInput{{Name: "attr1"}})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestCreateConsentElements_ServerError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-500", Message: "internal error"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusInternalServerError, errBody), nil)

	result, svcErr := c.createConsentElements(context.Background(), "ou1", []ConsentElementInput{{Name: "attr1"}})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

// ----- listConsentElements -----

func (s *DefaultClientTestSuite) TestListConsentElements_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := elementListResponseDTO{
		Data: []elementResponseDTO{{ID: "e1", Name: "email", Type: "basic"}},
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	result, svcErr := c.listConsentElements(context.Background(), "ou1", NamespaceAttribute, "email")

	s.Nil(svcErr)
	s.Len(result, 1)
	s.Equal("email", result[0].Name)
}

func (s *DefaultClientTestSuite) TestListConsentElements_NoFilter_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := elementListResponseDTO{Data: []elementResponseDTO{}}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	result, svcErr := c.listConsentElements(context.Background(), "ou1", NamespaceAttribute, "")

	s.Nil(svcErr)
	s.Empty(result)
}

func (s *DefaultClientTestSuite) TestListConsentElements_ServerError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-500", Message: "internal error"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusInternalServerError, errBody), nil)

	result, svcErr := c.listConsentElements(context.Background(), "ou1", NamespaceAttribute, "")

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

// ----- updateConsentElement -----

func (s *DefaultClientTestSuite) TestUpdateConsentElement_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := elementResponseDTO{ID: "e1", Name: "updated-email", Type: "basic"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	input := &ConsentElementInput{Name: "updated-email"}
	result, svcErr := c.updateConsentElement(context.Background(), "ou1", "e1", input)

	s.Nil(svcErr)
	s.Equal("updated-email", result.Name)
}

func (s *DefaultClientTestSuite) TestUpdateConsentElement_NotFound() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-404"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusNotFound, errBody), nil)

	result, svcErr := c.updateConsentElement(context.Background(), "ou1", "missing", &ConsentElementInput{Name: "x"})

	s.Nil(result)
	s.Equal(&ErrorConsentElementNotFound, svcErr)
}

func (s *DefaultClientTestSuite) TestUpdateConsentElement_BadRequest() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-400"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusBadRequest, errBody), nil)

	result, svcErr := c.updateConsentElement(context.Background(), "ou1", "e1", &ConsentElementInput{Name: "x"})

	s.Nil(result)
	s.Equal(&ErrorInvalidConsentElementRequest, svcErr)
}

func (s *DefaultClientTestSuite) TestUpdateConsentElement_Conflict() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-409"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusConflict, errBody), nil)

	result, svcErr := c.updateConsentElement(context.Background(), "ou1", "e1", &ConsentElementInput{Name: "dup"})

	s.Nil(result)
	s.Equal(&ErrorConsentElementAlreadyExists, svcErr)
}

// ----- deleteConsentElement -----

func (s *DefaultClientTestSuite) TestDeleteConsentElement_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusNoContent, nil), nil)

	svcErr := c.deleteConsentElement(context.Background(), "ou1", "e1")

	s.Nil(svcErr)
}

func (s *DefaultClientTestSuite) TestDeleteConsentElement_NotFound() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-404"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusNotFound, errBody), nil)

	svcErr := c.deleteConsentElement(context.Background(), "ou1", "missing")

	s.Equal(&ErrorConsentElementNotFound, svcErr)
}

func (s *DefaultClientTestSuite) TestDeleteConsentElement_CE5009_ReturnsElementAssociatedWithPurposeError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock) // maxRetries=0 → 1 attempt (the last attempt)

	errBody := consentBackendErrorDTO{
		Code:        "CE-5009",
		Message:     "element is associated with a purpose",
		Description: "The element is currently linked to a purpose",
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusInternalServerError, errBody), nil)

	svcErr := c.deleteConsentElement(context.Background(), "ou1", "e1")

	s.Equal(&ErrorDeletingConsentElementWithAssociatedPurpose, svcErr)
}

func (s *DefaultClientTestSuite) TestDeleteConsentElement_Unauthorized() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusUnauthorized, nil), nil)

	svcErr := c.deleteConsentElement(context.Background(), "ou1", "e1")

	s.Equal(&ErrorConsentServiceReturnedUnauthorized, svcErr)
}

// ----- validateConsentElements -----

func (s *DefaultClientTestSuite) TestValidateConsentElements_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := []string{"email", "phone"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	result, svcErr := c.validateConsentElements(context.Background(), "ou1", []string{"email", "phone"})

	s.Nil(svcErr)
	s.Equal([]string{"email", "phone"}, result)
}

func (s *DefaultClientTestSuite) TestValidateConsentElements_BadRequestReturnsEmpty() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-400", Message: "no elements found"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusBadRequest, errBody), nil)

	result, svcErr := c.validateConsentElements(context.Background(), "ou1", []string{"nonexistent"})

	s.Nil(svcErr)
	s.Equal([]string{}, result)
}

func (s *DefaultClientTestSuite) TestValidateConsentElements_ServerError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-500"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusInternalServerError, errBody), nil)

	result, svcErr := c.validateConsentElements(context.Background(), "ou1", []string{"email"})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

// ----- createConsentPurpose -----

func (s *DefaultClientTestSuite) TestCreateConsentPurpose_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := purposeResponseDTO{
		ID:       "p1",
		Name:     "Login Purpose",
		ClientID: "app-1",
		Elements: []purposeElementDTO{{Name: "email", IsMandatory: false}},
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	input := &ConsentPurposeInput{
		Name:     "Login Purpose",
		GroupID:  "app-1",
		Elements: []PurposeElement{{Name: "email", IsMandatory: false}},
	}
	result, svcErr := c.createConsentPurpose(context.Background(), "ou1", input)

	s.Nil(svcErr)
	s.Equal("Login Purpose", result.Name)
	s.Equal("app-1", result.GroupID)
}

func (s *DefaultClientTestSuite) TestCreateConsentPurpose_BadRequest() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-400"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusBadRequest, errBody), nil)

	result, svcErr := c.createConsentPurpose(context.Background(), "ou1", &ConsentPurposeInput{Name: "bad"})

	s.Nil(result)
	s.Equal(&ErrorInvalidConsentPurposeRequest, svcErr)
}

func (s *DefaultClientTestSuite) TestCreateConsentPurpose_Conflict() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-409"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusConflict, errBody), nil)

	result, svcErr := c.createConsentPurpose(context.Background(), "ou1", &ConsentPurposeInput{Name: "dup"})

	s.Nil(result)
	s.Equal(&ErrorConsentPurposeAlreadyExists, svcErr)
}

// ----- listConsentPurposes -----

func (s *DefaultClientTestSuite) TestListConsentPurposes_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := purposeListResponseDTO{
		Data: []purposeResponseDTO{{ID: "p1", Name: "Login", ClientID: "app-1"}},
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	result, svcErr := c.listConsentPurposes(context.Background(), "ou1", "app-1")

	s.Nil(svcErr)
	s.Len(result, 1)
	s.Equal("Login", result[0].Name)
}

func (s *DefaultClientTestSuite) TestListConsentPurposes_ServerError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-500"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusInternalServerError, errBody), nil)

	result, svcErr := c.listConsentPurposes(context.Background(), "ou1", "app-1")

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

// ----- updateConsentPurpose -----

func (s *DefaultClientTestSuite) TestUpdateConsentPurpose_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := purposeResponseDTO{ID: "p1", Name: "Updated", ClientID: "app-1"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	input := &ConsentPurposeInput{Name: "Updated", GroupID: "app-1"}
	result, svcErr := c.updateConsentPurpose(context.Background(), "ou1", "p1", input)

	s.Nil(svcErr)
	s.Equal("Updated", result.Name)
}

func (s *DefaultClientTestSuite) TestUpdateConsentPurpose_NotFound() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-404"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusNotFound, errBody), nil)

	result, svcErr := c.updateConsentPurpose(context.Background(), "ou1", "missing", &ConsentPurposeInput{Name: "x"})

	s.Nil(result)
	s.Equal(&ErrorConsentPurposeNotFound, svcErr)
}

func (s *DefaultClientTestSuite) TestUpdateConsentPurpose_BadRequest() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-400"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusBadRequest, errBody), nil)

	result, svcErr := c.updateConsentPurpose(context.Background(), "ou1", "p1", &ConsentPurposeInput{Name: "bad"})

	s.Nil(result)
	s.Equal(&ErrorInvalidConsentPurposeRequest, svcErr)
}

func (s *DefaultClientTestSuite) TestUpdateConsentPurpose_Conflict() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-409"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusConflict, errBody), nil)

	result, svcErr := c.updateConsentPurpose(context.Background(), "ou1", "p1", &ConsentPurposeInput{Name: "dup"})

	s.Nil(result)
	s.Equal(&ErrorConsentPurposeAlreadyExists, svcErr)
}

// ----- deleteConsentPurpose -----

func (s *DefaultClientTestSuite) TestDeleteConsentPurpose_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusNoContent, nil), nil)

	svcErr := c.deleteConsentPurpose(context.Background(), "ou1", "p1")

	s.Nil(svcErr)
}

func (s *DefaultClientTestSuite) TestDeleteConsentPurpose_NotFound() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-404"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusNotFound, errBody), nil)

	svcErr := c.deleteConsentPurpose(context.Background(), "ou1", "missing")

	s.Equal(&ErrorConsentPurposeNotFound, svcErr)
}

func (s *DefaultClientTestSuite) TestDeleteConsentPurpose_ConflictDueToAssociatedRecords() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-409", Message: "purpose has associated records"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusConflict, errBody), nil)

	svcErr := c.deleteConsentPurpose(context.Background(), "ou1", "p1")

	s.Equal(&ErrorDeletingConsentPurposeWithAssociatedRecords, svcErr)
}

// ----- createConsent -----

func (s *DefaultClientTestSuite) TestCreateConsent_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := consentResponseDTO{
		ID:       "c1",
		Type:     "authentication",
		ClientID: "app-1",
		Status:   "ACTIVE",
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	req := &ConsentRequest{
		Type:    "authentication",
		GroupID: "app-1",
	}
	result, svcErr := c.createConsent(context.Background(), "ou1", req)

	s.Nil(svcErr)
	s.Equal("c1", result.ID)
	s.Equal(ConsentStatusActive, result.Status)
}

func (s *DefaultClientTestSuite) TestCreateConsent_BadRequest() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-400"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusBadRequest, errBody), nil)

	result, svcErr := c.createConsent(context.Background(), "ou1", &ConsentRequest{Type: "bad"})

	s.Nil(result)
	s.Equal(&ErrorInvalidConsentRecordRequest, svcErr)
}

func (s *DefaultClientTestSuite) TestCreateConsent_ServerError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-500"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusInternalServerError, errBody), nil)

	result, svcErr := c.createConsent(context.Background(), "ou1", &ConsentRequest{Type: "x"})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

// ----- searchConsents -----

func (s *DefaultClientTestSuite) TestSearchConsents_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := consentSearchResponseDTO{
		Data: []consentResponseDTO{
			{ID: "c1", Type: "authentication", Status: "ACTIVE"},
		},
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	filter := &ConsentSearchFilter{ConsentTypes: []ConsentType{ConsentTypeAuthentication}}
	result, svcErr := c.searchConsents(context.Background(), "ou1", filter)

	s.Nil(svcErr)
	s.Len(result, 1)
	s.Equal("c1", result[0].ID)
}

func (s *DefaultClientTestSuite) TestSearchConsents_BadRequest() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-400"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusBadRequest, errBody), nil)

	result, svcErr := c.searchConsents(context.Background(), "ou1", &ConsentSearchFilter{})

	s.Nil(result)
	s.Equal(&ErrorInvalidConsentSearchFilter, svcErr)
}

func (s *DefaultClientTestSuite) TestSearchConsents_EmptyFilter_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := consentSearchResponseDTO{Data: []consentResponseDTO{}}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	result, svcErr := c.searchConsents(context.Background(), "ou1", &ConsentSearchFilter{
		ConsentStatuses: []ConsentStatus{ConsentStatusActive},
		GroupIDs:        []string{"app-1"},
		UserIDs:         []string{"user-1"},
		PurposeNames:    []string{"login"},
		Limit:           10,
		Offset:          5,
	})

	s.Nil(svcErr)
	s.Empty(result)
}

func (s *DefaultClientTestSuite) TestSearchConsents_ExpiredStatusFilter_ValidityTimeNormalization() {
	nowUnix := time.Now().Unix()
	tests := []struct {
		name string
		data []consentResponseDTO
	}{
		{
			name: "converts active consent to expired when validity time has elapsed",
			data: []consentResponseDTO{
				{ID: "c-expired", Type: "authentication", Status: "ACTIVE", ValidityTime: nowUnix - 60},
				{ID: "c-active", Type: "authentication", Status: "ACTIVE", ValidityTime: nowUnix + 3600},
			},
		},
		{
			name: "does not convert non-active consent status",
			data: []consentResponseDTO{
				{ID: "c-revoked", Type: "authentication", Status: "REVOKED", ValidityTime: nowUnix - 60},
				{ID: "c-expired", Type: "authentication", Status: "ACTIVE", ValidityTime: nowUnix - 60},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		s.Run(tc.name, func() {
			httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
			c := newTestClient(s.T(), httpMock)

			respBody := consentSearchResponseDTO{Data: tc.data}
			httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
				Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

			result, svcErr := c.searchConsents(context.Background(), "ou1", &ConsentSearchFilter{
				ConsentStatuses: []ConsentStatus{ConsentStatusExpired},
			})

			s.Nil(svcErr)
			s.Len(result, 1)
			s.Equal("c-expired", result[0].ID)
			s.Equal(ConsentStatusExpired, result[0].Status)
		})
	}
}

// ----- validateConsent -----

func (s *DefaultClientTestSuite) TestValidateConsent_Valid() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := consentValidateResponseDTO{
		IsValid: true,
		ConsentInformation: consentResponseDTO{
			ID: "c1", Status: "ACTIVE",
		},
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	result, svcErr := c.validateConsent(context.Background(), "ou1", "c1")

	s.Nil(svcErr)
	s.True(result.IsValid)
	s.NotNil(result.ConsentInformation)
	s.Equal("c1", result.ConsentInformation.ID)
}

func (s *DefaultClientTestSuite) TestValidateConsent_Invalid() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := consentValidateResponseDTO{
		IsValid:      false,
		ErrorCode:    "CONSENT_EXPIRED",
		ErrorMessage: "Consent has expired",
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	result, svcErr := c.validateConsent(context.Background(), "ou1", "c1")

	s.Nil(svcErr)
	s.False(result.IsValid)
	s.Nil(result.ConsentInformation)
}

func (s *DefaultClientTestSuite) TestValidateConsent_BadRequest() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-400"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusBadRequest, errBody), nil)

	result, svcErr := c.validateConsent(context.Background(), "ou1", "c1")

	s.Nil(result)
	s.Equal(&ErrorInvalidConsentValidationRequest, svcErr)
}

// ----- updateConsent -----

func (s *DefaultClientTestSuite) TestUpdateConsent_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := consentResponseDTO{
		ID:       "c1",
		Type:     "authentication",
		ClientID: "app-1",
		Status:   "ACTIVE",
		Purposes: []purposeItemResponseDTO{
			{
				Name: "login",
				Elements: []elementApprovalResponseDTO{
					{Name: "email", IsUserApproved: true},
					{Name: "family_name", IsUserApproved: false},
				},
			},
		},
	}
	httpMock.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPut && req.URL.Path == "/consents/c1"
	})).Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	req := &ConsentRequest{
		Type:    ConsentTypeAuthentication,
		GroupID: "app-1",
		Purposes: []ConsentPurposeItem{
			{
				Name: "login",
				Elements: []ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "family_name", IsUserApproved: false},
				},
			},
		},
	}
	result, svcErr := c.updateConsent(context.Background(), "ou1", "c1", req)

	s.Nil(svcErr)
	s.Equal("c1", result.ID)
	s.Equal(ConsentStatusActive, result.Status)
	s.Len(result.Purposes, 1)
	s.Equal("login", result.Purposes[0].Name)
	s.Len(result.Purposes[0].Elements, 2)
	s.True(result.Purposes[0].Elements[0].IsUserApproved)  // email approved
	s.False(result.Purposes[0].Elements[1].IsUserApproved) // family_name denied
}

func (s *DefaultClientTestSuite) TestUpdateConsent_BadRequest() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-400"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusBadRequest, errBody), nil)

	result, svcErr := c.updateConsent(context.Background(), "ou1", "c1",
		&ConsentRequest{Type: ConsentTypeAuthentication})

	s.Nil(result)
	s.Equal(&ErrorInvalidConsentUpdateRequest, svcErr)
}

func (s *DefaultClientTestSuite) TestUpdateConsent_NotFound() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-404"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusNotFound, errBody), nil)

	result, svcErr := c.updateConsent(context.Background(), "ou1", "missing",
		&ConsentRequest{Type: ConsentTypeAuthentication})

	s.Nil(result)
	s.Equal(&ErrorConsentRecordNotFound, svcErr)
}

func (s *DefaultClientTestSuite) TestUpdateConsent_ServerError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-500"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusInternalServerError, errBody), nil)

	result, svcErr := c.updateConsent(context.Background(), "ou1", "c1",
		&ConsentRequest{Type: ConsentTypeAuthentication})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestUpdateConsent_UsesCorrectHTTPMethod() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := consentResponseDTO{ID: "c1", Status: "ACTIVE"}
	httpMock.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPut
	})).Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	_, svcErr := c.updateConsent(context.Background(), "ou1", "c1", &ConsentRequest{
		Type:    ConsentTypeAuthentication,
		GroupID: "app-1",
	})

	s.Nil(svcErr)
}

// ----- revokeConsent -----

func (s *DefaultClientTestSuite) TestRevokeConsent_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, nil), nil)

	svcErr := c.revokeConsent(context.Background(), "ou1", "c1", &ConsentRevokeRequest{Reason: "user requested"})

	s.Nil(svcErr)
}

func (s *DefaultClientTestSuite) TestRevokeConsent_BadRequest() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-400"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusBadRequest, errBody), nil)

	svcErr := c.revokeConsent(context.Background(), "ou1", "c1", &ConsentRevokeRequest{})

	s.Equal(&ErrorInvalidConsentRevokeRequest, svcErr)
}

func (s *DefaultClientTestSuite) TestRevokeConsent_ServerError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-500"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusInternalServerError, errBody), nil)

	svcErr := c.revokeConsent(context.Background(), "ou1", "c1", &ConsentRevokeRequest{Reason: "reason"})

	s.Equal(&serviceerror.InternalServerError, svcErr)
}

// ----- checkStatus corner cases -----

func (s *DefaultClientTestSuite) TestCheckStatus_2xxReturnsNil() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	for _, code := range []int{200, 201, 204} {
		resp := buildHTTPResponse(s.T(), code, nil)
		s.Nil(c.checkStatus(context.Background(), resp), "expected nil for status %d", code)
	}
}

func (s *DefaultClientTestSuite) TestCheckStatus_401ReturnsUnauthorized() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	resp := buildHTTPResponse(s.T(), http.StatusUnauthorized, nil)
	svcErr := c.checkStatus(context.Background(), resp)

	s.Equal(&ErrorConsentServiceReturnedUnauthorized, svcErr)
}

func (s *DefaultClientTestSuite) TestCheckStatus_403ReturnsForbidden() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	resp := buildHTTPResponse(s.T(), http.StatusForbidden, nil)
	svcErr := c.checkStatus(context.Background(), resp)

	s.Equal(&ErrorConsentServiceReturnedForbidden, svcErr)
}

func (s *DefaultClientTestSuite) TestCheckStatus_5xxReturnsInternalServerError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "CE-500"}
	resp := buildHTTPResponse(s.T(), http.StatusInternalServerError, errBody)
	svcErr := c.checkStatus(context.Background(), resp)

	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestCheckStatus_5xxWithCE5009ReturnsElementAssociatedWithPurposeError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	specialErrBody := consentBackendErrorDTO{
		Code:        "CE-5009",
		Message:     "element is associated with a purpose",
		Description: "The element is currently linked to a purpose",
	}
	resp := buildHTTPResponse(s.T(), http.StatusInternalServerError, specialErrBody)
	svcErr := c.checkStatus(context.Background(), resp)

	s.Equal(&ErrorDeletingConsentElementWithAssociatedPurpose, svcErr)
}

func (s *DefaultClientTestSuite) TestCheckStatus_OtherError4xxReturnsInvalidConsentRequest() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	errBody := consentBackendErrorDTO{Code: "UNKNOWN"}
	resp := buildHTTPResponse(s.T(), http.StatusUnprocessableEntity, errBody)
	svcErr := c.checkStatus(context.Background(), resp)

	s.Equal(&ErrorInvalidConsentRequest, svcErr)
}

// ----- buildConsentSearchURL -----

func (s *DefaultClientTestSuite) TestBuildConsentSearchURL_AllFilters() {
	initClientRuntime(s.T())
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newDefaultClient(httpMock).(*defaultClient)

	filter := &ConsentSearchFilter{
		ConsentTypes:    []ConsentType{ConsentTypeAuthentication},
		ConsentStatuses: []ConsentStatus{ConsentStatusActive},
		GroupIDs:        []string{"app-1"},
		UserIDs:         []string{"user-1"},
		PurposeNames:    []string{"login"},
		Limit:           10,
		Offset:          5,
	}

	u, svcErr := c.buildConsentSearchURL(context.Background(), filter)

	s.Nil(svcErr)
	// Ensure the "?" query separator is not escaped to "%3F" in the URL path.
	s.NotContains(u, "%3F")
	s.Contains(u, "?")
	s.Contains(u, "consentTypes=")
	s.Contains(u, "consentStatuses=")
	s.Contains(u, "clientIds=")
	s.Contains(u, "userIds=")
	s.Contains(u, "purposeNames=")
	s.Contains(u, "limit=")
	s.Contains(u, "offset=")
}

func (s *DefaultClientTestSuite) TestBuildConsentSearchURL_EmptyFilter() {
	initClientRuntime(s.T())
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newDefaultClient(httpMock).(*defaultClient)

	u, svcErr := c.buildConsentSearchURL(context.Background(), &ConsentSearchFilter{})

	s.Nil(svcErr)
	s.Contains(u, testBaseURL)
	s.NotContains(u, "consentTypes")
	// An empty filter should produce no query string.
	s.NotContains(u, "?")
}

func (s *DefaultClientTestSuite) TestBuildConsentSearchURL_NilFilter() {
	initClientRuntime(s.T())
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newDefaultClient(httpMock).(*defaultClient)

	// A nil filter should not panic and should return the bare endpoint URL.
	u, svcErr := c.buildConsentSearchURL(context.Background(), nil)

	s.Nil(svcErr)
	s.Contains(u, testBaseURL)
	s.NotContains(u, "?")
}

// ----- doRequest error paths -----

func (s *DefaultClientTestSuite) TestDoRequest_HTTPClientReturnsError_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("network failure"))

	// Trigger a path that uses doRequest (e.g., listConsentPurposes)
	respBody := purposeListResponseDTO{Data: []purposeResponseDTO{}}
	_ = respBody
	result, svcErr := c.listConsentPurposes(context.Background(), "ou1", "")

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

// ----- HTTP client error paths for remaining functions -----

func (s *DefaultClientTestSuite) TestListConsentElements_HTTPError_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("network failure"))

	result, svcErr := c.listConsentElements(context.Background(), "ou1", NamespaceAttribute, "email")

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestUpdateConsentElement_HTTPError_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("network failure"))

	result, svcErr := c.updateConsentElement(context.Background(), "ou1", "e1", &ConsentElementInput{Name: "email"})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestDeleteConsentElement_HTTPError_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("network failure"))

	svcErr := c.deleteConsentElement(context.Background(), "ou1", "e1")

	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestValidateConsentElements_HTTPError_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("network failure"))

	result, svcErr := c.validateConsentElements(context.Background(), "ou1", []string{"email"})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestCreateConsentPurpose_HTTPError_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("network failure"))

	result, svcErr := c.createConsentPurpose(context.Background(), "ou1", &ConsentPurposeInput{Name: "test"})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestUpdateConsentPurpose_HTTPError_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("network failure"))

	result, svcErr := c.updateConsentPurpose(context.Background(), "ou1", "p1", &ConsentPurposeInput{Name: "test"})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestCreateConsent_HTTPError_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("network failure"))

	result, svcErr := c.createConsent(context.Background(), "ou1", &ConsentRequest{Type: "authentication"})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestSearchConsents_HTTPError_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("network failure"))

	result, svcErr := c.searchConsents(context.Background(), "ou1", &ConsentSearchFilter{})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestValidateConsent_HTTPError_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("network failure"))

	result, svcErr := c.validateConsent(context.Background(), "ou1", "c1")

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestRevokeConsent_HTTPError_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("network failure"))

	svcErr := c.revokeConsent(context.Background(), "ou1", "c1", &ConsentRevokeRequest{Reason: "test"})

	s.Equal(&serviceerror.InternalServerError, svcErr)
}

// ----- createConsent with Authorizations (covers consentAuthorizationRequestToDTO) -----

func (s *DefaultClientTestSuite) TestCreateConsent_WithAuthorizations_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	// Response includes authorizations to cover consentAuthorizationDtoToResponse.
	respBody := consentResponseDTO{
		ID:     "c2",
		Type:   "authentication",
		Status: "ACTIVE",
		Authorizations: []authorizationResponseDTO{
			{ID: "auth-1", UserID: "user-1", Type: "primary", Status: "ACTIVE"},
		},
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	req := &ConsentRequest{
		Type:    "authentication",
		GroupID: "app-1",
		Authorizations: []ConsentAuthorizationRequest{
			{UserID: "user-1", Type: "primary", Status: "ACTIVE"},
		},
	}
	result, svcErr := c.createConsent(context.Background(), "ou1", req)

	s.Nil(svcErr)
	s.Equal("c2", result.ID)
	s.Len(result.Authorizations, 1)
	s.Equal("auth-1", result.Authorizations[0].ID)
}

// ----- searchConsents with Authorizations in response (covers consentAuthorizationDtoToResponse) -----

func (s *DefaultClientTestSuite) TestSearchConsents_WithAuthorizationsInResponse() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := consentSearchResponseDTO{
		Data: []consentResponseDTO{
			{
				ID:     "c3",
				Type:   "authentication",
				Status: "ACTIVE",
				Authorizations: []authorizationResponseDTO{
					{ID: "auth-2", UserID: "user-2", Type: "primary", Status: "ACTIVE"},
				},
			},
		},
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	filter := &ConsentSearchFilter{ConsentTypes: []ConsentType{ConsentTypeAuthentication}}
	result, svcErr := c.searchConsents(context.Background(), "ou1", filter)

	s.Nil(svcErr)
	s.Len(result, 1)
	s.Len(result[0].Authorizations, 1)
	s.Equal("auth-2", result[0].Authorizations[0].ID)
}

// ----- validateConsentElements nil result path -----

func (s *DefaultClientTestSuite) TestValidateConsentElements_NilResult_ReturnsEmpty() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	// Return a response with a null/empty array body to trigger the nil-result path
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte("null"))),
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(resp, nil)

	result, svcErr := c.validateConsentElements(context.Background(), "ou1", []string{"email"})

	s.Nil(svcErr)
	s.Empty(result)
}

// ----- JSON decode error paths -----

// buildInvalidJSONResponse returns an HTTP 200 response with a non-JSON body to trigger decode errors.
func buildInvalidJSONResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte("{invalid json}"))),
	}
}

func (s *DefaultClientTestSuite) TestCreateConsentElements_InvalidJSON_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).Return(buildInvalidJSONResponse(), nil)

	result, svcErr := c.createConsentElements(context.Background(), "ou1", []ConsentElementInput{{Name: "email"}})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestListConsentElements_InvalidJSON_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).Return(buildInvalidJSONResponse(), nil)

	result, svcErr := c.listConsentElements(context.Background(), "ou1", NamespaceAttribute, "email")

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestUpdateConsentElement_InvalidJSON_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).Return(buildInvalidJSONResponse(), nil)

	result, svcErr := c.updateConsentElement(context.Background(), "ou1", "e1", &ConsentElementInput{Name: "email"})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestValidateConsentElements_InvalidJSON_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).Return(buildInvalidJSONResponse(), nil)

	result, svcErr := c.validateConsentElements(context.Background(), "ou1", []string{"email"})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestCreateConsentPurpose_InvalidJSON_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).Return(buildInvalidJSONResponse(), nil)

	result, svcErr := c.createConsentPurpose(context.Background(), "ou1", &ConsentPurposeInput{Name: "login"})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestListConsentPurposes_InvalidJSON_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).Return(buildInvalidJSONResponse(), nil)

	result, svcErr := c.listConsentPurposes(context.Background(), "ou1", "app-1")

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestUpdateConsentPurpose_InvalidJSON_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).Return(buildInvalidJSONResponse(), nil)

	result, svcErr := c.updateConsentPurpose(context.Background(), "ou1", "p1", &ConsentPurposeInput{Name: "login"})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestCreateConsent_InvalidJSON_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).Return(buildInvalidJSONResponse(), nil)

	result, svcErr := c.createConsent(context.Background(), "ou1", &ConsentRequest{Type: "authentication"})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestSearchConsents_InvalidJSON_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).Return(buildInvalidJSONResponse(), nil)

	result, svcErr := c.searchConsents(context.Background(), "ou1", &ConsentSearchFilter{})

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

func (s *DefaultClientTestSuite) TestValidateConsent_InvalidJSON_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).Return(buildInvalidJSONResponse(), nil)

	result, svcErr := c.validateConsent(context.Background(), "ou1", "c1")

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

// ----- consentRequestToDTO inner element loop (Purposes with Elements) -----

func (s *DefaultClientTestSuite) TestCreateConsent_WithPurposesAndElements_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := consentResponseDTO{
		ID:     "c4",
		Type:   "authentication",
		Status: "ACTIVE",
		Purposes: []purposeItemResponseDTO{
			{
				Name: "login",
				Elements: []elementApprovalResponseDTO{
					{Name: "email", IsUserApproved: true},
					{Name: "phone", IsUserApproved: false},
				},
			},
		},
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	req := &ConsentRequest{
		Type:    "authentication",
		GroupID: "app-1",
		Purposes: []ConsentPurposeItem{
			{
				Name: "login",
				Elements: []ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "phone", IsUserApproved: false},
				},
			},
		},
	}
	result, svcErr := c.createConsent(context.Background(), "ou1", req)

	s.Nil(svcErr)
	s.Equal("c4", result.ID)
	s.Len(result.Purposes, 1)
	s.Equal("login", result.Purposes[0].Name)
	s.Len(result.Purposes[0].Elements, 2)
	s.Equal("email", result.Purposes[0].Elements[0].Name)
	s.True(result.Purposes[0].Elements[0].IsUserApproved)
}

// ----- dtoToConsent inner element loop (response Purposes with Elements) -----

func (s *DefaultClientTestSuite) TestSearchConsents_WithPurposesAndElements_Success() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := consentSearchResponseDTO{
		Data: []consentResponseDTO{
			{
				ID:     "c5",
				Type:   "authentication",
				Status: "ACTIVE",
				Purposes: []purposeItemResponseDTO{
					{
						Name: "profile",
						Elements: []elementApprovalResponseDTO{
							{Name: "given_name", IsUserApproved: true},
						},
					},
				},
			},
		},
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	result, svcErr := c.searchConsents(context.Background(), "ou1", &ConsentSearchFilter{})

	s.Nil(svcErr)
	s.Len(result, 1)
	s.Len(result[0].Purposes, 1)
	s.Equal("profile", result[0].Purposes[0].Name)
	s.Len(result[0].Purposes[0].Elements, 1)
	s.Equal("given_name", result[0].Purposes[0].Elements[0].Name)
}

// newTestClientWithConfig creates a defaultClient backed by httpMock and with explicit timeout/maxRetries.
func newTestClientWithConfig(t *testing.T, httpMock *httpmock.HTTPClientInterfaceMock,
	timeout, maxRetries int) *defaultClient {
	t.Helper()
	cfg := &config.Config{
		Consent: config.ConsentConfig{
			Enabled:    true,
			BaseURL:    testBaseURL,
			Timeout:    timeout,
			MaxRetries: maxRetries,
		},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", cfg))
	t.Cleanup(config.ResetServerRuntime)
	return newDefaultClient(httpMock).(*defaultClient)
}

// ----- getClientConfig -----

func (s *DefaultClientTestSuite) TestGetClientConfig_DefaultsTimeout_WhenZero() {
	cfg := &config.Config{
		Consent: config.ConsentConfig{Enabled: true, BaseURL: testBaseURL, Timeout: 0},
	}
	config.ResetServerRuntime()
	require.NoError(s.T(), config.InitializeServerRuntime("/tmp/test", cfg))
	s.T().Cleanup(config.ResetServerRuntime)

	c := getClientConfig()

	s.Equal(5*time.Second, c.timeout)
}

func (s *DefaultClientTestSuite) TestGetClientConfig_DefaultsMaxRetries_WhenNegative() {
	cfg := &config.Config{
		Consent: config.ConsentConfig{Enabled: true, BaseURL: testBaseURL, MaxRetries: -1},
	}
	config.ResetServerRuntime()
	require.NoError(s.T(), config.InitializeServerRuntime("/tmp/test", cfg))
	s.T().Cleanup(config.ResetServerRuntime)

	c := getClientConfig()

	s.Equal(3, c.maxRetries)
}

func (s *DefaultClientTestSuite) TestGetClientConfig_ExplicitValues() {
	cfg := &config.Config{
		Consent: config.ConsentConfig{Enabled: true, BaseURL: testBaseURL, Timeout: 10, MaxRetries: 2},
	}
	config.ResetServerRuntime()
	require.NoError(s.T(), config.InitializeServerRuntime("/tmp/test", cfg))
	s.T().Cleanup(config.ResetServerRuntime)

	c := getClientConfig()

	s.Equal(10*time.Second, c.timeout)
	s.Equal(2, c.maxRetries)
}

// ----- doRequest retry logic -----

// TestDoRequest_ContextCancelledDuringRetry covers the select{case <-ctx.Done()} path.
// The context is cancelled before the first call returns, so at attempt=1 the select
// immediately picks ctx.Done() and returns InternalServerError without the 1 s backoff.
func (s *DefaultClientTestSuite) TestDoRequest_ContextCancelledDuringRetry_ReturnsInternalError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClientWithConfig(s.T(), httpMock, 1, 1) // maxRetries=1 → 2 attempts

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	// Mock is called exactly once (attempt=0); attempt=1 bails on ctx.Done().
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("network error")).Once()

	result, svcErr := c.listConsentPurposes(ctx, "ou1", "")

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
}

// TestDoRequest_5xxRetryThenFinal5xx_PassesThroughToCheckStatus verifies that on the last
// retry attempt a 5xx response is returned to the caller instead of being swallowed.  The
// first attempt returns 500 (retry), and the second (last) attempt also returns 500; the
// response must flow through checkStatus so the final error is InternalServerError and
// the HTTP mock is invoked exactly twice.
func (s *DefaultClientTestSuite) TestDoRequest_5xxRetryThenFinal5xx_PassesThroughToCheckStatus() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClientWithConfig(s.T(), httpMock, 1, 1) // maxRetries=1 → 2 attempts

	errBody := consentBackendErrorDTO{Code: "CE-500", Message: "internal error"}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusInternalServerError, errBody), nil).Once()
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusInternalServerError, errBody), nil).Once()

	result, svcErr := c.listConsentPurposes(context.Background(), "ou1", "")

	s.Nil(result)
	s.Equal(&serviceerror.InternalServerError, svcErr)
	httpMock.AssertNumberOfCalls(s.T(), "Do", 2)
}

// TestDoRequest_NetworkErrorRetry_SucceedsOnSecond covers the time.After(backoff) path.
// The first attempt fails, the backoff elapses (~1 s), and the second attempt succeeds.
func (s *DefaultClientTestSuite) TestDoRequest_NetworkErrorRetry_SucceedsOnSecond() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClientWithConfig(s.T(), httpMock, 1, 1) // maxRetries=1 → 2 attempts

	successResp := buildHTTPResponse(s.T(), http.StatusOK,
		purposeListResponseDTO{Data: []purposeResponseDTO{{ID: "p1", Name: "Login"}}})

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(nil, errors.New("transient error")).Once()
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(successResp, nil).Once()

	result, svcErr := c.listConsentPurposes(context.Background(), "ou1", "")

	s.Nil(svcErr)
	s.Len(result, 1)
	s.Equal("Login", result[0].Name)
}

// ----- closeBody nil-safety -----

func (s *DefaultClientTestSuite) TestCloseBody_NilResponse_DoesNotPanic() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	s.NotPanics(func() { c.closeBody(context.Background(), nil) })
	s.NotPanics(func() { c.closeBody(context.Background(), &http.Response{}) })
}

// ----- handleClientError with undecodable body -----

func (s *DefaultClientTestSuite) TestHandleClientError_InvalidJSONBody_ReturnsProvidedError() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	// 400 response with invalid JSON body triggers the decode-error branch in handleClientError.
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewReader([]byte("{invalid}"))),
	}
	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).Return(resp, nil)

	result, svcErr := c.updateConsentElement(context.Background(), "ou1", "e1",
		&ConsentElementInput{Name: "email"})

	s.Nil(result)
	s.Equal(&ErrorInvalidConsentElementRequest, svcErr)
}

// ----- revokeConsent nil payload -----

func (s *DefaultClientTestSuite) TestRevokeConsent_NilPayload_Succeeds() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	httpMock.EXPECT().Do(mock.AnythingOfType("*http.Request")).
		Return(buildHTTPResponse(s.T(), http.StatusOK, nil), nil)

	svcErr := c.revokeConsent(context.Background(), "ou1", "c1", nil)

	s.Nil(svcErr)
}

// ----- buildConsentSearchURL nil filter -----

func (s *DefaultClientTestSuite) TestBuildConsentSearchURL_NilFilter_ReturnsBaseURL() {
	initClientRuntime(s.T())
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newDefaultClient(httpMock).(*defaultClient)

	u, svcErr := c.buildConsentSearchURL(context.Background(), nil)

	s.Nil(svcErr)
	s.Contains(u, testBaseURL)
	s.Contains(u, "consents")
	s.NotContains(u, "?")
}

// ----- listConsentPurposes uses clientIds query param (not name) -----

func (s *DefaultClientTestSuite) TestListConsentPurposes_WithGroupID_UsesClientIdsParam() {
	httpMock := httpmock.NewHTTPClientInterfaceMock(s.T())
	c := newTestClient(s.T(), httpMock)

	respBody := purposeListResponseDTO{
		Data: []purposeResponseDTO{{ID: "p1", Name: "Login", ClientID: "app-99"}},
	}
	httpMock.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
		query := req.URL.RawQuery
		return query != "" && req.URL.Query().Get("clientIds") == "app-99"
	})).Return(buildHTTPResponse(s.T(), http.StatusOK, respBody), nil)

	result, svcErr := c.listConsentPurposes(context.Background(), "ou1", "app-99")

	s.Nil(svcErr)
	s.Len(result, 1)
	s.Equal("app-99", result[0].GroupID)
}

// ----- purpose name + type helpers -----

func (s *DefaultClientTestSuite) TestNamespaceFromPurposeName() {
	s.Equal(NamespaceAttribute, NamespaceFromPurposeName("attributes:app1"))
	s.Equal(NamespaceAttribute, NamespaceFromPurposeName(AttributesPurposeName("app1")))
	s.Equal(NamespacePermission, NamespaceFromPurposeName("permissions:app1"))
	s.Equal(NamespacePermission, NamespaceFromPurposeName(PermissionsPurposeName("app1")))
	s.Equal(Namespace(""), NamespaceFromPurposeName("custom-purpose"), "names without a recognized prefix return empty")
	s.Equal(Namespace(""), NamespaceFromPurposeName(""))
}

func (s *DefaultClientTestSuite) TestAttributesPurposeName() {
	s.Equal("attributes:app1", AttributesPurposeName("app1"))
	s.Equal("attributes:", AttributesPurposeName(""))
}

func (s *DefaultClientTestSuite) TestFilterAttributePurposes() {
	input := []ConsentPurpose{
		{ID: "1", Namespace: NamespaceAttribute},
		{ID: "2", Namespace: NamespacePermission},
		{ID: "3", Namespace: ""},
		{ID: "4", Namespace: NamespaceAttribute},
	}
	got := FilterAttributePurposes(input)
	s.Len(got, 2)
	s.Equal("1", got[0].ID)
	s.Equal("4", got[1].ID)
}
