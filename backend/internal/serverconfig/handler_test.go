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

	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
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

var corsLayers = ServerConfigLayers{
	ReadOnly: declarative,
	Writable: corsValue,
	Merged:   mergedValue,
}

// --- GET /list ---

func (suite *HandlerTestSuite) TestHandleListServerConfigs_OK() {
	suite.mockService.EXPECT().ListConfigNames(mock.Anything).Return([]ConfigName{ConfigNameCORS}, nil)

	req := httptest.NewRequest(http.MethodGet, "/server-config", nil)
	w := httptest.NewRecorder()
	suite.handler.HandleListServerConfigs(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	var names []ConfigName
	suite.Require().NoError(json.Unmarshal(w.Body.Bytes(), &names))
	assert.Equal(suite.T(), []ConfigName{ConfigNameCORS}, names)
}

func (suite *HandlerTestSuite) TestHandleListServerConfigs_ServiceError() {
	suite.mockService.EXPECT().ListConfigNames(mock.Anything).Return(nil, &common.InternalServerError)

	req := httptest.NewRequest(http.MethodGet, "/server-config", nil)
	w := httptest.NewRecorder()
	suite.handler.HandleListServerConfigs(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
}

// --- GET /{name} ---

func (suite *HandlerTestSuite) TestHandleGetServerConfig_OK() {
	suite.mockService.EXPECT().GetConfig(mock.Anything, ConfigNameCORS).Return(corsLayers, nil)

	req := httptest.NewRequest(http.MethodGet, "/server-config/cors", nil)
	req.SetPathValue("name", string(ConfigNameCORS))
	w := httptest.NewRecorder()
	suite.handler.HandleGetServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	resp := decodeLayersResponse(suite, w.Body.Bytes())
	assert.JSONEq(suite.T(), string(declarative), string(resp.ReadOnly))
	assert.JSONEq(suite.T(), string(corsValue), string(resp.Writable))
	assert.JSONEq(suite.T(), string(mergedValue), string(resp.Merged))
}

// rawLayersResponse mirrors the GET/PUT response as raw bytes, so tests assert on the wire JSON rather
// than the decoded any-typed fields.
type rawLayersResponse struct {
	ReadOnly json.RawMessage `json:"readOnly"`
	Writable json.RawMessage `json:"writable"`
	Merged   json.RawMessage `json:"merged"`
}

func decodeLayersResponse(suite *HandlerTestSuite, body []byte) rawLayersResponse {
	var resp rawLayersResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	return resp
}

func (suite *HandlerTestSuite) TestHandleGetServerConfig_UnsupportedName() {
	suite.mockService.EXPECT().GetConfig(mock.Anything, ConfigName("bogus")).
		Return(ServerConfigLayers{}, &ErrorUnsupportedConfigName)

	req := httptest.NewRequest(http.MethodGet, "/server-config/bogus", nil)
	req.SetPathValue("name", "bogus")
	w := httptest.NewRecorder()
	suite.handler.HandleGetServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), ErrorUnsupportedConfigName.Code, suite.decodeErrorCode(w.Body.Bytes()))
}

func (suite *HandlerTestSuite) TestHandleGetServerConfig_ServiceError() {
	suite.mockService.EXPECT().GetConfig(mock.Anything, ConfigNameCORS).
		Return(ServerConfigLayers{}, &common.InternalServerError)

	req := httptest.NewRequest(http.MethodGet, "/server-config/cors", nil)
	req.SetPathValue("name", string(ConfigNameCORS))
	w := httptest.NewRecorder()
	suite.handler.HandleGetServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
}

// --- PUT /{name} ---

func (suite *HandlerTestSuite) TestHandleUpdateServerConfig_OK() {
	suite.mockService.EXPECT().
		SetConfig(mock.Anything, ConfigNameCORS, json.RawMessage(`["https://app.example.com"]`)).Return(nil)
	suite.mockService.EXPECT().GetConfig(mock.Anything, ConfigNameCORS).Return(corsLayers, nil)

	req := httptest.NewRequest(http.MethodPut, "/server-config/cors",
		strings.NewReader(`["https://app.example.com"]`))
	req.SetPathValue("name", string(ConfigNameCORS))
	w := httptest.NewRecorder()
	suite.handler.HandleUpdateServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	resp := decodeLayersResponse(suite, w.Body.Bytes())
	assert.JSONEq(suite.T(), string(mergedValue), string(resp.Merged))
}

func (suite *HandlerTestSuite) TestHandleUpdateServerConfig_MalformedBody() {
	req := httptest.NewRequest(http.MethodPut, "/server-config/cors", strings.NewReader(`not json`))
	req.SetPathValue("name", string(ConfigNameCORS))
	w := httptest.NewRecorder()
	suite.handler.HandleUpdateServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), ErrorInvalidRequestFormat.Code, suite.decodeErrorCode(w.Body.Bytes()))
	suite.mockService.AssertNotCalled(suite.T(), "SetConfig", mock.Anything, mock.Anything, mock.Anything)
}

func (suite *HandlerTestSuite) TestHandleUpdateServerConfig_InvalidValue() {
	suite.mockService.EXPECT().SetConfig(mock.Anything, ConfigNameCORS, mock.Anything).
		Return(&ErrorInvalidConfigValue)

	req := httptest.NewRequest(http.MethodPut, "/server-config/cors", strings.NewReader(`["*"]`))
	req.SetPathValue("name", string(ConfigNameCORS))
	w := httptest.NewRecorder()
	suite.handler.HandleUpdateServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Equal(suite.T(), ErrorInvalidConfigValue.Code, suite.decodeErrorCode(w.Body.Bytes()))
}

func (suite *HandlerTestSuite) TestHandleUpdateServerConfig_GetAfterSetError() {
	suite.mockService.EXPECT().SetConfig(mock.Anything, ConfigNameCORS, mock.Anything).Return(nil)
	suite.mockService.EXPECT().GetConfig(mock.Anything, ConfigNameCORS).
		Return(ServerConfigLayers{}, &common.InternalServerError)

	req := httptest.NewRequest(http.MethodPut, "/server-config/cors",
		strings.NewReader(`["https://app.example.com"]`))
	req.SetPathValue("name", string(ConfigNameCORS))
	w := httptest.NewRecorder()
	suite.handler.HandleUpdateServerConfig(w, req)

	assert.Equal(suite.T(), http.StatusInternalServerError, w.Code)
}
