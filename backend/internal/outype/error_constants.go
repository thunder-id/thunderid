// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outype

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for organization unit type management operations.
var (
	// ErrorInvalidRequestFormat is the error returned when the request body is malformed or has a field the
	// API does not define.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OUT-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.outypeservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.outypeservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains a field that is not supported",
		},
	}

	// ErrorOUTypeNotFound is the error returned when an organization unit type does not exist.
	ErrorOUTypeNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OUT-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.outypeservice.ou_type_not_found",
			DefaultValue: "Organization unit type not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.outypeservice.ou_type_not_found_description",
			DefaultValue: "The organization unit type with the specified id does not exist",
		},
	}

	// ErrorOUTypeHandleConflict is the error returned when another organization unit type has the handle.
	ErrorOUTypeHandleConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OUT-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.outypeservice.ou_type_handle_conflict",
			DefaultValue: "Organization unit type handle conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.outypeservice.ou_type_handle_conflict_description",
			DefaultValue: "An organization unit type with the same handle already exists",
		},
	}

	// ErrorInvalidOUTypeRequest is the error returned when the organization unit type request is invalid.
	ErrorInvalidOUTypeRequest = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OUT-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.outypeservice.invalid_ou_type_request",
			DefaultValue: "Invalid organization unit type request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.outypeservice.invalid_ou_type_request_description",
			DefaultValue: "The organization unit type request contains invalid or missing required fields",
		},
	}

	// ErrorInvalidLimit is the error returned when the limit parameter is invalid.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OUT-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.outypeservice.invalid_limit_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.outypeservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}

	// ErrorInvalidOffset is the error returned when the offset parameter is invalid.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OUT-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.outypeservice.invalid_offset_parameter",
			DefaultValue: "Invalid pagination parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.outypeservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}

	// ErrorInvalidOUTypeHandle is the error returned when the handle has an invalid format.
	ErrorInvalidOUTypeHandle = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OUT-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.outypeservice.invalid_ou_type_handle",
			DefaultValue: "Invalid organization unit type handle",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.outypeservice.invalid_ou_type_handle_description",
			DefaultValue: "The handle must contain only lowercase letters, numbers, hyphens and underscores, " +
				"and must start and end with a letter or number",
		},
	}

	// ErrorOUTypeHandleUpdateNotAllowed is the error returned when an update changes the handle.
	ErrorOUTypeHandleUpdateNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OUT-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.outypeservice.ou_type_handle_update_not_allowed",
			DefaultValue: "Organization unit type handle cannot be changed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.outypeservice.ou_type_handle_update_not_allowed_description",
			DefaultValue: "Omit the handle or send the current handle of the organization unit type",
		},
	}

	// ErrorCredentialAttributeNotAllowed is the error returned when an organization unit type declares a
	// credential attribute. Organization units never sign in, so their types hold no credentials.
	ErrorCredentialAttributeNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OUT-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.outypeservice.credential_attribute_not_allowed",
			DefaultValue: "Credential attribute not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.outypeservice.credential_attribute_not_allowed_description",
			DefaultValue: "Organization unit types cannot declare credential attributes. Remove the credential flag",
		},
	}

	// ErrorCannotModifyDeclarativeOUType is the error returned when a change targets an organization unit
	// type declared in a resource file.
	ErrorCannotModifyDeclarativeOUType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OUT-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.outypeservice.cannot_modify_declarative_ou_type",
			DefaultValue: "Cannot modify a declarative organization unit type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.outypeservice.cannot_modify_declarative_ou_type_description",
			DefaultValue: "The organization unit type is declared in a resource file and is read only",
		},
	}

	// ErrorCannotModifyManagedOUType is the error returned when a change targets an organization unit type
	// a control plane deployed.
	ErrorCannotModifyManagedOUType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "OUT-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.outypeservice.cannot_modify_managed_ou_type",
			DefaultValue: "Cannot modify an organization unit type deployed by the control plane",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.outypeservice.cannot_modify_managed_ou_type_description",
			DefaultValue: "The organization unit type was deployed by the control plane and can only be changed there",
		},
	}
)

// invalidOUTypeRequestErr returns ErrorInvalidOUTypeRequest with the detail appended to the description's
// default value.
func invalidOUTypeRequestErr(detail string) *tidcommon.ServiceError {
	e := ErrorInvalidOUTypeRequest
	if detail != "" {
		e.ErrorDescription.DefaultValue += ": " + detail
	}
	return &e
}
