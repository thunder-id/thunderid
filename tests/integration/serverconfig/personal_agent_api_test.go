// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	personalAgentConfigURL = serverConfigURL + "/personalAgent"

	// defaultPersonalAgentAuthFlowHandle is the login flow shipped for personal agents.
	defaultPersonalAgentAuthFlowHandle = "default-personal-agent-auth-flow"
)

// personalAgentSectionValue mirrors the personalAgent section value.
type personalAgentSectionValue struct {
	Enabled                *bool `json:"enabled,omitempty"`
	MaxAccessTokenValidity int64 `json:"maxAccessTokenValidity,omitempty"`
	AuthFlow               struct {
		DefaultHandle string `json:"defaultHandle,omitempty"`
	} `json:"authFlow"`
}

type personalAgentLayers struct {
	ReadOnly personalAgentSectionValue `json:"readOnly"`
	Writable personalAgentSectionValue `json:"writable"`
	Merged   personalAgentSectionValue `json:"merged"`
}

// PersonalAgentConfigAPITestSuite covers the personalAgent server-config section: the defaults, the
// writable layer, and validation of the token lifetime cap and the login flow handle.
type PersonalAgentConfigAPITestSuite struct {
	suite.Suite
	adminClient *http.Client
	// initialWritable is the writable layer found before the suite, restored after each test.
	initialWritable json.RawMessage
}

func TestPersonalAgentConfigAPITestSuite(t *testing.T) {
	suite.Run(t, new(PersonalAgentConfigAPITestSuite))
}

func (suite *PersonalAgentConfigAPITestSuite) SetupSuite() {
	suite.adminClient = testutils.GetHTTPClient()

	status, body := suite.get(personalAgentConfigURL)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)
	var raw struct {
		Writable json.RawMessage `json:"writable"`
	}
	suite.Require().NoError(json.Unmarshal(body, &raw))
	suite.initialWritable = raw.Writable
}

// TearDownTest restores the initial writable layer, so later suites see the defaults.
func (suite *PersonalAgentConfigAPITestSuite) TearDownTest() {
	suite.putPersonalAgent(string(suite.initialWritable))
}

func (suite *PersonalAgentConfigAPITestSuite) TestListIncludesPersonalAgentSection() {
	status, body := suite.get(serverConfigURL)
	suite.Require().Equal(http.StatusOK, status)

	var names []string
	suite.Require().NoError(json.Unmarshal(body, &names))
	suite.Contains(names, "personalAgent")
}

// TestDefaults pins what a fresh deployment reads with nothing configured: personal agents off, a
// one-hour token cap, and the shipped login flow, which bootstrap creates as an AUTHENTICATION flow.
func (suite *PersonalAgentConfigAPITestSuite) TestDefaults() {
	layers := suite.getLayers()
	suite.Nil(layers.Writable.Enabled, "nothing is stored until an administrator sets a value")
	suite.Zero(layers.Writable.MaxAccessTokenValidity)

	merged := layers.Merged
	suite.Require().NotNil(merged.Enabled)
	suite.False(*merged.Enabled)
	suite.Equal(int64(3600), merged.MaxAccessTokenValidity)
	suite.Equal(defaultPersonalAgentAuthFlowHandle, merged.AuthFlow.DefaultHandle)

	flowID, err := testutils.GetFlowIDByHandle(defaultPersonalAgentAuthFlowHandle, "AUTHENTICATION")
	suite.Require().NoError(err)
	suite.NotEmpty(flowID)
}

func (suite *PersonalAgentConfigAPITestSuite) TestPutPersistsAndReadsBack() {
	suite.putPersonalAgent(`{"enabled":true,"maxAccessTokenValidity":900,` +
		`"authFlow":{"defaultHandle":"` + defaultPersonalAgentAuthFlowHandle + `"}}`)

	layers := suite.getLayers()
	suite.Require().NotNil(layers.Writable.Enabled)
	suite.True(*layers.Writable.Enabled)
	suite.Require().NotNil(layers.Merged.Enabled)
	suite.True(*layers.Merged.Enabled)
	suite.Equal(int64(900), layers.Merged.MaxAccessTokenValidity)
	suite.Equal(defaultPersonalAgentAuthFlowHandle, layers.Merged.AuthFlow.DefaultHandle)
}

// TestPutPartialKeepsOtherDefaults shows that setting one value leaves the others at their defaults.
func (suite *PersonalAgentConfigAPITestSuite) TestPutPartialKeepsOtherDefaults() {
	suite.putPersonalAgent(`{"enabled":true}`)

	merged := suite.getLayers().Merged
	suite.Require().NotNil(merged.Enabled)
	suite.True(*merged.Enabled)
	suite.Equal(int64(3600), merged.MaxAccessTokenValidity)
	suite.Equal(defaultPersonalAgentAuthFlowHandle, merged.AuthFlow.DefaultHandle)
}

// TestPutAcceptsAnyAuthenticationFlow shows the login flow handle may name any AUTHENTICATION flow.
func (suite *PersonalAgentConfigAPITestSuite) TestPutAcceptsAnyAuthenticationFlow() {
	suite.putPersonalAgent(`{"authFlow":{"defaultHandle":"default-flow"}}`)

	suite.Equal("default-flow", suite.getLayers().Merged.AuthFlow.DefaultHandle)
}

// --- Validation branches ---

func (suite *PersonalAgentConfigAPITestSuite) TestPutNegativeValidityReturns400() {
	status, body := suite.put(personalAgentConfigURL, `{"maxAccessTokenValidity":-1}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1003", suite.errorCode(body))
	suite.Equal(int64(3600), suite.getLayers().Merged.MaxAccessTokenValidity,
		"a rejected value must not persist")
}

func (suite *PersonalAgentConfigAPITestSuite) TestPutUnknownAuthFlowReturns400() {
	status, body := suite.put(personalAgentConfigURL, `{"authFlow":{"defaultHandle":"no-such-flow"}}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1003", suite.errorCode(body))
}

// TestPutNonAuthenticationFlowReturns400 refuses a handle that only names a flow of another type.
func (suite *PersonalAgentConfigAPITestSuite) TestPutNonAuthenticationFlowReturns400() {
	status, body := suite.put(personalAgentConfigURL,
		`{"authFlow":{"defaultHandle":"default-agent-onboarding-flow"}}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1003", suite.errorCode(body))
	suite.Equal(defaultPersonalAgentAuthFlowHandle, suite.getLayers().Merged.AuthFlow.DefaultHandle)
}

func (suite *PersonalAgentConfigAPITestSuite) TestPutWrongFieldTypeReturns400() {
	status, body := suite.put(personalAgentConfigURL, `{"enabled":"yes"}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1003", suite.errorCode(body))
}

// --- helpers ---

// putPersonalAgent sets the writable personalAgent layer and requires a 200 response.
func (suite *PersonalAgentConfigAPITestSuite) putPersonalAgent(body string) {
	status, respBody := suite.put(personalAgentConfigURL, body)
	suite.Require().Equal(http.StatusOK, status, "put personalAgent failed: %s", respBody)
}

func (suite *PersonalAgentConfigAPITestSuite) getLayers() personalAgentLayers {
	status, body := suite.get(personalAgentConfigURL)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var layers personalAgentLayers
	suite.Require().NoError(json.Unmarshal(body, &layers))
	return layers
}

func (suite *PersonalAgentConfigAPITestSuite) get(url string) (int, []byte) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	suite.Require().NoError(err)
	return suite.send(req)
}

func (suite *PersonalAgentConfigAPITestSuite) put(url, body string) (int, []byte) {
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader([]byte(body)))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	return suite.send(req)
}

func (suite *PersonalAgentConfigAPITestSuite) send(req *http.Request) (int, []byte) {
	resp, err := suite.adminClient.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, body
}

func (suite *PersonalAgentConfigAPITestSuite) errorCode(body []byte) string {
	var errResp apiErrorResponse
	suite.Require().NoError(json.Unmarshal(body, &errResp), "body: %s", body)
	return errResp.Code
}
