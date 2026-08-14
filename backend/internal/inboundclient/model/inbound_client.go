// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package model holds public data types for the inbound client subsystem.
//
//nolint:lll
package model

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

type (
	// InboundClient is the persistence shape for protocol-agnostic inbound client record.
	InboundClient = providers.InboundClient
	// AssertionConfig is the entity-level assertion config; token configs fall back to it.
	AssertionConfig = providers.AssertionConfig
	// LoginConsentConfig is the login consent configuration.
	LoginConsentConfig = providers.LoginConsentConfig
	// Certificate is a user-supplied certificate input.
	Certificate = providers.Certificate
)

// InboundAuthProfileReq carries the inbound authentication profile attributes accepted and returned by
// the management APIs, and accepted from declarative resource files. It is embedded by the request and
// response types of both the application and agent modules.
//
// It mirrors providers.InboundAuthProfile with one deliberate omission: SubjectAttribute. That
// attribute is internal-only, allowed to be fed with Providers.
type InboundAuthProfileReq struct {
	AuthFlowID                string                        `json:"authFlowId,omitempty"             yaml:"authFlowId,omitempty"             jsonschema:"Authentication flow ID. Optional. Specifies which login flow to use (e.g., MFA, passwordless). If omitted, the default authentication flow is used."`
	AuthFlowHandle            string                        `json:"authFlowHandle,omitempty"         yaml:"authFlowHandle,omitempty"         jsonschema:"Authentication flow handle. Optional. Alternative to authFlowId — resolved to an ID at import time."`
	RegistrationFlowID        string                        `json:"registrationFlowId,omitempty"     yaml:"registrationFlowId,omitempty"     jsonschema:"Registration flow ID. Optional. Specifies the user registration/signup flow."`
	RegistrationFlowHandle    string                        `json:"registrationFlowHandle,omitempty" yaml:"registrationFlowHandle,omitempty" jsonschema:"Registration flow handle. Optional. Alternative to registrationFlowId — resolved to an ID at import time."`
	IsRegistrationFlowEnabled bool                          `json:"isRegistrationFlowEnabled"        yaml:"isRegistrationFlowEnabled"        jsonschema:"Enable self-service registration. Set to true to allow users to sign up themselves. Requires registrationFlowId or registrationFlowHandle to be set."`
	RecoveryFlowID            string                        `json:"recoveryFlowId,omitempty"         yaml:"recoveryFlowId,omitempty"         jsonschema:"Recovery flow ID. Optional. Specifies the user recovery flow."`
	RecoveryFlowHandle        string                        `json:"recoveryFlowHandle,omitempty"     yaml:"recoveryFlowHandle,omitempty"     jsonschema:"Recovery flow handle. Optional. Alternative to recoveryFlowId — resolved to an ID at import time."`
	IsRecoveryFlowEnabled     bool                          `json:"isRecoveryFlowEnabled"            yaml:"isRecoveryFlowEnabled"            jsonschema:"Enable self-service recovery. Set to true to allow users to recover their accounts (e.g., password reset). Requires recoveryFlowId or recoveryFlowHandle to be set."`
	SignOutFlowID             string                        `json:"signOutFlowId,omitempty"           yaml:"signOutFlowId,omitempty"           jsonschema:"Sign-out flow ID. Optional. Specifies the flow that terminates the SSO session established by the authentication flow."`
	SignOutFlowHandle         string                        `json:"signOutFlowHandle,omitempty"       yaml:"signOutFlowHandle,omitempty"       jsonschema:"Sign-out flow handle. Optional. Alternative to signOutFlowId — resolved to an ID at import time."`
	ThemeID                   string                        `json:"themeId,omitempty"                yaml:"themeId,omitempty"                jsonschema:"Theme configuration ID. Optional. Customizes the visual styling of login pages."`
	LayoutID                  string                        `json:"layoutId,omitempty"               yaml:"layoutId,omitempty"               jsonschema:"Layout configuration ID. Optional. Customizes the screen structure and component positioning of login pages."`
	Assertion                 *providers.AssertionConfig    `json:"assertion,omitempty"              yaml:"assertion,omitempty"              jsonschema:"Assertion configuration. Optional. Customize assertion validity periods and included user attributes."`
	LoginConsent              *providers.LoginConsentConfig `json:"loginConsent,omitempty"           yaml:"loginConsent,omitempty"           jsonschema:"Login consent configuration settings."`
	AllowedUserTypes          []string                      `json:"allowedUserTypes,omitempty"           yaml:"allowedUserTypes,omitempty"           jsonschema:"Allowed user types. Optional. Restricts which user types can register or sign up through this resource."`
	SubjectAttribute          map[string]string             `json:"subjectAttribute,omitempty"           yaml:"subjectAttribute,omitempty"           jsonschema:"Per-user-type mapping of the schema attribute to use as the token subject (sub) claim, keyed by user type name. The attribute must be unique, required, and string-typed in that user type's schema. When no entry applies, the user's ID is used as the subject."`
	PasskeyAllowedOrigins     []string                      `json:"passkeyAllowedOrigins,omitempty"      yaml:"passkeyAllowedOrigins,omitempty"      jsonschema:"Allowed origins for WebAuthn/passkey operations for this application. Optional. When set, overrides the server-level passkey allowed origins for flow-based passkey operations."`
	Attestation               *providers.AttestationConfig  `json:"attestation,omitempty"                yaml:"attestation,omitempty"                jsonschema:"Platform attestation configuration. Optional. Enables a mobile client to initiate flows directly by proving its binary identity (e.g. Google Play Integrity), regardless of protocol. The service account credentials are write-only and never returned in responses."`
}

// InboundClientAttributes is the flattened view of one inbound client's configured user attributes.
type InboundClientAttributes struct {
	InboundClientID string
	Attributes      []string
}

// DeclarativeLoaderConfig describes how to load inbound clients from a YAML resource directory.
type DeclarativeLoaderConfig struct {
	ResourceType  string
	DirectoryName string
	Parser        func(data []byte) (*InboundClient, error)
	Validator     func(*InboundClient) error
}
