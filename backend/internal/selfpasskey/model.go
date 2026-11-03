// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package selfpasskey

import "github.com/thunder-id/thunderid/internal/authn/passkey"

// registrationStartRequest is the request to start adding a passkey.
type registrationStartRequest struct {
	RelyingPartyID string `json:"relyingPartyId"`
}

// publicKeyCredential is the WebAuthn credential the device created.
type publicKeyCredential struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Response attestationResponse `json:"response"`
}

// attestationResponse is the attestation the device returned for a new passkey.
type attestationResponse struct {
	ClientDataJSON    string `json:"clientDataJSON"`
	AttestationObject string `json:"attestationObject"`
}

// registrationFinishRequest is the request to save a new passkey.
type registrationFinishRequest struct {
	PublicKeyCredential publicKeyCredential `json:"publicKeyCredential"`
	SessionToken        string              `json:"sessionToken"`
	Name                string              `json:"name,omitempty"`
}

// passkeyListResponse is the list of the user's passkeys.
type passkeyListResponse struct {
	Passkeys []passkey.PasskeyInfo `json:"passkeys"`
}
