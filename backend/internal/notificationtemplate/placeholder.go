// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"regexp"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// placeholderRegex finds every {{...}} block in a template string. Each block's inner text is checked
// against validPlaceholderRegex so a malformed or unsupported placeholder is rejected at create/update
// rather than surfacing as literal text at render time.
var placeholderRegex = regexp.MustCompile(`(?s)\{\{(.*?)\}\}`)

// validPlaceholderRegex matches a supported placeholder body: a ctx/t/design function over a
// dot/underscore/alphanumeric key, with no surrounding spaces (the render engine matches the exact
// form). Examples: ctx(otp), t(notification.otp.email.subject), design(palette.primary.main).
var validPlaceholderRegex = regexp.MustCompile(`^(ctx|t|design)\(([A-Za-z0-9_.]+)\)$`)

// placeholderFuncDesign is the design-token placeholder function name.
const placeholderFuncDesign = "design"

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
