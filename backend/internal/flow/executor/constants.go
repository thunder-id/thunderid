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

package executor

// Executor name constants
const (
	ExecutorNameBasicAuth     = "BasicAuthExecutor"
	ExecutorNameSMSAuth       = "SMSOTPAuthExecutor"
	ExecutorNameMagicLinkAuth = "MagicLinkAuthExecutor"
	// nolint:gosec // G101: This is an executor name, not a credential
	ExecutorNamePasskeyAuth                  = "PasskeyAuthExecutor"
	ExecutorNameOAuth                        = "OAuthExecutor"
	ExecutorNameOIDCAuth                     = "OIDCAuthExecutor"
	ExecutorNameGitHubAuth                   = "GithubOAuthExecutor"
	ExecutorNameGoogleAuth                   = "GoogleOIDCAuthExecutor"
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
)

// Executor mode constants
const (
	ExecutorModeSend     = "send"
	ExecutorModeGenerate = "generate"
	ExecutorModeVerify   = "verify"
	ExecutorModeIdentify = "identify"
	ExecutorModeResolve  = "resolve"
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
	userInputNonce = "nonce"
	userInputState = "state"

	userInputOuName           = "ouName"
	userInputOuHandle         = "ouHandle"
	userInputOuDesc           = "ouDescription"
	userInputInviteToken      = "inviteToken"
	userInputOTP              = "otp"
	userInputMagicLinkToken   = "token"
	userInputConsentDecisions = "consent_decisions"

	ouIDKey        = "ouId"
	defaultOUIDKey = "defaultOUID"
	userTypeKey    = "userType"

	dataValueTrue  = "true"
	dataValueFalse = "false"
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
)

// nonSearchableInputs contains the list of user inputs/ attributes that are non-searchable.
var nonSearchableInputs = []string{"password", "code", "nonce", "otp", "token", "userInputMagicLinkToken"}

// Failure reason constants
const (
	failureReasonUserNotAuthenticated = "User is not authenticated"
	failureReasonUserNotFound         = "User not found"
	failureReasonInvalidCredentials   = "Invalid credentials provided" // #nosec G101
	failureReasonFailedToIdentifyUser = "Failed to identify user"
	failureReasonAmbiguousUser        = "User identity is ambiguous"
	failureReasonInvalidOTP           = "invalid OTP provided"
	failureReasonInvalidMagicLink     = "Invalid magic link token"
)
