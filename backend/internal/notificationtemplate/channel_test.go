// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ChannelTestSuite struct {
	suite.Suite
}

func TestChannelTestSuite(t *testing.T) {
	suite.Run(t, new(ChannelTestSuite))
}

func (s *ChannelTestSuite) TestRulesFor() {
	s.Run("email", func() {
		h, err := rulesFor(ChannelTypeEmail)
		s.Require().Nil(err)
		s.Require().IsType(emailRules{}, h)
	})
	s.Run("sms", func() {
		h, err := rulesFor(ChannelTypeSMS)
		s.Require().Nil(err)
		s.Require().IsType(smsRules{}, h)
	})
	s.Run("unknown", func() {
		h, err := rulesFor("push")
		s.Require().Nil(h)
		s.Require().NotNil(err)
		s.Require().Equal(ErrorInvalidChannel.Code, err.Code)
	})
}

func (s *ChannelTestSuite) TestValidateChannel() {
	s.Require().Nil(validateChannel(ChannelTypeEmail))
	s.Require().Nil(validateChannel(ChannelTypeSMS))
	s.Require().NotNil(validateChannel(""))
	s.Require().Equal(ErrorInvalidChannel.Code, validateChannel("carrier-pigeon").Code)
}

func (s *ChannelTestSuite) TestEmailRulesValidate() {
	h := emailRules{}

	s.Require().Nil(h.validate(templateDAO{Content: TemplateContent{Subject: "s", Body: "b"}}))
	s.Require().Nil(h.validate(templateDAO{Content: TemplateContent{Subject: "s", Body: "b"},
		Design: &TemplateDesign{ColorScheme: colorSchemeDark}}))

	// Email requires a subject.
	s.Require().Equal(ErrorMissingSubject.Code,
		h.validate(templateDAO{Content: TemplateContent{Body: "b"}}).Code)
	s.Require().Equal(ErrorMissingSubject.Code,
		h.validate(templateDAO{Content: TemplateContent{Subject: "  \t ", Body: "b"}}).Code)

	// Color scheme is the only design value validated.
	s.Require().Equal(ErrorInvalidColorScheme.Code,
		h.validate(templateDAO{Content: TemplateContent{Subject: "s", Body: "b"},
			Design: &TemplateDesign{ColorScheme: "teal"}}).Code)
}

func (s *ChannelTestSuite) TestEmailRulesNormalize() {
	h := emailRules{}

	// No design stays absent.
	got := h.normalize(templateDAO{Content: TemplateContent{Body: "b"}})
	s.Require().Nil(got.Design)

	// A design with no color scheme is treated as absent (default applied later at resolution).
	got = h.normalize(templateDAO{Content: TemplateContent{Body: "b"}, Design: &TemplateDesign{}})
	s.Require().Nil(got.Design)

	// A real color scheme is preserved and the subject is kept for email.
	got = h.normalize(templateDAO{Content: TemplateContent{Subject: "s", Body: "b"},
		Design: &TemplateDesign{ColorScheme: colorSchemeLight}})
	s.Require().Equal("s", got.Content.Subject)
	s.Require().NotNil(got.Design)
	s.Require().Equal(colorSchemeLight, got.Design.ColorScheme)
}

func (s *ChannelTestSuite) TestSMSRulesStrict() {
	h := smsRules{}

	// A plain-text body only is valid.
	s.Require().Nil(h.validate(templateDAO{Content: TemplateContent{Body: "b"}}))

	// A subject or a design is rejected, not silently dropped.
	s.Require().Equal(ErrorSubjectNotAllowed.Code,
		h.validate(templateDAO{Content: TemplateContent{Subject: "s", Body: "b"}}).Code)
	s.Require().Equal(ErrorDesignNotAllowed.Code,
		h.validate(templateDAO{Content: TemplateContent{Body: "b"},
			Design: &TemplateDesign{ColorScheme: colorSchemeDark}}).Code)

	// Normalize drops any subject and design.
	got := h.normalize(templateDAO{Content: TemplateContent{Subject: "s", Body: "b"}})
	s.Require().Empty(got.Content.Subject)
	s.Require().Equal("b", got.Content.Body)
	s.Require().Nil(got.Design)
}
