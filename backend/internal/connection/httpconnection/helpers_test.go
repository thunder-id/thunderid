// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

const (
	testCryptoKey    = "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"
	testDeploymentID = "test-deployment"
	testURL          = "https://risk.example.com/evaluate"
	testPassword     = "basic-password-value"
	// testMaskedSecret is the placeholder every read returns in place of a secret.
	testMaskedSecret = "******"
)

// initTestRuntime loads a server runtime with an encryption key, so secret properties are encrypted
// as they are on a running server.
func initTestRuntime(t *testing.T, declarative bool) {
	t.Helper()
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", &config.Config{
		Server:               engineconfig.ServerConfig{Identifier: testDeploymentID},
		Crypto:               config.CryptoConfig{Encryption: engineconfig.EncryptionConfig{Key: testCryptoKey}},
		DeclarativeResources: config.DeclarativeResources{Enabled: declarative},
	}))
	t.Cleanup(config.ResetServerRuntime)
	_, cfgCryptoSvc, err := defaultkm.Initialize(nil)
	require.NoError(t, err)
	cmodels.SetConfigCryptoProvider(cfgCryptoSvc)
}

// authProperties renders an outbound authentication configuration as the stored property bag.
func authProperties(t *testing.T, authType outboundauth.Type, values map[string]string) []cmodels.Property {
	t.Helper()
	props, err := outboundauth.ToProperties(outboundauth.Config{Type: authType, Properties: values})
	require.NoError(t, err)
	return props
}

func basicAuthProperties(t *testing.T) []cmodels.Property {
	t.Helper()
	return authProperties(t, outboundauth.TypeBasic, map[string]string{
		outboundauth.FieldBasicUsername: "svc-user",
		outboundauth.FieldBasicPassword: testPassword,
	})
}
