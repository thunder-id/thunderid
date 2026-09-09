// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import "github.com/thunder-id/thunderid/internal/flow/executormeta"

// Executor name constants. The values live in executormeta, which carries no runtime dependency,
// so flow validation can name an executor without linking the executor itself. These aliases keep
// the existing executor.ExecutorNameX references working.
const (
	ExecutorNameCredentialsAuth = executormeta.ExecutorNameCredentialsAuth
	ExecutorNameMagicLink       = executormeta.ExecutorNameMagicLink
	// nolint:gosec // G101: This is an executor name, not a credential
	ExecutorNamePasskeyAuth                  = executormeta.ExecutorNamePasskeyAuth
	ExecutorNameOAuth                        = executormeta.ExecutorNameOAuth
	ExecutorNameOIDCAuth                     = executormeta.ExecutorNameOIDCAuth
	ExecutorNameGitHubAuth                   = executormeta.ExecutorNameGitHubAuth
	ExecutorNameGoogleAuth                   = executormeta.ExecutorNameGoogleAuth
	ExecutorNameOpenID4VPVerify              = executormeta.ExecutorNameOpenID4VPVerify
	ExecutorNameIdentifying                  = executormeta.ExecutorNameIdentifying
	ExecutorNameAuthAssert                   = executormeta.ExecutorNameAuthAssert
	ExecutorNameProvisioning                 = executormeta.ExecutorNameProvisioning
	ExecutorNameAttributeCollect             = executormeta.ExecutorNameAttributeCollect
	ExecutorNameAuthorization                = executormeta.ExecutorNameAuthorization
	ExecutorNamePermissionValidator          = executormeta.ExecutorNamePermissionValidator
	ExecutorNameOUCreation                   = executormeta.ExecutorNameOUCreation
	ExecutorNameHTTPRequest                  = executormeta.ExecutorNameHTTPRequest
	ExecutorNameUserTypeResolver             = executormeta.ExecutorNameUserTypeResolver
	ExecutorNameInviteExecutor               = executormeta.ExecutorNameInviteExecutor
	ExecutorNameEmailExecutor                = executormeta.ExecutorNameEmailExecutor
	ExecutorNameCredentialSetter             = executormeta.ExecutorNameCredentialSetter
	ExecutorNameConsent                      = executormeta.ExecutorNameConsent
	ExecutorNameOUResolver                   = executormeta.ExecutorNameOUResolver
	ExecutorNameAttributeUniquenessValidator = executormeta.ExecutorNameAttributeUniquenessValidator
	ExecutorNameSMSExecutor                  = executormeta.ExecutorNameSMSExecutor
	ExecutorNameFederatedAuthResolver        = executormeta.ExecutorNameFederatedAuthResolver
	ExecutorNameSSOCheck                     = executormeta.ExecutorNameSSOCheck
	ExecutorNameSession                      = executormeta.ExecutorNameSession
	ExecutorNameSessionSignOut               = executormeta.ExecutorNameSessionSignOut
	ExecutorNameOTPExecutor                  = executormeta.ExecutorNameOTPExecutor
	ExecutorNamePreDelete                    = executormeta.ExecutorNamePreDelete
	ExecutorNameCriteriaRevocation           = executormeta.ExecutorNameCriteriaRevocation
	ExecutorNameSessionRevocation            = executormeta.ExecutorNameSessionRevocation
	ExecutorNameUserDelete                   = executormeta.ExecutorNameUserDelete
)

// Executor mode constants
const (
	ExecutorModeSend       = executormeta.ExecutorModeSend
	ExecutorModeGenerate   = executormeta.ExecutorModeGenerate
	ExecutorModeVerify     = executormeta.ExecutorModeVerify
	ExecutorModeIdentify   = executormeta.ExecutorModeIdentify
	ExecutorModeResolve    = executormeta.ExecutorModeResolve
	ExecutorModeCheckState = executormeta.ExecutorModeCheckState
)

// User attribute and input constants
const (
	userAttributeUsername = "username"
	userAttributePassword = "password"
	userAttributeUserID   = "userID"
	userAttributeEmail    = "email"
	userAttributeGroups   = "groups"
	userAttributeSub      = "sub"

	userInputCode  = "code"
	userInputState = "state"

	userInputOuName           = "ouName"
	userInputOuHandle         = "ouHandle"
	userInputOuDesc           = "ouDescription"
	userInputInviteToken      = "inviteToken"
	userInputOTP              = "otp"
	userInputMagicLinkToken   = "token"
	userInputConsentDecisions = "consent_decisions"
	userInputLoginHint        = "login_hint"
	revocationInputSubject    = "subject"

	ouIDKey        = "ouId"
	defaultOUIDKey = "defaultOUID"
	userTypeKey    = "userType"

	dataValueTrue  = "true"
	dataValueFalse = "false"

	entityStateNotExists = "not_exists"
	entityStateExists    = "exists"
	entityStateAmbiguous = "ambiguous"
)

// Executor property keys
const (
	propertyKeyAssignGroup    = "assignGroup"
	propertyKeyAssignRole     = "assignRole"
	propertyKeyRequiredScopes = "requiredScopes"
	propertyKeyEmailTemplate  = "emailTemplate"
	// TODO: Revisit propertyKeyTokenExpiry and propertyKeyMagicLinkURL — these should not be node properties.
	propertyKeyTokenExpiry                             = "tokenExpiry"
	propertyKeyMagicLinkURL                            = "magicLinkURL"
	propertyKeySMSTemplate                             = "smsTemplate"
	propertyKeyAllowedUserTypes                        = "allowedUserTypes"
	propertyKeyNotificationSenderID                    = "senderId"
	propertyKeyDynamicInputsIncludeOptional            = "includeOptional"
	propertyKeyDynamicInputsIncludeOptionalCredentials = "includeOptionalCredentials"
	propertyKeyMaxDynamicInputsPerPrompt               = "maxPerPrompt"
	propertyKeyInviteBaseURL                           = "inviteBaseURL"
	propertyKeyPresentationDefinitionID                = "presentation_definition_id"
	propertyKeyCallbackType                            = "callbackType"
	propertyKeyLoginHintAttribute                      = "loginHintAttribute"
	propertyKeyMaxOTPAttempts                          = "maxAttempts"
	propertyKeyOTPLength                               = "otpLength"
	propertyKeyOTPUseNumericOnly                       = "otpUseNumericOnly"
	propertyKeyOTPValidityPeriodSeconds                = "otpValidityPeriodSeconds"
	// propertyKeyPromptOnSignOut, when set to boolean true on a session sign-out node, makes the executor
	// confirm the logout with the End-User (via the node's onIncomplete prompt) whenever the RP-initiated
	// logout was not accompanied by a valid id_token_hint (RuntimeKeyLogoutPromptRequired).
	propertyKeyPromptOnSignOut = "promptOnSignOut"
	// propertyKeyConsentFailOnDeny, when set to boolean true on a consent node, makes the executor
	// fail the flow if the user did not approve the consent prompt, either by pressing
	// the Deny button or by letting the prompt time out. This applies even when every prompted
	// element is optional.
	propertyKeyConsentFailOnDeny = "failOnDeny"
)

// nonSearchableInputs contains the list of user inputs/ attributes that are non-searchable.
var nonSearchableInputs = []string{
	"password", "code", "otp", "token", "userInputMagicLinkToken", "otpSessionToken",
}
