// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package design

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// DesignUsagesHandleAPITestSuite covers the parts of the theme and layout management APIs the
// per-resource CRUD suites leave untested: the /usages endpoints, handle uniqueness, and handle
// immutability on update.
type DesignUsagesHandleAPITestSuite struct {
	suite.Suite
	client *http.Client
	// ouID owns the applications created to reference a theme or layout; applications require an OU.
	ouID string
}

func TestDesignUsagesHandleAPITestSuite(t *testing.T) {
	suite.Run(t, new(DesignUsagesHandleAPITestSuite))
}

func (suite *DesignUsagesHandleAPITestSuite) SetupSuite() {
	suite.client = testutils.GetHTTPClient()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Name:        "Design Usages OU " + suffix,
		Handle:      "design-usages-ou-" + suffix,
		Description: "OU owning the applications used by design usages tests",
	})
	suite.Require().NoError(err, "Failed to create OU")
	suite.ouID = ouID
}

func (suite *DesignUsagesHandleAPITestSuite) TearDownSuite() {
	if suite.ouID == "" {
		return
	}
	if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
		suite.T().Logf("Failed to delete OU %s: %v", suite.ouID, err)
	}
}

// --- Theme usages ---

// TestThemeUsagesEmptyWhenUnreferenced asserts an unreferenced theme reports a confirmed-empty
// usage set rather than an unknown one.
func (suite *DesignUsagesHandleAPITestSuite) TestThemeUsagesEmptyWhenUnreferenced() {
	themeID := suite.createTheme("usages-theme-unreferenced")
	defer suite.deleteResource(themeBasePath, themeID)

	usages := suite.getUsages(themeBasePath, themeID)

	suite.Equal(0, usages.Count)
	suite.Empty(usages.Usages)
}

// TestThemeUsagesListsReferencingApplication asserts an application that binds the theme shows up
// in the theme's usage list.
func (suite *DesignUsagesHandleAPITestSuite) TestThemeUsagesListsReferencingApplication() {
	themeID := suite.createTheme("usages-theme-referenced")
	defer suite.deleteResource(themeBasePath, themeID)

	appID := suite.createApplication("Theme Usages App", themeID, "")
	defer suite.deleteApplication(appID)

	usages := suite.getUsages(themeBasePath, themeID)

	suite.Equal(1, usages.Count)
	suite.Require().Len(usages.Usages, 1)
	suite.Equal(appID, usages.Usages[0].ID)
	suite.Equal("Theme Usages App", usages.Usages[0].DisplayName)
	suite.NotEmpty(usages.Usages[0].ResourceType)
}

func (suite *DesignUsagesHandleAPITestSuite) TestThemeUsagesNotFound() {
	status, body := suite.do(http.MethodGet,
		testServerURL+themeBasePath+"/00000000-0000-0000-0000-000000000000/usages", nil)

	suite.Equal(http.StatusNotFound, status)
	suite.Equal("THM-1003", suite.errorCode(body))
}

func (suite *DesignUsagesHandleAPITestSuite) TestThemeUsagesInvalidPagination() {
	themeID := suite.createTheme("usages-theme-pagination")
	defer suite.deleteResource(themeBasePath, themeID)

	limitStatus, limitBody := suite.do(http.MethodGet,
		fmt.Sprintf("%s%s/%s/usages?limit=-1", testServerURL, themeBasePath, themeID), nil)
	suite.Equal(http.StatusBadRequest, limitStatus)
	suite.Equal("THM-1008", suite.errorCode(limitBody))

	offsetStatus, offsetBody := suite.do(http.MethodGet,
		fmt.Sprintf("%s%s/%s/usages?offset=-1", testServerURL, themeBasePath, themeID), nil)
	suite.Equal(http.StatusBadRequest, offsetStatus)
	suite.Equal("THM-1009", suite.errorCode(offsetBody))
}

// TestThemeUsagesNonNumericPagination covers the handler-level strconv failures, which map to
// different codes than the service-level range checks.
func (suite *DesignUsagesHandleAPITestSuite) TestThemeUsagesNonNumericPagination() {
	themeID := suite.createTheme("usages-theme-nan")
	defer suite.deleteResource(themeBasePath, themeID)

	limitStatus, limitBody := suite.do(http.MethodGet,
		fmt.Sprintf("%s%s/%s/usages?limit=abc", testServerURL, themeBasePath, themeID), nil)
	suite.Equal(http.StatusBadRequest, limitStatus)
	suite.Equal("THM-1010", suite.errorCode(limitBody))

	offsetStatus, offsetBody := suite.do(http.MethodGet,
		fmt.Sprintf("%s%s/%s/usages?offset=abc", testServerURL, themeBasePath, themeID), nil)
	suite.Equal(http.StatusBadRequest, offsetStatus)
	suite.Equal("THM-1011", suite.errorCode(offsetBody))
}

// --- Layout usages ---

func (suite *DesignUsagesHandleAPITestSuite) TestLayoutUsagesEmptyWhenUnreferenced() {
	layoutID := suite.createLayout("usages-layout-unreferenced")
	defer suite.deleteResource(layoutBasePath, layoutID)

	usages := suite.getUsages(layoutBasePath, layoutID)

	suite.Equal(0, usages.Count)
	suite.Empty(usages.Usages)
}

func (suite *DesignUsagesHandleAPITestSuite) TestLayoutUsagesListsReferencingApplication() {
	layoutID := suite.createLayout("usages-layout-referenced")
	defer suite.deleteResource(layoutBasePath, layoutID)

	appID := suite.createApplication("Layout Usages App", "", layoutID)
	defer suite.deleteApplication(appID)

	usages := suite.getUsages(layoutBasePath, layoutID)

	suite.Equal(1, usages.Count)
	suite.Require().Len(usages.Usages, 1)
	suite.Equal(appID, usages.Usages[0].ID)
	suite.Equal("Layout Usages App", usages.Usages[0].DisplayName)
}

func (suite *DesignUsagesHandleAPITestSuite) TestLayoutUsagesNotFound() {
	status, body := suite.do(http.MethodGet,
		testServerURL+layoutBasePath+"/00000000-0000-0000-0000-000000000000/usages", nil)

	suite.Equal(http.StatusNotFound, status)
	suite.Equal("LAY-1003", suite.errorCode(body))
}

func (suite *DesignUsagesHandleAPITestSuite) TestLayoutUsagesInvalidPagination() {
	layoutID := suite.createLayout("usages-layout-pagination")
	defer suite.deleteResource(layoutBasePath, layoutID)

	limitStatus, limitBody := suite.do(http.MethodGet,
		fmt.Sprintf("%s%s/%s/usages?limit=-1", testServerURL, layoutBasePath, layoutID), nil)
	suite.Equal(http.StatusBadRequest, limitStatus)
	suite.Equal("LAY-1009", suite.errorCode(limitBody))

	offsetStatus, offsetBody := suite.do(http.MethodGet,
		fmt.Sprintf("%s%s/%s/usages?offset=-1", testServerURL, layoutBasePath, layoutID), nil)
	suite.Equal(http.StatusBadRequest, offsetStatus)
	suite.Equal("LAY-1010", suite.errorCode(offsetBody))
}

// --- Handle semantics ---

// TestCreateThemeMissingHandleReturnsError pins THM-1016; the existing validation tests always
// omit displayName too, and displayName is checked first, so this branch was never reached.
func (suite *DesignUsagesHandleAPITestSuite) TestCreateThemeMissingHandleReturnsError() {
	payload, err := json.Marshal(CreateThemeRequest{
		DisplayName: "Theme Without Handle",
		Theme:       testTheme,
	})
	suite.Require().NoError(err)

	status, body := suite.do(http.MethodPost, testServerURL+themeBasePath, payload)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("THM-1016", suite.errorCode(body))
}

func (suite *DesignUsagesHandleAPITestSuite) TestCreateLayoutMissingHandleReturnsError() {
	payload, err := json.Marshal(CreateLayoutRequest{
		DisplayName: "Layout Without Handle",
		Layout:      testLayout,
	})
	suite.Require().NoError(err)

	status, body := suite.do(http.MethodPost, testServerURL+layoutBasePath, payload)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("LAY-1017", suite.errorCode(body))
}

func (suite *DesignUsagesHandleAPITestSuite) TestCreateThemeDuplicateHandleReturnsError() {
	handle := "duplicate-theme-handle"
	themeID := suite.createTheme(handle)
	defer suite.deleteResource(themeBasePath, themeID)

	payload, err := json.Marshal(CreateThemeRequest{
		Handle:      handle,
		DisplayName: "Duplicate Handle Theme",
		Theme:       testTheme,
	})
	suite.Require().NoError(err)

	status, body := suite.do(http.MethodPost, testServerURL+themeBasePath, payload)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("THM-1015", suite.errorCode(body))
}

func (suite *DesignUsagesHandleAPITestSuite) TestCreateLayoutDuplicateHandleReturnsError() {
	handle := "duplicate-layout-handle"
	layoutID := suite.createLayout(handle)
	defer suite.deleteResource(layoutBasePath, layoutID)

	payload, err := json.Marshal(CreateLayoutRequest{
		Handle:      handle,
		DisplayName: "Duplicate Handle Layout",
		Layout:      testLayout,
	})
	suite.Require().NoError(err)

	status, body := suite.do(http.MethodPost, testServerURL+layoutBasePath, payload)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("LAY-1016", suite.errorCode(body))
}

// TestUpdateThemeHandleIsImmutable asserts a PUT that changes the handle is rejected; the existing
// update tests always resend the same handle.
func (suite *DesignUsagesHandleAPITestSuite) TestUpdateThemeHandleIsImmutable() {
	themeID := suite.createTheme("immutable-theme-handle")
	defer suite.deleteResource(themeBasePath, themeID)

	payload, err := json.Marshal(UpdateThemeRequest{
		Handle:      "immutable-theme-handle-changed",
		DisplayName: "Renamed Handle Theme",
		Theme:       testTheme,
	})
	suite.Require().NoError(err)

	status, body := suite.do(http.MethodPut, testServerURL+themeBasePath+"/"+themeID, payload)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("THM-1017", suite.errorCode(body))
}

// TestUpdateThemeOmittedHandleKeepsExistingHandle asserts an update that omits the handle is
// accepted and leaves the stored handle untouched.
func (suite *DesignUsagesHandleAPITestSuite) TestUpdateThemeOmittedHandleKeepsExistingHandle() {
	handle := "omitted-theme-handle"
	themeID := suite.createTheme(handle)
	defer suite.deleteResource(themeBasePath, themeID)

	payload, err := json.Marshal(UpdateThemeRequest{
		DisplayName: "Updated Without Handle",
		Theme:       testTheme,
	})
	suite.Require().NoError(err)

	status, body := suite.do(http.MethodPut, testServerURL+themeBasePath+"/"+themeID, payload)
	suite.Require().Equal(http.StatusOK, status, "update failed: %s", body)

	var updated ThemeResponse
	suite.Require().NoError(json.Unmarshal(body, &updated))
	suite.Equal(handle, updated.Handle)
	suite.Equal("Updated Without Handle", updated.DisplayName)
}

func (suite *DesignUsagesHandleAPITestSuite) TestUpdateLayoutHandleIsImmutable() {
	layoutID := suite.createLayout("immutable-layout-handle")
	defer suite.deleteResource(layoutBasePath, layoutID)

	payload, err := json.Marshal(UpdateLayoutRequest{
		Handle:      "immutable-layout-handle-changed",
		DisplayName: "Renamed Handle Layout",
		Layout:      testLayout,
	})
	suite.Require().NoError(err)

	status, body := suite.do(http.MethodPut, testServerURL+layoutBasePath+"/"+layoutID, payload)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("LAY-1018", suite.errorCode(body))
}

// --- helpers ---

func (suite *DesignUsagesHandleAPITestSuite) createTheme(handle string) string {
	payload, err := json.Marshal(CreateThemeRequest{
		Handle:      handle,
		DisplayName: "Usages Test Theme " + handle,
		Theme:       testTheme,
	})
	suite.Require().NoError(err)

	status, body := suite.do(http.MethodPost, testServerURL+themeBasePath, payload)
	suite.Require().Equal(http.StatusCreated, status, "create theme failed: %s", body)

	var created ThemeResponse
	suite.Require().NoError(json.Unmarshal(body, &created))
	suite.Require().NotEmpty(created.ID)
	return created.ID
}

func (suite *DesignUsagesHandleAPITestSuite) createLayout(handle string) string {
	payload, err := json.Marshal(CreateLayoutRequest{
		Handle:      handle,
		DisplayName: "Usages Test Layout " + handle,
		Layout:      testLayout,
	})
	suite.Require().NoError(err)

	status, body := suite.do(http.MethodPost, testServerURL+layoutBasePath, payload)
	suite.Require().Equal(http.StatusCreated, status, "create layout failed: %s", body)

	var created LayoutResponse
	suite.Require().NoError(json.Unmarshal(body, &created))
	suite.Require().NotEmpty(created.ID)
	return created.ID
}

// getUsages GETs a theme or layout usages endpoint and requires a 200 response.
func (suite *DesignUsagesHandleAPITestSuite) getUsages(basePath, id string) DependenciesResponse {
	status, body := suite.do(http.MethodGet, testServerURL+basePath+"/"+id+"/usages", nil)
	suite.Require().Equal(http.StatusOK, status, "usages failed: %s", body)

	var usages DependenciesResponse
	suite.Require().NoError(json.Unmarshal(body, &usages))
	return usages
}

// createApplication creates an application optionally bound to a theme and layout. ouId and type
// are required by the application API; only the design bindings vary per test.
func (suite *DesignUsagesHandleAPITestSuite) createApplication(name, themeID, layoutID string) string {
	body := map[string]interface{}{
		"name":        name,
		"description": "Application for design usages integration tests",
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

	status, respBody := suite.do(http.MethodPost, testServerURL+"/applications", payload)
	suite.Require().Equal(http.StatusCreated, status, "create application failed: %s", respBody)

	var created struct {
		ID string `json:"id"`
	}
	suite.Require().NoError(json.Unmarshal(respBody, &created))
	suite.Require().NotEmpty(created.ID)
	return created.ID
}

func (suite *DesignUsagesHandleAPITestSuite) deleteApplication(id string) {
	if id == "" {
		return
	}
	if err := testutils.DeleteApplication(id); err != nil {
		suite.T().Logf("Failed to delete application %s: %v", id, err)
	}
}

// deleteResource removes a theme or layout, tolerating an already-deleted resource.
func (suite *DesignUsagesHandleAPITestSuite) deleteResource(basePath, id string) {
	if id == "" {
		return
	}

	status, body := suite.do(http.MethodDelete, testServerURL+basePath+"/"+id, nil)
	if status != http.StatusNoContent && status != http.StatusNotFound {
		suite.T().Logf("Failed to delete %s/%s: status %d: %s", basePath, id, status, body)
	}
}

// do issues a request and returns the status code and response body.
func (suite *DesignUsagesHandleAPITestSuite) do(method, target string, body []byte) (int, []byte) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, target, reader)
	suite.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			suite.T().Logf("Failed to close response body: %v", closeErr)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, respBody
}

// errorCode decodes the error code from an API error response body.
func (suite *DesignUsagesHandleAPITestSuite) errorCode(body []byte) string {
	var errResp ErrorResponse
	suite.Require().NoError(json.Unmarshal(body, &errResp), "body: %s", body)
	return errResp.Code
}
