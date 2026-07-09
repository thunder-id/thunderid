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

// Package configkm provides the default config crypto provider implementation
// backed by statically configured symmetric keys.
package configkm

import (
	"encoding/hex"
	"errors"
	"sync"

	"github.com/thunder-id/thunderid/internal/system/config"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
)

var (
	globalCfgSvc kmprovider.ConfigCryptoProvider
	globalOnce   sync.Once
	initErr      error
)

// GetConfigCryptoService returns the singleton ConfigCryptoProvider for the default key manager.
//
// Deprecated
func GetConfigCryptoService() (kmprovider.ConfigCryptoProvider, error) {
	globalOnce.Do(func() {
		globalCfgSvc, initErr = InitConfigProvider()
	})
	if initErr != nil {
		return nil, initErr
	}
	return globalCfgSvc, nil
}

// InitConfigProvider builds a new ConfigCryptoProvider from the configured encryption key.
func InitConfigProvider() (kmprovider.ConfigCryptoProvider, error) {
	encryptionKey := config.GetServerRuntime().Config.Crypto.Encryption.Key
	if encryptionKey == "" {
		return nil, errors.New("encryption key not configured in crypto.encryption.key")
	}
	key, err := hex.DecodeString(encryptionKey)
	if err != nil {
		return nil, err
	}
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("invalid AES key length: must be 16, 24, or 32 bytes")
	}
	return newConfigCryptoService(key), nil
}
