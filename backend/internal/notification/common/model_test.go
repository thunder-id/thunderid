// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type EmailDataTestSuite struct {
	suite.Suite
}

func TestEmailDataTestSuite(t *testing.T) {
	suite.Run(t, new(EmailDataTestSuite))
}

func (suite *EmailDataTestSuite) TestValidate_TrimsInPlace() {
	data := EmailData{
		To:      []string{"  user@example.com  "},
		CC:      []string{" cc@example.com "},
		BCC:     []string{" bcc@example.com "},
		Subject: "  Hello  ",
	}

	suite.Require().NoError(data.Validate())
	suite.Equal([]string{"user@example.com"}, data.To)
	suite.Equal([]string{"cc@example.com"}, data.CC)
	suite.Equal([]string{"bcc@example.com"}, data.BCC)
	suite.Equal("Hello", data.Subject)
}

// A list that is only blank entries must not pass as "has a recipient".
func (suite *EmailDataTestSuite) TestValidate_DropsBlankEntries() {
	data := EmailData{To: []string{"user@example.com", "   "}, CC: []string{"  "}}

	suite.Require().NoError(data.Validate())
	suite.Equal([]string{"user@example.com"}, data.To)
	suite.Empty(data.CC)
}

func (suite *EmailDataTestSuite) TestValidate_AllowsEmptySubjectAndBody() {
	data := EmailData{To: []string{"user@example.com"}}
	suite.NoError(data.Validate())
}

func (suite *EmailDataTestSuite) TestValidate_Errors() {
	cases := []struct {
		name string
		data EmailData
	}{
		{"no recipient", EmailData{Subject: "s"}},
		{"only blank recipients", EmailData{To: []string{"   ", ""}}},
		{"malformed recipient", EmailData{To: []string{"not-an-address"}}},
		{"malformed cc", EmailData{To: []string{"user@example.com"}, CC: []string{"bad"}}},
		{"malformed bcc", EmailData{To: []string{"user@example.com"}, BCC: []string{"bad"}}},
		{"display name recipient", EmailData{To: []string{`"Bob" <bob@example.com>`}}},
		{"CR in recipient", EmailData{To: []string{"user@example.com\rBcc: a@evil.test"}}},
		{"LF in recipient", EmailData{To: []string{"user@example.com\nBcc: a@evil.test"}}},
		{"CRLF in subject", EmailData{
			To: []string{"user@example.com"}, Subject: "hi\r\nBcc: a@evil.test"}},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.Error(tc.data.Validate())
		})
	}
}

func (suite *EmailDataTestSuite) TestIsValidEmailAddress() {
	valid := []string{
		"user@example.com",
		"user.name+tag@sub.example.co.uk",
		"u@e.io",
	}
	for _, address := range valid {
		suite.True(IsValidEmailAddress(address), address)
	}

	invalid := []string{
		"",
		"   ",
		"plainstring",
		"@example.com",
		"user@",
		`"Bob" <bob@example.com>`,
		"<bob@example.com>",
		"user@example.com\r\nBcc: attacker@evil.test",
		"user@example.com\n",
	}
	for _, address := range invalid {
		suite.False(IsValidEmailAddress(address), address)
	}
}

func (suite *EmailDataTestSuite) TestIsValidHeaderText() {
	// A display name is free text: everything but a line break belongs to the header value.
	for _, value := range []string{"", "Acme Support", "Acme, Inc.", `Acme "Support"`, "Café"} {
		suite.True(IsValidHeaderText(value), value)
	}

	for _, value := range []string{
		"Acme\r\nBcc: attacker@evil.test",
		"Acme\nBcc: attacker@evil.test",
		"Acme\r",
	} {
		suite.False(IsValidHeaderText(value), value)
	}
}

func (suite *EmailDataTestSuite) TestParseTLSMode() {
	cases := []struct {
		input    string
		expected TLSMode
		ok       bool
	}{
		// An omitted value resolves to the secure default, not plaintext.
		{"", TLSModeSTARTTLS, true},
		{"  ", TLSModeSTARTTLS, true},
		{"none", TLSModeNone, true},
		{"NONE", TLSModeNone, true},
		{"starttls", TLSModeSTARTTLS, true},
		{"StartTLS", TLSModeSTARTTLS, true},
		{"implicit", TLSModeImplicit, true},
		{"true", "", false},
		{"false", "", false},
		{"ssl", "", false},
	}

	for _, tc := range cases {
		mode, ok := ParseTLSMode(tc.input)
		suite.Equal(tc.ok, ok, tc.input)
		suite.Equal(tc.expected, mode, tc.input)
	}
}
