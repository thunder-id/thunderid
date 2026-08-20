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

package bundle

import (
	"reflect"
	"testing"
)

const testRedirectURIsVar = "APP_A_REDIRECT_URIS"

const sample = `# File: AppA.yaml
resource_type: application
id: app-a
name: App A
inboundAuthConfig:
  oauth2Config:
    clientId: {{.APP_A_CLIENT_ID}}
    clientSecret: {{.APP_A_CLIENT_SECRET}}
    redirectUris:
      {{- range .APP_A_REDIRECT_URIS}}
      - {{.}}
      {{- end}}
---
# File: AppB.yaml
resource_type: application
id: app-b
name: App B`

func TestParseIdentifiesResources(t *testing.T) {
	resources := Parse(sample)
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}
	if resources[0].Type != "application" || resources[0].ID != "app-a" || resources[0].Name != "App A" {
		t.Fatalf("unexpected first resource: %+v", resources[0])
	}
	if resources[0].Key() != "application/id:app-a" {
		t.Fatalf("unexpected key: %s", resources[0].Key())
	}
	// The file-name comment must be stripped from normalized content.
	for _, r := range resources {
		if got := r.Content; contains(got, "# File:") {
			t.Fatalf("file comment not stripped: %q", got)
		}
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	resources := Parse(sample)
	out := Parse(Marshal(resources))
	if len(out) != len(resources) {
		t.Fatalf("round trip changed count: %d -> %d", len(resources), len(out))
	}
	for i := range resources {
		if out[i].Key() != resources[i].Key() || out[i].Content != resources[i].Content {
			t.Fatalf("round trip mismatch at %d: %+v vs %+v", i, resources[i], out[i])
		}
	}
}

func TestParseEnv(t *testing.T) {
	env := "# comment\nAPP_A_CLIENT_ID=abc\nAPP_A_CLIENT_SECRET=\"s3cr3t\"\n\n" +
		"APP_A_REDIRECT_URIS_0=https://a\nAPP_A_REDIRECT_URIS_1=https://b\n"
	got := ParseEnv(env)
	want := map[string]string{
		"APP_A_CLIENT_ID":       "abc",
		"APP_A_CLIENT_SECRET":   "s3cr3t",
		"APP_A_REDIRECT_URIS_0": "https://a",
		"APP_A_REDIRECT_URIS_1": "https://b",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseEnv = %#v, want %#v", got, want)
	}
}

func TestBuildTemplateVariables(t *testing.T) {
	values := map[string]string{
		"APP_A_CLIENT_ID":       "abc",
		"APP_A_CLIENT_SECRET":   "s3cr3t",
		"APP_A_REDIRECT_URIS_0": "https://a",
		"APP_A_REDIRECT_URIS_1": "https://b",
	}
	vars := BuildTemplateVariables(sample, values, nil)

	if vars["APP_A_CLIENT_ID"] != "abc" || vars["APP_A_CLIENT_SECRET"] != "s3cr3t" {
		t.Fatalf("scalar resolution wrong: %#v", vars)
	}
	arr, ok := vars[testRedirectURIsVar].([]interface{})
	if !ok {
		t.Fatalf("array not rebuilt as slice: %#v", vars[testRedirectURIsVar])
	}
	if len(arr) != 2 || arr[0] != "https://a" || arr[1] != "https://b" {
		t.Fatalf("array values wrong: %#v", arr)
	}
	// The indexed keys must not also leak in as scalars.
	if _, exists := vars["APP_A_REDIRECT_URIS_0"]; exists {
		t.Fatalf("indexed key leaked as scalar")
	}
}

func TestBuildTemplateVariablesAcceptsJSONArrayEncoding(t *testing.T) {
	// The export API writes an array as a JSON literal in one variable.
	vars := BuildTemplateVariables(sample, map[string]string{
		testRedirectURIsVar: `["https://a","https://b"]`,
	}, nil)
	arr, ok := vars[testRedirectURIsVar].([]interface{})
	if !ok || len(arr) != 2 || arr[0] != "https://a" || arr[1] != "https://b" {
		t.Fatalf("JSON array not parsed: %#v", vars[testRedirectURIsVar])
	}
}

func TestBuildTemplateVariablesPrefersIndexedOverJSON(t *testing.T) {
	vars := BuildTemplateVariables(sample, map[string]string{
		testRedirectURIsVar:     `["ignored"]`,
		"APP_A_REDIRECT_URIS_0": "https://indexed",
	}, nil)
	arr := vars[testRedirectURIsVar].([]interface{})
	if len(arr) != 1 || arr[0] != "https://indexed" {
		t.Fatalf("indexed values should win: %#v", arr)
	}
}

func TestBuildTemplateVariablesOmitsUnresolvablePlaceholders(t *testing.T) {
	// An unresolvable placeholder is delegated to the data plane rather than sent as an empty value.
	// Sending empty is what silently produced an application with no redirect URIs.
	for _, raw := range []string{"", "   "} {
		vars := BuildTemplateVariables(sample, map[string]string{testRedirectURIsVar: raw}, nil)
		if _, present := vars[testRedirectURIsVar]; present {
			t.Fatalf("expected the placeholder to be omitted for %q, got %#v", raw, vars[testRedirectURIsVar])
		}
	}

	// A blank scalar is delegated for the same reason.
	vars := BuildTemplateVariables(sample, map[string]string{"APP_A_CLIENT_ID": "  "}, nil)
	if _, present := vars["APP_A_CLIENT_ID"]; present {
		t.Fatalf("expected a blank scalar to be omitted, got %#v", vars["APP_A_CLIENT_ID"])
	}
}

func TestBuildTemplateVariablesSingleScalarBecomesOneElement(t *testing.T) {
	vars := BuildTemplateVariables(sample, map[string]string{testRedirectURIsVar: "https://only"}, nil)
	arr := vars[testRedirectURIsVar].([]interface{})
	if len(arr) != 1 || arr[0] != "https://only" {
		t.Fatalf("expected a one element list, got %#v", arr)
	}
}

func TestBuildTemplateVariablesOmitsSecretsEntirely(t *testing.T) {
	values := map[string]string{"APP_A_CLIENT_SECRET": "should-not-be-sent", "APP_A_CLIENT_ID": "abc"}
	vars := BuildTemplateVariables(sample, values, []string{"APP_A_CLIENT_SECRET", testRedirectURIsVar})

	// A secret is left out so its placeholder survives into the import, where the data plane fills it
	// from its own provider before the value is hashed. Sending anything here, value or reference, would
	// either leak the secret or be hashed into something no login can match.
	if _, present := vars["APP_A_CLIENT_SECRET"]; present {
		t.Fatalf("a secret must not be sent, got %#v", vars["APP_A_CLIENT_SECRET"])
	}
	if _, present := vars[testRedirectURIsVar]; present {
		t.Fatalf("a secret backed array must not be sent, got %#v", vars[testRedirectURIsVar])
	}
	// Everything else is still supplied.
	if vars["APP_A_CLIENT_ID"] != "abc" {
		t.Fatalf("non-secret values must still be sent, got %#v", vars["APP_A_CLIENT_ID"])
	}
}

func TestMissingVariablesFindsUnresolvedPlaceholders(t *testing.T) {
	// Only the client id is configured, so the secret and the array are the interesting cases.
	missing := MissingVariables(sample, map[string]string{"APP_A_CLIENT_ID": "abc"},
		[]string{"APP_A_CLIENT_SECRET"})

	// The secret is supplied as a reference, so it is not missing; the redirect URIs are.
	if len(missing) != 1 || missing[0] != testRedirectURIsVar {
		t.Fatalf("expected only the redirect URIs to be missing, got %v", missing)
	}
}

func TestMissingVariablesEmptyWhenEverythingResolves(t *testing.T) {
	missing := MissingVariables(sample, map[string]string{
		"APP_A_CLIENT_ID":   "abc",
		testRedirectURIsVar: `["https://a"]`,
	}, []string{"APP_A_CLIENT_SECRET"})

	if len(missing) != 0 {
		t.Fatalf("expected nothing missing, got %v", missing)
	}
}

// A list configured as empty is configured: the resource genuinely has no entries, and reporting it
// as missing both blocks an apply that is fine and drops the value, leaving the placeholder
// unresolved and failing the import.
func TestMissingVariablesTreatsAnEmptyListAsConfigured(t *testing.T) {
	missing := MissingVariables(sample, map[string]string{
		"APP_A_CLIENT_ID":   "app-a",
		testRedirectURIsVar: "[]",
	}, []string{"APP_A_CLIENT_SECRET"})

	if len(missing) != 0 {
		t.Fatalf("an empty list is a value, got %v", missing)
	}

	// It still resolves to an empty list rather than being dropped.
	vars := BuildTemplateVariables(sample, map[string]string{testRedirectURIsVar: "[]"}, nil)
	resolved, ok := vars[testRedirectURIsVar].([]interface{})
	if !ok || len(resolved) != 0 {
		t.Fatalf("expected an empty list to be sent, got %#v", vars[testRedirectURIsVar])
	}
}

func TestMissingVariablesTreatsBlankAsMissing(t *testing.T) {
	missing := MissingVariables(sample, map[string]string{
		"APP_A_CLIENT_ID": "   ",
	}, nil)

	// A blank scalar, an unset array and the unset secret all count as unresolved.
	want := map[string]bool{"APP_A_CLIENT_ID": true, "APP_A_CLIENT_SECRET": true, testRedirectURIsVar: true}
	if len(missing) != len(want) {
		t.Fatalf("expected %d missing, got %v", len(want), missing)
	}
	for _, m := range missing {
		if !want[m] {
			t.Fatalf("unexpected missing entry %q", m)
		}
	}
}

func TestRequiredVariablesSplitsScalarsAndArrays(t *testing.T) {
	scalars, arrays := RequiredVariables(sample)
	if len(arrays) != 1 || arrays[0] != testRedirectURIsVar {
		t.Fatalf("unexpected arrays: %v", arrays)
	}
	for _, name := range scalars {
		if name == testRedirectURIsVar {
			t.Fatalf("an array placeholder must not be reported as a scalar")
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
