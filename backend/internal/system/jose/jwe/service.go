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
	"crypto"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pkiservice"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// JWEServiceInterface defines the interface for JWE operations.
type JWEServiceInterface interface {
	Encrypt(payload []byte, recipientPublicKey crypto.PublicKey,
		alg KeyEncAlgorithm, enc ContentEncAlgorithm, cty string, kid string) (string, *serviceerror.ServiceError)
	Decrypt(jweToken string) ([]byte, *serviceerror.ServiceError)
}

// jweService implements the JWEServiceInterface.
type jweService struct {
	privateKey crypto.PrivateKey
	kid        string
	logger     *log.Logger
}

// newJWEService creates a new JWE service instance.
func newJWEService(pkiService pkiservice.PKIServiceInterface) (JWEServiceInterface, error) {
	preferredKid := config.GetServerRuntime().Config.JWT.PreferredKeyID

	privateKey, err := pkiService.GetPrivateKey(preferredKid)
	if err != nil {
		return nil, errors.New("failed to retrieve private key for the key id: " + preferredKid)
	}

	kid := pkiService.GetCertThumbprint(preferredKid)
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWEService"))

	return &jweService{
		privateKey: privateKey,
		kid:        kid,
		logger:     logger,
	}, nil
}

// Encrypt encrypts the payload using the recipient's public key.
// cty is the content type placed in the JWE protected header (e.g. "json" or "JWT").
// kid identifies the recipient's key; it is stamped in the header only when non-empty.
func (js *jweService) Encrypt(payload []byte, recipientPublicKey crypto.PublicKey,
	alg KeyEncAlgorithm, enc ContentEncAlgorithm, cty string, kid string) (string, *serviceerror.ServiceError) {
	// 1. Generate CEK
	cekSize := 0
	switch enc {
	case A128CBCHS256:
		cekSize = 32
	case A192CBCHS384:
		cekSize = 48
	case A256CBCHS512:
		cekSize = 64
	case A128GCM:
		cekSize = 16
	case A192GCM:
		cekSize = 24
	case A256GCM:
		cekSize = 32
	default:
		return "", &ErrorUnsupportedEncryptionAlgorithm
	}

	cek := make([]byte, cekSize)
	if _, err := rand.Read(cek); err != nil {
		js.logger.Error("Failed to generate CEK: " + err.Error())
		return "", &serviceerror.InternalServerError
	}

	// 2. Encrypt CEK
	encryptedKey, headerExtras, err := encryptKey(cek, alg, recipientPublicKey, enc)
	if err != nil {
		js.logger.Error("Failed to encrypt CEK: " + err.Error())
		return "", &ErrorUnsupportedJWEAlgorithm
	}

	// 3. Create Header
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

	// Add extras (like epk for ECDH-ES)
	for k, v := range headerExtras {
		header[k] = v
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		js.logger.Error("Failed to marshal JWE header: " + err.Error())
		return "", &serviceerror.InternalServerError
	}
	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// 4. Encrypt Content
	iv, ciphertext, tag, err := encryptContent(payload, cek, enc, []byte(headerBase64))
	if err != nil {
		js.logger.Error("Failed to encrypt content: " + err.Error())
		return "", &serviceerror.InternalServerError
	}

	// 5. Build Compact Serialization
	jweToken := fmt.Sprintf("%s.%s.%s.%s.%s",
		headerBase64,
		base64.RawURLEncoding.EncodeToString(encryptedKey),
		base64.RawURLEncoding.EncodeToString(iv),
		base64.RawURLEncoding.EncodeToString(ciphertext),
		base64.RawURLEncoding.EncodeToString(tag),
	)

	return jweToken, nil
}

// Decrypt decrypts the JWE compact serialization using the server's private key.
func (js *jweService) Decrypt(jweToken string) ([]byte, *serviceerror.ServiceError) {
	header, headerBase64, encryptedKey, iv, ciphertext, tag, err := DecodeJWE(jweToken)
	if err != nil {
		js.logger.Debug("Failed to decode JWE: " + err.Error())
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

	// 1. Decrypt CEK
	cek, err := decryptKey(encryptedKey, alg, js.privateKey, header, enc)
	if err != nil {
		js.logger.Error("Failed to decrypt CEK: " + err.Error())
		return nil, &ErrorJWEDecryptionFailed
	}

	// 2. Decrypt Content
	payload, err := decryptContent(ciphertext, iv, tag, cek, enc, []byte(headerBase64))
	if err != nil {
		js.logger.Error("Failed to decrypt content: " + err.Error())
		return nil, &ErrorJWEDecryptionFailed
	}

	return payload, nil
}
