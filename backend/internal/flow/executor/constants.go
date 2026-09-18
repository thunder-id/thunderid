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
	ExecutorNameCriteriaRevocationRestamp    = "CriteriaRevocationRestampExecutor"
	ExecutorNameSessionRevocation            = "SessionRevocationExecutor"
	ExecutorNameUserDelete                   = "UserDeleteExecutor"
	ExecutorNameApplicationActionValidator   = "ApplicationActionValidator"
	ExecutorNameApplicationDelete            = "ApplicationDeleteExecutor"
	ExecutorNameClientSecret                 = "ClientSecretExecutor"
	ExecutorNamePreRoleAssignmentRemoval     = "PreRoleAssignmentRemovalExecutor"
	ExecutorNameRoleAssignmentRemoval        = "RoleAssignmentRemovalExecutor"
	ExecutorNamePreRoleDeletion              = "PreRoleDeletionExecutor"
	ExecutorNameRoleDeletion                 = "RoleDeletionExecutor"
	ExecutorNamePreRolePermissionRemoval     = "PreRolePermissionRemovalExecutor"
	ExecutorNameRolePermissionRemoval        = "RolePermissionRemovalExecutor"
	ExecutorNamePreGroupDeletion             = "PreGroupDeletionExecutor"
	ExecutorNameGroupDeletion                = "GroupDeletionExecutor"
	ExecutorNamePreGroupMembershipRemoval    = "PreGroupMembershipRemovalExecutor"
	ExecutorNameGroupMembershipRemoval       = "GroupMembershipRemovalExecutor"
	ExecutorNamePreScopeDeletion             = "PreScopeDeletionExecutor"
	ExecutorNameScopeDeletion                = "ScopeDeletionExecutor"
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
	// revocationInputRole and revocationInputAssignee name the role and the principal a role
	// administration flow acts on. They follow revocationInputApplication in not reusing a name the
	// flow execution request already carries at the top level.
	revocationInputRole     = "targetRoleId"
	revocationInputAssignee = "targetAssigneeId"
	// revocationInputGroup and revocationInputMember name the group and the departing principal a group
	// administration flow acts on, following the same rule.
	revocationInputGroup  = "targetGroupId"
	revocationInputMember = "targetMemberId"
	// revocationInputPermissions carries a role's new permission set, as the JSON array of
	// {resourceServerId, permissions} objects the roles API itself accepts. It is an input rather than a
	// value the plan derives, because only the acting node needs it: the revocation covers every scope
	// the role grants today, not the delta this set implies.
	revocationInputPermissions = "targetPermissions"
	// revocationInputResourceServer, revocationInputResource and revocationInputAction locate the action
	// whose scope a scope deletion flow retires. The resource is optional: an action may be defined
	// directly on the resource server rather than under one of its resources.
	revocationInputResourceServer = "targetResourceServerId"
	revocationInputResource       = "targetResourceId"
	revocationInputAction         = "targetActionId"

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
