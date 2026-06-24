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

package serverconfig

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

type HandlerTestSuite struct {
	suite.Suite
	mockService *ServerConfigServiceMock
	handler     *serverConfigHandler
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (suite *HandlerTestSuite) SetupTest() {
	suite.mockService = NewServerConfigServiceMock(suite.T())
	suite.handler = newServerConfigHandler(suite.mockService)
}

func (suite *HandlerTestSuite) decodeErrorCode(body []byte) string {
	var errResp struct {
		Code string `json:"code"`
	}
	suite.Require().NoError(json.Unmarshal(body, &errResp))
	return errResp.Code
}

// --- GET ---

func (suite *HandlerTestSuite) TestHandleGetServerConfig_OK() {
	suite.mockService.EXPECT().ListConfigs(mock.Anything).
		Return(map[ConfigName]json.RawMessage{ConfigNameCORS: corsValue}, nil)

	req := httptest.NewRequest(http.MethodGet, "/server-config", nil)
	w := httptest.NewRecorder()
	suite.handler.HandleGetServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	var resp ServerConfigResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(suite.T(), corsValue, resp.CORS)
}

func (suite *HandlerTestSuite) TestHandleGetServerConfig_ServiceError() {
	suite.mockService.EXPECT().ListConfigs(mock.Anything).Return(nil, &serviceerror.InternalServerError)

	req := httptest.NewRequest(http.MethodGet, "/server-config", nil)
	w := httptest.NewRecorder()
	suite.handler.HandleGetServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, suite.decodeErrorCode(w.Body.Bytes()))
}

func (suite *HandlerTestSuite) TestHandleGetServerConfig_NotFoundMapsTo404() {
	suite.mockService.EXPECT().ListConfigs(mock.Anything).Return(nil, &ErrorConfigNotFound)

	req := httptest.NewRequest(http.MethodGet, "/server-config", nil)
	w := httptest.NewRecorder()
	suite.handler.HandleGetServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
	assert.Equal(suite.T(), ErrorConfigNotFound.Code, suite.decodeErrorCode(w.Body.Bytes()))
}

func (suite *HandlerTestSuite) TestHandleGetServerConfig_EmptyReturnsCorsArray() {
	suite.mockService.EXPECT().ListConfigs(mock.Anything).Return(map[ConfigName]json.RawMessage{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/server-config", nil)
	w := httptest.NewRecorder()
	suite.handler.HandleGetServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Contains(suite.T(), w.Body.String(), `"cors"`)
	var resp ServerConfigResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(suite.T(), json.RawMessage("[]"), resp.CORS)
}

func (suite *HandlerTestSuite) TestEmptyConfigValue() {
	assert.Equal(suite.T(), json.RawMessage("[]"), emptyConfigValue(ConfigNameCORS))
	assert.Equal(suite.T(), json.RawMessage("null"), emptyConfigValue(ConfigName("unknown")))
}

// --- GET by name ---

func (suite *HandlerTestSuite) TestHandleGetServerConfigByName_OK() {
	suite.mockService.EXPECT().GetConfig(mock.Anything, ConfigNameCORS).Return(corsValue, nil)

	req := httptest.NewRequest(http.MethodGet, "/server-config/cors", nil)
	req.SetPathValue("name", string(ConfigNameCORS))
	w := httptest.NewRecorder()
	suite.handler.HandleGetServerConfigByName(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	var resp ServerConfigResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(suite.T(), corsValue, resp.CORS)
}

func (suite *HandlerTestSuite) TestHandleGetServerConfigByName_UnsupportedName() {
	suite.mockService.EXPECT().GetConfig(mock.Anything, ConfigName("bogus")).
		Return(nil, &ErrorUnsupportedConfigName)

	req := httptest.NewRequest(http.MethodGet, "/server-config/bogus", nil)
	req.SetPathValue("name", "bogus")
	w := httptest.NewRecorder()
	suite.handler.HandleGetServerConfigByName(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), ErrorUnsupportedConfigName.Code, suite.decodeErrorCode(w.Body.Bytes()))
}

func (suite *HandlerTestSuite) TestHandleGetServerConfigByName_UnsetReturnsEmptyDefault() {
	suite.mockService.EXPECT().GetConfig(mock.Anything, ConfigNameCORS).Return(nil, &ErrorConfigNotFound)

	req := httptest.NewRequest(http.MethodGet, "/server-config/cors", nil)
	req.SetPathValue("name", string(ConfigNameCORS))
	w := httptest.NewRecorder()
	suite.handler.HandleGetServerConfigByName(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	var resp ServerConfigResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(suite.T(), json.RawMessage("[]"), resp.CORS)
}

// --- PUT ---

func (suite *HandlerTestSuite) TestHandleUpdateServerConfig_OK() {
	suite.mockService.EXPECT().SetConfigs(mock.Anything, mock.Anything).Return(nil)
	suite.mockService.EXPECT().ListConfigs(mock.Anything).
		Return(map[ConfigName]json.RawMessage{ConfigNameCORS: corsValue}, nil)

	req := httptest.NewRequest(http.MethodPut, "/server-config",
		strings.NewReader(`{"cors":["https://app.example.com"]}`))
	w := httptest.NewRecorder()
	suite.handler.HandleUpdateServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	var resp ServerConfigResponse
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(suite.T(), corsValue, resp.CORS)
}

func (suite *HandlerTestSuite) TestHandleUpdateServerConfig_DecodeError() {
	req := httptest.NewRequest(http.MethodPut, "/server-config", strings.NewReader(`not json`))
	w := httptest.NewRecorder()
	suite.handler.HandleUpdateServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), ErrorInvalidRequestFormat.Code, suite.decodeErrorCode(w.Body.Bytes()))
	suite.mockService.AssertNotCalled(suite.T(), "SetConfigs", mock.Anything, mock.Anything)
}

func (suite *HandlerTestSuite) TestHandleUpdateServerConfig_ValidatorError() {
	suite.mockService.EXPECT().SetConfigs(mock.Anything, mock.Anything).Return(&ErrorInvalidConfigValue)

	req := httptest.NewRequest(http.MethodPut, "/server-config",
		strings.NewReader(`{"cors":["*"]}`))
	w := httptest.NewRecorder()
	suite.handler.HandleUpdateServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), ErrorInvalidConfigValue.Code, suite.decodeErrorCode(w.Body.Bytes()))
}

func (suite *HandlerTestSuite) TestHandleUpdateServerConfig_UnknownKeyIgnored() {
	// Only the cors section reaches the service; the unknown key is dropped on decode.
	suite.mockService.EXPECT().
		SetConfigs(mock.Anything, mock.MatchedBy(func(m map[ConfigName]json.RawMessage) bool {
			_, hasCORS := m[ConfigNameCORS]
			return len(m) == 1 && hasCORS
		})).Return(nil)
	suite.mockService.EXPECT().ListConfigs(mock.Anything).
		Return(map[ConfigName]json.RawMessage{ConfigNameCORS: corsValue}, nil)

	req := httptest.NewRequest(http.MethodPut, "/server-config",
		strings.NewReader(`{"cors":["https://app.example.com"],"bogus":1}`))
	w := httptest.NewRecorder()
	suite.handler.HandleUpdateServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
}

func (suite *HandlerTestSuite) TestHandleUpdateServerConfig_ListAfterSetError() {
	suite.mockService.EXPECT().SetConfigs(mock.Anything, mock.Anything).Return(nil)
	suite.mockService.EXPECT().ListConfigs(mock.Anything).Return(nil, &serviceerror.InternalServerError)

	req := httptest.NewRequest(http.MethodPut, "/server-config",
		strings.NewReader(`{"cors":["https://app.example.com"]}`))
	w := httptest.NewRecorder()
	suite.handler.HandleUpdateServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
}

func (suite *HandlerTestSuite) TestHandleUpdateServerConfig_EmptyBody() {
	// No sections present → SetConfigs receives an empty map (no-op), still 200.
	suite.mockService.EXPECT().SetConfigs(mock.Anything, map[ConfigName]json.RawMessage{}).Return(nil)
	suite.mockService.EXPECT().ListConfigs(mock.Anything).
		Return(map[ConfigName]json.RawMessage{}, nil)

	req := httptest.NewRequest(http.MethodPut, "/server-config", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	suite.handler.HandleUpdateServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
}
