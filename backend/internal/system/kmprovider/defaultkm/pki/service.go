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

// Package pki loads PEM key/certificate pairs from configuration and provides
// key material lookup by ID for the default key manager.
package pki

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// PKIServiceInterface defines the interface for PKI key/certificate operations.
type PKIServiceInterface interface {
	GetPrivateKey(ctx context.Context, id string) (crypto.PrivateKey, *tidcommon.ServiceError)
	GetCertThumbprint(id string) string
	GetX509Certificate(ctx context.Context, id string) (*x509.Certificate, *tidcommon.ServiceError)
	GetAllX509Certificates(ctx context.Context) (map[string]*x509.Certificate, *tidcommon.ServiceError)
	GetCertificateChain(id string) [][]byte
	GetSupportedSigningAlgorithms() []string
	GetTLSConfig() (*tls.Config, error)
}

// pkiService stores loaded certificates indexed by their ID.
type pkiService struct {
	certificates map[string]PKI
	logger       *log.Logger
}

// newPKIService initializes and returns the PKI service, loading all key/cert pairs from config.
func newPKIService() (PKIServiceInterface, error) {
	serverRuntime := config.GetServerRuntime()
	keyConfigs := serverRuntime.Config.Crypto.Keys
	if len(keyConfigs) == 0 {
		return nil, errors.New("no key configurations found in the system configuration")
	}

	certificates := make(map[string]PKI)
	for _, keyConfig := range keyConfigs {
		if keyConfig.ID == "" {
			return nil, errors.New("key configuration has empty ID")
		}

		certFilePath := path.Join(serverRuntime.ServerHome, keyConfig.CertFile)
		keyFilePath := path.Join(serverRuntime.ServerHome, keyConfig.KeyFile)

		if _, err := os.Stat(certFilePath); os.IsNotExist(err) {
			return nil, errors.New("certificate file not found at " + certFilePath)
		}
		if _, err := os.Stat(keyFilePath); os.IsNotExist(err) {
			return nil, errors.New("key file not found at " + keyFilePath)
		}

		tlsCert, algorithm, err := loadCertKeyPair(certFilePath, keyFilePath)
		if err != nil {
			return nil, err
		}
		thumbprint, err := getThumbprint(tlsCert)
		if err != nil {
			return nil, err
		}
		certificates[keyConfig.ID] = PKI{
			ID:          keyConfig.ID,
			Algorithm:   algorithm,
			PrivateKey:  tlsCert.PrivateKey,
			Certificate: tlsCert,
			ThumbPrint:  thumbprint,
		}
	}

	if len(certificates) == 0 {
		return nil, errors.New("no certificates loaded in PKI service")
	}

	return &pkiService{
		certificates: certificates,
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "PKIService")),
	}, nil
}

// GetPrivateKey retrieves the private key associated with the given ID.
func (s *pkiService) GetPrivateKey(ctx context.Context, id string) (crypto.PrivateKey, *tidcommon.ServiceError) {
	cert, exists := s.certificates[id]
	if !exists || cert.PrivateKey == nil {
		s.logger.Error(ctx, "Private key not found for certificate ID: "+id)
		return nil, &tidcommon.InternalServerError
	}
	return cert.PrivateKey, nil
}

// GetCertificateChain returns the DER-encoded certificate chain for the given ID (leaf first).
func (s *pkiService) GetCertificateChain(id string) [][]byte {
	cert, exists := s.certificates[id]
	if !exists {
		return nil
	}
	return cert.Certificate.Certificate
}

// GetCertThumbprint retrieves the thumbprint of the certificate associated with the given ID.
func (s *pkiService) GetCertThumbprint(id string) string {
	cert, exists := s.certificates[id]
	if !exists {
		return ""
	}
	return cert.ThumbPrint
}

// GetX509Certificate retrieves the x509 certificate associated with the given ID.
func (s *pkiService) GetX509Certificate(
	ctx context.Context, id string) (*x509.Certificate, *tidcommon.ServiceError) {
	cert, exists := s.certificates[id]
	if !exists {
		s.logger.Error(ctx, "Certificate not found for certificate ID: "+id)
		return nil, &tidcommon.InternalServerError
	}
	if len(cert.Certificate.Certificate) == 0 {
		s.logger.Error(ctx, "Certificate data is empty for certificate ID: "+id)
		return nil, &tidcommon.InternalServerError
	}
	parsedCert, err := x509.ParseCertificate(cert.Certificate.Certificate[0])
	if err != nil {
		s.logger.Error(ctx, "Failed to parse x509 certificate for ID: "+id+" Error: "+err.Error())
		return nil, &tidcommon.InternalServerError
	}
	return parsedCert, nil
}

// GetAllX509Certificates retrieves all x509 certificates as a map indexed by their ID.
func (s *pkiService) GetAllX509Certificates(
	ctx context.Context) (map[string]*x509.Certificate, *tidcommon.ServiceError) {
	result := make(map[string]*x509.Certificate)
	for id, cert := range s.certificates {
		if len(cert.Certificate.Certificate) == 0 {
			s.logger.Error(ctx, "Certificate data is empty for certificate ID: "+id)
			return nil, &tidcommon.InternalServerError
		}
		parsedCert, err := x509.ParseCertificate(cert.Certificate.Certificate[0])
		if err != nil {
			s.logger.Error(ctx, "Failed to parse x509 certificate for ID: "+id+" Error: "+err.Error())
			return nil, &tidcommon.InternalServerError
		}
		result[id] = parsedCert
	}
	return result, nil
}

// GetSupportedSigningAlgorithms returns a deduplicated list of JWS algorithm strings
// supported across all configured keys.
func (s *pkiService) GetSupportedSigningAlgorithms() []string {
	var result []string
	for _, cert := range s.certificates {
		for _, alg := range pkiAlgorithmToJWSAlgorithms(cert.Algorithm) {
			if !slices.Contains(result, alg) {
				result = append(result, alg)
			}
		}
	}
	return result
}

// GetTLSConfig loads and returns the TLS configuration from the server's TLS cert and key files.
func (s *pkiService) GetTLSConfig() (*tls.Config, error) {
	serverRuntime := config.GetServerRuntime()
	certFilePath := path.Join(serverRuntime.ServerHome, serverRuntime.Config.TLS.CertFile)
	keyFilePath := path.Join(serverRuntime.ServerHome, serverRuntime.Config.TLS.KeyFile)
	return LoadTLSConfig(&serverRuntime.Config, certFilePath, keyFilePath)
}

// pkiAlgorithmToJWSAlgorithms returns the JWS algorithm strings supported for the given PKI algorithm.
func pkiAlgorithmToJWSAlgorithms(alg PKIAlgorithm) []string {
	switch alg {
	case RSA:
		return []string{string(jws.RS256)}
	case P256:
		return []string{string(jws.ES256)}
	case P384:
		return []string{string(jws.ES384)}
	case P521:
		return []string{string(jws.ES512)}
	case Ed25519:
		return []string{string(jws.EdDSA)}
	case MLDSA44:
		return []string{string(jws.MLDSA44)}
	case MLDSA65:
		return []string{string(jws.MLDSA65)}
	case MLDSA87:
		return []string{string(jws.MLDSA87)}
	default:
		return nil
	}
}

// loadCertKeyPair loads a certificate/key pair from the given file paths. ML-DSA
// keys (RFC 9881 PKCS#8) are loaded via the ML-DSA codec since the standard
// library cannot parse them; all other keys use tls.LoadX509KeyPair.
func loadCertKeyPair(certFilePath, keyFilePath string) (tls.Certificate, PKIAlgorithm, error) {
	keyPEM, err := os.ReadFile(path.Clean(keyFilePath))
	if err != nil {
		return tls.Certificate{}, "", err
	}
	if keyBlock, _ := pem.Decode(keyPEM); keyBlock != nil {
		if _, isMLDSA := cryptolib.MLDSAAlgFromPKCS8(keyBlock.Bytes); isMLDSA {
			return loadMLDSACertKeyPair(certFilePath, keyBlock.Bytes)
		}
	}

	tlsCert, err := tls.LoadX509KeyPair(certFilePath, keyFilePath)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	algorithm, err := getAlgorithmFromKey(tlsCert.PrivateKey)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	return tlsCert, algorithm, nil
}

// loadMLDSACertKeyPair builds a tls.Certificate for an ML-DSA key pair. The
// private key is reconstructed with the ML-DSA codec and the certificate DER is
// retained as-is (the standard library cannot parse the ML-DSA public key, so it
// is derived from the private key when needed).
func loadMLDSACertKeyPair(certFilePath string, keyDER []byte) (tls.Certificate, PKIAlgorithm, error) {
	privKey, alg, err := cryptolib.ParseMLDSAPKCS8(keyDER)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	certPEM, err := os.ReadFile(path.Clean(certFilePath))
	if err != nil {
		return tls.Certificate{}, "", err
	}
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return tls.Certificate{}, "", errors.New("failed to decode ML-DSA certificate PEM")
	}
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certBlock.Bytes},
		PrivateKey:  privKey,
	}
	return tlsCert, mldsaPKIAlgorithm(alg), nil
}

// mldsaPKIAlgorithm maps a cryptolib ML-DSA algorithm to its PKIAlgorithm.
func mldsaPKIAlgorithm(alg cryptolib.Algorithm) PKIAlgorithm {
	switch alg {
	case cryptolib.AlgorithmMLDSA44:
		return MLDSA44
	case cryptolib.AlgorithmMLDSA65:
		return MLDSA65
	case cryptolib.AlgorithmMLDSA87:
		return MLDSA87
	default:
		return ""
	}
}

// getAlgorithmFromKey determines the PKIAlgorithm based on the type of the private key.
func getAlgorithmFromKey(key crypto.PrivateKey) (PKIAlgorithm, error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return RSA, nil
	case *ecdsa.PrivateKey:
		crvName := k.Curve.Params().Name
		switch crvName {
		case "P-256":
			return P256, nil
		case "P-384":
			return P384, nil
		case "P-521":
			return P521, nil
		default:
			return "", errors.New("unsupported ECDSA curve: " + crvName)
		}
	case ed25519.PrivateKey:
		return Ed25519, nil
	default:
		return "", errors.New("unsupported key type")
	}
}

// getThumbprint computes the SHA-256 thumbprint of the given TLS certificate.
func getThumbprint(cert tls.Certificate) (string, error) {
	certData := cert.Certificate[0]
	parsedCert, err := x509.ParseCertificate(certData)
	if err != nil {
		return "", err
	}
	return cryptolib.GenerateThumbprint(parsedCert.Raw), nil
}
