// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for authorization details type management.
var (
	// ErrorInvalidRequestFormat is returned when the request body cannot be decoded.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorTypeNotFound is returned when no type matches the given id.
	ErrorTypeNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.type_not_found",
			DefaultValue: "Authorization details type not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.type_not_found_description",
			DefaultValue: "The authorization details type with the specified id does not exist",
		},
	}
	// ErrorTypeConflict is returned when the type value is already registered.
	ErrorTypeConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.type_conflict",
			DefaultValue: "Authorization details type conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.type_conflict_description",
			DefaultValue: "An authorization details type with the same type value already exists",
		},
	}
	// ErrorInvalidTypeValue is returned when the type value is missing or malformed.
	ErrorInvalidTypeValue = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_type_value",
			DefaultValue: "Invalid authorization details type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_type_value_description",
			DefaultValue: "The type value is required, must be printable ASCII, and must be at most 512 characters",
		},
	}
	// ErrorInvalidName is returned when the name is missing or too long.
	ErrorInvalidName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_name",
			DefaultValue: "Invalid name",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_name_description",
			DefaultValue: "The name is required and must be at most 100 characters",
		},
	}
	// ErrorInvalidSchema is returned when the schema or its field policies are invalid.
	ErrorInvalidSchema = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_schema",
			DefaultValue: "Invalid schema",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_schema_description",
			DefaultValue: "The schema is not valid: {{param(reason)}}",
		},
	}
	// ErrorInvalidProcessor is returned when the processor configuration is invalid.
	ErrorInvalidProcessor = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_processor",
			DefaultValue: "Invalid processor",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.authorizationdetailservice.invalid_processor_description",
			DefaultValue: "The processor endpoint must be an absolute HTTPS URL; HTTP is accepted only on a " +
				"loopback host",
		},
	}
	// ErrorInvalidProcessorAuthentication is returned when the processor's authentication is invalid.
	ErrorInvalidProcessorAuthentication = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_processor_authentication",
			DefaultValue: "Invalid processor authentication",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.authorizationdetailservice.invalid_processor_authentication_description",
			DefaultValue: "The processor authentication must be a supported method with its required " +
				"properties, and is accepted only for a type with a processor",
		},
	}
	// ErrorProcessorAuthenticationRequired is returned when a processor endpoint change omits credentials.
	ErrorProcessorAuthenticationRequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.processor_authentication_required",
			DefaultValue: "Processor authentication required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.authorizationdetailservice.processor_authentication_required_description",
			DefaultValue: "The processor credentials must be given again when its endpoint changes, so " +
				"they are never sent to an endpoint they were not given for",
		},
	}
	// ErrorTypeValueImmutable is returned when an update attempts to change the type value.
	ErrorTypeValueImmutable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.type_value_immutable",
			DefaultValue: "Type value cannot be changed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.type_value_immutable_description",
			DefaultValue: "The type value of a registered authorization details type cannot be changed",
		},
	}
	// ErrorInvalidConsentTemplate is returned when a consent template is too long or names unknown fields.
	ErrorInvalidConsentTemplate = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_consent_template",
			DefaultValue: "Invalid consent template",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_consent_template_description",
			DefaultValue: "The consent template is not valid: {{param(reason)}}",
		},
	}
	// ErrorInvalidPagination is returned when a list request's limit or offset is out of range.
	ErrorInvalidPagination = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_pagination",
			DefaultValue: "Invalid pagination parameters",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.invalid_pagination_description",
			DefaultValue: "The limit must be between 1 and 100, and the offset zero or more",
		},
	}
	// ErrorImmutableType is returned when attempting to modify or delete a declarative type.
	ErrorImmutableType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.cannot_modify_declarative_type",
			DefaultValue: "Cannot modify declarative authorization details type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.authorizationdetailservice.cannot_modify_declarative_type_description",
			DefaultValue: "The authorization details type is defined in declarative configuration and " +
				"cannot be modified",
		},
	}
	// ErrorDeclarativeModeCreateNotAllowed is returned when creating a type in declarative-only mode.
	ErrorDeclarativeModeCreateNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.cannot_create_type_in_declarative_only_mode",
			DefaultValue: "Cannot create authorization details type in declarative-only mode",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.authorizationdetailservice.cannot_create_type_in_declarative_only_mode_description",
			DefaultValue: "Authorization details type creation is not allowed when running in declarative-only " +
				"mode. Types must be defined in declarative configuration files",
		},
	}
	// ErrorTypeIDConflict is returned when a type is created with an id that is already in use.
	ErrorTypeIDConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1015",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.type_id_conflict",
			DefaultValue: "Authorization details type id conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.type_id_conflict_description",
			DefaultValue: "An authorization details type with the same id already exists",
		},
	}
	// ErrorResultLimitExceededInCompositeMode is returned when the merged list of types exceeds the composite
	// mode limit.
	ErrorResultLimitExceededInCompositeMode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1017",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.result_limit_exceeded_in_composite_mode",
			DefaultValue: "Result limit exceeded in composite mode",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.result_limit_exceeded_in_composite_mode_description",
			DefaultValue: "The total number of records exceeds the maximum limit in composite mode",
		},
	}
	// ErrorTypeChanged is returned when a type's version moved while it was being updated.
	ErrorTypeChanged = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "RAR-1016",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorizationdetailservice.type_changed",
			DefaultValue: "Authorization details type changed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.authorizationdetailservice.type_changed_description",
			DefaultValue: "The authorization details type changed while it was being updated; read it again " +
				"and reapply the change",
		},
	}
)

// Internal errors.
var (
	// errTypeNotFound is returned by the store when no type matches.
	errTypeNotFound = errors.New("authorization details type not found")
	// errTypeImmutable is returned by the store when a declarative type is written to.
	errTypeImmutable = errors.New("authorization details type is immutable")
	// errResultLimitExceededInCompositeMode is returned when the merged list of types exceeds the composite
	// mode limit.
	errResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")
	// errTypeDataCorrupted is returned when a declarative entry does not hold a type.
	errTypeDataCorrupted = errors.New("authorization details type data is corrupted")
)
