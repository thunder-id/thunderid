// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// MaxReasonLength caps a processor's refusal reason shown to the user or the client.
const MaxReasonLength = 200

// IsSafeText reports whether text carries no control characters other than whitespace and no bidirectional
// formatting characters, so it can be shown to a user as it is.
func IsSafeText(text string) bool {
	return checkText(text) == nil
}

// checkText refuses control characters other than whitespace, and bidirectional formatting characters.
func checkText(text string) error {
	for _, r := range text {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
		case r < 0x20 || (r >= 0x7f && r <= 0x9f):
			return errors.New("value carries a control character")
		case r == 0x061c || r == 0x200e || r == 0x200f || (r >= 0x202a && r <= 0x202e) ||
			(r >= 0x2066 && r <= 0x2069):
			return errors.New("value carries a bidirectional formatting character")
		}
	}
	return nil
}

// DisplayableReason returns a processor's refusal reason as it may be shown, cut to MaxReasonLength, or empty
// when it is not safe to show.
func DisplayableReason(reason string) string {
	if !IsSafeText(reason) {
		return ""
	}
	if runes := []rune(reason); len(runes) > MaxReasonLength {
		return string(runes[:MaxReasonLength])
	}
	return reason
}

// RefusalError returns base for a refusal, described by withReason when the processor's reason is safe to show.
func RefusalError(base tidcommon.ServiceError, withReason tidcommon.I18nMessage,
	reason string) *tidcommon.ServiceError {
	reason = DisplayableReason(reason)
	if reason == "" {
		return &base
	}
	withReason.Params = map[string]string{"reason": reason}
	return tidcommon.CustomServiceError(base, withReason)
}
