// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"regexp"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// placeholderKeyPattern is the key grammar shared by the create/update validator and the render-time
// matchers: a dot, underscore, hyphen, or alphanumeric key. The hyphen matches the i18n and
// design-token key grammars (e.g. palette.primary.on-hover).
const placeholderKeyPattern = `[A-Za-z0-9_.-]+`

// placeholderFuncDesign is the design-token placeholder function name.
const placeholderFuncDesign = "design"

// placeholderRegex finds every {{...}} block in a template string. Each block's inner text is checked
// against validPlaceholderRegex so a malformed or unsupported placeholder is rejected at create/update
// rather than surfacing as literal text at render time.
var placeholderRegex = regexp.MustCompile(`(?s)\{\{(.*?)\}\}`)

// validPlaceholderRegex matches a supported placeholder body: a ctx/t/design function over a key, with
// no surrounding spaces. Examples: ctx(otp), t(otp.email.subject), design(palette.primary.main).
var validPlaceholderRegex = regexp.MustCompile(`^(ctx|t|design)\((` + placeholderKeyPattern + `)\)$`)

// Render-time matchers for the two functions the renderer substitutes itself (design is delegated to the
// design service). They are built from placeholderKeyPattern, the same grammar validPlaceholderRegex
// enforces on write.
var (
	// translationPlaceholderRegex matches a {{t(key)}} translation reference in a stored field.
	translationPlaceholderRegex = regexp.MustCompile(`\{\{t\((` + placeholderKeyPattern + `)\)\}\}`)

	// contextPlaceholderRegex matches a {{ctx(key)}} context reference in a stored field.
	contextPlaceholderRegex = regexp.MustCompile(`\{\{ctx\((` + placeholderKeyPattern + `)\)\}\}`)
)

// designPlaceholderPrefix is the opening of a {{design(...)}} token, used to detect a token left
// unresolved after substitution so the render can fail closed.
const designPlaceholderPrefix = "{{" + placeholderFuncDesign + "("

// unresolvedBeforeContext returns the first unresolved t/design or malformed placeholder.
// Valid ctx placeholders are allowed for the following context pass.
// It runs first so placeholder-like context values are not misinterpreted.
func unresolvedBeforeContext(text string) string {
	for _, match := range placeholderRegex.FindAllStringSubmatch(text, -1) {
		if contextPlaceholderRegex.MatchString(match[0]) {
			continue
		}
		return match[0]
	}
	return ""
}

// validatePlaceholders checks the placeholders embedded in a template's content. ctx (flow-context) and
// t (translation) placeholders are allowed everywhere; design (design-token) placeholders are allowed
// only in the body of a channel that supports a design, never in the subject (which is plain text).
func validatePlaceholders(content TemplateContent, designAllowedInBody bool) *tidcommon.ServiceError {
	if svcErr := validatePlaceholderText(content.Subject, false); svcErr != nil {
		return svcErr
	}
	return validatePlaceholderText(content.Body, designAllowedInBody)
}

// validatePlaceholderText validates every {{...}} block in text, rejecting a malformed/unsupported
// placeholder and a design placeholder where a design does not apply.
func validatePlaceholderText(text string, designAllowed bool) *tidcommon.ServiceError {
	for _, match := range placeholderRegex.FindAllStringSubmatch(text, -1) {
		parts := validPlaceholderRegex.FindStringSubmatch(match[1])
		if parts == nil {
			return &ErrorInvalidPlaceholder
		}
		if parts[1] == placeholderFuncDesign && !designAllowed {
			return &ErrorDesignPlaceholderNotAllowed
		}
	}
	return nil
}
