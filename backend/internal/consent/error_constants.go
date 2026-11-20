// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package consent

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// errConsentNotFound is the sentinel error returned by the store when a consent record does not exist.
var errConsentNotFound = errors.New("consent not found")

// errNilEventBus is returned when the consent service is initialized without the event bus it audits on.
var errNilEventBus = errors.New("the consent service needs an event bus")

// Client errors for consent operations.
var (
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CNS-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorMissingConsentID is the error returned when the consent ID is missing.
	ErrorMissingConsentID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CNS-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentservice.missing_consent_id",
			DefaultValue: "Missing consent ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentservice.missing_consent_id_description",
			DefaultValue: "Consent ID is required",
		},
	}
	// ErrorConsentNotFound is the error returned when a consent record is not found.
	ErrorConsentNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CNS-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentservice.consent_not_found",
			DefaultValue: "Consent not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentservice.consent_not_found_description",
			DefaultValue: "The consent with the specified id does not exist",
		},
	}
	// ErrorInvalidConsentStatus is the error returned when the consent status filter is not recognized.
	ErrorInvalidConsentStatus = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CNS-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentservice.invalid_consent_status",
			DefaultValue: "Invalid consent status",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentservice.invalid_consent_status_description",
			DefaultValue: "The provided consent status is not a recognized value",
		},
	}
	// ErrorInvalidAuthorizationType is the error returned when an authorization type is not recognized.
	ErrorInvalidAuthorizationType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CNS-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentservice.invalid_authorization_type",
			DefaultValue: "Invalid authorization type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentservice.invalid_authorization_type_description",
			DefaultValue: "The provided consent authorization type is not a recognized value",
		},
	}
	// ErrorInvalidAuthorizationStatus is the error returned when an authorization status is not recognized.
	ErrorInvalidAuthorizationStatus = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CNS-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentservice.invalid_authorization_status",
			DefaultValue: "Invalid authorization status",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentservice.invalid_authorization_status_description",
			DefaultValue: "The provided consent authorization status is not a recognized value",
		},
	}
	// ErrorInvalidNamespace is the error returned when a consent element namespace is not recognized.
	ErrorInvalidNamespace = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CNS-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentservice.invalid_namespace",
			DefaultValue: "Invalid namespace",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentservice.invalid_namespace_description",
			DefaultValue: "The provided consent namespace is not a recognized value",
		},
	}
)
