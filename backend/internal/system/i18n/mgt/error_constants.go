/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package mgt

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for i18n operations.
var (
	// ErrorInvalidLanguage is the error returned when language tag format is invalid.
	ErrorInvalidLanguage = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "I18N-1001",
		Error: core.I18nMessage{
			Key:          "error.i18nservice.invalid_language",
			DefaultValue: "Invalid language tag format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.i18nservice.invalid_language_description",
			DefaultValue: "The language tag must follow canonical BCP 47 format (e.g., 'en', 'es', 'fr-CA')",
		},
	}
	// ErrorInvalidNamespace is the error returned when namespace format is invalid.
	ErrorInvalidNamespace = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "I18N-1002",
		Error: core.I18nMessage{
			Key:          "error.i18nservice.invalid_namespace",
			DefaultValue: "Invalid namespace format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.i18nservice.invalid_namespace_description",
			DefaultValue: "The namespace can only contain alphanumeric characters, underscores, and hyphens",
		},
	}
	// ErrorInvalidKey is the error returned when key format is invalid.
	ErrorInvalidKey = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "I18N-1003",
		Error: core.I18nMessage{
			Key:          "error.i18nservice.invalid_key",
			DefaultValue: "Invalid key format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.i18nservice.invalid_key_description",
			DefaultValue: "The key can only contain alphanumeric characters, dots, underscores, and hyphens",
		},
	}
	// ErrorMissingLanguage is the error returned when language code is missing.
	ErrorMissingLanguage = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "I18N-1004",
		Error: core.I18nMessage{
			Key:          "error.i18nservice.missing_language",
			DefaultValue: "Missing language code",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.i18nservice.missing_language_description",
			DefaultValue: "Language code is required",
		},
	}
	// ErrorMissingValue is the error returned when translation value is missing.
	ErrorMissingValue = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "I18N-1005",
		Error: core.I18nMessage{
			Key:          "error.i18nservice.missing_value",
			DefaultValue: "Missing translation value",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.i18nservice.missing_value_description",
			DefaultValue: "Translation value is required",
		},
	}
	// ErrorTranslationNotFound is the error returned when translation is not found.
	ErrorTranslationNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "I18N-1006",
		Error: core.I18nMessage{
			Key:          "error.i18nservice.translation_not_found",
			DefaultValue: "Translation not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.i18nservice.translation_not_found_description",
			DefaultValue: "The requested translation does not exist for the specified language, namespace, and key",
		},
	}
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "I18N-1007",
		Error: core.I18nMessage{
			Key:          "error.i18nservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.i18nservice.invalid_request_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorEmptyTranslations is the error returned when translations map is empty.
	ErrorEmptyTranslations = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "I18N-1008",
		Error: core.I18nMessage{
			Key:          "error.i18nservice.empty_translations",
			DefaultValue: "Empty translations",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.i18nservice.empty_translations_description",
			DefaultValue: "At least one translation must be provided",
		},
	}
)
