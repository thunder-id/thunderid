// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package consent

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) TestInitialize_RefusesANilEventBus() {
	service, err := Initialize(NewInboundClientProviderMock(s.T()), nil)

	s.ErrorIs(err, errNilEventBus)
	s.Nil(service)
}
