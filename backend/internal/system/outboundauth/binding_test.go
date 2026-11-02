// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type BindingTestSuite struct {
	suite.Suite
}

func TestBindingTestSuite(t *testing.T) {
	suite.Run(t, new(BindingTestSuite))
}

// mockBindings binds a mock Authenticator to each type, and records the configuration each
// binding was built from. A mock with no expectation fails the test if it is ever called.
func (s *BindingTestSuite) mockBindings(types ...Type) (
	Bindings[string], map[Type]*AuthenticatorMock[string], map[Type]Config) {
	authenticators := make(map[Type]*AuthenticatorMock[string], len(types))
	built := make(map[Type]Config, len(types))
	bindings := make(Bindings[string], 0, len(types))
	for _, authType := range types {
		authenticator := NewAuthenticatorMock[string](s.T())
		authenticators[authType] = authenticator
		bindings = append(bindings, Binding[string]{
			Type: authType,
			New: func(authConfig Config) Authenticator[string] {
				built[authType] = authConfig
				return authenticator
			},
		})
	}
	return bindings, authenticators, built
}

func (s *BindingTestSuite) TestBindingsSupportedTypesKeepsBindingOrder() {
	bindings, _, _ := s.mockBindings(TypeBasic, TypeNone)
	s.Equal([]Type{TypeBasic, TypeNone}, bindings.SupportedTypes())
	s.Empty(Bindings[string]{}.SupportedTypes())
}

func (s *BindingTestSuite) TestBindingsNewBuildsTheBoundMethod() {
	bindings, authenticators, built := s.mockBindings(TypeNone, TypeBasic)
	authConfig := basicConfig()

	authenticator, err := bindings.New(authConfig)
	s.Require().NoError(err)
	s.Same(authenticators[TypeBasic], authenticator)
	s.Equal(authConfig, built[TypeBasic])
	s.NotContains(built, TypeNone)

	authenticators[TypeBasic].EXPECT().Authenticate(mock.Anything, "target").Return(nil).Once()
	s.Require().NoError(authenticator.Authenticate(context.Background(), "target"))
}

func (s *BindingTestSuite) TestBindingsNewTreatsEmptyTypeAsNone() {
	bindings, authenticators, built := s.mockBindings(TypeNone, TypeBasic)

	authenticator, err := bindings.New(Config{})
	s.Require().NoError(err)
	s.Same(authenticators[TypeNone], authenticator)
	s.NotContains(built, TypeBasic)
}

func (s *BindingTestSuite) TestBindingsNewRejectsAnUnboundType() {
	bindings, _, _ := s.mockBindings(TypeNone)
	_, err := bindings.New(basicConfig())
	s.ErrorContains(err, "not supported by this transport: basic")

	bindings, _, _ = s.mockBindings(TypeBasic)
	_, err = bindings.New(Config{})
	s.ErrorContains(err, "not supported by this transport: none")
}
