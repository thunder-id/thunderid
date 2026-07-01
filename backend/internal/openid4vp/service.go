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
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/openid4vp/definition"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// definitionReader resolves a managed presentation definition by handle.
type definitionReader interface {
	GetPresentationDefinitionByHandle(
		ctx context.Context, handle string,
	) (*definition.PresentationDefinitionDTO, *tidcommon.ServiceError)
}

const defaultStateTTL = 5 * time.Minute

// requestSigner signs the OpenID4VP request object (JAR) claims into a compact
// JWS using the verifier's registered key and x5c header.
type requestSigner interface {
	signRequestObject(ctx context.Context, claims map[string]interface{}) (string, error)
}

// OpenID4VPServiceInterface is the contract for the OpenID4VP verifier service.
// It is implemented by *service and consumed by the HTTP handler and in-process
// callers such as flow executors (which depend only on Initiate and GetResult).
type OpenID4VPServiceInterface interface {
	Initiate(ctx context.Context, definitionID string) (*Initiation, *tidcommon.ServiceError)
	GetResult(ctx context.Context, state string) (*RequestState, *tidcommon.ServiceError)
	GetRequestObject(ctx context.Context, state string) (string, *tidcommon.ServiceError)
	SubmitResponse(ctx context.Context, state string, body []byte) (*VerifiedPresentation, *tidcommon.ServiceError)
	SubmitError(ctx context.Context, state, code, description string) *tidcommon.ServiceError
	GetResultRedirectURI(state string) string
	InitiateForRP(ctx context.Context, definitionID, rpID string) (*Initiation, *tidcommon.ServiceError)
	LookupState(ctx context.Context, state string) (*RequestState, *tidcommon.ServiceError)
	Authenticate(ctx context.Context, state string) (*authncommon.AuthnResult, *tidcommon.ServiceError)
	GetTrustAnchors() []TrustAnchorInfo
}

var _ OpenID4VPServiceInterface = (*service)(nil)

// service drives the OpenID4VP verifier: issues signed requests, verifies encrypted responses.
type service struct {
	cfg      serviceConfig
	store    openID4VPStoreInterface
	defStore definitionReader
	clientID string
	signer   requestSigner
	// trust is the shared engine-wide trust anchor store; nil when unconfigured.
	trust *trustAnchorStore
}

// newOpenID4VPService creates an OpenID4VP verifier engine.
func newOpenID4VPService(
	cfg serviceConfig, store openID4VPStoreInterface, clientID string, signer requestSigner,
	trust *trustAnchorStore, defStore definitionReader,
) (OpenID4VPServiceInterface, error) {
	if store == nil || clientID == "" || signer == nil || defStore == nil {
		return nil, fmt.Errorf("%w: store, client id, signer and definition store are required", ErrPolicy)
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
		defStore: defStore,
		clientID: clientID,
		signer:   signer,
		trust:    trust,
	}, nil
}

// InitiateForRP is the RP-facing variant of Initiate; stores rpID so the result token audience matches.
func (s *service) InitiateForRP(
	ctx context.Context, definitionID, rpID string,
) (*Initiation, *tidcommon.ServiceError) {
	init, err := s.initiateForRP(ctx, definitionID, rpID)
	if err != nil {
		return nil, toServiceError(err)
	}
	return init, nil
}

// initiateForRP initiates a request for definitionID and records rpID on the resulting request state.
func (s *service) initiateForRP(ctx context.Context, definitionID, rpID string) (*Initiation, error) {
	init, err := s.initiate(ctx, definitionID)
	if err != nil {
		return nil, err
	}
	if rpID == "" {
		return init, nil
	}
	rs, ok := s.store.GetRequestState(ctx, init.State)
	if !ok || rs == nil {
		return init, nil
	}
	rs.RPID = rpID
	if err := s.store.SaveRequestState(ctx, rs); err != nil {
		return nil, fmt.Errorf("failed to persist rp_id on request state: %w", err)
	}
	return init, nil
}

// Initiate creates a fresh request for definitionID: a random state and nonce
// and an ephemeral encryption keypair, stored pending under a short TTL.
func (s *service) Initiate(ctx context.Context, definitionID string) (*Initiation, *tidcommon.ServiceError) {
	init, err := s.initiate(ctx, definitionID)
	if err != nil {
		return nil, toServiceError(err)
	}
	return init, nil
}

// initiate creates a fresh request for definitionID with a random state, nonce, and ephemeral key, stored pending.
func (s *service) initiate(ctx context.Context, definitionID string) (*Initiation, error) {
	def, err := s.resolveDefinition(ctx, definitionID)
	if err != nil {
		return nil, err
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
	if err := s.store.SaveRequestState(ctx, rs); err != nil {
		return nil, fmt.Errorf("failed to store request state: %w", err)
	}

	return &Initiation{
		State:      state,
		ClientID:   s.clientID,
		RequestURI: s.requestURI(state),
	}, nil
}

// GetRequestObject builds and signs the request object (JAR) for state. The
// definition recorded on the state drives DCQL, encryption advertisement and
// signing.
func (s *service) GetRequestObject(ctx context.Context, state string) (string, *tidcommon.ServiceError) {
	jar, err := s.requestObject(ctx, state)
	if err != nil {
		return "", toServiceError(err)
	}
	return jar, nil
}

// requestObject builds and signs the request object (JAR) for the given state.
func (s *service) requestObject(ctx context.Context, state string) (string, error) {
	rs, err := s.load(ctx, state)
	if err != nil {
		return "", err
	}
	def, err := s.resolveDefinition(ctx, rs.DefinitionID)
	if err != nil {
		return "", err
	}

	dcql := def.DCQL
	if s.trust != nil {
		dcql.TrustedAuthorityKeyIDs = s.trust.skisFor(dcql.TrustedAuthorities)
	}

	cfg := requestConfig{
		ClientID:          s.clientID,
		ResponseURI:       s.responseURI(state),
		ResponseMode:      ResponseModeDirectPostJWT,
		Audience:          s.cfg.RequestAudience,
		Validity:          s.cfg.RequestValidity,
		DCQL:              dcql,
		ResponseEncValues: s.cfg.ResponseEncValues,
		VerifierInfo:      s.cfg.VerifierInfo,
	}
	params := requestParams{
		Nonce:          rs.Nonce,
		State:          state,
		EphemeralKey:   &rs.EphemeralKey.PublicKey,
		EphemeralKeyID: s.cfg.EphemeralKeyID,
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
func (s *service) SubmitResponse(
	ctx context.Context, state string, body []byte,
) (*VerifiedPresentation, *tidcommon.ServiceError) {
	vp, err := s.submitResponse(ctx, state, body)
	if err != nil {
		return nil, toServiceError(err)
	}
	return vp, nil
}

// submitResponse decrypts and verifies the wallet response body for state, recording the outcome.
func (s *service) submitResponse(ctx context.Context, state string, body []byte) (*VerifiedPresentation, error) {
	rs, err := s.load(ctx, state)
	if err != nil {
		return nil, err
	}
	def, err := s.resolveDefinition(ctx, rs.DefinitionID)
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
	candidates, err := resp.presentationsFor(def.DCQL.CredentialID)
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

	// A holder may return several matching credentials for a single query; accept
	// the first that verifies and satisfies the policy, otherwise report the last error.
	var vp *VerifiedPresentation
	var lastErr error
	for _, presentation := range candidates {
		cred, verr := verifySDJWTPresentation(
			presentation, s.trust, policy.Audience, rs.Nonce, policy.Leeway, policy.KeyBindingMaxAge,
			policy.EnforceTrustedIssuer, policy.EnforceKeyBinding, policy.TrustedAuthorities)
		if verr != nil {
			lastErr = verr
			continue
		}
		candidate, verr := finalizePresentation(cred, policy)
		if verr != nil {
			lastErr = verr
			continue
		}
		vp = candidate
		break
	}
	if vp == nil {
		return nil, s.fail(ctx, rs, lastErr)
	}
	if def.DeriveSubject != nil {
		if subject := def.DeriveSubject(vp); subject != "" {
			vp.Subject = subject
		}
	}

	rs.Status = StatusCompleted
	rs.Result = vp
	if err := s.store.SaveRequestState(ctx, rs); err != nil {
		return nil, fmt.Errorf("failed to persist verification result: %w", err)
	}
	return vp, nil
}

// SubmitError records a wallet-reported error (e.g. access_denied) and marks the transaction failed.
func (s *service) SubmitError(ctx context.Context, state, code, description string) *tidcommon.ServiceError {
	rs, err := s.load(ctx, state)
	if err != nil {
		return toServiceError(err)
	}
	reason := code
	if description != "" {
		reason = code + ": " + description
	}
	_ = s.fail(ctx, rs, fmt.Errorf("wallet reported error: %s", reason))
	return nil
}

// GetResult returns the current state record for polling.
func (s *service) GetResult(ctx context.Context, state string) (*RequestState, *tidcommon.ServiceError) {
	rs, err := s.load(ctx, state)
	if err != nil {
		return nil, toServiceError(err)
	}
	return rs, nil
}

// LookupState returns the state record without evicting expired entries; returns ErrExpiredState vs ErrUnknownState.
func (s *service) LookupState(ctx context.Context, state string) (*RequestState, *tidcommon.ServiceError) {
	rs, ok := s.store.GetRequestState(ctx, state)
	if !ok || rs == nil {
		return nil, toServiceError(ErrUnknownState)
	}
	if time.Now().After(rs.ExpiresAt) {
		return rs, toServiceError(ErrExpiredState)
	}
	return rs, nil
}

// Authenticate retrieves the completed verified presentation and converts it to an AuthnResult.
func (s *service) Authenticate(ctx context.Context, state string) (
	*authncommon.AuthnResult, *tidcommon.ServiceError) {
	rs, svcErr := s.GetResult(ctx, state)
	if svcErr != nil {
		return nil, svcErr
	}
	if rs.Status != StatusCompleted {
		return nil, &ErrorVerificationFailed
	}
	vp := rs.Result
	if vp == nil {
		return nil, &tidcommon.InternalServerError
	}

	claims := make(map[string]interface{}, len(vp.Claims)+3)
	for k, v := range vp.Claims {
		claims[k] = v
	}
	claims["openid4vp_issuer"] = vp.Issuer
	claims["openid4vp_vct"] = vp.VCT
	if vp.Subject != "" {
		claims["sub"] = vp.Subject
	}

	// Identification must key on the stable subject only, like the OIDC executor.
	// Passing the full claim set (disclosed attributes, holder key-binding material
	// under cnf.*, and openid4vp_* metadata) makes IdentifyEntity AND over fields
	// no user carries, so a returning holder never matches. AuthenticatedClaims
	// still carries the full set for attribute provisioning and runtime data.
	identification := map[string]interface{}{}
	if vp.Subject != "" {
		identification["sub"] = vp.Subject
	}

	return &authncommon.AuthnResult{
		Token:               identification,
		AuthenticatedClaims: claims,
	}, nil
}

// GetTrustAnchors returns the configured trust anchors (root CAs).
func (s *service) GetTrustAnchors() []TrustAnchorInfo {
	if s.trust == nil {
		return []TrustAnchorInfo{}
	}
	anchors := s.trust.list()
	out := make([]TrustAnchorInfo, 0, len(anchors))
	for _, a := range anchors {
		out = append(out, TrustAnchorInfo{
			Name:     a.Name,
			Subject:  a.Subject,
			SKI:      a.SKI,
			NotAfter: a.NotAfter,
		})
	}
	return out
}

// resolveDefinition fetches the presentation definition by handle and builds its engine representation.
func (s *service) resolveDefinition(ctx context.Context, definitionID string) (*presentationDefinition, error) {
	dto, svcErr := s.defStore.GetPresentationDefinitionByHandle(ctx, definitionID)
	if svcErr != nil {
		if svcErr.Code == definition.ErrorDefinitionNotFound.Code {
			return nil, fmt.Errorf("%w: no presentation definition registered for %q",
				ErrUnknownDefinition, definitionID)
		}
		return nil, fmt.Errorf("failed to resolve presentation definition %q", definitionID)
	}
	return dtoToDefinition(*dto, s.clientID, s.cfg.EnforceKeyBinding), nil
}

// load fetches non-expired state, deleting and rejecting expired entries.
func (s *service) load(ctx context.Context, state string) (*RequestState, error) {
	rs, ok := s.store.GetRequestState(ctx, state)
	if !ok || rs == nil {
		return nil, ErrUnknownState
	}
	if time.Now().After(rs.ExpiresAt) {
		_ = s.store.DeleteRequestState(ctx, state)
		return nil, ErrUnknownState
	}
	return rs, nil
}

// fail records a verification failure and returns the wrapped reason.
func (s *service) fail(ctx context.Context, rs *RequestState, reason error) error {
	rs.Status = StatusFailed
	rs.FailureReason = reason.Error()
	_ = s.store.SaveRequestState(ctx, rs)
	return reason
}

// GetResultRedirectURI returns the URL the wallet should follow after posting its
// response, or an empty string when none is configured.
func (s *service) GetResultRedirectURI(state string) string {
	if s.cfg.ResultRedirectURIBase == "" {
		return ""
	}
	return withState(s.cfg.ResultRedirectURIBase, state)
}

// requestURI builds the request URI for the given state from the configured base.
func (s *service) requestURI(state string) string {
	return withState(s.cfg.RequestURIBase, state)
}

// responseURI builds the response URI for the given state from the configured base.
func (s *service) responseURI(state string) string {
	return withState(s.cfg.ResponseURIBase, state)
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

// OpenID4VP request object constants.
const (
	ResponseTypeVPToken       = "vp_token"
	ResponseModeDirectPostJWT = "direct_post.jwt"
	DefaultResponseEncValue   = "A128GCM"
	defaultRequestValidity    = 5 * time.Minute
)

// buildRequestObject assembles the OpenID4VP signed-request (JAR) claims.
func buildRequestObject(cfg requestConfig, params requestParams) (map[string]interface{}, error) {
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("%w: client_id is required", ErrPolicy)
	}
	if cfg.ResponseURI == "" {
		return nil, fmt.Errorf("%w: response_uri is required", ErrPolicy)
	}
	if params.Nonce == "" || params.State == "" {
		return nil, fmt.Errorf("%w: nonce and state are required", ErrPolicy)
	}
	if params.EphemeralKey == nil {
		return nil, fmt.Errorf("%w: ephemeral encryption key is required", ErrPolicy)
	}

	clientMetadata, err := buildClientMetadata(cfg, params)
	if err != nil {
		return nil, err
	}

	query, err := buildQuery(cfg.DCQL)
	if err != nil {
		return nil, err
	}

	validity := cfg.Validity
	if validity == 0 {
		validity = defaultRequestValidity
	}
	responseMode := cfg.ResponseMode
	if responseMode == "" {
		responseMode = ResponseModeDirectPostJWT
	}
	iat := params.IssuedAt
	if iat.IsZero() {
		iat = time.Now()
	}

	request := map[string]interface{}{
		"iss":             cfg.ClientID,
		"response_type":   ResponseTypeVPToken,
		"response_mode":   responseMode,
		"client_id":       cfg.ClientID,
		"response_uri":    cfg.ResponseURI,
		"nonce":           params.Nonce,
		"state":           params.State,
		"iat":             iat.Unix(),
		"exp":             iat.Add(validity).Unix(),
		"dcql_query":      query,
		"client_metadata": clientMetadata,
	}
	// Omit SIOP audience for a pure vp_token request; some wallets treat it as SIOP if present.
	if cfg.Audience != "" {
		request["aud"] = cfg.Audience
	}
	if len(cfg.VerifierInfo) > 0 {
		request["verifier_info"] = cfg.VerifierInfo
	}

	return request, nil
}

// buildClientMetadata advertises the ephemeral encryption key and supported response enc algorithms.
func buildClientMetadata(cfg requestConfig, params requestParams) (map[string]interface{}, error) {
	jwk, err := ecdsaPublicKeyToEncJWK(params.EphemeralKey, params.EphemeralKeyID)
	if err != nil {
		return nil, err
	}

	encValues := cfg.ResponseEncValues
	if len(encValues) == 0 {
		encValues = []string{DefaultResponseEncValue}
	}

	vpFormats := map[string]interface{}{
		FormatSDJWTVC: map[string]interface{}{
			"kb-jwt_alg_values": []string{"ES256", "EdDSA"},
			"sd-jwt_alg_values": []string{"ES256", "EdDSA"},
		},
	}

	return map[string]interface{}{
		"jwks": map[string]interface{}{
			"keys": []interface{}{jwk},
		},
		"vp_formats_supported":                    vpFormats,
		"encrypted_response_enc_values_supported": encValues,
	}, nil
}

// ecdsaPublicKeyToEncJWK encodes an EC public key as an encryption-use JWK.
func ecdsaPublicKeyToEncJWK(pub *ecdsa.PublicKey, kid string) (map[string]interface{}, error) {
	raw, err := pub.Bytes()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPolicy, err)
	}

	var crv string
	var coordLen int
	switch len(raw) {
	case 65:
		crv, coordLen = "P-256", 32
	case 97:
		crv, coordLen = "P-384", 48
	case 133:
		crv, coordLen = "P-521", 66
	default:
		return nil, fmt.Errorf("%w: unsupported EC public key length %d", ErrPolicy, len(raw))
	}

	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": crv,
		"x":   base64.RawURLEncoding.EncodeToString(raw[1 : 1+coordLen]),
		"y":   base64.RawURLEncoding.EncodeToString(raw[1+coordLen:]),
		"use": "enc",
		"alg": "ECDH-ES",
	}
	if kid != "" {
		jwk["kid"] = kid
	}
	return jwk, nil
}

// ParseAuthorizationResponse parses the decrypted OpenID4VP response body.
func parseAuthorizationResponse(body []byte) (*authorizationResponse, error) {
	var raw struct {
		State   string          `json:"state"`
		VPToken json.RawMessage `json:"vp_token"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidResponse, err)
	}
	if len(raw.VPToken) == 0 {
		return nil, fmt.Errorf("%w: missing vp_token", ErrInvalidResponse)
	}

	// vp_token is a DCQL object mapping each credential query id to its presentation(s).
	var byID map[string]json.RawMessage
	if err := json.Unmarshal(raw.VPToken, &byID); err != nil {
		return nil, fmt.Errorf("%w: vp_token must be a DCQL object keyed by credential id: %w", ErrInvalidResponse, err)
	}
	presentations := make(map[string][]string, len(byID))
	for id, val := range byID {
		list, err := decodePresentationValue(val)
		if err != nil {
			return nil, fmt.Errorf("%w: credential %q: %w", ErrInvalidResponse, id, err)
		}
		presentations[id] = list
	}
	return &authorizationResponse{State: raw.State, Presentations: presentations}, nil
}

// presentationsFor returns the presentations the wallet supplied for credentialID;
// errors if none. A single credential query may yield several when the holder owns
// multiple matching credentials; the caller verifies each and accepts the first
// that satisfies the policy.
func (r *authorizationResponse) presentationsFor(credentialID string) ([]string, error) {
	list, ok := r.Presentations[credentialID]
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("%w: no presentation for credential %q", ErrInvalidResponse, credentialID)
	}
	return list, nil
}

// decodePresentationValue accepts either a single presentation string or an
// array of them.
func decodePresentationValue(raw json.RawMessage) ([]string, error) {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}
	return nil, fmt.Errorf("presentation must be a string or array of strings")
}

// resultTokenIssuer issues a signed result token for a completed verification.
type resultTokenIssuer interface {
	issueResultToken(ctx context.Context, rpID string, rs *RequestState, validitySeconds int64) (string, error)
}

// jwtResultTokenIssuer signs result tokens with the server's JWT service so
// the token is verifiable against Thunder's published JWKS.
type jwtResultTokenIssuer struct {
	jwt      jwt.JWTServiceInterface
	issuer   string
	clientID string
}

// newJWTresultTokenIssuer constructs a JWT-based result token issuer with the given issuer and client ID.
func newJWTresultTokenIssuer(svc jwt.JWTServiceInterface, issuer, clientID string) resultTokenIssuer {
	return &jwtResultTokenIssuer{jwt: svc, issuer: issuer, clientID: clientID}
}

// issueResultToken issues a signed JWT result token for a completed verification addressed to rpID.
func (i *jwtResultTokenIssuer) issueResultToken(
	ctx context.Context, rpID string, rs *RequestState, validitySeconds int64,
) (string, error) {
	if rs == nil {
		return "", fmt.Errorf("%w: request state is required to issue a result token", ErrPolicy)
	}
	if rs.Status != StatusCompleted || rs.Result == nil {
		return "", fmt.Errorf("%w: result token can only be issued for completed verifications", ErrPolicy)
	}
	if i.jwt == nil {
		return "", fmt.Errorf("%w: jwt service is not configured", ErrPolicy)
	}

	claims := map[string]interface{}{
		"aud":             rpID,
		"txn":             rs.State,
		"definition_id":   rs.DefinitionID,
		"subject":         rs.Result.Subject,
		"verified_claims": rs.Result.Claims,
		"verifier":        i.clientID,
	}

	token, _, svcErr := i.jwt.GenerateJWT(ctx, rs.Result.Subject, i.issuer, validitySeconds, claims, "JWT", "")
	if svcErr != nil {
		return "", fmt.Errorf("failed to sign result token: %s", svcErr.Code)
	}
	return token, nil
}
