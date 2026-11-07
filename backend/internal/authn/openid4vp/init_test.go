// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type OpenID4VPInitTestSuite struct {
	suite.Suite
}

func TestOpenID4VPInitTestSuite(t *testing.T) {
	suite.Run(t, new(OpenID4VPInitTestSuite))
}

func (suite *OpenID4VPInitTestSuite) TestLoadVerifierInfoEmptyPath() {
	got, err := loadVerifierInfo("", "")
	suite.Require().NoError(err)
	suite.Nil(got)
}

func (suite *OpenID4VPInitTestSuite) TestLoadVerifierInfoReadsFile() {
	dir := suite.T().TempDir()
	path := filepath.Join(dir, "registration.jwt")
	suite.Require().NoError(os.WriteFile(path, []byte("  eyJhbGciOiJFUzI1NiJ9.payload.sig  \n"), 0o600))

	got, err := loadVerifierInfo(path, "")
	suite.Require().NoError(err)
	suite.Require().Len(got, 1)
	entry := got[0].(map[string]interface{})
	suite.Equal("registration_cert", entry["format"])
	suite.Equal("eyJhbGciOiJFUzI1NiJ9.payload.sig", entry["data"])
}

func (suite *OpenID4VPInitTestSuite) TestLoadVerifierInfoResolvesRelativePath() {
	home := suite.T().TempDir()
	rel := "regcert.jwt"
	suite.Require().NoError(os.WriteFile(filepath.Join(home, rel), []byte("jwt-data"), 0o600))

	got, err := loadVerifierInfo(rel, home)
	suite.Require().NoError(err)
	suite.Require().Len(got, 1)
	entry := got[0].(map[string]interface{})
	suite.Equal("jwt-data", entry["data"])
}

func (suite *OpenID4VPInitTestSuite) TestLoadVerifierInfoMissingFile() {
	_, err := loadVerifierInfo(filepath.Join(suite.T().TempDir(), "missing"), "")
	suite.Error(err)
}

func (suite *OpenID4VPInitTestSuite) TestLoadVerifierInfoEmptyFile() {
	path := filepath.Join(suite.T().TempDir(), "empty.jwt")
	suite.Require().NoError(os.WriteFile(path, []byte("   \n"), 0o600))
	_, err := loadVerifierInfo(path, "")
	suite.ErrorIs(err, ErrPolicy)
}

// Initialize fails when a signing key is configured but client_id is missing.
func (suite *OpenID4VPInitTestSuite) TestInitializeRequiresClientID() {
	config.ResetServerRuntime()
	cfg := &config.Config{}
	cfg.OpenID4VP.SigningKeyID = "signing-key"
	suite.Require().NoError(config.InitializeServerRuntime("", cfg))
	defer config.ResetServerRuntime()

	svc, err := Initialize(http.NewServeMux(), nil, nil, nil, nil, nil)
	suite.Require().Error(err)
	suite.ErrorIs(err, ErrPolicy)
	suite.Nil(svc)
}

// resolvePath joins relative cert paths to serverHome and leaves absolute and
// empty paths untouched.
func (suite *OpenID4VPInitTestSuite) TestResolvePath() {
	home := filepath.Join(suite.T().TempDir(), "home")
	abs := filepath.Join(home, "abs", "cert")
	suite.Equal("", resolvePath(home, ""))
	suite.Equal(abs, resolvePath(home, abs))
	suite.Equal(filepath.Join(home, "rel", "cert"), resolvePath(home, filepath.Join("rel", "cert")))
	suite.Equal(filepath.Join("rel", "cert"), resolvePath("", filepath.Join("rel", "cert")))
}
