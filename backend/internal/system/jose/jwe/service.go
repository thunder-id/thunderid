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

package jwe

import (
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// JWEServiceInterface defines the interface for JWE operations.
type JWEServiceInterface interface {
	Encrypt(ctx context.Context, payload []byte, recipientPublicKey crypto.PublicKey,
		alg KeyEncAlgorithm, enc ContentEncAlgorithm, cty string, kid string) (string, *tidcommon.ServiceError)
	Decrypt(ctx context.Context, jweToken string) ([]byte, *tidcommon.ServiceError)
}

// jweService implements the JWEServiceInterface.
type jweService struct {
	cryptoProvider kmprovider.RuntimeCryptoProvider
	keyRef         kmprovider.KeyRef
	logger         *log.Logger
}

// newJWEService creates a new JWE service instance.
func newJWEService(
	cryptoProvider kmprovider.RuntimeCryptoProvider, cfg joseconfig.Config,
) (JWEServiceInterface, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWEService"))

	return &jweService{
		cryptoProvider: cryptoProvider,
		keyRef:         kmprovider.KeyRef{KeyID: cfg.PreferredKeyID},
		logger:         logger,
	}, nil
}

// Encrypt encrypts the payload using the recipient's public key.
// cty is the content type placed in the JWE protected header (e.g. "json" or "JWT").
// kid identifies the recipient's key; it is stamped in the header only when non-empty.
func (js *jweService) Encrypt(ctx context.Context, payload []byte, recipientPublicKey crypto.PublicKey,
	alg KeyEncAlgorithm, enc ContentEncAlgorithm, cty string, kid string) (string, *tidcommon.ServiceError) {
	if !isSupportedEnc(enc) {
		return "", &ErrorUnsupportedEncryptionAlgorithm
	}

	params, paramsErr := buildEncryptParams(alg, enc)
	if paramsErr != nil {
		return "", &ErrorUnsupportedJWEAlgorithm
	}

	// Establish the CEK via cryptolib key establishment.
	encryptedKey, details, err := cryptolib.Encrypt(recipientPublicKey, &params, nil)
	if err != nil {
		js.logger.Error(ctx, "Failed to encrypt CEK: "+err.Error())
		return "", &ErrorUnsupportedJWEAlgorithm
	}

	cek := details.CEK

	// Build the JWE protected header.
	header := map[string]interface{}{
		"typ": "JWE",
		"alg": string(alg),
		"enc": string(enc),
	}
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

	alg := KeyEncAlgorithm(algStr)
	enc := ContentEncAlgorithm(encStr)

	// Build cryptolib params for CEK decryption using the server's key.
	params, paramsErr := buildDecryptParams(alg, enc, header)
	if paramsErr != nil {
		js.logger.Debug(ctx, "Failed to build decrypt params: "+paramsErr.Error())
		return nil, &ErrorUnsupportedJWEAlgorithm
	}

	// Decrypt CEK via the runtime crypto provider (uses server's private key).
	cek, err := js.cryptoProvider.Decrypt(ctx, &js.keyRef, params, encryptedKey)
	if err != nil {
		js.logger.Error(ctx, "Failed to decrypt CEK: "+err.Error())
		return nil, &ErrorJWEDecryptionFailed
	}

	// Decrypt content.
	payload, err := decryptContent(ciphertext, iv, tag, cek, enc, []byte(headerBase64))
	if err != nil {
		js.logger.Error(ctx, "Failed to decrypt content: "+err.Error())
		return nil, &ErrorJWEDecryptionFailed
	}

	return payload, nil
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

// isSupportedEnc returns true when enc is a supported content encryption algorithm.
func isSupportedEnc(enc ContentEncAlgorithm) bool {
	switch enc {
	case A128CBCHS256, A192CBCHS384, A256CBCHS512, A128GCM, A192GCM, A256GCM:
		return true
	default:
		return false
	}
}

// buildEncryptParams converts a JWE key management algorithm and content encryption algorithm
// into a cryptolib.AlgorithmParams for key establishment during Encrypt.
func buildEncryptParams(alg KeyEncAlgorithm, enc ContentEncAlgorithm) (cryptolib.AlgorithmParams, error) {
	encAlg := cryptolib.Algorithm(enc)
	switch alg {
	case RSAOAEP:
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmRSAOAEP,
			RSAOAEP:   cryptolib.RSAOAEPParams{ContentEncryptionAlgorithm: encAlg},
		}, nil
	case RSAOAEP256:
		return cryptolib.AlgorithmParams{
			Algorithm:  cryptolib.AlgorithmRSAOAEP256,
			RSAOAEP256: cryptolib.RSAOAEP256Params{ContentEncryptionAlgorithm: encAlg},
		}, nil
	case ECDHES:
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmECDHES,
			ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: encAlg},
		}, nil
	case ECDHESA128KW:
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmECDHESA128KW,
			ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: encAlg},
		}, nil
	case ECDHESA192KW:
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmECDHESA192KW,
			ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: encAlg},
		}, nil
	case ECDHESA256KW:
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmECDHESA256KW,
			ECDHES:    cryptolib.ECDHESParams{ContentEncryptionAlgorithm: encAlg},
		}, nil
	case A128KW:
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmA128KW,
			AESKW:     cryptolib.AESKWParams{ContentEncryptionAlgorithm: encAlg},
		}, nil
	case A192KW:
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmA192KW,
			AESKW:     cryptolib.AESKWParams{ContentEncryptionAlgorithm: encAlg},
		}, nil
	case A256KW:
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmA256KW,
			AESKW:     cryptolib.AESKWParams{ContentEncryptionAlgorithm: encAlg},
		}, nil
	case A128GCMKW:
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmA128GCMKW,
			AESGCMKW:  cryptolib.AESGCMKWParams{ContentEncryptionAlgorithm: encAlg},
		}, nil
	case A192GCMKW:
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmA192GCMKW,
			AESGCMKW:  cryptolib.AESGCMKWParams{ContentEncryptionAlgorithm: encAlg},
		}, nil
	case A256GCMKW:
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmA256GCMKW,
			AESGCMKW:  cryptolib.AESGCMKWParams{ContentEncryptionAlgorithm: encAlg},
		}, nil
	default:
		return cryptolib.AlgorithmParams{}, fmt.Errorf("unsupported JWE algorithm: %s", alg)
	}
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
		epk, err := extractEPKFromHeader(header)
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
		epk, err := extractEPKFromHeader(header)
		if err != nil {
			return cryptolib.AlgorithmParams{}, err
		}
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmECDHESA128KW,
			ECDHES:    cryptolib.ECDHESParams{EPK: epk},
		}, nil
	case ECDHESA192KW:
		epk, err := extractEPKFromHeader(header)
		if err != nil {
			return cryptolib.AlgorithmParams{}, err
		}
		return cryptolib.AlgorithmParams{
			Algorithm: cryptolib.AlgorithmECDHESA192KW,
			ECDHES:    cryptolib.ECDHESParams{EPK: epk},
		}, nil
	case ECDHESA256KW:
		epk, err := extractEPKFromHeader(header)
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
