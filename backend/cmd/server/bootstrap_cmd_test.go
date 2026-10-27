// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability"
)

type BootstrapCmdTestSuite struct {
	suite.Suite
}

func TestBootstrapCmdTestSuite(t *testing.T) {
	suite.Run(t, new(BootstrapCmdTestSuite))
}

func (suite *BootstrapCmdTestSuite) SetupTest() {
	suite.T().Setenv("ADMIN_USERNAME", "")
	suite.T().Setenv("ADMIN_PASSWORD", "")
	suite.T().Setenv("PUBLIC_URL", "https://localhost:8090")

	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", &config.Config{})
}

func (suite *BootstrapCmdTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *BootstrapCmdTestSuite) TestParseBootstrapOptions_FlagsWin() {
	_, err := parseBootstrapOptions("/tmp/test", []string{
		"--admin-username", "custom-admin",
		"--admin-password", "custom-password",
	})

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "custom-admin", os.Getenv("ADMIN_USERNAME"))
	assert.Equal(suite.T(), "custom-password", os.Getenv("ADMIN_PASSWORD"))
}

func (suite *BootstrapCmdTestSuite) TestParseBootstrapOptions_ExportsIndexedRedirectURIs() {
	_, err := parseBootstrapOptions("/tmp/test", []string{
		"--admin-password", "custom-password",
		"--console-redirect-uris", "https://localhost:5191/console",
		"--admin-redirect-uris", " https://localhost:5192/admin , ,https://admin.example.com/admin",
		"--account-redirect-uris", "https://localhost:5193/account",
	})

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "https://localhost:5191/console", os.Getenv("CONSOLE_REDIRECT_URIS_0"))
	assert.Equal(suite.T(), "https://localhost:5192/admin", os.Getenv("ADMIN_REDIRECT_URIS_0"))
	assert.Equal(suite.T(), "https://admin.example.com/admin", os.Getenv("ADMIN_REDIRECT_URIS_1"))
	assert.Equal(suite.T(), "https://localhost:5193/account", os.Getenv("ACCOUNT_REDIRECT_URIS_0"))
	_, exists := os.LookupEnv("ADMIN_REDIRECT_URIS_2")
	assert.False(suite.T(), exists)
}

func (suite *BootstrapCmdTestSuite) TestParseBootstrapOptions_EnvVarsWinOverDefaults() {
	suite.T().Setenv("ADMIN_USERNAME", "env-admin")
	suite.T().Setenv("ADMIN_PASSWORD", "env-password")

	_, err := parseBootstrapOptions("/tmp/test", []string{})

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "env-admin", os.Getenv("ADMIN_USERNAME"))
	assert.Equal(suite.T(), "env-password", os.Getenv("ADMIN_PASSWORD"))
}

func (suite *BootstrapCmdTestSuite) TestParseBootstrapOptions_FailsWhenPasswordMissing() {
	_, err := parseBootstrapOptions("/tmp/test", []string{})

	assert.Error(suite.T(), err, "bootstrap must fail rather than default or generate a password itself")
}

func (suite *BootstrapCmdTestSuite) TestParseBootstrapOptions_UsernameDefaultsToAdminWhenPasswordSupplied() {
	suite.T().Setenv("ADMIN_PASSWORD", "supplied-password")

	_, err := parseBootstrapOptions("/tmp/test", []string{})

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "admin", os.Getenv("ADMIN_USERNAME"), "username should default to admin")
	assert.Equal(suite.T(), "supplied-password", os.Getenv("ADMIN_PASSWORD"))
}

func (suite *BootstrapCmdTestSuite) TestParseBootstrapOptions_FlagUsernameOnlyStillFailsWithoutPassword() {
	_, err := parseBootstrapOptions("/tmp/test", []string{"--admin-username", "custom-admin"})

	assert.Error(suite.T(), err)
}

func (suite *BootstrapCmdTestSuite) TestRunBootstrap_TearsDownAndReturnsErrorWhenPasswordMissing() {
	// Simulate `thunderid bootstrap` with no --admin-password and no ADMIN_PASSWORD, so
	// parseBootstrapOptions fails and runBootstrap must tear down and return the error
	// instead of proceeding to seed resources.
	origCmdLine := flag.CommandLine
	origObservability := observabilitySvc
	suite.T().Cleanup(func() {
		flag.CommandLine = origCmdLine
		observabilitySvc = origObservability
	})

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	_ = flag.CommandLine.Parse([]string{bootstrapSubcommand})

	// shutdownBootstrap tears observability down; a disabled service handles it as a no-op.
	observabilitySvc = observability.Initialize(config.GetServerRuntime().Config.Observability)

	err := runBootstrap(context.Background(), log.GetLogger(), suite.T().TempDir(), nil, nil)

	assert.Error(suite.T(), err, "runBootstrap must return an error when no admin password is supplied")
}
