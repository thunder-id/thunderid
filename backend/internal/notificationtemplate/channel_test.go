// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"strconv"
	"strings"
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

	// Subject and body are length-bounded; at-limit passes, over-limit is rejected and the error
	// reports the configured limit via the maxLength param.
	subjectLimit := templateConfig().SubjectLengthLimit()
	emailBodyLimit := templateConfig().EmailBodyLengthLimit()
	s.Require().Nil(h.validate(templateDAO{Content: TemplateContent{
		Subject: strings.Repeat("s", subjectLimit),
		Body:    strings.Repeat("b", emailBodyLimit)}}))
	subjErr := h.validate(templateDAO{Content: TemplateContent{
		Subject: strings.Repeat("s", subjectLimit+1), Body: "b"}})
	s.Require().Equal(ErrorSubjectTooLong.Code, subjErr.Code)
	s.Require().Equal(strconv.Itoa(subjectLimit), subjErr.ErrorDescription.Params["maxLength"])
	bodyErr := h.validate(templateDAO{Content: TemplateContent{
		Subject: "s", Body: strings.Repeat("b", emailBodyLimit+1)}})
	s.Require().Equal(ErrorBodyTooLong.Code, bodyErr.Code)
	s.Require().Equal(strconv.Itoa(emailBodyLimit), bodyErr.ErrorDescription.Params["maxLength"])
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

	// Body is length-bounded; at-limit passes, over-limit is rejected and reports the limit.
	smsBodyLimit := templateConfig().SMSBodyLengthLimit()
	s.Require().Nil(h.validate(templateDAO{Content: TemplateContent{
		Body: strings.Repeat("b", smsBodyLimit)}}))
	smsBodyErr := h.validate(templateDAO{Content: TemplateContent{
		Body: strings.Repeat("b", smsBodyLimit+1)}})
	s.Require().Equal(ErrorBodyTooLong.Code, smsBodyErr.Code)
	s.Require().Equal(strconv.Itoa(smsBodyLimit), smsBodyErr.ErrorDescription.Params["maxLength"])

	// Normalize drops any subject and design.
	got := h.normalize(templateDAO{Content: TemplateContent{Subject: "s", Body: "b"}})
	s.Require().Empty(got.Content.Subject)
	s.Require().Equal("b", got.Content.Body)
	s.Require().Nil(got.Design)
}
