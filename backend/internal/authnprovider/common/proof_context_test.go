// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ProofContextTestSuite struct {
	suite.Suite
}

func TestProofContextTestSuite(t *testing.T) {
	suite.Run(t, new(ProofContextTestSuite))
}

func (suite *ProofContextTestSuite) TestRecoveryProofRoundTrip() {
	ctx := WithRecoveryProof(context.Background())
	suite.True(IsRecoveryProof(ctx))
	suite.False(IsContactProof(ctx))
}

func (suite *ProofContextTestSuite) TestContactProofRoundTrip() {
	ctx := WithContactProof(context.Background())
	suite.True(IsContactProof(ctx))
	suite.False(IsRecoveryProof(ctx))
}

func (suite *ProofContextTestSuite) TestUnmarkedContextIsNeitherProof() {
	suite.False(IsRecoveryProof(context.Background()))
	suite.False(IsContactProof(context.Background()))
}
