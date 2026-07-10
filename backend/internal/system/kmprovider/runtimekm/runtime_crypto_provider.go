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

package runtimekm

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/runtimekm/pki"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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
) providers.RuntimeCryptoProvider {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RuntimeCryptoService"))
	return &runtimeCryptoService{
		pkiService: pkiSvc,
		cfgService: cfgSvc,
		logger:     logger,
	}
}

func (s *runtimeCryptoService) Encrypt(
	ctx context.Context, keyRef *providers.KeyRef, algorithm string, params map[string]interface{}, content []byte,
) ([]byte, *providers.CryptoDetails, error) {
	algorithmParams, err := cryptolib.AlgorithmParamsFromMap(algorithm, params)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid algorithm parameters: %w", err)
	}
	switch algorithmParams.Algorithm {
	case cryptolib.AlgorithmAESGCM:
		if s.cfgService == nil {
			return nil, nil, errors.New("config crypto service not initialized")
		}
		encrypted, err := s.cfgService.Encrypt(ctx, content)
		return encrypted, nil, err
	case cryptolib.AlgorithmRSAOAEP, cryptolib.AlgorithmRSAOAEP256:
		if keyRef == nil {
			return nil, nil, fmt.Errorf("keyRef required for %s", algorithmParams.Algorithm)
		}
		rsaPub, err := s.getRSAPublicKey(ctx, *keyRef)
		if err != nil {
			return nil, nil, err
		}
		wrappedCEK, details, err := cryptolib.Encrypt(rsaPub, &algorithmParams, content)
		return wrappedCEK, toProviderCryptoDetails(details), err
	case cryptolib.AlgorithmECDHES,
		cryptolib.AlgorithmECDHESA128KW, cryptolib.AlgorithmECDHESA192KW, cryptolib.AlgorithmECDHESA256KW:
		if keyRef == nil {
			return nil, nil, fmt.Errorf("keyRef required for %s", algorithmParams.Algorithm)
		}
		ecPub, err := s.getECPublicKey(ctx, *keyRef)
		if err != nil {
			return nil, nil, err
		}
		wrappedCEK, details, err := cryptolib.Encrypt(ecPub, &algorithmParams, content)
		return wrappedCEK, toProviderCryptoDetails(details), err
	default:
		return nil, nil, fmt.Errorf("unsupported algorithm: %s", algorithmParams.Algorithm)
	}
}

func (s *runtimeCryptoService) Decrypt(
	ctx context.Context, keyRef *providers.KeyRef, algorithm string, params map[string]interface{}, content []byte,
) ([]byte, error) {
	algorithmParams, err := cryptolib.AlgorithmParamsFromMap(algorithm, params)
	if err != nil {
		return nil, fmt.Errorf("invalid algorithm parameters: %w", err)
	}
	switch algorithmParams.Algorithm {
	case cryptolib.AlgorithmAESGCM:
		if s.cfgService == nil {
			return nil, errors.New("config crypto service not initialized")
		}
		return s.cfgService.Decrypt(ctx, content)
	case cryptolib.AlgorithmRSAOAEP, cryptolib.AlgorithmRSAOAEP256:
		if keyRef == nil {
			return nil, fmt.Errorf("keyRef required for %s", algorithmParams.Algorithm)
		}
		rsaPriv, err := s.getRSAPrivateKey(ctx, *keyRef)
		if err != nil {
			return nil, err
		}
		return cryptolib.Decrypt(rsaPriv, algorithmParams, content)
	case cryptolib.AlgorithmECDHES,
		cryptolib.AlgorithmECDHESA128KW, cryptolib.AlgorithmECDHESA192KW, cryptolib.AlgorithmECDHESA256KW:
		if keyRef == nil {
			return nil, fmt.Errorf("keyRef required for %s", algorithmParams.Algorithm)
		}
		ecPriv, err := s.getECPrivateKey(ctx, *keyRef)
		if err != nil {
			return nil, err
		}
		return cryptolib.Decrypt(ecPriv, algorithmParams, content)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithmParams.Algorithm)
	}
}

func (s *runtimeCryptoService) Sign(
	ctx context.Context, keyRef providers.KeyRef, alg string, content []byte,
) ([]byte, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}
	signAlg, err := cryptolib.SignAlgorithmFor(cryptolib.Algorithm(alg))
	if err != nil {
		return nil, fmt.Errorf("%w: %q", providers.ErrUnsupportedAlgorithm, alg)
	}
	privKey, svcErr := s.pkiService.GetPrivateKey(ctx, keyRef.KeyID)
	if svcErr != nil {
		return nil, fmt.Errorf("key not found for id %s: [%s] %s",
			keyRef.KeyID, svcErr.Code, svcErr.Error.DefaultValue)
	}
	return cryptolib.Generate(content, signAlg, privKey)
}

func (s *runtimeCryptoService) GetPublicKeys(
	ctx context.Context, filter providers.PublicKeyFilter,
) ([]providers.PublicKeyInfo, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}

	allCerts, svcErr := s.pkiService.GetAllX509Certificates(ctx)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to retrieve certificates: [%s] %s",
			svcErr.Code, svcErr.Error.DefaultValue)
	}

	keys := make([]providers.PublicKeyInfo, 0, len(allCerts))
	for id, cert := range allCerts {
		var alg cryptolib.Algorithm
		switch pub := cert.PublicKey.(type) {
		case *rsa.PublicKey:
			alg = cryptolib.AlgorithmRS256
		case *ecdsa.PublicKey:
			switch pub.Curve.Params().Name {
			case "P-256":
				alg = cryptolib.AlgorithmES256
			case "P-384":
				alg = cryptolib.AlgorithmES384
			case "P-521":
				alg = cryptolib.AlgorithmES512
			default:
				s.logger.Warn(ctx, "Unsupported EC curve; skipping",
					log.String("keyID", id),
					log.String("curve", pub.Curve.Params().Name))
				continue
			}
		case ed25519.PublicKey:
			alg = cryptolib.AlgorithmEdDSA
		default:
			s.logger.Debug(ctx, "Unsupported public key type; skipping", log.String("keyID", id))
			continue
		}

		if filter.KeyID != "" && filter.KeyID != id {
			continue
		}
		if filter.Algorithm != "" && filter.Algorithm != string(alg) {
			continue
		}

		keys = append(keys, providers.PublicKeyInfo{
			KeyID:               id,
			Algorithm:           string(alg),
			PublicKey:           cert.PublicKey,
			Thumbprint:          s.pkiService.GetCertThumbprint(id),
			CertificateDER:      cert.Raw,
			CertificateChainDER: s.pkiService.GetCertificateChain(id),
		})
	}

	return keys, nil
}

func (s *runtimeCryptoService) Verify(
	ctx context.Context, keyRef providers.KeyRef, alg string, content []byte, signature []byte,
) error {
	if s.pkiService == nil {
		return errors.New("PKI service not initialized")
	}
	signAlg, err := cryptolib.SignAlgorithmFor(cryptolib.Algorithm(alg))
	if err != nil {
		return fmt.Errorf("%w: %q", providers.ErrUnsupportedAlgorithm, alg)
	}

	publicKey := keyRef.PublicKey
	if keyRef.KeyID != "" {
		keys, err := s.GetPublicKeys(ctx, providers.PublicKeyFilter{})
		if err != nil {
			return fmt.Errorf("failed to retrieve public keys: %w", err)
		}
		for _, key := range keys {
			if key.Thumbprint == keyRef.KeyID {
				publicKey = key.PublicKey
				break
			}
		}
	}

	if publicKey != nil {
		return cryptolib.Verify(content, signature, signAlg, publicKey)
	}

	return fmt.Errorf("%w: kid=%s", providers.ErrKeyNotFound, keyRef.KeyID)
}

// IsSupportedSigningAlgorithm checks if the given signing algorithm is supported.
func (s *runtimeCryptoService) IsSupportedSigningAlgorithm(alg string) bool {
	_, err := cryptolib.SignAlgorithmFor(cryptolib.Algorithm(alg))
	return err == nil
}

// IsSupportedEncAlgorithm checks if the given encryption algorithm is supported.
func (s *runtimeCryptoService) IsSupportedEncAlgorithm(alg string) bool {
	_, err := cryptolib.EncryptionAlgorithmFor(cryptolib.Algorithm(alg))
	return err == nil
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

func (s *runtimeCryptoService) getRSAPublicKey(ctx context.Context, keyRef providers.KeyRef) (*rsa.PublicKey, error) {
	if keyRef.KeyID == "" {
		rsaPub, ok := keyRef.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("key is not an RSA public key")
		}
		return rsaPub, nil
	}
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

func (s *runtimeCryptoService) getECPublicKey(ctx context.Context, keyRef providers.KeyRef) (*ecdsa.PublicKey, error) {
	if keyRef.KeyID == "" {
		ecPub, ok := keyRef.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			return nil, errors.New("key is not an EC public key")
		}
		return ecPub, nil
	}
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
	ctx context.Context, keyRef providers.KeyRef) (*rsa.PrivateKey, error) {
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

func toProviderCryptoDetails(details *cryptolib.CryptoDetails) *providers.CryptoDetails {
	if details == nil {
		return nil
	}
	return &providers.CryptoDetails{
		EPK: details.EPK,
		CEK: details.CEK,
		IV:  details.IV,
		Tag: details.Tag,
	}
}

func (s *runtimeCryptoService) getECPrivateKey(
	ctx context.Context, keyRef providers.KeyRef) (*ecdsa.PrivateKey, error) {
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
