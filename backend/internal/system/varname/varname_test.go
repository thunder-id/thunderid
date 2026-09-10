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

package varname

import (
	"regexp"
	"testing"
)

// TestDeriveVariableName locks the placeholder-naming contract that the export parameterizer and the
// Control Plane secret auto-capture both depend on. If this changes, captured secret keys and export
// placeholders would diverge and the apply flow would fail to resolve them.
func TestDeriveVariableName(t *testing.T) {
	cases := []struct {
		resourceType string
		resourceName string
		fieldName    string
		want         string
	}{
		{"application", "My App", "ClientSecret", "APPLICATION_MY_APP_CLIENT_SECRET"},
		{"connection", "Export Test IDP", "client_id", "CONNECTION_EXPORT_TEST_IDP_CLIENT_ID"},
		{"user", "alice", "password", "USER_ALICE_PASSWORD"},
		{"agent", "ClientID", "value", "AGENT_CLIENT_ID_VALUE"},
		// An unqualified call keeps the old shape, so a caller with no type still produces a name.
		{"", "My App", "ClientSecret", "MY_APP_CLIENT_SECRET"},
	}
	for _, c := range cases {
		if got := DeriveVariableName(c.resourceType, c.resourceName, c.fieldName); got != c.want {
			t.Errorf("DeriveVariableName(%q, %q, %q) = %q, want %q",
				c.resourceType, c.resourceName, c.fieldName, got, c.want)
		}
	}
}

// The whole point of the type prefix: two resources of different types may share a name.
func TestDeriveVariableNameSeparatesTypesThatShareAName(t *testing.T) {
	app := DeriveVariableName("application", "dummy", "ClientID")
	agent := DeriveVariableName("agent", "dummy", "ClientID")
	if app == agent {
		t.Fatalf("an application and an agent named the same must not share a variable: %q", app)
	}
}

func TestDeriveVariableNameProducesValidTemplateIdentifiers(t *testing.T) {
	// A username is commonly an email address, whose characters cannot appear in a template variable.
	if got := DeriveVariableName("user", "user@example.com", "password"); got != "USER_USER_EXAMPLE_COM_PASSWORD" {
		t.Fatalf("email based name not sanitized: %q", got)
	}
	if got := DeriveVariableName("application", "My App", "ClientSecret"); got != "APPLICATION_MY_APP_CLIENT_SECRET" {
		t.Fatalf("regression on the ordinary case: %q", got)
	}
	// Runs of separators collapse rather than producing empty segments.
	if got := DeriveVariableName("", "a..b--c", "password"); got != "A_B_C_PASSWORD" {
		t.Fatalf("separators not collapsed: %q", got)
	}
	// A leading digit would be an invalid identifier.
	if got := DeriveVariableName("", "1app", "password"); got != "_1APP_PASSWORD" {
		t.Fatalf("leading digit not handled: %q", got)
	}

	// Every derived name must be a valid Go template identifier.
	for _, name := range []string{
		DeriveVariableName("user", "user@example.com", "password"),
		DeriveVariableName("", "a..b--c", "password"),
		DeriveVariableName("", "1app", "password"),
	} {
		if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name) {
			t.Fatalf("%q is not a valid template identifier", name)
		}
	}
}
