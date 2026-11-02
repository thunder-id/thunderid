// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// errorAuthorizationDetailsTypeNotFound is returned when no type is registered under the requested value.
var errorAuthorizationDetailsTypeNotFound = apierror.ErrorResponse{
	Code: "RAR-1029",
	Message: tidcommon.I18nMessage{
		Key:          "error.discoveryservice.authorization_details_type_not_found",
		DefaultValue: "Authorization details type not found",
	},
	Description: tidcommon.I18nMessage{
		Key:          "error.discoveryservice.authorization_details_type_not_found_description",
		DefaultValue: "No authorization details type is registered under the requested value",
	},
}

// internalServerError is returned when the type metadata cannot be served.
var internalServerError = apierror.ErrorResponse{
	Code:        tidcommon.InternalServerError.Code,
	Message:     tidcommon.InternalServerError.Error,
	Description: tidcommon.InternalServerError.ErrorDescription,
}

// errAuthorizationDetailsTypesUnavailable is returned when the registered types cannot be read.
var errAuthorizationDetailsTypesUnavailable = errors.New("authorization details types cannot be read")
