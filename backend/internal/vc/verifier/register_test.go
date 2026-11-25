// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/vc/vcverifierprovidermock"
)

const testFormat = "dc+sd-jwt"

type VCVerifierRegistryTestSuite struct {
	suite.Suite
}

func TestVCVerifierRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(VCVerifierRegistryTestSuite))
}

func (suite *VCVerifierRegistryTestSuite) TestRegisterAndGetVerifier() {
	registry := newVCVerifierRegistry()
	verifier := vcverifierprovidermock.NewVCVerifierProviderMock(suite.T())

	suite.Require().NoError(registry.Register(testFormat, verifier))

	got, err := registry.GetVerifier(testFormat)
	suite.Require().NoError(err)
	suite.Same(verifier, got)
}

func (suite *VCVerifierRegistryTestSuite) TestRegisterRejectsDuplicateFormat() {
	registry := newVCVerifierRegistry()
	first := vcverifierprovidermock.NewVCVerifierProviderMock(suite.T())
	second := vcverifierprovidermock.NewVCVerifierProviderMock(suite.T())

	suite.Require().NoError(registry.Register(testFormat, first))
	suite.ErrorIs(registry.Register(testFormat, second), ErrVerifierAlreadyRegistered)

	got, err := registry.GetVerifier(testFormat)
	suite.Require().NoError(err)
	suite.Same(first, got)
}

func (suite *VCVerifierRegistryTestSuite) TestRegisterRejectsInvalidInput() {
	registry := newVCVerifierRegistry()

	suite.ErrorIs(registry.Register("", vcverifierprovidermock.NewVCVerifierProviderMock(suite.T())),
		ErrInvalidRegistration)
	suite.ErrorIs(registry.Register(testFormat, nil), ErrInvalidRegistration)
}

func (suite *VCVerifierRegistryTestSuite) TestGetVerifierUnknownFormat() {
	registry := newVCVerifierRegistry()

	got, err := registry.GetVerifier("jwt_vc_json")
	suite.ErrorIs(err, ErrVerifierNotFound)
	suite.Nil(got)
}

func (suite *VCVerifierRegistryTestSuite) TestRegisterDistinctFormats() {
	registry := newVCVerifierRegistry()
	sdjwt := vcverifierprovidermock.NewVCVerifierProviderMock(suite.T())
	mdoc := vcverifierprovidermock.NewVCVerifierProviderMock(suite.T())

	suite.Require().NoError(registry.Register(testFormat, sdjwt))
	suite.Require().NoError(registry.Register("mso_mdoc", mdoc))

	got, err := registry.GetVerifier("mso_mdoc")
	suite.Require().NoError(err)
	suite.Same(mdoc, got)
}
