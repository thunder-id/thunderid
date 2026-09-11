// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package apiauth

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const testServerURL = testutils.TestServerURL

// maxRequestPathLength mirrors the maximum request path length the server matches against the API
// permission map. The server-side constant is unexported and lives in a separate module, so the
// value is repeated here.
const maxRequestPathLength = 4096

// i18nMessage mirrors the i18n message structure returned in API error responses.
type i18nMessage struct {
	Key          string `json:"key"`
	DefaultValue string `json:"defaultValue"`
}

// apiErrorResponse mirrors apierror.ErrorResponse for decoding security error responses.
type apiErrorResponse struct {
	Code        string      `json:"code"`
	Message     i18nMessage `json:"message"`
	Description i18nMessage `json:"description"`
}

// APIAuthTestSuite validates both authentication and authorization behavior for protected APIs.
type APIAuthTestSuite struct {
	suite.Suite
	adminClient        *http.Client
	plainClient        *http.Client
	invalidTokenClient *http.Client
	userClient         *http.Client
	ouID               string
	entityTypeID       string
	regularUserID      string
}

func TestAPIAuthTestSuite(t *testing.T) {
	suite.Run(t, new(APIAuthTestSuite))
}

func (suite *APIAuthTestSuite) SetupSuite() {
	suite.adminClient = testutils.GetHTTPClient()
	suite.plainClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	suite.invalidTokenClient = testutils.GetHTTPClientWithToken("invalid-token")

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("api-auth-ou-%d", time.Now().UnixNano()),
		Name:   "API Auth Test OU",
	})
	suite.Require().NoError(err)
	suite.ouID = ouID

	entityType := testutils.UserType{
		Name:                  fmt.Sprintf("api-auth-user-%d", time.Now().UnixNano()),
		OUID:                  suite.ouID,
		AllowSelfRegistration: true,
		Schema: map[string]interface{}{
			"username":    map[string]interface{}{"type": "string"},
			"password":    map[string]interface{}{"type": "string", "credential": true},
			"email":       map[string]interface{}{"type": "string"},
			"given_name":  map[string]interface{}{"type": "string"},
			"family_name": map[string]interface{}{"type": "string"},
		},
	}

	entityTypeID, err := testutils.CreateUserType(entityType)
	suite.Require().NoError(err)
	suite.entityTypeID = entityTypeID

	username := fmt.Sprintf("authuser_%d", time.Now().UnixNano())
	password := "ApiAuthTest123!"
	userAttrs := map[string]interface{}{
		"username":    username,
		"password":    password,
		"email":       fmt.Sprintf("%s@example.com", username),
		"given_name":  "API",
		"family_name": "Auth",
	}
	attrBytes, err := json.Marshal(userAttrs)
	suite.Require().NoError(err)

	userID, err := testutils.CreateUser(testutils.User{
		OUID:       suite.ouID,
		Type:       entityType.Name,
		Attributes: attrBytes,
	})
	suite.Require().NoError(err)
	suite.regularUserID = userID

	userClient, err := testutils.GetHTTPClientForUser(username, password)
	suite.Require().NoError(err)
	suite.userClient = userClient
}

func (suite *APIAuthTestSuite) TearDownSuite() {
	if suite.regularUserID != "" {
		if err := testutils.DeleteUser(suite.regularUserID); err != nil {
			suite.T().Logf("Failed to delete regular user: %v", err)
		}
	}

	if suite.entityTypeID != "" {
		if err := testutils.DeleteUserType(suite.entityTypeID); err != nil {
			suite.T().Logf("Failed to delete user type: %v", err)
		}
	}

	if suite.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
			suite.T().Logf("Failed to delete organization unit: %v", err)
		}
	}
}

// Authentication: valid system token succeeds.
func (suite *APIAuthTestSuite) TestSystemTokenAuthorized() {
	req, err := http.NewRequest(http.MethodGet, suite.protectedResourceURL(), nil)
	suite.Require().NoError(err)

	resp, err := suite.adminClient.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	suite.Equal(http.StatusOK, resp.StatusCode)

	var ou testutils.OrganizationUnit
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&ou))
	suite.Equal(suite.ouID, ou.ID)
}

// Authentication: missing token rejected.
func (suite *APIAuthTestSuite) TestMissingTokenIsUnauthorized() {
	req, err := http.NewRequest(http.MethodGet, suite.protectedResourceURL(), nil)
	suite.Require().NoError(err)

	resp, err := suite.plainClient.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	suite.assertSecurityError(resp, http.StatusUnauthorized, "AUTH-4010",
		"Authentication is required to access this resource")
	suite.Equal("Bearer", resp.Header.Get("WWW-Authenticate"))
}

// Authentication: malformed token rejected.
func (suite *APIAuthTestSuite) TestInvalidTokenIsUnauthorized() {
	req, err := http.NewRequest(http.MethodGet, suite.protectedResourceURL(), nil)
	suite.Require().NoError(err)

	resp, err := suite.invalidTokenClient.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	suite.assertSecurityError(resp, http.StatusUnauthorized, "AUTH-4010",
		"Authentication is required to access this resource")
	// A presented-but-rejected token returns the RFC 6750 invalid_token challenge with a generic
	// description that does not disclose why the token was rejected.
	suite.Equal(
		`Bearer error="invalid_token", error_description="The access token is invalid, expired, or malformed"`,
		resp.Header.Get("WWW-Authenticate"))
}

// Authorization: non-system token is forbidden.
func (suite *APIAuthTestSuite) TestNonSystemScopeIsForbidden() {
	suite.Require().NotNil(suite.userClient, "Non-system user client must be available for the test")

	req, err := http.NewRequest(http.MethodGet, suite.protectedResourceURL(), nil)
	suite.Require().NoError(err)

	resp, err := suite.userClient.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	suite.assertSecurityError(resp, http.StatusForbidden, "AUTH-4030",
		"You do not have sufficient permissions to access this resource")
	// A 403 is an authorization failure, not an authentication challenge: no WWW-Authenticate header.
	suite.Empty(resp.Header.Get("WWW-Authenticate"))
}

// Authorization: a request path longer than the matcher limit is forbidden.
//
// The system token carries the root permission, so this request would be authorized had the path
// been matched against the API permission map. The 403 is what shows it was refused on length alone,
// and that refusing rather than matching fails closed.
func (suite *APIAuthTestSuite) TestOversizedPathIsForbidden() {
	req, err := http.NewRequest(http.MethodGet, suite.oversizedPathURL(), nil)
	suite.Require().NoError(err)

	resp, err := suite.adminClient.Do(req)
	suite.Require().NoError(err)
	defer closeBodyQuietly(suite.T(), resp.Body)

	suite.assertSecurityError(resp, http.StatusForbidden, "AUTH-4030",
		"You do not have sufficient permissions to access this resource")
	suite.Empty(resp.Header.Get("WWW-Authenticate"))
}

func (suite *APIAuthTestSuite) assertSecurityError(resp *http.Response, expectedStatus int,
	expectedCode, expectedDescription string) {
	assertSecurityErrorResponse(&suite.Suite, resp, expectedStatus, expectedCode, expectedDescription)
}

// assertSecurityErrorResponse asserts the status and error body the security middleware returns.
// Shared by the suites in this package.
func assertSecurityErrorResponse(s *suite.Suite, resp *http.Response, expectedStatus int,
	expectedCode, expectedDescription string) {
	s.Equal(expectedStatus, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var errResp apiErrorResponse
	s.Require().NoError(json.Unmarshal(bodyBytes, &errResp))

	s.Equal(expectedCode, errResp.Code)
	s.Equal(expectedDescription, errResp.Description.DefaultValue)
}

func (suite *APIAuthTestSuite) protectedResourceURL() string {
	return fmt.Sprintf("%s/organization-units/%s", testServerURL, suite.ouID)
}

// oversizedPathURL returns the URL of an existing route whose path exceeds maxRequestPathLength.
func (suite *APIAuthTestSuite) oversizedPathURL() string {
	return fmt.Sprintf("%s/organization-units/%s", testServerURL,
		strings.Repeat("a", maxRequestPathLength))
}

func closeBodyQuietly(t *testing.T, body io.ReadCloser) {
	if body != nil {
		if err := body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}
}
