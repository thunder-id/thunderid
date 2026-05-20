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

// Package jwksresolver provides utilities for resolving RSA encryption keys from
// a relying party's JWKS (inline or remote URI). It is intentionally RSA- and
// encryption-specific and is not a general-purpose JWK resolver.
package jwksresolver

import (
	"context"
	"crypto"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	certmodel "github.com/thunder-id/thunderid/internal/cert"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// KeyUsePolicy controls how the "use" field in a JWK is interpreted when selecting an encryption key.
type KeyUsePolicy int

const (
	// KeyUseStrictEnc requires the JWK to have use == "enc". Used by UserInfo encryption.
	KeyUseStrictEnc KeyUsePolicy = iota
	// KeyUseLenientEnc allows a JWK with an absent "use" field per RFC 7517 §4.2. Used by ID token encryption.
	KeyUseLenientEnc
)

// Resolver resolves an RP's RSA public key from an application certificate configuration.
// It supports inline JWKS and remote JWKS URIs.
//
// The resolver propagates ctx to HTTP requests but does not set its own timeout —
// the provided httpClient must already be configured with appropriate timeouts.
type Resolver struct {
	httpClient syshttp.HTTPClientInterface
	logger     *log.Logger
}

// newJWKSResolver creates a new Resolver. httpClient must be pre-configured with timeouts.
func newJWKSResolver(httpClient syshttp.HTTPClientInterface) *Resolver {
	return &Resolver{
		httpClient: httpClient,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWKSResolver")),
	}
}

// ResolveEncryptionKey resolves the RP's RSA public key from the application certificate.
// It returns the public key and the kid from the matching JWK entry (empty string when absent).
// encryptionAlg is used to filter incompatible keys. policy controls "use" field strictness.
func (r *Resolver) ResolveEncryptionKey(
	ctx context.Context,
	certificate *inboundmodel.Certificate,
	encryptionAlg string,
	policy KeyUsePolicy,
) (crypto.PublicKey, string, *serviceerror.ServiceError) {
	if certificate == nil || certificate.Type == "" {
		r.logger.Error("No certificate configured for encryption key resolution")
		return nil, "", &serviceerror.InternalServerError
	}

	var jwksData []byte
	switch certificate.Type {
	case certmodel.CertificateTypeJWKS:
		jwksData = []byte(certificate.Value)
	case certmodel.CertificateTypeJWKSURI:
		body, svcErr := r.fetchJWKS(ctx, certificate.Value)
		if svcErr != nil {
			return nil, "", svcErr
		}
		jwksData = body
	default:
		r.logger.Error("Unsupported certificate type for encryption key resolution",
			log.String("type", string(certificate.Type)))
		return nil, "", &serviceerror.InternalServerError
	}

	return r.parseEncryptionKeyFromJWKS(jwksData, encryptionAlg, policy)
}

// fetchJWKS fetches the JWKS document from the given URI with SSRF protection and a 1 MB size cap.
// It does not log JWKS body, key material, or HTTP response headers.
func (r *Resolver) fetchJWKS(ctx context.Context, jwksURI string) ([]byte, *serviceerror.ServiceError) {
	if r.httpClient == nil {
		r.logger.Error("HTTP client is not configured for JWKS resolver")
		return nil, &serviceerror.InternalServerError
	}
	if err := syshttp.IsSSRFSafeURL(jwksURI); err != nil {
		r.logger.Error("JWKS URI is not SSRF-safe", log.String("endpoint", jwksEndpoint(jwksURI)), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		r.logger.Error("Failed to build JWKS request", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		r.logger.Error("Failed to fetch JWKS from URI", log.String("endpoint", jwksEndpoint(jwksURI)), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		r.logger.Error("JWKS URI returned non-200 status",
			log.String("endpoint", jwksEndpoint(jwksURI)), log.Int("statusCode", resp.StatusCode))
		return nil, &serviceerror.InternalServerError
	}
	const maxJWKSBytes = 1 << 20 // 1 MB
	limitedReader := io.LimitReader(resp.Body, maxJWKSBytes+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		r.logger.Error("Failed to read JWKS response body", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if len(body) > maxJWKSBytes {
		r.logger.Error("JWKS URI response exceeds 1 MB size limit", log.String("endpoint", jwksEndpoint(jwksURI)))
		return nil, &serviceerror.InternalServerError
	}
	return body, nil
}

// parseEncryptionKeyFromJWKS finds the first RSA enc key in the JWKS that matches encryptionAlg.
// Returns the public key and its kid (empty when absent in the JWK entry).
func (r *Resolver) parseEncryptionKeyFromJWKS(
	jwksData []byte,
	encryptionAlg string,
	policy KeyUsePolicy,
) (crypto.PublicKey, string, *serviceerror.ServiceError) {
	var jwksObj struct {
		Keys []map[string]interface{} `json:"keys"`
	}
	if err := json.Unmarshal(jwksData, &jwksObj); err != nil {
		r.logger.Error("Failed to parse JWKS for encryption key resolution", log.Error(err))
		return nil, "", &serviceerror.InternalServerError
	}

	for _, key := range jwksObj.Keys {
		use, _ := key["use"].(string)
		if policy == KeyUseStrictEnc {
			// Strict: require use == "enc" (existing UserInfo behavior).
			if use != "enc" {
				continue
			}
		} else {
			// Lenient: allow absent "use" per RFC 7517 §4.2 — only skip if explicitly non-enc.
			if use != "" && use != "enc" {
				continue
			}
		}
		kty, _ := key["kty"].(string)
		if kty != "RSA" {
			continue
		}
		// Only filter by alg when the field is explicitly present.
		if keyAlg, _ := key["alg"].(string); keyAlg != "" && keyAlg != encryptionAlg {
			continue
		}
		pub, err := jws.JWKToPublicKey(key)
		if err == nil && pub != nil {
			kid, _ := key["kid"].(string)
			return pub, kid, nil
		}
	}

	r.logger.Error("No suitable RSA encryption key found in JWKS", log.String("alg", encryptionAlg))
	return nil, "", &serviceerror.InternalServerError
}

// jwksEndpoint returns the scheme and host of the given URI for safe log output,
// omitting path, query, and fragment to avoid leaking credentials or tokens.
func jwksEndpoint(uri string) string {
	if uri == "" {
		return "(empty)"
	}
	if u, err := url.Parse(uri); err == nil {
		return u.Scheme + "://" + u.Host
	}
	return "(unparseable)"
}
