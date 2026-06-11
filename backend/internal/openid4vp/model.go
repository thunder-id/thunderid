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
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Sentinel errors returned by the verifier. HTTP-facing service errors live in error_constants.go.
var (
	ErrUntrustedIssuer       = errors.New("openid4vp: untrusted credential issuer")
	ErrUnexpectedVCT         = errors.New("openid4vp: unexpected credential type (vct)")
	ErrUnrequestedClaim      = errors.New("openid4vp: disclosed claim was not requested")
	ErrMissingMandatoryClaim = errors.New("openid4vp: mandatory claim missing")
	ErrInvalidPresentation   = errors.New("openid4vp: invalid presentation")
	ErrInvalidResponse       = errors.New("openid4vp: invalid authorization response")
	ErrPolicy                = errors.New("openid4vp: invalid verification policy")
	ErrUnknownState          = errors.New("openid4vp: unknown or expired request state")
	ErrExpiredState          = errors.New("openid4vp: request state expired")
	ErrStateMismatch         = errors.New("openid4vp: response state mismatch")
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
	// EnforceTrustedIssuer requires the credential issuer to be pinned in the
	// trust store and its signature to be valid. Defaults to false (skip).
	EnforceTrustedIssuer bool
	// EnforceKeyBinding requires a Key Binding JWT proving holder possession.
	// Defaults to false (skip).
	EnforceKeyBinding bool
}

// verifiedCredential is the raw output of an SD-JWT VC verification, before policy enforcement.
type verifiedCredential struct {
	Issuer         string
	VCT            string
	Claims         map[string]interface{}
	DisclosedPaths []string
}

// VerifiedPresentation is the result of a successful presentation verification.
type VerifiedPresentation struct {
	Subject        string
	Issuer         string
	VCT            string
	Claims         map[string]interface{}
	DisclosedPaths []string
}

// subjectDeriver produces the authenticated subject for a verified presentation.
type subjectDeriver func(*VerifiedPresentation) string

// presentationDefinition describes a single, named presentation request the verifier can serve.
// Optional fields fall back to the engine defaults supplied by the Service.
type presentationDefinition struct {
	ID                string
	DisplayName       string
	DCQL              dcqlConfig
	policy            policy
	Trust             *staticTrustStore
	DeriveSubject     subjectDeriver
	SubjectClaims     []string
	AttributeMapper   func(*VerifiedPresentation) map[string]interface{}
	EphemeralKeyID    string
	ResponseEncValues []string
	RequestValidity   time.Duration
	StateTTL          time.Duration
	Leeway            time.Duration
}

// registry is a thread-safe map of presentation definitions keyed by ID.
type registry struct {
	mu   sync.RWMutex
	defs map[string]*presentationDefinition
}

// newRegistry returns an empty registry.
func newRegistry() *registry {
	return &registry{defs: map[string]*presentationDefinition{}}
}

// register adds a definition. It errors when the definition is nil, has an
// empty ID, or collides with an existing entry.
func (r *registry) register(def *presentationDefinition) error {
	if def == nil {
		return fmt.Errorf("%w: presentation definition is nil", ErrPolicy)
	}
	if def.ID == "" {
		return fmt.Errorf("%w: presentation definition requires an ID", ErrPolicy)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.defs[def.ID]; exists {
		return fmt.Errorf("%w: presentation definition %q is already registered", ErrPolicy, def.ID)
	}
	r.defs[def.ID] = def
	return nil
}

// get returns the definition registered under id.
func (r *registry) get(id string) (*presentationDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.defs[id]
	return def, ok
}

// list returns the IDs of all registered definitions in lexicographic order.
func (r *registry) list() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.defs))
	for id := range r.defs {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// dcqlConfig describes the credential query to build.
type dcqlConfig struct {
	CredentialID string
	VCT          string
	Claims       []string
}

// dcqlQuery is the OpenID4VP Digital Credentials Query Language query object.
type dcqlQuery struct {
	Credentials    []dcqlCredential    `json:"credentials"`
	CredentialSets []dcqlCredentialSet `json:"credential_sets,omitempty"`
}

// dcqlCredential is a single credential query.
type dcqlCredential struct {
	ID     string      `json:"id"`
	Format string      `json:"format"`
	Meta   *dcqlMeta   `json:"meta,omitempty"`
	Claims []dcqlClaim `json:"claims,omitempty"`
}

// dcqlMeta carries format-specific matching metadata.
type dcqlMeta struct {
	VCTValues []string `json:"vct_values,omitempty"`
}

// dcqlClaim selects a single claim by its path.
type dcqlClaim struct {
	Path []interface{} `json:"path"`
}

// dcqlCredentialSet groups credential options that together satisfy the request.
type dcqlCredentialSet struct {
	Options [][]string `json:"options"`
}

// requestConfig is the static (config-driven) part of the OpenID4VP request.
type requestConfig struct {
	ClientID          string
	ClientIDScheme    string
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
// With DCQL, vp_token is a JSON object mapping each credential query id to one or more presentations.
type authorizationResponse struct {
	State         string
	Presentations map[string][]string
}

// serviceConfig is the engine-level configuration of the OpenID4VP verifier.
// Credential-specific values live on the presentationDefinition.
type serviceConfig struct {
	RequestURIBase        string
	ResponseURIBase       string
	ClientIDScheme        string
	EphemeralKeyID        string
	ResponseEncValues     []string
	RequestAudience       string
	RequestValidity       time.Duration
	TTL                   time.Duration
	Leeway                time.Duration
	KeyBindingMaxAge      time.Duration
	ResultRedirectURIBase string
	// VerifierInfo is the engine-wide verifier_attestations array attached to every signed
	// request object (e.g. a trust-framework registration certificate JWT). Optional per spec.
	VerifierInfo         []interface{}
	EnforceTrustedIssuer bool
	EnforceKeyBinding    bool
}

// Initiation is what the client needs to render the QR / deep link.
type Initiation struct {
	State      string
	ClientID   string
	RequestURI string
}

// trustedIssuer pins a trusted credential issuer's signing certificate.
type trustedIssuer struct {
	Issuer string
	// CertFile is a path to a PEM CERTIFICATE or PUBLIC KEY, resolved before passing.
	CertFile string
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
