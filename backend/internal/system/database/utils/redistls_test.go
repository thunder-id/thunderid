// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

func writeTestCertPair(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "redis-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))
	return certPath, keyPath
}

func TestBuildRedisTLSConfig_Disabled(t *testing.T) {
	cfg, err := BuildRedisTLSConfig(engineconfig.RedisTLSConfig{CAFile: "ignored.pem"})
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestBuildRedisTLSConfig_EnabledDefaults(t *testing.T) {
	cfg, err := BuildRedisTLSConfig(engineconfig.RedisTLSConfig{Enabled: true, ServerName: "redis.internal"})
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	assert.Equal(t, "redis.internal", cfg.ServerName)
	assert.False(t, cfg.InsecureSkipVerify)
	assert.Nil(t, cfg.RootCAs)
	assert.Empty(t, cfg.Certificates)
}

func TestBuildRedisTLSConfig_CAAndClientCert(t *testing.T) {
	certPath, keyPath := writeTestCertPair(t, t.TempDir())

	cfg, err := BuildRedisTLSConfig(engineconfig.RedisTLSConfig{
		Enabled:            true,
		CAFile:             certPath,
		CertFile:           certPath,
		KeyFile:            keyPath,
		InsecureSkipVerify: true,
	})
	require.NoError(t, err)
	assert.NotNil(t, cfg.RootCAs)
	assert.Len(t, cfg.Certificates, 1)
	assert.True(t, cfg.InsecureSkipVerify)
}

func TestBuildRedisTLSConfig_Errors(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeTestCertPair(t, dir)
	badPEM := filepath.Join(dir, "bad.pem")
	require.NoError(t, os.WriteFile(badPEM, []byte("not a pem"), 0o600))

	tests := []struct {
		name string
		cfg  engineconfig.RedisTLSConfig
	}{
		{"missing ca file", engineconfig.RedisTLSConfig{Enabled: true, CAFile: filepath.Join(dir, "missing.pem")}},
		{"invalid ca pem", engineconfig.RedisTLSConfig{Enabled: true, CAFile: badPEM}},
		{"cert without key", engineconfig.RedisTLSConfig{Enabled: true, CertFile: certPath}},
		{"key without cert", engineconfig.RedisTLSConfig{Enabled: true, KeyFile: keyPath}},
		{"invalid key pair", engineconfig.RedisTLSConfig{Enabled: true, CertFile: certPath, KeyFile: badPEM}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := BuildRedisTLSConfig(tc.cfg)
			assert.Error(t, err)
			assert.Nil(t, cfg)
		})
	}
}
