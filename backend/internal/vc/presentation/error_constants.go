// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

import (
	"errors"
	"net/http"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Internal sentinel errors for the composite presentation-definition store.
var (
	// ErrDefinitionIsImmutable is returned when trying to modify or delete an
	// immutable (file-based) presentation definition.
	ErrDefinitionIsImmutable = errors.New("presentation definition is immutable")

	// ErrResultLimitExceededInCompositeMode is the internal sentinel error returned
	// when composite store results exceed the configured limit.
	ErrResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")

	// ErrDefinitionDataCorrupted is returned when declarative store data is malformed.
	ErrDefinitionDataCorrupted = errors.New("presentation definition data is corrupted")
)

// Client-facing API errors for the presentation-definition management endpoints.
var (
	// ErrorDefinitionInvalidRequest indicates a malformed create/update request.
	ErrorDefinitionInvalidRequest = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2001",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_invalid_request",
			DefaultValue: "Invalid request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vp.definition_invalid_request_description",
			DefaultValue: "The presentation definition request is missing required fields or is malformed",
		},
	}

	// ErrorDefinitionNotFound indicates the presentation definition does not exist.
	ErrorDefinitionNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2002",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_not_found",
			DefaultValue: "Presentation definition not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vp.definition_not_found_description",
			DefaultValue: "No presentation definition exists for the supplied identifier",
		},
	}

	// ErrorDefinitionAlreadyExists indicates the handle is already in use.
	ErrorDefinitionAlreadyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2003",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_already_exists",
			DefaultValue: "Presentation definition already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vp.definition_already_exists_description",
			DefaultValue: "A presentation definition with the supplied handle already exists",
		},
	}

	// ErrorDefinitionUnsupportedFormat indicates an unsupported credential format.
	ErrorDefinitionUnsupportedFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2004",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_unsupported_format",
			DefaultValue: "Unsupported credential format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vp.definition_unsupported_format_description",
			DefaultValue: "Only the dc+sd-jwt credential format is supported",
		},
	}

	// ErrorDefinitionImmutable indicates the presentation definition is declarative
	// (file-based) and cannot be modified or deleted via the management API.
	ErrorDefinitionImmutable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2005",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_immutable",
			DefaultValue: "Presentation definition is immutable",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.vp.definition_immutable_description",
			DefaultValue: "The presentation definition is defined in declarative configuration " +
				"and cannot be modified or deleted",
		},
	}

	// ErrorDefinitionResultLimitExceeded indicates the merged composite-store result
	// set exceeds the supported maximum.
	ErrorDefinitionResultLimitExceeded = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2006",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_result_limit_exceeded",
			DefaultValue: "Result limit exceeded",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.vp.definition_result_limit_exceeded_description",
			DefaultValue: "The number of presentation definitions exceeds the supported limit in " +
				"hybrid mode. Use search for larger datasets",
		},
	}

	// ErrorDefinitionInvalidOU indicates the organization unit is missing or does not exist.
	ErrorDefinitionInvalidOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2007",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_invalid_ou",
			DefaultValue: "Invalid organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vp.definition_invalid_ou_description",
			DefaultValue: "A valid organization unit (ouId or ouHandle) is required",
		},
	}

	// ErrorDefinitionUnknownTrustedAuthority indicates a trusted authority is not a configured trust anchor.
	ErrorDefinitionUnknownTrustedAuthority = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2008",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_unknown_trusted_authority",
			DefaultValue: "Unknown trusted authority",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vp.definition_unknown_trusted_authority_description",
			DefaultValue: "Each trusted authority must name a configured trust anchor",
		},
	}
)

// definitionClientErrorStatus maps a client-facing definition error to its HTTP status.
func definitionClientErrorStatus(code string) int {
	switch code {
	case ErrorDefinitionNotFound.Code:
		return http.StatusNotFound
	case ErrorDefinitionAlreadyExists.Code, ErrorDefinitionImmutable.Code:
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
