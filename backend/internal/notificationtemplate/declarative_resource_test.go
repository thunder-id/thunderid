// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// declaredEmailYAML is a minimal valid email template declaration.
const declaredEmailYAML = `resource_type: notification_template
id: welcome-email
channel: email
handle: welcome
displayName: Welcome
description: Greeting
content:
  subject: Hi
  body: Hello {{ctx(name)}}
`

func loadDeclared(t *testing.T, fileStore *templateFileBasedStore, yaml string) {
	t.Helper()
	dto, err := parseDeclaredTemplate([]byte(yaml))
	if err != nil {
		t.Fatalf("parseDeclaredTemplate: %v", err)
	}
	store := &declaredTemplateStore{fileStore: fileStore, logger: log.GetLogger()}
	if err := store.Create("", dto); err != nil {
		t.Fatalf("declaredTemplateStore.Create: %v", err)
	}
}

type DeclarativeResourceTestSuite struct {
	suite.Suite
}

func TestDeclarativeResourceTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeResourceTestSuite))
}

func (s *DeclarativeResourceTestSuite) TestValidationRejectsBadDoc() {
	// SMS with a subject is invalid.
	const badSMS = `resource_type: notification_template
id: otp-sms
channel: sms
handle: otp
displayName: OTP
content:
  subject: nope
  body: Code {{ctx(code)}}
`
	dto, err := parseDeclaredTemplate([]byte(badSMS))
	s.Require().NoError(err)
	s.Require().Error(validateDeclaredTemplate(dto), "expected validation to reject an SMS template with a subject")
}
