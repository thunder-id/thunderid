// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mgt

import (
	"regexp"

	goi18n "golang.org/x/text/language"
)

// SystemLanguage is the default language code for the system.
const SystemLanguage = "en-US"

// SystemNamespace is the default namespace for system translations.
const SystemNamespace = "system"

// NotificationNamespace is the namespace for notification translations.
// Unlike the system namespace, its default values are seeded from the
// bootstrap resources (backend/cmd/server/bootstrap) rather than compiled-in defaults.
const NotificationNamespace = "notification"

// LanguagePreferenceOrder defines the priority of languages for fallback.
var LanguagePreferenceOrder = map[string]int{
	"en-US": 0,
	"en":    1,
}

// namespaceRegex defines the valid format for namespace strings.
// Namespaces can contain alphanumeric characters, underscores, and hyphens.
var namespaceRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// keyRegex defines the valid format for translation keys.
// Keys can contain alphanumeric characters, dots, underscores, and hyphens.
var keyRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

const maxBCP47TagLength = 35

// NormaliseBCP47Tag returns the canonical BCP 47 form of tag (e.g. "en-US" for "en-us").
// Returns ("", false) if the tag is empty, exceeds the length limit, or is not a valid BCP 47 tag.
func NormaliseBCP47Tag(tag string) (string, bool) {
	if tag == "" || len(tag) > maxBCP47TagLength {
		return "", false
	}
	t, err := goi18n.Parse(tag)
	if err != nil {
		return "", false
	}
	return t.String(), true
}

// ValidateLanguage validates that a language tag is in the canonical form according to BCP 47 format.
func ValidateLanguage(language string) bool {
	tag, err := goi18n.BCP47.Parse(language)
	if err != nil {
		return false
	}
	return tag.String() == language
}

// ValidateNamespace validates that a namespace string matches the required format.
// Returns true if the namespace is non-empty and contains only alphanumeric characters, underscores, and hyphens.
func ValidateNamespace(namespace string) bool {
	if namespace == "" {
		return false
	}
	if len(namespace) > 64 {
		return false
	}
	return namespaceRegex.MatchString(namespace)
}

// ValidateKey validates that a key string matches the required format.
// Returns true if the key is non-empty and contains only alphanumeric characters, dots, underscores, and hyphens.
func ValidateKey(key string) bool {
	if key == "" {
		return false
	}
	if len(key) > 256 {
		return false
	}
	return keyRegex.MatchString(key)
}
