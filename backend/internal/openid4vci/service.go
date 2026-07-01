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

package openid4vci

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/jose/sdjwt"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/internal/vc/credential"
)

// OpenID4VCIServiceInterface is the contract for the OpenID4VCI issuer service,
// consumed by the HTTP handler.
type OpenID4VCIServiceInterface interface {
	GetMetadata(ctx context.Context) map[string]interface{}
	GenerateCredentialOffer(ctx context.Context, configID string) (map[string]interface{}, string, error)
	GetCredentialOffer(ctx context.Context, id string) (map[string]interface{}, error)
	GenerateNonce(ctx context.Context) (string, error)
	IssueCredential(ctx context.Context, accessToken string, body []byte) (*CredentialResponse, error)
}

var _ OpenID4VCIServiceInterface = (*openid4vciService)(nil)

// openid4vciService drives the OpenID4VCI issuer: it advertises issuer metadata, issues
// c_nonces, and issues SD-JWT VCs bound to the holder key after validating the
// access token and holder proof.
type openid4vciService struct {
	cfg            serviceConfig
	cryptoProvider kmprovider.RuntimeCryptoProvider
	signingKeyRef  kmprovider.KeyRef
	signingAlg     string
	kid            string
	x5c            []string
	store          openID4VCIStoreInterface
	jwtService     jwt.JWTServiceInterface
	userService    user.UserServiceInterface
	creds          credential.CredentialConfigurationServiceInterface
}

// newOpenID4VCIService creates an OpenID4VCI issuer engine.
func newOpenID4VCIService(
	cfg serviceConfig,
	cryptoProvider kmprovider.RuntimeCryptoProvider, signingKeyRef kmprovider.KeyRef,
	signingAlg, kid string, x5c []string,
	store openID4VCIStoreInterface,
	jwtService jwt.JWTServiceInterface, userService user.UserServiceInterface,
	creds credential.CredentialConfigurationServiceInterface,
) (OpenID4VCIServiceInterface, error) {
	if cryptoProvider == nil || store == nil ||
		jwtService == nil || userService == nil || creds == nil {
		return nil, fmt.Errorf("%w: required issuer dependencies are missing", ErrPolicy)
	}
	if cfg.CredentialIssuer == "" {
		return nil, fmt.Errorf("%w: credential_issuer is required", ErrPolicy)
	}
	return &openid4vciService{
		cfg:            cfg,
		cryptoProvider: cryptoProvider,
		signingKeyRef:  signingKeyRef,
		signingAlg:     signingAlg,
		kid:            kid,
		x5c:            x5c,
		store:          store,
		jwtService:     jwtService,
		userService:    userService,
		creds:          creds,
	}, nil
}

const (
	// credentialOfferScheme is the URI scheme wallets handle for credential offers.
	credentialOfferScheme = "openid-credential-offer://" //nolint:gosec
	// defaultOfferTTL bounds how long a stored credential offer is retrievable.
	defaultOfferTTL = 5 * time.Minute
)

// GetMetadata builds the OpenID4VCI credential issuer metadata from the managed
// credential configurations.
func (s *openid4vciService) GetMetadata(ctx context.Context) map[string]interface{} {
	configs, svcErr := s.creds.ListCredentialConfigurations(ctx)
	if svcErr != nil {
		log.GetLogger().Error(ctx, "Failed to list credential configurations for issuer metadata")
	}
	return buildMetadata(s.cfg, configs)
}

// GenerateNonce issues a fresh, single-use c_nonce and records it under the nonce TTL.
func (s *openid4vciService) GenerateNonce(ctx context.Context) (string, error) {
	nonce, err := randomToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate c_nonce: %w", err)
	}
	rec := &nonceRecord{ExpiresAt: time.Now().Add(s.cfg.NonceTTL)}
	if err := s.store.SaveNonce(ctx, nonce, rec); err != nil {
		return "", fmt.Errorf("failed to store c_nonce: %w", err)
	}
	return nonce, nil
}

// GenerateCredentialOffer builds and stores an issuer-initiated credential offer for configID and
// returns it with the openid-credential-offer:// deep link (offer by reference).
func (s *openid4vciService) GenerateCredentialOffer(
	ctx context.Context, configID string,
) (map[string]interface{}, string, error) {
	cred, svcErr := s.creds.GetCredentialConfigurationByHandle(ctx, configID)
	if svcErr != nil {
		return nil, "", fmt.Errorf("%w: %s", ErrUnsupportedCredential, configID)
	}

	offer := map[string]interface{}{
		"credential_issuer":            s.cfg.CredentialIssuer,
		"credential_configuration_ids": []string{cred.Handle},
		"grants": map[string]interface{}{
			"authorization_code": map[string]interface{}{},
		},
	}

	id, err := randomToken()
	if err != nil {
		return nil, "", fmt.Errorf("%w: failed to generate offer id: %w", ErrIssuance, err)
	}
	rec := &offerRecord{Offer: offer, ExpiresAt: time.Now().Add(defaultOfferTTL)}
	if err := s.store.SaveOffer(ctx, id, rec); err != nil {
		return nil, "", fmt.Errorf("%w: failed to store credential offer: %w", ErrIssuance, err)
	}

	offerURI := s.cfg.BaseURL + credentialOfferPath + "/" + id
	deepLink := credentialOfferScheme + "?credential_offer_uri=" + url.QueryEscape(offerURI)
	return offer, deepLink, nil
}

// GetCredentialOffer returns a previously stored issuer-initiated credential offer by
// id, so a wallet can resolve the credential_offer_uri.
func (s *openid4vciService) GetCredentialOffer(ctx context.Context, id string) (map[string]interface{}, error) {
	rec, ok := s.store.GetOffer(ctx, id)
	if !ok || rec == nil {
		return nil, fmt.Errorf("%w: unknown credential offer", ErrUnsupportedCredential)
	}
	if time.Now().After(rec.ExpiresAt) {
		return nil, fmt.Errorf("%w: credential offer expired", ErrUnsupportedCredential)
	}
	return rec.Offer, nil
}

// IssueCredential validates the bearer access token and holder proof, then
// issues an SD-JWT VC bound to the holder key with claims sourced from the
// authenticated subject's profile. The credential the wallet is authorized for
// is determined by the access-token scope (matched against credential configs).
func (s *openid4vciService) IssueCredential(
	ctx context.Context, accessToken string, body []byte,
) (*CredentialResponse, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("%w: missing access token", ErrInvalidToken)
	}
	if svcErr := s.jwtService.VerifyJWT(ctx, accessToken, "", ""); svcErr != nil {
		return nil, fmt.Errorf("%w: access token verification failed", ErrInvalidToken)
	}
	payload, err := jwt.DecodeJWTPayload(accessToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	subject, _ := payload["sub"].(string)
	if subject == "" {
		return nil, fmt.Errorf("%w: access token missing subject", ErrInvalidToken)
	}
	scopes := strings.Fields(scopeString(payload))

	var req CredentialRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}

	cred, err := s.authorizedCredential(ctx, req.CredentialConfigurationID, scopes)
	if err != nil {
		return nil, err
	}

	proofs := req.holderProofs()
	if len(proofs) == 0 {
		return nil, fmt.Errorf("%w: missing holder proof", ErrInvalidProof)
	}
	if len(proofs) > s.cfg.BatchSize {
		return nil, fmt.Errorf("%w: %d proofs exceeds batch size %d", ErrInvalidRequest, len(proofs), s.cfg.BatchSize)
	}

	holderJWKs, err := s.verifyProofs(ctx, proofs)
	if err != nil {
		return nil, err
	}

	claims, err := s.resolveClaims(ctx, subject, cred.SDClaims)
	if err != nil {
		return nil, err
	}

	validity := s.cfg.CredentialValidity
	if cred.Validity > 0 {
		validity = cred.Validity
	}

	sigHeader := map[string]interface{}{
		"alg": s.signingAlg,
		"typ": cred.Format,
		"x5c": s.x5c,
	}
	if s.kid != "" {
		sigHeader["kid"] = s.kid
	}

	now := time.Now()
	issued := make([]IssuedCredential, 0, len(holderJWKs))
	for _, holderJWK := range holderJWKs {
		combined, _, err := sdjwt.Issue(sdjwt.IssueParams{
			Header:          sigHeader,
			Issuer:          s.cfg.CredentialIssuer,
			VCT:             cred.VCT,
			IssuedAt:        now,
			ExpiresAt:       now.Add(validity),
			SelectiveClaims: claims,
			AlwaysVisible:   map[string]interface{}{"sub": subject},
			ConfirmationJWK: holderJWK,
		}, func(signingInput string) ([]byte, error) {
			derSig, err := s.cryptoProvider.Sign(ctx, s.signingKeyRef, s.signingAlg, []byte(signingInput))
			if err != nil {
				return nil, fmt.Errorf("failed to sign credential: %w", err)
			}
			return ecdsaDERToJWS(derSig, s.signingAlg), nil
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrIssuance, err)
		}
		issued = append(issued, IssuedCredential{Credential: combined})
	}

	return &CredentialResponse{Credentials: issued}, nil
}

// buildMetadata assembles the OpenID4VCI credential issuer metadata document
// served at /.well-known/openid-credential-issuer.
func buildMetadata(cfg serviceConfig, creds []credential.CredentialConfigurationDTO) map[string]interface{} {
	configs := make(map[string]interface{}, len(creds))
	for _, c := range creds {
		format := c.Format
		if format == "" {
			format = credential.DefaultCredentialFormat
		}
		entry := map[string]interface{}{
			"format": format,
			"scope":  c.Handle,
			"vct":    c.VCT,
			"cryptographic_binding_methods_supported": []string{"jwk"},
			"credential_signing_alg_values_supported": []string{"ES256"},
			"proof_types_supported": map[string]interface{}{
				"jwt": map[string]interface{}{
					"proof_signing_alg_values_supported": []string{"ES256"},
				},
			},
		}
		if d := credentialDisplay(c.Name, c.Description, c.Display); d != nil {
			entry["display"] = d
		}
		if cl := credentialClaims(c.Claims); cl != nil {
			entry["claims"] = cl
		}
		configs[c.Handle] = entry
	}

	metadata := map[string]interface{}{
		"credential_issuer":                   cfg.CredentialIssuer,
		"credential_endpoint":                 cfg.BaseURL + credentialPath,
		"nonce_endpoint":                      cfg.BaseURL + noncePath,
		"credential_configurations_supported": configs,
	}
	if len(cfg.AuthorizationServers) > 0 {
		metadata["authorization_servers"] = cfg.AuthorizationServers
	}
	if cfg.BatchSize > 1 {
		metadata["batch_credential_issuance"] = map[string]interface{}{"batch_size": cfg.BatchSize}
	}
	return metadata
}

// credentialClaims builds the per-claim display map for the metadata document.
// Only claims with a DisplayName set are included; returns nil when none qualify.
func credentialClaims(claims []credential.ClaimMapping) map[string]interface{} {
	out := make(map[string]interface{}, len(claims))
	for _, c := range claims {
		if c.DisplayName == "" {
			continue
		}
		out[c.Name] = map[string]interface{}{
			"display": []interface{}{
				map[string]interface{}{"name": c.DisplayName},
			},
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// credentialDisplay builds the metadata display array from the configuration's
// admin-facing name/description and its optional locale/logo, or nil if none are set.
func credentialDisplay(name, description string, d *credential.CredentialDisplay) []interface{} {
	entry := map[string]interface{}{}
	if name != "" {
		entry["name"] = name
	}
	if description != "" {
		entry["description"] = description
	}
	if d != nil {
		if d.Locale != "" {
			entry["locale"] = d.Locale
		}
		if d.LogoURI != "" {
			entry["logo"] = map[string]interface{}{"uri": d.LogoURI}
		}
	}
	if len(entry) == 0 {
		return nil
	}
	return []interface{}{entry}
}

// authorizedCredential resolves the credential configuration the request is
// authorized for. When a credential_configuration_id is supplied it must match a
// managed credential; scope binding is enforced only when configured. Without a
// configuration id, the first managed credential whose scope is in the token is used.
func (s *openid4vciService) authorizedCredential(
	ctx context.Context, configID string, scopes []string,
) (credentialConfig, error) {
	scopeSet := make(map[string]bool, len(scopes))
	for _, sc := range scopes {
		scopeSet[sc] = true
	}

	if configID != "" {
		dto, svcErr := s.creds.GetCredentialConfigurationByHandle(ctx, configID)
		if svcErr != nil {
			return credentialConfig{}, fmt.Errorf("%w: %s", ErrUnsupportedCredential, configID)
		}
		if s.cfg.EnforceScope && !scopeSet[dto.Handle] {
			return credentialConfig{}, fmt.Errorf("%w: scope %q not authorized", ErrInvalidToken, dto.Handle)
		}
		return dtoToCredentialConfig(*dto), nil
	}

	configs, svcErr := s.creds.ListCredentialConfigurations(ctx)
	if svcErr != nil {
		return credentialConfig{}, fmt.Errorf("%w: failed to resolve credential configurations", ErrIssuance)
	}
	for _, dto := range configs {
		if scopeSet[dto.Handle] {
			return dtoToCredentialConfig(dto), nil
		}
	}
	return credentialConfig{}, fmt.Errorf("%w: no authorized credential scope present", ErrInvalidToken)
}

// dtoToCredentialConfig maps a managed credential DTO to the issuer's internal config.
func dtoToCredentialConfig(dto credential.CredentialConfigurationDTO) credentialConfig {
	format := dto.Format
	if format == "" {
		format = credential.DefaultCredentialFormat
	}
	names := make([]string, 0, len(dto.Claims))
	for _, c := range dto.Claims {
		names = append(names, c.Name)
	}
	var validity time.Duration
	if dto.ValiditySeconds != nil {
		validity = time.Duration(*dto.ValiditySeconds) * time.Second
	}
	return credentialConfig{
		Format:   format,
		VCT:      dto.VCT,
		SDClaims: names,
		Validity: validity,
	}
}

// resolveClaims loads the authenticated subject's profile attributes and selects
// the configured selectively disclosable claims for the credential.
func (s *openid4vciService) resolveClaims(
	ctx context.Context, userID string, claimNames []string,
) (map[string]interface{}, error) {
	u, svcErr := s.userService.GetUser(ctx, userID, false)
	if svcErr != nil || u == nil {
		return nil, fmt.Errorf("%w: %s", ErrUserNotFound, userID)
	}

	var attrs map[string]interface{}
	if len(u.Attributes) > 0 {
		if err := json.Unmarshal(u.Attributes, &attrs); err != nil {
			return nil, fmt.Errorf("%w: failed to decode user attributes: %w", ErrIssuance, err)
		}
	}

	claims := make(map[string]interface{}, len(claimNames))
	for _, name := range claimNames {
		if v, ok := attrs[name]; ok {
			claims[name] = v
		}
	}
	return claims, nil
}

// scopeString extracts the space-delimited scope claim, tolerating either a
// string or a []string representation.
func scopeString(payload map[string]interface{}) string {
	switch v := payload["scope"].(type) {
	case string:
		return v
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}

// verifyProofs validates a batch of OpenID4VCI holder proofs of possession and
// returns one confirmation JWK per proof (to bind into each issued credential's
// cnf). Each distinct nonce is consumed exactly once.
func (s *openid4vciService) verifyProofs(ctx context.Context, proofs []Proof) ([]map[string]interface{}, error) {
	jwks := make([]map[string]interface{}, 0, len(proofs))
	nonces := make([]string, 0, len(proofs))
	for _, proof := range proofs {
		jwk, nonce, err := s.checkProof(proof)
		if err != nil {
			return nil, err
		}
		jwks = append(jwks, jwk)
		nonces = append(nonces, nonce)
	}

	consumed := make(map[string]bool, len(nonces))
	for _, nonce := range nonces {
		if consumed[nonce] {
			continue
		}
		if err := s.consumeNonce(ctx, nonce); err != nil {
			return nil, err
		}
		consumed[nonce] = true
	}

	return jwks, nil
}

// checkProof validates a single holder proof JWT — proof typ, signature (against
// the embedded jwk), audience, and iat freshness — and returns the holder's
// confirmation JWK and the proof's c_nonce.
func (s *openid4vciService) checkProof(proof Proof) (map[string]interface{}, string, error) {
	if proof.ProofType != "jwt" || proof.JWT == "" {
		return nil, "", fmt.Errorf("%w: proof must be a jwt proof", ErrInvalidProof)
	}

	header, err := jws.DecodeHeader(proof.JWT)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInvalidProof, err)
	}
	if typ, _ := header["typ"].(string); typ != proofType {
		return nil, "", fmt.Errorf("%w: unexpected proof typ %q", ErrInvalidProof, typ)
	}
	jwk, ok := header["jwk"].(map[string]interface{})
	if !ok || len(jwk) == 0 {
		return nil, "", fmt.Errorf("%w: proof header missing jwk", ErrInvalidProof)
	}

	if err := verifyJWSWithJWK(proof.JWT, jwk); err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInvalidProof, err)
	}

	payload, err := jwt.DecodeJWTPayload(proof.JWT)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInvalidProof, err)
	}
	if aud, _ := payload["aud"].(string); aud != s.cfg.CredentialIssuer {
		return nil, "", fmt.Errorf("%w: proof audience mismatch", ErrInvalidProof)
	}
	if err := s.checkProofIat(payload); err != nil {
		return nil, "", err
	}
	nonce, _ := payload["nonce"].(string)
	if nonce == "" {
		return nil, "", fmt.Errorf("%w: proof missing nonce", ErrInvalidNonce)
	}

	return jwk, nonce, nil
}

// checkProofIat rejects proofs whose iat is in the future or older than the
// configured maximum age (replay protection).
func (s *openid4vciService) checkProofIat(payload map[string]interface{}) error {
	iatRaw, ok := payload["iat"].(float64)
	if !ok {
		return fmt.Errorf("%w: proof missing iat", ErrInvalidProof)
	}
	iat := time.Unix(int64(iatRaw), 0)
	now := time.Now()
	if iat.After(now.Add(time.Minute)) {
		return fmt.Errorf("%w: proof iat is in the future", ErrInvalidProof)
	}
	if s.cfg.ProofMaxAge > 0 && iat.Before(now.Add(-s.cfg.ProofMaxAge)) {
		return fmt.Errorf("%w: proof iat too old", ErrInvalidProof)
	}
	return nil
}

// consumeNonce validates a proof's c_nonce against the live nonce store and
// deletes it so it cannot be replayed.
func (s *openid4vciService) consumeNonce(ctx context.Context, nonce string) error {
	rec, ok := s.store.GetNonce(ctx, nonce)
	if !ok || rec == nil {
		return fmt.Errorf("%w: unknown c_nonce", ErrInvalidNonce)
	}
	if err := s.store.DeleteNonce(ctx, nonce); err != nil {
		return fmt.Errorf("failed to consume c_nonce: %w", err)
	}
	if time.Now().After(rec.ExpiresAt) {
		return fmt.Errorf("%w: c_nonce expired", ErrInvalidNonce)
	}
	return nil
}

// verifyJWSWithJWK verifies a compact JWS against the public key in jwk.
func verifyJWSWithJWK(token string, jwk map[string]interface{}) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid JWS format")
	}
	header, err := jws.DecodeHeader(token)
	if err != nil {
		return err
	}
	algStr, _ := header["alg"].(string)
	alg, err := jws.MapAlgorithmToSignAlg(jws.Algorithm(algStr))
	if err != nil {
		return err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}
	signingInput := []byte(parts[0] + "." + parts[1])

	switch alg {
	case cryptolib.ECDSASHA256, cryptolib.ECDSASHA384, cryptolib.ECDSASHA512:
		pub, err := ecJWKToECDSAPublicKey(jwk)
		if err != nil {
			return err
		}
		return cryptolib.Verify(signingInput, sig, alg, pub)
	default:
		pub, err := jws.JWKToPublicKey(jwk)
		if err != nil {
			return err
		}
		return cryptolib.Verify(signingInput, sig, alg, pub)
	}
}

// ecJWKToECDSAPublicKey builds an *ecdsa.PublicKey from an EC JWK.
func ecJWKToECDSAPublicKey(jwk map[string]interface{}) (*ecdsa.PublicKey, error) {
	crv, _ := jwk["crv"].(string)
	xStr, _ := jwk["x"].(string)
	yStr, _ := jwk["y"].(string)
	if crv == "" || xStr == "" || yStr == "" {
		return nil, fmt.Errorf("EC JWK missing crv/x/y")
	}

	var curve elliptic.Curve
	var coordLen int
	switch crv {
	case "P-256":
		curve, coordLen = elliptic.P256(), 32
	case "P-384":
		curve, coordLen = elliptic.P384(), 48
	case "P-521":
		curve, coordLen = elliptic.P521(), 66
	default:
		return nil, fmt.Errorf("unsupported EC curve: %s", crv)
	}

	xBytes, err := base64.RawURLEncoding.DecodeString(xStr)
	if err != nil {
		return nil, fmt.Errorf("decode EC x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(yStr)
	if err != nil {
		return nil, fmt.Errorf("decode EC y: %w", err)
	}
	if len(xBytes) > coordLen || len(yBytes) > coordLen {
		return nil, fmt.Errorf("EC coordinate exceeds curve size for %s", crv)
	}

	uncompressed := make([]byte, 1+2*coordLen)
	uncompressed[0] = 0x04
	copy(uncompressed[1+coordLen-len(xBytes):1+coordLen], xBytes)
	copy(uncompressed[1+2*coordLen-len(yBytes):], yBytes)

	pub, err := ecdsa.ParseUncompressedPublicKey(curve, uncompressed)
	if err != nil {
		return nil, fmt.Errorf("invalid EC public key: %w", err)
	}
	return pub, nil
}

// randomToken returns 32 cryptographically random bytes, base64url-encoded.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
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
	sig.R.FillBytes(raw[:coordLen])
	sig.S.FillBytes(raw[coordLen:])
	return raw
}
