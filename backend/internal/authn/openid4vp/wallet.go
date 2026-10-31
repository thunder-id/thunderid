// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
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
	"github.com/thunder-id/thunderid/internal/system/jose/sdjwt"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// walletInterface is the internal interface consumed by the wallet-facing HTTP handler.
type walletInterface interface {
	GetRequestObject(ctx context.Context, state string) (string, *tidcommon.ServiceError)
	SubmitResponse(ctx context.Context, state string, body []byte) (
		*VerifiedPresentation, string, *tidcommon.ServiceError,
	)
	SubmitError(ctx context.Context, state, code, description string) *tidcommon.ServiceError
	GetTrustAnchors() []TrustAnchorInfo
}

// OpenID4VP protocol constants.
const (
	ResponseTypeVPToken       = "vp_token"
	ResponseModeDirectPostJWT = "direct_post.jwt"
	DefaultResponseEncValue   = "A128GCM"
	defaultRequestValidity    = 5 * time.Minute
	requestObjectType         = "oauth-authz-req+jwt"
)

// GetRequestObject builds and signs the request object (JAR) for state.
func (s *openid4vpService) GetRequestObject(ctx context.Context, state string) (string, *tidcommon.ServiceError) {
	rs, err := s.load(ctx, state)
	if err != nil {
		return "", toServiceError(err)
	}
	if rs.EphemeralKey == nil {
		return "", &tidcommon.InternalServerError
	}
	def, err := s.resolveDefinition(ctx, rs.DefinitionID)
	if err != nil {
		return "", toServiceError(err)
	}

	dcql := def.DCQL
	if s.trust != nil {
		dcql.TrustedAuthorityKeyIDs = s.trust.skisFor(dcql.TrustedAuthorities)
	}

	claims, err := buildRequestObject(requestConfig{
		ClientID:          s.clientID,
		ResponseURI:       s.responseURI(state),
		ResponseMode:      ResponseModeDirectPostJWT,
		Audience:          s.cfg.RequestAudience,
		Validity:          s.cfg.RequestValidity,
		DCQL:              dcql,
		ResponseEncValues: s.cfg.ResponseEncValues,
		VerifierInfo:      s.cfg.VerifierInfo,
	}, requestParams{
		Nonce:          rs.Nonce,
		State:          state,
		EphemeralKey:   &rs.EphemeralKey.PublicKey,
		EphemeralKeyID: s.cfg.EphemeralKeyID,
		IssuedAt:       time.Now(),
	})
	if err != nil {
		return "", toServiceError(err)
	}
	jar, err := s.signRequestObject(ctx, claims)
	if err != nil {
		return "", toServiceError(err)
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
	def, err := s.resolveDefinition(ctx, rs.DefinitionID)
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
	candidates, err := resp.presentationsFor(def.DCQL.CredentialID)
	if err != nil {
		return nil, "", toServiceError(s.fail(ctx, rs, err))
	}

	policy := def.policy
	if policy.Leeway == 0 {
		policy.Leeway = s.cfg.Leeway
	}
	if policy.KeyBindingMaxAge == 0 {
		policy.KeyBindingMaxAge = s.cfg.KeyBindingMaxAge
	}

	// Accept the first candidate that verifies and satisfies the policy.
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
		return nil, "", toServiceError(s.fail(ctx, rs, lastErr))
	}
	if def.DeriveSubject != nil {
		if subject := def.DeriveSubject(vp); subject != "" {
			vp.Subject = subject
		}
	}

	rs.Status = StatusCompleted
	rs.Result = vp
	if err := s.store.SaveRequestState(ctx, rs); err != nil {
		return nil, "", toServiceError(fmt.Errorf("failed to persist verification result: %w", err))
	}

	redirect := ""
	if s.cfg.ResultRedirectURIBase != "" {
		redirect = withState(s.cfg.ResultRedirectURIBase, state)
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

// GetTrustAnchors returns the configured trust anchors (root CAs).
func (s *openid4vpService) GetTrustAnchors() []TrustAnchorInfo {
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

func (s *openid4vpService) requestURI(state string) string {
	return withState(s.cfg.RequestURIBase, state)
}

func (s *openid4vpService) responseURI(state string) string {
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

// WalletAuthorizationURI builds the openid4vp:// deep link the wallet scans,
// carrying the client_id and request_uri.
func WalletAuthorizationURI(clientID, requestURI string) string {
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("request_uri", requestURI)
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

// verifySDJWTPresentation parses, verifies, and resolves disclosures for an SD-JWT VC presentation.
func verifySDJWTPresentation(
	presentation string, trust *trustAnchorStore,
	expectedAudience, expectedNonce string, leeway, maxIATAge time.Duration,
	enforceTrustedIssuer, enforceKeyBinding bool, allowedAnchors []string,
) (*verifiedCredential, error) {
	p, err := sdjwt.Parse(presentation)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
	}

	issuerClaims, err := p.IssuerClaims()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
	}
	issuer, _ := issuerClaims["iss"].(string)
	if issuer == "" {
		return nil, fmt.Errorf("%w: credential missing iss", ErrInvalidPresentation)
	}

	chain, err := x5cChain(p.IssuerJWT)
	if err != nil {
		return nil, err
	}
	issuerLeaf := chain[0]
	if enforceTrustedIssuer {
		if trust == nil {
			return nil, fmt.Errorf("%w: no trust anchors configured", ErrUntrustedIssuer)
		}
		var err error
		issuerLeaf, err = trust.verifyChain(chain, time.Now(), allowedAnchors)
		if err != nil {
			return nil, err
		}
	}
	if err := sdjwt.VerifyIssuerSignature(p, issuerLeaf.PublicKey); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
	}

	cred, err := sdjwt.ResolveDisclosures(p)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
	}

	if enforceKeyBinding || p.HasKeyBinding() {
		if err := sdjwt.VerifyKeyBinding(p, cred, sdjwt.VerifyOptions{
			ExpectedAudience: expectedAudience,
			ExpectedNonce:    expectedNonce,
			Leeway:           leeway,
			MaxIATAge:        maxIATAge,
		}); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
		}
	}

	vct, _ := cred.Claims["vct"].(string)
	var keyBindingThumbprint string
	if cred.ConfirmationKey != nil {
		if jkt, jktErr := jws.ComputeJKT(cred.ConfirmationKey); jktErr == nil {
			keyBindingThumbprint = jkt
		}
	}
	return &verifiedCredential{
		Issuer:               issuer,
		VCT:                  vct,
		Claims:               cred.Claims,
		DisclosedPaths:       cred.DisclosedPaths,
		KeyBindingThumbprint: keyBindingThumbprint,
	}, nil
}

// x5cChain extracts the leaf-first DER certificate chain from the issuer JWT's x5c header (RFC 7515).
func x5cChain(issuerJWT string) ([]*x509.Certificate, error) {
	header, err := jws.DecodeHeader(issuerJWT)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
	}
	raw, ok := header["x5c"].([]interface{})
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("%w: issuer JWT missing x5c header", ErrInvalidPresentation)
	}
	chain := make([]*x509.Certificate, 0, len(raw))
	for _, entry := range raw {
		encoded, ok := entry.(string)
		if !ok {
			return nil, fmt.Errorf("%w: malformed x5c entry", ErrInvalidPresentation)
		}
		der, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidPresentation, err)
		}
		chain = append(chain, cert)
	}
	return chain, nil
}

// finalizePresentation enforces VCT and selective-disclosure policy on a raw credential.
func finalizePresentation(cred *verifiedCredential, policy policy) (*VerifiedPresentation, error) {
	if cred.VCT != policy.ExpectedVCT {
		return nil, fmt.Errorf("%w: got %q, want %q", ErrUnexpectedVCT, cred.VCT, policy.ExpectedVCT)
	}
	if err := enforceClaimPolicy(cred.DisclosedPaths, cred.Claims, policy); err != nil {
		return nil, err
	}

	subject, _ := cred.Claims["sub"].(string)
	return &VerifiedPresentation{
		Issuer:               cred.Issuer,
		Subject:              subject,
		Claims:               flattenClaims(cred.Claims),
		KeyBindingThumbprint: cred.KeyBindingThumbprint,
	}, nil
}

// enforceClaimPolicy applies data minimisation and mandatory-claim checks.
func enforceClaimPolicy(disclosed []string, claims map[string]interface{}, policy policy) error {
	if len(policy.RequestedClaims) > 0 {
		requested := make(map[string]bool, len(policy.RequestedClaims))
		for _, c := range policy.RequestedClaims {
			requested[c] = true
		}
		for _, path := range disclosed {
			if !requested[path] {
				return fmt.Errorf("%w: %s", ErrUnrequestedClaim, path)
			}
		}
	}

	for _, mandatory := range policy.MandatoryClaims {
		if _, ok := lookupClaim(claims, mandatory); !ok {
			return fmt.Errorf("%w: %s", ErrMissingMandatoryClaim, mandatory)
		}
	}

	for path, allowed := range policy.ClaimValues {
		value, ok := lookupClaim(claims, path)
		if !ok {
			continue
		}
		if !valueAllowed(value, allowed) {
			return fmt.Errorf("%w: %s", ErrClaimValueNotAllowed, path)
		}
	}
	return nil
}

func valueAllowed(value interface{}, allowed []string) bool {
	return slices.Contains(allowed, fmt.Sprint(value))
}

// lookupClaim resolves a dotted-path claim value from the nested claims map.
func lookupClaim(claims map[string]interface{}, path string) (interface{}, bool) {
	segments := strings.Split(path, ".")
	var current interface{} = claims
	for _, seg := range segments {
		obj, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = obj[seg]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// flattenClaims returns dotted-path-keyed attributes, omitting SD-JWT metadata claims.
func flattenClaims(claims map[string]interface{}) map[string]interface{} {
	metaRoots := map[string]struct{}{
		"iss": {}, "vct": {}, "cnf": {}, "iat": {}, "exp": {}, "nbf": {}, "status": {}, "_sd_alg": {},
	}
	filtered := make(map[string]interface{}, len(claims))
	for k, v := range claims {
		if _, isMeta := metaRoots[k]; !isMeta {
			filtered[k] = v
		}
	}
	out := make(map[string]interface{})
	flattenInto(out, "", filtered)
	return out
}

func flattenInto(out map[string]interface{}, prefix string, node map[string]interface{}) {
	for k, val := range node {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nested, ok := val.(map[string]interface{}); ok {
			flattenInto(out, key, nested)
			continue
		}
		out[key] = val
	}
}

// defaultSubjectDeriver returns a deriver that falls back to the key-binding thumbprint when sub is absent.
func defaultSubjectDeriver() subjectDeriver {
	return func(vp *VerifiedPresentation) string {
		if vp == nil {
			return ""
		}
		if vp.Subject != "" {
			return vp.Subject
		}
		if vp.KeyBindingThumbprint != "" {
			return "urn:ietf:params:oauth:jwk-thumbprint:sha-256:" + vp.KeyBindingThumbprint
		}
		return ""
	}
}
