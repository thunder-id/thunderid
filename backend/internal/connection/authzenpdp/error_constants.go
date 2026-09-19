// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

var (
	// ErrorNotFound is returned when an AuthZEN PDP connection does not exist.
	ErrorNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.connection_not_found",
			DefaultValue: "Connection not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.connection_not_found_description",
			DefaultValue: "No connection exists for the supplied identifier",
		},
	}
	// ErrorHasBlockingDependencies is returned when an AuthZEN PDP connection cannot be deleted
	// because resource servers depend on it.
	ErrorHasBlockingDependencies = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.connection_has_blocking_dependencies",
			DefaultValue: "Connection cannot be deleted",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.connectionservice.connection_has_blocking_dependencies_description",
			DefaultValue: "The connection cannot be deleted because other resources depend on it. " +
				"Remove or reassign them first.",
		},
	}
	// ErrorInvalidEndpoint is returned when an AuthZEN PDP endpoint is invalid.
	ErrorInvalidEndpoint = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_authzen_pdp_endpoint",
			DefaultValue: "Invalid AuthZEN PDP endpoint",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.invalid_authzen_pdp_endpoint_description",
			DefaultValue: "The single and batch evaluation endpoints must be absolute URLs.",
		},
	}
	// ErrorAlreadyExists is returned when an AuthZEN PDP has the same name.
	ErrorAlreadyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CON-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.connectionservice.authzen_pdp_already_exists",
			DefaultValue: "An AuthZEN PDP connection with the same name already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.connectionservice.authzen_pdp_already_exists_description",
			DefaultValue: "Choose a different name for the AuthZEN PDP connection",
		},
	}
)
