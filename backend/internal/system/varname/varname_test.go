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
	"strings"
	"testing"
)

const (
	idAlice = "0198f3a1-7c2e-7b10-9a44-5d3e8f21c6b7"
	idOther = "0198f3a1-7c2e-7b10-9a44-5d3e8f21c6b8"
)

var templateIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// TestDeriveVariableName locks the placeholder-naming contract that the export parameterizer and the
// Control Plane secret auto-capture both depend on. If this changes, captured secret keys and export
// placeholders would diverge and the apply flow would fail to resolve them.
func TestDeriveVariableName(t *testing.T) {
	cases := []struct {
		resourceType string
		resourceID   string
		fieldName    string
		want         string
	}{
		{"application", idAlice, "ClientSecret", "APPLICATION__0198F3A17C2E7B109A445D3E8F21C6B7__CLIENT_SECRET"},
		{"connection", idAlice, "client_id", "CONNECTION__0198F3A17C2E7B109A445D3E8F21C6B7__CLIENT_ID"},
		{"user", idAlice, "password", "USER__0198F3A17C2E7B109A445D3E8F21C6B7__PASSWORD"},
		{"resource_server", idAlice, "identifier", "RESOURCE_SERVER__0198F3A17C2E7B109A445D3E8F21C6B7__IDENTIFIER"},
		// An unqualified call still produces a name, and one that is not an invalid identifier.
		{"", idAlice, "password", "_0198F3A17C2E7B109A445D3E8F21C6B7__PASSWORD"},
	}
	for _, c := range cases {
		if got := DeriveVariableName(c.resourceType, c.resourceID, c.fieldName); got != c.want {
			t.Errorf("DeriveVariableName(%q, %q, %q) = %q, want %q",
				c.resourceType, c.resourceID, c.fieldName, got, c.want)
		}
	}
}

// The reason an ID is the name: resources whose names or usernames differ only in case, dots, at signs,
// hyphens or underscores read the same once sanitized, and must not share a name. Their IDs differ.
func TestDeriveVariableNameSeparatesResourcesOfOneType(t *testing.T) {
	victim := DeriveVariableName("user", idAlice, "password")
	other := DeriveVariableName("user", idOther, "password")
	if victim == other {
		t.Fatalf("two users must not share a name: %q", victim)
	}
}

func TestDeriveVariableNameIsTheSameForTheSameResource(t *testing.T) {
	first := DeriveVariableName("user", idAlice, "password")
	second := DeriveVariableName("user", idAlice, "password")
	if first != second {
		t.Fatalf("a resource must derive one name, got %q and %q", first, second)
	}
}

// The type prefix still keeps two resources of different types that share an ID apart.
func TestDeriveVariableNameSeparatesTypesThatShareAnID(t *testing.T) {
	app := DeriveVariableName("application", idAlice, "ClientID")
	agent := DeriveVariableName("agent", idAlice, "ClientID")
	if app == agent {
		t.Fatalf("an application and an agent with one ID must not share a variable: %q", app)
	}
}

func TestDeriveVariableNameSeparatesFieldsOfOneResource(t *testing.T) {
	id := DeriveVariableName("application", idAlice, "ClientID")
	secret := DeriveVariableName("application", idAlice, "ClientSecret")
	if id == secret {
		t.Fatalf("two fields of one resource must not share a variable: %q", id)
	}
}

// An ID that is not a lower-case UUID is hashed: a declarative file may name resources "my-app" and
// "my_app", which sanitizing alone would make the same.
func TestDeriveVariableNameSeparatesIDsThatSanitizeAlike(t *testing.T) {
	first := DeriveVariableName("application", "my-app", "ClientID")
	second := DeriveVariableName("application", "my_app", "ClientID")
	third := DeriveVariableName("application", "MY-APP", "ClientID")
	if first == second || first == third || second == third {
		t.Fatalf("IDs that sanitize alike must not share a name: %q %q %q", first, second, third)
	}
}

// A hashed ID segment can never be the spelling of a UUID, since it starts with a letter no hex digit has.
func TestDeriveVariableNameKeepsHashedAndUUIDSegmentsApart(t *testing.T) {
	if got := idSegment("not-a-uuid"); !strings.HasPrefix(got, hashedIDPrefix) {
		t.Fatalf("a free-form ID must be hashed, got %q", got)
	}
	if got := idSegment(idAlice); strings.HasPrefix(got, hashedIDPrefix) {
		t.Fatalf("a lower-case UUID must not be hashed, got %q", got)
	}
	if upper := strings.ToUpper(idAlice); idSegment(upper) == idSegment(idAlice) {
		t.Fatalf("an upper-case UUID is a different ID and must not share a spelling with the lower-case one")
	}
}

func TestDeriveVariableNameProducesValidTemplateIdentifiers(t *testing.T) {
	for _, name := range []string{
		DeriveVariableName("user", idAlice, "password"),
		DeriveVariableName("", "free-form", "password"),
		DeriveVariableName("user", "", "password"),
	} {
		if !templateIdentifier.MatchString(name) {
			t.Fatalf("%q is not a valid template identifier", name)
		}
	}
}

// Every part is sanitized on its own and joined with a separator no part can hold, so a name always
// reads back as exactly one resource type, ID and field.
func TestDeriveVariableNameSeparatorNeverAppearsInsideAPart(t *testing.T) {
	name := DeriveVariableName("user", "a__b", "pass__word")
	parts := strings.Split(name, segmentSeparator)
	if len(parts) != 3 {
		t.Fatalf("want 3 parts, got %d in %q", len(parts), name)
	}
	for _, part := range parts {
		if part == "" || strings.Contains(part, segmentSeparator) {
			t.Fatalf("a part must be non-empty and free of the separator: %q in %q", part, name)
		}
	}
}
