/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package design

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// Helper function to create a layout
func (suite *LayoutAPITestSuite) createLayout(request CreateLayoutRequest) (*LayoutResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal layout request: %w", err)
	}

	req, err := http.NewRequest("POST", testServerURL+layoutBasePath, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var errResp ErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil {
			return nil, fmt.Errorf("expected status 201, got %d. Code: %s, Message: %s", resp.StatusCode, errResp.Code, errResp.Message.DefaultValue)
		}
		return nil, fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var layout LayoutResponse
	if err := json.Unmarshal(bodyBytes, &layout); err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	return &layout, nil
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

// Helper function to update a layout
func (suite *LayoutAPITestSuite) updateLayout(id string, request UpdateLayoutRequest) (*LayoutResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal layout request: %w", err)
	}

	req, err := http.NewRequest("PUT", testServerURL+layoutBasePath+"/"+id, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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

// Helper function to delete a layout
func (suite *LayoutAPITestSuite) deleteLayout(id string) error {
	req, err := http.NewRequest("DELETE", testServerURL+layoutBasePath+"/"+id, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := suite.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err == nil {
			return fmt.Errorf("expected status 204 or 404, got %d. Code: %s, Message: %s", resp.StatusCode, errResp.Code, errResp.Message.DefaultValue)
		}
		return fmt.Errorf("expected status 204 or 404, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
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
		requestBody string
		expectedErr string
	}{
		{
			name:        "Missing DisplayName",
			requestBody: `{"layout": {}}`,
			expectedErr: "LAY-1005",
		},
		{
			name:        "Missing Layout",
			requestBody: `{"displayName": "Test", "handle": "test-handle"}`,
			expectedErr: "LAY-1006",
		},
		{
			name:        "Invalid JSON Layout",
			requestBody: `{"displayName": "Test", "handle": "test-handle", "layout": invalid json}`,
			expectedErr: "LAY-1001",
		},
		{
			name:        "Array Instead of Object",
			requestBody: `{"displayName": "Test", "handle": "test-handle", "layout": ["item1", "item2"]}`,
			expectedErr: "LAY-1007",
		},
		{
			name:        "Primitive Instead of Object",
			requestBody: `{"displayName": "Test", "handle": "test-handle", "layout": "string"}`,
			expectedErr: "LAY-1007",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req, err := http.NewRequest("POST", testServerURL+layoutBasePath, bytes.NewReader([]byte(tc.requestBody)))
			suite.Require().NoError(err)
			req.Header.Set("Content-Type", "application/json")

			resp, err := suite.client.Do(req)
			suite.Require().NoError(err)
			defer resp.Body.Close()

			suite.Equal(http.StatusBadRequest, resp.StatusCode)

			bodyBytes, err := io.ReadAll(resp.Body)
			suite.Require().NoError(err)

			var errResp ErrorResponse
			err = json.Unmarshal(bodyBytes, &errResp)
			suite.Require().NoError(err)
			suite.Equal(tc.expectedErr, errResp.Code)
		})
	}
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

// Update Layout - Not Found
func (suite *LayoutAPITestSuite) TestUpdateLayout_NotFound() {
	updateRequest := UpdateLayoutRequest{
		Handle:      "test-layout-not-found",
		DisplayName: "Test Layout",
		Description: "Test description",
		Layout:      testLayoutUpdate,
	}

	layout, err := suite.updateLayout("00000000-0000-0000-0000-000000000000", updateRequest)
	suite.Error(err)
	suite.Nil(layout)
	suite.Contains(err.Error(), "LAY-1003")
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
		requestBody string
		expectedErr string
	}{
		{
			name:        "Missing DisplayName",
			requestBody: `{"layout": {}}`,
			expectedErr: "LAY-1005",
		},
		{
			name:        "Missing Layout",
			requestBody: `{"displayName": "Test", "handle": "test-layout-validation"}`,
			expectedErr: "LAY-1006",
		},
		{
			name:        "Invalid JSON Layout",
			requestBody: `{"displayName": "Test", "handle": "test-layout-validation", "layout": invalid json}`,
			expectedErr: "LAY-1001",
		},
		{
			name:        "Array Instead of Object",
			requestBody: `{"displayName": "Test", "handle": "test-layout-validation", "layout": ["item1", "item2"]}`,
			expectedErr: "LAY-1007",
		},
		{
			name:        "Primitive Instead of Object",
			requestBody: `{"displayName": "Test", "handle": "test-layout-validation", "layout": "string"}`,
			expectedErr: "LAY-1007",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req, err := http.NewRequest("PUT", testServerURL+layoutBasePath+"/"+layout.ID, bytes.NewReader([]byte(tc.requestBody)))
			suite.Require().NoError(err)
			req.Header.Set("Content-Type", "application/json")

			resp, err := suite.client.Do(req)
			suite.Require().NoError(err)
			defer resp.Body.Close()

			suite.Equal(http.StatusBadRequest, resp.StatusCode)

			bodyBytes, err := io.ReadAll(resp.Body)
			suite.Require().NoError(err)

			var errResp ErrorResponse
			err = json.Unmarshal(bodyBytes, &errResp)
			suite.Require().NoError(err)
			suite.Equal(tc.expectedErr, errResp.Code)
		})
	}
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
