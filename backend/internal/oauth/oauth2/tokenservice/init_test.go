// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package tokenservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

type InitTestSuite struct {
	suite.Suite
	mockJWTService *jwtmock.JWTServiceInterfaceMock
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupTest() {
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
}

func (suite *InitTestSuite) TestInitialize() {
	tokenBuilder, tokenValidator := Initialize(
		testhelpers.OAuthConfig(), suite.mockJWTService, nil, nil, nil, nil, nil, nil)

	assert.NotNil(suite.T(), tokenBuilder)
	assert.Implements(suite.T(), (*TokenBuilderInterface)(nil), tokenBuilder)

	assert.NotNil(suite.T(), tokenValidator)
	assert.Implements(suite.T(), (*TokenValidatorInterface)(nil), tokenValidator)
}
