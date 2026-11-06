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

	// errInvalidCertificatePEM is returned when a trusted authority certificate is not a single
	// PEM-encoded X.509 certificate.
	errInvalidCertificatePEM = errors.New("certificate must be a single PEM-encoded X.509 certificate")
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

	// ErrorDefinitionEmptyClaimName indicates a requested claim was declared without a name.
	ErrorDefinitionEmptyClaimName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2008",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_empty_claim_name",
			DefaultValue: "Invalid claim",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vp.definition_empty_claim_name_description",
			DefaultValue: "Claim names must not be empty",
		},
	}

	// ErrorDefinitionDuplicateClaim indicates the same claim name was requested more than once,
	// either twice within a list or as both a mandatory and an optional claim.
	ErrorDefinitionDuplicateClaim = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2009",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_duplicate_claim",
			DefaultValue: "Duplicate claim",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vp.definition_duplicate_claim_description",
			DefaultValue: "Claim name '{{param(claim)}}' is requested more than once",
		},
	}

	// ErrorDefinitionDeclarativeModeCreateNotAllowed indicates a create was attempted while
	// the store is in declarative-only mode, where definitions come from files.
	ErrorDefinitionDeclarativeModeCreateNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2010",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_declarative_mode_create_not_allowed",
			DefaultValue: "Cannot create presentation definition in declarative-only mode",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.vp.definition_declarative_mode_create_not_allowed_description",
			DefaultValue: "Presentation definition creation is not allowed when running in " +
				"declarative-only mode. Definitions must be defined in declarative configuration files",
		},
	}

	// ErrorDefinitionEmptyTrustedAuthorityName indicates a trusted authority was declared without a name.
	ErrorDefinitionEmptyTrustedAuthorityName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2011",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_empty_trusted_authority_name",
			DefaultValue: "Invalid trusted authority",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vp.definition_empty_trusted_authority_name_description",
			DefaultValue: "Trusted authority names must not be empty",
		},
	}

	// ErrorDefinitionDuplicateTrustedAuthority indicates a trusted authority name or certificate
	// appears more than once in the same presentation definition.
	ErrorDefinitionDuplicateTrustedAuthority = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2012",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_duplicate_trusted_authority",
			DefaultValue: "Duplicate trusted authority",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.vp.definition_duplicate_trusted_authority_description",
			DefaultValue: "Trusted authority '{{param(name)}}' repeats a name or certificate " +
				"already registered on this presentation definition",
		},
	}

	// ErrorDefinitionInvalidTrustedAuthorityCertificate indicates a trusted authority has no
	// certificate, or its certificate is not a valid PEM-encoded X.509 certificate.
	ErrorDefinitionInvalidTrustedAuthorityCertificate = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2013",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_invalid_trusted_authority_certificate",
			DefaultValue: "Invalid trusted authority certificate",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.vp.definition_invalid_trusted_authority_certificate_description",
			DefaultValue: "Trusted authority '{{param(name)}}' requires a single PEM-encoded X.509 " +
				"certificate. Supply the certificate when adding a trusted authority",
		},
	}

	// ErrorDefinitionExpiredTrustedAuthorityCertificate indicates a trusted authority certificate has expired.
	ErrorDefinitionExpiredTrustedAuthorityCertificate = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2014",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_expired_trusted_authority_certificate",
			DefaultValue: "Expired trusted authority certificate",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vp.definition_expired_trusted_authority_certificate_description",
			DefaultValue: "The certificate of trusted authority '{{param(name)}}' has expired",
		},
	}

	// ErrorDefinitionTooManyTrustedAuthorities indicates more trusted authorities were supplied than allowed.
	ErrorDefinitionTooManyTrustedAuthorities = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2015",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_too_many_trusted_authorities",
			DefaultValue: "Too many trusted authorities",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vp.definition_too_many_trusted_authorities_description",
			DefaultValue: "A presentation definition supports at most {{param(max)}} trusted authorities",
		},
	}

	// ErrorDefinitionTrustedIssuerWithoutAuthorities indicates issuer trust is enforced
	// on a presentation definition that has no trusted authorities.
	ErrorDefinitionTrustedIssuerWithoutAuthorities = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2016",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_trusted_issuer_without_authorities",
			DefaultValue: "No trusted authorities",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vp.definition_trusted_issuer_without_authorities_description",
			DefaultValue: "Add at least one trusted authority certificate before enforcing issuer trust",
		},
	}

	// ErrorDefinitionIDConflict indicates a create named an ID that another presentation definition already holds.
	ErrorDefinitionIDConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "VP-2017",
		Error: tidcommon.I18nMessage{
			Key:          "error.vp.definition_id_conflict",
			DefaultValue: "Presentation definition ID conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.vp.definition_id_conflict_description",
			DefaultValue: "A presentation definition with the specified ID already exists",
		},
	}
)

// definitionClientErrorStatus maps a client-facing definition error to its HTTP status.
func definitionClientErrorStatus(code string) int {
	switch code {
	case ErrorDefinitionNotFound.Code:
		return http.StatusNotFound
	case ErrorDefinitionAlreadyExists.Code, ErrorDefinitionImmutable.Code, ErrorDefinitionIDConflict.Code:
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
