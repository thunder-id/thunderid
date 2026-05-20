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

package authn

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for credentials authentication.
var (
	// ErrorEmptyAttributesOrCredentials is the error when the provided user attributes or credentials are empty.
	ErrorEmptyAttributesOrCredentials = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTH-CRED-1001",
		Error: core.I18nMessage{
			Key:          "error.authnservice.empty_attributes_or_credentials",
			DefaultValue: "Empty attributes or credentials",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.empty_attributes_or_credentials_description",
			DefaultValue: "The user attributes or credentials cannot be empty",
		},
	}
	// ErrorInvalidCredentials is the error when the provided credentials are invalid.
	ErrorInvalidCredentials = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTH-CRED-1002",
		Error: core.I18nMessage{
			Key:          "error.authnservice.invalid_credentials",
			DefaultValue: "Invalid credentials",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.invalid_credentials_description",
			DefaultValue: "The provided credentials are invalid",
		},
	}
	// ErrorClientErrorFromUserSvcAuthentication is the error when there is a client error from
	// the user service during authentication.
	ErrorClientErrorFromUserSvcAuthentication = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTH-CRED-1003",
		Error: core.I18nMessage{
			Key:          "error.authnservice.authentication_failed",
			DefaultValue: "authentication failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.authentication_failed_description",
			DefaultValue: "An error occurred while authenticating the user",
		},
	}
	// ErrorInvalidToken is the error when the provided token is invalid.
	ErrorInvalidToken = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTH-CRED-1004",
		Error: core.I18nMessage{
			Key:          "error.authnservice.invalid_token",
			DefaultValue: "Invalid token",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.invalid_token_description",
			DefaultValue: "The provided token is invalid",
		},
	}
	// ErrorOTPAuthenticationFailed is the error when the OTP authentication attempt fails.
	ErrorOTPAuthenticationFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-OTPAUTHN-1009",
		Error: core.I18nMessage{
			Key:          "error.authnservice.otp_authentication_failed",
			DefaultValue: "Authentication failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.otp_authentication_failed_description",
			DefaultValue: "The OTP authentication attempt failed",
		},
	}
	// ErrorPasskeyAuthenticationFailed is the error when the passkey authentication attempt fails.
	ErrorPasskeyAuthenticationFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-PSK-1001",
		Error: core.I18nMessage{
			Key:          "error.authnservice.passkey_authentication_failed",
			DefaultValue: "Authentication failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.passkey_authentication_failed_description",
			DefaultValue: "The passkey authentication attempt failed",
		},
	}
	// ErrorFederatedAuthenticationFailed is the error when federated authentication fails.
	ErrorFederatedAuthenticationFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "AUTHN-FED-1001",
		Error: core.I18nMessage{
			Key:          "error.authnservice.federated_authentication_failed",
			DefaultValue: "Federated authentication failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.authnservice.federated_authentication_failed_description",
			DefaultValue: "The federated authentication attempt failed",
		},
	}
)
