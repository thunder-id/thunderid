/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

// Package jwt provides functionality for generating and managing JWT tokens.
package jwt

import (
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// JWTServiceInterface defines the interface for JWT operations.
type JWTServiceInterface interface {
	GenerateJWT(ctx context.Context, sub, iss string, validityPeriod int64,
		claims map[string]interface{}, typ, alg string) (string, int64, *tidcommon.ServiceError)
	VerifyJWT(ctx context.Context, jwtToken string, expectedAud, expectedIss string) *tidcommon.ServiceError
	VerifyJWTWithPublicKey(ctx context.Context, jwtToken string, jwtPublicKey crypto.PublicKey, expectedAud,
		expectedIss string) *tidcommon.ServiceError
	VerifyJWTWithJWKS(ctx context.Context,
		jwtToken, jwksURL, expectedAud, expectedIss string) *tidcommon.ServiceError
	VerifyJWTSignature(ctx context.Context, jwtToken string) *tidcommon.ServiceError
	VerifyJWTSignatureWithPublicKey(jwtToken string, jwtPublicKey crypto.PublicKey) *tidcommon.ServiceError
	VerifyJWTSignatureWithJWKS(ctx context.Context, jwtToken string, jwksURL string) *tidcommon.ServiceError
}

// jwksCacheEntry holds a cached JWKS response with its expiry time.
type jwksCacheEntry struct {
	keys      []map[string]interface{}
	expiresAt time.Time
}

// jwtService implements the JWTServiceInterface for generating and managing JWT tokens.
type jwtService struct {
	cryptoProvider kmprovider.RuntimeCryptoProvider
	cfg            joseconfig.Config
	keyRef         kmprovider.KeyRef
	jwsAlg         jws.Algorithm
	kid            string
	logger         *log.Logger
	jwksCache      sync.Map
	httpClient     httpservice.HTTPClientInterface
}

// newJWTService creates a new JWT service instance.
func newJWTService(
	httpClient httpservice.HTTPClientInterface, cryptoProvider kmprovider.RuntimeCryptoProvider,
	cfg joseconfig.Config,
) (JWTServiceInterface, error) {
	preferredKid := cfg.PreferredKeyID
	keyRef := kmprovider.KeyRef{KeyID: preferredKid}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWTService"))

	keys, err := cryptoProvider.GetPublicKeys(context.Background(), kmprovider.PublicKeyFilter{KeyID: preferredKid})
	if err != nil {
		return nil, errors.New("failed to retrieve public key for the key id: " + preferredKid)
	}
	if len(keys) == 0 {
		return nil, errors.New("no public key found for the key id: " + preferredKid)
	}
	key := keys[0]

	if _, err := jws.MapAlgorithmToSignAlg(jws.Algorithm(key.Algorithm)); err != nil {
		return nil, errors.New("unsupported algorithm for key id: " + preferredKid)
	}

	return &jwtService{
		cryptoProvider: cryptoProvider,
		cfg:            cfg,
		keyRef:         keyRef,
		jwsAlg:         jws.Algorithm(key.Algorithm),
		kid:            key.Thumbprint,
		logger:         logger,
		httpClient:     httpClient,
	}, nil
}

// GenerateJWT generates a JWT signed with the server's private key.
// The typ parameter sets the JWT header "typ" field. If empty, defaults to "JWT".
// The alg parameter overrides the signing algorithm (e.g. "RS256"). When empty, the server's
// default algorithm is used. When set but incompatible with the server's private key,
// ErrorUnsupportedJWSAlgorithm is returned.
// claims["aud"] must be set by the caller as either a string or []string; omitting it
// or providing another type is a programmer error and returns InternalServerError.
func (js *jwtService) GenerateJWT(
	ctx context.Context, sub, iss string, validityPeriod int64, claims map[string]interface{}, typ, alg string,
) (string, int64, *tidcommon.ServiceError) {
	jwsAlg := js.jwsAlg
	if alg != "" {
		if alg != string(js.jwsAlg) {
			return "", 0, &ErrorUnsupportedJWSAlgorithm
		}
		jwsAlg = jws.Algorithm(alg)
	}
	if js.cryptoProvider == nil {
		js.logger.Error(ctx, "Crypto provider not initialized for JWT generation")
		return "", 0, &tidcommon.InternalServerError
	}

	// Validate that claims["aud"] is present and of an accepted type.
	audValue, hasAud := claims["aud"]
	if !hasAud {
		js.logger.Error(ctx, "GenerateJWT called without aud in claims")
		return "", 0, &tidcommon.InternalServerError
	}
	switch audValue.(type) {
	case string, []string:
		// valid
	default:
		js.logger.Error(ctx, "GenerateJWT called with unsupported aud type in claims")
		return "", 0, &tidcommon.InternalServerError
	}

	// Create the JWT header.
	if typ == "" {
		typ = TokenTypeJWT
	}
	header := map[string]string{
		"alg": string(jwsAlg),
		"typ": typ,
		"kid": js.kid,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		js.logger.Error(ctx, "Failed to marshal JWT header: "+err.Error())
		return "", 0, &tidcommon.InternalServerError
	}

	tokenIssuer := iss
	if tokenIssuer == "" {
		tokenIssuer = js.cfg.Issuer
	}

	// Calculate the expiration time based on the validity period.
	if validityPeriod == 0 {
		validityPeriod = js.cfg.ValidityPeriod
	}
	iat := time.Now()
	expirationTime := iat.Add(time.Duration(validityPeriod) * time.Second).Unix()

	jti, err := utils.GenerateUUIDv7()
	if err != nil {
		js.logger.Error(ctx, "Failed to generate UUID", log.Error(err))
		return "", 0, &tidcommon.InternalServerError
	}

	// Create the JWT payload.
	payload := map[string]interface{}{
		"sub": sub,
		"iss": tokenIssuer,
		"exp": expirationTime,
		"iat": iat.Unix(),
		"nbf": iat.Unix(),
		"jti": jti,
	}

	// Add custom claims if provided.
	if len(claims) > 0 {
		for key, value := range claims {
			payload[key] = value
		}
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		js.logger.Error(ctx, "Failed to marshal JWT payload: "+err.Error())
		return "", 0, &tidcommon.InternalServerError
	}

	// Encode the header and payload in base64 URL format.
	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadBase64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Create the signing input and sign it with the crypto provider.
	signingInput := headerBase64 + "." + payloadBase64
	signature, err := js.cryptoProvider.Sign(ctx, js.keyRef, string(jwsAlg), []byte(signingInput))
	if err != nil {
		js.logger.Error(ctx, "Failed to sign JWT: "+err.Error())
		return "", 0, &tidcommon.InternalServerError
	}

	// Encode the signature in base64 URL format.
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureBase64, iat.Unix(), nil
}

// VerifyJWT verifies the JWT token using the server's public key.
func (js *jwtService) VerifyJWT(
	ctx context.Context, jwtToken string, expectedAud, expectedIss string,
) *tidcommon.ServiceError {
	if js.cryptoProvider == nil {
		js.logger.Error(ctx, "Crypto provider not initialized for JWT verification")
		return &tidcommon.InternalServerError
	}

	// First verify signature using the configured server key and algorithm
	if err := js.VerifyJWTSignature(ctx, jwtToken); err != nil {
		return &ErrorInvalidTokenSignature
	}

	// Then verify claims
	return js.verifyJWTClaims(ctx, jwtToken, expectedAud, expectedIss)
}

// VerifyJWTWithPublicKey verifies the JWT token using the provided public key.
func (js *jwtService) VerifyJWTWithPublicKey(ctx context.Context, jwtToken string, jwtPublicKey crypto.PublicKey,
	expectedAud, expectedIss string) *tidcommon.ServiceError {
	parts := strings.Split(jwtToken, ".")
	if len(parts) != 3 {
		return &ErrorInvalidJWTFormat
	}

	if err := js.VerifyJWTSignatureWithPublicKey(jwtToken, jwtPublicKey); err != nil {
		return err
	}

	return js.verifyJWTClaims(ctx, jwtToken, expectedAud, expectedIss)
}

// VerifyJWTWithJWKS verifies the JWT token using a JWK Set (JWKS) endpoint.
func (js *jwtService) VerifyJWTWithJWKS(ctx context.Context,
	jwtToken, jwksURL, expectedAud, expectedIss string) *tidcommon.ServiceError {
	parts := strings.Split(jwtToken, ".")
	if len(parts) != 3 {
		return &ErrorInvalidJWTFormat
	}

	if err := js.VerifyJWTSignatureWithJWKS(ctx, jwtToken, jwksURL); err != nil {
		return &ErrorInvalidTokenSignature
	}

	return js.verifyJWTClaims(ctx, jwtToken, expectedAud, expectedIss)
}

// VerifyJWTSignature verifies the signature of a JWT token using the server's public key.
func (js *jwtService) VerifyJWTSignature(ctx context.Context, jwtToken string) *tidcommon.ServiceError {
	if js.cryptoProvider == nil {
		js.logger.Error(ctx, "Crypto provider not initialized for JWT verification")
		return &tidcommon.InternalServerError
	}
	parts := strings.Split(jwtToken, ".")
	if len(parts) != 3 {
		return &ErrorInvalidJWTFormat
	}

	// Decode the signature
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return &ErrorInvalidTokenSignature
	}

	// Create the signing input
	signingInput := parts[0] + "." + parts[1]

	// Determine kid and algorithm from JWT header
	header, err := DecodeJWTHeader(jwtToken)
	if err != nil {
		return &ErrorDecodingJWTHeader
	}
	kid, _ := header["kid"].(string)
	algStr, _ := header["alg"].(string)

	// Verify the signature through the provider (resolves kid to key and validates alg internally).
	if err = js.cryptoProvider.Verify(ctx, kid, algStr, []byte(signingInput), signature); err != nil {
		if errors.Is(err, kmprovider.ErrKeyNotFound) {
			return &ErrorNoMatchingJWKFound
		}
		if errors.Is(err, kmprovider.ErrUnsupportedAlgorithm) {
			return &ErrorUnsupportedJWSAlgorithm
		}
		return &ErrorInvalidTokenSignature
	}
	return nil
}

// VerifyJWTSignatureWithPublicKey verifies the signature of a JWT token using the provided public key.
func (js *jwtService) VerifyJWTSignatureWithPublicKey(jwtToken string,
	jwtPublicKey crypto.PublicKey) *tidcommon.ServiceError {
	parts := strings.Split(jwtToken, ".")
	if len(parts) != 3 {
		return &ErrorInvalidJWTFormat
	}

	// Decode the signature
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return &ErrorInvalidTokenSignature
	}

	// Create the signing input
	signingInput := parts[0] + "." + parts[1]

	// Determine algorithm from JWT header
	header, err := DecodeJWTHeader(jwtToken)
	if err != nil {
		return &ErrorDecodingJWTHeader
	}
	algStr, _ := header["alg"].(string)
	alg, err := jws.MapAlgorithmToSignAlg(jws.Algorithm(algStr))
	if err != nil {
		return &ErrorUnsupportedJWSAlgorithm
	}

	// Verify the signature
	err = cryptolib.Verify([]byte(signingInput), signature, alg, jwtPublicKey)
	if err != nil {
		return &ErrorInvalidTokenSignature
	}
	return nil
}

// VerifyJWTSignatureWithJWKS verifies the signature of a JWT token using a JWK Set (JWKS) endpoint.
func (js *jwtService) VerifyJWTSignatureWithJWKS(
	ctx context.Context, jwtToken string, jwksURL string) *tidcommon.ServiceError {
	// Get the key ID from the JWT header
	header, err := DecodeJWTHeader(jwtToken)
	if err != nil {
		return &ErrorDecodingJWTHeader
	}

	kid, ok := header["kid"].(string)
	if !ok {
		return &ErrorDecodingJWTHeader
	}

	// Get JWKS keys (from cache or fetch)
	keys, svcErr := js.getJWKSKeys(ctx, jwksURL)
	if svcErr != nil {
		return svcErr
	}

	// Find the key with matching kid
	var jwk map[string]interface{}
	for _, key := range keys {
		if keyID, ok := key["kid"].(string); ok && keyID == kid {
			jwk = key
			break
		}
	}
	if jwk == nil {
		return &ErrorNoMatchingJWKFound
	}

	// Convert JWK to public key
	pubKey, err := jws.JWKToPublicKey(jwk)
	if err != nil {
		js.logger.Debug(ctx, "Failed to convert JWK to public key: "+err.Error())
		return &ErrorFailedToParseJWKS
	}

	// Verify JWT signature
	if err := js.VerifyJWTSignatureWithPublicKey(jwtToken, pubKey); err != nil {
		return err
	}

	return nil
}

// getJWKSKeys returns JWKS keys for the given URL, using a TTL-based cache.
func (js *jwtService) getJWKSKeys(
	ctx context.Context, jwksURL string) ([]map[string]interface{}, *tidcommon.ServiceError) {
	if cached, ok := js.jwksCache.Load(jwksURL); ok {
		entry := cached.(*jwksCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.keys, nil
		}
	}

	resp, err := js.httpClient.Get(jwksURL)
	if err != nil {
		js.logger.Debug(ctx, "Failed to fetch JWKS from URL: "+err.Error())
		return nil, &ErrorFailedToGetJWKS
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			js.logger.Error(ctx, "Failed to close response body", log.Error(closeErr))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		js.logger.Debug(ctx, "Failed to fetch JWKS, HTTP status: "+resp.Status)
		return nil, &ErrorFailedToGetJWKS
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		js.logger.Debug(ctx, "Failed to read JWKS response body: "+err.Error())
		return nil, &ErrorFailedToParseJWKS
	}

	var jwks struct {
		Keys []map[string]interface{} `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		js.logger.Debug(ctx, "Failed to parse JWKS JSON: "+err.Error())
		return nil, &ErrorFailedToParseJWKS
	}

	js.jwksCache.Store(jwksURL, &jwksCacheEntry{
		keys:      jwks.Keys,
		expiresAt: time.Now().Add(js.cfg.JWKSCacheTTL),
	})

	return jwks.Keys, nil
}

// verifyJWTClaims verifies the standard claims of a JWT token.
func (js *jwtService) verifyJWTClaims(
	ctx context.Context, jwtToken string, expectedAud, expectedIss string) *tidcommon.ServiceError {
	// Decode the JWT payload
	payload, err := DecodeJWTPayload(jwtToken)
	if err != nil {
		js.logger.Debug(ctx, "Failed to decode JWT payload: "+err.Error())
		return &ErrorDecodingJWTPayload
	}

	// Get leeway from config to account for clock skew
	leeway := js.cfg.Leeway

	// Validate standard claims (exp, nbf, aud, iss)
	now := time.Now().Unix()

	if exp, ok := payload["exp"].(float64); ok {
		if now >= int64(exp)+leeway {
			js.logger.Debug(ctx, "JWT token has expired")
			return &ErrorTokenExpired
		}
	} else {
		js.logger.Debug(ctx, "JWT token missing 'exp' claim or it is not a number")
		return &ErrorInvalidJWTFormat
	}

	// Validate nbf only when present. Many OIDC providers omit this claim.
	if nbfRaw, ok := payload["nbf"]; ok {
		nbf, isNumber := nbfRaw.(float64)
		if !isNumber {
			js.logger.Debug(ctx, "JWT token 'nbf' claim present but not a number")
			return &ErrorInvalidJWTFormat
		}
		if now < int64(nbf)-leeway {
			js.logger.Debug(ctx, "JWT token is not valid yet (nbf claim)")
			return &ErrorInvalidJWTFormat
		}
	}

	if expectedAud != "" {
		switch aud := payload["aud"].(type) {
		case string:
			if aud != expectedAud {
				js.logger.Debug(ctx, "Invalid audience: expected "+expectedAud+", got "+aud)
				return &ErrorInvalidJWTFormat
			}
		case []interface{}:
			found := false
			for _, v := range aud {
				if s, ok := v.(string); ok && s == expectedAud {
					found = true
					break
				}
			}
			if !found {
				js.logger.Debug(ctx, "Invalid audience: expected "+expectedAud+" not found in aud array")
				return &ErrorInvalidJWTFormat
			}
		default:
			js.logger.Debug(ctx, "Missing or invalid 'aud' claim")
			return &ErrorInvalidJWTFormat
		}
	}

	if expectedIss != "" {
		if iss, ok := payload["iss"].(string); ok {
			if iss != expectedIss {
				js.logger.Debug(ctx, "Invalid issuer: expected "+expectedIss+", got "+iss)
				return &ErrorInvalidJWTFormat
			}
		} else {
			js.logger.Debug(ctx, "Missing 'iss' claim or it is not a string")
			return &ErrorInvalidJWTFormat
		}
	}

	return nil
}
