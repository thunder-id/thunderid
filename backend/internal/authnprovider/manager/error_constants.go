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

// Package manager coordinates authentication provider registration and dispatch.
package manager

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var (
	// ErrorAuthenticationFailed is returned when the underlying provider rejects the authentication
	// attempt due to a client-side reason (e.g. invalid credentials).
	ErrorAuthenticationFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-MGR-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.authentication_failed",
			DefaultValue: "Authentication failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.authentication_failed_description",
			DefaultValue: "The authentication attempt failed",
		},
	}

	// ErrorEnrollmentFailed is returned when the underlying provider rejects the enrollment
	// attempt due to a client-side reason (e.g. invalid credential data).
	ErrorEnrollmentFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-MGR-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.enrollment_failed",
			DefaultValue: "Enrollment failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.enrollment_failed_description",
			DefaultValue: "The enrollment attempt failed",
		},
	}

	// ErrorGetAttributesClientError is returned when the underlying provider rejects the
	// attribute fetch due to a client-side reason (e.g. invalid or expired token).
	ErrorGetAttributesClientError = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-MGR-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.failed_to_get_attributes",
			DefaultValue: "Failed to get attributes",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.failed_to_get_attributes_description",
			DefaultValue: "The attribute fetch was rejected by the provider",
		},
	}

	// ErrorUserNotFound is returned when the underlying provider indicates no user was found
	// matching the provided identifiers.
	ErrorUserNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-MGR-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.user_not_found",
			DefaultValue: "User not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.user_not_found_description",
			DefaultValue: "No user found matching the provided identifiers",
		},
	}

	// ErrorInvalidRequest is returned when the underlying provider rejects the authentication
	// request as invalid.
	ErrorInvalidRequest = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-MGR-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.invalid_request",
			DefaultValue: "Invalid request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.invalid_request_description",
			DefaultValue: "The authentication request is invalid",
		},
	}

	// ErrorAmbiguousUser is returned when the underlying provider finds multiple users
	// matching the provided identifiers.
	ErrorAmbiguousUser = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-MGR-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.ambiguous_user",
			DefaultValue: "Ambiguous user",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.ambiguous_user_description",
			DefaultValue: "Multiple users found matching the provided identifiers",
		},
	}

	// ErrorGetEntityReferenceClientError is returned when the underlying provider rejects the
	// entity reference fetch due to a client-side reason.
	ErrorGetEntityReferenceClientError = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-MGR-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.get_entity_reference_client_error",
			DefaultValue: "Failed to get entity reference",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnmgrservice.get_entity_reference_client_error_description",
			DefaultValue: "The entity reference fetch was rejected by the provider",
		},
	}
)
