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
	ExecutorNameApplicationActionValidator   = "ApplicationActionValidator"
	ExecutorNameApplicationDelete            = "ApplicationDeleteExecutor"
	ExecutorNameClientSecret                 = "ClientSecretExecutor"
	ExecutorNameOwnerResolver                = "OwnerResolver"
	ExecutorNameAgentTypeResolver            = "AgentTypeResolver"
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

// defaultProvisioningCategory is the entity default category a provisioning node provisions into when its
// mode property is not set.
const defaultProvisioningCategory = "user"

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

	userInputOuName            = "ouName"
	userInputOuHandle          = "ouHandle"
	userInputOuDesc            = "ouDescription"
	userInputInviteToken       = "inviteToken"
	userInputOTP               = "otp"
	userInputMagicLinkToken    = "token"
	userInputConsentDecisions  = "consent_decisions"
	userInputLoginHint         = "login_hint"
	revocationInputSubject     = "subject"
	revocationInputApplication = "targetApplicationId"

	ouIDKey        = "ouId"
	defaultOUIDKey = "defaultOUID"
	userTypeKey    = "userType"
	agentTypeKey   = "agentType"

	// The agent record's own fields, collected by the flow rather than declared by the entity type
	// schema. The agent service owns their validation.
	ownerKey       = "owner"
	nameKey        = "name"
	descriptionKey = "description"
	logoURLKey     = "logoUrl"
	// delegatedKey marks an agent that acts on behalf of a signed-in user. Absent means false.
	delegatedKey = "delegated"
	// redirectURIsKey carries the callback URIs of a delegated agent, comma separated. It is the
	// only part of an agent's OAuth configuration a flow supplies; the provider derives the rest.
	redirectURIsKey = "redirectUris"

	// agentAttributeConflictCode is the agent service's unique-attribute clash code. It is repeated
	// here rather than referenced from internal/agent: that package reaches the flow executor through
	// the inbound client and flow management services, so importing it would close an import cycle.
	agentAttributeConflictCode = "AGT-1014"

	dataValueTrue  = "true"
	dataValueFalse = "false"

	entityStateNotExists = "not_exists"
	entityStateExists    = "exists"
	entityStateAmbiguous = "ambiguous"
)

// Executor property keys
const (
	propertyKeyAssignGroup = "assignGroup"
	propertyKeyAssignRole  = "assignRole"
	// propertyKeyProvisioningMode names the entity category a provisioning node provisions into.
	// The value is the category name; absent or empty means the default category.
	propertyKeyProvisioningMode = "mode"
	propertyKeyRequiredScopes   = "requiredScopes"
	propertyKeyEmailTemplate    = "emailTemplate"
	// TODO: Revisit propertyKeyTokenExpiry and propertyKeyMagicLinkURL — these should not be node properties.
	propertyKeyTokenExpiry                             = "tokenExpiry"
	propertyKeyMagicLinkURL                            = "magicLinkURL"
	propertyKeySMSTemplate                             = "smsTemplate"
	propertyKeyAllowedUserTypes                        = "allowedUserTypes"
	propertyKeyAllowedAgentTypes                       = "allowedAgentTypes"
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
