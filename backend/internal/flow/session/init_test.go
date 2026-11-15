// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) TestInitialize_RefusesANilEventBus() {
	svc, hook, err := Initialize(providermock.NewDBProviderInterfaceMock(suite.T()), "deployment", Timeouts{},
		nil, nil, nil)

	suite.ErrorIs(err, errNilEventBus)
	suite.Nil(svc)
	suite.Nil(hook)
}
