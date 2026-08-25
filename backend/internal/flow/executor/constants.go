// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

// Executor name constants
const (
	ExecutorNameCredentialsAuth = "CredentialsAuthExecutor"
	ExecutorNameMagicLink       = "MagicLinkExecutor"
	// nolint:gosec // G101: This is an executor name, not a credential
	ExecutorNamePasskeyAuth                  = "PasskeyAuthExecutor"
	ExecutorNameOAuth                        = "OAuthExecutor"
	ExecutorNameOIDCAuth                     = "OIDCAuthExecutor"
	ExecutorNameGitHubAuth                   = "GithubOAuthExecutor"
	ExecutorNameGoogleAuth                   = "GoogleOIDCAuthExecutor"
	ExecutorNameOpenID4VPVerify              = "OpenID4VPVerifyExecutor"
	ExecutorNameIdentifying                  = "IdentifyingExecutor"
	ExecutorNameAuthAssert                   = "AuthAssertExecutor"
	ExecutorNameProvisioning                 = "ProvisioningExecutor"
	ExecutorNameAttributeCollect             = "AttributeCollector"
	ExecutorNameAuthorization                = "AuthorizationExecutor"
	ExecutorNamePermissionValidator          = "PermissionValidator"
	ExecutorNameOUCreation                   = "OUExecutor"
	ExecutorNameHTTPRequest                  = "HTTPRequestExecutor"
	ExecutorNameUserTypeResolver             = "UserTypeResolver"
	ExecutorNameInviteExecutor               = "InviteExecutor"
	ExecutorNameEmailExecutor                = "EmailExecutor"
	ExecutorNameCredentialSetter             = "CredentialSetter"
	ExecutorNameConsent                      = "ConsentExecutor"
	ExecutorNameOUResolver                   = "OUResolverExecutor"
	ExecutorNameAttributeUniquenessValidator = "AttributeUniquenessValidator"
	ExecutorNameSMSExecutor                  = "SMSExecutor"
	ExecutorNameFederatedAuthResolver        = "FederatedAuthResolverExecutor"
	ExecutorNameSSOCheck                     = "SSOCheckExecutor"
	ExecutorNameSession                      = "SessionExecutor"
	ExecutorNameSessionSignOut               = "SessionSignOutExecutor"
	ExecutorNameOTPExecutor                  = "OTPExecutor"
	ExecutorNamePreDelete                    = "PreDeleteExecutor"
	ExecutorNameCriteriaRevocation           = "CriteriaRevocationExecutor"
	ExecutorNameSessionRevocation            = "SessionRevocationExecutor"
	ExecutorNameUserDelete                   = "UserDeleteExecutor"
	ExecutorNameValidateApplicationDeletion  = "ValidateApplicationDeletionExecutor"
	ExecutorNameValidateSecretRegeneration   = "ValidateSecretRegenerationExecutor"
	ExecutorNameApplicationDelete            = "ApplicationDeleteExecutor"
	ExecutorNameClientSecret                 = "ClientSecretExecutor"
)

// Executor mode constants
const (
	ExecutorModeSend       = "send"
	ExecutorModeGenerate   = "generate"
	ExecutorModeVerify     = "verify"
	ExecutorModeIdentify   = "identify"
	ExecutorModeResolve    = "resolve"
	ExecutorModeCheckState = "check_state"
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
	// revocationInputApplication names the application an administrative flow acts on. It is deliberately
	// not "applicationId": the flow execution request already carries a top-level applicationId meaning
	// the application a flow runs for, and reusing that name one level down would invite supplying the
	// value in the wrong place, where the failure is a flow that pauses asking for input.
	revocationInputApplication = "targetApplicationId"
	// dataKeyClientSecret carries a regenerated client secret back to the caller. AdditionalData is the
	// only executor output the engine serializes, and this is the single moment the value is readable.
	dataKeyClientSecret = "clientSecret" // #nosec G101 -- response field name, not a secret

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
