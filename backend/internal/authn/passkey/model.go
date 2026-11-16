// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package passkey

// AuthenticatorSelection represents criteria for selecting authenticators during registration.
type AuthenticatorSelection struct {
	AuthenticatorAttachment string
	RequireResidentKey      bool
	ResidentKey             string
	UserVerification        string
}

// PasskeyRegistrationStartRequest represents the request to start passkey credential registration.
type PasskeyRegistrationStartRequest struct {
	UserID                 string
	RelyingPartyID         string
	RelyingPartyName       string
	AuthenticatorSelection *AuthenticatorSelection
	Attestation            string
	AllowedOrigins         []string
}

// PasskeyRegistrationStartData represents the data returned when initiating passkey registration.
type PasskeyRegistrationStartData struct {
	PublicKeyCredentialCreationOptions PublicKeyCredentialCreationOptions `json:"publicKeyCredentialCreationOptions"`
	SessionToken                       string                             `json:"sessionToken"`
}

// PublicKeyCredentialCreationOptions represents the options for credential creation.
type PublicKeyCredentialCreationOptions struct {
	Challenge              string                   `json:"challenge"`
	RelyingParty           relyingPartyEntity       `json:"rp"`
	User                   userEntity               `json:"user"`
	Parameters             []credentialParameter    `json:"pubKeyCredParams"`
	AuthenticatorSelection authenticatorSelection   `json:"authenticatorSelection,omitempty"`
	Timeout                int                      `json:"timeout,omitempty"`
	CredentialExcludeList  []credentialDescriptor   `json:"excludeCredentials,omitempty"`
	Extensions             authenticationExtensions `json:"extensions,omitempty"`
	Attestation            conveyancePreference     `json:"attestation,omitempty"`
}

// PasskeyRegistrationFinishRequest represents the request to finish passkey credential registration.
type PasskeyRegistrationFinishRequest struct {
	CredentialID      string
	CredentialType    string
	ClientDataJSON    string
	AttestationObject string
	SessionToken      string
	AllowedOrigins    []string
	UserID            string
	Name              string
}

// PasskeyInfo describes one of a user's registered passkeys.
type PasskeyInfo struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// PasskeyAuthenticationStartRequest represents the request to start passkey authentication.
type PasskeyAuthenticationStartRequest struct {
	UserID         string
	RelyingPartyID string
	AllowedOrigins []string
}

// PasskeyAuthenticationStartData represents the data returned when initiating passkey authentication.
type PasskeyAuthenticationStartData struct {
	PublicKeyCredentialRequestOptions PublicKeyCredentialRequestOptions `json:"publicKeyCredentialRequestOptions"`
	SessionToken                      string                            `json:"sessionToken"`
}

// PublicKeyCredentialRequestOptions represents the options for credential assertion.
type PublicKeyCredentialRequestOptions struct {
	Challenge        string                      `json:"challenge"`
	Timeout          int                         `json:"timeout,omitempty"`
	RelyingPartyID   string                      `json:"rpId,omitempty"`
	AllowCredentials []credentialDescriptor      `json:"allowCredentials,omitempty"`
	UserVerification userVerificationRequirement `json:"userVerification,omitempty"`
	Extensions       authenticationExtensions    `json:"extensions,omitempty"`
}

// CredentialDescriptor represents a WebAuthn credential descriptor.
type CredentialDescriptor struct {
	Type       string
	ID         string
	Transports []string
}

// PasskeyAuthenticationFinishRequest represents the request to finish passkey authentication.
type PasskeyAuthenticationFinishRequest struct {
	CredentialID      string
	CredentialType    string
	ClientDataJSON    string
	AuthenticatorData string
	Signature         string
	UserHandle        string
	SessionToken      string
	AllowedOrigins    []string
}

// PasskeyFinishRequest represents the request to complete passkey authentication.
type PasskeyFinishRequest struct {
	PublicKeyCredential *parsedCredentialAssertionData
	SessionToken        string
	SkipAssertion       bool
	Assertion           string
}

// webauthnUserInterface defines the interface for WebAuthn user operations.
type webauthnUserInterface interface {
	WebAuthnID() []byte
	WebAuthnName() string
	WebAuthnDisplayName() string
	WebAuthnCredentials() []webauthnCredential
}
