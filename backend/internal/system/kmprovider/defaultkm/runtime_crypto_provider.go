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

package defaultkm

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"errors"
	"fmt"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pki"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type runtimeCryptoService struct {
	pkiService pki.PKIServiceInterface
	cfgService kmprovider.ConfigCryptoProvider
	logger     *log.Logger
}

// NewRuntimeCryptoService creates a RuntimeCryptoProvider backed by the given PKI and config services.
func NewRuntimeCryptoService(
	pkiSvc pki.PKIServiceInterface,
	cfgSvc kmprovider.ConfigCryptoProvider,
) kmprovider.RuntimeCryptoProvider {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RuntimeCryptoService"))
	return &runtimeCryptoService{
		pkiService: pkiSvc,
		cfgService: cfgSvc,
		logger:     logger,
	}
}

func (s *runtimeCryptoService) Encrypt(
	ctx context.Context, keyRef *kmprovider.KeyRef, params cryptolib.AlgorithmParams, content []byte,
) ([]byte, *cryptolib.CryptoDetails, error) {
	switch params.Algorithm {
	case cryptolib.AlgorithmAESGCM:
		if s.cfgService == nil {
			return nil, nil, errors.New("config crypto service not initialized")
		}
		encrypted, err := s.cfgService.Encrypt(ctx, content)
		return encrypted, nil, err
	case cryptolib.AlgorithmRSAOAEP, cryptolib.AlgorithmRSAOAEP256:
		if keyRef == nil {
			return nil, nil, fmt.Errorf("keyRef required for %s", params.Algorithm)
		}
		rsaPub, err := s.getRSAPublicKey(ctx, *keyRef)
		if err != nil {
			return nil, nil, err
		}
		return cryptolib.Encrypt(rsaPub, &params, content)
	case cryptolib.AlgorithmECDHES,
		cryptolib.AlgorithmECDHESA128KW, cryptolib.AlgorithmECDHESA192KW, cryptolib.AlgorithmECDHESA256KW:
		if keyRef == nil {
			return nil, nil, fmt.Errorf("keyRef required for %s", params.Algorithm)
		}
		ecPub, err := s.getECPublicKey(ctx, *keyRef)
		if err != nil {
			return nil, nil, err
		}
		return cryptolib.Encrypt(ecPub, &params, content)
	default:
		return nil, nil, fmt.Errorf("unsupported algorithm: %s", params.Algorithm)
	}
}

func (s *runtimeCryptoService) Decrypt(
	ctx context.Context, keyRef *kmprovider.KeyRef, params cryptolib.AlgorithmParams, content []byte,
) ([]byte, error) {
	switch params.Algorithm {
	case cryptolib.AlgorithmAESGCM:
		if s.cfgService == nil {
			return nil, errors.New("config crypto service not initialized")
		}
		return s.cfgService.Decrypt(ctx, content)
	case cryptolib.AlgorithmRSAOAEP, cryptolib.AlgorithmRSAOAEP256:
		if keyRef == nil {
			return nil, fmt.Errorf("keyRef required for %s", params.Algorithm)
		}
		rsaPriv, err := s.getRSAPrivateKey(ctx, *keyRef)
		if err != nil {
			return nil, err
		}
		return cryptolib.Decrypt(rsaPriv, params, content)
	case cryptolib.AlgorithmECDHES,
		cryptolib.AlgorithmECDHESA128KW, cryptolib.AlgorithmECDHESA192KW, cryptolib.AlgorithmECDHESA256KW:
		if keyRef == nil {
			return nil, fmt.Errorf("keyRef required for %s", params.Algorithm)
		}
		ecPriv, err := s.getECPrivateKey(ctx, *keyRef)
		if err != nil {
			return nil, err
		}
		return cryptolib.Decrypt(ecPriv, params, content)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", params.Algorithm)
	}
}

func (s *runtimeCryptoService) Sign(
	ctx context.Context, keyRef kmprovider.KeyRef, alg string, content []byte,
) ([]byte, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}
	signAlg, err := cryptolib.SignAlgorithmFor(cryptolib.Algorithm(alg))
	if err != nil {
		return nil, fmt.Errorf("%w: %q", kmprovider.ErrUnsupportedAlgorithm, alg)
	}
	privKey, svcErr := s.pkiService.GetPrivateKey(ctx, keyRef.KeyID)
	if svcErr != nil {
		return nil, fmt.Errorf("key not found for id %s: [%s] %s",
			keyRef.KeyID, svcErr.Code, svcErr.Error.DefaultValue)
	}
	return cryptolib.Generate(content, signAlg, privKey)
}

func (s *runtimeCryptoService) GetPublicKeys(
	ctx context.Context, filter kmprovider.PublicKeyFilter,
) ([]kmprovider.PublicKeyInfo, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}

	allCerts, svcErr := s.pkiService.GetAllX509Certificates(ctx)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to retrieve certificates: [%s] %s",
			svcErr.Code, svcErr.Error.DefaultValue)
	}

	keys := make([]kmprovider.PublicKeyInfo, 0, len(allCerts))
	for id, cert := range allCerts {
		pub := cert.PublicKey
		if pub == nil {
			// ML-DSA: the standard library cannot parse the certificate's public
			// key, so derive it from the configured private key.
			derived, ok := s.derivePublicKey(ctx, id)
			if !ok {
				continue
			}
			pub = derived
		}

		var alg cryptolib.Algorithm
		switch p := pub.(type) {
		case *rsa.PublicKey:
			alg = cryptolib.AlgorithmRS256
		case *ecdsa.PublicKey:
			switch p.Curve.Params().Name {
			case "P-256":
				alg = cryptolib.AlgorithmES256
			case "P-384":
				alg = cryptolib.AlgorithmES384
			case "P-521":
				alg = cryptolib.AlgorithmES512
			default:
				s.logger.Warn(ctx, "Unsupported EC curve; skipping",
					log.String("keyID", id),
					log.String("curve", p.Curve.Params().Name))
				continue
			}
		case ed25519.PublicKey:
			alg = cryptolib.AlgorithmEdDSA
		case *mldsa44.PublicKey, *mldsa65.PublicKey, *mldsa87.PublicKey:
			// ML-DSA (RFC 9964).
			mldsaAlg, ok := cryptolib.MLDSAAlgForPublicKey(p)
			if !ok {
				s.logger.Debug(ctx, "Unsupported public key type; skipping", log.String("keyID", id))
				continue
			}
			alg = mldsaAlg
		default:
			s.logger.Debug(ctx, "Unsupported public key type; skipping", log.String("keyID", id))
			continue
		}

		if filter.KeyID != "" && filter.KeyID != id {
			continue
		}
		if filter.Algorithm != "" && filter.Algorithm != alg {
			continue
		}

		keys = append(keys, kmprovider.PublicKeyInfo{
			KeyID:               id,
			Algorithm:           alg,
			PublicKey:           pub,
			Thumbprint:          s.pkiService.GetCertThumbprint(id),
			CertificateDER:      cert.Raw,
			CertificateChainDER: s.pkiService.GetCertificateChain(id),
		})
	}

	return keys, nil
}

// derivePublicKey returns the public key derived from the configured private key
// for the given key ID. It is used for ML-DSA keys, whose certificate public key
// the standard library cannot parse.
func (s *runtimeCryptoService) derivePublicKey(ctx context.Context, id string) (crypto.PublicKey, bool) {
	privKey, svcErr := s.pkiService.GetPrivateKey(ctx, id)
	if svcErr != nil {
		s.logger.Debug(ctx, "No public key available; skipping", log.String("keyID", id))
		return nil, false
	}
	signer, ok := privKey.(crypto.Signer)
	if !ok {
		s.logger.Debug(ctx, "Unsupported private key type; skipping", log.String("keyID", id))
		return nil, false
	}
	return signer.Public(), true
}

func (s *runtimeCryptoService) Verify(
	ctx context.Context, kid string, alg string, content []byte, signature []byte,
) error {
	if s.pkiService == nil {
		return errors.New("PKI service not initialized")
	}
	signAlg, err := cryptolib.SignAlgorithmFor(cryptolib.Algorithm(alg))
	if err != nil {
		return fmt.Errorf("%w: %q", kmprovider.ErrUnsupportedAlgorithm, alg)
	}
	keys, err := s.GetPublicKeys(ctx, kmprovider.PublicKeyFilter{})
	if err != nil {
		return fmt.Errorf("failed to retrieve public keys: %w", err)
	}
	for _, key := range keys {
		if key.Thumbprint == kid {
			return cryptolib.Verify(content, signature, signAlg, key.PublicKey)
		}
	}
	return fmt.Errorf("%w: kid=%s", kmprovider.ErrKeyNotFound, kid)
}

func (s *runtimeCryptoService) GetTLSMaterial(
	ctx context.Context,
) (*kmprovider.TLSMaterial, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}
	tlsCfg, err := s.pkiService.GetTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS config: %w", err)
	}
	return &kmprovider.TLSMaterial{
		Certificate: tlsCfg.Certificates[0],
		MinVersion:  tlsCfg.MinVersion,
	}, nil
}

func (s *runtimeCryptoService) getRSAPublicKey(ctx context.Context, keyRef kmprovider.KeyRef) (*rsa.PublicKey, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}
	cert, svcErr := s.pkiService.GetX509Certificate(ctx, keyRef.KeyID)
	if svcErr != nil {
		return nil, fmt.Errorf("key not found for id %s: [%s] %s",
			keyRef.KeyID, svcErr.Code, svcErr.Error.DefaultValue)
	}
	rsaPub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("key is not an RSA public key")
	}
	return rsaPub, nil
}

func (s *runtimeCryptoService) getECPublicKey(ctx context.Context, keyRef kmprovider.KeyRef) (*ecdsa.PublicKey, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}
	cert, svcErr := s.pkiService.GetX509Certificate(ctx, keyRef.KeyID)
	if svcErr != nil {
		return nil, fmt.Errorf("key not found for id %s: [%s] %s",
			keyRef.KeyID, svcErr.Code, svcErr.Error.DefaultValue)
	}
	ecPub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("key is not an EC public key")
	}
	return ecPub, nil
}

func (s *runtimeCryptoService) getRSAPrivateKey(
	ctx context.Context, keyRef kmprovider.KeyRef) (*rsa.PrivateKey, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}
	privKey, svcErr := s.pkiService.GetPrivateKey(ctx, keyRef.KeyID)
	if svcErr != nil {
		return nil, fmt.Errorf("key not found for id %s: [%s] %s",
			keyRef.KeyID, svcErr.Code, svcErr.Error.DefaultValue)
	}
	rsaPriv, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("key is not an RSA private key")
	}
	return rsaPriv, nil
}

func (s *runtimeCryptoService) getECPrivateKey(
	ctx context.Context, keyRef kmprovider.KeyRef) (*ecdsa.PrivateKey, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}
	privKey, svcErr := s.pkiService.GetPrivateKey(ctx, keyRef.KeyID)
	if svcErr != nil {
		return nil, fmt.Errorf("key not found for id %s: [%s] %s",
			keyRef.KeyID, svcErr.Code, svcErr.Error.DefaultValue)
	}
	ecPriv, ok := privKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("key is not an EC private key")
	}
	return ecPriv, nil
}
