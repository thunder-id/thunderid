// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmeta

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Error constants for flow metadata service

// ErrorInvalidType defines the error response for invalid type parameter.
var ErrorInvalidType = tidcommon.ServiceError{
	Code: "FM-1001",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.invalid_request",
		DefaultValue: "Invalid request",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.invalid_type_description",
		DefaultValue: "The 'type' parameter must be one of 'APP', 'REF' or 'OU'",
	},
}

// ErrorApplicationNotFound defines the error response for application not found.
var ErrorApplicationNotFound = tidcommon.ServiceError{
	Code: "FM-1002",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.resource_not_found",
		DefaultValue: "Resource not found",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.application_not_found_description",
		DefaultValue: "The specified application does not exist",
	},
}

// ErrorOUNotFound defines the error response for organization unit not found.
var ErrorOUNotFound = tidcommon.ServiceError{
	Code: "FM-1003",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.ou_not_found",
		DefaultValue: "Resource not found",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.ou_not_found_description",
		DefaultValue: "The specified organization unit does not exist",
	},
}

// ErrorMissingType defines the error response for missing type parameter.
var ErrorMissingType = tidcommon.ServiceError{
	Code: "FM-1004",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.missing_required_parameter",
		DefaultValue: "Missing required parameter",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.missing_type_description",
		DefaultValue: "The 'type' query parameter is required",
	},
}

// ErrorMissingID defines the error response for missing id parameter.
var ErrorMissingID = tidcommon.ServiceError{
	Code: "FM-1005",
	Type: tidcommon.ClientErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.missing_id_parameter",
		DefaultValue: "Missing required parameter",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.missing_id_description",
		DefaultValue: "The 'id' query parameter is required",
	},
}

// ErrorApplicationFetchFailed defines the error response for application fetch failure.
var ErrorApplicationFetchFailed = tidcommon.ServiceError{
	Code: "FM-5001",
	Type: tidcommon.ServerErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.internal_server_error",
		DefaultValue: "Internal server error",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.application_fetch_failed_description",
		DefaultValue: "Failed to retrieve application information",
	},
}

// ErrorOUFetchFailed defines the error response for organization unit fetch failure.
var ErrorOUFetchFailed = tidcommon.ServiceError{
	Code: "FM-5002",
	Type: tidcommon.ServerErrorType,
	Error: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.ou_fetch_failed",
		DefaultValue: "Internal server error",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.flowmetaservice.ou_fetch_failed_description",
		DefaultValue: "Failed to retrieve organization unit information",
	},
}
