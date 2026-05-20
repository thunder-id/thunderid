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

package oauth

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for OAuth authentication.
var (
	// ErrorEmptyIdpID is the error when the IDP identifier is empty.
	ErrorEmptyIdpID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTH-OAUTH-1001",
		Error: core.I18nMessage{
			Key:          "error.authoauthservice.empty_idp_id",
			DefaultValue: "IDP id is empty",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authoauthservice.empty_idp_id_description",
			DefaultValue: "The identity provider id cannot be empty",
		},
	}
	// ErrorInvalidIDP is the error when the retrieved IDP is invalid.
	ErrorInvalidIDP = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTH-OAUTH-1002",
		Error: core.I18nMessage{
			Key:          "error.authoauthservice.invalid_idp",
			DefaultValue: "Invalid identity provider",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authoauthservice.invalid_idp_description",
			DefaultValue: "The retrieved identity provider is invalid or empty",
		},
	}
	// ErrorEmptyAuthorizationCode is the error when the authorization code is empty.
	ErrorEmptyAuthorizationCode = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTH-OAUTH-1003",
		Error: core.I18nMessage{
			Key:          "error.authoauthservice.empty_authorization_code",
			DefaultValue: "Empty authorization code",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authoauthservice.empty_authorization_code_description",
			DefaultValue: "The authorization code cannot be empty",
		},
	}
	// ErrorEmptyAccessToken is the error when the access token is empty.
	ErrorEmptyAccessToken = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTH-OAUTH-1004",
		Error: core.I18nMessage{
			Key:          "error.authoauthservice.empty_access_token",
			DefaultValue: "Empty access token",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authoauthservice.empty_access_token_description",
			DefaultValue: "The access token cannot be empty",
		},
	}
	// ErrorClientErrorWhileRetrievingIDP is the error when there is a client error while retrieving the IDP.
	ErrorClientErrorWhileRetrievingIDP = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTH-OAUTH-1005",
		Error: core.I18nMessage{
			Key:          "error.authoauthservice.failed_to_retrieve_idp",
			DefaultValue: "Failed to retrieve identity provider",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authoauthservice.failed_to_retrieve_idp_description",
			DefaultValue: "A client error occurred while retrieving the identity provider configuration",
		},
	}
	// ErrorEmptySubClaim is the error when the sub claim is empty.
	ErrorEmptySubClaim = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTH-OAUTH-1006",
		Error: core.I18nMessage{
			Key:          "error.authoauthservice.empty_sub_claim",
			DefaultValue: "Empty sub claim",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authoauthservice.empty_sub_claim_description",
			DefaultValue: "The sub claim cannot be empty",
		},
	}
	// ErrorClientErrorWhileRetrievingUser is the error when there is a client error while retrieving the user.
	ErrorClientErrorWhileRetrievingUser = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTH-OAUTH-1007",
		Error: core.I18nMessage{
			Key:          "error.authoauthservice.failed_to_retrieve_user",
			DefaultValue: "Failed to retrieve user",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authoauthservice.failed_to_retrieve_user_description",
			DefaultValue: "A client error occurred while retrieving the internal user",
		},
	}
	// ErrorInvalidTokenResponse is the error when the token response is invalid.
	ErrorInvalidTokenResponse = serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "AUTH-OAUTH-1008",
		Error: core.I18nMessage{
			Key:          "error.authoauthservice.invalid_token_response",
			DefaultValue: "Invalid token response",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authoauthservice.invalid_token_response_description",
			DefaultValue: "The token response received from the identity provider is invalid",
		},
	}
)
