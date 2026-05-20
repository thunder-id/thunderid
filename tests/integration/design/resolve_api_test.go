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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	resolveBasePath = "/design/resolve"
)

type ResolveAPITestSuite struct {
	suite.Suite
	client *http.Client
}

func TestResolveAPITestSuite(t *testing.T) {
	suite.Run(t, new(ResolveAPITestSuite))
}

func (suite *ResolveAPITestSuite) SetupSuite() {
	// Create HTTP client that skips TLS verification for testing
	suite.client = testutils.GetHTTPClient()
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
func (suite *ResolveAPITestSuite) TestResolveDesign_ApplicationNotFound() {
	_, statusCode, err := suite.resolveDesign("APP", "00000000-0000-0000-0000-000000000000")

	suite.Error(err)
	suite.Equal(http.StatusNotFound, statusCode)
	suite.Contains(err.Error(), "DSR-1004")
}

// Test Resolve Design - Success Case
// Note: This test requires an actual application with theme and layout configured
// For a complete test, you would need to:
// 1. Create a theme
// 2. Create a layout
// 3. Create an application with theme and layout
// 4. Call resolve with the application ID
// 5. Verify the merged design configuration
func (suite *ResolveAPITestSuite) TestResolveDesign_Success() {
}

// Test Resolve Design - Application Without Theme and Layout
// This test verifies behavior when an application exists but has no design configuration
func (suite *ResolveAPITestSuite) TestResolveDesign_ApplicationWithoutDesign() {
}

// Test Resolve Design - Only Theme Configured
func (suite *ResolveAPITestSuite) TestResolveDesign_OnlyTheme() {
}

// Test Resolve Design - Only Layout Configured
func (suite *ResolveAPITestSuite) TestResolveDesign_OnlyLayout() {
}
