// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package smtpauth

import (
	"context"
	"net/smtp"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/outboundauth"
)

type SMTPAuthTestSuite struct {
	suite.Suite
}

func TestSMTPAuthTestSuite(t *testing.T) {
	suite.Run(t, new(SMTPAuthTestSuite))
}

func (s *SMTPAuthTestSuite) TestBasicPresentsPlainMechanism() {
	authenticator, err := New(outboundauth.Config{
		Type: outboundauth.TypeBasic,
		Properties: map[string]string{
			outboundauth.FieldBasicUsername: "mailer",
			outboundauth.FieldBasicPassword: "s3cret",
		},
	})
	s.Require().NoError(err)

	auth, err := authenticator.Auth(context.Background(), "smtp.example.com")
	s.Require().NoError(err)
	s.Require().NotNil(auth)

	mechanism, response, err := auth.Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: true})
	s.Require().NoError(err)
	s.Equal("PLAIN", mechanism)
	s.Equal("\x00mailer\x00s3cret", string(response))
}

func (s *SMTPAuthTestSuite) TestBasicIsBoundToTheHost() {
	authenticator, err := New(outboundauth.Config{
		Type: outboundauth.TypeBasic,
		Properties: map[string]string{
			outboundauth.FieldBasicUsername: "mailer",
			outboundauth.FieldBasicPassword: "s3cret",
		},
	})
	s.Require().NoError(err)

	auth, err := authenticator.Auth(context.Background(), "smtp.example.com")
	s.Require().NoError(err)

	// PlainAuth refuses to hand credentials to a server it was not bound to, which is what
	// stops a redirected or spoofed connection from collecting them.
	_, _, err = auth.Start(&smtp.ServerInfo{Name: "evil.example.com", TLS: true})
	s.Require().Error(err)
}

func (s *SMTPAuthTestSuite) TestNoneAndZeroConfigPresentNothing() {
	for _, cfg := range []outboundauth.Config{{Type: outboundauth.TypeNone}, {}} {
		authenticator, err := New(cfg)
		s.Require().NoError(err)

		auth, err := authenticator.Auth(context.Background(), "smtp.example.com")
		s.Require().NoError(err)
		s.Nil(auth)
	}
}

func (s *SMTPAuthTestSuite) TestUnsupportedTypeIsRejected() {
	_, err := New(outboundauth.Config{Type: outboundauth.Type("bearer")})
	s.Require().Error(err)
	s.Contains(err.Error(), "not supported over SMTP")
}

// Every registered method must be either bound here or listed as unbindable with a reason. That
// is what stops a method added to the registry from compiling cleanly and then failing at send
// time: whoever adds it has to decide, in this package, whether SMTP can carry it.
func (s *SMTPAuthTestSuite) TestEveryRegisteredMethodIsAccountedFor() {
	for _, authType := range outboundauth.GetTypes() {
		_, bound := New(outboundauth.Config{Type: authType})
		reason, excluded := unbindable[authType]

		if bound == nil {
			s.NotContains(unbindable, authType,
				"method %q is both bound and listed as unbindable", authType)
			continue
		}

		s.True(excluded,
			"method %q is registered but neither bound by SMTP nor listed as unbindable: bind it "+
				"in bindings, or add it to unbindable with the reason SMTP cannot carry it", authType)
		s.NotEmpty(reason, "method %q must say why SMTP cannot carry it", authType)
	}
}

// SupportedTypes and New read the same table, so a caller cannot validate against a method the
// transport is unable to build.
func (s *SMTPAuthTestSuite) TestSupportedTypesMatchesWhatNewCanBuild() {
	supported := SupportedTypes()
	s.Require().NotEmpty(supported)

	for _, authType := range supported {
		_, err := New(outboundauth.Config{Type: authType})
		s.NoError(err, "SupportedTypes advertises %q but New cannot build it", authType)
	}

	// Render order is part of the contract: the console offers the methods in this order.
	s.Equal([]outboundauth.Type{outboundauth.TypeNone, outboundauth.TypeBasic}, supported)

	// The returned slice is a copy, so a caller holding it cannot reorder the transport's table.
	supported[0] = outboundauth.Type("tampered")
	s.Equal(outboundauth.TypeNone, SupportedTypes()[0])
}
