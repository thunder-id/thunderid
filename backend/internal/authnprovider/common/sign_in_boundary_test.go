// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SignInBoundaryTestSuite struct {
	suite.Suite
}

func TestSignInBoundaryTestSuite(t *testing.T) {
	suite.Run(t, new(SignInBoundaryTestSuite))
}

func (suite *SignInBoundaryTestSuite) TestRoundTrip() {
	boundary := SignInBoundary{OUID: "ou-1", ApplicationID: "app-1", EntityID: "user-1"}

	got, ok := SignInBoundaryFrom(WithSignInBoundary(context.Background(), boundary))

	suite.True(ok)
	suite.Equal(boundary, got)
}

func (suite *SignInBoundaryTestSuite) TestAbsentWhenNotSet() {
	got, ok := SignInBoundaryFrom(context.Background())

	suite.False(ok)
	suite.Equal(SignInBoundary{}, got)
}

func (suite *SignInBoundaryTestSuite) TestEmptyOUIDIsStillSet() {
	got, ok := SignInBoundaryFrom(WithSignInBoundary(context.Background(), SignInBoundary{ApplicationID: "app-1"}))

	suite.True(ok)
	suite.Empty(got.OUID)
	suite.Equal("app-1", got.ApplicationID)
}
