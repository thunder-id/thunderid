// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package executormeta holds the runtime-free static description of the flow executors: their
// names, their modes, and the metadata catalog used to validate a flow definition. It imports no
// executor constructor, so a build that only authors and validates configuration can depend on it
// without linking the runtime services the executors need (authn, oauth, notification, session,
// and the rest).
package executormeta

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
