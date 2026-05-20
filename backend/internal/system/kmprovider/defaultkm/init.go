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

// Package defaultkm provides the default key manager implementation backed by PKI key material.
package defaultkm

import (
	"encoding/hex"
	"errors"
	"sync"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pkiservice"
	"github.com/thunder-id/thunderid/internal/system/log"
)

var (
	globalRuntimeSvc kmprovider.RuntimeCryptoProvider
	globalCfgSvc     kmprovider.ConfigCryptoProvider
	globalOnce       sync.Once
	initErr          error
)

// GetRuntimeCryptoService returns the singleton RuntimeCryptoProvider for the default key manager.
func GetRuntimeCryptoService() (kmprovider.RuntimeCryptoProvider, error) {
	globalOnce.Do(func() {
		globalRuntimeSvc, globalCfgSvc, initErr = Initialize()
	})
	return globalRuntimeSvc, initErr
}

// GetEncryptionService returns the singleton ConfigCryptoProvider for the default key manager.
func GetEncryptionService() (kmprovider.ConfigCryptoProvider, error) {
	if _, err := GetRuntimeCryptoService(); err != nil {
		return nil, err
	}
	return globalCfgSvc, nil
}

// InitConfigProvider returns a ConfigCryptoProvider initialized from the server config.
func InitConfigProvider() (kmprovider.ConfigCryptoProvider, error) {
	return initConfigProvider()
}

// Initialize returns a fully wired RuntimeCryptoProvider and ConfigCryptoProvider.
func Initialize() (kmprovider.RuntimeCryptoProvider, kmprovider.ConfigCryptoProvider, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "defaultkm"))

	cfgSvc, err := initConfigProvider()
	if err != nil {
		return nil, nil, err
	}

	pkiSvc, err := pkiservice.Initialize()
	if err != nil {
		logger.Warn("PKI service unavailable; asymmetric operations will fail",
			log.String("reason", err.Error()))
	}

	runtimeSvc := NewRuntimeCryptoService(pkiSvc, cfgSvc)
	return runtimeSvc, cfgSvc, nil
}

func initConfigProvider() (kmprovider.ConfigCryptoProvider, error) {
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
	return newEncryptionService(key), nil
}
