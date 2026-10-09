// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/identitygovernance/model"
)

type EnforcementTestSuite struct {
	suite.Suite
}

func TestEnforcementTestSuite(t *testing.T) {
	suite.Run(t, new(EnforcementTestSuite))
}

// Under entity granularity, entries under authentication methods are not enforced.
func (s *EnforcementTestSuite) TestEntityGranularityUnenforcesEveryMethodScope() {
	enforcement := NewLockEnforcement(allMethodScopes())

	s.False(enforcement.Enforces(model.AccessScopeCredential))
	s.False(enforcement.Enforces(model.AccessScopeOTP))
	s.True(enforcement.Enforces(model.AccessScopeEntity),
		"the reserved key is never unenforced")
}
