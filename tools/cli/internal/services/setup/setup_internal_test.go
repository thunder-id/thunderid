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
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = orig

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}
	return string(out)
}

func TestPrintAdminCredentials_PrintsBlock(t *testing.T) {
	output := "some noise\n" +
		"Admin credentials:\n" +
		"  Username: admin\n" +
		"  Password: abc123\n" +
		"  Sign in to the Console with these credentials.\n" +
		"\n" +
		"trailing noise\n"

	captured := captureStdout(t, func() {
		printAdminCredentials(output)
	})

	assert.Contains(t, captured, "Admin credentials:")
	assert.Contains(t, captured, "Username: admin")
	assert.Contains(t, captured, "Password: abc123")
	assert.NotContains(t, captured, "trailing noise")
	assert.NotContains(t, captured, "some noise")
}

func TestPrintAdminCredentials_CRLFPrintsBlock(t *testing.T) {
	output := "some noise\r\n" +
		"Admin credentials:\r\n" +
		"  Username: admin\r\n" +
		"  Password: abc123\r\n" +
		"  Sign in to the Console with these credentials.\r\n" +
		"\r\n" +
		"trailing noise\r\n"

	captured := captureStdout(t, func() {
		printAdminCredentials(output)
	})

	assert.Contains(t, captured, "Admin credentials:")
	assert.Contains(t, captured, "Username: admin")
	assert.Contains(t, captured, "Password: abc123")
	assert.NotContains(t, captured, "trailing noise")
	assert.NotContains(t, captured, "some noise")
}

func TestPrintAdminCredentials_NoBlockPrintsNothing(t *testing.T) {
	captured := captureStdout(t, func() {
		printAdminCredentials("no credentials here at all")
	})

	assert.Empty(t, captured)
}
