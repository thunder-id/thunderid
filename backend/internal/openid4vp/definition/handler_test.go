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

package definition

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type DefinitionHandlerTestSuite struct {
	suite.Suite
	service *PresentationDefinitionServiceInterfaceMock
	handler *definitionHandler
}

func TestDefinitionHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(DefinitionHandlerTestSuite))
}

func (s *DefinitionHandlerTestSuite) SetupTest() {
	s.service = NewPresentationDefinitionServiceInterfaceMock(s.T())
	s.handler = newDefinitionHandler(s.service)
}

func (s *DefinitionHandlerTestSuite) TestNewDefinitionHandler() {
	s.NotNil(s.handler)
	s.Equal(s.service, s.handler.service)
}

func (s *DefinitionHandlerTestSuite) TestHandleCreateSuccess() {
	body := `{"handle":"eudi-pid","vct":"urn:eudi:pid:1","ouId":"ou-1"}`
	s.service.EXPECT().CreatePresentationDefinition(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, dto *PresentationDefinitionDTO) (
			*PresentationDefinitionDTO, *tidcommon.ServiceError,
		) {
			dto.ID = "def-1"
			return dto, nil
		})

	req := httptest.NewRequest(http.MethodPost, definitionsPath, strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.handler.HandleCreate(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Contains(rec.Body.String(), "def-1")
}

func (s *DefinitionHandlerTestSuite) TestHandleCreateInvalidBody() {
	req := httptest.NewRequest(http.MethodPost, definitionsPath, strings.NewReader("not-json"))
	rec := httptest.NewRecorder()
	s.handler.HandleCreate(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Contains(rec.Body.String(), ErrorDefinitionInvalidRequest.Code)
}

func (s *DefinitionHandlerTestSuite) TestHandleCreateServiceError() {
	s.service.EXPECT().CreatePresentationDefinition(mock.Anything, mock.Anything).
		Return(nil, &ErrorDefinitionAlreadyExists)

	req := httptest.NewRequest(http.MethodPost, definitionsPath, strings.NewReader(`{"handle":"h","vct":"v"}`))
	rec := httptest.NewRecorder()
	s.handler.HandleCreate(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Contains(rec.Body.String(), ErrorDefinitionAlreadyExists.Code)
}

func (s *DefinitionHandlerTestSuite) TestHandleListSuccess() {
	s.service.EXPECT().ListPresentationDefinitionSummaries(mock.Anything).
		Return([]PresentationDefinitionList{{ID: "def-1", Handle: "h"}}, nil)

	req := httptest.NewRequest(http.MethodGet, definitionsPath, nil)
	rec := httptest.NewRecorder()
	s.handler.HandleList(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Contains(rec.Body.String(), "def-1")
}

func (s *DefinitionHandlerTestSuite) TestHandleListServiceError() {
	s.service.EXPECT().ListPresentationDefinitionSummaries(mock.Anything).
		Return(nil, &tidcommon.InternalServerError)

	req := httptest.NewRequest(http.MethodGet, definitionsPath, nil)
	rec := httptest.NewRecorder()
	s.handler.HandleList(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *DefinitionHandlerTestSuite) TestHandleGetSuccess() {
	s.service.EXPECT().GetPresentationDefinition(mock.Anything, "def-1").
		Return(&PresentationDefinitionDTO{ID: "def-1", Handle: "h", VCT: "v"}, nil)

	req := httptest.NewRequest(http.MethodGet, definitionsPath+"/def-1", nil)
	req.SetPathValue("id", "def-1")
	rec := httptest.NewRecorder()
	s.handler.HandleGet(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Contains(rec.Body.String(), "def-1")
}

func (s *DefinitionHandlerTestSuite) TestHandleGetMissingID() {
	req := httptest.NewRequest(http.MethodGet, definitionsPath+"/", nil)
	rec := httptest.NewRecorder()
	s.handler.HandleGet(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Contains(rec.Body.String(), ErrorDefinitionInvalidRequest.Code)
}

func (s *DefinitionHandlerTestSuite) TestHandleGetNotFound() {
	s.service.EXPECT().GetPresentationDefinition(mock.Anything, "missing").
		Return(nil, &ErrorDefinitionNotFound)

	req := httptest.NewRequest(http.MethodGet, definitionsPath+"/missing", nil)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	s.handler.HandleGet(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *DefinitionHandlerTestSuite) TestHandleUpdateSuccess() {
	s.service.EXPECT().UpdatePresentationDefinition(mock.Anything, "def-1", mock.Anything).RunAndReturn(
		func(_ context.Context, id string, dto *PresentationDefinitionDTO) (
			*PresentationDefinitionDTO, *tidcommon.ServiceError) {
			dto.ID = id
			return dto, nil
		})

	req := httptest.NewRequest(http.MethodPut, definitionsPath+"/def-1",
		strings.NewReader(`{"handle":"h","vct":"v"}`))
	req.SetPathValue("id", "def-1")
	rec := httptest.NewRecorder()
	s.handler.HandleUpdate(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Contains(rec.Body.String(), "def-1")
}

func (s *DefinitionHandlerTestSuite) TestHandleUpdateMissingID() {
	req := httptest.NewRequest(http.MethodPut, definitionsPath+"/", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	s.handler.HandleUpdate(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *DefinitionHandlerTestSuite) TestHandleUpdateInvalidBody() {
	req := httptest.NewRequest(http.MethodPut, definitionsPath+"/def-1", strings.NewReader("bad"))
	req.SetPathValue("id", "def-1")
	rec := httptest.NewRecorder()
	s.handler.HandleUpdate(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Contains(rec.Body.String(), ErrorDefinitionInvalidRequest.Code)
}

func (s *DefinitionHandlerTestSuite) TestHandleUpdateServiceError() {
	s.service.EXPECT().UpdatePresentationDefinition(mock.Anything, "def-1", mock.Anything).
		Return(nil, &ErrorDefinitionImmutable)

	req := httptest.NewRequest(http.MethodPut, definitionsPath+"/def-1",
		strings.NewReader(`{"handle":"h","vct":"v"}`))
	req.SetPathValue("id", "def-1")
	rec := httptest.NewRecorder()
	s.handler.HandleUpdate(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
}

func (s *DefinitionHandlerTestSuite) TestHandleDeleteSuccess() {
	s.service.EXPECT().DeletePresentationDefinition(mock.Anything, "def-1").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, definitionsPath+"/def-1", nil)
	req.SetPathValue("id", "def-1")
	rec := httptest.NewRecorder()
	s.handler.HandleDelete(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DefinitionHandlerTestSuite) TestHandleDeleteMissingID() {
	req := httptest.NewRequest(http.MethodDelete, definitionsPath+"/", nil)
	rec := httptest.NewRecorder()
	s.handler.HandleDelete(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *DefinitionHandlerTestSuite) TestHandleDeleteServiceError() {
	s.service.EXPECT().DeletePresentationDefinition(mock.Anything, "def-1").
		Return(&ErrorDefinitionImmutable)

	req := httptest.NewRequest(http.MethodDelete, definitionsPath+"/def-1", nil)
	req.SetPathValue("id", "def-1")
	rec := httptest.NewRecorder()
	s.handler.HandleDelete(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
}

func (s *DefinitionHandlerTestSuite) TestRequestToDTOSanitizes() {
	enforce := true
	dto := requestToDTO(&presentationDefinitionRequest{
		Handle:               "  eudi-pid  ",
		OUID:                 "ou-1",
		DisplayName:          "  EUDI  ",
		VCT:                  " v ",
		Format:               DefaultCredentialFormat,
		RequestedClaims:      []string{" given_name ", "family_name"},
		MandatoryClaims:      []string{"given_name"},
		ClaimValues:          map[string][]string{" address.country ": {" DE ", "", "AT"}, "  ": {"x"}, "empty": {""}},
		EnforceTrustedIssuer: &enforce,
		TrustedAuthorities:   []string{" root-a "},
	})

	s.Equal("eudi-pid", dto.Handle)
	s.Equal("EUDI", dto.DisplayName)
	s.Equal("v", dto.VCT)
	s.Equal([]string{"given_name", "family_name"}, dto.RequestedClaims)
	s.Equal([]string{"DE", "AT"}, dto.ClaimValues["address.country"])
	// Entry with blank path is dropped; entry with only-empty values is dropped.
	s.Len(dto.ClaimValues, 1)
	s.Equal([]string{"root-a"}, dto.TrustedAuthorities)
	s.Require().NotNil(dto.EnforceTrustedIssuer)
	s.True(*dto.EnforceTrustedIssuer)
}

func (s *DefinitionHandlerTestSuite) TestSanitizeStrings() {
	s.Equal([]string{"a", "b"}, sanitizeStrings([]string{" a ", "b"}))
	s.Empty(sanitizeStrings(nil))
}

func (s *DefinitionHandlerTestSuite) TestSanitizeClaimValuesEmpty() {
	s.Nil(sanitizeClaimValues(nil))
	s.Nil(sanitizeClaimValues(map[string][]string{}))
	// Map that sanitizes down to nothing returns nil.
	s.Nil(sanitizeClaimValues(map[string][]string{"  ": {"x"}, "k": {""}}))
}

func (s *DefinitionHandlerTestSuite) TestWriteDefinitionErrorServerError() {
	rec := httptest.NewRecorder()
	writeDefinitionError(context.Background(), rec, &tidcommon.InternalServerError)
	s.Equal(http.StatusInternalServerError, rec.Code)
}
