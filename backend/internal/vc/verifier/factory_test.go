// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/vc/vcverifierprovidermock"
)

const testFormat = "dc+sd-jwt"

type VCVerifierFactoryTestSuite struct {
	suite.Suite
}

func TestVCVerifierFactoryTestSuite(t *testing.T) {
	suite.Run(t, new(VCVerifierFactoryTestSuite))
}

func (suite *VCVerifierFactoryTestSuite) TestRegisterAndGetVerifier() {
	factory := NewVCVerifierFactory()
	verifier := vcverifierprovidermock.NewVCVerifierProviderMock(suite.T())

	suite.Require().NoError(factory.Register(testFormat, verifier))

	got, err := factory.GetVerifier(testFormat)
	suite.Require().NoError(err)
	suite.Same(verifier, got)
}

func (suite *VCVerifierFactoryTestSuite) TestRegisterRejectsDuplicateFormat() {
	factory := NewVCVerifierFactory()
	first := vcverifierprovidermock.NewVCVerifierProviderMock(suite.T())
	second := vcverifierprovidermock.NewVCVerifierProviderMock(suite.T())

	suite.Require().NoError(factory.Register(testFormat, first))
	suite.ErrorIs(factory.Register(testFormat, second), ErrVerifierAlreadyRegistered)

	got, err := factory.GetVerifier(testFormat)
	suite.Require().NoError(err)
	suite.Same(first, got)
}

func (suite *VCVerifierFactoryTestSuite) TestRegisterRejectsInvalidInput() {
	factory := NewVCVerifierFactory()

	suite.ErrorIs(factory.Register("", vcverifierprovidermock.NewVCVerifierProviderMock(suite.T())),
		ErrInvalidRegistration)
	suite.ErrorIs(factory.Register(testFormat, nil), ErrInvalidRegistration)
}

func (suite *VCVerifierFactoryTestSuite) TestGetVerifierUnknownFormat() {
	factory := NewVCVerifierFactory()

	got, err := factory.GetVerifier("jwt_vc_json")
	suite.ErrorIs(err, ErrVerifierNotFound)
	suite.Nil(got)
}

func (suite *VCVerifierFactoryTestSuite) TestRegisterDistinctFormats() {
	factory := NewVCVerifierFactory()
	sdjwt := vcverifierprovidermock.NewVCVerifierProviderMock(suite.T())
	mdoc := vcverifierprovidermock.NewVCVerifierProviderMock(suite.T())

	suite.Require().NoError(factory.Register(testFormat, sdjwt))
	suite.Require().NoError(factory.Register("mso_mdoc", mdoc))

	got, err := factory.GetVerifier("mso_mdoc")
	suite.Require().NoError(err)
	suite.Same(mdoc, got)
}
