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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

const testCryptoKey = "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"

func setTestEncryptionKey(t *testing.T, key string) {
	t.Helper()
	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	err := config.InitializeServerRuntime("", &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{Key: key},
		},
	})
	require.NoError(t, err)
}

func TestInitConfigProvider_EmptyKey(t *testing.T) {
	setTestEncryptionKey(t, "")
	_, err := InitConfigProvider()
	assert.EqualError(t, err, "encryption key not configured in crypto.encryption.key")
}

func TestInitConfigProvider_InvalidHex(t *testing.T) {
	setTestEncryptionKey(t, "not-valid-hex")
	_, err := InitConfigProvider()
	assert.Error(t, err)
}

func TestInitConfigProvider_InvalidKeyLength(t *testing.T) {
	setTestEncryptionKey(t, "aabbcc")
	_, err := InitConfigProvider()
	assert.EqualError(t, err, "invalid AES key length: must be 16, 24, or 32 bytes")
}

func TestInitConfigProvider_Success(t *testing.T) {
	setTestEncryptionKey(t, testCryptoKey)
	svc, err := InitConfigProvider()
	require.NoError(t, err)
	assert.NotNil(t, svc)
}

func resetGlobalConfigCryptoService(t *testing.T) {
	t.Helper()
	globalOnce = sync.Once{}
	globalCfgSvc = nil
	initErr = nil
	t.Cleanup(func() {
		globalOnce = sync.Once{}
		globalCfgSvc = nil
		initErr = nil
	})
}

func TestGetConfigCryptoService_Success(t *testing.T) {
	resetGlobalConfigCryptoService(t)
	setTestEncryptionKey(t, testCryptoKey)

	svc, err := GetConfigCryptoService()
	require.NoError(t, err)
	assert.NotNil(t, svc)
}

func TestGetConfigCryptoService_Error(t *testing.T) {
	resetGlobalConfigCryptoService(t)
	setTestEncryptionKey(t, "")

	_, err := GetConfigCryptoService()
	assert.EqualError(t, err, "encryption key not configured in crypto.encryption.key")
}
