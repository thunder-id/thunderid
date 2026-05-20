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

package authn

import "github.com/thunder-id/thunderid/internal/idp"

// IDPAuthInitData represents the data returned when initiating IDP authentication.
type IDPAuthInitData struct {
	RedirectURL  string
	SessionToken string
}

// AuthSessionData represents the data stored in the authentication session token.
type AuthSessionData struct {
	IDPID   string      `json:"idpId"`
	IDPType idp.IDPType `json:"idpType"`
}

// AuthenticationResponseDTO represents the data transfer object for the authentication response.
type AuthenticationResponseDTO struct {
	ID        string `json:"id"`
	Type      string `json:"type,omitempty"`
	OUID      string `json:"ouId,omitempty"`
	Assertion string `json:"assertion,omitempty"`
}

// IDPAuthInitRequestDTO is the request to initiate IDP authentication.
type IDPAuthInitRequestDTO struct {
	IDPID string `json:"idpId"`
}

// IDPAuthInitResponseDTO is the response after initiating IDP authentication.
type IDPAuthInitResponseDTO struct {
	RedirectURL  string `json:"redirectUrl,omitempty"`
	SessionToken string `json:"sessionToken"`
}

// IDPAuthFinishRequestDTO is the request to complete IDP authentication.
type IDPAuthFinishRequestDTO struct {
	SessionToken  string `json:"sessionToken"`
	SkipAssertion bool   `json:"skipAssertion"`
	Assertion     string `json:"assertion,omitempty"`
	Code          string `json:"code"`
}

// SendOTPAuthRequestDTO is the request to send an OTP for authentication.
type SendOTPAuthRequestDTO struct {
	SenderID  string `json:"senderId"`
	Recipient string `json:"recipient"`
}

// SendOTPAuthResponseDTO is the response after sending an OTP for authentication.
type SendOTPAuthResponseDTO struct {
	Status       string `json:"status"`
	SessionToken string `json:"sessionToken"`
}

// VerifyOTPAuthRequestDTO is the request to verify an OTP for authentication.
type VerifyOTPAuthRequestDTO struct {
	SessionToken  string `json:"sessionToken"`
	SkipAssertion bool   `json:"skipAssertion"`
	Assertion     string `json:"assertion,omitempty"`
	OTP           string `json:"otp"`
}

// PasskeyAuthenticatorSelectionDTO represents the authenticator selection criteria for passkey.
type PasskeyAuthenticatorSelectionDTO struct {
	AuthenticatorAttachment string `json:"authenticatorAttachment,omitempty"`
	RequireResidentKey      bool   `json:"requireResidentKey,omitempty"`
	ResidentKey             string `json:"residentKey,omitempty"`
	UserVerification        string `json:"userVerification,omitempty"`
}

// PasskeyRegisterStartRequestDTO is the request to start passkey registration.
type PasskeyRegisterStartRequestDTO struct {
	UserID                 string                            `json:"userId"`
	RelyingPartyID         string                            `json:"relyingPartyId"`
	RelyingPartyName       string                            `json:"relyingPartyName"`
	AuthenticatorSelection *PasskeyAuthenticatorSelectionDTO `json:"authenticatorSelection,omitempty"`
	Attestation            string                            `json:"attestation,omitempty"`
}

// PasskeyPublicKeyCredentialDTO represents a WebAuthn public key credential.
type PasskeyPublicKeyCredentialDTO struct {
	ID       string                       `json:"id"`
	RawID    string                       `json:"rawId,omitempty"`
	Type     string                       `json:"type"`
	Response PasskeyCredentialResponseDTO `json:"response"`
}

// PasskeyCredentialResponseDTO represents the response from a WebAuthn credential.
type PasskeyCredentialResponseDTO struct {
	ClientDataJSON    string `json:"clientDataJSON"`
	AttestationObject string `json:"attestationObject,omitempty"`
	AuthenticatorData string `json:"authenticatorData,omitempty"`
	Signature         string `json:"signature,omitempty"`
	UserHandle        string `json:"userHandle,omitempty"`
}

// PasskeyRegisterFinishRequestDTO is the request to finish passkey registration.
type PasskeyRegisterFinishRequestDTO struct {
	PublicKeyCredential PasskeyPublicKeyCredentialDTO `json:"publicKeyCredential"`
	SessionToken        string                        `json:"sessionToken"`
	CredentialName      string                        `json:"credentialName,omitempty"`
}

// PasskeyStartRequestDTO is the request to start passkey authentication.
type PasskeyStartRequestDTO struct {
	UserID         string `json:"userId"`
	RelyingPartyID string `json:"relyingPartyId"`
}

// PasskeyFinishRequestDTO is the request to finish passkey authentication.
type PasskeyFinishRequestDTO struct {
	PublicKeyCredential PasskeyPublicKeyCredentialDTO `json:"publicKeyCredential"`
	SessionToken        string                        `json:"sessionToken"`
	SkipAssertion       bool                          `json:"skipAssertion"`
	Assertion           string                        `json:"assertion,omitempty"`
}

// AuthenticateWithCredentialsRequestDTO represents the request body for authenticating with credentials.
type AuthenticateWithCredentialsRequestDTO struct {
	Identifiers   map[string]interface{} `json:"identifiers"`
	Credentials   map[string]interface{} `json:"credentials"`
	SkipAssertion *bool                  `json:"skipAssertion,omitempty"`
	Assertion     *string                `json:"assertion,omitempty"`
}
