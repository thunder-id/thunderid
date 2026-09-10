// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// errorCodeInvalidCredential is returned when a supplied credential value is blank.
const errorCodeInvalidCredential = "APP-1046"

var credentialValidationOU = testutils.OrganizationUnit{
	Handle:      "credential-validation-test-ou",
	Name:        "Credential Validation Test OU",
	Description: "Organization unit for the application credential validation tests",
	Parent:      nil,
}

// CredentialValidationTestSuite covers the rejection of blank credential values on
// PUT /applications/{id}. A value that is non-empty but whitespace-only clears every length check
// on the request and is rejected when the credential is stored, so the API answers 400 with
// APP-1046 rather than storing an unusable secret.
type CredentialValidationTestSuite struct {
	suite.Suite
	ouID string
}

func TestCredentialValidationTestSuite(t *testing.T) {
	suite.Run(t, new(CredentialValidationTestSuite))
}

func (ts *CredentialValidationTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(credentialValidationOU)
	ts.Require().NoError(err, "Failed to create the test organization unit")
	ts.ouID = ouID
}

func (ts *CredentialValidationTestSuite) TearDownSuite() {
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete the test organization unit during teardown: %v", err)
		}
	}
}

// --- helpers ---

// updateExpectingError sends a PUT /applications/{id} and returns the status code and error code
// the server reports. It fails the test if the response is not an error response.
func (ts *CredentialValidationTestSuite) updateExpectingError(appID string, app Application) (int, string) {
	appJSON, err := json.Marshal(app)
	ts.Require().NoError(err, "Failed to marshal the update request")

	req, err := http.NewRequest(http.MethodPut, testServerURL+"/applications/"+appID, bytes.NewReader(appJSON))
	ts.Require().NoError(err, "Failed to build the PUT request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to send the PUT request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read the response body")

	var errResp struct {
		Code        string                `json:"code"`
		Message     testutils.I18nMessage `json:"message"`
		Description testutils.I18nMessage `json:"description"`
	}
	ts.Require().NoError(json.Unmarshal(body, &errResp),
		"The response must be an error response, got: %s", string(body))

	return resp.StatusCode, errResp.Code
}

// embeddedApp builds an application with no OAuth configuration. Such an application initiates
// flows directly through the Flow Execution API, so it is issued a Flow Secret and can rotate one.
func (ts *CredentialValidationTestSuite) embeddedApp(name string) Application {
	return Application{
		OUID:              ts.ouID,
		Name:              name,
		Description:       "Application for the credential validation tests",
		URL:               "https://credential-validation.example.com",
		InboundAuthConfig: []InboundAuthConfig{},
	}
}

// confidentialApp builds an application whose OAuth profile authenticates with a client secret,
// which is the configuration that makes a supplied client secret reach the credential store.
func (ts *CredentialValidationTestSuite) confidentialApp(name, clientID, clientSecret string) Application {
	return Application{
		OUID:        ts.ouID,
		Name:        name,
		Description: "Application for the credential validation tests",
		URL:         "https://credential-validation.example.com",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                clientID,
					ClientSecret:            clientSecret,
					RedirectURIs:            []string{"https://credential-validation.example.com/callback"},
					GrantTypes:              []string{"client_credentials"},
					ResponseTypes:           []string{},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}
}

// --- tests ---

// TestBlankFlowSecretRejectedOnUpdate asserts that a whitespace-only Flow Secret is rejected with
// APP-1046. The preceding rotation with a real value is the control: it proves the request shape is
// accepted, so the rejection is attributable to the blank value alone.
func (ts *CredentialValidationTestSuite) TestBlankFlowSecretRejectedOnUpdate() {
	app := ts.embeddedApp("Blank Flow Secret App")
	appID, err := createApplication(app)
	ts.Require().NoError(err, "Failed to create the application")
	defer func() { _ = deleteApplication(appID) }()

	rotated := app
	rotated.ID = appID
	rotated.FlowSecret = "rotated-flow-secret-value"
	ts.Require().NoError(updateApplication(appID, rotated),
		"Rotating the Flow Secret with a usable value must succeed")

	blank := app
	blank.ID = appID
	blank.FlowSecret = "   "
	status, code := ts.updateExpectingError(appID, blank)

	ts.Equal(http.StatusBadRequest, status, "A blank Flow Secret must be rejected as a client error")
	ts.Equal(errorCodeInvalidCredential, code, "A blank Flow Secret must be reported as an invalid credential")
}

// TestBlankClientSecretRejectedOnUpdate asserts that a whitespace-only client secret is rejected
// with APP-1046, on the same control-then-blank pattern as the Flow Secret case.
func (ts *CredentialValidationTestSuite) TestBlankClientSecretRejectedOnUpdate() {
	app := ts.confidentialApp("Blank Client Secret App", "blank_client_secret_client",
		"blank-client-secret-initial")
	appID, err := createApplication(app)
	ts.Require().NoError(err, "Failed to create the application")
	defer func() { _ = deleteApplication(appID) }()

	rotated := ts.confidentialApp("Blank Client Secret App", "blank_client_secret_client",
		"blank-client-secret-rotated")
	rotated.ID = appID
	ts.Require().NoError(updateApplication(appID, rotated),
		"Rotating the client secret with a usable value must succeed")

	blank := ts.confidentialApp("Blank Client Secret App", "blank_client_secret_client", "  ")
	blank.ID = appID
	status, code := ts.updateExpectingError(appID, blank)

	ts.Equal(http.StatusBadRequest, status, "A blank client secret must be rejected as a client error")
	ts.Equal(errorCodeInvalidCredential, code, "A blank client secret must be reported as an invalid credential")
}
