// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// SCIMMeTestSuite exercises GET/PUT /scim/v2/Me (RFC 7644 §3.11), the
// authenticated-subject alias: userID is taken from the caller's own token
// subject (security.GetSubject), not a path parameter, so this needs a real
// end-user login (username+password), not the suite-wide admin client every
// other SCIM suite here uses. /Me requires only authentication, no specific
// permission — same self-service shape as GET/PUT /users/me exercised by
// tests/integration/user's SelfUserEndpointsSuite, whose
// testutils.GetHTTPClientForUser this suite reuses directly.
type SCIMMeTestSuite struct {
	suite.Suite
	ouID           string
	entityTypeID   string
	entityTypeName string
	extensionURN   string

	selfUserID string
	username   string
	password   string
	email      string
	selfClient *http.Client
}

// TestSCIMMeTestSuite tests SCIM Me Test Suite.
func TestSCIMMeTestSuite(t *testing.T) {
	suite.Run(t, new(SCIMMeTestSuite))
}

// SetupSuite initializes the test suite environment.
func (ts *SCIMMeTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "scim-it-me-ou",
		Name:        "SCIM Me Integration Test OU",
		Description: "Organization unit for SCIM /Me endpoint tests",
	})
	ts.Require().NoError(err, "failed to create test organization unit")
	ts.ouID = ouID

	ts.entityTypeName = "scim-it-me-person"
	ts.username = "scim.it.me"
	ts.password = "ScimItMe@123"
	ts.email = "scim.it.me@example.com"

	entityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name:                  ts.entityTypeName,
		OUID:                  ouID,
		AllowSelfRegistration: true,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string", "required": true, "unique": true},
			"nickname": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "failed to create test entity type")
	ts.entityTypeID = entityTypeID

	urn, _, err := discoverExtensionSchema(ts.entityTypeName)
	ts.Require().NoError(err, "failed to discover extension schema via GET /Schemas")
	ts.extensionURN = urn

	attrs, err := json.Marshal(map[string]interface{}{
		"username": ts.username,
		"password": ts.password,
		"email":    ts.email,
	})
	ts.Require().NoError(err)

	selfUserID, err := testutils.CreateUser(testutils.User{
		OUID:       ouID,
		Type:       ts.entityTypeName,
		Attributes: attrs,
	})
	ts.Require().NoError(err, "failed to create self-service user")
	ts.selfUserID = selfUserID

	selfClient, err := testutils.GetHTTPClientForUser(ts.username, ts.password)
	ts.Require().NoError(err, "failed to obtain self-service client")
	ts.selfClient = selfClient
}

// TearDownSuite cleans up the test suite environment.
func (ts *SCIMMeTestSuite) TearDownSuite() {
	if ts.selfUserID != "" {
		_ = testutils.DeleteUser(ts.selfUserID)
	}
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// doMe issues a request against /scim/v2/Me via the self-service user's own
// client, mirroring scimRequest but authenticated as the end-user rather
// than the suite-wide admin.
// doMe handles do me.
func (ts *SCIMMeTestSuite) doMe(method string, body []byte, headers map[string]string) (int, []byte) {
	ts.T().Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, scimBaseURL+"/Me", reader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/scim+json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := ts.selfClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, respBody
}

// buildMeBody handles build me body.
func (ts *SCIMMeTestSuite) buildMeBody(nickname string) []byte {
	payload := map[string]interface{}{
		"schemas":  []string{scimCoreUserSchemaURN, ts.extensionURN},
		"userName": ts.username,
		"emails":   []map[string]interface{}{{"value": ts.email, "type": "work"}},
		ts.extensionURN: map[string]interface{}{
			"nickname": nickname,
		},
	}
	b, err := json.Marshal(payload)
	ts.Require().NoError(err)
	return b
}

// ---------------------------------------------------------------------------
// Success cases
// ---------------------------------------------------------------------------

// TestGetMe tests Get Me.
func (ts *SCIMMeTestSuite) TestGetMe() {
	status, body := ts.doMe(http.MethodGet, nil, nil)
	ts.Require().Equal(http.StatusOK, status, "GET /Me failed: %s", body)

	var me map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &me))
	ts.Equal(ts.selfUserID, me["id"], "/Me must resolve to the caller's own user, not any other resource")

	email, ok := extensionStringValue(me, ts.extensionURN, "email")
	ts.Require().True(ok, "GET /Me response should include the custom-schema email attribute")
	ts.Equal(ts.email, email)
}

// TestReplaceMe tests Replace Me.
func (ts *SCIMMeTestSuite) TestReplaceMe() {
	status, body := ts.doMe(http.MethodPut, ts.buildMeBody("Scimmy"), nil)
	ts.Require().Equal(http.StatusOK, status, "PUT /Me failed: %s", body)

	status, body = ts.doMe(http.MethodGet, nil, nil)
	ts.Require().Equal(http.StatusOK, status)

	var me map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &me))
	ext, ok := me[ts.extensionURN].(map[string]interface{})
	ts.Require().True(ok)
	ts.Equal("Scimmy", ext["nickname"], "PUT /Me should persist the replaced extension attribute")
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

// TestGetMeNoTokenUnauthorized tests Get Me No Token Unauthorized.
func (ts *SCIMMeTestSuite) TestGetMeNoTokenUnauthorized() {
	status, _, err := scimRequestUnauthenticated(http.MethodGet, "/Me", nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, status)
}

// TestGetMeInvalidTokenUnauthorized tests Get Me Invalid Token Unauthorized.
func (ts *SCIMMeTestSuite) TestGetMeInvalidTokenUnauthorized() {
	status, _, err := scimRequestUnauthenticated(http.MethodGet, "/Me",
		map[string]string{"Authorization": "Bearer this-is-not-a-real-token"})
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, status)
}

// TestReplaceMeWrongContentTypeRejected tests Replace Me Wrong Content Type Rejected.
func (ts *SCIMMeTestSuite) TestReplaceMeWrongContentTypeRejected() {
	status, body := ts.doMe(http.MethodPut, ts.buildMeBody("does-not-matter"),
		map[string]string{"Content-Type": "text/plain"})
	ts.Equal(http.StatusBadRequest, status, "wrong Content-Type must be rejected: %s", body)
}

// TestReplaceMeEmptyBodyRejected tests Replace Me Empty Body Rejected.
func (ts *SCIMMeTestSuite) TestReplaceMeEmptyBodyRejected() {
	status, body := ts.doMe(http.MethodPut, []byte{}, nil)
	ts.Equal(http.StatusBadRequest, status, "empty body must be rejected: %s", body)
}
