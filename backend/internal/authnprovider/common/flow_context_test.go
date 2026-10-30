// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type FlowContextTestSuite struct {
	suite.Suite
}

func TestFlowContextTestSuite(t *testing.T) {
	suite.Run(t, new(FlowContextTestSuite))
}

func (suite *FlowContextTestSuite) TestRecoveryFlowRoundTrip() {
	ctx := WithRecoveryFlow(context.Background())
	suite.True(IsRecoveryFlow(ctx))
	suite.False(IsVerificationFlow(ctx))
}

func (suite *FlowContextTestSuite) TestVerificationFlowRoundTrip() {
	ctx := WithVerificationFlow(context.Background())
	suite.True(IsVerificationFlow(ctx))
	suite.False(IsRecoveryFlow(ctx))
}

func (suite *FlowContextTestSuite) TestUnmarkedContext() {
	suite.False(IsRecoveryFlow(context.Background()))
	suite.False(IsVerificationFlow(context.Background()))
}
