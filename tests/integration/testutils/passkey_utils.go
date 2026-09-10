// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// passkeyRegisterStartRequest mirrors PasskeyRegisterStartRequestDTO in the backend.
type passkeyRegisterStartRequest struct {
	UserID                 string                         `json:"userId"`
	RelyingPartyID         string                         `json:"relyingPartyId"`
	RelyingPartyName       string                         `json:"relyingPartyName,omitempty"`
	AuthenticatorSelection *passkeyAuthenticatorSelection `json:"authenticatorSelection,omitempty"`
	Assertion              string                         `json:"assertion,omitempty"`
}

type passkeyAuthenticatorSelection struct {
	ResidentKey      string `json:"residentKey,omitempty"`
	UserVerification string `json:"userVerification,omitempty"`
}

// passkeyRegisterStartResponse captures only the fields callers need from the start response.
type passkeyRegisterStartResponse struct {
	SessionToken                       string `json:"sessionToken"`
	PublicKeyCredentialCreationOptions struct {
		Challenge string `json:"challenge"`
		User      struct {
			ID string `json:"id"`
		} `json:"user"`
	} `json:"publicKeyCredentialCreationOptions"`
}

// passkeyRegisterFinishRequest mirrors PasskeyRegisterFinishRequestDTO in the backend, where the
// credential is nested under publicKeyCredential.
type passkeyRegisterFinishRequest struct {
	PublicKeyCredential passkeyAttestationCredential `json:"publicKeyCredential"`
	SessionToken        string                       `json:"sessionToken"`
}

type passkeyAttestationCredential struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	RawID    string `json:"rawId"`
	Response struct {
		ClientDataJSON    string `json:"clientDataJSON"`
		AttestationObject string `json:"attestationObject"`
	} `json:"response"`
}

// RegisterPasskeyCredential registers a passkey for the given user through the direct passkey API
// and returns the virtual authenticator holding it, along with the WebAuthn user handle the server
// issued for that user. Flow based suites use this to obtain a credential without having to drive a
// registration flow.
//
// The assertion must prove userID, since the direct API enrolls a credential only for the subject
// it names; ObtainAuthAssertion produces one from a password login.
//
// The origin must be accepted by the server level passkey.allowed_origins, since the direct API
// does not take allowed origins from the request or the application.
func RegisterPasskeyCredential(userID, relyingPartyID, relyingPartyName, origin, assertion string) (
	*VirtualAuthenticator, string, error) {
	authenticator, err := NewVirtualAuthenticator(relyingPartyID, origin)
	if err != nil {
		return nil, "", err
	}

	startBody, err := passkeyPost("/register/passkey/start", passkeyRegisterStartRequest{
		UserID:           userID,
		RelyingPartyID:   relyingPartyID,
		RelyingPartyName: relyingPartyName,
		AuthenticatorSelection: &passkeyAuthenticatorSelection{
			ResidentKey:      "required",
			UserVerification: "required",
		},
		Assertion: assertion,
	})
	if err != nil {
		return nil, "", fmt.Errorf("passkey registration start failed: %w", err)
	}

	var startResponse passkeyRegisterStartResponse
	if err := json.Unmarshal(startBody, &startResponse); err != nil {
		return nil, "", fmt.Errorf("failed to decode passkey registration start response: %w", err)
	}

	credentialID, clientDataJSON, attestationObject, err := authenticator.CreateAttestationResponse(
		startResponse.PublicKeyCredentialCreationOptions.Challenge, true)
	if err != nil {
		return nil, "", err
	}

	credential := passkeyAttestationCredential{ID: credentialID, Type: "public-key", RawID: credentialID}
	credential.Response.ClientDataJSON = clientDataJSON
	credential.Response.AttestationObject = attestationObject

	if _, err := passkeyPost("/register/passkey/finish", passkeyRegisterFinishRequest{
		PublicKeyCredential: credential,
		SessionToken:        startResponse.SessionToken,
	}); err != nil {
		return nil, "", fmt.Errorf("passkey registration finish failed: %w", err)
	}

	return authenticator, startResponse.PublicKeyCredentialCreationOptions.User.ID, nil
}

// passkeyAuthStartRequest mirrors PasskeyStartRequestDTO in the backend.
type passkeyAuthStartRequest struct {
	UserID         string `json:"userId"`
	RelyingPartyID string `json:"relyingPartyId"`
}

type passkeyAuthStartResponse struct {
	SessionToken                      string `json:"sessionToken"`
	PublicKeyCredentialRequestOptions struct {
		Challenge string `json:"challenge"`
	} `json:"publicKeyCredentialRequestOptions"`
}

// passkeyAuthFinishRequest mirrors PasskeyFinishRequestDTO in the backend, where the credential is
// nested under publicKeyCredential just as it is for registration.
type passkeyAuthFinishRequest struct {
	PublicKeyCredential passkeyAssertionCredential `json:"publicKeyCredential"`
	SessionToken        string                     `json:"sessionToken"`
}

type passkeyAssertionCredential struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	RawID    string `json:"rawId"`
	Response struct {
		ClientDataJSON    string `json:"clientDataJSON"`
		AuthenticatorData string `json:"authenticatorData"`
		Signature         string `json:"signature"`
		UserHandle        string `json:"userHandle,omitempty"`
	} `json:"response"`
}

// AuthenticateWithPasskey runs a full authentication ceremony for the given user with the supplied
// authenticator, and returns the authentication response. Registration suites use it to prove a
// credential they just enrolled is actually usable, which is the only externally visible evidence
// that the credential was stored correctly.
func AuthenticateWithPasskey(
	userID, relyingPartyID, userHandle string, authenticator *VirtualAuthenticator,
) (*AuthenticationResponse, error) {
	startBody, err := passkeyPost("/auth/passkey/start", passkeyAuthStartRequest{
		UserID:         userID,
		RelyingPartyID: relyingPartyID,
	})
	if err != nil {
		return nil, fmt.Errorf("passkey authentication start failed: %w", err)
	}

	var startResponse passkeyAuthStartResponse
	if err := json.Unmarshal(startBody, &startResponse); err != nil {
		return nil, fmt.Errorf("failed to decode passkey authentication start response: %w", err)
	}

	credentialID, clientDataJSON, authenticatorData, signature, err :=
		authenticator.CreateAssertionResponse(
			startResponse.PublicKeyCredentialRequestOptions.Challenge, true)
	if err != nil {
		return nil, err
	}

	credential := passkeyAssertionCredential{ID: credentialID, Type: "public-key", RawID: credentialID}
	credential.Response.ClientDataJSON = clientDataJSON
	credential.Response.AuthenticatorData = authenticatorData
	credential.Response.Signature = signature
	credential.Response.UserHandle = userHandle

	finishBody, err := passkeyPost("/auth/passkey/finish", passkeyAuthFinishRequest{
		PublicKeyCredential: credential,
		SessionToken:        startResponse.SessionToken,
	})
	if err != nil {
		return nil, fmt.Errorf("passkey authentication finish failed: %w", err)
	}

	var response AuthenticationResponse
	if err := json.Unmarshal(finishBody, &response); err != nil {
		return nil, fmt.Errorf("failed to decode passkey authentication response: %w", err)
	}

	return &response, nil
}

// passkeyPost posts a JSON body to a passkey endpoint and returns the response body, treating any
// non-200 status as an error.
func passkeyPost(path string, payload interface{}) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, TestServerURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, path, responseBody)
	}

	return responseBody, nil
}
