// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const sessionConfigURL = serverConfigURL + "/session"

// sessionSectionValue mirrors the session section value: SSO session lifetimes in seconds.
type sessionSectionValue struct {
	IdleTimeoutSeconds             int64 `json:"idleTimeoutSeconds"`
	AbsoluteTimeoutSeconds         int64 `json:"absoluteTimeoutSeconds"`
	ActivityRefreshIntervalSeconds int64 `json:"activityRefreshIntervalSeconds"`
}

type sessionLayers struct {
	ReadOnly sessionSectionValue `json:"readOnly"`
	Writable sessionSectionValue `json:"writable"`
	Merged   sessionSectionValue `json:"merged"`
}

// SessionConfigAPITestSuite covers the session server-config section and the PUT branches the CORS
// suite does not reach: the response body of a successful PUT, PUT against an unsupported name, and
// a value that fails Decode rather than Validate.
type SessionConfigAPITestSuite struct {
	suite.Suite
	adminClient *http.Client
}

func TestSessionConfigAPITestSuite(t *testing.T) {
	suite.Run(t, new(SessionConfigAPITestSuite))
}

func (suite *SessionConfigAPITestSuite) SetupSuite() {
	suite.adminClient = testutils.GetHTTPClient()
}

// SetupTest clears the writable session layer so each test starts from a known state.
func (suite *SessionConfigAPITestSuite) SetupTest() {
	suite.putSession(`{}`)
}

// TearDownSuite leaves the writable session layer empty so the shared server is not left mutated.
func (suite *SessionConfigAPITestSuite) TearDownSuite() {
	suite.putSession(`{}`)
}

func (suite *SessionConfigAPITestSuite) TestListIncludesSessionSection() {
	status, body := suite.get(serverConfigURL)
	suite.Require().Equal(http.StatusOK, status)

	var names []string
	suite.Require().NoError(json.Unmarshal(body, &names))
	suite.Contains(names, "session")
}

func (suite *SessionConfigAPITestSuite) TestPutPersistsAndReadsBack() {
	suite.putSession(`{"idleTimeoutSeconds":1800,"absoluteTimeoutSeconds":28800,` +
		`"activityRefreshIntervalSeconds":300}`)

	layers := suite.getLayers()
	suite.Equal(int64(1800), layers.Writable.IdleTimeoutSeconds)
	suite.Equal(int64(28800), layers.Writable.AbsoluteTimeoutSeconds)
	suite.Equal(int64(300), layers.Writable.ActivityRefreshIntervalSeconds)

	// The writable layer wins for each field it sets.
	suite.Equal(int64(1800), layers.Merged.IdleTimeoutSeconds)
	suite.Equal(int64(28800), layers.Merged.AbsoluteTimeoutSeconds)
}

// TestPutReturnsRecomputedLayers pins that a successful PUT answers with the new layers, not just a
// status code. The CORS suite only checks the status, so a stale response body would go unnoticed.
func (suite *SessionConfigAPITestSuite) TestPutReturnsRecomputedLayers() {
	status, body := suite.put(sessionConfigURL,
		`{"idleTimeoutSeconds":900,"absoluteTimeoutSeconds":7200,"activityRefreshIntervalSeconds":60}`)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var layers sessionLayers
	suite.Require().NoError(json.Unmarshal(body, &layers))
	suite.Equal(int64(900), layers.Writable.IdleTimeoutSeconds)
	suite.Equal(int64(900), layers.Merged.IdleTimeoutSeconds)

	// The PUT response matches a follow-up GET.
	suite.Equal(layers.Writable, suite.getLayers().Writable)
}

// TestPutReplacesRatherThanMerges asserts a second PUT replaces the writable layer wholesale.
func (suite *SessionConfigAPITestSuite) TestPutReplacesRatherThanMerges() {
	suite.putSession(`{"idleTimeoutSeconds":1800,"absoluteTimeoutSeconds":28800,` +
		`"activityRefreshIntervalSeconds":300}`)
	suite.putSession(`{"idleTimeoutSeconds":600,"absoluteTimeoutSeconds":3600,` +
		`"activityRefreshIntervalSeconds":60}`)

	layers := suite.getLayers()
	suite.Equal(int64(600), layers.Writable.IdleTimeoutSeconds)
	suite.Equal(int64(3600), layers.Writable.AbsoluteTimeoutSeconds)
	suite.Equal(int64(60), layers.Writable.ActivityRefreshIntervalSeconds)
}

// --- Validation branches ---

// TestPutIdleExceedingAbsoluteReturns400 covers a cross-field invariant: idle must not exceed the
// absolute timeout.
func (suite *SessionConfigAPITestSuite) TestPutIdleExceedingAbsoluteReturns400() {
	status, body := suite.put(sessionConfigURL,
		`{"idleTimeoutSeconds":7200,"absoluteTimeoutSeconds":3600,"activityRefreshIntervalSeconds":60}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1003", suite.errorCode(body))
	suite.Zero(suite.getLayers().Writable.IdleTimeoutSeconds, "a rejected value must not persist")
}

// TestPutActivityRefreshNotLessThanIdleReturns400 covers the resolved-duration invariant.
func (suite *SessionConfigAPITestSuite) TestPutActivityRefreshNotLessThanIdleReturns400() {
	status, body := suite.put(sessionConfigURL,
		`{"idleTimeoutSeconds":600,"absoluteTimeoutSeconds":3600,"activityRefreshIntervalSeconds":600}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1003", suite.errorCode(body))
}

func (suite *SessionConfigAPITestSuite) TestPutNegativeValueReturns400() {
	status, body := suite.put(sessionConfigURL, `{"idleTimeoutSeconds":-1}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1003", suite.errorCode(body))
}

// TestPutWrongFieldTypeReturns400 covers the Decode failure path, distinct from the Validate
// failures above: the JSON is well-formed but a field has the wrong type.
func (suite *SessionConfigAPITestSuite) TestPutWrongFieldTypeReturns400() {
	status, body := suite.put(sessionConfigURL, `{"idleTimeoutSeconds":"not-a-number"}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1003", suite.errorCode(body))
}

// TestPutWrongTopLevelShapeReturns400 covers a top-level shape mismatch: the section value is an
// object, not an array.
func (suite *SessionConfigAPITestSuite) TestPutWrongTopLevelShapeReturns400() {
	status, body := suite.put(sessionConfigURL, `["not","an","object"]`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1003", suite.errorCode(body))
}

func (suite *SessionConfigAPITestSuite) TestPutMalformedBodyReturns400() {
	status, body := suite.put(sessionConfigURL, `{`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1004", suite.errorCode(body))
}

// TestPutUnsupportedNameReturns400 is the PUT counterpart of the tested GET case.
func (suite *SessionConfigAPITestSuite) TestPutUnsupportedNameReturns400() {
	status, body := suite.put(serverConfigURL+"/bogus", `{}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1001", suite.errorCode(body))
}

// TestPutOversizedBodyIsRejected covers the 1 MiB request-body cap.
func (suite *SessionConfigAPITestSuite) TestPutOversizedBodyIsRejected() {
	// A syntactically valid object whose padding field pushes it past the cap.
	oversized := `{"idleTimeoutSeconds":1800,"padding":"` + strings.Repeat("a", 1<<20+1024) + `"}`

	status, _ := suite.put(sessionConfigURL, oversized)

	suite.Equal(http.StatusBadRequest, status)
	suite.Zero(suite.getLayers().Writable.IdleTimeoutSeconds, "a rejected body must not persist")
}

// --- helpers ---

// putSession sets the writable session layer and requires a 200 response.
func (suite *SessionConfigAPITestSuite) putSession(body string) {
	status, respBody := suite.put(sessionConfigURL, body)
	suite.Require().Equal(http.StatusOK, status, "put session failed: %s", respBody)
}

func (suite *SessionConfigAPITestSuite) getLayers() sessionLayers {
	status, body := suite.get(sessionConfigURL)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var layers sessionLayers
	suite.Require().NoError(json.Unmarshal(body, &layers))
	return layers
}

func (suite *SessionConfigAPITestSuite) get(url string) (int, []byte) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	suite.Require().NoError(err)
	return suite.send(req)
}

func (suite *SessionConfigAPITestSuite) put(url, body string) (int, []byte) {
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader([]byte(body)))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	return suite.send(req)
}

func (suite *SessionConfigAPITestSuite) send(req *http.Request) (int, []byte) {
	resp, err := suite.adminClient.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, body
}

func (suite *SessionConfigAPITestSuite) errorCode(body []byte) string {
	var errResp apiErrorResponse
	suite.Require().NoError(json.Unmarshal(body, &errResp), "body: %s", body)
	return errResp.Code
}
