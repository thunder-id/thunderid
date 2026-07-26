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

package executor

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var (
	// ErrUserNotFound is returned when the user is not found in the system.
	ErrUserNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1001",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_not_found",
			DefaultValue: "User not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_not_found_desc",
			DefaultValue: "The user could not be found in the system",
		},
	}

	// ErrFailedToIdentifyUser is returned when the user cannot be identified.
	ErrFailedToIdentifyUser = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1002",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.failed_to_identify_user",
			DefaultValue: "Failed to identify user",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.failed_to_identify_user_desc",
			DefaultValue: "Unable to identify the user with the provided information",
		},
	}

	// ErrAmbiguousUserIdentity is returned when the user identity is ambiguous.
	ErrAmbiguousUserIdentity = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1003",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ambiguous_user_identity",
			DefaultValue: "Ambiguous user identity",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ambiguous_user_identity_desc",
			DefaultValue: "User identity is ambiguous and cannot be determined",
		},
	}

	// ErrUserNotAuthenticated is returned when the user is not authenticated.
	ErrUserNotAuthenticated = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1004",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_not_authenticated",
			DefaultValue: "User is not authenticated",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_not_authenticated_desc",
			DefaultValue: "The user has not been authenticated in this flow",
		},
	}

	// ErrInvalidCredentials is returned when the provided credentials are invalid.
	ErrInvalidCredentials = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1005",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_credentials",
			DefaultValue: "Invalid credentials provided",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_credentials_desc",
			DefaultValue: "The credentials provided are invalid",
		},
	}

	// ErrUserAuthFailed is returned when user authentication fails.
	ErrUserAuthFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1006",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_auth_failed",
			DefaultValue: "User authentication failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_auth_failed_desc",
			DefaultValue: "An error occurred while authenticating the user",
		},
	}

	// ErrUserAlreadyExists is returned when the user already exists in the system.
	ErrUserAlreadyExists = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1007",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_already_exists",
			DefaultValue: "User already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_already_exists_desc",
			DefaultValue: "A user already exists with the provided attributes",
		},
	}

	// ErrInvalidOTP is returned when the provided OTP is invalid.
	ErrInvalidOTP = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1008",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_otp",
			DefaultValue: "Invalid OTP provided",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_otp_desc",
			DefaultValue: "The one-time password provided is invalid or has expired",
		},
	}

	// ErrMaxOTPAttemptsReached is returned when the maximum OTP attempts are reached.
	ErrMaxOTPAttemptsReached = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1009",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.max_otp_attempts_reached",
			DefaultValue: "Maximum OTP attempts reached",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.max_otp_attempts_reached_desc",
			DefaultValue: "The maximum number of OTP verification attempts has been reached",
		},
	}

	// ErrInvalidMagicLinkToken is returned when the magic link token is invalid.
	ErrInvalidMagicLinkToken = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1010",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_magic_link_token",
			DefaultValue: "Invalid magic link token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_magic_link_token_desc",
			DefaultValue: "The magic link token is invalid or has expired",
		},
	}

	// ErrMagicLinkGeneration is returned when generating a magic link fails due to a client error.
	ErrMagicLinkGeneration = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1011",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.magic_link_generation_failed",
			DefaultValue: "Magic link generation failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.magic_link_generation_failed_desc",
			DefaultValue: "Failed to generate the magic link",
		},
	}

	// ErrInvalidOAuthState is returned when the OAuth state parameter is invalid.
	ErrInvalidOAuthState = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1012",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_oauth_state",
			DefaultValue: "Invalid OAuth state parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_oauth_state_desc",
			DefaultValue: "The OAuth state parameter is invalid or does not match the expected value",
		},
	}

	// ErrInvalidOAuthCode is returned when the OAuth authorization code is invalid.
	ErrInvalidOAuthCode = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1013",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_oauth_code",
			DefaultValue: "Invalid OAuth authorization code",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_oauth_code_desc",
			DefaultValue: "The OAuth authorization code is invalid or could not be exchanged for tokens",
		},
	}

	// ErrInvalidFederatedUser is returned when the federated user information is invalid during authentication.
	ErrInvalidFederatedUser = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1014",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_federated_user",
			DefaultValue: "Invalid federated user",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_federated_user_desc",
			DefaultValue: "The federated user information is invalid or inconsistent",
		},
	}

	// ErrInvalidPasskey is returned when the passkey credentials are invalid.
	ErrInvalidPasskey = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1015",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_passkey",
			DefaultValue: "Invalid passkey credentials",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_passkey_desc",
			DefaultValue: "The passkey credentials provided are invalid",
		},
	}

	// ErrNoRegisteredPasskeys is returned when no registered passkeys are found for the user.
	ErrNoRegisteredPasskeys = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1016",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.no_registered_passkeys",
			DefaultValue: "No registered passkeys found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.no_registered_passkeys_desc",
			DefaultValue: "No registered passkeys were found for the user",
		},
	}

	// ErrUserIDRequiredForPasskeyReg is returned when user ID is missing for passkey registration.
	ErrUserIDRequiredForPasskeyReg = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1017",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_id_required_for_passkey_reg",
			DefaultValue: "User ID missing for passkey registration",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_id_required_for_passkey_reg_desc",
			DefaultValue: "A user ID is required to register a passkey",
		},
	}

	// ErrPasskeyRegistrationFailed is returned when passkey registration fails.
	ErrPasskeyRegistrationFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1018",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.passkey_registration_failed",
			DefaultValue: "Passkey registration failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.passkey_registration_failed_desc",
			DefaultValue: "An error occurred while registering the passkey",
		},
	}

	// ErrPasskeyAuthFailed is returned when passkey authentication fails.
	ErrPasskeyAuthFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1019",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.passkey_auth_failed",
			DefaultValue: "Passkey authentication failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.passkey_auth_failed_desc",
			DefaultValue: "An error occurred while authenticating with the passkey",
		},
	}

	// ErrProvisioningUserAttrsMissing is returned when no user attributes are provided for provisioning.
	ErrProvisioningUserAttrsMissing = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1020",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.provisioning_user_attrs_missing",
			DefaultValue: "No user attributes provided for provisioning",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.provisioning_user_attrs_missing_desc",
			DefaultValue: "User attributes are required to provision a new user",
		},
	}

	// ErrProvisioningFailed is returned when user provisioning fails.
	ErrProvisioningFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1021",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.provisioning_failed",
			DefaultValue: "User provisioning failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.provisioning_failed_desc",
			DefaultValue: "An error occurred while provisioning the user",
		},
	}

	// ErrProvisioningAssignmentFailed is returned when group or role assignment fails during provisioning.
	ErrProvisioningAssignmentFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1022",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.provisioning_assignment_failed",
			DefaultValue: "Failed to assign groups and roles",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.provisioning_assignment_failed_desc",
			DefaultValue: "An error occurred while assigning groups and roles to the provisioned user",
		},
	}

	// ErrCrossOUProvisioningTargetMissing is returned when target OU is missing for cross-OU provisioning.
	ErrCrossOUProvisioningTargetMissing = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1023",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.cross_ou_provisioning_target_missing",
			DefaultValue: "Target OU is not set for cross-OU provisioning",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.cross_ou_provisioning_target_missing_desc",
			DefaultValue: "A target organization unit must be specified for cross-OU user provisioning",
		},
	}

	// ErrUserAlreadyExistsInTargetOU is returned when the user already exists in the target organization.
	ErrUserAlreadyExistsInTargetOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1024",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_already_exists_in_target_ou",
			DefaultValue: "User already exists in the target organization",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_already_exists_in_target_ou_desc",
			DefaultValue: "A user with the same identity already exists in the target organization unit",
		},
	}

	// ErrCannotProvisionAutomatically is returned when the user cannot be provisioned automatically.
	ErrCannotProvisionAutomatically = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1025",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.cannot_provision_automatically",
			DefaultValue: "Cannot provision user automatically",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.cannot_provision_automatically_desc",
			DefaultValue: "The user cannot be provisioned automatically with the provided information",
		},
	}

	// ErrSelfRegistrationDisabled is returned when self-registration is not enabled.
	ErrSelfRegistrationDisabled = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1026",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.self_registration_disabled",
			DefaultValue: "Self-registration not enabled",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.self_registration_disabled_desc",
			DefaultValue: "Self-registration is not enabled for this application or user type",
		},
	}

	// ErrInsufficientPermissions is returned when the user lacks required permissions.
	ErrInsufficientPermissions = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1027",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.insufficient_permissions",
			DefaultValue: "Insufficient permissions",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.insufficient_permissions_desc",
			DefaultValue: "The user does not have sufficient permissions to perform this action",
		},
	}

	// ErrAuthorizationFailed is returned when authorization validation fails.
	ErrAuthorizationFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1028",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.authorization_failed",
			DefaultValue: "Authorization validation failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.authorization_failed_desc",
			DefaultValue: "Authorization validation failed for the current user",
		},
	}

	// ErrInvalidOU is returned when the selected organization unit is invalid.
	ErrInvalidOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1029",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_ou",
			DefaultValue: "Selected organization unit is invalid",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_ou_desc",
			DefaultValue: "The selected organization unit is not valid for this operation",
		},
	}

	// ErrOUNotFound is returned when the organization unit is not found.
	ErrOUNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1030",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_not_found",
			DefaultValue: "Organization unit not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_not_found_desc",
			DefaultValue: "The selected organization unit does not exist",
		},
	}

	// ErrOUNameConflict is returned when an organization unit with the same name already exists.
	ErrOUNameConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1031",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_name_conflict",
			DefaultValue: "Organization unit with the same name already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_name_conflict_desc",
			DefaultValue: "An organization unit with the same name already exists in this context",
		},
	}

	// ErrOUHandleConflict is returned when an organization unit with the same handle already exists.
	ErrOUHandleConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1032",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_handle_conflict",
			DefaultValue: "Organization unit with the same handle already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_handle_conflict_desc",
			DefaultValue: "An organization unit with the same handle already exists in this context",
		},
	}

	// ErrOUCreationPrereqFailed is returned when prerequisites validation fails for OU creation.
	ErrOUCreationPrereqFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1033",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_creation_prereq_failed",
			DefaultValue: "Prerequisites validation failed for OU creation",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_creation_prereq_failed_desc",
			DefaultValue: "The prerequisites for creating an organization unit have not been met",
		},
	}

	// ErrOUResolutionFailed is returned when the organization unit cannot be resolved.
	ErrOUResolutionFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1034",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_resolution_failed",
			DefaultValue: "Failed to resolve organization unit",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_resolution_failed_desc",
			DefaultValue: "Unable to resolve the organization unit for the current context",
		},
	}

	// ErrOUCreationFailed is returned when organization unit creation fails.
	ErrOUCreationFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1035",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_creation_failed",
			DefaultValue: "Organization unit creation failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_creation_failed_desc",
			DefaultValue: "An error occurred while creating the organization unit",
		},
	}

	// ErrEmailSendFailed is returned when the email fails to send.
	ErrEmailSendFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1036",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.email_send_failed",
			DefaultValue: "Failed to send email",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.email_send_failed_desc",
			DefaultValue: "An error occurred while sending the email",
		},
	}

	// ErrEmailRecipientMissing is returned when the email recipient is not provided.
	ErrEmailRecipientMissing = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1037",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.email_recipient_missing",
			DefaultValue: "Email recipient is required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.email_recipient_missing_desc",
			DefaultValue: "An email recipient must be provided to send the notification",
		},
	}

	// ErrEmailServiceNotConfigured is returned when the email service is not configured.
	ErrEmailServiceNotConfigured = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1038",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.email_service_not_configured",
			DefaultValue: "Email service is not configured",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.email_service_not_configured_desc",
			DefaultValue: "The email notification service has not been configured",
		},
	}

	// ErrSMSRecipientMissing is returned when the SMS recipient is not provided.
	ErrSMSRecipientMissing = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1039",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.sms_recipient_missing",
			DefaultValue: "SMS recipient is required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.sms_recipient_missing_desc",
			DefaultValue: "An SMS recipient must be provided to send the notification",
		},
	}

	// ErrSMSInvalidPhone is returned when the SMS recipient phone number is invalid.
	ErrSMSInvalidPhone = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1040",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.sms_invalid_phone",
			DefaultValue: "SMS recipient is not a valid phone number",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.sms_invalid_phone_desc",
			DefaultValue: "The provided SMS recipient is not a valid phone number",
		},
	}

	// ErrSMSTemplateMissing is returned when the SMS template is not provided.
	ErrSMSTemplateMissing = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1041",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.sms_template_missing",
			DefaultValue: "SMS template is required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.sms_template_missing_desc",
			DefaultValue: "An SMS template must be provided to send the notification",
		},
	}

	// ErrSMSProviderNotConfigured is returned when the SMS provider is not configured.
	ErrSMSProviderNotConfigured = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1042",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.sms_provider_not_configured",
			DefaultValue: "SMS notification provider is not configured",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.sms_provider_not_configured_desc",
			DefaultValue: "The SMS notification provider has not been configured or is erroneous",
		},
	}

	// ErrPrerequisitesFailed is returned when prerequisites validation fails.
	ErrPrerequisitesFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1043",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.prerequisites_failed",
			DefaultValue: "Prerequisites validation failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.prerequisites_failed_desc",
			DefaultValue: "The prerequisites for this operation have not been met",
		},
	}

	// ErrUserIDMissingInContext is returned when the user ID is not found in the flow context.
	ErrUserIDMissingInContext = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1044",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_id_missing_in_context",
			DefaultValue: "User ID not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_id_missing_in_context_desc",
			DefaultValue: "The user ID could not be resolved from the current flow context",
		},
	}

	// ErrCredentialInputMissing is returned when no credential input is configured.
	ErrCredentialInputMissing = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1045",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.credential_input_missing",
			DefaultValue: "No credential input configured",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.credential_input_missing_desc",
			DefaultValue: "No credential input has been configured for the credential setter",
		},
	}

	// ErrCredentialInputInvalid is returned when the credential input configuration is invalid.
	ErrCredentialInputInvalid = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1046",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.credential_input_invalid",
			DefaultValue: "Invalid credential input configuration",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.credential_input_invalid_desc",
			DefaultValue: "The credential input configuration is invalid for the credential setter",
		},
	}

	// ErrCredentialValueEmpty is returned when the credential value is empty.
	ErrCredentialValueEmpty = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1047",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.credential_value_empty",
			DefaultValue: "Credential value is empty",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.credential_value_empty_desc",
			DefaultValue: "The credential value must not be empty for the credential setter",
		},
	}

	// ErrCredentialSetFailed is returned when setting credentials fails.
	ErrCredentialSetFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1048",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.credential_set_failed",
			DefaultValue: "Failed to set credentials",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.credential_set_failed_desc",
			DefaultValue: "An error occurred while setting the user credentials",
		},
	}

	// ErrAttributeCollectFailed is returned when updating user attributes fails.
	ErrAttributeCollectFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1049",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.attribute_collect_failed",
			DefaultValue: "Failed to update user attributes",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.attribute_collect_failed_desc",
			DefaultValue: "An error occurred while updating the user attributes",
		},
	}

	// ErrHTTPRequestConfigInvalid is returned when the HTTP request executor configuration is invalid.
	ErrHTTPRequestConfigInvalid = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1050",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.http_request_config_invalid",
			DefaultValue: "Configuration error",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.http_request_config_invalid_desc",
			DefaultValue: "The HTTP request executor configuration is invalid",
		},
	}

	// ErrAuthNotAvailableForApp is returned when authentication is not available for the application.
	ErrAuthNotAvailableForApp = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1051",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.auth_not_available_for_app",
			DefaultValue: "Authentication not available for this application",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.auth_not_available_for_app_desc",
			DefaultValue: "The requested authentication method is not available for this application",
		},
	}

	// ErrSelfRegNotAvailableForApp is returned when self-registration is not available for the application.
	ErrSelfRegNotAvailableForApp = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1052",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.self_reg_not_available_for_app",
			DefaultValue: "Self-registration not available for this application",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.self_reg_not_available_for_app_desc",
			DefaultValue: "Self-registration is not available for this application",
		},
	}

	// ErrNoValidUserTypes is returned when no valid user types are available for the flow.
	ErrNoValidUserTypes = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1053",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.no_valid_user_types",
			DefaultValue: "No valid user types available",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.no_valid_user_types_desc",
			DefaultValue: "There are no valid user types configured for this flow",
		},
	}

	// ErrUserTypeNotAllowed is returned when the user type is not allowed for the flow.
	ErrUserTypeNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1054",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_type_not_allowed",
			DefaultValue: "User type not allowed for this flow",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_type_not_allowed_desc",
			DefaultValue: "The selected user type is not allowed for this flow",
		},
	}

	// ErrInvalidUserType is returned when the user type is invalid.
	ErrInvalidUserType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1055",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_user_type",
			DefaultValue: "Invalid user type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_user_type_desc",
			DefaultValue: "The provided user type is not valid",
		},
	}

	// ErrNoUserTypesAvailable is returned when no user types are available.
	ErrNoUserTypesAvailable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1056",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.no_user_types_available",
			DefaultValue: "No user types available",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.no_user_types_available_desc",
			DefaultValue: "No user types are currently available",
		},
	}

	// ErrUserTypeRetrievalFailed is returned when user type retrieval fails.
	ErrUserTypeRetrievalFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1057",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_type_retrieval_failed",
			DefaultValue: "Failed to retrieve user types",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_type_retrieval_failed_desc",
			DefaultValue: "An error occurred while retrieving available user types",
		},
	}

	// ErrUserTypeNotValidForOU is returned when the user type is not valid for the selected OU.
	ErrUserTypeNotValidForOU = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1058",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_type_not_valid_for_ou",
			DefaultValue: "User type is not valid",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.user_type_not_valid_for_ou_desc",
			DefaultValue: "The selected user type is not valid for the chosen organization unit",
		},
	}

	// ErrSelfRegDisabledForUserType is returned when self-registration is disabled for a specific user type.
	ErrSelfRegDisabledForUserType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1059",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.self_reg_disabled_for_user_type",
			DefaultValue: "Self-registration is disabled for the user type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.self_reg_disabled_for_user_type_desc",
			DefaultValue: "Self-registration is not enabled for the selected user type",
		},
	}

	// ErrAttributeNotFoundForUser is returned when a required attribute is not found for the user.
	ErrAttributeNotFoundForUser = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1060",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.attribute_not_found_for_user",
			DefaultValue: "Required attribute not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.attribute_not_found_for_user_desc",
			DefaultValue: "A required attribute was not found for the user",
		},
	}

	// ErrAttributeNotUnique is returned when an attribute value already exists.
	ErrAttributeNotUnique = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1061",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.attribute_not_unique",
			DefaultValue: "User already exists with the provided {{param(attribute)}}",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "flows.executor.errors.attribute_not_unique_desc",
			DefaultValue: "The provided {{param(attribute)}} is already associated with another user" +
				" and expects a unique value",
		},
	}

	// ErrAttributeRetrievalFailed is returned when user attribute retrieval fails.
	ErrAttributeRetrievalFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1062",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.attribute_retrieval_failed",
			DefaultValue: "Failed to retrieve user attributes",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.attribute_retrieval_failed_desc",
			DefaultValue: "An error occurred while retrieving user attributes",
		},
	}

	// ErrCredentialProcessingFailed is returned when credential processing fails.
	ErrCredentialProcessingFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1063",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.credential_processing_failed",
			DefaultValue: "Failed to process credentials",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.credential_processing_failed_desc",
			DefaultValue: "An error occurred while processing the credentials",
		},
	}

	// ErrInvalidAction is returned when an invalid action is provided to a prompt node.
	ErrInvalidAction = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1064",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_action",
			DefaultValue: "Invalid action provided",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_action_desc",
			DefaultValue: "The action provided is not valid for the current flow step",
		},
	}

	// ErrConsentPrereqFailed is returned when prerequisites validation fails for consent.
	ErrConsentPrereqFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1065",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_prereq_failed",
			DefaultValue: "Prerequisites validation failed for consent",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_prereq_failed_desc",
			DefaultValue: "The prerequisites for the consent executor have not been met",
		},
	}

	// ErrConsentDenied is returned when the user denies consent.
	ErrConsentDenied = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1066",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_denied",
			DefaultValue: "User denied consent",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_denied_desc",
			DefaultValue: "The user has denied the required consent",
		},
	}

	// ErrConsentDecisionsMissing is returned when consent decisions input is missing.
	ErrConsentDecisionsMissing = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1067",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_decisions_missing",
			DefaultValue: "Consent decisions input is missing or empty",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_decisions_missing_desc",
			DefaultValue: "The consent decisions input is missing or empty",
		},
	}

	// ErrConsentDecisionsParseFail is returned when consent decisions cannot be parsed.
	ErrConsentDecisionsParseFail = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1068",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_decisions_parse",
			DefaultValue: "Failed to parse consent decisions",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_decisions_parse_desc",
			DefaultValue: "The consent decisions input could not be parsed",
		},
	}

	// ErrConsentPromptTimedOut is returned when the consent prompt times out.
	ErrConsentPromptTimedOut = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1069",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_prompt_timed_out",
			DefaultValue: "Consent prompt has timed out",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_prompt_timed_out_desc",
			DefaultValue: "The consent prompt has timed out without a response",
		},
	}

	// ErrConsentRecordFailed is returned when recording consent fails.
	ErrConsentRecordFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1070",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_record_failed",
			DefaultValue: "Failed to record consent",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_record_failed_desc",
			DefaultValue: "An error occurred while recording the user consent",
		},
	}

	// ErrConsentResolutionFailed is returned when resolving consent fails.
	ErrConsentResolutionFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1071",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_resolution_failed",
			DefaultValue: "Failed to resolve consent",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.consent_resolution_failed_desc",
			DefaultValue: "An error occurred while resolving the user consent requirements",
		},
	}

	// ErrHTTPRequestFailed is returned when the HTTP request executor fails.
	ErrHTTPRequestFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1072",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.http_request_failed",
			DefaultValue: "HTTP request executor failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.http_request_failed_desc",
			DefaultValue: "The HTTP request executor failed to complete the request",
		},
	}

	// ErrInvalidInviteToken is returned when the invite token is invalid.
	ErrInvalidInviteToken = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1073",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_invite_token",
			DefaultValue: "Invalid invite token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invalid_invite_token_desc",
			DefaultValue: "The provided invite token is invalid or has expired",
		},
	}

	// ErrInviteTokenGenerationFailed is returned when generating an invite token fails.
	ErrInviteTokenGenerationFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1074",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invite_token_generation_failed",
			DefaultValue: "Failed to generate invite token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.invite_token_generation_failed_desc",
			DefaultValue: "An error occurred while generating the invite token",
		},
	}

	// ErrOUNotValidForUserType is returned when the selected OU is not valid for the chosen user type.
	ErrOUNotValidForUserType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1075",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_not_valid_for_user_type",
			DefaultValue: "The selected organization unit is not valid for the chosen user type.",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.ou_not_valid_for_user_type_desc",
			DefaultValue: "The selected organization unit is not valid for the chosen user type",
		},
	}

	// ErrOpenID4VPDefinitionNotConfigured is returned when the OpenID4VP node has no presentation_definition_id.
	ErrOpenID4VPDefinitionNotConfigured = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "FET-1076",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.openid4vp_definition_not_configured",
			DefaultValue: "OpenID4VP presentation definition is not configured",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.openid4vp_definition_not_configured_desc",
			DefaultValue: "The OpenID4VP node is missing the presentation_definition_id property",
		},
	}

	// ErrOpenID4VPInitiateFailed is returned when initiating the OpenID4VP request fails.
	ErrOpenID4VPInitiateFailed = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "FET-1077",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.openid4vp_initiate_failed",
			DefaultValue: "Failed to initiate the OpenID4VP request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.openid4vp_initiate_failed_desc",
			DefaultValue: "An error occurred while initiating the OpenID4VP presentation request",
		},
	}

	// ErrOpenID4VPExpired is returned when the OpenID4VP request expires before a response is received.
	ErrOpenID4VPExpired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1078",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.openid4vp_expired",
			DefaultValue: "The OpenID4VP request expired before a response was received",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.openid4vp_expired_desc",
			DefaultValue: "The OpenID4VP presentation request expired before the wallet submitted a response",
		},
	}

	// ErrOpenID4VPVerificationFailed is returned when the OpenID4VP presentation verification fails.
	ErrOpenID4VPVerificationFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1079",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.openid4vp_verification_failed",
			DefaultValue: "OpenID4VP presentation verification failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.openid4vp_verification_failed_desc",
			DefaultValue: "The OpenID4VP presentation verification failed",
		},
	}

	// ErrProvisioningAttributeConflict is returned when user provisioning fails due to an attribute conflict.
	ErrProvisioningAttributeConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1080",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.provisioning_attribute_conflict",
			DefaultValue: "A user with the provided attributes already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.provisioning_attribute_conflict_desc",
			DefaultValue: "User provisioning failed because one or more unique attribute values are already taken",
		},
	}

	// ErrNoLiveSSOSession is returned by the SSO-Check node when no live, compatible session is
	// available for the current flow. It is not a hard failure: it routes the node's "Unavailable"
	// (onFailure) outcome to the full-authentication path.
	ErrNoLiveSSOSession = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1081",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.no_live_sso_session",
			DefaultValue: "No live SSO session",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.no_live_sso_session_desc",
			DefaultValue: "No live, compatible SSO session exists for this flow; full authentication is required",
		},
	}

	// ErrInteractionRequired is returned when the assurance accumulated in this execution does
	// not satisfy the request's acr_values / max_age, so user interaction (step-up or
	// re-authentication) is required before an assertion can be issued. Its code maps to the
	// OAuth2 `interaction_required` error.
	// TODO(sso): wire this to an OAuth2 `interaction_required` authorize-error redirect and
	// drive step-up re-authentication.
	ErrInteractionRequired = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "FET-1082",
		Error: tidcommon.I18nMessage{
			Key:          "flows.executor.errors.interaction_required",
			DefaultValue: "Interaction required",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "flows.executor.errors.interaction_required_desc",
			DefaultValue: "The accumulated authentication assurance does not satisfy the requested " +
				"acr_values or max_age",
		},
	}
)

// errAttributeNotUniqueFor returns a ServiceError for a specific attribute that is not unique.
func errAttributeNotUniqueFor(attrName string) *tidcommon.ServiceError {
	params := map[string]string{"attribute": attrName}
	e := ErrAttributeNotUnique
	e.Error.Params = params
	e.ErrorDescription.Params = params
	return &e
}
