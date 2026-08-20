// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package defaultkm provides the default key manager implementation backed by PKI key material.
package defaultkm

import (
	"encoding/hex"
	"errors"
	"sync"

	"github.com/thunder-id/thunderid/internal/system/config"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pki"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize returns a fully wired RuntimeCryptoProvider and ConfigCryptoProvider.
func Initialize(pkiSvc pki.PKIServiceInterface) (
	providers.RuntimeCryptoProvider, kmprovider.ConfigCryptoProvider, error,
) {
	cfgSvc, err := initConfigProvider()
	if err != nil {
		return nil, nil, err
	}

	runtimeSvc := NewRuntimeCryptoService(pkiSvc, cfgSvc)
	return runtimeSvc, cfgSvc, nil
}

var (
	globalCfgSvc kmprovider.ConfigCryptoProvider
	globalOnce   sync.Once
	initErr      error
)

// GetConfigCryptoService returns the singleton ConfigCryptoProvider for the default key manager.
//
// Most callers should take the provider as a dependency, as the server does elsewhere. This exists
// for the few built during start-up, before the key manager has been installed, which therefore
// cannot be handed one.
func GetConfigCryptoService() (kmprovider.ConfigCryptoProvider, error) {
	globalOnce.Do(func() {
		globalCfgSvc, initErr = initConfigProvider()
	})
	if initErr != nil {
		return nil, initErr
	}
	return globalCfgSvc, nil
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
	return newConfigCryptoService(key), nil
}
