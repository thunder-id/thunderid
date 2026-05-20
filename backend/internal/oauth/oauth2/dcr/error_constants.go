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

package dcr

import (
	"strconv"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// errInvalidBCP47Tag is returned when a language tag in a DCR request field is not valid BCP 47.
type errInvalidBCP47Tag struct{ key string }

// Error implements the error interface.
func (e *errInvalidBCP47Tag) Error() string {
	return "invalid BCP 47 language tag in field \"" + e.key + "\""
}

// errTooManyLocalizedVariants is returned when a localizable field exceeds maxLocalizedVariantsPerField.
type errTooManyLocalizedVariants struct{ field string }

// Error implements the error interface.
func (e *errTooManyLocalizedVariants) Error() string {
	return "field \"" + e.field + "\" exceeds the maximum of " +
		strconv.Itoa(maxLocalizedVariantsPerField) + " localized variants"
}

// DCR standard service error constants
var (
	// ErrorInvalidRequestFormat is used for nil request validation
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "invalid_client_metadata",
		Error: core.I18nMessage{
			Key:          "error.dcr.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.dcr.invalid_request_format_description",
			DefaultValue: "The request body is missing or has an invalid format",
		},
	}

	// ErrorInvalidRedirectURI is the standard error for redirect URI issues
	ErrorInvalidRedirectURI = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "invalid_redirect_uri",
		Error: core.I18nMessage{
			Key:          "error.dcr.invalid_redirect_uri",
			DefaultValue: "Invalid redirect URI",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.dcr.invalid_redirect_uri_description",
			DefaultValue: "One or more redirect URIs are invalid",
		},
	}

	// ErrorInvalidClientMetadata is the standard error for client metadata issues
	ErrorInvalidClientMetadata = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "invalid_client_metadata",
		Error: core.I18nMessage{
			Key:          "error.dcr.invalid_client_metadata",
			DefaultValue: "Invalid client metadata",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.dcr.invalid_client_metadata_description",
			DefaultValue: "One or more client metadata values are invalid",
		},
	}

	// ErrorJWKSConfigurationConflict is the error returned when both jwks and jwks_uri are provided
	ErrorJWKSConfigurationConflict = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "invalid_client_metadata",
		Error: core.I18nMessage{
			Key:          "error.dcr.jwks_configuration_conflict",
			DefaultValue: "JWKS configuration conflict",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.dcr.jwks_configuration_conflict_description",
			DefaultValue: "Cannot specify both 'jwks' and 'jwks_uri' parameters",
		},
	}

	// ErrorServerError is the standard error for server issues
	ErrorServerError = serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "server_error",
		Error: core.I18nMessage{
			Key:          "error.dcr.server_error",
			DefaultValue: "Server error",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.dcr.server_error_description",
			DefaultValue: "An unexpected error occurred while processing the request",
		},
	}

	// ErrorUnauthorized is the error returned when the request lacks valid authentication
	// or the authenticated caller does not hold required permissions.
	ErrorUnauthorized = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "unauthorized_client",
		Error: core.I18nMessage{
			Key:          "error.dcr.unauthorized",
			DefaultValue: "Unauthorized",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.dcr.unauthorized_description",
			DefaultValue: "Authentication with sufficient permissions is required to register a client",
		},
	}
)
