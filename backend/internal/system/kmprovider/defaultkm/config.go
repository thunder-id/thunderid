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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/cryptolab"
	"github.com/thunder-id/thunderid/internal/system/cryptolab/hash"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
)

type encryptionService struct {
	defaultKeyID string
	keys         map[string][]byte
}

func newEncryptionService(key []byte) kmprovider.ConfigCryptoProvider {
	kid := hash.GenerateThumbprint(key)
	return &encryptionService{
		defaultKeyID: kid,
		keys:         map[string][]byte{kid: key},
	}
}

func (es *encryptionService) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	key := es.defaultKey()
	if len(key) == 0 {
		return nil, errors.New("default encryption key not found")
	}
	ciphertext, _, err := cryptolab.Encrypt(
		key, &cryptolab.AlgorithmParams{Algorithm: cryptolab.AlgorithmAESGCM}, plaintext,
	)
	if err != nil {
		return nil, err
	}
	encData := EncryptedData{
		Algorithm:  AESGCM,
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		KeyID:      es.defaultKeyID,
	}
	jsonData, err := json.Marshal(encData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize encrypted data: %w", err)
	}
	return jsonData, nil
}

func (es *encryptionService) Decrypt(_ context.Context, encodedData []byte) ([]byte, error) {
	var encData EncryptedData
	if err := json.Unmarshal(encodedData, &encData); err != nil {
		return nil, fmt.Errorf("invalid data format: %w", err)
	}
	if encData.Algorithm != AESGCM {
		return nil, fmt.Errorf("unsupported algorithm: %s", encData.Algorithm)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encData.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding: %w", err)
	}
	key := es.keyForDecrypt(encData.KeyID)
	if len(key) == 0 {
		return nil, errors.New("decryption key not found for kid")
	}
	return cryptolab.Decrypt(key, cryptolab.AlgorithmParams{Algorithm: cryptolab.AlgorithmAESGCM}, ciphertext)
}

func (es *encryptionService) defaultKey() []byte {
	if es.defaultKeyID == "" || len(es.keys) == 0 {
		return nil
	}
	return es.keys[es.defaultKeyID]
}

func (es *encryptionService) keyForDecrypt(kid string) []byte {
	if kid == "" || len(es.keys) == 0 {
		return nil
	}
	return es.keys[kid]
}
