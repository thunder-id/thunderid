// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package openid4vp implements an OpenID4VP verifier and the authentication service for authentication via OpenID4VP.
package openid4vp

import (
	"crypto/ecdsa"
	"slices"
	"time"

	"github.com/thunder-id/thunderid/internal/vc/presentation"
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
	// EnforceTrustedIssuer requires the issuer's x5c chain to verify against the definition's trust anchors.
	EnforceTrustedIssuer bool
	// EnforceKeyBinding requires a Key Binding JWT proving holder possession.
	EnforceKeyBinding bool
	// ClaimValues constrains specific claims to an allowed set of values.
	ClaimValues map[string][]string
}

// verifiedCredential is the raw output of an SD-JWT VC verification, before policy enforcement.
type verifiedCredential struct {
	Issuer               string
	IssuerBound          bool
	VCT                  string
	Claims               map[string]interface{}
	DisclosedPaths       []string
	KeyBindingThumbprint string
}

// VerifiedPresentation is the result of a successful presentation verification.
type VerifiedPresentation struct {
	// DefinitionID is the stable id of the presentation definition whose trusted authorities verified the
	// credential. Links are scoped to it, since each definition trusts its own authorities.
	DefinitionID string
	Issuer       string
	// IssuerTrusted reports whether iss was verified: the chain ends in a trust anchor and the leaf
	// certificate carries iss. Only then does the subject identify a holder: otherwise anyone can mint any.
	IssuerTrusted        bool
	Subject              string
	Claims               map[string]interface{}
	KeyBindingThumbprint string
}

// maxLinkedConnectionIDLength bounds the connection id a link is recorded for, so the indexed name
// "linkedIds.<connection id>" fits the entity identifier name column.
const maxLinkedConnectionIDLength = 245

// LinkedConnectionID returns the connection id that links a holder's subject to a local user. A subject
// is unique only within the issuer that asserted it, and an issuer is trusted only by the definition whose
// authorities verified it, so links are scoped per definition and issuer.
func LinkedConnectionID(definitionID, issuer string) string {
	return "openid4vp:" + definitionID + ":" + issuer
}

// subjectDeriver produces the authenticated subject for a verified presentation.
type subjectDeriver func(*VerifiedPresentation) string

// presentationDefinition describes a single, named presentation request the verifier can serve.
// Optional fields fall back to the engine defaults supplied by the Service.
type presentationDefinition struct {
	ID string
	// stableID is the definition's id, which unlike its handle never changes or passes to another
	// definition.
	stableID      string
	DCQL          dcqlConfig
	policy        policy
	DeriveSubject subjectDeriver
	// trust holds the definition's trusted issuer certificates; nil when it has none.
	trust *trustAnchorStore
}

// dtoToDefinition converts a managed presentation definition DTO into the verifier engine's representation.
func dtoToDefinition(
	dto presentation.PresentationDefinitionDTO, clientID string, enforceKB bool,
) (*presentationDefinition, error) {
	trust, err := buildTrustStore(dto.TrustedAuthorities)
	if err != nil {
		return nil, err
	}
	allClaims := dto.RequestedClaims
	if len(allClaims) == 0 {
		allClaims = slices.Concat(dto.MandatoryClaims, dto.OptionalClaims)
	}
	enforceTrustedIssuer := false
	if dto.EnforceTrustedIssuer != nil {
		enforceTrustedIssuer = *dto.EnforceTrustedIssuer
	}
	var keyIDs []string
	if enforceTrustedIssuer && trust != nil {
		keyIDs = trust.keyIDs()
	}
	return &presentationDefinition{
		ID:       dto.Handle,
		stableID: dto.ID,
		DCQL: dcqlConfig{
			CredentialID:           dto.Handle,
			VCT:                    dto.VCT,
			Claims:                 allClaims,
			ClaimValues:            dto.ClaimValues,
			TrustedAuthorityKeyIDs: keyIDs,
		},
		policy: policy{
			ExpectedVCT:          dto.VCT,
			Audience:             clientID,
			RequestedClaims:      allClaims,
			MandatoryClaims:      dto.MandatoryClaims,
			EnforceTrustedIssuer: enforceTrustedIssuer,
			EnforceKeyBinding:    enforceKB,
			ClaimValues:          dto.ClaimValues,
		},
		DeriveSubject: subjectDeriverFor(dto.SubjectClaim),
		trust:         trust,
	}, nil
}

// dcqlConfig describes the credential query to build.
type dcqlConfig struct {
	CredentialID string
	VCT          string
	Claims       []string
	// ClaimValues maps a claim path to its allowed values (DCQL "values").
	ClaimValues map[string][]string
	// TrustedAuthorityKeyIDs holds the trust anchors' base64url SubjectKeyIds, emitted as DCQL trusted_authorities.
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
	ResultTokenValidity   time.Duration
	// VerifierInfo is attached to every signed request object (e.g. a registration certificate JWT).
	VerifierInfo      []interface{}
	EnforceKeyBinding bool
}

// Initiation is what the client needs to render the QR / deep link.
type Initiation struct {
	State      string
	ClientID   string
	RequestURI string
	// WalletURI is the pre-computed openid4vp:// deep link for the wallet.
	WalletURI string
	// ExpiresAt is when the request state expires.
	ExpiresAt time.Time
}

// Status is the lifecycle status of an OpenID4VP request.
type Status string

// Status values reported by the engine and consumed by callers (flow executor,
// RP-facing status endpoint).
const (
	StatusPending   Status = "PENDING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
	StatusExpired   Status = "EXPIRED"
)

// RequestState is the short-lived per-request state correlated by State.
type RequestState struct {
	State         string
	DefinitionID  string
	Nonce         string
	EphemeralKey  *ecdsa.PrivateKey
	ClientID      string
	RequestURI    string
	Status        Status
	Result        *VerifiedPresentation
	FailureReason string
	ExpiresAt     time.Time
	// ResultToken is a signed JWT result token, set by GetResult when Status==StatusCompleted.
	ResultToken string
}
