// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/executormeta"
)

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
	ExecutorNameApplicationActionValidator   = executormeta.ExecutorNameApplicationActionValidator
	ExecutorNameApplicationDelete            = executormeta.ExecutorNameApplicationDelete
	ExecutorNameClientSecret                 = executormeta.ExecutorNameClientSecret
	ExecutorNameOwnerResolver                = executormeta.ExecutorNameOwnerResolver
	ExecutorNameAgentTypeResolver            = executormeta.ExecutorNameAgentTypeResolver
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

// defaultProvisioningCategory is the entity default category a provisioning node provisions into when its
// mode property is not set.
const defaultProvisioningCategory = entitytype.TypeCategoryUser

// categoryUnscoped is the category an executor names when its work is not tied to one kind of
// entity. The errors raised for it read for the user category, which is what those paths serve
// today. An executor that gains a second category resolves one and names it instead of this.
const categoryUnscoped = entitytype.TypeCategoryUser

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
	ouHandleKey    = "ouHandle"
	defaultOUIDKey = "defaultOUID"

	// Input identifiers a flow definition declares for the entity type, one per category. A prompt
	// node names the same string, so these are wire names rather than internal ones.
	userTypeKey  = "userType"
	agentTypeKey = "agentType"

	// categoryTypeKey is the runtime slot carrying the resolved type name. A run provisions one
	// category, so the resolvers share a single slot. Readers pair the name with the category they
	// already hold from the node's mode property, which is what every type lookup needs.
	categoryTypeKey = "categoryType"

	// The agent record's own fields, collected by the flow rather than declared by the entity type
	// schema. The agent service owns their validation.
	ownerKey = "owner"
	// ownerIDKey is the runtime slot carrying the owner the resolver verified. It differs from the
	// input identifier so a submitted value cannot stand in for a resolved one.
	ownerIDKey     = "ownerId"
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
	propertyKeyAssignGroup           = "assignGroup"
	propertyKeyAssignRole            = "assignRole"
	propertyKeySeedGroupsFromMapping = "seedGroupsFromMapping"
	propertyKeySeedRolesFromMapping  = "seedRolesFromMapping"
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
