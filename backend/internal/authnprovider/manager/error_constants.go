/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package manager

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

var (
	// ErrorAuthenticationFailed is returned when the underlying provider rejects the authentication
	// attempt due to a client-side reason (e.g. invalid credentials).
	ErrorAuthenticationFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-MGR-1001",
		Error: core.I18nMessage{
			Key:          "error.authnmgrservice.authentication_failed",
			DefaultValue: "Authentication failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnmgrservice.authentication_failed_description",
			DefaultValue: "The authentication attempt failed",
		},
	}

	// ErrorGetAttributesClientError is returned when the underlying provider rejects the
	// attribute fetch due to a client-side reason (e.g. invalid or expired token).
	ErrorGetAttributesClientError = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-MGR-1004",
		Error: core.I18nMessage{
			Key:          "error.authnmgrservice.failed_to_get_attributes",
			DefaultValue: "Failed to get attributes",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnmgrservice.failed_to_get_attributes_description",
			DefaultValue: "The attribute fetch was rejected by the provider",
		},
	}

	// ErrorUserNotFound is returned when the underlying provider indicates no user was found
	// matching the provided identifiers.
	ErrorUserNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-MGR-1007",
		Error: core.I18nMessage{
			Key:          "error.authnmgrservice.user_not_found",
			DefaultValue: "User not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnmgrservice.user_not_found_description",
			DefaultValue: "No user found matching the provided identifiers",
		},
	}

	// ErrorInvalidRequest is returned when the underlying provider rejects the authentication
	// request as invalid.
	ErrorInvalidRequest = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-MGR-1008",
		Error: core.I18nMessage{
			Key:          "error.authnmgrservice.invalid_request",
			DefaultValue: "Invalid request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnmgrservice.invalid_request_description",
			DefaultValue: "The authentication request is invalid",
		},
	}
)
