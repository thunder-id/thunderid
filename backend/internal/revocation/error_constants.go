// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ErrorEntryLimitExceeded is returned when one change would record more revocation entries than allowed.
var ErrorEntryLimitExceeded = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "RVS-1001",
	Error: tidcommon.I18nMessage{
		Key:          "error.revocationservice.entry_limit_exceeded",
		DefaultValue: "Too many revocation entries",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key: "error.revocationservice.entry_limit_exceeded_description",
		DefaultValue: "The change would record {{param(entries)}} revocation entries, more than the " +
			"{{param(max)}} allowed for one change",
	},
}

// errMalformedAudience is returned when a token's audience claim is present but malformed.
var errMalformedAudience = errors.New("audience claim is malformed")
