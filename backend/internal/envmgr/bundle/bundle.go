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

// Package bundle parses and reassembles the multi-document declarative resource payload produced by
// the ThunderID export API. Parsing is textual on purpose: exported resources carry Go-template
// placeholders (for example clientSecret: {{.MY_APP_CLIENT_SECRET}}) that are not valid YAML until
// the templates are resolved, so a real YAML parse would fail. Each document is instead split on the
// YAML document separator and its identity fields are read from the top-level lines.
package bundle

import (
	"regexp"
	"strings"
)

// identityKeys are the top-level fields, in priority order, used to identify a resource for matching
// across versions. This mirrors the keys the import API matches on (id, then name, handle,
// identifier, language, section).
var identityKeys = []string{"id", "name", "handle", "identifier", "language", "section"}

var (
	resourceTypeRe = regexp.MustCompile(`(?m)^resource_type:\s*(.+?)\s*$`)
	fileCommentRe  = regexp.MustCompile(`(?m)^#\s*File:.*$`)
)

// Resource is a single declarative document within a bundle.
type Resource struct {
	// Type is the resource_type discriminator (application, connection, flow, ...).
	Type string
	// ID is the top-level id, when present. Used to drive id-based delete on apply.
	ID string
	// Name is the top-level name, when present.
	Name string
	// IdentityField/IdentityValue record which top-level field identified the resource.
	IdentityField string
	IdentityValue string
	// Category scopes user_type resources, which are addressed by category plus id.
	Category string
	// Content is the normalized document text (file-name comment stripped, trailing space trimmed).
	Content string
}

// Key returns a stable identity for matching the same resource across versions.
func (r Resource) Key() string {
	if r.IdentityValue != "" {
		return r.Type + "/" + r.IdentityField + ":" + r.IdentityValue
	}
	return r.Type + "/"
}

// Parse splits a combined resources payload into its documents.
func Parse(resources string) []Resource {
	docs := splitDocuments(resources)
	out := make([]Resource, 0, len(docs))
	for _, doc := range docs {
		content := normalizeDocument(doc)
		if content == "" {
			continue
		}
		r := Resource{Content: content}
		if m := resourceTypeRe.FindStringSubmatch(content); m != nil {
			r.Type = unquote(m[1])
		}
		if r.Type == "" {
			// Not a resource document (stray separator / comment block); skip it.
			continue
		}
		r.IdentityField, r.IdentityValue = topLevelIdentity(content)
		switch r.IdentityField {
		case "id":
			r.ID = r.IdentityValue
		case "name":
			r.Name = r.IdentityValue
		}
		if r.Name == "" {
			r.Name = topLevelValue(content, "name")
		}
		if r.ID == "" {
			r.ID = topLevelValue(content, "id")
		}
		r.Category = topLevelValue(content, "category")
		out = append(out, r)
	}
	return out
}

// Marshal reassembles resources into a combined payload suitable for the import API's content field.
func Marshal(resources []Resource) string {
	parts := make([]string, 0, len(resources))
	for _, r := range resources {
		parts = append(parts, strings.TrimSpace(r.Content))
	}
	return strings.Join(parts, "\n---\n") + "\n"
}

// Index maps resources by their identity key. When two documents collide on a key the later one wins,
// matching the last-write-wins nature of an idempotent import.
func Index(resources []Resource) map[string]Resource {
	m := make(map[string]Resource, len(resources))
	for _, r := range resources {
		m[r.Key()] = r
	}
	return m
}

// splitDocuments breaks a payload on lines that are exactly a YAML document separator ("---").
func splitDocuments(payload string) []string {
	lines := strings.Split(payload, "\n")
	var docs []string
	cur := make([]string, 0, len(lines))
	flush := func() {
		if len(cur) > 0 {
			docs = append(docs, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			flush()
			continue
		}
		cur = append(cur, line)
	}
	flush()
	return docs
}

// normalizeDocument strips the export's "# File:" hint comment and trims surrounding blank lines and
// trailing whitespace so equality comparisons are stable.
func normalizeDocument(doc string) string {
	doc = fileCommentRe.ReplaceAllString(doc, "")
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// topLevelIdentity returns the first present identity field (by priority) and its value.
func topLevelIdentity(content string) (string, string) {
	for _, key := range identityKeys {
		if v := topLevelValue(content, key); v != "" {
			return key, v
		}
	}
	return "", ""
}

// topLevelValue reads a top-level (column-zero) scalar value for key, or "" if absent.
func topLevelValue(content, key string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:\s*(.+?)\s*$`)
	if m := re.FindStringSubmatch(content); m != nil {
		return unquote(m[1])
	}
	return ""
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
