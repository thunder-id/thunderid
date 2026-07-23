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

package ui

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/tools/cli/internal/services/setup"
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
	w.Close() //nolint:errcheck
	os.Stdout = orig
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}
	return string(out)
}

func TestPrintCredentialsFallback_PrintsValues(t *testing.T) {
	creds := &setup.AdminCredentials{Username: "admin", Password: "s3cr3t#Pass"}
	out := captureStdout(t, func() { PrintCredentialsFallback(creds) })
	if !strings.Contains(out, "admin") || !strings.Contains(out, "s3cr3t#Pass") {
		t.Fatalf("fallback output missing credentials:\n%s", out)
	}
}

func TestPrintCredentialsFallback_NilIsNoOp(t *testing.T) {
	out := captureStdout(t, func() { PrintCredentialsFallback(nil) })
	if out != "" {
		t.Fatalf("expected no output for nil creds, got:\n%s", out)
	}
}

func TestCredentialsBox_RendersValues(t *testing.T) {
	creds := &setup.AdminCredentials{Username: "admin", Password: "s3cr3t#Pass"}
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, creds)
	box := m.credentialsBox()
	if !strings.Contains(box, "admin") || !strings.Contains(box, "s3cr3t#Pass") {
		t.Fatalf("credentials box missing values:\n%s", box)
	}
}

func TestCredentialsBox_NilReturnsEmpty(t *testing.T) {
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	if box := m.credentialsBox(); box != "" {
		t.Fatalf("expected empty box for nil creds, got:\n%s", box)
	}
}

func TestRender_IncludesCredentials(t *testing.T) {
	creds := &setup.AdminCredentials{Username: "admin", Password: "s3cr3t#Pass"}
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, creds)
	out := m.render()
	if !strings.Contains(out, "admin") || !strings.Contains(out, "s3cr3t#Pass") {
		t.Fatalf("render output missing credentials:\n%s", out)
	}
}
