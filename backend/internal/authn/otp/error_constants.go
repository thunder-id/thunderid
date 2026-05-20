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

package otp

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for OTP authentication service
var (
	// ErrorInvalidSenderID is the error returned when the provided sender ID is invalid.
	ErrorInvalidSenderID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-OTP-1001",
		Error: core.I18nMessage{
			Key:          "error.authnotpservice.invalid_sender_id",
			DefaultValue: "Invalid sender ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnotpservice.invalid_sender_id_description",
			DefaultValue: "The provided sender ID is invalid or empty",
		},
	}
	// ErrorInvalidRecipient is the error returned when the provided recipient is invalid.
	ErrorInvalidRecipient = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-OTP-1002",
		Error: core.I18nMessage{
			Key:          "error.authnotpservice.invalid_recipient",
			DefaultValue: "Invalid recipient",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnotpservice.invalid_recipient_description",
			DefaultValue: "The provided recipient is invalid or empty",
		},
	}
	// ErrorUnsupportedChannel is the error returned when the provided channel is not supported.
	ErrorUnsupportedChannel = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-OTP-1003",
		Error: core.I18nMessage{
			Key:          "error.authnotpservice.unsupported_channel",
			DefaultValue: "Unsupported channel",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnotpservice.unsupported_channel_description",
			DefaultValue: "The provided channel is not supported for OTP authentication",
		},
	}
	// ErrorInvalidSessionToken is the error returned when the provided session token is invalid.
	ErrorInvalidSessionToken = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-OTP-1004",
		Error: core.I18nMessage{
			Key:          "error.authnotpservice.invalid_session_token",
			DefaultValue: "Invalid session token",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnotpservice.invalid_session_token_description",
			DefaultValue: "The provided session token is invalid or empty",
		},
	}
	// ErrorInvalidOTP is the error returned when the provided OTP is invalid.
	ErrorInvalidOTP = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-OTP-1005",
		Error: core.I18nMessage{
			Key:          "error.authnotpservice.invalid_otp",
			DefaultValue: "Invalid OTP",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnotpservice.invalid_otp_description",
			DefaultValue: "The provided OTP is invalid or empty",
		},
	}
	// ErrorIncorrectOTP is the error returned when the provided OTP is incorrect or has expired.
	ErrorIncorrectOTP = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-OTP-1006",
		Error: core.I18nMessage{
			Key:          "error.authnotpservice.incorrect_otp",
			DefaultValue: "Incorrect OTP",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnotpservice.incorrect_otp_description",
			DefaultValue: "The provided OTP is incorrect or has expired",
		},
	}
	// ErrorClientErrorFromOTPService is the error returned when there is a client error from the OTP service.
	ErrorClientErrorFromOTPService = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-OTP-1007",
		Error: core.I18nMessage{
			Key:          "error.authnotpservice.error_processing_otp",
			DefaultValue: "Error processing OTP",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnotpservice.error_processing_otp_description",
			DefaultValue: "An error occurred while processing the OTP request",
		},
	}
	// ErrorClientErrorWhileResolvingUser is the error returned when there is a client error while resolving the user.
	ErrorClientErrorWhileResolvingUser = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-OTP-1008",
		Error: core.I18nMessage{
			Key:          "error.authnotpservice.error_resolving_user",
			DefaultValue: "Error resolving user",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnotpservice.error_resolving_user_description",
			DefaultValue: "An error occurred while resolving the user for the recipient",
		},
	}
)
