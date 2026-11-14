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

var sessionConfigURL = serverConfigURL + "/session"

// sessionSectionValue mirrors the session section value: SSO session lifetimes in seconds.
type sessionSectionValue struct {
	IdleTimeoutSeconds     int64 `json:"idleTimeoutSeconds"`
	AbsoluteTimeoutSeconds int64 `json:"absoluteTimeoutSeconds"`
}

type sessionLayers struct {
	ReadOnly sessionSectionValue `json:"readOnly"`
	Writable sessionSectionValue `json:"writable"`
	Merged   sessionSectionValue `json:"merged"`
}

// builtInSessionDefaults are the session timeouts a deployment takes when nothing sets them.
var builtInSessionDefaults = sessionSectionValue{IdleTimeoutSeconds: 7200, AbsoluteTimeoutSeconds: 28800}

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

// An unset section reads as its built-in defaults in effect, with nothing set.
func (suite *SessionConfigAPITestSuite) TestReadShowsTheDefaults() {
	layers := suite.getLayers()

	suite.Equal(sessionSectionValue{}, layers.Writable)
	suite.Equal(builtInSessionDefaults, layers.Merged)
}

func (suite *SessionConfigAPITestSuite) TestPutPersistsAndReadsBack() {
	suite.putSession(`{"idleTimeoutSeconds":1800,"absoluteTimeoutSeconds":28800}`)

	layers := suite.getLayers()
	suite.Equal(int64(1800), layers.Writable.IdleTimeoutSeconds)
	suite.Equal(int64(28800), layers.Writable.AbsoluteTimeoutSeconds)

	// The writable layer wins for each field it sets.
	suite.Equal(int64(1800), layers.Merged.IdleTimeoutSeconds)
	suite.Equal(int64(28800), layers.Merged.AbsoluteTimeoutSeconds)
}

// A field a write leaves out keeps its default, and a later write that leaves it out again returns a
// set field to its default.
func (suite *SessionConfigAPITestSuite) TestAnUnsetFieldTakesItsDefault() {
	suite.putSession(`{"idleTimeoutSeconds":900}`)

	layers := suite.getLayers()
	suite.Equal(sessionSectionValue{IdleTimeoutSeconds: 900}, layers.Writable)
	suite.Equal(sessionSectionValue{IdleTimeoutSeconds: 900, AbsoluteTimeoutSeconds: 28800}, layers.Merged)

	suite.putSession(`{}`)
	suite.Equal(builtInSessionDefaults, suite.getLayers().Merged)
}

// TestPutReturnsRecomputedLayers pins that a successful PUT answers with the new layers, not just a
// status code. The CORS suite only checks the status, so a stale response body would go unnoticed.
func (suite *SessionConfigAPITestSuite) TestPutReturnsRecomputedLayers() {
	status, body := suite.put(sessionConfigURL, `{"idleTimeoutSeconds":900,"absoluteTimeoutSeconds":7200}`)
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
	suite.putSession(`{"idleTimeoutSeconds":1800,"absoluteTimeoutSeconds":28800}`)
	suite.putSession(`{"idleTimeoutSeconds":600}`)

	layers := suite.getLayers()
	suite.Equal(sessionSectionValue{IdleTimeoutSeconds: 600}, layers.Writable)
}

// --- Validation branches ---

// TestPutIdleExceedingAbsoluteReturns400 covers a cross-field invariant: idle must not exceed the
// absolute timeout.
func (suite *SessionConfigAPITestSuite) TestPutIdleExceedingAbsoluteReturns400() {
	status, body := suite.put(sessionConfigURL, `{"idleTimeoutSeconds":7200,"absoluteTimeoutSeconds":3600}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("SCF-1003", suite.errorCode(body))
	suite.Zero(suite.getLayers().Writable.IdleTimeoutSeconds, "a rejected value must not persist")
}

// An idle timeout is compared with an absolute one only when both are set, so an absolute timeout
// shorter than the default idle timeout is accepted on its own, and the idle timeout in effect is cut
// to it.
func (suite *SessionConfigAPITestSuite) TestPutAbsoluteBelowTheDefaultIdleIsAccepted() {
	status, body := suite.put(sessionConfigURL, `{"absoluteTimeoutSeconds":3600}`)

	suite.Require().Equal(http.StatusOK, status, "body: %s", body)
	suite.Equal(sessionSectionValue{AbsoluteTimeoutSeconds: 3600}, suite.getLayers().Writable)
}

// An idle timeout must be longer than the deployment's activity refresh interval, which the
// integration server sets to one second.
func (suite *SessionConfigAPITestSuite) TestPutIdleNotLongerThanTheActivityRefreshIntervalReturns400() {
	status, body := suite.put(sessionConfigURL, `{"idleTimeoutSeconds":1}`)

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
	status, body := suite.putWithToken(sessionConfigURL, `{`)

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

	status, _ := suite.putWithToken(sessionConfigURL, oversized)

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

// put writes the section the URL names through an import with the management API key, the only way a
// data plane takes server configuration, and returns the status and body the API would answer with.
func (suite *SessionConfigAPITestSuite) put(url, body string) (int, []byte) {
	section := strings.TrimPrefix(url, serverConfigURL+"/")
	status, respBody, err := testutils.PutWritableServerConfigWithStatus(section, []byte(body))
	suite.Require().NoError(err)
	return status, respBody
}

// putWithToken sends a PUT with the suite's token. A body the handler refuses never reaches
// authorization, so it is answered as on any server.
func (suite *SessionConfigAPITestSuite) putWithToken(url, body string) (int, []byte) {
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
