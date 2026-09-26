// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package jwe

import (
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// EncryptOption adds to the JWE protected header. Options never replace a header the service
// computes itself (typ, alg, enc, cty, kid, epk, iv, tag) and cannot declare zip, since Encrypt does
// not compress.
type EncryptOption func(header map[string]interface{})

// WithProtectedHeader adds one member to the JWE protected header, such as the "iss" a logout token
// replicates so a relying party can pick its decryption key before decrypting.
func WithProtectedHeader(name string, value interface{}) EncryptOption {
	return func(header map[string]interface{}) { header[name] = value }
}

// reservedProtectedHeaders cannot be supplied through an EncryptOption: Encrypt sets the first eight
// itself, and zip would announce a compression it does not apply.
var reservedProtectedHeaders = map[string]bool{
	"typ": true, "alg": true, "enc": true, "cty": true, "kid": true, "epk": true, "iv": true, "tag": true,
	"zip": true,
}

// JWEServiceInterface defines the interface for JWE operations.
type JWEServiceInterface interface {
	Encrypt(ctx context.Context, payload []byte, recipientPublicKey *providers.KeyRef,
		alg string, enc ContentEncAlgorithm, cty string, kid string,
		opts ...EncryptOption) (string, *tidcommon.ServiceError)
	Decrypt(ctx context.Context, jweToken string) ([]byte, *tidcommon.ServiceError)

	// SupportedKeyEncryptionAlgorithms returns the JWE "alg" (key management) algorithm values
	// usable by Encrypt and Decrypt: the intersection of the algorithms this package recognizes as
	// key-management roles with what the configured crypto provider actually supports.
	SupportedKeyEncryptionAlgorithms() []string

	// SupportedContentEncryptionAlgorithms returns the JWE "enc" (content encryption) algorithm
	// values supported by Encrypt and Decrypt.
	SupportedContentEncryptionAlgorithms() []string
}

// jweService implements the JWEServiceInterface.
type jweService struct {
	cryptoProvider providers.RuntimeCryptoProvider
	keyRef         providers.KeyRef
	logger         *log.Logger
}

// newJWEService creates a new JWE service instance.
func newJWEService(
	cryptoProvider providers.RuntimeCryptoProvider, cfg joseconfig.Config,
) (JWEServiceInterface, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWEService"))

	return &jweService{
		cryptoProvider: cryptoProvider,
		keyRef:         providers.KeyRef{KeyID: cfg.PreferredKeyID},
		logger:         logger,
	}, nil
}

// Encrypt encrypts the payload for the recipient's public key via the runtime crypto provider.
// cty is the content type placed in the JWE protected header (e.g. "json" or "JWT").
// kid identifies the recipient's key; it is stamped in the header only when non-empty.
// opts add further protected headers; see EncryptOption.
func (js *jweService) Encrypt(ctx context.Context, payload []byte, recipientPublicKey *providers.KeyRef,
	keyEncAlgorithm string, enc ContentEncAlgorithm, cty string, kid string,
	opts ...EncryptOption) (string, *tidcommon.ServiceError) {
	if !isSupportedEnc(enc) {
		return "", &ErrorUnsupportedEncryptionAlgorithm
	}

	params := make(map[string]interface{})
	params[providers.ParamContentEncryptionAlgorithm] = string(enc)
	params[providers.ParamKeyEncryptionAlgorithm] = keyEncAlgorithm

	if js.cryptoProvider == nil {
		js.logger.Error(ctx, "Failed to encrypt cryptoProvider is nil")
		return "", &ErrorUnsupportedJWEAlgorithm
	}

	// Establish the CEK via the runtime crypto provider, using the recipient's public key.
	encryptedKey, details, err := js.cryptoProvider.Encrypt(
		ctx, recipientPublicKey, keyEncAlgorithm, params, nil)
	if err != nil {
		js.logger.Error(ctx, "Failed to encrypt CEK: "+err.Error())
		return "", &ErrorUnsupportedJWEAlgorithm
	}

	cek := details.CEK

	// Build the JWE protected header. Option-supplied members go first so the computed ones win.
	header := make(map[string]interface{})
	for _, opt := range opts {
		opt(header)
	}
	for name := range reservedProtectedHeaders {
		delete(header, name)
	}
	header["typ"] = "JWE"
	header["alg"] = keyEncAlgorithm
	header["enc"] = string(enc)
	if kid != "" {
		header["kid"] = kid
	}
	if cty != "" {
		header["cty"] = cty
	}

	// Add ECDH-ES ephemeral public key to header.
	if details.EPK != nil {
		epkMap, epkErr := epkToMap(details.EPK)
		if epkErr != nil {
			js.logger.Error(ctx, "Failed to encode EPK: "+epkErr.Error())
			return "", &tidcommon.InternalServerError
		}
		header["epk"] = epkMap
	}

	// Add AES-GCM KW IV and tag to header.
	if details.IV != nil {
		header["iv"] = base64.RawURLEncoding.EncodeToString(details.IV)
		header["tag"] = base64.RawURLEncoding.EncodeToString(details.Tag)
	}

	headerJSON, jsonErr := json.Marshal(header)
	if jsonErr != nil {
		js.logger.Error(ctx, "Failed to marshal JWE header: "+jsonErr.Error())
		return "", &tidcommon.InternalServerError
	}
	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Encrypt the content payload.
	iv, ciphertext, tag, err := encryptContent(payload, cek, enc, []byte(headerBase64))
	if err != nil {
		js.logger.Error(ctx, "Failed to encrypt content: "+err.Error())
		return "", &tidcommon.InternalServerError
	}

	// Assemble compact serialization.
	jweToken := fmt.Sprintf("%s.%s.%s.%s.%s",
		headerBase64,
		base64.RawURLEncoding.EncodeToString(encryptedKey),
		base64.RawURLEncoding.EncodeToString(iv),
		base64.RawURLEncoding.EncodeToString(ciphertext),
		base64.RawURLEncoding.EncodeToString(tag),
	)

	return jweToken, nil
}

// Decrypt decrypts the JWE compact serialization using the server's private key via the crypto provider.
func (js *jweService) Decrypt(ctx context.Context, jweToken string) ([]byte, *tidcommon.ServiceError) {
	header, headerBase64, encryptedKey, iv, ciphertext, tag, err := DecodeJWE(jweToken)
	if err != nil {
		js.logger.Debug(ctx, "Failed to decode JWE: "+err.Error())
		return nil, &ErrorDecodingJWE
	}

	algStr, ok := header["alg"].(string)
	if !ok {
		return nil, &ErrorUnsupportedJWEAlgorithm
	}
	encStr, ok := header["enc"].(string)
	if !ok {
		return nil, &ErrorUnsupportedEncryptionAlgorithm
	}

	if js.cryptoProvider == nil {
		js.logger.Error(ctx, "Failed to decrypt cryptoProvider is nil")
		return nil, &ErrorUnsupportedJWEAlgorithm
	}

	params := map[string]interface{}{}
	params[providers.ParamContentEncryptionAlgorithm] = encStr
	params[providers.ParamKeyEncryptionAlgorithm] = algStr

	if _, hasEPK := header["epk"]; hasEPK {
		epkMap, ok := header["epk"].(map[string]interface{})
		if !ok {
			js.logger.Error(ctx, "Failed to extract epk from header")
			return nil, &ErrorUnsupportedJWEAlgorithm
		}
		params[providers.ParamEPK] = epkMap
	}
	apu := decodeAPUAPV(header, "apu")
	if apu != nil {
		params[providers.ParamAPU] = apu
	}
	apv := decodeAPUAPV(header, "apv")
	if apv != nil {
		params[providers.ParamAPV] = apv
	}

	// Decrypt CEK via the runtime crypto provider (uses server's private key).
	cek, err := js.cryptoProvider.Decrypt(ctx, &js.keyRef, algStr, params, encryptedKey)
	if err != nil {
		js.logger.Error(ctx, "Failed to decrypt CEK", log.Error(err))
		return nil, &ErrorJWEDecryptionFailed
	}

	enc := ContentEncAlgorithm(encStr)
	// Decrypt content.
	payload, err := decryptContent(ciphertext, iv, tag, cek, enc, []byte(headerBase64))
	if err != nil {
		js.logger.Error(ctx, "Failed to decrypt content", log.Error(err))
		return nil, &ErrorJWEDecryptionFailed
	}

	return payload, nil
}

// SupportedKeyEncryptionAlgorithms returns the JWE "alg" (key management) algorithm values usable
// by Encrypt and Decrypt: the intersection of the algorithms this package recognizes as
// key-management roles with what the configured crypto provider actually supports.
func (js *jweService) SupportedKeyEncryptionAlgorithms() []string {
	providerSupported := js.cryptoProvider.GetSupportedEncryptionAlgorithms()
	result := make([]string, 0, len(knownKeyEncAlgorithms))
	for _, alg := range knownKeyEncAlgorithms {
		if slices.Contains(providerSupported, string(alg)) {
			result = append(result, string(alg))
		}
	}
	return result
}

// SupportedContentEncryptionAlgorithms returns the JWE "enc" (content encryption) algorithm
// values supported by Encrypt and Decrypt.
func (js *jweService) SupportedContentEncryptionAlgorithms() []string {
	result := make([]string, len(supportedContentEncAlgorithms))
	for i, enc := range supportedContentEncAlgorithms {
		result[i] = string(enc)
	}
	return result
}

// DecryptWithKey decrypts a JWE compact serialization using an explicitly
// supplied recipient private key instead of the server's configured key. It
// supports ephemeral per-request keys, as required by OpenID4VP encrypted
// responses (response_mode=direct_post.jwt). Only the key management algorithms
// accepted by buildDecryptParams are supported.
func DecryptWithKey(jweToken string, privateKey crypto.PrivateKey) ([]byte, error) {
	header, headerBase64, encryptedKey, iv, ciphertext, tag, err := DecodeJWE(jweToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWE: %w", err)
	}

	algStr, ok := header["alg"].(string)
	if !ok {
		return nil, errors.New("JWE header missing alg")
	}
	encStr, ok := header["enc"].(string)
	if !ok {
		return nil, errors.New("JWE header missing enc")
	}
	alg := KeyEncAlgorithm(algStr)
	enc := ContentEncAlgorithm(encStr)

	params, err := buildDecryptParams(alg, enc, header)
	if err != nil {
		return nil, err
	}

	cek, err := cryptolib.Decrypt(privateKey, params, encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt CEK: %w", err)
	}

	payload, err := decryptContent(ciphertext, iv, tag, cek, enc, []byte(headerBase64))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt content: %w", err)
	}
	return payload, nil
}

// supportedContentEncAlgorithms lists the JWE "enc" (content encryption) algorithms supported by
// Encrypt and Decrypt.
var supportedContentEncAlgorithms = []ContentEncAlgorithm{
	A128CBCHS256, A192CBCHS384, A256CBCHS512, A128GCM, A192GCM, A256GCM,
}

// isSupportedEnc returns true when enc is a supported content encryption algorithm.
func isSupportedEnc(enc ContentEncAlgorithm) bool {
	for _, supported := range supportedContentEncAlgorithms {
		if supported == enc {
			return true
		}
	}
	return false
}

// buildDecryptParams builds cryptolib.AlgorithmParams for server-side CEK decryption.
// For ECDH-ES variants it reads the ephemeral public key from the JWE protected header.
func buildDecryptParams(alg KeyEncAlgorithm, enc ContentEncAlgorithm,
	header map[string]interface{}) (cryptolib.AlgorithmParams, error) {
	switch alg {
	case RSAOAEP:
		return cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmRSAOAEP}, nil
	case RSAOAEP256:
		return cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmRSAOAEP256}, nil
	case ECDHES:
		epk, err := extractECDHPublicKeyFromHeader(header)
		if err != nil {
			return cryptolib.AlgorithmParams{}, err
		}
		apu := decodeAPUAPV(header, "apu")
		apv := decodeAPUAPV(header, "apv")
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmECDHES,
			ECDHES: cryptolib.ECDHESParams{
				EPK:                        epk,
				ContentEncryptionAlgorithm: cryptolib.Algorithm(enc),
				APU:                        apu,
				APV:                        apv,
			},
		}, nil
	case ECDHESA128KW:
		epk, err := extractECDHPublicKeyFromHeader(header)
		if err != nil {
			return cryptolib.AlgorithmParams{}, err
		}
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmECDHESA128KW,
			ECDHES:    cryptolib.ECDHESParams{EPK: epk},
		}, nil
	case ECDHESA192KW:
		epk, err := extractECDHPublicKeyFromHeader(header)
		if err != nil {
			return cryptolib.AlgorithmParams{}, err
		}
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmECDHESA192KW,
			ECDHES:    cryptolib.ECDHESParams{EPK: epk},
		}, nil
	case ECDHESA256KW:
		epk, err := extractECDHPublicKeyFromHeader(header)
		if err != nil {
			return cryptolib.AlgorithmParams{}, err
		}
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmECDHESA256KW,
			ECDHES:    cryptolib.ECDHESParams{EPK: epk},
		}, nil
	default:
		return cryptolib.AlgorithmParams{}, fmt.Errorf("unsupported JWE algorithm for server-side decryption: %s", alg)
	}
}
