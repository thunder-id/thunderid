// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	notificationconfig "github.com/thunder-id/thunderid/internal/notification/config"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
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
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupTest() {
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
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
}

func (suite *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *InitTestSuite) TestInitialize() {
	mgtService, _, _, configHandler, err := Initialize(suite.mockJWTService, notificationconfig.Config{}, nil)
	suite.NoError(err)

	suite.NotNil(mgtService)
	suite.Implements((*NotificationSenderMgtSvcInterface)(nil), mgtService)
	suite.NotNil(configHandler)
}

// The returned handler decodes the notification section and protects its declarative layer.
func (suite *InitTestSuite) TestInitialize_ReturnsTheNotificationSectionHandler() {
	_, _, _, configHandler, err := Initialize(suite.mockJWTService, notificationconfig.Config{}, nil)
	suite.Require().NoError(err)
	suite.IsType(&serverConfigHandler{}, configHandler)

	declarative, err := configHandler.Decode(json.RawMessage(`{"defaultEmailSenderId":"file"}`))
	suite.Require().NoError(err)
	suite.Equal(notificationconfig.NotificationSectionConfig{DefaultEmailSenderID: "file"}, declarative)
	_, err = configHandler.Decode(json.RawMessage(`{"defaultEmailSender":"file"}`))
	suite.Error(err, "an unknown field must be refused")
	writable := notificationconfig.NotificationSectionConfig{DefaultEmailSenderID: "db"}
	suite.ErrorIs(configHandler.Validate(writable, declarative, notificationconfig.NotificationSectionConfig{}),
		errDeclarativeSenderLocked)
	suite.Equal(declarative, configHandler.Merge(declarative, writable))
}

func (suite *InitTestSuite) TestInitialize_StoreErrorPropagates() {
	originalGetDBProvider := getDBProvider
	mockProvider := providermock.NewDBProviderInterfaceMock(suite.T())
	mockProvider.On("GetConfigDBClient").Return(nil, errors.New("db unavailable"))
	getDBProvider = func() provider.DBProviderInterface { return mockProvider }
	defer func() { getDBProvider = originalGetDBProvider }()

	mgtService, otpService, senderService, configHandler, err :=
		Initialize(suite.mockJWTService, notificationconfig.Config{}, nil)

	suite.Error(err)
	suite.Nil(mgtService)
	suite.Nil(otpService)
	suite.Nil(senderService)
	suite.Nil(configHandler)
}

func (suite *InitTestSuite) TestInitializeUsesInjectedConfig() {
	originalGetDBProvider := getDBProvider
	mockProvider := providermock.NewDBProviderInterfaceMock(suite.T())
	getDBProvider = func() provider.DBProviderInterface { return mockProvider }
	defer func() { getDBProvider = originalGetDBProvider }()
	cfg := notificationconfig.Config{
		DeclarativeResourcesEnabled: true,
		JWT:                         engineconfig.JWTConfig{Issuer: "injected-issuer"},
		OTP: config.OTPConfig{
			Length: 8, UseNumericOnly: new(true), ValidityPeriodSeconds: 300,
		},
	}
	suite.mockJWTService.EXPECT().GenerateJWT(mock.Anything, otpSessionAudience, "injected-issuer",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("injected-session", int64(0), (*tidcommon.ServiceError)(nil)).Once()

	_, otpService, _, _, err := Initialize(suite.mockJWTService, cfg, nil)

	suite.Require().NoError(err)
	token, otp, expirySeconds, svcErr := otpService.GenerateOTP(
		context.Background(), "holder@example.com", "email", nil)
	suite.Require().Nil(svcErr)
	suite.Equal("injected-session", token)
	suite.Len(otp, 8)
	suite.Equal(int64(300), expirySeconds)
	for _, ch := range otp {
		suite.Contains("0123456789", string(ch))
	}
}
