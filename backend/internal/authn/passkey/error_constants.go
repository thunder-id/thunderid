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

package passkey

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for passkey authentication service

var (
	// ErrorEmptyUserIdentifier is returned when both userID and username are empty.
	ErrorEmptyUserIdentifier = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1001",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.empty_user_identifier",
			DefaultValue: "Empty user identifier",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.empty_user_identifier_description",
			DefaultValue: "Either user ID or username must be provided",
		},
	}
	// ErrorEmptyRelyingPartyID is returned when the relying party ID is empty.
	ErrorEmptyRelyingPartyID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1002",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.empty_relying_party_id",
			DefaultValue: "Empty relying party ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.empty_relying_party_id_description",
			DefaultValue: "The relying party ID is required",
		},
	}
	// ErrorEmptyCredentialID is returned when the credential ID is empty.
	ErrorEmptyCredentialID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1003",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.empty_credential_id",
			DefaultValue: "Empty credential ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.empty_credential_id_description",
			DefaultValue: "The credential ID is required",
		},
	}
	// ErrorEmptyCredentialType is returned when the credential type is empty.
	ErrorEmptyCredentialType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1004",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.empty_credential_type",
			DefaultValue: "Empty credential type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.empty_credential_type_description",
			DefaultValue: "The credential type is required",
		},
	}
	// ErrorInvalidAuthenticatorResponse is returned when the authenticator response is invalid.
	ErrorInvalidAuthenticatorResponse = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1005",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.invalid_authenticator_response",
			DefaultValue: "Invalid authenticator response",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.passkeyservice.invalid_authenticator_response_description",
			DefaultValue: "The authenticator response is missing required fields " +
				"(clientDataJSON, authenticatorData, or signature)",
		},
	}
	// ErrorEmptySessionToken is returned when the session token is empty.
	ErrorEmptySessionToken = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1006",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.empty_session_token",
			DefaultValue: "Empty session token",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.empty_session_token_description",
			DefaultValue: "The session token is required",
		},
	}
	// ErrorInvalidFinishData is returned when the finish data is nil.
	ErrorInvalidFinishData = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1007",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.invalid_finish_data",
			DefaultValue: "Invalid finish data",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.invalid_finish_data_description",
			DefaultValue: "The finish data cannot be null",
		},
	}
	// ErrorInvalidChallenge is returned when the challenge validation fails.
	ErrorInvalidChallenge = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1008",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.invalid_challenge",
			DefaultValue: "Invalid challenge",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.invalid_challenge_description",
			DefaultValue: "The challenge in the response does not match the expected challenge",
		},
	}
	// ErrorInvalidSignature is returned when signature verification fails.
	ErrorInvalidSignature = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1009",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.invalid_signature",
			DefaultValue: "Invalid signature",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.invalid_signature_description",
			DefaultValue: "The signature verification failed",
		},
	}
	// ErrorCredentialNotFound is returned when the credential is not found.
	ErrorCredentialNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1010",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.credential_not_found",
			DefaultValue: "Passkey credential not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.credential_not_found_description",
			DefaultValue: "The specified credential was not found for the user",
		},
	}
	// ErrorInvalidAttestationResponse is returned when the attestation response is invalid.
	ErrorInvalidAttestationResponse = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1011",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.invalid_attestation_response",
			DefaultValue: "Invalid attestation response",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.invalid_attestation_response_description",
			DefaultValue: "The attestation response is missing required fields (clientDataJSON or attestationObject)",
		},
	}
	// ErrorUserNotFound is returned when the user is not found.
	ErrorUserNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1012",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.user_not_found",
			DefaultValue: "User not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.user_not_found_description",
			DefaultValue: "The specified user was not found",
		},
	}
	// ErrorInvalidSessionToken is returned when the session token is invalid.
	ErrorInvalidSessionToken = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1013",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.invalid_session_token",
			DefaultValue: "Invalid session token",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.invalid_session_token_description",
			DefaultValue: "The session token is invalid or malformed",
		},
	}
	// ErrorSessionExpired is returned when the session has expired.
	ErrorSessionExpired = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1014",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.session_expired",
			DefaultValue: "Session expired",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.session_expired_description",
			DefaultValue: "The session has expired. Please start a new session",
		},
	}
	// ErrorNoCredentialsFound is returned when no credentials are found for the user.
	ErrorNoCredentialsFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "PSK-1015",
		Error: core.I18nMessage{
			Key:          "error.passkeyservice.no_credentials_found",
			DefaultValue: "No credentials found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.passkeyservice.no_credentials_found_description",
			DefaultValue: "No credentials found for the user. Please register a credential first",
		},
	}
)
