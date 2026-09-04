// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package jwks provides the implementation for retrieving JSON Web Key Sets (JWKS).
package jwks

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/base64"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	// Use crypto/sha1 only for JWKS x5t as required by spec for thumbprint.
	"crypto/sha1" //nolint:gosec

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// JWKSServiceInterface defines the interface for JWKS service.
type JWKSServiceInterface interface {
	GetJWKS(ctx context.Context) (*JWKSResponse, *tidcommon.ServiceError)
}

// jwksService implements the JWKSServiceInterface.
type jwksService struct {
	cryptoProvider providers.RuntimeCryptoProvider
	logger         *log.Logger
}

// newJWKSService creates a new instance of JWKSService.
func newJWKSService(cryptoProvider providers.RuntimeCryptoProvider) JWKSServiceInterface {
	return &jwksService{
		cryptoProvider: cryptoProvider,
		logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWKSService")),
	}
}

// GetJWKS retrieves the JSON Web Key Set (JWKS) from the runtime crypto provider.
func (s *jwksService) GetJWKS(ctx context.Context) (*JWKSResponse, *tidcommon.ServiceError) {
	publicKeys, err := s.cryptoProvider.GetPublicKeys(ctx, providers.PublicKeyFilter{})
	if err != nil {
		s.logger.Error(ctx, "Failed to retrieve public keys", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	if len(publicKeys) == 0 {
		return nil, &tidcommon.InternalServerError
	}

	var jwksKeys []JWKS

	for _, keyInfo := range publicKeys {
		kid := keyInfo.KeyID
		if kid == "" {
			s.logger.Error(ctx, "Public key is missing key ID")
			return nil, &tidcommon.InternalServerError
		}
		alg := keyInfo.Algorithm

		var x5c []string
		var x5t, x5tS256 string
		if len(keyInfo.CertificateDER) > 0 {
			x5c = []string{base64.StdEncoding.EncodeToString(keyInfo.CertificateDER)}
			sha1Sum := sha1.Sum(keyInfo.CertificateDER) //nolint:gosec // x5t (SHA-1 thumbprint) is required by spec
			x5t = encodeBase64URL(sha1Sum[:])
			x5tS256 = cryptolib.GenerateThumbprint(keyInfo.CertificateDER)
		}

		switch pub := keyInfo.PublicKey.(type) {
		case *rsa.PublicKey:
			jwksKeys = append(jwksKeys, getRSAPublicKeyJWKS(pub, kid, alg, x5c, x5t, x5tS256))
		case *ecdsa.PublicKey:
			jwksKeys = append(jwksKeys, getECDSAPublicKeyJWKS(pub, kid, alg, x5c, x5t, x5tS256))
		case ed25519.PublicKey:
			jwksKeys = append(jwksKeys, getEdDSAPublicKeyJWKS(pub, kid, alg, x5c, x5t, x5tS256))
		case *mldsa44.PublicKey, *mldsa65.PublicKey, *mldsa87.PublicKey:
			// ML-DSA (RFC 9964 AKP).
			mldsaJWK, ok := getMLDSAPublicKeyJWKS(pub, kid, alg, x5c, x5t, x5tS256)
			if !ok {
				s.logger.Debug(ctx, "Unsupported public key type for JWKS", log.String("keyID", keyInfo.KeyID))
				continue
			}
			jwksKeys = append(jwksKeys, mldsaJWK)
		default:
			s.logger.Debug(ctx, "Unsupported public key type for JWKS", log.String("keyID", keyInfo.KeyID))
			continue
		}
	}

	if len(jwksKeys) == 0 {
		return nil, &tidcommon.InternalServerError
	}

	return &JWKSResponse{
		Keys: jwksKeys,
	}, nil
}

// getRSAPublicKeyJWKS converts an RSA public key to JWKS format.
func getRSAPublicKeyJWKS(pub *rsa.PublicKey, kid, alg string, x5c []string, x5t, x5tS256 string) JWKS {
	n := encodeBase64URL(pub.N.Bytes())
	// Properly encode the exponent as a big-endian byte slice, trimmed of leading zeros
	eBytes := make([]byte, 0, 8)
	e := pub.E
	for e > 0 {
		eBytes = append([]byte{byte(e & 0xff)}, eBytes...)
		e >>= 8
	}
	if len(eBytes) == 0 {
		eBytes = []byte{0}
	}
	eEnc := encodeBase64URL(eBytes)

	return JWKS{
		Kid:     kid,
		Kty:     "RSA",
		Use:     "sig",
		Alg:     alg,
		N:       n,
		E:       eEnc,
		X5c:     x5c,
		X5t:     x5t,
		X5tS256: x5tS256,
	}
}

// getECDSAPublicKeyJWKS converts an ECDSA public key to JWKS format.
func getECDSAPublicKeyJWKS(pub *ecdsa.PublicKey, kid, alg string, x5c []string, x5t, x5tS256 string) JWKS {
	crv := pub.Curve.Params().Name
	coordLen := (pub.Curve.Params().BitSize + 7) / 8
	x := encodeBase64URL(padCoordinate(pub.X.Bytes(), coordLen))
	y := encodeBase64URL(padCoordinate(pub.Y.Bytes(), coordLen))

	return JWKS{
		Kid:     kid,
		Kty:     "EC",
		Use:     "sig",
		Alg:     alg,
		Crv:     crv,
		X:       x,
		Y:       y,
		X5c:     x5c,
		X5t:     x5t,
		X5tS256: x5tS256,
	}
}

// getEdDSAPublicKeyJWKS converts an EdDSA public key to JWKS format.
func getEdDSAPublicKeyJWKS(pub ed25519.PublicKey, kid, alg string, x5c []string, x5t, x5tS256 string) JWKS {
	x := encodeBase64URL(pub)

	return JWKS{
		Kid:     kid,
		Kty:     "OKP",
		Use:     "sig",
		Alg:     alg,
		Crv:     "Ed25519",
		X:       x,
		X5c:     x5c,
		X5t:     x5t,
		X5tS256: x5tS256,
	}
}

// getMLDSAPublicKeyJWKS converts an ML-DSA public key to an AKP JWK (RFC 9964).
// It reports false when pub is not an ML-DSA public key.
func getMLDSAPublicKeyJWKS(pub crypto.PublicKey, kid, alg string, x5c []string, x5t, x5tS256 string) (JWKS, bool) {
	pubBytes, ok := cryptolib.MLDSAPublicKeyBytes(pub)
	if !ok {
		return JWKS{}, false
	}

	return JWKS{
		Kid:     kid,
		Kty:     "AKP",
		Use:     "sig",
		Alg:     alg,
		Pub:     encodeBase64URL(pubBytes),
		X5c:     x5c,
		X5t:     x5t,
		X5tS256: x5tS256,
	}, true
}

func encodeBase64URL(b []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

// padCoordinate left-pads an EC coordinate with zero bytes to the curve's fixed
// coordinate size, per RFC 7518 Section 6.2.1.2.
func padCoordinate(b []byte, coordLen int) []byte {
	if len(b) >= coordLen {
		return b
	}
	padded := make([]byte, coordLen)
	copy(padded[coordLen-len(b):], b)
	return padded
}
