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
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/cryptolab"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pkiservice"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type runtimeCryptoService struct {
	pkiService pkiservice.PKIServiceInterface
	cfgService kmprovider.ConfigCryptoProvider
	logger     *log.Logger
}

// NewRuntimeCryptoService creates a RuntimeCryptoProvider backed by the given PKI and config services.
func NewRuntimeCryptoService(
	pkiSvc pkiservice.PKIServiceInterface,
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
	ctx context.Context, keyRef *kmprovider.KeyRef, params cryptolab.AlgorithmParams, content []byte,
) ([]byte, *cryptolab.CryptoDetails, error) {
	switch params.Algorithm {
	case cryptolab.AlgorithmAESGCM:
		if s.cfgService == nil {
			return nil, nil, errors.New("config crypto service not initialized")
		}
		encrypted, err := s.cfgService.Encrypt(ctx, content)
		return encrypted, nil, err
	case cryptolab.AlgorithmRSAOAEP256:
		if keyRef == nil {
			return nil, nil, errors.New("keyRef required for RSA-OAEP-256")
		}
		rsaPub, err := s.getRSAPublicKey(*keyRef)
		if err != nil {
			return nil, nil, err
		}
		return cryptolab.Encrypt(rsaPub, &params, content)
	case cryptolab.AlgorithmECDHES, cryptolab.AlgorithmECDHESA128KW, cryptolab.AlgorithmECDHESA256KW:
		if keyRef == nil {
			return nil, nil, errors.New("keyRef required for ECDH-ES")
		}
		ecPub, err := s.getECPublicKey(*keyRef)
		if err != nil {
			return nil, nil, err
		}
		return cryptolab.Encrypt(ecPub, &params, content)
	default:
		return nil, nil, fmt.Errorf("unsupported algorithm: %s", params.Algorithm)
	}
}

func (s *runtimeCryptoService) Decrypt(
	ctx context.Context, keyRef *kmprovider.KeyRef, params cryptolab.AlgorithmParams, content []byte,
) ([]byte, error) {
	switch params.Algorithm {
	case cryptolab.AlgorithmAESGCM:
		if s.cfgService == nil {
			return nil, errors.New("config crypto service not initialized")
		}
		return s.cfgService.Decrypt(ctx, content)
	case cryptolab.AlgorithmRSAOAEP256:
		if keyRef == nil {
			return nil, errors.New("keyRef required for RSA-OAEP-256")
		}
		rsaPriv, err := s.getRSAPrivateKey(*keyRef)
		if err != nil {
			return nil, err
		}
		return cryptolab.Decrypt(rsaPriv, params, content)
	case cryptolab.AlgorithmECDHES, cryptolab.AlgorithmECDHESA128KW, cryptolab.AlgorithmECDHESA256KW:
		if keyRef == nil {
			return nil, errors.New("keyRef required for ECDH-ES")
		}
		ecPriv, err := s.getECPrivateKey(*keyRef)
		if err != nil {
			return nil, err
		}
		return cryptolab.Decrypt(ecPriv, params, content)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", params.Algorithm)
	}
}

func (s *runtimeCryptoService) Sign(
	_ context.Context, keyRef kmprovider.KeyRef, algorithm cryptolab.SignAlgorithm, content []byte,
) ([]byte, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}
	privKey, svcErr := s.pkiService.GetPrivateKey(keyRef.KeyID)
	if svcErr != nil {
		return nil, fmt.Errorf("key not found for id %s: [%s] %s",
			keyRef.KeyID, svcErr.Code, svcErr.Error.DefaultValue)
	}
	return cryptolab.Generate(content, algorithm, privKey)
}

func (s *runtimeCryptoService) GetPublicKeys(
	_ context.Context, filter kmprovider.PublicKeyFilter,
) ([]kmprovider.PublicKeyInfo, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}

	allCerts, svcErr := s.pkiService.GetAllX509Certificates()
	if svcErr != nil {
		return nil, fmt.Errorf("failed to retrieve certificates: [%s] %s",
			svcErr.Code, svcErr.Error.DefaultValue)
	}

	keys := make([]kmprovider.PublicKeyInfo, 0, len(allCerts))
	for id, cert := range allCerts {
		var alg cryptolab.Algorithm
		switch pub := cert.PublicKey.(type) {
		case *rsa.PublicKey:
			alg = cryptolab.AlgorithmRS256
		case *ecdsa.PublicKey:
			switch pub.Curve.Params().Name {
			case "P-256":
				alg = cryptolab.AlgorithmES256
			case "P-384":
				alg = cryptolab.AlgorithmES384
			case "P-521":
				alg = cryptolab.AlgorithmES512
			default:
				s.logger.Warn("Unsupported EC curve; skipping",
					log.String("keyID", id),
					log.String("curve", pub.Curve.Params().Name))
				continue
			}
		case ed25519.PublicKey:
			alg = cryptolab.AlgorithmEdDSA
		default:
			s.logger.Debug("Unsupported public key type; skipping", log.String("keyID", id))
			continue
		}

		if filter.KeyID != "" && filter.KeyID != id {
			continue
		}
		if filter.Algorithm != "" && filter.Algorithm != alg {
			continue
		}

		keys = append(keys, kmprovider.PublicKeyInfo{
			KeyID:          id,
			Algorithm:      alg,
			PublicKey:      cert.PublicKey,
			Thumbprint:     s.pkiService.GetCertThumbprint(id),
			CertificateDER: cert.Raw,
		})
	}

	return keys, nil
}

func (s *runtimeCryptoService) GetTLSMaterial(
	_ context.Context, _ *kmprovider.KeyRef,
) (*kmprovider.TLSMaterial, error) {
	return nil, errors.New("not implemented")
}

func (s *runtimeCryptoService) getRSAPublicKey(keyRef kmprovider.KeyRef) (*rsa.PublicKey, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}
	cert, svcErr := s.pkiService.GetX509Certificate(keyRef.KeyID)
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

func (s *runtimeCryptoService) getECPublicKey(keyRef kmprovider.KeyRef) (*ecdsa.PublicKey, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}
	cert, svcErr := s.pkiService.GetX509Certificate(keyRef.KeyID)
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

func (s *runtimeCryptoService) getRSAPrivateKey(keyRef kmprovider.KeyRef) (*rsa.PrivateKey, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}
	privKey, svcErr := s.pkiService.GetPrivateKey(keyRef.KeyID)
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

func (s *runtimeCryptoService) getECPrivateKey(keyRef kmprovider.KeyRef) (*ecdsa.PrivateKey, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}
	privKey, svcErr := s.pkiService.GetPrivateKey(keyRef.KeyID)
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
