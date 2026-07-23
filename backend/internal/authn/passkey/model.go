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
}

// PasskeyAuthenticationStartRequest represents the request to start passkey authentication.
type PasskeyAuthenticationStartRequest struct {
	UserID         string
	RelyingPartyID string
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
