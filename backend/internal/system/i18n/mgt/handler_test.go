/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package mgt

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

type I18nHandlerTestSuite struct {
	suite.Suite
	mockService *I18nServiceInterfaceMock
	handler     *i18nHandler
}

func TestI18nHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(I18nHandlerTestSuite))
}

func (suite *I18nHandlerTestSuite) SetupTest() {
	suite.mockService = NewI18nServiceInterfaceMock(suite.T())
	suite.handler = newI18nHandler(suite.mockService)
}

func (suite *I18nHandlerTestSuite) TestHandleListLanguages_Success() {
	expectedLangs := []string{"en-US", "fr-FR"}
	suite.mockService.On("ListLanguages").Return(expectedLangs, nil)

	req := httptest.NewRequest(http.MethodGet, "/i18n/languages", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleListLanguages(w, req)

	suite.Equal(http.StatusOK, w.Code)

	var response LanguageListResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	suite.NoError(err)
	suite.Len(response.Languages, 2)
	suite.Contains(response.Languages, "en-US")
}

func (suite *I18nHandlerTestSuite) TestHandleListLanguages_ServiceError() {
	suite.mockService.On("ListLanguages").Return(nil, &serviceerror.InternalServerError)

	req := httptest.NewRequest(http.MethodGet, "/i18n/languages", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleListLanguages(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *I18nHandlerTestSuite) TestHandleResolveTranslationsByLanguage_Success() {
	expectedResp := &LanguageTranslationsResponse{
		Language:     "en-US",
		TotalResults: 10,
		Translations: map[string]map[string]string{
			"common": {"welcome": "Welcome"},
		},
	}
	suite.mockService.On("ResolveTranslations", "en-US", "common").
		Return(expectedResp, nil)

	req := httptest.NewRequest(http.MethodGet, "/i18n/languages/en-US/translations/resolve?namespace=common", nil)
	req.SetPathValue("language", "en-US")
	w := httptest.NewRecorder()

	suite.handler.HandleResolveTranslationsByLanguage(w, req)

	suite.Equal(http.StatusOK, w.Code)
	var response LanguageTranslationsResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	suite.NoError(err)
	suite.Equal("en-US", response.Language)
}

func (suite *I18nHandlerTestSuite) TestHandleResolveTranslationsByLanguage_ServiceError() {
	suite.mockService.On("ResolveTranslations", "en-US", "").
		Return(nil, &ErrorInvalidLanguage)

	req := httptest.NewRequest(http.MethodGet, "/i18n/languages/en-US/translations/resolve", nil)
	req.SetPathValue("language", "en-US") // Assuming no query params
	w := httptest.NewRecorder()

	suite.handler.HandleResolveTranslationsByLanguage(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *I18nHandlerTestSuite) TestHandleSetOverrideTranslationsByLanguage_Success() {
	inputTranslations := map[string]map[string]string{
		"common": {"key": "value"},
	}
	request := SetTranslationsRequest{
		Translations: inputTranslations,
	}

	expectedResp := &LanguageTranslationsResponse{
		Language:     "en-US",
		TotalResults: 1,
		Translations: inputTranslations,
	}

	suite.mockService.On("SetTranslationOverrides", "en-US", inputTranslations).Return(expectedResp, nil)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/i18n/languages/en-US/translations", bytes.NewBuffer(body))
	req.SetPathValue("language", "en-US")
	w := httptest.NewRecorder()

	suite.handler.HandleSetOverrideTranslationsByLanguage(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *I18nHandlerTestSuite) TestHandleSetOverrideTranslationsByLanguage_InvalidJSON() {
	req := httptest.NewRequest(http.MethodPost, "/i18n/languages/en-US/translations",
		bytes.NewBufferString("invalid"))
	req.SetPathValue("language", "en-US")
	w := httptest.NewRecorder()

	suite.handler.HandleSetOverrideTranslationsByLanguage(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
}

func (suite *I18nHandlerTestSuite) TestHandleClearOverrideTranslationsByLanguage_Success() {
	suite.mockService.On("ClearTranslationOverrides", "en-US").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/i18n/languages/en-US/translations", nil)
	req.SetPathValue("language", "en-US")
	w := httptest.NewRecorder()

	suite.handler.HandleClearOverrideTranslationsByLanguage(w, req)

	suite.Equal(http.StatusNoContent, w.Code)
}

func (suite *I18nHandlerTestSuite) TestHandleResolveTranslation_Success() {
	expectedResp := &TranslationResponse{
		Language:  "en-US",
		Namespace: "ns",
		Key:       "key",
		Value:     "val",
	}
	suite.mockService.On("ResolveTranslationsForKey", "en-US", "ns", "key").Return(expectedResp, nil)

	req := httptest.NewRequest(http.MethodGet, "/i18n/languages/en-US/translations/ns/ns/keys/key/resolve", nil)
	req.SetPathValue("language", "en-US")
	req.SetPathValue("namespace", "ns")
	req.SetPathValue("key", "key")
	w := httptest.NewRecorder()

	suite.handler.HandleResolveTranslation(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *I18nHandlerTestSuite) TestHandleSetOverrideTranslation_Success() {
	request := SetTranslationRequest{Value: "new val"}
	expectedResp := &TranslationResponse{
		Language:  "en-US",
		Namespace: "ns",
		Key:       "key",
		Value:     "new val",
	}

	suite.mockService.On("SetTranslationOverrideForKey", "en-US", "ns", "key", "new val").
		Return(expectedResp, nil)

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/i18n/languages/en-US/translations/ns/ns/keys/key",
		bytes.NewBuffer(body))
	req.SetPathValue("language", "en-US")
	req.SetPathValue("namespace", "ns")
	req.SetPathValue("key", "key")
	w := httptest.NewRecorder()

	suite.handler.HandleSetOverrideTranslation(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *I18nHandlerTestSuite) TestHandleClearOverrideTranslation_Success() {
	suite.mockService.On("ClearTranslationOverrideForKey", "en-US", "ns", "key").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/i18n/languages/en-US/translations/ns/ns/keys/key", nil)
	req.SetPathValue("language", "en-US")
	req.SetPathValue("namespace", "ns")
	req.SetPathValue("key", "key")
	w := httptest.NewRecorder()

	suite.handler.HandleClearOverrideTranslation(w, req)

	suite.Equal(http.StatusNoContent, w.Code)
}

func (suite *I18nHandlerTestSuite) TestHandleError_NotFound() {
	// Testing manual error construction/mapping in handleError
	svcErr := &serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "I18N-1006", // Assuming this is TranslationNotFound
	}

	w := httptest.NewRecorder()
	handleError(w, svcErr)

	suite.Equal(http.StatusNotFound, w.Code)
}
