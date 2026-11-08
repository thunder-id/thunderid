// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mgt

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for i18n operations.
var (
	// ErrorInvalidLanguage is the error returned when language tag format is invalid.
	ErrorInvalidLanguage = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "I18N-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.i18nservice.invalid_language",
			DefaultValue: "Invalid language tag format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.i18nservice.invalid_language_description",
			DefaultValue: "The language tag must follow canonical BCP 47 format (e.g., 'en', 'es', 'fr-CA')",
		},
	}
	// ErrorInvalidNamespace is the error returned when namespace format is invalid.
	ErrorInvalidNamespace = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "I18N-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.i18nservice.invalid_namespace",
			DefaultValue: "Invalid namespace format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.i18nservice.invalid_namespace_description",
			DefaultValue: "The namespace can only contain alphanumeric characters, underscores, and hyphens",
		},
	}
	// ErrorInvalidKey is the error returned when key format is invalid.
	ErrorInvalidKey = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "I18N-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.i18nservice.invalid_key",
			DefaultValue: "Invalid key format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.i18nservice.invalid_key_description",
			DefaultValue: "The key can only contain alphanumeric characters, dots, underscores, and hyphens",
		},
	}
	// ErrorMissingLanguage is the error returned when language code is missing.
	ErrorMissingLanguage = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "I18N-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.i18nservice.missing_language",
			DefaultValue: "Missing language code",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.i18nservice.missing_language_description",
			DefaultValue: "Language code is required",
		},
	}
	// ErrorMissingValue is the error returned when translation value is missing.
	ErrorMissingValue = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "I18N-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.i18nservice.missing_value",
			DefaultValue: "Missing translation value",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.i18nservice.missing_value_description",
			DefaultValue: "Translation value is required",
		},
	}
	// ErrorTranslationNotFound is the error returned when translation is not found.
	ErrorTranslationNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "I18N-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.i18nservice.translation_not_found",
			DefaultValue: "Translation not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.i18nservice.translation_not_found_description",
			DefaultValue: "The requested translation does not exist for the specified language, namespace, and key",
		},
	}
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "I18N-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.i18nservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.i18nservice.invalid_request_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorEmptyTranslations is the error returned when translations map is empty.
	ErrorEmptyTranslations = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "I18N-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.i18nservice.empty_translations",
			DefaultValue: "Empty translations",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.i18nservice.empty_translations_description",
			DefaultValue: "At least one translation must be provided",
		},
	} // ErrorManagedLanguage is the error returned when attempting to change the translations of a language a
	// control plane deployed. Only a deploy from the control plane changes them.
	ErrorManagedLanguage = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "I18N-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.i18nservice.managed_language",
			DefaultValue: "Cannot modify a language deployed by the control plane",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.i18nservice.managed_language_description",
			DefaultValue: "The language's translations were deployed by the control plane and can only be " +
				"changed there",
		},
	}
)
