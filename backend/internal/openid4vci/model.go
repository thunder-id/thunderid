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

// Package openid4vci implements a minimal OpenID4VCI credential issuer that
// issues SD-JWT VCs to wallets via the authorization_code flow. The credential
// subject is the authenticated Thunder user; claims are sourced from the user's
// profile attributes. Engine and credential configuration are config-driven.
package openid4vci

import (
	"errors"
	"time"
)

// proofType is the required "typ" header of an OpenID4VCI holder proof JWT.
const proofType = "openid4vci-proof+jwt"

// Sentinel errors returned by the issuer. HTTP-facing errors live in error_constants.go.
var (
	// ErrPolicy indicates an invalid issuer configuration.
	ErrPolicy = errors.New("openid4vci: invalid issuer configuration")
	// ErrInvalidRequest indicates a malformed credential request.
	ErrInvalidRequest = errors.New("openid4vci: invalid request")
	// ErrInvalidToken indicates a missing, invalid, or unauthorized access token.
	ErrInvalidToken = errors.New("openid4vci: invalid or unauthorized access token")
	// ErrUnsupportedCredential indicates the requested credential configuration is unknown.
	ErrUnsupportedCredential = errors.New("openid4vci: unsupported credential configuration")
	// ErrInvalidProof indicates the holder proof JWT failed validation.
	ErrInvalidProof = errors.New("openid4vci: invalid holder proof")
	// ErrInvalidDPoP indicates the DPoP proof bound to the access token failed validation.
	ErrInvalidDPoP = errors.New("openid4vci: invalid DPoP proof")
	// ErrInvalidNonce indicates the proof carried an unknown or expired c_nonce.
	ErrInvalidNonce = errors.New("openid4vci: invalid or expired c_nonce")
	// ErrUserNotFound indicates the access-token subject resolves to no user.
	ErrUserNotFound = errors.New("openid4vci: subject user not found")
	// ErrIssuance indicates the credential could not be signed/assembled.
	ErrIssuance = errors.New("openid4vci: credential issuance failed")
)

// credentialConfig is a resolved credential configuration the issuer can serve.
type credentialConfig struct {
	Format   string
	VCT      string
	SDClaims []string
	Validity time.Duration
}

// nonceRecord is the stored c_nonce state, keyed by the nonce value.
type nonceRecord struct {
	Nonce     string
	ExpiresAt time.Time
}

// offerRecord is a stored issuer-initiated credential offer, keyed by its id.
type offerRecord struct {
	ID        string
	Offer     map[string]interface{}
	ExpiresAt time.Time
}

// CredentialRequest is the POST /credential request body. It accepts both the
// single "proof" (older drafts) and the batched "proofs" (draft 15+/1.0) holder
// proof-of-possession forms.
type CredentialRequest struct {
	CredentialConfigurationID string  `json:"credential_configuration_id,omitempty"`
	Proof                     Proof   `json:"proof"`
	Proofs                    *Proofs `json:"proofs,omitempty"`
}

// Proof is the single holder proof of possession (older OID4VCI drafts).
type Proof struct {
	ProofType string `json:"proof_type"`
	JWT       string `json:"jwt"`
}

// Proofs is the batched holder proof form (OID4VCI draft 15+): proof type to a
// list of proof JWTs.
type Proofs struct {
	JWT []string `json:"jwt,omitempty"`
}

// holderProofs returns all holder proof JWTs, preferring the batched "proofs"
// form (one proof per holder key, for unlinkable batch issuance) and falling
// back to the single "proof". The wallet binds each issued copy to a distinct
// key, so one credential is issued per returned proof.
func (c *CredentialRequest) holderProofs() []Proof {
	if c.Proofs != nil && len(c.Proofs.JWT) > 0 {
		proofs := make([]Proof, 0, len(c.Proofs.JWT))
		for _, j := range c.Proofs.JWT {
			proofs = append(proofs, Proof{ProofType: "jwt", JWT: j})
		}
		return proofs
	}
	if c.Proof.JWT != "" {
		return []Proof{c.Proof}
	}
	return nil
}

// CredentialResponse is the POST /credential response body.
type CredentialResponse struct {
	Credentials []IssuedCredential `json:"credentials"`
}

// IssuedCredential carries a single issued credential string.
type IssuedCredential struct {
	Credential string `json:"credential"`
}

// NonceResponse is the POST /nonce response body.
type NonceResponse struct {
	CNonce string `json:"c_nonce"`
}
