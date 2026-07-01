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

// Package openid4vp implements an OpenID4VP verifier.
package openid4vp

import (
	"crypto/ecdsa"
	"slices"
	"time"

	"github.com/thunder-id/thunderid/internal/openid4vp/definition"
)

// policy is the verification policy applied to a presentation.
type policy struct {
	ExpectedVCT     string
	Audience        string
	RequestedClaims []string
	MandatoryClaims []string
	Leeway          time.Duration
	// KeyBindingMaxAge rejects KB-JWTs older than this (replay protection). 0 = disabled.
	KeyBindingMaxAge time.Duration
	// EnforceTrustedIssuer requires the issuer to be in the trust store.
	EnforceTrustedIssuer bool
	// TrustedAuthorities restricts acceptable trust anchors by name; empty means any.
	TrustedAuthorities []string
	// EnforceKeyBinding requires a Key Binding JWT proving holder possession.
	EnforceKeyBinding bool
	// ClaimValues constrains specific claims to an allowed set of values.
	ClaimValues map[string][]string
}

// verifiedCredential is the raw output of an SD-JWT VC verification, before policy enforcement.
type verifiedCredential struct {
	Issuer               string
	VCT                  string
	Claims               map[string]interface{}
	DisclosedPaths       []string
	KeyBindingThumbprint string
}

// VerifiedPresentation is the result of a successful presentation verification.
type VerifiedPresentation struct {
	Subject              string
	Issuer               string
	VCT                  string
	Claims               map[string]interface{}
	DisclosedPaths       []string
	KeyBindingThumbprint string
}

// subjectDeriver produces the authenticated subject for a verified presentation.
type subjectDeriver func(*VerifiedPresentation) string

// presentationDefinition describes a single, named presentation request the verifier can serve.
// Optional fields fall back to the engine defaults supplied by the Service.
type presentationDefinition struct {
	ID            string
	DCQL          dcqlConfig
	policy        policy
	DeriveSubject subjectDeriver
}

// dtoToDefinition converts a managed presentation definition DTO into the verifier engine's representation.
func dtoToDefinition(
	dto definition.PresentationDefinitionDTO, clientID string, enforceKB bool,
) *presentationDefinition {
	allClaims := dto.RequestedClaims
	if len(allClaims) == 0 {
		allClaims = slices.Concat(dto.MandatoryClaims, dto.OptionalClaims)
	}
	enforceTrustedIssuer := false
	if dto.EnforceTrustedIssuer != nil {
		enforceTrustedIssuer = *dto.EnforceTrustedIssuer
	}
	return &presentationDefinition{
		ID: dto.Handle,
		DCQL: dcqlConfig{
			CredentialID:       dto.Handle,
			VCT:                dto.VCT,
			Claims:             allClaims,
			ClaimValues:        dto.ClaimValues,
			TrustedAuthorities: dto.TrustedAuthorities,
		},
		policy: policy{
			ExpectedVCT:          dto.VCT,
			Audience:             clientID,
			RequestedClaims:      allClaims,
			MandatoryClaims:      dto.MandatoryClaims,
			EnforceTrustedIssuer: enforceTrustedIssuer,
			TrustedAuthorities:   dto.TrustedAuthorities,
			EnforceKeyBinding:    enforceKB,
			ClaimValues:          dto.ClaimValues,
		},
		DeriveSubject: defaultSubjectDeriver(),
	}
}

// dcqlConfig describes the credential query to build.
type dcqlConfig struct {
	CredentialID string
	VCT          string
	Claims       []string
	// ClaimValues maps a claim path to its allowed values (DCQL "values").
	ClaimValues map[string][]string
	// TrustedAuthorities names the trust anchors this definition restricts to.
	TrustedAuthorities []string
	// TrustedAuthorityKeyIDs holds the resolved base64url SubjectKeyIds, emitted as DCQL trusted_authorities.
	TrustedAuthorityKeyIDs []string
}

// dcqlQuery is the OpenID4VP Digital Credentials Query Language query object.
type dcqlQuery struct {
	Credentials []dcqlCredential `json:"credentials"`
}

// dcqlCredential is a single credential query.
type dcqlCredential struct {
	ID                 string             `json:"id"`
	Format             string             `json:"format"`
	Meta               *dcqlMeta          `json:"meta,omitempty"`
	Claims             []dcqlClaim        `json:"claims,omitempty"`
	TrustedAuthorities []trustedAuthority `json:"trusted_authorities,omitempty"`
}

// trustedAuthority is a per-credential-query trust constraint (OpenID4VP 1.0).
type trustedAuthority struct {
	Type   string   `json:"type"`
	Values []string `json:"values"`
}

// dcqlMeta carries format-specific matching metadata.
type dcqlMeta struct {
	VCTValues []string `json:"vct_values,omitempty"`
}

// dcqlClaim selects a single claim by its path.
type dcqlClaim struct {
	Path   []interface{} `json:"path"`
	Values []interface{} `json:"values,omitempty"`
}

// requestConfig is the static (config-driven) part of the OpenID4VP request.
type requestConfig struct {
	ClientID          string
	ResponseURI       string
	ResponseMode      string
	Audience          string
	Validity          time.Duration
	DCQL              dcqlConfig
	ResponseEncValues []string
	// VerifierInfo is passed through opaquely; obtaining it is part of RP onboarding.
	VerifierInfo []interface{}
}

// requestParams is the per-request dynamic input to the request object builder.
type requestParams struct {
	Nonce          string
	State          string
	EphemeralKey   *ecdsa.PublicKey
	EphemeralKeyID string
	IssuedAt       time.Time
}

// authorizationResponse is the decrypted direct_post.jwt body returned by the wallet.
type authorizationResponse struct {
	State         string
	Presentations map[string][]string
}

// serviceConfig is the engine-level configuration of the OpenID4VP verifier.
// Credential-specific values live on the presentationDefinition.
type serviceConfig struct {
	RequestURIBase        string
	ResponseURIBase       string
	EphemeralKeyID        string
	ResponseEncValues     []string
	RequestAudience       string
	RequestValidity       time.Duration
	TTL                   time.Duration
	Leeway                time.Duration
	KeyBindingMaxAge      time.Duration
	ResultRedirectURIBase string
	// VerifierInfo is attached to every signed request object (e.g. a registration certificate JWT).
	VerifierInfo      []interface{}
	EnforceKeyBinding bool
}

// Initiation is what the client needs to render the QR / deep link.
type Initiation struct {
	State      string
	ClientID   string
	RequestURI string
}

// trustAnchor is a configured trust anchor (root CA) certificate file.
type trustAnchor struct {
	Name string
	// CertFile is a path to a PEM CERTIFICATE, resolved before passing.
	CertFile string
}

// TrustAnchorInfo is the public view of a configured trust anchor.
type TrustAnchorInfo struct {
	Name     string    `json:"name"`
	Subject  string    `json:"subject"`
	SKI      string    `json:"ski"`
	NotAfter time.Time `json:"not_after"`
}

// Status is the lifecycle status of an OpenID4VP request.
type Status string

// Status values reported by the engine and consumed by callers (flow executor,
// RP-facing status endpoint).
const (
	StatusPending   Status = "PENDING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
)

// RequestState is the short-lived per-request state correlated by State.
type RequestState struct {
	State        string
	DefinitionID string
	Nonce        string
	EphemeralKey *ecdsa.PrivateKey
	ClientID     string
	// RPID is the calling relying party (RP-facing API only). Recorded so the result token's aud matches.
	RPID          string
	RequestURI    string
	Status        Status
	Result        *VerifiedPresentation
	FailureReason string
	ExpiresAt     time.Time
}

// initiateRequest is the RP-facing initiate payload.
type initiateRequest struct {
	DefinitionID string `json:"definition_id"`
	RPID         string `json:"rp_id"`
}

// initiateResponse is the RP-facing initiate response.
type initiateResponse struct {
	TxnID     string `json:"txn_id"`
	WalletURL string `json:"wallet_url"`
	StatusURL string `json:"status_url"`
	ExpiresAt string `json:"expires_at"`
}

// statusResponse is the RP-facing status response.
// On COMPLETED the RP validates result_token to extract subject + verified_claims — not echoed unsigned here.
type statusResponse struct {
	Status      string `json:"status"`
	ResultToken string `json:"result_token,omitempty"`
	Error       string `json:"error,omitempty"`
}
