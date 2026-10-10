// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type PlaceholderTestSuite struct {
	suite.Suite
	mockStore *notificationTemplateStoreInterfaceMock
	svc       NotificationTemplateServiceInterface
	ctx       context.Context
}

func TestPlaceholderTestSuite(t *testing.T) {
	suite.Run(t, new(PlaceholderTestSuite))
}

func (s *PlaceholderTestSuite) SetupTest() {
	s.mockStore = newNotificationTemplateStoreInterfaceMock(s.T())
	s.svc = newNotificationTemplateService(s.mockStore, executingTransactioner(s.T()))
	s.ctx = context.Background()
}

func (s *PlaceholderTestSuite) TestValidatePlaceholders_Valid() {
	// ctx and t are allowed in the subject; ctx, t and design are allowed in an email body.
	content := TemplateContent{
		Subject: "{{t(notification.otp.email.subject)}}",
		Body: "<p>{{t(notification.otp.email.message)}}</p><p>{{ctx(otp)}}</p>" +
			"<span style=\"color:{{design(palette.primary.main)}}\">{{ctx(expiryTime)}}</span>",
	}
	s.Require().Nil(validatePlaceholders(content, true))

	// A body with no placeholders is fine.
	s.Require().Nil(validatePlaceholders(TemplateContent{Body: "<p>plain</p>"}, true))
}

func (s *PlaceholderTestSuite) TestValidatePlaceholders_Malformed() {
	cases := []string{
		"{{ctx()}}",           // empty key
		"{{ctx( otp )}}",      // spaces not allowed (must match the render form)
		"{{translate(x)}}",    // unsupported function
		"{{otp}}",             // not a function call
		"{{ctx(otp)}} {{t(}}", // second placeholder malformed
	}
	for _, body := range cases {
		s.Require().Equal(ErrorInvalidPlaceholder.Code,
			validatePlaceholders(TemplateContent{Body: body}, true).Code, "body=%q", body)
	}
}

func (s *PlaceholderTestSuite) TestValidatePlaceholders_DesignScoping() {
	// design in a body is allowed only when the channel supports it.
	s.Require().Nil(validatePlaceholders(TemplateContent{Body: "{{design(palette.primary.main)}}"}, true))
	s.Require().Equal(ErrorDesignPlaceholderNotAllowed.Code,
		validatePlaceholders(TemplateContent{Body: "{{design(palette.primary.main)}}"}, false).Code)

	// design is never allowed in the subject, even for a design-capable channel.
	s.Require().Equal(ErrorDesignPlaceholderNotAllowed.Code,
		validatePlaceholders(TemplateContent{Subject: "{{design(x)}}", Body: "b"}, true).Code)
}

// TestUnresolvedBeforeContext reports leftover {{t}}/{{design}} and malformed blocks, but allows a
// well-formed {{ctx(...)}} (resolved by the following context pass) and ignores non-placeholder braces.
func (s *PlaceholderTestSuite) TestUnresolvedBeforeContext() {
	// Leftover translation/design tokens must be reported (an earlier pass should have resolved them).
	s.Equal("{{t(inner)}}", unresolvedBeforeContext("see {{t(inner)}} now"))
	s.Equal("{{design(palette.primary.main)}}", unresolvedBeforeContext("c {{design(palette.primary.main)}}"))
	// Malformed placeholders are reported.
	s.Equal("{{ctx( otp )}}", unresolvedBeforeContext("hi {{ctx( otp )}}"))
	s.Equal("{{translate(x)}}", unresolvedBeforeContext("{{translate(x)}}"))

	// A well-formed {{ctx(...)}} is allowed to remain for the context pass.
	s.Equal("", unresolvedBeforeContext("hi {{ctx(user.name)}}"))
	// Placeholder-free content returns "".
	s.Equal("", unresolvedBeforeContext("no placeholders"))
	s.Equal("", unresolvedBeforeContext("#fa7b3f and {text}"))
}

func (s *PlaceholderTestSuite) TestCreateTemplate_PlaceholderValidation() {
	s.mockStore.On("IsHandleExists", mock.Anything, ChannelTypeEmail, "otp").Return(false, nil)
	s.mockStore.On("CreateTemplate", mock.Anything, mock.Anything).Return(nil)

	// Email with a body design token is accepted.
	_, err := s.svc.CreateTemplate(s.ctx, ChannelTypeEmail, CreateTemplateRequest{
		Handle: "otp", DisplayName: "OTP",
		Content: TemplateContent{Subject: "s", Body: "{{ctx(otp)}} {{design(palette.primary.main)}}"}})
	s.Require().Nil(err)

	// SMS with a design token is rejected (before store).
	_, err = s.svc.CreateTemplate(s.ctx, ChannelTypeSMS, CreateTemplateRequest{
		Handle: "otp", DisplayName: "OTP",
		Content: TemplateContent{Body: "{{design(palette.primary.main)}}"}})
	s.Require().Equal(ErrorDesignPlaceholderNotAllowed.Code, err.Code)

	// A malformed placeholder is rejected on create (before store).
	_, err = s.svc.CreateTemplate(s.ctx, ChannelTypeEmail, CreateTemplateRequest{
		Handle: "otp2", DisplayName: "OTP",
		Content: TemplateContent{Subject: "s", Body: "{{ctx()}}"}})
	s.Require().Equal(ErrorInvalidPlaceholder.Code, err.Code)
}
