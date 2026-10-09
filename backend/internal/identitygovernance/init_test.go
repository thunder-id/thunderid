// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"testing"

	"github.com/stretchr/testify/suite"

	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) TestInitializeRequiresAnEntityStateProvider() {
	svc, err := Initialize(nil, sysconfig.AccountAccessConfig{}, nil, nil, nil)
	s.Require().ErrorIs(err, ErrNoEntityStateProvider)
	s.Require().Nil(svc)
}

func (s *InitTestSuite) TestInitializeBuildsTheService() {
	svc, err := Initialize(NewEntityStateProviderMock(s.T()), sysconfig.AccountAccessConfig{}, nil, nil, nil)
	s.Require().NoError(err)
	s.Require().NotNil(svc)
}
