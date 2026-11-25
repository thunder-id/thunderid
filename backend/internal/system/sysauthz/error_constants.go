// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Authorization errors for system-level authorization checks.
var (
	// ErrorGrantNotPermitted is returned when an operation would confer permissions that the caller
	// does not itself hold. Distinct from tidcommon.ErrorUnauthorized so that clients can tell the
	// two refusals apart; both map to 403.
	ErrorGrantNotPermitted = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "SAZ-4030",
		Error: tidcommon.I18nMessage{
			Key:          "error.sysauthz.grant_not_permitted",
			DefaultValue: "Insufficient privileges to grant these permissions",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.sysauthz.grant_not_permitted_description",
			DefaultValue: "The operation would grant permissions that the caller does not hold",
		},
	}
)

// errMultipleListOUs is returned by AccessibleResources.listOU when a list would span several
// organization units.
var errMultipleListOUs = errors.New("the list spans more than one organization unit")
