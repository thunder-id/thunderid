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

package openid4vp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"time"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
)

const defaultStateTTL = 5 * time.Minute

// requestSigner signs the OpenID4VP request object (JAR) claims into a compact
// JWS using the verifier's registered key and x5c header.
type requestSigner interface {
	signRequestObject(ctx context.Context, claims map[string]interface{}) (string, error)
}

// OpenID4VPServiceInterface is the contract for the OpenID4VP verifier service.
// It is implemented by *service and consumed by the HTTP handler and in-process
// callers such as flow executors (which depend only on Initiate and Result).
type OpenID4VPServiceInterface interface {
	Initiate(ctx context.Context, definitionID string) (*Initiation, error)
	Result(ctx context.Context, state string) (*RequestState, error)
	RequestObject(ctx context.Context, state string) (string, error)
	SubmitResponse(ctx context.Context, state string, body []byte) (*VerifiedPresentation, error)
	ResultRedirectURI(state string) string
	InitiateForRP(ctx context.Context, definitionID, rpID string) (*Initiation, error)
	LookupState(ctx context.Context, state string) (*RequestState, error)
	Authenticate(ctx context.Context, state string) (*authncommon.AuthnResult, *serviceerror.ServiceError)
}

var _ OpenID4VPServiceInterface = (*service)(nil)

// service drives the OpenID4VP verifier: it issues signed requests with a
// fresh nonce and ephemeral encryption key, and verifies the encrypted
// responses. Per-credential behavior is plugged in via the registry.
type service struct {
	cfg      serviceConfig
	store    stateStore
	registry *registry
	clientID string
	signer   requestSigner
}

// newService creates an OpenID4VP verifier engine. clientID is the verifier
// identifier (e.g. "x509_hash:<hash>") and signer signs the JAR request object.
func newService(
	cfg serviceConfig, store stateStore, clientID string, signer requestSigner,
) (*service, error) {
	if store == nil || clientID == "" || signer == nil {
		return nil, fmt.Errorf("%w: store, client id and signer are required", ErrPolicy)
	}
	if cfg.RequestURIBase == "" || cfg.ResponseURIBase == "" {
		return nil, fmt.Errorf("%w: request_uri and response_uri base URLs are required", ErrPolicy)
	}
	if cfg.TTL == 0 {
		cfg.TTL = defaultStateTTL
	}
	return &service{
		cfg:      cfg,
		store:    store,
		registry: newRegistry(),
		clientID: clientID,
		signer:   signer,
	}, nil
}

// InitiateForRP is the RP-facing variant of Initiate: it stores the calling
// relying party's identifier on the state so the result-token audience matches
// the RP that initiated the transaction.
func (s *service) InitiateForRP(ctx context.Context, definitionID, rpID string) (*Initiation, error) {
	init, err := s.Initiate(ctx, definitionID)
	if err != nil {
		return nil, err
	}
	if rpID == "" {
		return init, nil
	}
	rs, ok := s.store.Get(ctx, init.State)
	if !ok || rs == nil {
		return init, nil
	}
	rs.RPID = rpID
	if err := s.store.Save(ctx, rs); err != nil {
		return nil, fmt.Errorf("failed to persist rp_id on request state: %w", err)
	}
	return init, nil
}

// Initiate creates a fresh request for definitionID: a random state and nonce
// and an ephemeral encryption keypair, stored pending under a short TTL.
func (s *service) Initiate(ctx context.Context, definitionID string) (*Initiation, error) {
	def, ok := s.registry.get(definitionID)
	if !ok {
		return nil, fmt.Errorf("%w: no presentation definition registered for %q", ErrPolicy, definitionID)
	}

	state, err := randomToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}
	nonce, err := randomToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	rs := &RequestState{
		State:        state,
		DefinitionID: def.ID,
		Nonce:        nonce,
		EphemeralKey: key,
		ClientID:     s.clientID,
		RequestURI:   s.requestURI(state),
		Status:       StatusPending,
		ExpiresAt:    time.Now().Add(s.cfg.TTL),
	}
	if err := s.store.Save(ctx, rs); err != nil {
		return nil, fmt.Errorf("failed to store request state: %w", err)
	}

	return &Initiation{
		State:      state,
		ClientID:   s.clientID,
		RequestURI: s.requestURI(state),
	}, nil
}

// RequestObject builds and signs the request object (JAR) for state. The
// definition recorded on the state drives DCQL, encryption advertisement and
// signing.
func (s *service) RequestObject(ctx context.Context, state string) (string, error) {
	rs, err := s.load(ctx, state)
	if err != nil {
		return "", err
	}
	def, err := s.resolveDefinition(rs)
	if err != nil {
		return "", err
	}

	cfg := requestConfig{
		ClientID:          s.clientID,
		ClientIDScheme:    s.cfg.ClientIDScheme,
		ResponseURI:       s.responseURI(state),
		ResponseMode:      ResponseModeDirectPostJWT,
		Audience:          s.cfg.RequestAudience,
		Validity:          firstNonZeroDuration(def.RequestValidity, s.cfg.RequestValidity),
		DCQL:              def.DCQL,
		ResponseEncValues: firstNonEmptyStringSlice(def.ResponseEncValues, s.cfg.ResponseEncValues),
		VerifierInfo:      s.cfg.VerifierInfo,
	}
	params := requestParams{
		Nonce:          rs.Nonce,
		State:          state,
		EphemeralKey:   &rs.EphemeralKey.PublicKey,
		EphemeralKeyID: firstNonEmptyString(def.EphemeralKeyID, s.cfg.EphemeralKeyID),
		IssuedAt:       time.Now(),
	}

	claims, err := buildRequestObject(cfg, params)
	if err != nil {
		return "", err
	}
	return s.signer.signRequestObject(ctx, claims)
}

// SubmitResponse decrypts and verifies a wallet response for state, recording
// the outcome. It returns the verified presentation on success.
func (s *service) SubmitResponse(ctx context.Context, state string, body []byte) (*VerifiedPresentation, error) {
	rs, err := s.load(ctx, state)
	if err != nil {
		return nil, err
	}
	def, err := s.resolveDefinition(rs)
	if err != nil {
		return nil, err
	}

	if rs.EphemeralKey == nil {
		return nil, s.fail(ctx, rs,
			fmt.Errorf("%w: ephemeral key not available for decryption", ErrInvalidResponse))
	}
	plaintext, err := jwe.DecryptWithKey(string(body), rs.EphemeralKey)
	if err != nil {
		return nil, s.fail(ctx, rs, fmt.Errorf("%w: decryption failed: %w", ErrInvalidResponse, err))
	}

	resp, err := parseAuthorizationResponse(plaintext)
	if err != nil {
		return nil, s.fail(ctx, rs, err)
	}
	if resp.State != "" && resp.State != state {
		return nil, s.fail(ctx, rs, ErrStateMismatch)
	}
	presentation, err := resp.presentation(def.DCQL.CredentialID)
	if err != nil {
		return nil, s.fail(ctx, rs, err)
	}

	policy := def.policy
	if policy.Leeway == 0 {
		policy.Leeway = s.cfg.Leeway
	}
	if policy.KeyBindingMaxAge == 0 {
		policy.KeyBindingMaxAge = s.cfg.KeyBindingMaxAge
	}

	cred, err := verifySDJWTPresentation(
		ctx, presentation, def.Trust, policy.Audience, rs.Nonce, policy.Leeway, policy.KeyBindingMaxAge,
		policy.EnforceTrustedIssuer, policy.EnforceKeyBinding)
	if err != nil {
		return nil, s.fail(ctx, rs, err)
	}
	vp, err := finalizePresentation(cred, policy)
	if err != nil {
		return nil, s.fail(ctx, rs, err)
	}
	if def.DeriveSubject != nil {
		if subject := def.DeriveSubject(vp); subject != "" {
			vp.Subject = subject
		}
	}

	rs.Status = StatusCompleted
	rs.Result = vp
	if err := s.store.Save(ctx, rs); err != nil {
		return nil, fmt.Errorf("failed to persist verification result: %w", err)
	}
	return vp, nil
}

// Result returns the current state record for polling. It returns
// ErrUnknownState when the state is unknown or expired.
func (s *service) Result(ctx context.Context, state string) (*RequestState, error) {
	return s.load(ctx, state)
}

// LookupState returns the raw state record without auto-evicting expired
// entries. Expired entries are returned with ErrExpiredState so callers (the
// RP-facing status endpoint) can report EXPIRED distinctly from a truly
// unknown state. Missing entries return ErrUnknownState.
func (s *service) LookupState(ctx context.Context, state string) (*RequestState, error) {
	rs, ok := s.store.Get(ctx, state)
	if !ok || rs == nil {
		return nil, ErrUnknownState
	}
	if time.Now().After(rs.ExpiresAt) {
		return rs, ErrExpiredState
	}
	return rs, nil
}

// Authenticate retrieves the completed verified presentation for the given state
// and converts it to an AuthnResult for use by the authn provider chain.
// It returns an error when the state is unknown, expired, or the presentation
// has not yet reached StatusCompleted.
func (s *service) Authenticate(ctx context.Context, state string) (
	*authncommon.AuthnResult, *serviceerror.ServiceError) {
	rs, err := s.Result(ctx, state)
	if err != nil {
		return nil, &ErrorUnknownState
	}
	if rs.Status != StatusCompleted {
		return nil, &ErrorVerificationFailed
	}
	vp := rs.Result
	if vp == nil {
		return nil, &serviceerror.InternalServerError
	}

	token := make(map[string]interface{}, len(vp.Claims)+3)
	for k, v := range vp.Claims {
		token[k] = v
	}
	token["openid4vp_issuer"] = vp.Issuer
	token["openid4vp_vct"] = vp.VCT
	if vp.Subject != "" {
		token["sub"] = vp.Subject
	}

	return &authncommon.AuthnResult{
		Token:               token,
		AuthenticatedClaims: token,
	}, nil
}

// ResultRedirectURIBase returns the engine-configured result-redirect base URL
// (empty when none is configured).
func (s *service) resultRedirectURIBase() string { return s.cfg.ResultRedirectURIBase }

// resolveDefinition resolves the definition pinned on the state, erroring when
// the registry no longer carries it.
func (s *service) resolveDefinition(rs *RequestState) (*presentationDefinition, error) {
	def, ok := s.registry.get(rs.DefinitionID)
	if !ok {
		return nil, fmt.Errorf("%w: presentation definition %q no longer registered", ErrPolicy, rs.DefinitionID)
	}
	return def, nil
}

// load fetches non-expired state, deleting and rejecting expired entries.
func (s *service) load(ctx context.Context, state string) (*RequestState, error) {
	rs, ok := s.store.Get(ctx, state)
	if !ok || rs == nil {
		return nil, ErrUnknownState
	}
	if time.Now().After(rs.ExpiresAt) {
		_ = s.store.Delete(ctx, state)
		return nil, ErrUnknownState
	}
	return rs, nil
}

// fail records a verification failure and returns the wrapped reason.
func (s *service) fail(ctx context.Context, rs *RequestState, reason error) error {
	rs.Status = StatusFailed
	rs.FailureReason = reason.Error()
	_ = s.store.Save(ctx, rs)
	return reason
}

// ResultRedirectURI returns the URL the wallet should follow after posting its
// response, or an empty string when none is configured.
func (s *service) ResultRedirectURI(state string) string {
	if s.cfg.ResultRedirectURIBase == "" {
		return ""
	}
	return withState(s.cfg.ResultRedirectURIBase, state)
}

func (s *service) requestURI(state string) string {
	return withState(s.cfg.RequestURIBase, state)
}

func (s *service) responseURI(state string) string {
	return withState(s.cfg.ResponseURIBase, state)
}

// withState appends the state query parameter to a base URL.
func withState(base, state string) string {
	sep := "?"
	if u, err := url.Parse(base); err == nil && u.RawQuery != "" {
		sep = "&"
	}
	return base + sep + "state=" + url.QueryEscape(state)
}

// randomToken returns 32 cryptographically random bytes, base64url-encoded.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// WalletAuthorizationURI builds the openid4vp:// deep link the wallet scans,
// carrying the client_id and request_uri.
func WalletAuthorizationURI(clientID, requestURI string) string {
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("request_uri", requestURI)
	v.Set("request_uri_method", "get")
	return "openid4vp://?" + v.Encode()
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonEmptyStringSlice(values ...[]string) []string {
	for _, v := range values {
		if len(v) > 0 {
			return v
		}
	}
	return nil
}

func firstNonZeroDuration(values ...time.Duration) time.Duration {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}
