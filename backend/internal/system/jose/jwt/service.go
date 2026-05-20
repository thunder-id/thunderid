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
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolab"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pkiservice"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// JWTServiceInterface defines the interface for JWT operations.
type JWTServiceInterface interface {
	GenerateJWT(ctx context.Context, sub, iss string, validityPeriod int64,
		claims map[string]interface{}, typ, alg string) (string, int64, *serviceerror.ServiceError)
	VerifyJWT(jwtToken string, expectedAud, expectedIss string) *serviceerror.ServiceError
	VerifyJWTWithPublicKey(jwtToken string, jwtPublicKey crypto.PublicKey, expectedAud,
		expectedIss string) *serviceerror.ServiceError
	VerifyJWTWithJWKS(jwtToken, jwksURL, expectedAud, expectedIss string) *serviceerror.ServiceError
	VerifyJWTSignature(jwtToken string) *serviceerror.ServiceError
	VerifyJWTSignatureWithPublicKey(jwtToken string, jwtPublicKey crypto.PublicKey) *serviceerror.ServiceError
	VerifyJWTSignatureWithJWKS(jwtToken string, jwksURL string) *serviceerror.ServiceError
}

// jwksCacheEntry holds a cached JWKS response with its expiry time.
type jwksCacheEntry struct {
	keys      []map[string]interface{}
	expiresAt time.Time
}

// jwtService implements the JWTServiceInterface for generating and managing JWT tokens.
type jwtService struct {
	cryptoProvider kmprovider.RuntimeCryptoProvider
	keyRef         kmprovider.KeyRef
	publicKey      crypto.PublicKey
	signAlg        cryptolab.SignAlgorithm
	jwsAlg         jws.Algorithm
	kid            string
	logger         *log.Logger
	jwksCache      sync.Map
	httpClient     httpservice.HTTPClientInterface
}

// newJWTService creates a new JWT service instance.
func newJWTService(
	pkiService pkiservice.PKIServiceInterface,
	httpClient httpservice.HTTPClientInterface, cryptoProvider kmprovider.RuntimeCryptoProvider,
) (JWTServiceInterface, error) {
	preferredKid := config.GetServerRuntime().Config.JWT.PreferredKeyID

	privateKey, err := pkiService.GetPrivateKey(preferredKid)
	if err != nil {
		return nil, errors.New("failed to retrieve private key for the key id: " + preferredKid)
	}

	kid := pkiService.GetCertThumbprint(preferredKid)
	keyRef := kmprovider.KeyRef{KeyID: preferredKid}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWTService"))

	// Get algorithm based on the type of private key
	switch k := privateKey.(type) {
	case *rsa.PrivateKey:
		return &jwtService{
			cryptoProvider: cryptoProvider,
			keyRef:         keyRef,
			publicKey:      &k.PublicKey,
			signAlg:        cryptolab.RSASHA256,
			jwsAlg:         jws.RS256,
			kid:            kid,
			logger:         logger,
			httpClient:     httpClient,
		}, nil
	case *ecdsa.PrivateKey:
		// Determine ECDSA algorithm based on curve
		crvName := k.Curve.Params().Name
		switch crvName {
		case jws.P256:
			return &jwtService{
				cryptoProvider: cryptoProvider,
				keyRef:         keyRef,
				publicKey:      &k.PublicKey,
				signAlg:        cryptolab.ECDSASHA256,
				jwsAlg:         jws.ES256,
				kid:            kid,
				logger:         logger,
				httpClient:     httpClient,
			}, nil
		case jws.P384:
			return &jwtService{
				cryptoProvider: cryptoProvider,
				keyRef:         keyRef,
				publicKey:      &k.PublicKey,
				signAlg:        cryptolab.ECDSASHA384,
				jwsAlg:         jws.ES384,
				kid:            kid,
				logger:         logger,
				httpClient:     httpClient,
			}, nil
		case jws.P521:
			return &jwtService{
				cryptoProvider: cryptoProvider,
				keyRef:         keyRef,
				publicKey:      &k.PublicKey,
				signAlg:        cryptolab.ECDSASHA512,
				jwsAlg:         jws.ES512,
				kid:            kid,
				logger:         logger,
				httpClient:     httpClient,
			}, nil
		default:
			return nil, errors.New("unsupported EC curve: " + crvName + " only P-256, P-384 and P-521 are supported")
		}
	case ed25519.PrivateKey:
		return &jwtService{
			cryptoProvider: cryptoProvider,
			keyRef:         keyRef,
			publicKey:      k.Public(),
			signAlg:        cryptolab.ED25519,
			jwsAlg:         jws.EdDSA,
			kid:            kid,
			logger:         logger,
			httpClient:     httpClient,
		}, nil
	default:
		return nil, errors.New("unsupported private key type")
	}
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
) (string, int64, *serviceerror.ServiceError) {
	if ctx == nil {
		ctx = context.Background()
	}

	jwsAlg := js.jwsAlg
	if alg != "" {
		mapped, err := jws.MapAlgorithmToSignAlg(jws.Algorithm(alg))
		if err != nil || mapped != js.signAlg {
			return "", 0, &ErrorUnsupportedJWSAlgorithm
		}
		jwsAlg = jws.Algorithm(alg)
	}
	if js.cryptoProvider == nil {
		js.logger.Error("Crypto provider not initialized for JWT generation")
		return "", 0, &serviceerror.InternalServerError
	}

	// Validate that claims["aud"] is present and of an accepted type.
	audValue, hasAud := claims["aud"]
	if !hasAud {
		js.logger.Error("GenerateJWT called without aud in claims")
		return "", 0, &serviceerror.InternalServerError
	}
	switch audValue.(type) {
	case string, []string:
		// valid
	default:
		js.logger.Error("GenerateJWT called with unsupported aud type in claims")
		return "", 0, &serviceerror.InternalServerError
	}

	serverRuntime := config.GetServerRuntime()

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
		js.logger.Error("Failed to marshal JWT header: " + err.Error())
		return "", 0, &serviceerror.InternalServerError
	}

	tokenIssuer := iss
	if tokenIssuer == "" {
		tokenIssuer = serverRuntime.Config.JWT.Issuer
	}

	// Calculate the expiration time based on the validity period.
	if validityPeriod == 0 {
		validityPeriod = serverRuntime.Config.JWT.ValidityPeriod
	}
	iat := time.Now()
	expirationTime := iat.Add(time.Duration(validityPeriod) * time.Second).Unix()

	jti, err := utils.GenerateUUIDv7()
	if err != nil {
		js.logger.Error("Failed to generate UUID", log.Error(err))
		return "", 0, &serviceerror.InternalServerError
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
		js.logger.Error("Failed to marshal JWT payload: " + err.Error())
		return "", 0, &serviceerror.InternalServerError
	}

	// Encode the header and payload in base64 URL format.
	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadBase64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Create the signing input and sign it with the crypto provider.
	signingInput := headerBase64 + "." + payloadBase64
	signature, err := js.cryptoProvider.Sign(ctx, js.keyRef, js.signAlg, []byte(signingInput))
	if err != nil {
		js.logger.Error("Failed to sign JWT: " + err.Error())
		return "", 0, &serviceerror.InternalServerError
	}

	// Encode the signature in base64 URL format.
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureBase64, iat.Unix(), nil
}

// VerifyJWT verifies the JWT token using the server's public key.
func (js *jwtService) VerifyJWT(jwtToken string, expectedAud, expectedIss string) *serviceerror.ServiceError {
	if js.publicKey == nil {
		js.logger.Error("Public key not found for JWT verification")
		return &serviceerror.InternalServerError
	}

	// First verify signature using the configured server key and algorithm
	if err := js.VerifyJWTSignature(jwtToken); err != nil {
		return &ErrorInvalidTokenSignature
	}

	// Then verify claims
	return js.verifyJWTClaims(jwtToken, expectedAud, expectedIss)
}

// VerifyJWTWithPublicKey verifies the JWT token using the provided public key.
func (js *jwtService) VerifyJWTWithPublicKey(jwtToken string, jwtPublicKey crypto.PublicKey,
	expectedAud, expectedIss string) *serviceerror.ServiceError {
	parts := strings.Split(jwtToken, ".")
	if len(parts) != 3 {
		return &ErrorInvalidJWTFormat
	}

	if err := js.VerifyJWTSignatureWithPublicKey(jwtToken, jwtPublicKey); err != nil {
		return err
	}

	return js.verifyJWTClaims(jwtToken, expectedAud, expectedIss)
}

// VerifyJWTWithJWKS verifies the JWT token using a JWK Set (JWKS) endpoint.
func (js *jwtService) VerifyJWTWithJWKS(
	jwtToken, jwksURL, expectedAud, expectedIss string) *serviceerror.ServiceError {
	parts := strings.Split(jwtToken, ".")
	if len(parts) != 3 {
		return &ErrorInvalidJWTFormat
	}

	if err := js.VerifyJWTSignatureWithJWKS(jwtToken, jwksURL); err != nil {
		return &ErrorInvalidTokenSignature
	}

	return js.verifyJWTClaims(jwtToken, expectedAud, expectedIss)
}

// VerifyJWTSignature verifies the signature of a JWT token using the server's public key.
func (js *jwtService) VerifyJWTSignature(jwtToken string) *serviceerror.ServiceError {
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

	// Verify the signature using the configured algorithm
	err = cryptolab.Verify([]byte(signingInput), signature, js.signAlg, js.publicKey)
	if err != nil {
		return &ErrorInvalidTokenSignature
	}
	return nil
}

// VerifyJWTSignatureWithPublicKey verifies the signature of a JWT token using the provided public key.
func (js *jwtService) VerifyJWTSignatureWithPublicKey(jwtToken string,
	jwtPublicKey crypto.PublicKey) *serviceerror.ServiceError {
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
	err = cryptolab.Verify([]byte(signingInput), signature, alg, jwtPublicKey)
	if err != nil {
		return &ErrorInvalidTokenSignature
	}
	return nil
}

// VerifyJWTSignatureWithJWKS verifies the signature of a JWT token using a JWK Set (JWKS) endpoint.
func (js *jwtService) VerifyJWTSignatureWithJWKS(jwtToken string, jwksURL string) *serviceerror.ServiceError {
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
	keys, svcErr := js.getJWKSKeys(jwksURL)
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
		js.logger.Debug("Failed to convert JWK to public key: " + err.Error())
		return &ErrorFailedToParseJWKS
	}

	// Verify JWT signature
	if err := js.VerifyJWTSignatureWithPublicKey(jwtToken, pubKey); err != nil {
		return err
	}

	return nil
}

// getJWKSKeys returns JWKS keys for the given URL, using a TTL-based cache.
func (js *jwtService) getJWKSKeys(jwksURL string) ([]map[string]interface{}, *serviceerror.ServiceError) {
	if cached, ok := js.jwksCache.Load(jwksURL); ok {
		entry := cached.(*jwksCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.keys, nil
		}
	}

	resp, err := js.httpClient.Get(jwksURL)
	if err != nil {
		js.logger.Debug("Failed to fetch JWKS from URL: " + err.Error())
		return nil, &ErrorFailedToGetJWKS
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			js.logger.Error("Failed to close response body", log.Error(closeErr))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		js.logger.Debug("Failed to fetch JWKS, HTTP status: " + resp.Status)
		return nil, &ErrorFailedToGetJWKS
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		js.logger.Debug("Failed to read JWKS response body: " + err.Error())
		return nil, &ErrorFailedToParseJWKS
	}

	var jwks struct {
		Keys []map[string]interface{} `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		js.logger.Debug("Failed to parse JWKS JSON: " + err.Error())
		return nil, &ErrorFailedToParseJWKS
	}

	ttl := time.Duration(config.GetServerRuntime().Config.Server.SecurityConfig.JWKSCacheTTL) * time.Second
	js.jwksCache.Store(jwksURL, &jwksCacheEntry{
		keys:      jwks.Keys,
		expiresAt: time.Now().Add(ttl),
	})

	return jwks.Keys, nil
}

// verifyJWTClaims verifies the standard claims of a JWT token.
func (js *jwtService) verifyJWTClaims(jwtToken string, expectedAud, expectedIss string) *serviceerror.ServiceError {
	// Decode the JWT payload
	payload, err := DecodeJWTPayload(jwtToken)
	if err != nil {
		js.logger.Debug("Failed to decode JWT payload: " + err.Error())
		return &ErrorDecodingJWTPayload
	}

	// Get leeway from config to account for clock skew
	leeway := config.GetServerRuntime().Config.JWT.Leeway

	// Validate standard claims (exp, nbf, aud, iss)
	now := time.Now().Unix()

	if exp, ok := payload["exp"].(float64); ok {
		if now >= int64(exp)+leeway {
			js.logger.Debug("JWT token has expired")
			return &ErrorTokenExpired
		}
	} else {
		js.logger.Debug("JWT token missing 'exp' claim or it is not a number")
		return &ErrorInvalidJWTFormat
	}

	// Validate nbf only when present. Many OIDC providers omit this claim.
	if nbfRaw, ok := payload["nbf"]; ok {
		nbf, isNumber := nbfRaw.(float64)
		if !isNumber {
			js.logger.Debug("JWT token 'nbf' claim present but not a number")
			return &ErrorInvalidJWTFormat
		}
		if now < int64(nbf)-leeway {
			js.logger.Debug("JWT token is not valid yet (nbf claim)")
			return &ErrorInvalidJWTFormat
		}
	}

	if expectedAud != "" {
		switch aud := payload["aud"].(type) {
		case string:
			if aud != expectedAud {
				js.logger.Debug("Invalid audience: expected " + expectedAud + ", got " + aud)
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
				js.logger.Debug("Invalid audience: expected " + expectedAud + " not found in aud array")
				return &ErrorInvalidJWTFormat
			}
		default:
			js.logger.Debug("Missing or invalid 'aud' claim")
			return &ErrorInvalidJWTFormat
		}
	}

	if expectedIss != "" {
		if iss, ok := payload["iss"].(string); ok {
			if iss != expectedIss {
				js.logger.Debug("Invalid issuer: expected " + expectedIss + ", got " + iss)
				return &ErrorInvalidJWTFormat
			}
		} else {
			js.logger.Debug("Missing 'iss' claim or it is not a string")
			return &ErrorInvalidJWTFormat
		}
	}

	return nil
}
