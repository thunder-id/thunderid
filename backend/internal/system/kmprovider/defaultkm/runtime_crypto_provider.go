// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package defaultkm

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pki"
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
	switch cryptolib.Algorithm(algorithm) {
	case cryptolib.AlgorithmAESGCM:
		if s.cfgService == nil {
			return nil, nil, errors.New("config crypto service not initialized")
		}
		encrypted, err := s.cfgService.Encrypt(ctx, content)
		return encrypted, nil, err
	case cryptolib.AlgorithmRSAOAEP, cryptolib.AlgorithmRSAOAEP256:
		if keyRef == nil {
			return nil, nil, fmt.Errorf("keyRef required for %s", algorithm)
		}
		rsaPub, err := s.getRSAPublicKey(ctx, *keyRef)
		if err != nil {
			return nil, nil, err
		}
		algorithmParams, err := algorithmParamsFromMap(algorithm, params)
		if err != nil {
			return nil, nil, err
		}
		return cryptolib.Encrypt(rsaPub, &algorithmParams, content)
	case cryptolib.AlgorithmECDHES,
		cryptolib.AlgorithmECDHESA128KW, cryptolib.AlgorithmECDHESA192KW, cryptolib.AlgorithmECDHESA256KW:
		if keyRef == nil {
			return nil, nil, fmt.Errorf("keyRef required for %s", algorithm)
		}
		ecPub, err := s.getECPublicKey(ctx, *keyRef)
		if err != nil {
			return nil, nil, err
		}
		algorithmParams, err := algorithmParamsFromMap(algorithm, params)
		if err != nil {
			return nil, nil, err
		}
		return cryptolib.Encrypt(ecPub, &algorithmParams, content)
	default:
		return nil, nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}
}

func (s *runtimeCryptoService) Decrypt(
	ctx context.Context, keyRef *providers.KeyRef, algorithm string, params map[string]interface{}, content []byte,
) ([]byte, error) {
	switch cryptolib.Algorithm(algorithm) {
	case cryptolib.AlgorithmAESGCM:
		if s.cfgService == nil {
			return nil, errors.New("config crypto service not initialized")
		}
		return s.cfgService.Decrypt(ctx, content)
	case cryptolib.AlgorithmRSAOAEP, cryptolib.AlgorithmRSAOAEP256:
		if keyRef == nil {
			return nil, fmt.Errorf("keyRef required for %s", algorithm)
		}
		rsaPriv, err := s.getRSAPrivateKey(ctx, *keyRef)
		if err != nil {
			return nil, err
		}
		algorithmParams, err := algorithmParamsFromMap(algorithm, params)
		if err != nil {
			return nil, err
		}
		return cryptolib.Decrypt(rsaPriv, algorithmParams, content)
	case cryptolib.AlgorithmECDHES,
		cryptolib.AlgorithmECDHESA128KW, cryptolib.AlgorithmECDHESA192KW, cryptolib.AlgorithmECDHESA256KW:
		if keyRef == nil {
			return nil, fmt.Errorf("keyRef required for %s", algorithm)
		}
		ecPriv, err := s.getECPrivateKey(ctx, *keyRef)
		if err != nil {
			return nil, err
		}
		algorithmParams, err := algorithmParamsFromMap(algorithm, params)
		if err != nil {
			return nil, err
		}
		return cryptolib.Decrypt(ecPriv, algorithmParams, content)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}
}

// algorithmParamsFromMap reconstructs a cryptolib.AlgorithmParams from the generic params map
// received via providers.RuntimeCryptoProvider.Encrypt or .Decrypt. See
// providers.ParamContentEncryptionAlgorithm, providers.ParamEPK, providers.ParamAPU and
// providers.ParamAPV for the recognized keys.
func algorithmParamsFromMap(algorithm string, params map[string]interface{}) (cryptolib.AlgorithmParams, error) {
	alg := cryptolib.Algorithm(algorithm)
	algorithmParams := cryptolib.AlgorithmParams{Algorithm: alg}

	encAlg, _ := params[providers.ParamContentEncryptionAlgorithm].(string)

	switch alg {
	case cryptolib.AlgorithmRSAOAEP:
		algorithmParams.RSAOAEP = cryptolib.RSAOAEPParams{ContentEncryptionAlgorithm: cryptolib.Algorithm(encAlg)}
	case cryptolib.AlgorithmRSAOAEP256:
		algorithmParams.RSAOAEP256 = cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: cryptolib.Algorithm(encAlg)}
	case cryptolib.AlgorithmECDHES,
		cryptolib.AlgorithmECDHESA128KW, cryptolib.AlgorithmECDHESA192KW, cryptolib.AlgorithmECDHESA256KW:
		apu, _ := params[providers.ParamAPU].([]byte)
		apv, _ := params[providers.ParamAPV].([]byte)

		epk, err := epkFromParam(params[providers.ParamEPK])
		if err != nil {
			return cryptolib.AlgorithmParams{}, fmt.Errorf("invalid epk param: %w", err)
		}

		algorithmParams.ECDHES = cryptolib.ECDHESParams{
			EPK:                        epk,
			ContentEncryptionAlgorithm: cryptolib.Algorithm(encAlg),
			APU:                        apu,
			APV:                        apv,
		}
	}

	return algorithmParams, nil
}

// epkFromParam resolves the ECDH-ES ephemeral public key received via the generic params map.
// It arrives as a JWK map (decoded from the JWE "epk" header) for real JWE flows, or already as a
// crypto.PublicKey when callers construct it directly. Returns the *ecdh.PublicKey cryptolib expects,
// nil if epk is absent, or an error if epk is present but cannot be resolved to an EC public key.
func epkFromParam(epk interface{}) (crypto.PublicKey, error) {
	if epk == nil {
		return nil, nil
	}
	epkMap, ok := epk.(map[string]interface{})
	if !ok {
		return epk, nil
	}

	pub, err := JWKToPublicKey(epkMap)
	if err != nil {
		return nil, err
	}
	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("epk is not an EC public key: %T", pub)
	}
	ecdhPub, err := ecdsaPub.ECDH()
	if err != nil {
		return nil, err
	}
	return ecdhPub, nil
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

	if keyRef.KeyID != "" {
		// keyRef.KeyID carries the JWT "kid" (a thumbprint), not the internal cert/key ID that
		// PublicKeyFilter.KeyID matches against, so fetch all keys and match by thumbprint below.
		keys, err := s.GetPublicKeys(ctx, providers.PublicKeyFilter{})
		if err != nil {
			return fmt.Errorf("failed to retrieve public keys: %w", err)
		}
		for _, key := range keys {
			if key.Thumbprint == keyRef.KeyID {
				return cryptolib.Verify(content, signature, signAlg, key.PublicKey)
			}
		}
	}

	if keyRef.PublicKey != nil {
		return cryptolib.Verify(content, signature, signAlg, keyRef.PublicKey)
	}

	if keyRef.PublicKeyJWK != nil {
		pubKey, err := JWKToPublicKey(keyRef.PublicKeyJWK)
		if err != nil {
			return fmt.Errorf("invalid JWK public key: %w", err)
		}
		return cryptolib.Verify(content, signature, signAlg, pubKey)
	}

	return fmt.Errorf("%w: kid=%s", providers.ErrKeyNotFound, keyRef.KeyID)
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
			case P256:
				alg = cryptolib.AlgorithmES256
			case P384:
				alg = cryptolib.AlgorithmES384
			case P521:
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
		if filter.Algorithm != "" && filter.Algorithm != string(alg) {
			continue
		}

		thumbprint := s.pkiService.GetCertThumbprint(id)

		keys = append(keys, providers.PublicKeyInfo{
			KeyID:               thumbprint,
			Algorithm:           string(alg),
			PublicKey:           pub,
			Thumbprint:          thumbprint,
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

func (s *runtimeCryptoService) GetTLSMaterial(
	ctx context.Context,
) (*common.TLSMaterial, error) {
	if s.pkiService == nil {
		return nil, errors.New("PKI service not initialized")
	}
	tlsCfg, err := s.pkiService.GetTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS config: %w", err)
	}
	return &common.TLSMaterial{
		Certificate: tlsCfg.Certificates[0],
		MinVersion:  tlsCfg.MinVersion,
	}, nil
}

// GetSupportedSigningAlgorithms returns the list of signing algorithms supported by Sign and Verify.
func (s *runtimeCryptoService) GetSupportedSigningAlgorithms() []string {
	return []string{
		string(RS256), string(RS512), string(PS256),
		string(ES256), string(ES384), string(ES512),
		string(EdDSA), string(MLDSA44), string(MLDSA65), string(MLDSA87),
	}
}

// GetSupportedEncryptionAlgorithms returns the list of algorithms supported by Encrypt and Decrypt.
func (s *runtimeCryptoService) GetSupportedEncryptionAlgorithms() []string {
	return []string{
		string(cryptolib.AlgorithmAESGCM),
		string(cryptolib.AlgorithmRSAOAEP), string(cryptolib.AlgorithmRSAOAEP256),
		string(cryptolib.AlgorithmECDHES),
		string(cryptolib.AlgorithmECDHESA128KW),
		string(cryptolib.AlgorithmECDHESA192KW),
		string(cryptolib.AlgorithmECDHESA256KW),
	}
}

func (s *runtimeCryptoService) getRSAPublicKey(ctx context.Context, keyRef providers.KeyRef) (*rsa.PublicKey, error) {
	return getTypedPublicKey[*rsa.PublicKey](ctx, s, keyRef, "RSA")
}

func (s *runtimeCryptoService) getECPublicKey(ctx context.Context, keyRef providers.KeyRef) (*ecdsa.PublicKey, error) {
	return getTypedPublicKey[*ecdsa.PublicKey](ctx, s, keyRef, "EC")
}

// getTypedPublicKey resolves keyRef to a public key of type T, either directly from
// keyRef.PublicKey, from keyRef.PublicKeyJWK, or by loading the X.509 certificate
// identified by keyRef.KeyID. keyTypeName is used only for error messages.
func getTypedPublicKey[T crypto.PublicKey](
	ctx context.Context, s *runtimeCryptoService, keyRef providers.KeyRef, keyTypeName string,
) (T, error) {
	var zero T
	if keyRef.KeyID == "" {
		if keyRef.PublicKey != nil {
			pub, ok := keyRef.PublicKey.(T)
			if !ok {
				return zero, fmt.Errorf("key is not an %s public key", keyTypeName)
			}
			return pub, nil
		}
		if keyRef.PublicKeyJWK != nil {
			pubKey, err := JWKToPublicKey(keyRef.PublicKeyJWK)
			if err != nil {
				return zero, fmt.Errorf("invalid JWK public key: %w", err)
			}
			pub, ok := pubKey.(T)
			if !ok {
				return zero, fmt.Errorf("key is not an %s public key", keyTypeName)
			}
			return pub, nil
		}
		return zero, errors.New("keyRef has neither a key ID nor a public key")
	}
	if s.pkiService == nil {
		return zero, errors.New("PKI service not initialized")
	}
	cert, svcErr := s.pkiService.GetX509Certificate(ctx, keyRef.KeyID)
	if svcErr != nil {
		return zero, fmt.Errorf("key not found for id %s: [%s] %s",
			keyRef.KeyID, svcErr.Code, svcErr.Error.DefaultValue)
	}
	pub, ok := cert.PublicKey.(T)
	if !ok {
		return zero, fmt.Errorf("key is not an %s public key", keyTypeName)
	}
	return pub, nil
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

// JWKToPublicKey converts a JWK map to a crypto.PublicKey supporting RSA, EC, and Ed25519.
func JWKToPublicKey(jwk map[string]interface{}) (crypto.PublicKey, error) {
	kty, ok := jwk["kty"].(string)
	if !ok {
		return nil, errors.New("JWK missing kty")
	}

	switch kty {
	case "RSA":
		return jwkToRSAPublicKey(jwk)
	case "EC":
		return jwkToECDSAPublicKey(jwk)
	case "OKP":
		return jwkToOKPPublicKey(jwk)
	case "AKP":
		return jwkToAKPPublicKey(jwk)
	default:
		return nil, fmt.Errorf("unsupported JWK kty: %s", kty)
	}
}

// jwkToAKPPublicKey converts an AKP (Algorithm Key Pair) JWK to an ML-DSA public
// key, per RFC 9964. The alg member identifies the ML-DSA parameter set and pub
// carries the base64url-encoded raw public key.
func jwkToAKPPublicKey(jwk map[string]interface{}) (crypto.PublicKey, error) {
	alg, algOK := jwk["alg"].(string)
	pubStr, pubOK := jwk["pub"].(string)
	if !algOK || !pubOK {
		return nil, errors.New("JWK missing AKP alg or pub")
	}
	pubBytes, err := base64.RawURLEncoding.DecodeString(pubStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode AKP pub: %w", err)
	}
	pub, err := cryptolib.MLDSAPublicKeyFromBytes(cryptolib.Algorithm(alg), pubBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AKP public key: %w", err)
	}
	return pub, nil
}

// Bounds for RSA public keys parsed from untrusted JWK input.
const (
	minRSAModulusBytes  = 256  // 2048 bits
	maxRSAModulusBytes  = 1024 // 8192 bits
	maxRSAExponentBytes = 4
)

// jwkToRSAPublicKey converts a JWK to an RSA public key.
func jwkToRSAPublicKey(jwk map[string]interface{}) (*rsa.PublicKey, error) {
	nStr, nOK := jwk["n"].(string)
	eStr, eOK := jwk["e"].(string)
	if !nOK || !eOK {
		return nil, errors.New("JWK missing RSA modulus or exponent")
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode RSA modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode RSA exponent: %w", err)
	}

	if len(nBytes) == 0 || nBytes[0] == 0x00 {
		return nil, errors.New("RSA modulus is not minimally encoded")
	}
	if len(nBytes) < minRSAModulusBytes || len(nBytes) > maxRSAModulusBytes {
		return nil, fmt.Errorf("unsupported RSA modulus size: %d bytes", len(nBytes))
	}
	if len(eBytes) == 0 || len(eBytes) > maxRSAExponentBytes || eBytes[0] == 0x00 {
		return nil, errors.New("invalid RSA exponent encoding")
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes).Int64()
	if e < 3 || e%2 == 0 {
		return nil, errors.New("invalid RSA exponent")
	}

	return &rsa.PublicKey{N: n, E: int(e)}, nil
}

// jwkToECDSAPublicKey converts a JWK to an ECDSA public key for JWS signature verification.
func jwkToECDSAPublicKey(jwk map[string]interface{}) (*ecdsa.PublicKey, error) {
	crv, _ := jwk["crv"].(string)
	xStr, _ := jwk["x"].(string)
	yStr, _ := jwk["y"].(string)
	if crv == "" || xStr == "" || yStr == "" {
		return nil, fmt.Errorf("EC JWK missing crv/x/y")
	}

	var curve elliptic.Curve
	var coordLen int
	switch crv {
	case P256:
		curve, coordLen = elliptic.P256(), 32
	case P384:
		curve, coordLen = elliptic.P384(), 48
	case P521:
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
	if len(xBytes) != coordLen || len(yBytes) != coordLen {
		return nil, fmt.Errorf("EC coordinate length mismatch for %s", crv)
	}

	uncompressed := make([]byte, 1+2*coordLen)
	uncompressed[0] = 0x04
	copy(uncompressed[1:1+coordLen], xBytes)
	copy(uncompressed[1+coordLen:], yBytes)

	pubKey, err := ecdsa.ParseUncompressedPublicKey(curve, uncompressed)
	if err != nil {
		return nil, fmt.Errorf("invalid EC public key: %w", err)
	}
	return pubKey, nil
}

// jwkToOKPPublicKey converts a JWK to an OKP public key.
func jwkToOKPPublicKey(jwk map[string]interface{}) (ed25519.PublicKey, error) {
	crv, crvOK := jwk["crv"].(string)
	xStr, xOK := jwk["x"].(string)
	if !crvOK || !xOK {
		return nil, errors.New("JWK missing OKP parameters")
	}

	switch crv {
	case "Ed25519":
		xBytes, err := base64.RawURLEncoding.DecodeString(xStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode Ed25519 x: %w", err)
		}
		if l := len(xBytes); l != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid Ed25519 public key length: %d", l)
		}
		return ed25519.PublicKey(xBytes), nil
	default:
		return nil, fmt.Errorf("unsupported OKP curve: %s", crv)
	}
}
