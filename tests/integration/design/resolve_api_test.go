// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package design

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	resolveBasePath = "/design/resolve"
)

type ResolveAPITestSuite struct {
	suite.Suite
	client *http.Client
	// ouID owns the applications created by the resolve fixtures; applications require an OU.
	ouID string
}

func TestResolveAPITestSuite(t *testing.T) {
	suite.Run(t, new(ResolveAPITestSuite))
}

func (suite *ResolveAPITestSuite) SetupSuite() {
	// Create HTTP client that skips TLS verification for testing
	suite.client = testutils.GetHTTPClient()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Name:        "Design Resolve OU " + suffix,
		Handle:      "design-resolve-ou-" + suffix,
		Description: "OU owning the applications used by design resolve tests",
	})
	suite.Require().NoError(err, "Failed to create OU")
	suite.ouID = ouID
}

func (suite *ResolveAPITestSuite) TearDownSuite() {
	if suite.ouID == "" {
		return
	}
	if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
		suite.T().Logf("Failed to delete OU %s: %v", suite.ouID, err)
	}
}

// Helper function to resolve design configuration
func (suite *ResolveAPITestSuite) resolveDesign(resolveType, id string) (*DesignResolveResponse, int, error) {
	url := fmt.Sprintf("%s%s?type=%s&id=%s", testServerURL, resolveBasePath, resolveType, id)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil {
			return nil, resp.StatusCode, fmt.Errorf("expected status 200, got %d. Code: %s, Message: %s", resp.StatusCode, errResp.Code, errResp.Message.DefaultValue)
		}
		return nil, resp.StatusCode, fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var resolveResponse DesignResolveResponse
	if err := json.Unmarshal(bodyBytes, &resolveResponse); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	return &resolveResponse, resp.StatusCode, nil
}

// Test Resolve Design - Missing Type Parameter
func (suite *ResolveAPITestSuite) TestResolveDesign_MissingType() {
	url := fmt.Sprintf("%s%s?id=00000000-0000-0000-0000-000000000000", testServerURL, resolveBasePath)

	req, err := http.NewRequest("GET", url, nil)
	suite.Require().NoError(err)

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errResp ErrorResponse
	err = json.Unmarshal(bodyBytes, &errResp)
	suite.Require().NoError(err)
	suite.Equal("DSR-1001", errResp.Code)
}

// Test Resolve Design - Missing ID Parameter
func (suite *ResolveAPITestSuite) TestResolveDesign_MissingID() {
	url := fmt.Sprintf("%s%s?type=APP", testServerURL, resolveBasePath)

	req, err := http.NewRequest("GET", url, nil)
	suite.Require().NoError(err)

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errResp ErrorResponse
	err = json.Unmarshal(bodyBytes, &errResp)
	suite.Require().NoError(err)
	suite.Equal("DSR-1002", errResp.Code)
}

// Test Resolve Design - Unsupported Type
func (suite *ResolveAPITestSuite) TestResolveDesign_UnsupportedType() {
	_, statusCode, err := suite.resolveDesign("OU", "00000000-0000-0000-0000-000000000000")

	suite.Error(err)
	suite.Equal(http.StatusBadRequest, statusCode)
	suite.Contains(err.Error(), "DSR-1003")
}

// Test Resolve Design - Application Not Found
// An unknown application reports the same "no design" error as a known one with nothing
// configured, so the response cannot be used to tell whether an application ID exists.
func (suite *ResolveAPITestSuite) TestResolveDesign_ApplicationNotFound() {
	_, statusCode, err := suite.resolveDesign("APP", "00000000-0000-0000-0000-000000000000")

	suite.Error(err)
	suite.Equal(http.StatusNotFound, statusCode)
	suite.Contains(err.Error(), "DSR-1005")
	suite.NotContains(err.Error(), "DSR-1004")
}

// Test Resolve Design - Success Case
// An application bound to both a theme and a layout resolves both payloads.
func (suite *ResolveAPITestSuite) TestResolveDesign_Success() {
	themeID := suite.createResolveTheme("resolve-both")
	defer suite.deleteDesignResource(themeBasePath, themeID)
	layoutID := suite.createResolveLayout("resolve-both")
	defer suite.deleteDesignResource(layoutBasePath, layoutID)

	appID := suite.createResolveApplication("Resolve Both", themeID, layoutID)
	defer suite.deleteResolveApplication(appID)

	resolved, statusCode, err := suite.resolveDesign("APP", appID)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, statusCode)

	// Both payloads come back as the raw JSON stored on each resource.
	suite.Require().NotEmpty(resolved.Theme)
	suite.Require().NotEmpty(resolved.Layout)
	suite.JSONEq(string(testTheme), string(resolved.Theme))
	suite.JSONEq(string(testLayout), string(resolved.Layout))
}

// Test Resolve Design - Application Without Theme and Layout
// An application with neither a theme nor a layout has no design to resolve (DSR-1005).
func (suite *ResolveAPITestSuite) TestResolveDesign_ApplicationWithoutDesign() {
	appID := suite.createResolveApplication("Resolve No Design", "", "")
	defer suite.deleteResolveApplication(appID)

	_, statusCode, err := suite.resolveDesign("APP", appID)

	suite.Error(err)
	suite.Equal(http.StatusNotFound, statusCode)
	suite.Contains(err.Error(), "DSR-1005")
}

// Test Resolve Design - Unknown And Designless Applications Are Indistinguishable
// This endpoint is unauthenticated, so an anonymous caller must not be able to tell a real
// application from an unknown ID. Both cases have to answer with the same status and the same
// body, byte for byte.
func (suite *ResolveAPITestSuite) TestResolveDesign_UnknownAndDesignlessAreIndistinguishable() {
	knownAppID := suite.createResolveApplication("Resolve Enumeration Probe", "", "")
	defer suite.deleteResolveApplication(knownAppID)

	knownStatus, knownBody := suite.rawResolveDesign("APP", knownAppID)
	unknownStatus, unknownBody := suite.rawResolveDesign("APP", "00000000-0000-0000-0000-0000000000ff")

	suite.Equal(http.StatusNotFound, knownStatus, "body: %s", knownBody)
	suite.Equal(knownStatus, unknownStatus,
		"an unknown application must not be distinguishable by status")
	suite.Equal(knownBody, unknownBody,
		"an unknown application must not be distinguishable by response body")
}

// rawResolveDesign issues a resolve request and returns the status and the response body verbatim,
// so a test can compare two responses without the wrapping that resolveDesign applies.
//
// It deliberately uses a client that injects no credentials. The leak this guards against is one an
// anonymous caller could exploit, and suite.client would attach a bearer token: /design/resolve is
// not in the test transport's public-prefix list, so the comparison would otherwise run as an
// authenticated administrator rather than the caller the endpoint is exposed to.
func (suite *ResolveAPITestSuite) rawResolveDesign(resolveType, id string) (int, string) {
	url := fmt.Sprintf("%s%s?type=%s&id=%s", testServerURL, resolveBasePath, resolveType, id)

	req, err := http.NewRequest("GET", url, nil)
	suite.Require().NoError(err)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	return resp.StatusCode, string(bodyBytes)
}

// Test Resolve Design - Only Theme Configured
func (suite *ResolveAPITestSuite) TestResolveDesign_OnlyTheme() {
	themeID := suite.createResolveTheme("resolve-theme-only")
	defer suite.deleteDesignResource(themeBasePath, themeID)

	appID := suite.createResolveApplication("Resolve Theme Only", themeID, "")
	defer suite.deleteResolveApplication(appID)

	resolved, statusCode, err := suite.resolveDesign("APP", appID)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, statusCode)

	suite.Require().NotEmpty(resolved.Theme)
	suite.JSONEq(string(testTheme), string(resolved.Theme))
	// No layout is bound, so the layout is omitted rather than defaulted.
	suite.Empty(resolved.Layout)
}

// Test Resolve Design - Only Layout Configured
func (suite *ResolveAPITestSuite) TestResolveDesign_OnlyLayout() {
	layoutID := suite.createResolveLayout("resolve-layout-only")
	defer suite.deleteDesignResource(layoutBasePath, layoutID)

	appID := suite.createResolveApplication("Resolve Layout Only", "", layoutID)
	defer suite.deleteResolveApplication(appID)

	resolved, statusCode, err := suite.resolveDesign("APP", appID)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, statusCode)

	suite.Require().NotEmpty(resolved.Layout)
	suite.JSONEq(string(testLayout), string(resolved.Layout))
	suite.Empty(resolved.Theme)
}

// Test Resolve Design - Referenced Theme Deleted
// Deleting a theme an application still references is allowed; resolve degrades to omitting the
// theme (falling back to the default) instead of failing.
func (suite *ResolveAPITestSuite) TestResolveDesign_ReferencedThemeDeleted() {
	themeID := suite.createResolveTheme("resolve-theme-deleted")
	layoutID := suite.createResolveLayout("resolve-theme-deleted")
	defer suite.deleteDesignResource(layoutBasePath, layoutID)

	appID := suite.createResolveApplication("Resolve Theme Deleted", themeID, layoutID)
	defer suite.deleteResolveApplication(appID)

	suite.deleteDesignResource(themeBasePath, themeID)

	resolved, statusCode, err := suite.resolveDesign("APP", appID)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, statusCode)

	suite.Empty(resolved.Theme)
	suite.Require().NotEmpty(resolved.Layout)
	suite.JSONEq(string(testLayout), string(resolved.Layout))
}

// Test Resolve Design - Lowercase Type Is Accepted
// The handler upper-cases the type parameter before matching.
func (suite *ResolveAPITestSuite) TestResolveDesign_TypeIsCaseInsensitive() {
	themeID := suite.createResolveTheme("resolve-lowercase-type")
	defer suite.deleteDesignResource(themeBasePath, themeID)

	appID := suite.createResolveApplication("Resolve Lowercase Type", themeID, "")
	defer suite.deleteResolveApplication(appID)

	resolved, statusCode, err := suite.resolveDesign("app", appID)
	suite.Require().NoError(err)
	suite.Equal(http.StatusOK, statusCode)
	suite.JSONEq(string(testTheme), string(resolved.Theme))
}

// Test Resolve Design - Public Endpoint
// /design/resolve/** is in the public path allow-list, so it must answer without credentials.
func (suite *ResolveAPITestSuite) TestResolveDesign_IsPublic() {
	themeID := suite.createResolveTheme("resolve-public")
	defer suite.deleteDesignResource(themeBasePath, themeID)

	appID := suite.createResolveApplication("Resolve Public", themeID, "")
	defer suite.deleteResolveApplication(appID)

	plainClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	url := fmt.Sprintf("%s%s?type=APP&id=%s", testServerURL, resolveBasePath, appID)
	req, err := http.NewRequest("GET", url, nil)
	suite.Require().NoError(err)

	resp, err := plainClient.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)
}

// --- fixtures ---

// createResolveTheme creates a theme with a handle unique to this suite and returns its ID.
func (suite *ResolveAPITestSuite) createResolveTheme(handle string) string {
	payload, err := json.Marshal(CreateThemeRequest{
		Handle:      handle,
		DisplayName: "Resolve Test Theme " + handle,
		Theme:       testTheme,
	})
	suite.Require().NoError(err)

	id := suite.createDesignResource(themeBasePath, payload)
	suite.Require().NotEmpty(id)
	return id
}

// createResolveLayout creates a layout with a handle unique to this suite and returns its ID.
func (suite *ResolveAPITestSuite) createResolveLayout(handle string) string {
	payload, err := json.Marshal(CreateLayoutRequest{
		Handle:      handle,
		DisplayName: "Resolve Test Layout " + handle,
		Layout:      testLayout,
	})
	suite.Require().NoError(err)

	id := suite.createDesignResource(layoutBasePath, payload)
	suite.Require().NotEmpty(id)
	return id
}

// createDesignResource POSTs a theme or layout payload and returns the created resource ID.
func (suite *ResolveAPITestSuite) createDesignResource(basePath string, payload []byte) string {
	req, err := http.NewRequest("POST", testServerURL+basePath, bytes.NewReader(payload))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusCreated, resp.StatusCode, "create failed: %s", bodyBytes)

	var created struct {
		ID string `json:"id"`
	}
	suite.Require().NoError(json.Unmarshal(bodyBytes, &created))
	return created.ID
}

// deleteDesignResource removes a theme or layout, tolerating an already-deleted resource.
func (suite *ResolveAPITestSuite) deleteDesignResource(basePath, id string) {
	if id == "" {
		return
	}

	req, err := http.NewRequest("DELETE", testServerURL+basePath+"/"+id, nil)
	suite.Require().NoError(err)

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		bodyBytes, _ := io.ReadAll(resp.Body)
		suite.Failf("delete failed", "status %d: %s", resp.StatusCode, bodyBytes)
	}
}

// createResolveApplication creates an application optionally bound to a theme and layout. ouId and
// type are required by the application API; only the design bindings vary per test.
func (suite *ResolveAPITestSuite) createResolveApplication(name, themeID, layoutID string) string {
	body := map[string]interface{}{
		"name":        name,
		"description": "Application for design resolve integration tests",
		"ouId":        suite.ouID,
		"type":        "fullstack",
	}
	if themeID != "" {
		body["themeId"] = themeID
	}
	if layoutID != "" {
		body["layoutId"] = layoutID
	}

	payload, err := json.Marshal(body)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+"/applications", bytes.NewReader(payload))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusCreated, resp.StatusCode, "create application failed: %s", bodyBytes)

	var created struct {
		ID string `json:"id"`
	}
	suite.Require().NoError(json.Unmarshal(bodyBytes, &created))
	suite.Require().NotEmpty(created.ID)
	return created.ID
}

// deleteResolveApplication removes an application created by this suite.
func (suite *ResolveAPITestSuite) deleteResolveApplication(id string) {
	if id == "" {
		return
	}
	if err := testutils.DeleteApplication(id); err != nil {
		suite.T().Logf("Failed to delete application %s: %v", id, err)
	}
}
