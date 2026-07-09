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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/pki/pkimock"
)

const testCryptoKey = "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"

func setInitTestEncryptionKey(t *testing.T, key string) {
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

func TestInitialize_ConfigProviderError(t *testing.T) {
	setInitTestEncryptionKey(t, "")
	pkiSvc := pkimock.NewPKIServiceInterfaceMock(t)

	runtimeSvc, cfgSvc, err := Initialize(pkiSvc)

	assert.Nil(t, runtimeSvc)
	assert.Nil(t, cfgSvc)
	assert.EqualError(t, err, "encryption key not configured in crypto.encryption.key")
}

func TestInitialize_Success(t *testing.T) {
	setInitTestEncryptionKey(t, testCryptoKey)
	pkiSvc := pkimock.NewPKIServiceInterfaceMock(t)

	runtimeSvc, cfgSvc, err := Initialize(pkiSvc)

	require.NoError(t, err)
	assert.NotNil(t, runtimeSvc)
	assert.NotNil(t, cfgSvc)
}
