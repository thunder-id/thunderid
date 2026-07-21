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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type DefaultResourceServerAPITestSuite struct {
	suite.Suite
	adminClient *http.Client
}

func TestDefaultResourceServerAPITestSuite(t *testing.T) {
	suite.Run(t, new(DefaultResourceServerAPITestSuite))
}

func (suite *DefaultResourceServerAPITestSuite) SetupSuite() {
	suite.adminClient = testutils.GetHTTPClient()
}

func (suite *DefaultResourceServerAPITestSuite) SetupTest()     { suite.clear() }
func (suite *DefaultResourceServerAPITestSuite) TearDownSuite() { suite.clear() }

func (suite *DefaultResourceServerAPITestSuite) TestListIncludesSection() {
	status, body := suite.get(serverConfigURL)
	suite.Require().Equal(http.StatusOK, status)

	var names []string
	suite.Require().NoError(json.Unmarshal(body, &names))
	suite.Contains(names, "defaultResourceServer")
}

func (suite *DefaultResourceServerAPITestSuite) TestPutExistingIDPersistsAndReads() {
	status, _ := suite.put(`{"resourceServerId":"` + systemResourceServerID + `"}`)
	suite.Require().Equal(http.StatusOK, status)

	layers := suite.getLayers()
	suite.Equal(systemResourceServerID, layers.Writable.ResourceServerID)
	suite.Equal(systemResourceServerID, layers.Merged.ResourceServerID)
}

func (suite *DefaultResourceServerAPITestSuite) TestPutUnknownIDReturns400AndDoesNotPersist() {
	status, body := suite.put(`{"resourceServerId":"00000000-0000-0000-0000-000000000000"}`)
	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1003", suite.errorCode(body))

	suite.Empty(suite.getLayers().Writable.ResourceServerID)
}

func (suite *DefaultResourceServerAPITestSuite) TestClearIsAccepted() {
	suite.Require().Equal(http.StatusOK, suite.mustPut(`{"resourceServerId":"`+systemResourceServerID+`"}`))

	status, _ := suite.put(`{"resourceServerId":""}`)
	suite.Require().Equal(http.StatusOK, status)
	suite.Empty(suite.getLayers().Writable.ResourceServerID)
}

func (suite *DefaultResourceServerAPITestSuite) clear() {
	suite.Require().Equal(http.StatusOK, suite.mustPut(`{"resourceServerId":""}`))
}

func (suite *DefaultResourceServerAPITestSuite) mustPut(body string) int {
	status, _ := suite.put(body)
	return status
}

func (suite *DefaultResourceServerAPITestSuite) put(body string) (int, []byte) {
	req, err := http.NewRequest(http.MethodPut, defaultResourceServerConfigURL, strings.NewReader(body))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.adminClient.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	respBody, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, respBody
}

func (suite *DefaultResourceServerAPITestSuite) get(url string) (int, []byte) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	suite.Require().NoError(err)

	resp, err := suite.adminClient.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, body
}

func (suite *DefaultResourceServerAPITestSuite) getLayers() defaultResourceServerLayers {
	status, body := suite.get(defaultResourceServerConfigURL)
	suite.Require().Equal(http.StatusOK, status)
	var layers defaultResourceServerLayers
	suite.Require().NoError(json.Unmarshal(body, &layers))
	return layers
}

func (suite *DefaultResourceServerAPITestSuite) errorCode(body []byte) string {
	var errResp apiErrorResponse
	suite.Require().NoError(json.Unmarshal(body, &errResp))
	return errResp.Code
}
