// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type TestUtilsTestSuite struct {
	suite.Suite
}

func TestTestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(TestUtilsTestSuite))
}

func (ts *TestUtilsTestSuite) TestWithoutTopLevelKeyDropsTheWholeBlock() {
	doc := "id: user-1\nsharingPolicies:\n  - childUnits:\n      mode: all\n\n  - otherTreeRoots:\n      mode: all\nattributes:\n  username: alice\n"

	got := WithoutTopLevelKey(doc, "sharingPolicies")

	ts.Equal("id: user-1\nattributes:\n  username: alice\n", got)
}

func (ts *TestUtilsTestSuite) TestWithoutTopLevelKeyLeavesOtherKeysAlone() {
	doc := "id: user-1\nattributes:\n  id: nested\n"

	ts.Equal("attributes:\n  id: nested\n", WithoutTopLevelKey(doc, "id"))
}

func (ts *TestUtilsTestSuite) TestServerPortIgnoresAServerPortTheRunnerDidNotSet() {
	ts.T().Setenv(TestGroupEnv, "")
	ts.T().Setenv("SERVER_EXTRACTED_HOME", "")
	ts.T().Setenv("SERVER_PORT", "3000")

	ts.Equal(DefaultServerPort, ServerPort())
}

func (ts *TestUtilsTestSuite) TestServerPortUsesTheTestGroupPort() {
	ts.T().Setenv(TestGroupEnv, "g1")
	ts.T().Setenv("SERVER_EXTRACTED_HOME", "")
	ts.T().Setenv("SERVER_PORT", "8195")

	ts.Equal("8195", ServerPort())
}

func (ts *TestUtilsTestSuite) TestServerPortUsesThePortTheRunnerPassesToTestPackages() {
	ts.T().Setenv(TestGroupEnv, "")
	ts.T().Setenv("SERVER_EXTRACTED_HOME", "/tmp/product")
	ts.T().Setenv("SERVER_PORT", "8195")

	ts.Equal("8195", ServerPort())
}
