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

package magiclink

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for Magic Link authentication service.
var (
	// ErrorInvalidToken is the error returned when the provided magic link token is invalid.
	ErrorInvalidToken = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-ML-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.magiclinkservice.invalid_token",
			DefaultValue: "Invalid token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.magiclinkservice.invalid_token_description",
			DefaultValue: "The provided magic link token is invalid",
		},
	}
	// ErrorExpiredToken is the error returned when the magic link token has expired.
	ErrorExpiredToken = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-ML-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.magiclinkservice.expired_token",
			DefaultValue: "Expired token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.magiclinkservice.expired_token_description",
			DefaultValue: "The magic link token has expired",
		},
	}
	// ErrorMalformedTokenClaims is the error returned when the token claims are malformed.
	ErrorMalformedTokenClaims = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-ML-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.magiclinkservice.malformed_token_claims",
			DefaultValue: "Malformed token claims",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.magiclinkservice.malformed_token_claims_description",
			DefaultValue: "The magic link token contains invalid or missing claims",
		},
	}
	// ErrorClientErrorWhileResolvingUser is the error returned when there is a client error while resolving the user.
	ErrorClientErrorWhileResolvingUser = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-ML-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.magiclinkservice.resolving_user",
			DefaultValue: "Error resolving user",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.magiclinkservice.resolving_user_description",
			DefaultValue: "An error occurred while resolving the user for the recipient",
		},
	}
	// ErrorTokenGenerationFailed is the error returned when JWT token generation fails.
	ErrorTokenGenerationFailed = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "AUTHN-ML-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.magiclinkservice.token_generation_failed",
			DefaultValue: "Token generation failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.magiclinkservice.token_generation_failed_description",
			DefaultValue: "Failed to generate magic link token",
		},
	}
)
