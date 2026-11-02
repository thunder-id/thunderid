// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package usermgtprovider

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for user management provider operations. Codes follow the UMP-* convention. Failures
// raised by the user service keep their own USR-* codes and are not remapped here.
var (
	// ErrorInvalidRequestFormat is returned when no user is supplied.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "UMP-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.usermgtprovider.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.usermgtprovider.invalid_request_format_description",
			DefaultValue: "The user to provision must be provided",
		},
	}

	// ErrorUserProvisioningDisabled is returned when the user provider is disabled by configuration.
	ErrorUserProvisioningDisabled = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "UMP-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.usermgtprovider.user_provisioning_disabled",
			DefaultValue: "User provisioning is disabled",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.usermgtprovider.user_provisioning_disabled_description",
			DefaultValue: "The deployment is not configured to provision users from the runtime",
		},
	}
)
