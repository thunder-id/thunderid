/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
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

package setup

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAdminCredentials_ParsesBlock(t *testing.T) {
	output := "some noise\n" +
		"Admin credentials:\n" +
		"  Username: admin\n" +
		"  Password: abc123\n" +
		"  Sign in to the Console with these credentials.\n" +
		"\n" +
		"trailing noise\n"

	creds := parseAdminCredentials(output)

	assert.NotNil(t, creds)
	assert.Equal(t, "admin", creds.Username)
	assert.Equal(t, "abc123", creds.Password)
}

func TestParseAdminCredentials_CRLF(t *testing.T) {
	output := "some noise\r\n" +
		"Admin credentials:\r\n" +
		"  Username: admin\r\n" +
		"  Password: abc123\r\n" +
		"  Sign in to the Console with these credentials.\r\n" +
		"\r\n" +
		"trailing noise\r\n"

	creds := parseAdminCredentials(output)

	assert.NotNil(t, creds)
	assert.Equal(t, "admin", creds.Username)
	assert.Equal(t, "abc123", creds.Password)
}

func TestParseAdminCredentials_NoBlockReturnsNil(t *testing.T) {
	assert.Nil(t, parseAdminCredentials("no credentials here at all"))
}

func TestGenerateAdminPassword(t *testing.T) {
	const special = "@#%+=_.?-"
	for i := 0; i < 100; i++ {
		pw := GenerateAdminPassword()
		assert.Len(t, pw, 12)
		assert.True(t, strings.ContainsAny(pw, "0123456789"), "must contain a digit: %q", pw)
		assert.True(t, strings.ContainsAny(pw, special), "must contain a special char: %q", pw)
	}
	assert.NotEqual(t, GenerateAdminPassword(), GenerateAdminPassword())
}
