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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for credentials authentication.
var (
	// ErrorEmptyAttributesOrCredentials is the error when the provided user attributes or credentials are empty.
	ErrorEmptyAttributesOrCredentials = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-CRED-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnservice.empty_attributes_or_credentials",
			DefaultValue: "Empty attributes or credentials",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnservice.empty_attributes_or_credentials_description",
			DefaultValue: "The user attributes or credentials cannot be empty",
		},
	}
	// ErrorInvalidCredentials is the error when the provided credentials are invalid.
	ErrorInvalidCredentials = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-CRED-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnservice.invalid_credentials",
			DefaultValue: "Invalid credentials",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnservice.invalid_credentials_description",
			DefaultValue: "The provided credentials are invalid",
		},
	}
	// ErrorClientErrorFromUserSvcAuthentication is the error when there is a client error from
	// the user service during authentication.
	ErrorClientErrorFromUserSvcAuthentication = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-CRED-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnservice.authentication_failed",
			DefaultValue: "authentication failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnservice.authentication_failed_description",
			DefaultValue: "An error occurred while authenticating the user",
		},
	}
	// ErrorInvalidToken is the error when the provided token is invalid.
	ErrorInvalidToken = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-CRED-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnservice.invalid_token",
			DefaultValue: "Invalid token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnservice.invalid_token_description",
			DefaultValue: "The provided token is invalid",
		},
	}
	// ErrorOTPAuthenticationFailed is the error when the OTP authentication attempt fails.
	ErrorOTPAuthenticationFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-OTPAUTHN-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnservice.otp_authentication_failed",
			DefaultValue: "Authentication failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnservice.otp_authentication_failed_description",
			DefaultValue: "The OTP authentication attempt failed",
		},
	}
	// ErrorPasskeyAuthenticationFailed is the error when the passkey authentication attempt fails.
	ErrorPasskeyAuthenticationFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-PSK-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnservice.passkey_authentication_failed",
			DefaultValue: "Authentication failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnservice.passkey_authentication_failed_description",
			DefaultValue: "The passkey authentication attempt failed",
		},
	}
	// ErrorPasskeyEnrollmentFailed is the error when the passkey enrollment attempt fails.
	ErrorPasskeyEnrollmentFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-PSK-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnservice.passkey_enrollment_failed",
			DefaultValue: "Enrollment failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnservice.passkey_enrollment_failed_description",
			DefaultValue: "The passkey enrollment attempt failed",
		},
	}
	// ErrorFederatedAuthenticationFailed is the error when federated authentication fails.
	ErrorFederatedAuthenticationFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHN-FED-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.authnservice.federated_authentication_failed",
			DefaultValue: "Federated authentication failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authnservice.federated_authentication_failed_description",
			DefaultValue: "The federated authentication attempt failed",
		},
	}
)
