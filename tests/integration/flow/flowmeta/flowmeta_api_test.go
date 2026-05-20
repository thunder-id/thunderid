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

package flowmeta

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	testServerURL    = "https://localhost:8095"
	flowMetaEndpoint = "/flow/meta"
)

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
	appID string
	ouID  string
}

func TestFlowMetaAPITestSuite(t *testing.T) {
	suite.Run(t, new(FlowMetaAPITestSuite))
}

func (suite *FlowMetaAPITestSuite) SetupSuite() {
	// Create OU
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	suite.Require().NoError(err, "Failed to create OU during setup")
	suite.ouID = ouID

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
