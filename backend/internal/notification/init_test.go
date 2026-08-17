// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"errors"
	"fmt"
	"os"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testCryptoKey = "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"
)

// TestMain wires cmodels' package-level config crypto provider once for the whole test
// binary, so secret Property encryption works regardless of which test's SetupTest last
// reset the server runtime.
func TestMain(m *testing.M) {
	config.ResetServerRuntime()
	if err := config.InitializeServerRuntime("/tmp/test", &config.Config{
		Crypto: config.CryptoConfig{Encryption: engineconfig.EncryptionConfig{Key: testCryptoKey}},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize server runtime: %v\n", err)
		os.Exit(1)
	}
	_, cfgCryptoSvc, err := defaultkm.Initialize(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize default crypto provider: %v\n", err)
		os.Exit(1)
	}
	cmodels.SetConfigCryptoProvider(cfgCryptoSvc)
	config.ResetServerRuntime()
	os.Exit(m.Run())
}

type InitTestSuite struct {
	suite.Suite
	mockJWTService *jwtmock.JWTServiceInterfaceMock
	cacheManager   cache.CacheManagerInterface
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupSuite() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: engineconfig.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
		},
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *InitTestSuite) SetupTest() {
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.cacheManager = cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment")
}

func (suite *InitTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
}

func (suite *InitTestSuite) TestInitialize() {
	mgtService, _, _, err := Initialize(suite.cacheManager, suite.mockJWTService)
	suite.NoError(err)

	suite.NotNil(mgtService)
	suite.Implements((*NotificationSenderMgtSvcInterface)(nil), mgtService)
}

func (suite *InitTestSuite) TestInitialize_StoreErrorPropagates() {
	originalGetDBProvider := getDBProvider
	mockProvider := providermock.NewDBProviderInterfaceMock(suite.T())
	mockProvider.On("GetConfigDBClient").Return(nil, errors.New("db unavailable"))
	getDBProvider = func() provider.DBProviderInterface { return mockProvider }
	defer func() { getDBProvider = originalGetDBProvider }()

	mgtService, otpService, senderService, err := Initialize(suite.cacheManager, suite.mockJWTService)

	suite.Error(err)
	suite.Nil(mgtService)
	suite.Nil(otpService)
	suite.Nil(senderService)
}
