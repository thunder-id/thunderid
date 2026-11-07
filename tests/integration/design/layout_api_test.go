// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package design

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	layoutBasePath = "/design/layouts"
)

var (
	testLayout = json.RawMessage(`{
		"header": {
			"logo": {
				"url": "https://example.com/logo.png",
				"altText": "Company Logo"
			},
			"navigation": ["Home", "Products", "About"]
		},
		"footer": {
			"copyright": "© 2025 Company",
			"links": ["Privacy", "Terms"]
		}
	}`)

	testLayout2 = json.RawMessage(`{
		"header": {
			"logo": {
				"url": "https://example.com/logo2.png",
				"altText": "Brand Logo"
			}
		}
	}`)

	testLayoutUpdate = json.RawMessage(`{
		"header": {
			"logo": {
				"url": "https://example.com/updated-logo.png",
				"altText": "Updated Logo"
			},
			"navigation": ["Home", "Products", "Services", "Contact"]
		},
		"footer": {
			"copyright": "© 2025 Updated Company",
			"links": ["Privacy", "Terms", "Cookies"]
		}
	}`)
)

var (
	sharedLayoutID string // Shared layout created in SetupSuite
)

type LayoutAPITestSuite struct {
	suite.Suite
	client *http.Client
}

func TestLayoutAPITestSuite(t *testing.T) {
	suite.Run(t, new(LayoutAPITestSuite))
}

func (suite *LayoutAPITestSuite) SetupSuite() {
	// Create HTTP client that skips TLS verification for testing
	suite.client = testutils.GetHTTPClient()

	// Create a shared layout that can be used by multiple tests
	sharedLayout := CreateLayoutRequest{
		Handle:      "shared-test-layout",
		DisplayName: "Shared Test Layout",
		Description: "Shared layout for testing",
		Layout:      testLayout,
	}
	layout, err := suite.createLayout(sharedLayout)
	suite.Require().NoError(err, "Failed to create shared layout")
	sharedLayoutID = layout.ID
}

func (suite *LayoutAPITestSuite) TearDownSuite() {
	// Cleanup
	if sharedLayoutID != "" {
		_ = suite.deleteLayout(sharedLayoutID)
	}
}

// createLayout creates a layout through an import with the management API key, the only way a data plane
// takes one, and returns it as the API reads it.
func (suite *LayoutAPITestSuite) createLayout(request CreateLayoutRequest) (*LayoutResponse, error) {
	id, err := testutils.ImportResource("layout", request, false)
	if err != nil {
		return nil, err
	}
	return suite.getLayout(id)
}

// Helper function to get a layout by ID
func (suite *LayoutAPITestSuite) getLayout(id string) (*LayoutResponse, error) {
	req, err := http.NewRequest("GET", testServerURL+layoutBasePath+"/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil {
			return nil, fmt.Errorf("expected status 200, got %d. Code: %s, Message: %s", resp.StatusCode, errResp.Code, errResp.Message.DefaultValue)
		}
		return nil, fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var layout LayoutResponse
	if err := json.Unmarshal(bodyBytes, &layout); err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	return &layout, nil
}

// Helper function to list layouts
func (suite *LayoutAPITestSuite) listLayouts(limit, offset int) (*LayoutListResponse, error) {
	params := url.Values{}
	if limit > 0 {
		params.Add("limit", fmt.Sprintf("%d", limit))
	}
	if offset > 0 {
		params.Add("offset", fmt.Sprintf("%d", offset))
	}

	url := testServerURL + layoutBasePath
	if len(params) > 0 {
		url += "?" + params.Encode()
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil {
			return nil, fmt.Errorf("expected status 200, got %d. Code: %s, Message: %s", resp.StatusCode, errResp.Code, errResp.Message.DefaultValue)
		}
		return nil, fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var listResponse LayoutListResponse
	if err := json.Unmarshal(bodyBytes, &listResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	return &listResponse, nil
}

// updateLayout updates a layout through an import with the management API key and returns it as the API
// reads it.
func (suite *LayoutAPITestSuite) updateLayout(id string, request UpdateLayoutRequest) (*LayoutResponse, error) {
	document := layoutImportDocument{ID: id, UpdateLayoutRequest: request}
	if _, err := testutils.ImportResource("layout", document, true); err != nil {
		return nil, err
	}
	return suite.getLayout(id)
}

// deleteLayout deletes a layout through an import deletion. Deleting an absent layout succeeds.
func (suite *LayoutAPITestSuite) deleteLayout(id string) error {
	return testutils.DeleteImportedResource("layout", id)
}

// importOutcome imports a raw layout document and returns the outcome, whether it succeeded or not.
func (suite *LayoutAPITestSuite) importOutcome(document string) testutils.ImportItem {
	item, err := testutils.ImportResourceOutcome("layout", json.RawMessage(document), true)
	suite.Require().NoError(err)
	return item
}

// sendWithToken sends a request with the suite's token and returns the status and the error code.
func (suite *LayoutAPITestSuite) sendWithToken(method, target, body string) (int, string) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, target, reader)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	var errResp ErrorResponse
	suite.Require().NoError(json.Unmarshal(bodyBytes, &errResp), "body: %s", bodyBytes)
	return resp.StatusCode, errResp.Code
}

// Create Layout - Success
func (suite *LayoutAPITestSuite) TestCreateLayout_Success() {
	request := CreateLayoutRequest{
		Handle:      "test-layout-success",
		DisplayName: "Test Layout Success",
		Description: "Test layout for success case",
		Layout:      testLayout2,
	}

	layout, err := suite.createLayout(request)
	suite.Require().NoError(err)
	suite.Require().NotNil(layout)

	suite.NotEmpty(layout.ID)
	suite.Equal("Test Layout Success", layout.DisplayName)
	suite.Equal("Test layout for success case", layout.Description)
	suite.NotEmpty(layout.Layout)

	// Cleanup
	_ = suite.deleteLayout(layout.ID)
}

// Create Layout - Validation Errors
func (suite *LayoutAPITestSuite) TestCreateLayout_ValidationErrors() {
	testCases := []struct {
		name        string
		document    string
		expectedErr string
	}{
		{
			name:        "Missing DisplayName",
			document:    `{"layout": {}}`,
			expectedErr: "LAY-1005",
		},
		{
			name:        "Array Instead of Object",
			document:    `{"displayName": "Test", "handle": "test-handle", "layout": ["item1", "item2"]}`,
			expectedErr: "LAY-1007",
		},
		{
			name:        "Primitive Instead of Object",
			document:    `{"displayName": "Test", "handle": "test-handle", "layout": "string"}`,
			expectedErr: "LAY-1007",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			item := suite.importOutcome(tc.document)
			suite.Equal("failed", item.Status)
			suite.Equal(tc.expectedErr, item.Code)
		})
	}
}

// TestCreateLayout_MalformedBody checks that the handler refuses a body that is not JSON before
// authorization, so a token is answered as on any server.
func (suite *LayoutAPITestSuite) TestCreateLayout_MalformedBody() {
	status, code := suite.sendWithToken(http.MethodPost, testServerURL+layoutBasePath,
		`{"displayName": "Test", "handle": "test-handle", "layout": invalid json}`)

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("LAY-1001", code)
}

// Get Layout - Success
func (suite *LayoutAPITestSuite) TestGetLayout_Success() {
	suite.Require().NotEmpty(sharedLayoutID, "Shared layout must be created in SetupSuite")

	layout, err := suite.getLayout(sharedLayoutID)
	suite.Require().NoError(err)
	suite.Require().NotNil(layout)

	suite.Equal(sharedLayoutID, layout.ID)
	suite.NotEmpty(layout.Layout)
}

// Get Layout - Not Found
func (suite *LayoutAPITestSuite) TestGetLayout_NotFound() {
	layout, err := suite.getLayout("00000000-0000-0000-0000-000000000000")
	suite.Error(err)
	suite.Nil(layout)
	suite.Contains(err.Error(), "LAY-1003")
}

// List Layouts - Success
func (suite *LayoutAPITestSuite) TestListLayouts_Success() {
	suite.Require().NotEmpty(sharedLayoutID, "Shared layout must be created in SetupSuite")

	response, err := suite.listLayouts(0, 0)
	suite.Require().NoError(err)
	suite.Require().NotNil(response)

	suite.GreaterOrEqual(response.TotalResults, 1)
	suite.GreaterOrEqual(response.Count, 1)
	suite.NotEmpty(response.Layouts)

	// Verify our shared layout is in the list
	found := false
	for _, layout := range response.Layouts {
		if layout.ID == sharedLayoutID {
			found = true
			suite.NotEmpty(layout.DisplayName)
			break
		}
	}
	suite.True(found, "Shared layout should be in the list")
}

// List Layouts - Pagination
func (suite *LayoutAPITestSuite) TestListLayouts_Pagination() {
	// Create additional layouts for pagination testing
	layout1, err := suite.createLayout(CreateLayoutRequest{
		Handle:      "pagination-layout-1",
		DisplayName: "Pagination Layout 1",
		Description: "Layout for pagination test",
		Layout:      testLayout2,
	})
	suite.Require().NoError(err)
	defer suite.deleteLayout(layout1.ID)

	layout2, err := suite.createLayout(CreateLayoutRequest{
		Handle:      "pagination-layout-2",
		DisplayName: "Pagination Layout 2",
		Description: "Layout for pagination test",
		Layout:      testLayout2,
	})
	suite.Require().NoError(err)
	defer suite.deleteLayout(layout2.ID)

	// Test with limit
	response, err := suite.listLayouts(2, 0)
	suite.Require().NoError(err)
	suite.Require().NotNil(response)

	suite.GreaterOrEqual(response.TotalResults, 3)
	suite.LessOrEqual(response.Count, 2)
	suite.LessOrEqual(len(response.Layouts), 2)

	// Test pagination links
	if response.TotalResults > response.Count {
		suite.NotEmpty(response.Links)
		hasNext := false
		for _, link := range response.Links {
			if link.Rel == "next" {
				hasNext = true
				break
			}
		}
		suite.True(hasNext, "Should have next link when there are more results")
	}
}

// List Layouts - Invalid Pagination Parameters
func (suite *LayoutAPITestSuite) TestListLayouts_InvalidPagination() {
	testCases := []struct {
		name        string
		limit       int
		offset      int
		expectedErr string
	}{
		{
			name:        "Invalid Limit - Zero",
			limit:       0,
			offset:      0,
			expectedErr: "", // When limit is 0, default is applied, so no error
		},
		{
			name:        "Invalid Limit - Negative",
			limit:       -1,
			offset:      0,
			expectedErr: "LAY-1009",
		},
		{
			name:        "Invalid Offset - Negative",
			limit:       10,
			offset:      -1,
			expectedErr: "LAY-1010",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			params := url.Values{}
			if tc.limit != 0 {
				params.Add("limit", fmt.Sprintf("%d", tc.limit))
			}
			if tc.offset != 0 {
				params.Add("offset", fmt.Sprintf("%d", tc.offset))
			}

			url := testServerURL + layoutBasePath
			if len(params) > 0 {
				url += "?" + params.Encode()
			}

			req, err := http.NewRequest("GET", url, nil)
			suite.Require().NoError(err)

			resp, err := suite.client.Do(req)
			suite.Require().NoError(err)
			defer resp.Body.Close()

			if tc.expectedErr == "" {
				suite.Equal(http.StatusOK, resp.StatusCode)
			} else {
				suite.Equal(http.StatusBadRequest, resp.StatusCode)

				bodyBytes, err := io.ReadAll(resp.Body)
				suite.Require().NoError(err)

				var errResp ErrorResponse
				err = json.Unmarshal(bodyBytes, &errResp)
				suite.Require().NoError(err)
				suite.Equal(tc.expectedErr, errResp.Code)
			}
		})
	}
}

// Update Layout - Success
func (suite *LayoutAPITestSuite) TestUpdateLayout_Success() {
	// Create a layout for update testing
	layout, err := suite.createLayout(CreateLayoutRequest{
		Handle:      "test-layout-update",
		DisplayName: "Test Layout Update",
		Description: "Original description",
		Layout:      testLayout,
	})
	suite.Require().NoError(err)
	defer suite.deleteLayout(layout.ID)

	updateRequest := UpdateLayoutRequest{
		Handle:      "test-layout-update",
		DisplayName: "Updated Test Layout",
		Description: "Updated description",
		Layout:      testLayoutUpdate,
	}

	updatedLayout, err := suite.updateLayout(layout.ID, updateRequest)
	suite.Require().NoError(err)
	suite.Require().NotNil(updatedLayout)

	suite.Equal(layout.ID, updatedLayout.ID)
	suite.Equal("Updated Test Layout", updatedLayout.DisplayName)
	suite.Equal("Updated description", updatedLayout.Description)
	suite.NotEmpty(updatedLayout.Layout)

	// Verify the update by getting the layout again
	retrievedLayout, err := suite.getLayout(layout.ID)
	suite.Require().NoError(err)
	suite.Equal(layout.ID, retrievedLayout.ID)
}

// TestWritesWithTokenAreForbidden checks that a data plane takes no layout write from a token, whatever
// its scopes, and that a refused write changes nothing.
func (suite *LayoutAPITestSuite) TestWritesWithTokenAreForbidden() {
	before, err := suite.getLayout(sharedLayoutID)
	suite.Require().NoError(err)

	writes := []struct {
		method string
		target string
		body   string
	}{
		{http.MethodPost, testServerURL + layoutBasePath,
			`{"handle": "token-layout", "displayName": "Token Layout", "layout": {}}`},
		{http.MethodPut, testServerURL + layoutBasePath + "/" + sharedLayoutID,
			`{"handle": "shared-test-layout", "displayName": "Renamed", "layout": {}}`},
		{http.MethodDelete, testServerURL + layoutBasePath + "/" + sharedLayoutID, ""},
	}
	for _, write := range writes {
		status, code := suite.sendWithToken(write.method, write.target, write.body)
		suite.Equal(http.StatusForbidden, status, "%s %s", write.method, write.target)
		suite.Equal("SSE-4030", code)
	}

	after, err := suite.getLayout(sharedLayoutID)
	suite.Require().NoError(err)
	suite.Equal(before.DisplayName, after.DisplayName)
	list, err := suite.listLayouts(100, 0)
	suite.Require().NoError(err)
	for _, item := range list.Layouts {
		suite.NotEqual("token-layout", item.Handle)
	}
}

// Update Layout - Validation Errors
func (suite *LayoutAPITestSuite) TestUpdateLayout_ValidationErrors() {
	// Create a layout for update testing
	layout, err := suite.createLayout(CreateLayoutRequest{
		Handle:      "test-layout-validation",
		DisplayName: "Test Layout Validation",
		Description: "Test layout for validation",
		Layout:      testLayout,
	})
	suite.Require().NoError(err)
	defer suite.deleteLayout(layout.ID)

	testCases := []struct {
		name        string
		document    string
		expectedErr string
	}{
		{
			name:        "Missing DisplayName",
			document:    `{"id": %q, "layout": {}}`,
			expectedErr: "LAY-1005",
		},
		{
			name:        "Array Instead of Object",
			document:    `{"id": %q, "displayName": "Test", "handle": "test-layout-validation", "layout": ["item1", "item2"]}`,
			expectedErr: "LAY-1007",
		},
		{
			name:        "Primitive Instead of Object",
			document:    `{"id": %q, "displayName": "Test", "handle": "test-layout-validation", "layout": "string"}`,
			expectedErr: "LAY-1007",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			item := suite.importOutcome(fmt.Sprintf(tc.document, layout.ID))
			suite.Equal("failed", item.Status)
			suite.Equal(tc.expectedErr, item.Code)
		})
	}

	status, code := suite.sendWithToken(http.MethodPut, testServerURL+layoutBasePath+"/"+layout.ID,
		`{"displayName": "Test", "handle": "test-layout-validation", "layout": invalid json}`)
	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("LAY-1001", code)
}

// Delete Layout - Success
func (suite *LayoutAPITestSuite) TestDeleteLayout_Success() {
	// Create a layout for delete testing
	layout, err := suite.createLayout(CreateLayoutRequest{
		Handle:      "test-layout-delete",
		DisplayName: "Test Layout Delete",
		Description: "Layout to be deleted",
		Layout:      testLayout,
	})
	suite.Require().NoError(err)

	err = suite.deleteLayout(layout.ID)
	suite.NoError(err)

	// Verify deletion by trying to get the layout
	_, err = suite.getLayout(layout.ID)
	suite.Error(err)
	suite.Contains(err.Error(), "LAY-1003")
}

// Delete Layout - Not Found
func (suite *LayoutAPITestSuite) TestDeleteLayout_NotFound() {
	err := suite.deleteLayout("00000000-0000-0000-0000-000000000000")
	// Delete should not error for non-existent layout (returns 204 or 404)
	suite.NoError(err)
}
