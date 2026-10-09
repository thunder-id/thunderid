// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ValidateTestSuite struct {
	suite.Suite
}

func TestValidateTestSuite(t *testing.T) {
	suite.Run(t, new(ValidateTestSuite))
}

func (s *ValidateTestSuite) TestValidateAcceptsValidConfigurations() {
	s.NoError(Validate(basicConfig(), allTypes()))
	s.NoError(Validate(Config{Type: TypeNone}, allTypes()))
	s.NoError(Validate(Config{}, allTypes()))
}

func (s *ValidateTestSuite) TestValidateRejectsUnknownType() {
	err := Validate(Config{Type: Type("digest")}, allTypes())
	s.Require().Error(err)
	s.Contains(err.Error(), "unsupported authentication type")
}

func (s *ValidateTestSuite) TestValidateRejectsTypeOutsideSupportedSet() {
	err := Validate(basicConfig(), []Type{TypeNone})
	s.Require().Error(err)
	s.Contains(err.Error(), "not supported by this provider")
}

func (s *ValidateTestSuite) TestValidateRejectsMissingAndBlankRequiredFields() {
	missing := Config{Type: TypeBasic, Properties: map[string]string{FieldBasicUsername: "mailer"}}
	err := Validate(missing, allTypes())
	s.Require().Error(err)
	s.Contains(err.Error(), FieldBasicPassword)

	blank := Config{Type: TypeBasic, Properties: map[string]string{
		FieldBasicUsername: "   ",
		FieldBasicPassword: "s3cret",
	}}
	err = Validate(blank, allTypes())
	s.Require().Error(err)
	s.Contains(err.Error(), FieldBasicUsername)
}

func (s *ValidateTestSuite) TestValidateAcceptsBlankOptionalField() {
	const apiKey Type = "custom_optional"
	withTemporaryMethod(Method{
		Type:        apiKey,
		DisplayName: "API key",
		Fields:      []Field{{Key: "header", Type: fieldTypeText, DisplayName: "Header"}},
	}, func() {
		s.NoError(Validate(Config{Type: apiKey, Properties: map[string]string{"header": " "}},
			[]Type{apiKey}))
	})
}

func (s *ValidateTestSuite) TestValidateRejectsUndeclaredField() {
	cfg := basicConfig()
	cfg.Properties["clientSecret"] = "nope"

	err := Validate(cfg, allTypes())
	s.Require().Error(err)
	s.Contains(err.Error(), "unknown authentication property")
}

func (s *ValidateTestSuite) TestValidateRejectsPropertiesOnNone() {
	cfg := Config{Type: TypeNone, Properties: map[string]string{FieldBasicUsername: "mailer"}}
	err := Validate(cfg, allTypes())
	s.Require().Error(err)
	s.Contains(err.Error(), "unknown authentication property")
}
