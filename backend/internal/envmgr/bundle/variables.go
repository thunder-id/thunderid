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
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	scalarVarRe = regexp.MustCompile(`\{\{\s*\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)
	arrayVarRe  = regexp.MustCompile(`range\s+\.([A-Za-z_][A-Za-z0-9_]*)`)
	// credentialVarRe matches a placeholder that is the whole value of a YAML field.
	credentialVarRe = regexp.MustCompile(
		`(?m)^\s*([A-Za-z0-9_]+)\s*:\s*"?\{\{\s*\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}"?\s*$`)
)

// credentialFields are the fields whose value is a credential rather than configuration. A placeholder
// under one of these is filled from the data plane's secret service, never from an environment
// variable, so it must not be reported as an unset variable for an operator to go and set.
var credentialFields = map[string]bool{
	"password":     true,
	"clientsecret": true,
	"secret":       true,
	"apisecret":    true,
	"apikey":       true,
	"privatekey":   true,
	"flowsecret":   true,
	"token":        true,
	"authtoken":    true,
}

// SecretVariables lists the placeholders the resources fill from a secret service.
//
// This is read from the bundle itself rather than from a list the control plane keeps, because the
// control plane forwards a credential to the data plane instead of storing it: asking it which secrets
// exist would miss every credential it has already handed off, and those would then be reported as
// ordinary variables with no value.
func SecretVariables(resources string) []string {
	secrets := map[string]bool{}
	for _, placeholder := range SecretPlaceholders(resources) {
		secrets[placeholder.Name] = true
	}
	return sortedKeys(secrets)
}

// SecretPlaceholder is one credential placeholder together with the field it fills. The field is what
// tells a credential that is only ever verified, such as a password, apart from one that has to be
// replayed to a third party, such as an SMS gateway key.
type SecretPlaceholder struct {
	Name  string
	Field string
}

// SecretPlaceholders lists the credential placeholders of the resources with the field each fills.
func SecretPlaceholders(resources string) []SecretPlaceholder {
	seen := map[string]bool{}
	matches := credentialVarRe.FindAllStringSubmatch(resources, -1)
	out := make([]SecretPlaceholder, 0, len(matches))
	for _, m := range matches {
		if !credentialFields[strings.ToLower(m[1])] || seen[m[2]] {
			continue
		}
		seen[m[2]] = true
		out = append(out, SecretPlaceholder{Name: m[2], Field: m[1]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// StripCredentialLines removes the field that each named placeholder fills, so a document can be
// written somewhere that holds no credentials at all.
//
// The whole line goes, not just the value: an empty credential is not "leave it alone" everywhere. A
// user's password and a connection's client secret are both rejected as invalid when blank, while an
// absent field simply leaves the resource without one.
func StripCredentialLines(resources string, names []string) string {
	if len(names) == 0 {
		return resources
	}
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}

	lines := strings.Split(resources, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if m := credentialVarRe.FindStringSubmatch(line); len(m) > 2 && wanted[m[2]] {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// ParseEnv parses a .env body (KEY=VALUE lines) into a flat map. Blank lines and comments are skipped.
func ParseEnv(env string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(env, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		out[key] = unquote(val)
	}
	return out
}

// BuildTemplateVariables produces the variables map for the import API by scanning resources for
// placeholder references and resolving them from the flat values map. Scalar placeholders
// ({{.NAME}}) map to strings; array placeholders ({{- range .NAME}}) are rebuilt into slices from the
// indexed NAME_0, NAME_1, ... values that the export .env uses.
//
// A placeholder this service cannot resolve is left out of the map entirely rather than sent as an
// empty value, and so is anything known to be secret backed. Its placeholder stays in the content and
// the data plane fills it while importing, from its own secret provider or environment.
//
// That ordering is the only one that works for a credential stored as a one-way hash, such as an
// application's client secret: the real value has to be present before the import hashes it, and it can
// never be recovered afterwards. Sending an empty value instead is what silently produces an
// application with no client secret or no redirect URIs, so an unresolvable placeholder is delegated to
// the data plane, which either fills it or fails the import loudly.
func BuildTemplateVariables(resources string, values map[string]string,
	secretKeys []string) map[string]interface{} {
	arrays := map[string]bool{}
	for _, m := range arrayVarRe.FindAllStringSubmatch(resources, -1) {
		arrays[m[1]] = true
	}

	secrets := make(map[string]bool, len(secretKeys))
	for _, key := range secretKeys {
		secrets[key] = true
	}

	out := map[string]interface{}{}
	for name := range arrays {
		if secrets[name] {
			continue
		}
		// A list that is configured as empty is a value, not a missing one: the resource genuinely has
		// no entries. Dropping it would leave the placeholder unresolved and fail the whole import.
		if resolved, ok := arrayValues(name, values); ok {
			out[name] = resolved
		}
	}
	for _, m := range scalarVarRe.FindAllStringSubmatch(resources, -1) {
		name := m[1]
		if arrays[name] || secrets[name] {
			continue
		}
		if value := values[name]; strings.TrimSpace(value) != "" {
			out[name] = value
		}
	}
	return out
}

// arrayValues resolves the elements of an array placeholder, and reports whether the placeholder is
// configured at all.
//
// The two are different: a list configured as empty ("[]") has a value, and an unconfigured one has
// none. Returning only the elements cannot tell them apart, and treating the first as unconfigured
// leaves its placeholder unresolved, which fails the import it appears in.
//
// Two encodings are accepted, because the two producers differ. The export API writes the whole list
// as a JSON array in a single variable (REDIRECT_URIS=["https://a","https://b"]), while the data
// plane's declarative loader reads indexed variables (REDIRECT_URIS_0, REDIRECT_URIS_1). Indexed
// values win when both are present. Getting this wrong yields an empty list rather than an error,
// which silently strips values such as an application's redirect URIs.
func arrayValues(name string, values map[string]string) ([]interface{}, bool) {
	if indexed := indexedValues(name, values); len(indexed) > 0 {
		return indexed, true
	}

	raw, ok := values[name]
	if !ok || strings.TrimSpace(raw) == "" {
		return []interface{}{}, false
	}

	if trimmed := strings.TrimSpace(raw); strings.HasPrefix(trimmed, "[") {
		var parsed []interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			return parsed, true
		}
	}
	// Not a JSON array: treat the value as a single element.
	return []interface{}{raw}, true
}

// indexedValues collects NAME_0, NAME_1, ... from values in contiguous order.
func indexedValues(name string, values map[string]string) []interface{} {
	var out []interface{}
	for i := 0; ; i++ {
		v, ok := values[fmt.Sprintf("%s_%d", name, i)]
		if !ok {
			break
		}
		out = append(out, v)
	}
	return out
}

// RequiredVariables lists every placeholder the resources reference, split into the scalar and array
// forms, in sorted order.
func RequiredVariables(resources string) (scalars []string, arrays []string) {
	arraySet := map[string]bool{}
	for _, m := range arrayVarRe.FindAllStringSubmatch(resources, -1) {
		arraySet[m[1]] = true
	}
	scalarSet := map[string]bool{}
	for _, m := range scalarVarRe.FindAllStringSubmatch(resources, -1) {
		if !arraySet[m[1]] {
			scalarSet[m[1]] = true
		}
	}
	return sortedKeys(scalarSet), sortedKeys(arraySet)
}

// MissingVariables lists the placeholders that would resolve to nothing on an apply: referenced by the
// resources, not backed by a secret, and with no value configured. Applying with these unresolved
// silently strips the field, for example leaving an application with no redirect URIs, so a caller is
// expected to surface them before applying rather than after.
func MissingVariables(resources string, values map[string]string, secretKeys []string) []string {
	secrets := make(map[string]bool, len(secretKeys))
	for _, key := range secretKeys {
		secrets[key] = true
	}

	scalars, arrays := RequiredVariables(resources)
	missing := map[string]bool{}
	for _, name := range scalars {
		if secrets[name] {
			continue
		}
		if strings.TrimSpace(values[name]) == "" {
			missing[name] = true
		}
	}
	for _, name := range arrays {
		if secrets[name] {
			continue
		}
		// Configured as an empty list is configured. Only a placeholder with nothing behind it is
		// missing.
		if _, ok := arrayValues(name, values); !ok {
			missing[name] = true
		}
	}
	return sortedKeys(missing)
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// DeploymentURLVariable is the placeholder a captured bundle carries in place of the deployment's own
// base URL, resolved to each target's URL when the bundle is applied.
const DeploymentURLVariable = "DEPLOYMENT_URL"

// defaultResourceServerPattern finds the id the defaultResourceServer setting points at.
var defaultResourceServerPattern = regexp.MustCompile(
	`(?s)resource_type:\s*server_config.*?name:\s*defaultResourceServer.*?resourceServerId"?\s*:\s*"([^"]+)"`)

// identifierLinePattern matches a resource server's identifier line and splits its origin from the
// rest of the URL.
var identifierLinePattern = regexp.MustCompile(
	`(?m)^(\s*identifier:\s*"?)([a-zA-Z][a-zA-Z0-9+.-]*://[^/"\s]+)([^"\s]*)("?)\s*$`)

// TemplateDeploymentURL replaces the origin of the default resource server's identifier with a
// placeholder, so each environment resolves it to its own.
//
// That identifier is an audience: it is what a token issued by the deployment is bound to. Promoted
// verbatim, every environment would name the same audience as the one the bundle was captured from,
// and a token minted for one would name the audience of all of them. Only the origin is replaced, so
// the path an operator chose is kept.
//
// Only the resource server the deployment's own default points at is touched. Every other one is
// configuration an operator authored and is promoted as it stands.
func TemplateDeploymentURL(resources string) string {
	m := defaultResourceServerPattern.FindStringSubmatch(resources)
	if m == nil {
		return resources
	}
	targetID := m[1]

	docs := strings.Split(resources, "\n---")
	for i, doc := range docs {
		if !strings.Contains(doc, "resource_type: resource_server") || !strings.Contains(doc, targetID) {
			continue
		}
		docs[i] = identifierLinePattern.ReplaceAllString(doc,
			"${1}{{."+DeploymentURLVariable+"}}${3}${4}")
	}
	return strings.Join(docs, "\n---")
}
