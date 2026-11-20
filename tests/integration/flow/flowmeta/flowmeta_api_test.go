// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmeta

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	flowMetaEndpoint = "/flow/meta"

	refTestClientID     = "flowmeta_ref_test_client"
	refTestClientSecret = "flowmeta_ref_test_secret"
	refTestRedirectURI  = "https://localhost:8095/ref-callback"
)

var testServerURL = testutils.TestServerURL

var (
	testOU = testutils.OrganizationUnit{
		Handle:          "flowmeta-test-ou",
		Name:            "FlowMeta Test Organization Unit",
		Description:     "Organization unit for flow metadata integration testing",
		Parent:          nil,
		LogoURL:         "https://example.com/ou-logo.png",
		TosURI:          "https://example.com/tos",
		PolicyURI:       "https://example.com/privacy",
		CookiePolicyURI: "https://example.com/cookie-policy",
	}

	testApp = testutils.Application{
		Name:                      "FlowMeta Test Application",
		Description:               "Application for flow metadata integration testing",
		IsRegistrationFlowEnabled: true,
		ClientID:                  "flowmeta_test_client",
		ClientSecret:              "flowmeta_test_secret",
		RedirectURIs:              []string{"https://localhost:8095/callback"},
	}
)

type FlowMetaAPITestSuite struct {
	suite.Suite
	appID              string
	ouID               string
	isolatedAuthFlowID string
	isolatedRegFlowID  string
}

func TestFlowMetaAPITestSuite(t *testing.T) {
	suite.Run(t, new(FlowMetaAPITestSuite))
}

func (suite *FlowMetaAPITestSuite) SetupSuite() {
	// Create OU
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	suite.Require().NoError(err, "Failed to create OU during setup")
	suite.ouID = ouID

	// Create isolated auth flow to avoid cross-type reference validation with default auth flow.
	isolatedAuthID, err := testutils.CreateIsolatedAuthFlow("flowmeta-api-isolated-auth")
	suite.Require().NoError(err, "Failed to create isolated auth flow")
	suite.isolatedAuthFlowID = isolatedAuthID
	testApp.AuthFlowID = isolatedAuthID

	// Create isolated registration flow so IsRegistrationFlowEnabled is preserved after resolveFlowDefaults.
	isolatedRegID, err := testutils.CreateIsolatedRegistrationFlow("flowmeta-api-isolated-reg")
	suite.Require().NoError(err, "Failed to create isolated registration flow")
	suite.isolatedRegFlowID = isolatedRegID
	testApp.RegistrationFlowID = isolatedRegID

	// Create Application
	testApp.OUID = suite.ouID
	appID, err := testutils.CreateApplication(testApp)
	suite.Require().NoError(err, "Failed to create application during setup")
	suite.appID = appID
}

func (suite *FlowMetaAPITestSuite) TearDownSuite() {
	if suite.appID != "" {
		err := testutils.DeleteApplication(suite.appID)
		if err != nil {
			suite.T().Logf("Failed to delete application during teardown: %v", err)
		}
	}

	if suite.isolatedAuthFlowID != "" {
		if err := testutils.DeleteFlow(suite.isolatedAuthFlowID); err != nil {
			suite.T().Logf("Failed to delete isolated auth flow during teardown: %v", err)
		}
	}

	if suite.isolatedRegFlowID != "" {
		if err := testutils.DeleteFlow(suite.isolatedRegFlowID); err != nil {
			suite.T().Logf("Failed to delete isolated registration flow during teardown: %v", err)
		}
	}

	if suite.ouID != "" {
		err := testutils.DeleteOrganizationUnit(suite.ouID)
		if err != nil {
			suite.T().Logf("Failed to delete OU during teardown: %v", err)
		}
	}
}

// TestGetFlowMetadataWithAppType tests GET /flow/meta?type=APP&id={appID}
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataWithAppType() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s%s?type=APP&id=%s", testServerURL, flowMetaEndpoint, suite.appID), nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var metadata FlowMetadataResponse
	err = json.Unmarshal(body, &metadata)
	suite.Require().NoError(err)

	// Verify application metadata
	suite.NotNil(metadata.Application)
	suite.Equal(suite.appID, metadata.Application.ID)
	suite.Equal(testApp.Name, metadata.Application.Name)
	suite.Equal(testApp.Description, metadata.Application.Description)
	suite.True(metadata.IsRegistrationFlowEnabled)

	// Verify the organization unit is the one the application belongs to
	suite.NotNil(metadata.OU)
	suite.Equal(suite.ouID, metadata.OU.ID)
	suite.Equal(testOU.Handle, metadata.OU.Handle)
	suite.Equal(testOU.Name, metadata.OU.Name)
	suite.Equal(testOU.Description, metadata.OU.Description)
	suite.Equal(testOU.LogoURL, metadata.OU.LogoURL)
	suite.Equal(testOU.TosURI, metadata.OU.TosURI)
	suite.Equal(testOU.PolicyURI, metadata.OU.PolicyURI)
	suite.Equal(testOU.CookiePolicyURI, metadata.OU.CookiePolicyURI)

	// Verify design metadata is present (even if empty)
	suite.NotNil(metadata.Design.Theme)
	suite.NotNil(metadata.Design.Layout)

	// Verify i18n metadata is present
	suite.NotNil(metadata.I18n.Languages)
	suite.NotEmpty(metadata.I18n.Languages)
}

// TestGetFlowMetadataWithOUType tests GET /flow/meta?type=OU&id={ouID}
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataWithOUType() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s%s?type=OU&id=%s", testServerURL, flowMetaEndpoint, suite.ouID), nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var metadata FlowMetadataResponse
	err = json.Unmarshal(body, &metadata)
	suite.Require().NoError(err)

	// Verify OU metadata
	suite.NotNil(metadata.OU)
	suite.Equal(suite.ouID, metadata.OU.ID)
	suite.Equal(testOU.Name, metadata.OU.Name)
	suite.Equal(testOU.Handle, metadata.OU.Handle)
	suite.Equal(testOU.Description, metadata.OU.Description)
	suite.Equal(testOU.LogoURL, metadata.OU.LogoURL)
	suite.Equal(testOU.TosURI, metadata.OU.TosURI)
	suite.Equal(testOU.PolicyURI, metadata.OU.PolicyURI)
	suite.Equal(testOU.CookiePolicyURI, metadata.OU.CookiePolicyURI)

	// Application should be nil for OU type
	suite.Nil(metadata.Application)
	suite.False(metadata.IsRegistrationFlowEnabled)

	// Verify design metadata is present (even if empty)
	suite.NotNil(metadata.Design.Theme)
	suite.NotNil(metadata.Design.Layout)

	// Verify i18n metadata is present
	suite.NotNil(metadata.I18n.Languages)
	suite.NotEmpty(metadata.I18n.Languages)
}

// TestGetFlowMetadataWithLanguageParam tests GET /flow/meta with language parameter
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataWithLanguageParam() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s%s?type=APP&id=%s&language=en", testServerURL, flowMetaEndpoint, suite.appID), nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var metadata FlowMetadataResponse
	err = json.Unmarshal(body, &metadata)
	suite.Require().NoError(err)

	// Verify application metadata is present
	suite.NotNil(metadata.Application)
	suite.Equal(suite.appID, metadata.Application.ID)
}

// TestGetFlowMetadataWithNamespaceParam tests GET /flow/meta with namespace parameter
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataWithNamespaceParam() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s%s?type=APP&id=%s&language=en&namespace=common",
			testServerURL, flowMetaEndpoint, suite.appID), nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var metadata FlowMetadataResponse
	err = json.Unmarshal(body, &metadata)
	suite.Require().NoError(err)

	suite.NotNil(metadata.Application)
}

// TestGetFlowMetadataMissingType tests GET /flow/meta without type parameter
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataMissingType() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s%s?id=%s", testServerURL, flowMetaEndpoint, suite.appID), nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("FM-1004", errorResp.Code)
	suite.Equal("Missing required parameter", errorResp.Message.DefaultValue)
}

// TestGetFlowMetadataMissingID tests GET /flow/meta without id parameter
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataMissingID() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s%s?type=APP", testServerURL, flowMetaEndpoint), nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("FM-1005", errorResp.Code)
	suite.Equal("Missing required parameter", errorResp.Message.DefaultValue)
}

// TestGetFlowMetadataMissingBothParams tests GET /flow/meta without any parameters.
// When neither type nor id is provided, the endpoint returns i18n metadata only (system flow).
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataMissingBothParams() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s%s", testServerURL, flowMetaEndpoint), nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var metadata FlowMetadataResponse
	err = json.Unmarshal(body, &metadata)
	suite.Require().NoError(err)

	// No type/id means system flow: only i18n metadata is returned
	suite.Nil(metadata.Application)
	suite.Nil(metadata.OU)
	suite.NotNil(metadata.I18n.Languages)
	suite.NotEmpty(metadata.I18n.Languages)
}

// TestGetFlowMetadataInvalidType tests GET /flow/meta with invalid type parameter
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataInvalidType() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s%s?type=INVALID&id=%s", testServerURL, flowMetaEndpoint, suite.appID), nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("FM-1001", errorResp.Code)
	suite.Equal("Invalid request", errorResp.Message.DefaultValue)
}

// TestGetFlowMetadataAppNotFound tests GET /flow/meta with non-existent application ID
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataAppNotFound() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s%s?type=APP&id=non-existent-app-id", testServerURL, flowMetaEndpoint), nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("FM-1002", errorResp.Code)
	suite.Equal("Resource not found", errorResp.Message.DefaultValue)
}

// TestGetFlowMetadataOUNotFound tests GET /flow/meta with non-existent OU ID
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataOUNotFound() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s%s?type=OU&id=non-existent-ou-id", testServerURL, flowMetaEndpoint), nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("FM-1003", errorResp.Code)
	suite.Equal("Resource not found", errorResp.Message.DefaultValue)
}

// TestGetFlowMetadataDesignDefaults tests that design metadata returns defaults when no design is configured
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataDesignDefaults() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s%s?type=OU&id=%s", testServerURL, flowMetaEndpoint, suite.ouID), nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var metadata FlowMetadataResponse
	err = json.Unmarshal(body, &metadata)
	suite.Require().NoError(err)

	// Design should default to empty JSON objects when not configured
	suite.NotNil(metadata.Design.Theme)
	suite.NotNil(metadata.Design.Layout)

	// Verify they are valid JSON
	var theme map[string]interface{}
	err = json.Unmarshal(metadata.Design.Theme, &theme)
	suite.NoError(err)

	var layout map[string]interface{}
	err = json.Unmarshal(metadata.Design.Layout, &layout)
	suite.NoError(err)
}

// TestGetFlowMetadataI18nDefaults tests that i18n metadata returns defaults
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataI18nDefaults() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s%s?type=APP&id=%s", testServerURL, flowMetaEndpoint, suite.appID), nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var metadata FlowMetadataResponse
	err = json.Unmarshal(body, &metadata)
	suite.Require().NoError(err)

	// i18n should have at least "en-US" in the languages list
	suite.NotNil(metadata.I18n.Languages)
	suite.Contains(metadata.I18n.Languages, "en-US")

	// Translations map should not be nil
	suite.NotNil(metadata.I18n.Translations)
}

// TestGetFlowMetadataCaseSensitiveType tests that type parameter is case-sensitive
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataCaseSensitiveType() {
	client := testutils.GetHTTPClient()

	// Lowercase "app" should be invalid
	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s%s?type=app&id=%s", testServerURL, flowMetaEndpoint, suite.appID), nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("FM-1001", errorResp.Code)
}

// TestGetFlowMetadataWithRefFromAuthorizeRedirect follows the ref the authorize endpoint adds to the
// sign-in page URL back to the application through GET /flow/meta?type=REF.
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataWithRefFromAuthorizeRedirect() {
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:        "FlowMeta Ref Test Application",
		Description: "Application the sign-in page reference names",
		OUID:        suite.ouID,
		AuthFlowID:  suite.isolatedAuthFlowID,
		InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": map[string]interface{}{
			"clientId": refTestClientID, "clientSecret": refTestClientSecret,
			"redirectUris": []string{refTestRedirectURI}, "grantTypes": []string{"authorization_code"},
			"responseTypes": []string{"code"}, "tokenEndpointAuthMethod": "client_secret_basic",
		}}},
	})
	suite.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	resp, err := testutils.InitiateAuthorizationFlow(refTestClientID, refTestRedirectURI, "code", "openid", "s1")
	suite.Require().NoError(err)
	defer resp.Body.Close()
	suite.Require().Equal(http.StatusFound, resp.StatusCode)

	ref := testutils.ExtractQueryParam(resp.Header.Get("Location"), "ref")
	suite.Require().NotEmpty(ref, "the sign-in page URL carries ref")
	suite.NotContains(ref, appID, "ref does not show the application ID")

	status, body := suite.getFlowMetadata(url.Values{"type": {"REF"}, "id": {ref}})
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var metadata FlowMetadataResponse
	suite.Require().NoError(json.Unmarshal(body, &metadata))
	suite.Require().NotNil(metadata.Application)
	suite.Equal(appID, metadata.Application.ID)
	suite.Equal("FlowMeta Ref Test Application", metadata.Application.Name)
}

// TestGetFlowMetadataWithInvalidRef tests that a ref that names no application returns 404.
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataWithInvalidRef() {
	for _, ref := range []string{"not*a*ref", "AZAAAAAAcACAAAAAAAAAAA", "~bm9uLWV4aXN0ZW50"} {
		status, body := suite.getFlowMetadata(url.Values{"type": {"REF"}, "id": {ref}})
		suite.Equal(http.StatusNotFound, status, "ref %q: %s", ref, body)

		var errorResp ErrorResponse
		suite.Require().NoError(json.Unmarshal(body, &errorResp))
		suite.Equal("FM-1002", errorResp.Code)
	}
}

// TestGetFlowMetadataDefaultReturnURL tests that the flow section's defaultReturnUrl is returned with
// and without a type.
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataDefaultReturnURL() {
	original, err := testutils.GetWritableServerConfig("flow")
	suite.Require().NoError(err)
	defer func() { suite.NoError(testutils.PutWritableServerConfig("flow", original)) }()

	section := map[string]interface{}{}
	suite.Require().NoError(json.Unmarshal(original, &section))
	section["defaultReturnUrl"] = "https://home.example.com"
	payload, err := json.Marshal(section)
	suite.Require().NoError(err)
	suite.Require().NoError(testutils.PutWritableServerConfig("flow", payload))

	for name, query := range map[string]url.Values{
		"no type":  {},
		"app type": {"type": {"APP"}, "id": {suite.appID}},
	} {
		suite.Run(name, func() {
			status, body := suite.getFlowMetadata(query)
			suite.Require().Equal(http.StatusOK, status, "body: %s", body)

			var metadata FlowMetadataResponse
			suite.Require().NoError(json.Unmarshal(body, &metadata))
			suite.Equal("https://home.example.com", metadata.DefaultReturnURL)
		})
	}
}

// TestGetFlowMetadataDeclarativeDefaultReturnURL tests that a defaultReturnUrl seeded in the declarative
// flow section is returned while the writable layer sets none.
func (suite *FlowMetaAPITestSuite) TestGetFlowMetadataDeclarativeDefaultReturnURL() {
	status, body := suite.getFlowMetadata(url.Values{})
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var metadata FlowMetadataResponse
	suite.Require().NoError(json.Unmarshal(body, &metadata))
	suite.Equal(declarativeDefaultReturnURL, metadata.DefaultReturnURL)
}

// getFlowMetadata calls GET /flow/meta with the given query and returns the status and body.
func (suite *FlowMetaAPITestSuite) getFlowMetadata(query url.Values) (int, []byte) {
	resp, err := testutils.GetHTTPClient().Get(
		fmt.Sprintf("%s%s?%s", testServerURL, flowMetaEndpoint, query.Encode()))
	suite.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, body
}
