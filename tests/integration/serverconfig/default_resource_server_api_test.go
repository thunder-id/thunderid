// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"encoding/json"
	"io"
	"net/http"
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

// put writes the section through an import with the management API key, the only way a data plane
// takes server configuration, and returns the status and body the API would answer with.
func (suite *DefaultResourceServerAPITestSuite) put(body string) (int, []byte) {
	status, respBody, err := testutils.PutWritableServerConfigWithStatus("defaultResourceServer", []byte(body))
	suite.Require().NoError(err)
	return status, respBody
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
