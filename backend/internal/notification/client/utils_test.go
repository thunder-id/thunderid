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

package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHTTPHeaders_Valid(t *testing.T) {
	headersString := "Authorization: Bearer token, X-Custom-Header: custom_value"
	headers, err := parseHTTPHeaders(headersString)

	assert.NoError(t, err)
	assert.NotNil(t, headers)
	assert.Equal(t, "Bearer token", headers["Authorization"])
	assert.Equal(t, "custom_value", headers["X-Custom-Header"])
}

func TestParseHTTPHeaders_EmptyString(t *testing.T) {
	headersString := "   "
	headers, err := parseHTTPHeaders(headersString)

	assert.NoError(t, err)
	assert.NotNil(t, headers)
	assert.Empty(t, headers)
}

func TestParseHTTPHeaders_Invalid(t *testing.T) {
	headersString := "Invalid Header Format"
	headers, err := parseHTTPHeaders(headersString)

	assert.Error(t, err)
	assert.Nil(t, headers)
	assert.Contains(t, err.Error(), "invalid HTTP header format")
}
