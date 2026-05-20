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
	testServerURL = "https://localhost:8095"
	themeBasePath = "/design/themes"
)

var (
	testTheme = json.RawMessage(`{
		"direction": "ltr",
		"defaultColorScheme": "dark",
		"colorSchemes": {
			"light": {
				"palette": {
					"primary": {
						"main": "#BD93F9",
						"light": "#CBA6F7",
						"dark": "#9A7FD1",
						"contrastText": "#1E1F29"
					},
					"secondary": {
						"main": "#FF79C6",
						"light": "#FF92D0",
						"dark": "#E56FB3",
						"contrastText": "#1E1F29"
					},
					"background": {
						"main": "#F8F8F2",
						"light": "#FFFFFF",
						"dark": "#ECECEC",
						"contrastText": "#1E1F29"
					},
					"text": {
						"primary": "#1E1F29",
						"secondary": "#44475A",
						"disabled": "#00000061"
					},
					"divider": "#00000012",
					"error": {
						"main": "#FF5555",
						"light": "#FF6E6E",
						"dark": "#E04E4E",
						"contrastText": "#FFFFFF"
					},
					"warning": {
						"main": "#F1FA8C",
						"light": "#F4FB9C",
						"dark": "#DDEB7A",
						"contrastText": "#1E1F29"
					},
					"info": {
						"main": "#8BE9FD",
						"light": "#9CEBFE",
						"dark": "#6FDCEB",
						"contrastText": "#1E1F29"
					},
					"success": {
						"main": "#50FA7B",
						"light": "#6CFA8E",
						"dark": "#3CE66A",
						"contrastText": "#1E1F29"
					}
				}
			},
			"dark": {
				"palette": {
					"primary": {
						"main": "#BD93F9",
						"light": "#CBA6F7",
						"dark": "#9A7FD1",
						"contrastText": "#F8F8F2"
					},
					"secondary": {
						"main": "#FF79C6",
						"light": "#FF92D0",
						"dark": "#E56FB3",
						"contrastText": "#F8F8F2"
					},
					"background": {
						"main": "#282A36",
						"light": "#44475A",
						"dark": "#1E1F29",
						"contrastText": "#F8F8F2"
					},
					"text": {
						"primary": "#F8F8F2",
						"secondary": "#6272A4",
						"disabled": "#FFFFFF61"
					},
					"divider": "#FFFFFF12",
					"error": {
						"main": "#FF5555",
						"light": "#FF6E6E",
						"dark": "#E04E4E",
						"contrastText": "#F8F8F2"
					},
					"warning": {
						"main": "#F1FA8C",
						"light": "#F4FB9C",
						"dark": "#DDEB7A",
						"contrastText": "#282A36"
					},
					"info": {
						"main": "#8BE9FD",
						"light": "#9CEBFE",
						"dark": "#6FDCEB",
						"contrastText": "#282A36"
					},
					"success": {
						"main": "#50FA7B",
						"light": "#6CFA8E",
						"dark": "#3CE66A",
						"contrastText": "#282A36"
					}
				}
			}
		},
		"components": {
			"Button": {
				"root": {
					"shape": {
						"borderRadius": "8px"
					}
				}
			}
		},
		"shape": {
			"borderRadius": "8px"
		},
		"typography": {
			"fontFamily": "'Roboto', sans-serif",
			"h1": {
				"fontSize": "2.5rem",
				"fontWeight": 500,
				"fontFamily": "'Roboto', sans-serif",
				"lineHeight": 1.2
			},
			"h2": {
				"fontSize": "2rem",
				"fontWeight": 500,
				"fontFamily": "'Roboto', sans-serif",
				"lineHeight": 1.3
			},
			"h3": {
				"fontSize": "1.75rem",
				"fontWeight": 500,
				"fontFamily": "'Roboto', sans-serif",
				"lineHeight": 1.4
			},
			"h4": {
				"fontSize": "1.5rem",
				"fontWeight": 500,
				"fontFamily": "'Roboto', sans-serif",
				"lineHeight": 1.5
			},
			"h5": {
				"fontSize": "1.25rem",
				"fontWeight": 500,
				"fontFamily": "'Roboto', sans-serif",
				"lineHeight": 1.6
			},
			"h6": {
				"fontSize": "1rem",
				"fontWeight": 500,
				"fontFamily": "'Roboto', sans-serif",
				"lineHeight": 1.6
			},
			"body1": {
				"fontSize": "1rem",
				"fontWeight": 400,
				"fontFamily": "'Roboto', sans-serif",
				"lineHeight": 1.5
			},
			"body2": {
				"fontSize": "0.875rem",
				"fontWeight": 400,
				"fontFamily": "'Roboto', sans-serif",
				"lineHeight": 1.43
			},
			"subtitle1": {
				"fontSize": "1rem",
				"fontWeight": 500,
				"fontFamily": "'Roboto', sans-serif",
				"lineHeight": 1.75
			},
			"subtitle2": {
				"fontSize": "0.875rem",
				"fontWeight": 500,
				"fontFamily": "'Roboto', sans-serif",
				"lineHeight": 1.57
			},
			"caption": {
				"fontSize": "0.75rem",
				"fontWeight": 400,
				"fontFamily": "'Roboto', sans-serif",
				"lineHeight": 1.66
			}
		}
	}`)

	testTheme2 = json.RawMessage(`{
		"direction": "ltr",
		"defaultColorScheme": "light",
		"colorSchemes": {
			"light": {
				"palette": {
					"primary": {
						"main": "#1976d2",
						"light": "#42a5f5",
						"dark": "#1565c0",
						"contrastText": "#ffffff"
					},
					"secondary": {
						"main": "#dc004e",
						"light": "#e33371",
						"dark": "#9a0036",
						"contrastText": "#ffffff"
					}
				}
			}
		},
		"shape": {
			"borderRadius": "4px"
		},
		"typography": {
			"fontFamily": "'Inter', sans-serif"
		}
	}`)

	testThemeUpdate = json.RawMessage(`{
		"direction": "rtl",
		"defaultColorScheme": "dark",
		"colorSchemes": {
			"dark": {
				"palette": {
					"primary": {
						"main": "#90caf9",
						"light": "#e3f2fd",
						"dark": "#42a5f5",
						"contrastText": "#000000"
					},
					"secondary": {
						"main": "#f48fb1",
						"light": "#ffc1e3",
						"dark": "#bf5f82",
						"contrastText": "#000000"
					}
				}
			}
		},
		"shape": {
			"borderRadius": "12px"
		},
		"typography": {
			"fontFamily": "'Poppins', sans-serif"
		}
	}`)
)

var (
	sharedThemeID string // Shared theme created in SetupSuite
)

type ThemeAPITestSuite struct {
	suite.Suite
	client *http.Client
}

func TestThemeAPITestSuite(t *testing.T) {
	suite.Run(t, new(ThemeAPITestSuite))
}

func (suite *ThemeAPITestSuite) SetupSuite() {
	// Create HTTP client that skips TLS verification for testing
	suite.client = testutils.GetHTTPClient()

	// Create a shared theme that can be used by multiple tests
	sharedTheme := CreateThemeRequest{
		Handle:      "shared-test-theme",
		DisplayName: "Shared Test Theme",
		Description: "Test description",
		Theme:       testTheme,
	}
	theme, err := suite.createTheme(sharedTheme)
	suite.Require().NoError(err, "Failed to create shared theme")
	sharedThemeID = theme.ID
}

func (suite *ThemeAPITestSuite) TearDownSuite() {
	// Cleanup
	if sharedThemeID != "" {
		_ = suite.deleteTheme(sharedThemeID)
	}
}

// Helper function to create a theme
func (suite *ThemeAPITestSuite) createTheme(request CreateThemeRequest) (*ThemeResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal theme request: %w", err)
	}

	req, err := http.NewRequest("POST", testServerURL+themeBasePath, bytes.NewReader(payload))
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

	var theme ThemeResponse
	if err := json.Unmarshal(bodyBytes, &theme); err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	return &theme, nil
}

// Helper function to get a theme by ID
func (suite *ThemeAPITestSuite) getTheme(id string) (*ThemeResponse, error) {
	req, err := http.NewRequest("GET", testServerURL+themeBasePath+"/"+id, nil)
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

	var theme ThemeResponse
	if err := json.Unmarshal(bodyBytes, &theme); err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	return &theme, nil
}

// Helper function to list themes
func (suite *ThemeAPITestSuite) listThemes(limit, offset int) (*ThemeListResponse, error) {
	params := url.Values{}
	if limit > 0 {
		params.Add("limit", fmt.Sprintf("%d", limit))
	}
	if offset > 0 {
		params.Add("offset", fmt.Sprintf("%d", offset))
	}

	url := testServerURL + themeBasePath
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

	var listResponse ThemeListResponse
	if err := json.Unmarshal(bodyBytes, &listResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	return &listResponse, nil
}

// Helper function to update a theme
func (suite *ThemeAPITestSuite) updateTheme(id string, request UpdateThemeRequest) (*ThemeResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal theme request: %w", err)
	}

	req, err := http.NewRequest("PUT", testServerURL+themeBasePath+"/"+id, bytes.NewReader(payload))
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

	var theme ThemeResponse
	if err := json.Unmarshal(bodyBytes, &theme); err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	return &theme, nil
}

// Helper function to delete a theme
func (suite *ThemeAPITestSuite) deleteTheme(id string) error {
	req, err := http.NewRequest("DELETE", testServerURL+themeBasePath+"/"+id, nil)
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

// Create Theme - Success
func (suite *ThemeAPITestSuite) TestCreateTheme_Success() {
	request := CreateThemeRequest{
		Handle:      "test-theme-success",
		DisplayName: "Test Theme Success",
		Description: "Test description",
		Theme:       testTheme2,
	}

	theme, err := suite.createTheme(request)
	suite.Require().NoError(err)
	suite.Require().NotNil(theme)

	suite.NotEmpty(theme.ID)
	suite.Equal("Test Theme Success", theme.DisplayName)
	suite.NotEmpty(theme.Theme)

	// Cleanup
	_ = suite.deleteTheme(theme.ID)
}

// Create Theme - Validation Errors
func (suite *ThemeAPITestSuite) TestCreateTheme_ValidationErrors() {
	testCases := []struct {
		name        string
		requestBody string
		expectedErr string
	}{
		{
			name:        "Missing DisplayName",
			requestBody: `{"theme": {}}`,
			expectedErr: "THM-1005",
		},
		{
			name:        "Missing Theme",
			requestBody: `{"displayName": "Test", "handle": "test-handle"}`,
			expectedErr: "THM-1006",
		},
		{
			name:        "Invalid JSON Theme",
			requestBody: `{"displayName": "Test", "handle": "test-handle", "theme": invalid json}`,
			expectedErr: "THM-1001",
		},
		{
			name:        "Array Instead of Object",
			requestBody: `{"displayName": "Test", "handle": "test-handle", "theme": ["item1", "item2"]}`,
			expectedErr: "THM-1007",
		},
		{
			name:        "Primitive Instead of Object",
			requestBody: `{"displayName": "Test", "handle": "test-handle", "theme": "string"}`,
			expectedErr: "THM-1007",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req, err := http.NewRequest("POST", testServerURL+themeBasePath, bytes.NewReader([]byte(tc.requestBody)))
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

// Get Theme - Success
func (suite *ThemeAPITestSuite) TestGetTheme_Success() {
	suite.Require().NotEmpty(sharedThemeID, "Shared theme must be created in SetupSuite")

	theme, err := suite.getTheme(sharedThemeID)
	suite.Require().NoError(err)
	suite.Require().NotNil(theme)

	suite.Equal(sharedThemeID, theme.ID)
	suite.NotEmpty(theme.Theme)
}

// Get Theme - Not Found
func (suite *ThemeAPITestSuite) TestGetTheme_NotFound() {
	theme, err := suite.getTheme("00000000-0000-0000-0000-000000000000")
	suite.Error(err)
	suite.Nil(theme)
	suite.Contains(err.Error(), "THM-1003")
}

// List Themes - Success
func (suite *ThemeAPITestSuite) TestListThemes_Success() {
	suite.Require().NotEmpty(sharedThemeID, "Shared theme must be created in SetupSuite")

	response, err := suite.listThemes(0, 0)
	suite.Require().NoError(err)
	suite.Require().NotNil(response)

	suite.GreaterOrEqual(response.TotalResults, 1)
	suite.GreaterOrEqual(response.Count, 1)
	suite.NotEmpty(response.Themes)

	// Verify our shared theme is in the list
	found := false
	for _, theme := range response.Themes {
		if theme.ID == sharedThemeID {
			found = true
			suite.NotEmpty(theme.DisplayName)
			break
		}
	}
	suite.True(found, "Shared theme should be in the list")
}

// List Themes - Pagination
func (suite *ThemeAPITestSuite) TestListThemes_Pagination() {
	// Create additional themes for pagination testing
	theme1, err := suite.createTheme(CreateThemeRequest{
		Handle:      "pagination-theme-1",
		DisplayName: "Pagination Theme 1",
		Description: "Test description",
		Theme:       testTheme2,
	})
	suite.Require().NoError(err)
	defer suite.deleteTheme(theme1.ID)

	theme2, err := suite.createTheme(CreateThemeRequest{
		Handle:      "pagination-theme-2",
		DisplayName: "Pagination Theme 2",
		Description: "Test description",
		Theme:       testTheme2,
	})
	suite.Require().NoError(err)
	defer suite.deleteTheme(theme2.ID)

	// Test with limit
	response, err := suite.listThemes(2, 0)
	suite.Require().NoError(err)
	suite.Require().NotNil(response)

	suite.GreaterOrEqual(response.TotalResults, 3)
	suite.LessOrEqual(response.Count, 2)
	suite.LessOrEqual(len(response.Themes), 2)

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

// List Themes - Invalid Pagination Parameters
func (suite *ThemeAPITestSuite) TestListThemes_InvalidPagination() {
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
			expectedErr: "THM-1008",
		},
		{
			name:        "Invalid Offset - Negative",
			limit:       10,
			offset:      -1,
			expectedErr: "THM-1009",
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

			url := testServerURL + themeBasePath
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

// Update Theme - Success
func (suite *ThemeAPITestSuite) TestUpdateTheme_Success() {
	// Create a theme for update testing
	theme, err := suite.createTheme(CreateThemeRequest{
		Handle:      "test-theme-update",
		DisplayName: "Test Theme Update",
		Description: "Test description",
		Theme:       testTheme,
	})
	suite.Require().NoError(err)
	defer suite.deleteTheme(theme.ID)

	updateRequest := UpdateThemeRequest{
		Handle:      "test-theme-update",
		DisplayName: "Updated Test Theme",
		Description: "Test description",
		Theme:       testThemeUpdate,
	}

	updatedTheme, err := suite.updateTheme(theme.ID, updateRequest)
	suite.Require().NoError(err)
	suite.Require().NotNil(updatedTheme)

	suite.Equal(theme.ID, updatedTheme.ID)
	suite.Equal("Updated Test Theme", updatedTheme.DisplayName)
	suite.NotEmpty(updatedTheme.Theme)

	// Verify the update by getting the theme again
	retrievedTheme, err := suite.getTheme(theme.ID)
	suite.Require().NoError(err)
	suite.Equal(theme.ID, retrievedTheme.ID)
}

// Update Theme - Not Found
func (suite *ThemeAPITestSuite) TestUpdateTheme_NotFound() {
	updateRequest := UpdateThemeRequest{
		Handle:      "test-theme-not-found",
		DisplayName: "Test Theme",
		Description: "Test description",
		Theme:       testThemeUpdate,
	}

	theme, err := suite.updateTheme("00000000-0000-0000-0000-000000000000", updateRequest)
	suite.Error(err)
	suite.Nil(theme)
	suite.Contains(err.Error(), "THM-1003")
}

// Update Theme - Validation Errors
func (suite *ThemeAPITestSuite) TestUpdateTheme_ValidationErrors() {
	// Create a theme for update testing
	theme, err := suite.createTheme(CreateThemeRequest{
		Handle:      "test-theme-validation",
		DisplayName: "Test Theme Validation",
		Description: "Test description",
		Theme:       testTheme,
	})
	suite.Require().NoError(err)
	defer suite.deleteTheme(theme.ID)

	testCases := []struct {
		name        string
		requestBody string
		expectedErr string
	}{
		{
			name:        "Missing DisplayName",
			requestBody: `{"theme": {}}`,
			expectedErr: "THM-1005",
		},
		{
			name:        "Missing Theme",
			requestBody: `{"displayName": "Test", "handle": "test-theme-validation"}`,
			expectedErr: "THM-1006",
		},
		{
			name:        "Invalid JSON Theme",
			requestBody: `{"displayName": "Test", "handle": "test-theme-validation", "theme": invalid json}`,
			expectedErr: "THM-1001",
		},
		{
			name:        "Array Instead of Object",
			requestBody: `{"displayName": "Test", "handle": "test-theme-validation", "theme": ["item1", "item2"]}`,
			expectedErr: "THM-1007",
		},
		{
			name:        "Primitive Instead of Object",
			requestBody: `{"displayName": "Test", "handle": "test-theme-validation", "theme": "string"}`,
			expectedErr: "THM-1007",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req, err := http.NewRequest("PUT", testServerURL+themeBasePath+"/"+theme.ID, bytes.NewReader([]byte(tc.requestBody)))
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

// Delete Theme - Success
func (suite *ThemeAPITestSuite) TestDeleteTheme_Success() {
	// Create a theme for delete testing
	theme, err := suite.createTheme(CreateThemeRequest{
		Handle:      "test-theme-delete",
		DisplayName: "Test Theme Delete",
		Description: "Test description",
		Theme:       testTheme,
	})
	suite.Require().NoError(err)

	err = suite.deleteTheme(theme.ID)
	suite.NoError(err)

	// Verify deletion by trying to get the theme
	_, err = suite.getTheme(theme.ID)
	suite.Error(err)
	suite.Contains(err.Error(), "THM-1003")
}

// Delete Theme - Not Found
func (suite *ThemeAPITestSuite) TestDeleteTheme_NotFound() {
	err := suite.deleteTheme("00000000-0000-0000-0000-000000000000")
	// Delete should not error for non-existent theme (returns 204 or 404)
	suite.NoError(err)
}
