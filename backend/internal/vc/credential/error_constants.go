// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"errors"
	"net/http"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Internal sentinel errors for the composite/file-based credential store.
var (
	// ErrNotFound is the store-level not-found sentinel.
	ErrNotFound = errors.New("openid4vci: credential configuration not found")

	// ErrConfigurationIsImmutable is returned when trying to modify or delete an
	// immutable (file-based) credential configuration.
	ErrConfigurationIsImmutable = errors.New("credential configuration is immutable")

	// ErrResultLimitExceededInCompositeMode is returned when composite store results
	// exceed the configured limit.
	ErrResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")

	// ErrConfigurationDataCorrupted is returned when declarative store data is malformed.
	ErrConfigurationDataCorrupted = errors.New("credential configuration data is corrupted")
)

// Client-facing API errors for the credential-configuration management endpoints.
var (
	// ErrorConfigurationInvalidRequest indicates a malformed create/update request.
	ErrorConfigurationInvalidRequest = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2001",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_invalid_request",
			DefaultValue: "Invalid request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_invalid_request_description",
			DefaultValue: "The credential configuration request is missing required fields or is malformed",
		},
	}

	// ErrorConfigurationNotFound indicates the credential configuration does not exist.
	ErrorConfigurationNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2002",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_not_found",
			DefaultValue: "Credential configuration not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_not_found_description",
			DefaultValue: "No credential configuration exists for the supplied identifier",
		},
	}

	// ErrorConfigurationAlreadyExists indicates the handle is already in use.
	ErrorConfigurationAlreadyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2003",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_already_exists",
			DefaultValue: "Credential configuration already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_already_exists_description",
			DefaultValue: "A credential configuration with the supplied handle already exists",
		},
	}

	// ErrorConfigurationUnsupportedFormat indicates an unsupported credential format.
	ErrorConfigurationUnsupportedFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2004",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_unsupported_format",
			DefaultValue: "Unsupported credential format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_unsupported_format_description",
			DefaultValue: "Only the dc+sd-jwt credential format is supported",
		},
	}

	// ErrorConfigurationImmutable indicates the credential configuration is declarative
	// (file-based) and cannot be modified or deleted via the management API.
	ErrorConfigurationImmutable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2005",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_immutable",
			DefaultValue: "Credential configuration is immutable",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.vci.configuration_immutable_description",
			DefaultValue: "The credential configuration is defined in declarative configuration " +
				"and cannot be modified or deleted",
		},
	}

	// ErrorConfigurationResultLimitExceeded indicates the merged composite-store result
	// set exceeds the supported maximum.
	ErrorConfigurationResultLimitExceeded = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2006",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_result_limit_exceeded",
			DefaultValue: "Result limit exceeded",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.vci.configuration_result_limit_exceeded_description",
			DefaultValue: "The number of credential configurations exceeds the supported limit in " +
				"hybrid mode",
		},
	}

	// ErrorConfigurationInvalidOU indicates the organization unit is missing or does not exist.
	ErrorConfigurationInvalidOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2007",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_invalid_ou",
			DefaultValue: "Invalid organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_invalid_ou_description",
			DefaultValue: "A valid organization unit (ouId or ouHandle) is required",
		},
	}

	// ErrorConfigurationEmptyClaimName indicates a claim was declared without a name.
	ErrorConfigurationEmptyClaimName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2008",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_empty_claim_name",
			DefaultValue: "Invalid claim",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_empty_claim_name_description",
			DefaultValue: "Claim names must not be empty",
		},
	}

	// ErrorConfigurationDuplicateClaim indicates the same claim name was declared more than once.
	ErrorConfigurationDuplicateClaim = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2009",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_duplicate_claim",
			DefaultValue: "Duplicate claim",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_duplicate_claim_description",
			DefaultValue: "Claim name '{{param(claim)}}' is declared more than once",
		},
	}

	// ErrorConfigurationReservedClaim indicates a claim name collides with a registered SD-JWT
	// claim. Issuing such a credential would place the name in both the payload and a disclosure,
	// which a conformant wallet must reject.
	ErrorConfigurationReservedClaim = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2010",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_reserved_claim",
			DefaultValue: "Reserved claim",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.vci.configuration_reserved_claim_description",
			DefaultValue: "Claim name '{{param(claim)}}' is reserved by the SD-JWT VC format and " +
				"cannot be used as a credential claim",
		},
	}

	// ErrorConfigurationDeclarativeModeCreateNotAllowed indicates a create was attempted
	// while the store is in declarative-only mode, where configurations come from files.
	ErrorConfigurationDeclarativeModeCreateNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VCI-2011",
		Error: tidcommon.I18nMessage{
			Key:          "error.vci.configuration_declarative_mode_create_not_allowed",
			DefaultValue: "Cannot create credential configuration in declarative-only mode",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.vci.configuration_declarative_mode_create_not_allowed_description",
			DefaultValue: "Credential configuration creation is not allowed when running in " +
				"declarative-only mode. Configurations must be defined in declarative configuration files",
		},
	}
)

// configurationClientErrorStatus maps a client-facing error to its HTTP status.
func configurationClientErrorStatus(code string) int {
	switch code {
	case ErrorConfigurationNotFound.Code:
		return http.StatusNotFound
	case ErrorConfigurationAlreadyExists.Code, ErrorConfigurationImmutable.Code:
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
