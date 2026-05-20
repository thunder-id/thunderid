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

// Package jwks provides the implementation for retrieving JSON Web Key Sets (JWKS).
package jwks

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/base64"
	"strings"

	// Use crypto/sha1 only for JWKS x5t as required by spec for thumbprint.
	"crypto/sha1" //nolint:gosec

	"github.com/thunder-id/thunderid/internal/system/cryptolab/hash"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// JWKSServiceInterface defines the interface for JWKS service.
type JWKSServiceInterface interface {
	GetJWKS() (*JWKSResponse, *serviceerror.ServiceError)
}

// jwksService implements the JWKSServiceInterface.
type jwksService struct {
	cryptoProvider kmprovider.RuntimeCryptoProvider
	logger         *log.Logger
}

// newJWKSService creates a new instance of JWKSService.
func newJWKSService(cryptoProvider kmprovider.RuntimeCryptoProvider) JWKSServiceInterface {
	return &jwksService{
		cryptoProvider: cryptoProvider,
		logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWKSService")),
	}
}

// GetJWKS retrieves the JSON Web Key Set (JWKS) from the runtime crypto provider.
func (s *jwksService) GetJWKS() (*JWKSResponse, *serviceerror.ServiceError) {
	publicKeys, err := s.cryptoProvider.GetPublicKeys(context.Background(), kmprovider.PublicKeyFilter{})
	if err != nil {
		s.logger.Error("Failed to retrieve public keys", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	if len(publicKeys) == 0 {
		return nil, &serviceerror.InternalServerError
	}

	var jwksKeys []JWKS

	for _, keyInfo := range publicKeys {
		kid := keyInfo.Thumbprint

		var x5c []string
		var x5t, x5tS256 string
		if len(keyInfo.CertificateDER) > 0 {
			x5c = []string{base64.StdEncoding.EncodeToString(keyInfo.CertificateDER)}
			sha1Sum := sha1.Sum(keyInfo.CertificateDER) //nolint:gosec // x5t (SHA-1 thumbprint) is required by spec
			x5t = encodeBase64URL(sha1Sum[:])
			x5tS256 = hash.GenerateThumbprint(keyInfo.CertificateDER)
		}

		switch pub := keyInfo.PublicKey.(type) {
		case *rsa.PublicKey:
			jwksKeys = append(jwksKeys, getRSAPublicKeyJWKS(pub, kid, x5c, x5t, x5tS256))
		case *ecdsa.PublicKey:
			jwksKeys = append(jwksKeys, getECDSAPublicKeyJWKS(pub, kid, x5c, x5t, x5tS256))
		case ed25519.PublicKey:
			jwksKeys = append(jwksKeys, getEdDSAPublicKeyJWKS(pub, kid, x5c, x5t, x5tS256))
		default:
			s.logger.Debug("Unsupported public key type for JWKS", log.String("keyID", keyInfo.KeyID))
			continue
		}
	}

	if len(jwksKeys) == 0 {
		return nil, &serviceerror.InternalServerError
	}

	return &JWKSResponse{
		Keys: jwksKeys,
	}, nil
}

// getRSAPublicKeyJWKS converts an RSA public key to JWKS format.
func getRSAPublicKeyJWKS(pub *rsa.PublicKey, kid string, x5c []string, x5t, x5tS256 string) JWKS {
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
		Alg:     "RS256",
		N:       n,
		E:       eEnc,
		X5c:     x5c,
		X5t:     x5t,
		X5tS256: x5tS256,
	}
}

// getECDSAPublicKeyJWKS converts an ECDSA public key to JWKS format.
func getECDSAPublicKeyJWKS(pub *ecdsa.PublicKey, kid string, x5c []string, x5t, x5tS256 string) JWKS {
	crv := pub.Curve.Params().Name
	x := encodeBase64URL(pub.X.Bytes())
	y := encodeBase64URL(pub.Y.Bytes())

	alg := "ES256"
	switch crv {
	case "P-384":
		alg = "ES384"
	case "P-521":
		alg = "ES512"
	}

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
func getEdDSAPublicKeyJWKS(pub ed25519.PublicKey, kid string, x5c []string, x5t, x5tS256 string) JWKS {
	x := encodeBase64URL(pub)

	return JWKS{
		Kid:     kid,
		Kty:     "OKP",
		Use:     "sig",
		Alg:     "EdDSA",
		Crv:     "Ed25519",
		X:       x,
		X5c:     x5c,
		X5t:     x5t,
		X5tS256: x5tS256,
	}
}

func encodeBase64URL(b []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}
