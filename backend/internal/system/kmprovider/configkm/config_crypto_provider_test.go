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

package configkm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncryptionService_Encrypt_NoDefaultKey covers lines 48-50: when the
// configCryptoService has no default key, Encrypt should return an error.
func TestEncryptionService_Encrypt_NoDefaultKey(t *testing.T) {
	es := &configCryptoService{
		defaultKeyID: "",
		keys:         map[string][]byte{},
	}
	_, err := es.Encrypt(context.Background(), []byte("plaintext"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default encryption key not found")
}

// TestEncryptionService_Encrypt_InvalidKeySize covers lines 54-56: when the
// stored key has an invalid AES size, the underlying Encrypt call returns an
// error that the service propagates.
func TestEncryptionService_Encrypt_InvalidKeySize(t *testing.T) {
	es := &configCryptoService{
		defaultKeyID: "bad-key",
		keys:         map[string][]byte{"bad-key": {0x01}}, // 1 byte — invalid AES key length
	}
	_, err := es.Encrypt(context.Background(), []byte("plaintext"))
	require.Error(t, err)
}
