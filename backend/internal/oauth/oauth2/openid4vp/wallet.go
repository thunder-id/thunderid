// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"context"
	"crypto/ecdsa"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// walletInterface is the internal interface consumed by the wallet-facing HTTP handler.
type walletInterface interface {
	GetRequestObject(ctx context.Context, state string) (string, *tidcommon.ServiceError)
	SubmitResponse(ctx context.Context, state string, body []byte) (
		*VerifiedPresentation, string, *tidcommon.ServiceError,
	)
	SubmitError(ctx context.Context, state, code, description string) *tidcommon.ServiceError
}

// OpenID4VP protocol constants.
const (
	ResponseTypeVPToken       = "vp_token"
	ResponseModeDirectPostJWT = "direct_post.jwt"
	DefaultResponseEncValue   = "A128GCM"
	defaultRequestValidity    = 5 * time.Minute
	requestObjectType         = "oauth-authz-req+jwt"
	formatSDJWTVC             = "dc+sd-jwt"
	formatLDProofVC           = "dc+ldp"
)

// GetRequestObject builds and signs the request object (JAR) for state.
func (s *openid4vpService) GetRequestObject(ctx context.Context, state string) (string, *tidcommon.ServiceError) {
	rs, err := s.load(ctx, state)
	if err != nil {
		return "", toServiceError(err)
	}

	// Only allow the request object to be generated once per state. The wallet may retry the request
	// object fetch, but the request object itself is single-use and must be generated only once.
	if rs.Status != StatusPending {
		return "", &ErrorUnknownState
	}
	if rs.EphemeralKey == nil {
		return "", &tidcommon.InternalServerError
	}
	verifierClientConfig, err := s.resolveVerifierClientConfig(ctx, rs.VerifierID)
	if err != nil {
		return "", toServiceError(err)
	}

	claims, err := buildRequestObject(requestConfig{
		ClientID:          rs.ClientID,
		ResponseURI:       s.responseURI(state),
		ResponseMode:      ResponseModeDirectPostJWT,
		Audience:          s.cfg.RequestAudience,
		Validity:          time.Duration(s.cfg.RequestValiditySeconds) * time.Second,
		DCQL:              verifierClientConfig.DCQL,
		ResponseEncValues: s.cfg.ResponseEncValues,
		VerifierInfo:      verifierClientConfig.VerifierInfo,
	}, requestParams{
		Nonce:        rs.Nonce,
		State:        state,
		EphemeralKey: &rs.EphemeralKey.PublicKey,
		IssuedAt:     time.Now(),
	})
	if err != nil {
		return "", toServiceError(err)
	}
	jar, err := s.signRequestObject(ctx, claims)
	if err != nil {
		return "", toServiceError(err)
	}

	rs.Status = StatusStarted
	swapped, err := s.store.CompareAndSwapRequestState(ctx, rs, StatusPending)
	if err != nil {
		s.logger.Error(ctx, "Failed to mark OpenID4VP request as started",
			log.String("state", state), log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	if !swapped {
		return "", &ErrorUnknownState
	}
	return jar, nil
}

// SubmitResponse decrypts and verifies a wallet VP response, recording the outcome.
func (s *openid4vpService) SubmitResponse(
	ctx context.Context, state string, body []byte,
) (*VerifiedPresentation, string, *tidcommon.ServiceError) {
	rs, err := s.load(ctx, state)
	if err != nil {
		return nil, "", toServiceError(err)
	}
	verifierClientConfig, err := s.resolveVerifierClientConfig(ctx, rs.VerifierID)
	if err != nil {
		return nil, "", toServiceError(err)
	}

	if rs.EphemeralKey == nil {
		return nil, "", toServiceError(s.fail(ctx, rs,
			fmt.Errorf("%w: ephemeral key not available for decryption", ErrInvalidResponse)))
	}
	plaintext, err := jwe.DecryptWithKey(string(body), rs.EphemeralKey)
	if err != nil {
		return nil, "", toServiceError(s.fail(ctx, rs,
			fmt.Errorf("%w: decryption failed: %w", ErrInvalidResponse, err)))
	}

	resp, err := parseAuthorizationResponse(plaintext)
	if err != nil {
		return nil, "", toServiceError(s.fail(ctx, rs, err))
	}
	if resp.State != "" && resp.State != state {
		return nil, "", toServiceError(s.fail(ctx, rs, ErrStateMismatch))
	}

	var vp *VerifiedPresentation
	var validatedPresentation string
	var lastErr error

	// NOTE : As of now consdering only the first credential
	credentialQuery := verifierClientConfig.DCQL.Credentials[0]

	presentationCandidates, err := resp.presentationsFor(credentialQuery.ID)
	if err != nil {
		return nil, "", toServiceError(s.fail(ctx, rs, err))
	}

	vcVerifier, err := s.verifierRegistry.GetVerifier(credentialQuery.Format)
	if err != nil {
		return nil, "", toServiceError(s.fail(ctx, rs, fmt.Errorf("%w: %w", ErrPolicy, err)))
	}

	// Verify each presentation candidate until one succeeds. If none succeed, the last error is returned.
	for _, presentation := range presentationCandidates {
		verifiedPresentation, verr := verifyPresentation(ctx, vcVerifier, presentation, rs.Nonce, rs.ClientID,
			credentialQuery, s.cfg, verifierClientConfig)
		if verr != nil {
			lastErr = verr
			continue
		}
		vp = verifiedPresentation
		validatedPresentation = presentation
		break
	}

	if vp == nil {
		return nil, "", toServiceError(s.fail(ctx, rs, lastErr))
	}

	// TODO - Should be changed to StatusVerified.
	rs.Status = StatusCompleted
	rs.Result = vp
	rs.ValidatedVP = validatedPresentation
	if err := s.store.SaveRequestState(ctx, rs); err != nil {
		return nil, "", toServiceError(fmt.Errorf("failed to persist verification result: %w", err))
	}

	redirect := ""
	if s.cfg.ResultRedirectURI != "" {
		// TODO: need to add usercode support
		redirect = withState(s.cfg.ResultRedirectURI, state)
	}
	return vp, redirect, nil
}

// SubmitError records a wallet-reported error (e.g. access_denied) and marks the transaction failed.
func (s *openid4vpService) SubmitError(
	ctx context.Context, state, code, description string,
) *tidcommon.ServiceError {
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

func (s *openid4vpService) requestURI(state string) string {
	return withState(s.requestURL, state)
}

func (s *openid4vpService) responseURI(state string) string {
	return withState(s.responseURL, state)
}

// withState appends the state query parameter to a base URL.
func withState(base, state string) string {
	sep := "?"
	if u, err := url.Parse(base); err == nil && u.RawQuery != "" {
		sep = "&"
	}
	return base + sep + "state=" + url.QueryEscape(state)
}

// WalletAuthorizationURI builds the openid4vp:// deep link the wallet scans,
// carrying the client_id and request_uri.
func WalletAuthorizationURI(clientID, requestURI string) string {
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("request_uri", requestURI)
	// TODO - Make request_uri_method configurable (deployment level).
	v.Set("request_uri_method", "get")
	return "openid4vp://?" + v.Encode()
}

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

	if len(cfg.DCQL.Credentials) == 0 {
		return nil, fmt.Errorf("%w: dcql_query requires at least one credential query", ErrPolicy)
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
		"dcql_query":      cfg.DCQL,
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
	jwk, err := ecdsaPublicKeyToEncJWK(params.EphemeralKey)
	if err != nil {
		return nil, err
	}

	encValues := cfg.ResponseEncValues
	if len(encValues) == 0 {
		encValues = []string{DefaultResponseEncValue}
	}

	vpFormats := map[string]interface{}{
		formatSDJWTVC: map[string]interface{}{
			"kb-jwt_alg_values": []string{"ES256", "EdDSA"},
			"sd-jwt_alg_values": []string{"ES256", "EdDSA"},
		},
		formatLDProofVC: map[string]interface{}{
			"kb-jwt_alg_values": []string{"ES256", "EdDSA"},
			"ldp_alg_values":    []string{"ES256", "EdDSA"},
		},
	}

	// TODO - This should be dynamic based on the registered VC verifiers. For now, we hardcode the two known formats.
	return map[string]interface{}{
		"jwks": map[string]interface{}{
			"keys": []interface{}{jwk},
		},
		"vp_formats_supported":                    vpFormats,
		"encrypted_response_enc_values_supported": encValues,
	}, nil
}

// ecdsaPublicKeyToEncJWK encodes an EC public key as an encryption-use JWK whose kid is
// its RFC 7638 thumbprint.
func ecdsaPublicKeyToEncJWK(pub *ecdsa.PublicKey) (map[string]interface{}, error) {
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
	kid, err := jws.ComputeJKT(jwk)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPolicy, err)
	}
	jwk["kid"] = kid
	return jwk, nil
}

// signRequestObject signs the request object claims into a compact JWS using the crypto provider.
func (s *openid4vpService) signRequestObject(ctx context.Context, claims map[string]interface{}) (string, error) {
	// No kid header: for the x509_san_dns client scheme the wallet authenticates
	// the request via the x5c certificate. A stray kid (a JWK thumbprint) alongside
	// x5c trips strict wallets that try to resolve it first, so it is omitted.
	header := map[string]interface{}{
		"alg": s.signingAlg,
		"typ": requestObjectType,
		"x5c": s.x5c,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request object header: %w", err)
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request object claims: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON)
	derSig, err := s.cryptoProvider.Sign(ctx, s.signingKeyRef, s.signingAlg, []byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("failed to sign request object: %w", err)
	}
	jwsSig := ecdsaDERToJWS(derSig, s.signingAlg)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(jwsSig), nil
}

// ecdsaDERToJWS converts a DER-encoded ASN.1 ECDSA signature to the raw r||s
// fixed-size format required by JWS (RFC 7518 §3.4).
func ecdsaDERToJWS(derSig []byte, alg string) []byte {
	var sig struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(derSig, &sig); err != nil {
		return derSig // not DER (e.g. Ed25519): return as-is
	}
	var coordLen int
	switch jws.Algorithm(alg) {
	case jws.ES256:
		coordLen = 32
	case jws.ES384:
		coordLen = 48
	case jws.ES512:
		coordLen = 66
	default:
		return derSig
	}
	raw := make([]byte, 2*coordLen)
	rBytes := sig.R.Bytes()
	sBytes := sig.S.Bytes()
	copy(raw[coordLen-len(rBytes):coordLen], rBytes)
	copy(raw[2*coordLen-len(sBytes):], sBytes)
	return raw
}

// parseAuthorizationResponse parses the decrypted OpenID4VP response body.
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

// decodePresentationValue accepts either a single presentation string or an array of them.
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

// presentationsFor returns the wallet-supplied presentations for credentialID.
func (r *authorizationResponse) presentationsFor(credentialID string) ([]string, error) {
	list, ok := r.Presentations[credentialID]
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("%w: no presentation for credential %q", ErrInvalidResponse, credentialID)
	}
	return list, nil
}

// verifyPresentation verifies one wallet presentation with the format verifier and applies the claim policy.
// The holder key binding must carry nonce and be addressed to audience, the verifier's client_id. The
// verifier client configuration's trust settings override the engine defaults in cfg.
func verifyPresentation(
	ctx context.Context, vcVerifier providers.VCVerifierProvider, presentation, nonce, audience string,
	credentialQuery providers.DCQLCredentialQuery,
	cfg engineconfig.OpenID4VPConfig,
	clientConfig *providers.VCVerifierClientConfig,
) (*VerifiedPresentation, error) {
	enforceTrustedIssuer := cfg.EnforceTrustedIssuer
	if clientConfig.EnforceTrustedIssuer != nil {
		enforceTrustedIssuer = *clientConfig.EnforceTrustedIssuer
	}
	cred, err := vcVerifier.Verify(ctx, presentation, providers.VCVerifierParams{
		Nonce:                nonce,
		Audience:             audience,
		Leeway:               time.Duration(cfg.LeewaySeconds) * time.Second,
		KeyBindingMaxAge:     time.Duration(cfg.KeyBindingMaxAgeSeconds) * time.Second,
		EnforceTrustedIssuer: enforceTrustedIssuer,
		TrustedAuthorities:   clientConfig.TrustedAuthorities,
		EnforceKeyBinding:    cfg.EnforceKeyBindingEnabled(),
		CredentialQuery:      credentialQuery,
	})
	if err != nil {
		return nil, err
	}
	if err := verifyClaims(cred.Claims, credentialQuery); err != nil {
		return nil, err
	}

	// Derive the subject from the claim if present, otherwise fall back to "sub" or the key-binding thumbprint.
	subjectClaim := clientConfig.SubjectClaim
	subject, _ := cred.Claims[subjectClaim].(string)
	if subject == "" {
		subject, _ = cred.Claims["sub"].(string)
	}
	if subject == "" {
		subject = cred.KeyBindingThumbprint
	}
	if subject == "" {
		return nil, fmt.Errorf("%w: subject claim %q is missing and no key-binding thumbprint available",
			ErrPolicy, subjectClaim)
	}
	return &VerifiedPresentation{
		Format:               cred.Format,
		Type:                 cred.Type,
		Subject:              subject,
		Claims:               cred.Claims,
		KeyBindingThumbprint: cred.KeyBindingThumbprint,
	}, nil
}

// verifyClaims checks the verified claims against the DCQL credential query. A disclosed claim
// must match the query's allowed values. Without claim_sets every requested claim is mandatory;
// with claim_sets the disclosed claims must satisfy at least one set.
func verifyClaims(claims map[string]interface{}, credentialQuery providers.DCQLCredentialQuery) error {
	present := make(map[string]bool, len(credentialQuery.Claims))
	for _, claim := range credentialQuery.Claims {
		value, ok := lookupClaim(claims, claim.Path)
		if !ok {
			continue
		}
		if len(claim.Values) > 0 && !valueAllowed(value, claim.Values) {
			return fmt.Errorf("%w: %s", ErrClaimValueNotAllowed, claimPathString(claim.Path))
		}
		present[claim.ID] = true
	}

	if len(credentialQuery.ClaimSets) == 0 {
		for _, claim := range credentialQuery.Claims {
			if _, ok := lookupClaim(claims, claim.Path); !ok {
				return fmt.Errorf("%w: %s", ErrMissingMandatoryClaim, claimPathString(claim.Path))
			}
		}
		return nil
	}

	for _, set := range credentialQuery.ClaimSets {
		if !slices.ContainsFunc(set, func(id string) bool { return !present[id] }) {
			return nil
		}
	}
	return fmt.Errorf("%w: no claim set satisfied for credential %q", ErrMissingMandatoryClaim, credentialQuery.ID)
}

// valueAllowed reports whether value matches one of the allowed DCQL values.
func valueAllowed(value interface{}, allowed []interface{}) bool {
	return slices.ContainsFunc(allowed, func(a interface{}) bool { return fmt.Sprint(a) == fmt.Sprint(value) })
}

// lookupClaim resolves a DCQL claim path against the nested claims. String segments select object
// members and integer segments select array elements.
func lookupClaim(claims map[string]interface{}, path []interface{}) (interface{}, bool) {
	if len(path) == 0 {
		return nil, false
	}
	var current interface{} = claims
	for _, seg := range path {
		switch s := seg.(type) {
		case string:
			obj, ok := current.(map[string]interface{})
			if !ok {
				return nil, false
			}
			if current, ok = obj[s]; !ok {
				return nil, false
			}
		case int:
			arr, ok := current.([]interface{})
			if !ok || s < 0 || s >= len(arr) {
				return nil, false
			}
			current = arr[s]
		default:
			return nil, false
		}
	}
	return current, true
}

// claimPathString renders a DCQL claim path in dotted form for error messages.
func claimPathString(path []interface{}) string {
	parts := make([]string, 0, len(path))
	for _, seg := range path {
		parts = append(parts, fmt.Sprint(seg))
	}
	return strings.Join(parts, ".")
}
