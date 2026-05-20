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

package common

import (
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// API errors

// APIErrorInvalidRequestFormat is returned when the request body is malformed.
var APIErrorInvalidRequestFormat = apierror.ErrorResponse{
	Code: "AUTHN-1000",
	Message: core.I18nMessage{
		Key:          "error.authncredservice.invalid_request_format",
		DefaultValue: "Invalid request format",
	},
	Description: core.I18nMessage{
		Key:          "error.authncredservice.invalid_request_format_description",
		DefaultValue: "The request body is malformed or contains invalid data",
	},
}

// Client errors for the service
var (
	// ErrorInvalidIDPID is the error returned when the provided IDP ID is invalid.
	ErrorInvalidIDPID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-1001",
		Error: core.I18nMessage{
			Key:          "error.authnservice.invalid_idp_id",
			DefaultValue: "Invalid identity provider ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.invalid_idp_id_description",
			DefaultValue: "The provided identity provider ID is invalid or empty",
		},
	}
	// ErrorClientErrorWhileRetrievingIDP is the error returned when there is a client error while retrieving the IDP.
	ErrorClientErrorWhileRetrievingIDP = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-1002",
		Error: core.I18nMessage{
			Key:          "error.authnservice.error_retrieving_idp",
			DefaultValue: "Error retrieving identity provider",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.error_retrieving_idp_description",
			DefaultValue: "An error occurred while retrieving the identity provider",
		},
	}
	// ErrorInvalidIDPType is the error returned when the provided IDP type is invalid.
	ErrorInvalidIDPType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-1003",
		Error: core.I18nMessage{
			Key:          "error.authnservice.invalid_idp_type",
			DefaultValue: "Invalid identity provider type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.invalid_idp_type_description",
			DefaultValue: "The requested identity provider type is invalid",
		},
	}
	// ErrorEmptySessionToken is the error returned when the provided session token is invalid.
	ErrorEmptySessionToken = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-1004",
		Error: core.I18nMessage{
			Key:          "error.authnservice.empty_session_token",
			DefaultValue: "Empty session token",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.empty_session_token_description",
			DefaultValue: "The provided session token is empty",
		},
	}
	// ErrorEmptyAuthCode is the error returned when the provided authorization code is empty.
	ErrorEmptyAuthCode = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-1005",
		Error: core.I18nMessage{
			Key:          "error.authnservice.empty_auth_code",
			DefaultValue: "Empty authorization code",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.empty_auth_code_description",
			DefaultValue: "The provided authorization code is empty",
		},
	}
	// ErrorInvalidSessionToken is the error returned when the provided session token is invalid.
	ErrorInvalidSessionToken = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-1006",
		Error: core.I18nMessage{
			Key:          "error.authnservice.invalid_session_token",
			DefaultValue: "Invalid session token",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.invalid_session_token_description",
			DefaultValue: "The provided session token is invalid or has expired",
		},
	}
	// ErrorSubClaimNotFound is the error returned when the 'sub' claim is not found in the ID token.
	ErrorSubClaimNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-1007",
		Error: core.I18nMessage{
			Key:          "error.authnservice.sub_claim_not_found",
			DefaultValue: "user subject not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.sub_claim_not_found_description",
			DefaultValue: "The 'sub' claim is not found in the ID token claims",
		},
	}
	// ErrorUserNotFound is the error when no user is found with the provided attributes.
	ErrorUserNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-1008",
		Error: core.I18nMessage{
			Key:          "error.authnservice.user_not_found",
			DefaultValue: "User not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.user_not_found_description",
			DefaultValue: "No user found with the provided attributes",
		},
	}
	// ErrorInvalidAssertion is the error returned when the provided assertion token is invalid.
	ErrorInvalidAssertion = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-1009",
		Error: core.I18nMessage{
			Key:          "error.authnservice.invalid_assertion",
			DefaultValue: "Invalid assertion",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.invalid_assertion_description",
			DefaultValue: "The provided assertion token is invalid",
		},
	}
	// ErrorAssertionSubjectMismatch is the error returned when the assertion subject doesn't match
	// the authenticated user.
	ErrorAssertionSubjectMismatch = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-1010",
		Error: core.I18nMessage{
			Key:          "error.authnservice.assertion_subject_mismatch",
			DefaultValue: "Assertion subject mismatch",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.assertion_subject_mismatch_description",
			DefaultValue: "The subject in the assertion does not match the authenticated user",
		},
	}
	// ErrorAmbiguousUser is the error when multiple users match the provided attributes.
	ErrorAmbiguousUser = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-1011",
		Error: core.I18nMessage{
			Key:          "error.authnservice.ambiguous_user",
			DefaultValue: "Ambiguous user",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.ambiguous_user_description",
			DefaultValue: "Multiple users match the provided attributes",
		},
	}
)
