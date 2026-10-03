// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ModelTestSuite struct {
	suite.Suite
}

func TestModelTestSuite(t *testing.T) {
	suite.Run(t, new(ModelTestSuite))
}

func (s *ModelTestSuite) TestParseType() {
	cases := []struct {
		value    string
		expected Type
		ok       bool
	}{
		{"basic", TypeBasic, true},
		{"BASIC", TypeBasic, true},
		{"  Basic  ", TypeBasic, true},
		{"none", TypeNone, true},
		{"", TypeNone, true},
		{"   ", TypeNone, true},
		{"bearer", "", false},
		{"oauth2", "", false},
	}

	for _, tc := range cases {
		parsed, ok := ParseType(tc.value)
		s.Equal(tc.ok, ok, "value %q", tc.value)
		s.Equal(tc.expected, parsed, "value %q", tc.value)
	}
}

// ParseType reads the registry rather than a list of its own, so a method added to the table is
// parsable without a second edit.
func (s *ModelTestSuite) TestParseTypeFollowsRegistry() {
	const apiKey Type = "api_key"

	_, ok := ParseType(string(apiKey))
	s.False(ok)

	withTemporaryMethod(Method{Type: apiKey, DisplayName: "API key"}, func() {
		parsed, ok := ParseType("  API_KEY  ")
		s.True(ok)
		s.Equal(apiKey, parsed)
	})
}

func (s *ModelTestSuite) TestConfigEnabled() {
	s.False(Config{}.Enabled())
	s.False(Config{Type: TypeNone}.Enabled())
	s.True(basicConfig().Enabled())
}

// recordingTarget is a stand-in transport target that records which method touched it.
type recordingTarget struct {
	presented []Type
}

// recordingAuthenticator records its method on the target and the configuration it was built from.
type recordingAuthenticator struct {
	authType   Type
	authConfig Config
}

func (authenticator *recordingAuthenticator) Authenticate(_ context.Context, target *recordingTarget) error {
	target.presented = append(target.presented, authenticator.authType)
	return nil
}

func recordingBindings(types ...Type) Bindings[*recordingTarget] {
	bindings := make(Bindings[*recordingTarget], 0, len(types))
	for _, authType := range types {
		bindings = append(bindings, Binding[*recordingTarget]{
			Type: authType,
			New: func(authConfig Config) Authenticator[*recordingTarget] {
				return &recordingAuthenticator{authType: authType, authConfig: authConfig}
			},
		})
	}
	return bindings
}

func (s *ModelTestSuite) TestBindingsSupportedTypesKeepsBindingOrder() {
	s.Equal([]Type{TypeBasic, TypeNone}, recordingBindings(TypeBasic, TypeNone).SupportedTypes())
	s.Empty(Bindings[*recordingTarget]{}.SupportedTypes())
}

func (s *ModelTestSuite) TestBindingsNewBuildsTheBoundMethod() {
	authConfig := basicConfig()
	authenticator, err := recordingBindings(TypeNone, TypeBasic).New(authConfig)
	s.Require().NoError(err)

	built, ok := authenticator.(*recordingAuthenticator)
	s.Require().True(ok)
	s.Equal(authConfig, built.authConfig)

	target := &recordingTarget{}
	s.Require().NoError(authenticator.Authenticate(context.Background(), target))
	s.Equal([]Type{TypeBasic}, target.presented)
}

func (s *ModelTestSuite) TestBindingsNewTreatsEmptyTypeAsNone() {
	authenticator, err := recordingBindings(TypeNone, TypeBasic).New(Config{})
	s.Require().NoError(err)

	target := &recordingTarget{}
	s.Require().NoError(authenticator.Authenticate(context.Background(), target))
	s.Equal([]Type{TypeNone}, target.presented)
}

func (s *ModelTestSuite) TestBindingsNewRejectsAnUnboundType() {
	_, err := recordingBindings(TypeNone).New(basicConfig())
	s.ErrorContains(err, "not supported by this transport: basic")

	_, err = recordingBindings(TypeBasic).New(Config{})
	s.ErrorContains(err, "not supported by this transport: none")
}
