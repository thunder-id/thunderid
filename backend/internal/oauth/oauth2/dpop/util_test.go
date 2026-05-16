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

package dpop

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type UtilTestSuite struct {
	suite.Suite
}

func TestUtilTestSuite(t *testing.T) {
	suite.Run(t, new(UtilTestSuite))
}

func (suite *UtilTestSuite) TestIsDPoPAuth() {
	testCases := []struct {
		name     string
		header   string
		expected bool
	}{
		{"DPoPScheme", "DPoP token123", true},
		{"DPoPSchemeLowercase", "dpop token123", true},
		{"DPoPSchemeMixedCase", "DpOp token123", true},
		{"DPoPSchemeNoToken", "DPoP", true},
		{"BearerScheme", "Bearer token123", false},
		{"EmptyHeader", "", false},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, IsDPoPAuth(tc.header))
		})
	}
}

func (suite *UtilTestSuite) TestExtractDPoPToken() {
	testCases := []struct {
		name        string
		authHeader  string
		expected    string
		expectError bool
		errorMsg    string
	}{
		{"ValidDPoPToken", "DPoP token123", "token123", false, ""},
		{"CaseInsensitiveDPoP", "dpop token123", "token123", false, ""},
		{"UpperCaseDPoP", "DPOP token123", "token123", false, ""},
		{"EmptyHeader", "", "", true, "missing Authorization header"},
		{"BearerScheme", "Bearer token123", "", true, "invalid Authorization header format"},
		{"MissingToken", "DPoP ", "", true, "missing access token"},
		{"OnlyScheme", "DPoP", "", true, "invalid Authorization header format"},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result, err := ExtractDPoPToken(tc.authHeader)
			if tc.expectError {
				assert.Error(t, err)
				assert.Empty(t, result)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
