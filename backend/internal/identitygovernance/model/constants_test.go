// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConstantsTestSuite struct {
	suite.Suite
}

func TestConstantsTestSuite(t *testing.T) {
	suite.Run(t, new(ConstantsTestSuite))
}

func (s *ConstantsTestSuite) TestScopeClasses() {
	s.Equal(ClassLockable, ClassOf(AccessScopeCredential))
	s.Equal(ClassLockable, ClassOf(AccessScopeOTP))
	s.Equal(ClassLockable, ClassOf(AccessScopeEntity))
	s.Equal(ClassNotLockable, ClassOf(AccessScopeSystemCredential),
		"a client secret is never counted, whatever the category's configuration")

	for _, scope := range []AccessScope{
		AccessScopePasskey,
		AccessScopeMagicLink,
		AccessScopeFederated,
		AccessScopeOpenID4VP,
	} {
		s.Equal(ClassNotLockable, ClassOf(scope), "scope %s names no entity on failure", scope)
	}

	s.Equal(ClassNotLockable, ClassOf("unregistered"), "an unregistered scope locks nothing")
}

// A scope missing from scopeClasses would silently be unlockable whatever an operator configures.
func (s *ConstantsTestSuite) TestEveryRegisteredScopeHasAClass() {
	for _, scope := range AllAccessScopes {
		s.True(IsAccessScope(scope), "%s must be registered", scope)
		_, declared := scopeClasses[scope]
		s.True(declared, "registered scope %s must declare a class", scope)
	}
	s.Len(scopeClasses, len(AllAccessScopes))
}

func (s *ConstantsTestSuite) TestNoClassIsDeclaredForAnUnregisteredScope() {
	for scope := range scopeClasses {
		s.True(IsAccessScope(scope), "scope %q has a class but is not registered", scope)
	}
}

// Only a lockable scope is counted: nothing reads a count that can never form a lock.
func (s *ConstantsTestSuite) TestCountedClasses() {
	s.True(ClassLockable.Counted())
	s.False(ClassNotLockable.Counted())
}
