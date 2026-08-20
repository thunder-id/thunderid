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

// Package executormeta holds the runtime-free static metadata for flow executors: their names,
// modes, and the metadata catalog used for flow definition validation. It deliberately imports no
// runtime executor constructors, so control-plane builds can validate flow definitions without
// linking the data-plane executor dependencies (authn, oauth, notification, session, and so on).
package executormeta

// Executor name constants.
const (
	ExecutorNameCriteriaRevocation = "CriteriaRevocationExecutor"
	ExecutorNameSessionRevocation  = "SessionRevocationExecutor"
	ExecutorNamePreDelete          = "PreDeleteExecutor"
	ExecutorNameUserDelete         = "UserDeleteExecutor"
	ExecutorNameCredentialsAuth    = "CredentialsAuthExecutor"
	ExecutorNameMagicLink          = "MagicLinkExecutor"
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
)

// Executor mode constants.
const (
	ExecutorModeSend       = "send"
	ExecutorModeGenerate   = "generate"
	ExecutorModeVerify     = "verify"
	ExecutorModeIdentify   = "identify"
	ExecutorModeResolve    = "resolve"
	ExecutorModeCheckState = "check_state"
)

// Passkey executor mode constants. These mirror the unexported passkey executor modes in the
// runtime executor package and are duplicated here so the static catalog stays runtime-free.
const (
	passkeyExecutorModeChallenge = "challenge"
	passkeyExecutorModeVerify    = "verify"
	passkeyExecutorModeRegStart  = "register_start"
	passkeyExecutorModeRegFinish = "register_finish"
)

// Executor property key constants used by the metadata catalog. These mirror the unexported
// property keys in the runtime executor package.
const (
	propertyKeyCallbackType                            = "callbackType"
	propertyKeyEmailTemplate                           = "emailTemplate"
	propertyKeyInviteBaseURL                           = "inviteBaseURL"
	propertyKeyTokenExpiry                             = "tokenExpiry"
	propertyKeyMagicLinkURL                            = "magicLinkURL"
	propertyKeyMaxOTPAttempts                          = "maxAttempts"
	propertyKeyLoginHintAttribute                      = "loginHintAttribute"
	propertyKeyNotificationSenderID                    = "senderId"
	propertyKeySMSTemplate                             = "smsTemplate"
	propertyKeyDynamicInputsIncludeOptional            = "includeOptional"
	propertyKeyDynamicInputsIncludeOptionalCredentials = "includeOptionalCredentials"
	propertyKeyMaxDynamicInputsPerPrompt               = "maxPerPrompt"
	propertyKeyAssignGroup                             = "assignGroup"
	propertyKeyAssignRole                              = "assignRole"
	propertyKeyRequiredScopes                          = "requiredScopes"
	propertyKeyAllowedUserTypes                        = "allowedUserTypes"
	propertyKeyPresentationDefinitionID                = "presentation_definition_id"
)
