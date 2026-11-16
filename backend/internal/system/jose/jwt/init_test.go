// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

type InitTestSuite struct {
	suite.Suite
	testKeyPath    string
	testPrivateKey *rsa.PrivateKey
	tempFiles      []string
}

func TestInitSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupSuite() {
	// Generate a test RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(suite.T(), err)
	suite.testPrivateKey = privateKey

	// Create a temporary private key file
	tempFile, err := os.CreateTemp("", "test_init_key_*.pem")
	assert.NoError(suite.T(), err)
	suite.testKeyPath = tempFile.Name()

	// Encode the private key to PEM
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Write to file
	_, err = tempFile.Write(privateKeyPEM)
	assert.NoError(suite.T(), err)
	err = tempFile.Close()
	assert.NoError(suite.T(), err)
}

func (suite *InitTestSuite) TearDownSuite() {
	// Clean up any temporary files created during tests
	for _, file := range suite.tempFiles {
		err := os.Remove(file)
		if err != nil {
			suite.T().Logf("Failed to remove temp file %s: %v", file, err)
		}
	}
	if suite.testKeyPath != "" {
		err := os.Remove(suite.testKeyPath)
		if err != nil {
			suite.T().Logf("Failed to remove test key file %s: %v", suite.testKeyPath, err)
		}
	}
}

func (suite *InitTestSuite) SetupTest() {
	// jwt.Initialize builds an HTTP client that reads the TLS config from the global
	// runtime; initialize a minimal runtime so construction does not panic.
	config.ResetServerRuntime()
	if err := config.InitializeServerRuntime("", &config.Config{}); err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *InitTestSuite) TestInitialize_Success() {
	cfg := joseconfig.Config{
		Issuer:         "https://auth.example.com",
		ValidityPeriod: 3600,
		PreferredKeyID: "test-kid",
	}

	cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	cryptoMock.EXPECT().
		GetPublicKeys(mock.Anything, providers.PublicKeyFilter{KeyID: "test-kid"}).
		Return([]providers.PublicKeyInfo{{
			KeyID:      "test-kid",
			Algorithm:  string(cryptolib.AlgorithmRS256),
			PublicKey:  &suite.testPrivateKey.PublicKey,
			Thumbprint: "test-kid",
		}}, nil)
	cryptoMock.EXPECT().GetSupportedSigningAlgorithms().Return([]string{string(cryptolib.AlgorithmRS256)})

	jwtService, err := Initialize(cryptoMock, cfg, httpservice.Initialize(true), httpservice.Initialize(false))
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), jwtService)
	assert.Implements(suite.T(), (*JWTServiceInterface)(nil), jwtService)
}

func (suite *InitTestSuite) TestInitialize_PublicKeyRetrievalError() {
	cfg := joseconfig.Config{
		Issuer:         "https://auth.example.com",
		ValidityPeriod: 3600,
		PreferredKeyID: "test-kid",
	}

	cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	cryptoMock.EXPECT().
		GetPublicKeys(mock.Anything, providers.PublicKeyFilter{KeyID: "test-kid"}).
		Return(nil, errors.New("provider unavailable"))

	jwtService, err := Initialize(cryptoMock, cfg, httpservice.Initialize(true), httpservice.Initialize(false))
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), jwtService)
	assert.Contains(suite.T(), err.Error(), "failed to retrieve public key")
}

func (suite *InitTestSuite) TestInitialize_WithoutPreferredKeyID() {
	cfg := joseconfig.Config{
		Issuer:         "https://auth.example.com",
		ValidityPeriod: 3600,
		// PreferredKeyID is empty
	}

	cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	cryptoMock.EXPECT().
		GetPublicKeys(mock.Anything, providers.PublicKeyFilter{KeyID: ""}).
		Return([]providers.PublicKeyInfo{{
			KeyID:      "",
			Algorithm:  string(cryptolib.AlgorithmRS256),
			PublicKey:  &suite.testPrivateKey.PublicKey,
			Thumbprint: "test-kid",
		}}, nil)
	cryptoMock.EXPECT().GetSupportedSigningAlgorithms().Return([]string{string(cryptolib.AlgorithmRS256)})

	jwtService, err := Initialize(cryptoMock, cfg, httpservice.Initialize(true), httpservice.Initialize(false))
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), jwtService)
}
