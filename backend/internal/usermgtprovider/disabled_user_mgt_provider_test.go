// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package usermgtprovider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type DisabledUserMgtProviderTestSuite struct {
	suite.Suite
	provider providers.UserMgtProvider
}

func (suite *DisabledUserMgtProviderTestSuite) SetupTest() {
	suite.provider = NewDisabledUserMgtProvider()
}

func TestDisabledUserMgtProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DisabledUserMgtProviderTestSuite))
}

// A deployment that disables the provider must not provision users from the runtime, and the caller
// must be told why rather than receiving a generic failure.
func (suite *DisabledUserMgtProviderTestSuite) TestCreateUserIsRejected() {
	resp, svcErr := suite.provider.CreateUser(context.Background(), &providers.User{
		OUID: testUserOUID,
		Type: testUserType,
	})

	suite.Nil(resp)
	suite.NotNil(svcErr)
	suite.Equal(ErrorUserProvisioningDisabled.Code, svcErr.Code)
}
