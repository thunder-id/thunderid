// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type RecoveryContextTestSuite struct {
	suite.Suite
}

func TestRecoveryContextTestSuite(t *testing.T) {
	suite.Run(t, new(RecoveryContextTestSuite))
}

func (suite *RecoveryContextTestSuite) TestContextRoundTrip() {
	suite.True(IsRecoveryFlow(WithRecoveryFlow(context.Background())))
}

func (suite *RecoveryContextTestSuite) TestUnmarkedContextIsNotRecovery() {
	suite.False(IsRecoveryFlow(context.Background()))
}
