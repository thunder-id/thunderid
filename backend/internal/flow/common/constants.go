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

// Package common defines common constants and models used across the flow components.
package common

import "time"

// FlowType defines the type of flow execution.
type FlowType string

const (
	// FlowTypeAuthentication represents a flow execution for user authentication.
	FlowTypeAuthentication FlowType = "AUTHENTICATION"
	// FlowTypeRegistration represents a flow execution for user registration.
	FlowTypeRegistration FlowType = "REGISTRATION"
	// FlowTypeUserOnboarding represents an admin-initiated user onboarding flow.
	FlowTypeUserOnboarding FlowType = "USER_ONBOARDING"
	// FlowTypeRecovery represents a flow execution for account recovery (e.g., password reset).
	FlowTypeRecovery FlowType = "RECOVERY"
)

// FlowStatus defines the status of a flow execution.
type FlowStatus string

const (
	// FlowStatusComplete indicates that the flow execution is complete.
	FlowStatusComplete FlowStatus = "COMPLETE"
	// FlowStatusIncomplete indicates that the flow execution is incomplete.
	FlowStatusIncomplete FlowStatus = "INCOMPLETE"
	// FlowStatusError indicates that there was an error during the flow execution.
	FlowStatusError FlowStatus = "ERROR"
)

// FlowStepType defines the type of a step in the flow execution.
type FlowStepType string

const (
	// StepTypeView represents a step in the flow that requires user interaction.
	StepTypeView FlowStepType = "VIEW"
	// StepTypeRedirection represents a step in the flow that redirects the user to another URL.
	StepTypeRedirection FlowStepType = "REDIRECTION"
)

// NodeType defines the node types in the flow execution.
type NodeType string

const (
	// NodeTypeStart represents the beginning of a flow (representation node)
	NodeTypeStart NodeType = "START"
	// NodeTypeEnd represents the end of a flow (representation node)
	NodeTypeEnd NodeType = "END"
	// NodeTypeTaskExecution represents a task execution node
	NodeTypeTaskExecution NodeType = "TASK_EXECUTION"
	// NodeTypePrompt represents a prompt node
	NodeTypePrompt NodeType = "PROMPT"
)

// NodeStatus defines the status of a node in the flow execution.
type NodeStatus string

const (
	// NodeStatusComplete indicates that the node has completed its execution successfully.
	NodeStatusComplete NodeStatus = "COMPLETE"
	// NodeStatusIncomplete indicates that the node has not completed its execution.
	NodeStatusIncomplete NodeStatus = "INCOMPLETE"
	// NodeStatusFailure indicates that the node has failed during its execution.
	NodeStatusFailure NodeStatus = "FAILURE"
	// NodeStatusForward indicates that the engine should forward execution to NextNodeID.
	// Used for scenarios like onFailure handlers where context should be preserved.
	NodeStatusForward NodeStatus = "FORWARD"
)

// NodeResponseType defines the type of response from a node in the flow execution.
type NodeResponseType string

const (
	// NodeResponseTypeView indicates that the node response is a view type, requiring user interaction.
	NodeResponseTypeView NodeResponseType = "VIEW"
	// NodeResponseTypeRedirection indicates that the node response is a redirection type, redirecting to another URL.
	NodeResponseTypeRedirection NodeResponseType = "REDIRECTION"
	// NodeResponseTypeRetry indicates that the node response is a retry type, indicating a retry action.
	NodeResponseTypeRetry NodeResponseType = "RETRY"
)

// ExecutorStatus defines the status of an executor in the flow execution.
type ExecutorStatus string

const (
	// ExecComplete indicates that the executor has completed its execution successfully.
	ExecComplete ExecutorStatus = "COMPLETE"
	// ExecUserInputRequired indicates that the executor requires user input to proceed.
	ExecUserInputRequired ExecutorStatus = "USER_INPUT_REQUIRED"
	// ExecExternalRedirection indicates that the executor is redirecting to an external URL.
	ExecExternalRedirection ExecutorStatus = "EXTERNAL_REDIRECTION"
	// ExecFailure indicates that the executor has failed during its execution.
	ExecFailure ExecutorStatus = "FAILURE"
	// ExecRetry indicates that the executor is retrying its execution.
	ExecRetry ExecutorStatus = "RETRY"
)

// ExecutorType defines the type of an executor in the flow execution.
type ExecutorType string

const (
	// ExecutorTypeAuthentication represents an executor that performs authentication.
	ExecutorTypeAuthentication ExecutorType = "AUTHENTICATION"
	// ExecutorTypeRegistration represents an executor that handles user registration/provisioning.
	ExecutorTypeRegistration ExecutorType = "REGISTRATION"
	// ExecutorTypeUtility represents a utility executor for common operations.
	ExecutorTypeUtility ExecutorType = "UTILITY"
)

const (
	// DataIDPName is the key used for the identity provider name in the flow response.
	DataIDPName = "idpName"
	// DataConsentPrompt is the key used for the consent prompt data in the flow response.
	DataConsentPrompt = "consentPrompt"
	// DataStepTimeout is the key used for the step expiry timestamp in the flow response.
	DataStepTimeout = "stepTimeout"
	// DataInviteLink is the key used for the invite link in the flow response additional data.
	DataInviteLink = "inviteLink"
	// DataEmailSent is the key used to indicate that an email was sent successfully in the flow response.
	DataEmailSent = "emailSent"
	// DataSMSSent is the key used to indicate that an SMS was sent successfully in the flow response.
	DataSMSSent = "smsSent"
	// DataRootOUID is the key used to pass the root OU ID to the frontend for the OU tree picker.
	DataRootOUID = "rootOuId"
	// DataPromptMessage is the key used to pass a message to be displayed in the prompt node.
	DataPromptMessage = "message"
	// DataOpenID4VPClientID is the verifier client_id for the wallet QR / deep link.
	DataOpenID4VPClientID = "openid4vpClientId"
	// DataOpenID4VPRequestURI is the signed request URI the wallet fetches.
	DataOpenID4VPRequestURI = "openid4vpRequestUri"
	// DataOpenID4VPWalletURI is the openid4vp:// authorization URI for the wallet.
	DataOpenID4VPWalletURI = "openid4vpWalletUri"
)

// DefaultHTTPTimeout defines the default timeout duration for HTTP requests.
const DefaultHTTPTimeout = 5 * time.Second

// NodeVariant identifies a PROMPT node sub-type that activates a variant-specific code path.
type NodeVariant string

const (
	// NodeVariantLoginOptions identifies a PROMPT node that presents login method choices to the user.
	NodeVariantLoginOptions NodeVariant = "LOGIN_OPTIONS"
)

const (
	// NodePropertyAllowAuthenticationWithoutLocalUser indicates whether authentication is allowed without a local user
	NodePropertyAllowAuthenticationWithoutLocalUser = "allowAuthenticationWithoutLocalUser"
	// NodePropertyAllowRegistrationWithExistingUser indicates whether registration is allowed with an existing user
	NodePropertyAllowRegistrationWithExistingUser = "allowRegistrationWithExistingUser"
	// NodePropertyAllowCrossOUProvisioning indicates whether an existing user should be provisioned to the
	// target OU when they accept an invite. Used together with allowRegistrationWithExistingUser. When true,
	// the user is created in the target OU; when false, provisioning is skipped entirely.
	NodePropertyAllowCrossOUProvisioning = "allowCrossOUProvisioning"
	// NodePropertyOUResolveFrom specifies the strategy for resolving the organization unit.
	// Supported values: "caller" (use the caller's OU).
	NodePropertyOUResolveFrom = "resolveFrom"
	// NodePropertyAuthMethodMapping maps authentication classes to action refs on login_options PROMPT nodes.
	NodePropertyAuthMethodMapping = "authMethodMapping"
)

const (
	// RuntimeKeyUserAutoProvisioned indicates whether the user was auto-provisioned
	RuntimeKeyUserAutoProvisioned = "userAutoProvisioned"
	// RuntimeKeyUserEligibleForProvisioning indicates whether the user is eligible for auto provisioning
	RuntimeKeyUserEligibleForProvisioning = "userEligibleForProvisioning"
	// RuntimeKeyUserAmbiguous indicates the user exists in multiple OUs and requires disambiguation
	RuntimeKeyUserAmbiguous = "userAmbiguous"
	// RuntimeKeyClientID holds the OAuth client ID for the current flow execution, if applicable.
	RuntimeKeyClientID = "clientId"
	// RuntimeKeyRequestedPermissions holds the space-separated permission scopes requested by the OAuth client.
	RuntimeKeyRequestedPermissions = "requested_permissions"
	// RuntimeKeyConsentedPermissions holds the space-separated permission scopes the user has consented to
	// release to the client, as produced by the ConsentExecutor.
	RuntimeKeyConsentedPermissions = "consented_permissions"
	// RuntimeKeyRequiredEssentialAttributes holds the space-separated essential user attributes required for the flow.
	RuntimeKeyRequiredEssentialAttributes = "required_essential_attributes"
	// RuntimeKeyRequiredOptionalAttributes holds the space-separated optional user attributes required for the flow.
	RuntimeKeyRequiredOptionalAttributes = "required_optional_attributes"
	// RuntimeKeyRequiredLocales holds the space-separated locales requested for claims.
	RuntimeKeyRequiredLocales = "required_locales"
	// RuntimeKeyConsentID holds the consent record ID after consent has been recorded.
	RuntimeKeyConsentID = "consent_id"
	// RuntimeKeyStepTimeout holds the expiry timestamp for the current flow step.
	RuntimeKeyStepTimeout = "step_timeout"
	// RuntimeKeyConsentedAttributes holds a space-separated set of attributes that the user has consented to share.
	RuntimeKeyConsentedAttributes = "consented_attributes"
	// RuntimeKeyConsentSessionToken holds the signed JWT session token for consent validation.
	RuntimeKeyConsentSessionToken = "consent_session_token"
	// RuntimeKeyStoredInviteToken holds the generated invite token stored during the invite send phase.
	RuntimeKeyStoredInviteToken = "storedInviteToken"
	// RuntimeKeyUserAttributesCacheTTLSeconds indicates the TTL of the user attributes cache.
	RuntimeKeyUserAttributesCacheTTLSeconds = "user_attributes_cache_ttl_seconds"
	// RuntimeKeyInviteLink holds the generated invite link for downstream executors (e.g., EmailExecutor).
	RuntimeKeyInviteLink = "inviteLink"
	// RuntimeKeyMagicLinkURL holds the generated magic link URL for downstream executors.
	RuntimeKeyMagicLinkURL = "magicLinkURL"
	// RuntimeKeyMagicLinkExpiryMinutes holds the expiry duration used by the magic-link email template.
	RuntimeKeyMagicLinkExpiryMinutes = "magicLinkExpiryMinutes"
	// RuntimeKeyMagicLinkDestinationAttribute holds the destination attribute used to generate the magic link.
	RuntimeKeyMagicLinkDestinationAttribute = "magicLinkDestinationAttribute"
	// RuntimeKeySkipDelivery indicates that delivery should be skipped for the current flow.
	RuntimeKeySkipDelivery = "skipDelivery"
	// RuntimeKeyCandidateUsers holds serialized candidate users during disambiguation in resolve mode.
	RuntimeKeyCandidateUsers = "candidateUsers"
	// RuntimeKeyPresentedOptionalInputs holds a space-separated list of optional input identifiers
	// that have already been prompted to the user, even if the user left them empty.
	RuntimeKeyPresentedOptionalInputs = "presentedOptionalInputs"
	// RuntimeKeySMSOTPMobileNumber holds the resolved mobile number for SMS OTP verification.
	// TODO: Revisit when the generic OTP executor is implemented.
	RuntimeKeySMSOTPMobileNumber = "smsOTPMobileNumber"
	// RuntimeKeySMSOTPPhoneAttr holds the schema attribute name used to look up the mobile number.
	// TODO: Revisit when the generic OTP executor is implemented.
	RuntimeKeySMSOTPPhoneAttr = "smsOTPPhoneAttr"
	// RuntimeKeyMagicLinkUsedJti is the JWT ID claim value of a magic link token that has already been used.
	RuntimeKeyMagicLinkUsedJti = "magicLinkUsedJti"
	// RuntimeKeyOAuthState holds the generated OAuth state parameter for CSRF validation.
	RuntimeKeyOAuthState = "oauthState"
	// RuntimeKeyOpenID4VPState holds the OpenID4VP request state across poll steps.
	RuntimeKeyOpenID4VPState = "openid4vpVerificationState"
	// RuntimeKeyRequestedAuthClasses holds the space-separated ACR values from acr_values.
	RuntimeKeyRequestedAuthClasses = "requested_auth_classes"
	// RuntimeKeySelectedAuthClass holds the ACR value of the chosen authentication method.
	RuntimeKeySelectedAuthClass = "selected_auth_class"
	// RuntimeKeyAllowedLoginOptions holds the space-separated action refs allowed on a LOGIN_OPTIONS node.
	RuntimeKeyAllowedLoginOptions = "allowed_login_options"
	// RuntimeKeyAllowRegistrationWithExistingUser indicates whether registration is allowed with an existing user
	RuntimeKeyAllowRegistrationWithExistingUser = "allowRegistrationWithExistingUser"
	// RuntimeKeyCIBAAuthReqID holds the CIBA auth_req_id bound to the current flow execution, if applicable.
	RuntimeKeyCIBAAuthReqID = "cibaAuthReqId"
	// RuntimeKeyBindingMessage holds the human-readable binding message displayed to the user
	// on both the consumption device and the authentication device to correlate the CIBA request.
	RuntimeKeyBindingMessage = "bindingMessage"
	// RuntimeKeyEntityState holds the entity existence state set by the IdentifyingExecutor in check_state mode.
	RuntimeKeyEntityState = "entityState"
)

// User input key constants for well-known keys used in UserInputs across flow executors.
const (
	// UserInputKeyLoginHint is the UserInputs key for the CIBA login_hint value.
	UserInputKeyLoginHint = "login_hint"
)

// TODO: Define a go type for InputType when formalizing input types

// InputType constants define known input types used in flow definitions.
const (
	// InputTypeText represents a text input type.
	InputTypeText = "TEXT_INPUT"
	// InputTypeEmail represents an email input type.
	InputTypeEmail = "EMAIL_INPUT"
	// InputTypePassword represents a password credential input type.
	InputTypePassword = "PASSWORD_INPUT"
	// InputTypeOTP represents a one-time password input type.
	InputTypeOTP = "OTP_INPUT"
	// InputTypePhone represents a phone number input type.
	InputTypePhone = "PHONE_INPUT"
	// InputTypeConsent represents a consent decisions input type.
	InputTypeConsent = "CONSENT_INPUT"
	// InputTypeHidden represents a hidden input type.
	InputTypeHidden = "HIDDEN"
	// InputTypeSelect represents a select (dropdown) input type.
	InputTypeSelect = "SELECT"

	// TODO: Add support for other sensitive input types:
	// - Passkey credential fields (credentialId, clientDataJSON, authenticatorData, signature, userHandle)
	// - OAuth/OIDC authorization codes
	// - OIDC nonce
	// - Invite tokens
)

// MetaComponentType constants define known component types used in flow meta definitions.
const (
	// MetaComponentTypeBlock represents a block container component.
	MetaComponentTypeBlock = "BLOCK"
	// MetaComponentTypeAction represents an action (button) component.
	MetaComponentTypeAction = "ACTION"
	// MetaComponentTypeDynamicInputPlaceholder marks the insertion point for dynamically
	// derived input components. The renderer replaces this component with the resolved inputs.
	MetaComponentTypeDynamicInputPlaceholder = "DYNAMIC_INPUT_PLACEHOLDER"
)

// Attribute name constants for well-known user attributes used across flow executors.
const (
	// AttributeMobileNumber is the default attribute name for a user's mobile phone number.
	AttributeMobileNumber = "mobileNumber"
)

// sensitiveInputTypes contains the list of input types that are considered sensitive.
var sensitiveInputTypes = []string{
	InputTypePassword,
	InputTypeOTP,
}

const (
	// AttributeEmail is the default attribute name for a user's email.
	AttributeEmail = "email"
)

// ActionType represents the type of action in a prompt.
type ActionType string

const (
	// ActionTypeSubmit represents a primary/approve action
	ActionTypeSubmit ActionType = "SUBMIT"
	// ActionTypeReject represents a reject/deny action
	ActionTypeReject ActionType = "REJECT"
)

// ForwardedData key constants define keys used in the ForwardedData map.
const (
	// ForwardedDataKeyInputs is the key used to store input data in ForwardedData
	ForwardedDataKeyInputs = "inputs"
	// ForwardedDataKeyConsentPrompt is the key used to forward consent prompt data to the prompt node
	ForwardedDataKeyConsentPrompt = "consent_prompt"
	// ForwardedDataKeyActionType holds the action type selected by the user for the immediate next node
	ForwardedDataKeyActionType = "actionType"
	// ForwardedDataKeyTemplateData holds template parameters for notification executors
	ForwardedDataKeyTemplateData = "templateData"
)

// ValidationType identifies the constraint type of a ValidationRule.
type ValidationType string

// Validation rule types.
const (
	// ValidationTypeRegex matches the submitted value against a regex pattern.
	ValidationTypeRegex ValidationType = "regex"
	// ValidationTypeMinLength enforces a minimum string length on the submitted value.
	ValidationTypeMinLength ValidationType = "minLength"
	// ValidationTypeMaxLength enforces a maximum string length on the submitted value.
	ValidationTypeMaxLength ValidationType = "maxLength"
)

// Default i18n fallback message keys returned in fieldErrors when a validation
// rule does not specify a message.
const (
	DefaultValidationMessageRegex     = "validation.pattern.invalid"
	DefaultValidationMessageMinLength = "validation.minLength.invalid"
	DefaultValidationMessageMaxLength = "validation.maxLength.invalid"
)
