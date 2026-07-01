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

package credential

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type ConfigurationHandlerTestSuite struct {
	suite.Suite
	service *CredentialConfigurationServiceInterfaceMock
	handler *configurationHandler
}

func TestConfigurationHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigurationHandlerTestSuite))
}

func (s *ConfigurationHandlerTestSuite) SetupTest() {
	s.service = NewCredentialConfigurationServiceInterfaceMock(s.T())
	s.handler = newConfigurationHandler(s.service)
}

func (s *ConfigurationHandlerTestSuite) TestHandleCreate_Success() {
	dto := &CredentialConfigurationDTO{ID: "cfg-1", Handle: "eudi-pid", VCT: "v", Format: DefaultCredentialFormat}
	s.service.EXPECT().CreateCredentialConfiguration(mock.Anything, mock.Anything).Return(dto, nil)

	req := httptest.NewRequest(http.MethodPost, configurationsPath,
		strings.NewReader(`{"handle":"eudi-pid","vct":"v","ouId":"ou-1"}`))
	rec := httptest.NewRecorder()
	s.handler.HandleCreate(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleCreate_InvalidBody() {
	req := httptest.NewRequest(http.MethodPost, configurationsPath, strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	s.handler.HandleCreate(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleCreate_ServiceError() {
	s.service.EXPECT().CreateCredentialConfiguration(mock.Anything, mock.Anything).
		Return(nil, &ErrorConfigurationAlreadyExists)

	req := httptest.NewRequest(http.MethodPost, configurationsPath,
		strings.NewReader(`{"handle":"eudi-pid","vct":"v"}`))
	rec := httptest.NewRecorder()
	s.handler.HandleCreate(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleList_Success() {
	s.service.EXPECT().ListCredentialConfigurationSummaries(mock.Anything).
		Return([]CredentialConfigurationList{{ID: "cfg-1", Handle: "h"}}, nil)

	req := httptest.NewRequest(http.MethodGet, configurationsPath, nil)
	rec := httptest.NewRecorder()
	s.handler.HandleList(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleList_ServiceError() {
	s.service.EXPECT().ListCredentialConfigurationSummaries(mock.Anything).
		Return(nil, &tidcommon.InternalServerError)

	req := httptest.NewRequest(http.MethodGet, configurationsPath, nil)
	rec := httptest.NewRecorder()
	s.handler.HandleList(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleGet_Success() {
	dto := &CredentialConfigurationDTO{ID: "cfg-1", Handle: "h", VCT: "v", Format: DefaultCredentialFormat}
	s.service.EXPECT().GetCredentialConfiguration(mock.Anything, "cfg-1").Return(dto, nil)

	req := httptest.NewRequest(http.MethodGet, configurationsPath+"/cfg-1", nil)
	req.SetPathValue("id", "cfg-1")
	rec := httptest.NewRecorder()
	s.handler.HandleGet(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleGet_MissingID() {
	req := httptest.NewRequest(http.MethodGet, configurationsPath+"/", nil)
	rec := httptest.NewRecorder()
	s.handler.HandleGet(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleGet_NotFound() {
	s.service.EXPECT().GetCredentialConfiguration(mock.Anything, "missing").
		Return(nil, &ErrorConfigurationNotFound)

	req := httptest.NewRequest(http.MethodGet, configurationsPath+"/missing", nil)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	s.handler.HandleGet(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleUpdate_Success() {
	dto := &CredentialConfigurationDTO{ID: "cfg-1", Handle: "h", VCT: "v", Format: DefaultCredentialFormat}
	s.service.EXPECT().UpdateCredentialConfiguration(mock.Anything, "cfg-1", mock.Anything).Return(dto, nil)

	req := httptest.NewRequest(http.MethodPut, configurationsPath+"/cfg-1",
		strings.NewReader(`{"handle":"h","vct":"v"}`))
	req.SetPathValue("id", "cfg-1")
	rec := httptest.NewRecorder()
	s.handler.HandleUpdate(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleUpdate_MissingID() {
	req := httptest.NewRequest(http.MethodPut, configurationsPath+"/", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	s.handler.HandleUpdate(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleUpdate_InvalidBody() {
	req := httptest.NewRequest(http.MethodPut, configurationsPath+"/cfg-1", strings.NewReader("not json"))
	req.SetPathValue("id", "cfg-1")
	rec := httptest.NewRecorder()
	s.handler.HandleUpdate(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleUpdate_Immutable() {
	s.service.EXPECT().UpdateCredentialConfiguration(mock.Anything, "cfg-1", mock.Anything).
		Return(nil, &ErrorConfigurationImmutable)

	req := httptest.NewRequest(http.MethodPut, configurationsPath+"/cfg-1",
		strings.NewReader(`{"handle":"h","vct":"v"}`))
	req.SetPathValue("id", "cfg-1")
	rec := httptest.NewRecorder()
	s.handler.HandleUpdate(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleDelete_Success() {
	s.service.EXPECT().DeleteCredentialConfiguration(mock.Anything, "cfg-1").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, configurationsPath+"/cfg-1", nil)
	req.SetPathValue("id", "cfg-1")
	rec := httptest.NewRecorder()
	s.handler.HandleDelete(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleDelete_MissingID() {
	req := httptest.NewRequest(http.MethodDelete, configurationsPath+"/", nil)
	rec := httptest.NewRecorder()
	s.handler.HandleDelete(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestHandleDelete_ServiceError() {
	s.service.EXPECT().DeleteCredentialConfiguration(mock.Anything, "cfg-1").
		Return(&tidcommon.InternalServerError)

	req := httptest.NewRequest(http.MethodDelete, configurationsPath+"/cfg-1", nil)
	req.SetPathValue("id", "cfg-1")
	rec := httptest.NewRecorder()
	s.handler.HandleDelete(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *ConfigurationHandlerTestSuite) TestRequestToDTOSanitizes() {
	validity := 120
	req := &credentialConfigurationRequest{
		Handle:      "  eudi-pid  ",
		OUID:        " ou-1 ",
		OUHandle:    " default ",
		Name:        " EUDI PID ",
		Description: " A PID credential ",
		Format:      " dc+sd-jwt ",
		VCT:         " v ",
		Claims: []ClaimMapping{
			{Name: "  given_name  ", DisplayName: "  Given Name  "},
			{Name: "   ", DisplayName: "dropped"},
		},
		Display:         &CredentialDisplay{Locale: " en-US ", LogoURI: " uri "},
		ValiditySeconds: &validity,
	}

	dto := requestToDTO(req)
	s.Equal("eudi-pid", dto.Handle)
	s.Equal("ou-1", dto.OUID)
	s.Equal("default", dto.OUHandle)
	s.Equal("EUDI PID", dto.Name)
	s.Equal("A PID credential", dto.Description)
	s.Equal("dc+sd-jwt", dto.Format)
	s.Equal("v", dto.VCT)
	s.Require().Len(dto.Claims, 1)
	s.Equal("given_name", dto.Claims[0].Name)
	s.Equal("Given Name", dto.Claims[0].DisplayName)
	s.Require().NotNil(dto.Display)
	s.Equal("en-US", dto.Display.Locale)
	s.Equal(120, *dto.ValiditySeconds)
}

func (s *ConfigurationHandlerTestSuite) TestSanitizeClaimsAllEmpty() {
	out := sanitizeClaims([]ClaimMapping{{Name: "   "}})
	s.Nil(out)
}

func (s *ConfigurationHandlerTestSuite) TestSanitizeDisplayNil() {
	s.Nil(sanitizeDisplay(nil))
}

func (s *ConfigurationHandlerTestSuite) TestSanitizeDisplayAllEmpty() {
	s.Nil(sanitizeDisplay(&CredentialDisplay{Locale: " ", LogoURI: " "}))
}

func (s *ConfigurationHandlerTestSuite) TestWriteConfigurationErrorServerError() {
	rec := httptest.NewRecorder()
	writeConfigurationError(context.Background(), rec, &tidcommon.InternalServerError)
	s.Equal(http.StatusInternalServerError, rec.Code)
}
