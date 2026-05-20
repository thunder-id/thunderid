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

package userinfo

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// UserInfo standard service error constants
var (
	// errorInvalidAccessToken is returned when the access token is invalid, expired, or malformed
	errorInvalidAccessToken = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "invalid_token",
		Error: core.I18nMessage{
			Key:          "error.userinfoservice.invalid_access_token",
			DefaultValue: "Invalid access token",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userinfoservice.invalid_access_token_description",
			DefaultValue: "The access token is invalid, expired, or malformed",
		},
	}

	// errorMissingSubClaim is returned when the access token is missing or has an invalid 'sub' claim
	errorMissingSubClaim = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "invalid_token",
		Error: core.I18nMessage{
			Key:          "error.userinfoservice.missing_sub_claim",
			DefaultValue: "Invalid access token",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userinfoservice.missing_sub_claim_description",
			DefaultValue: "The access token is missing or has an invalid 'sub' claim",
		},
	}

	// errorClientCredentialsNotSupported is returned when the access token was issued using client_credentials grant
	errorClientCredentialsNotSupported = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "invalid_token",
		Error: core.I18nMessage{
			Key:          "error.userinfoservice.client_credentials_not_supported",
			DefaultValue: "Invalid access token",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userinfoservice.client_credentials_not_supported_description",
			DefaultValue: "UserInfo endpoint is not applicable for client_credentials grant type",
		},
	}

	// errorInsufficientScope is returned when the access token lacks the required 'openid' scope
	errorInsufficientScope = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "insufficient_scope",
		Error: core.I18nMessage{
			Key:          "error.userinfoservice.insufficient_scope",
			DefaultValue: "Insufficient scope",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.userinfoservice.insufficient_scope_description",
			DefaultValue: "The 'openid' scope is required for this request",
		},
	}
)
