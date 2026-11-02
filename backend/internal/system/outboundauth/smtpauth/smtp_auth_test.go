// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package smtpauth

import (
	"context"
	"errors"
	"net/smtp"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/tests/mocks/outboundauth/smtpauthmock"
)

type SMTPAuthTestSuite struct {
	suite.Suite
}

func TestSMTPAuthTestSuite(t *testing.T) {
	suite.Run(t, new(SMTPAuthTestSuite))
}

func basicConfig() outboundauth.Config {
	return outboundauth.Config{
		Type: outboundauth.TypeBasic,
		Properties: map[string]string{
			outboundauth.FieldBasicUsername: "mailer",
			outboundauth.FieldBasicPassword: "s3cret",
		},
	}
}

// expectAuth makes session accept one AUTH exchange and returns the mechanism it was handed.
func (s *SMTPAuthTestSuite) expectAuth(session *smtpauthmock.SessionMock) *smtp.Auth {
	var mechanism smtp.Auth
	session.EXPECT().Auth(mock.Anything).RunAndReturn(func(received smtp.Auth) error {
		mechanism = received
		return nil
	}).Once()
	return &mechanism
}

func (s *SMTPAuthTestSuite) TestBasicPresentsPlainMechanism() {
	authenticator, err := New(basicConfig(), "smtp.example.com")
	s.Require().NoError(err)

	session := smtpauthmock.NewSessionMock(s.T())
	mechanism := s.expectAuth(session)
	s.Require().NoError(authenticator.Authenticate(context.Background(), session))
	s.Require().NotNil(*mechanism)

	name, response, err := (*mechanism).Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: true})
	s.Require().NoError(err)
	s.Equal("PLAIN", name)
	s.Equal("\x00mailer\x00s3cret", string(response))
}

func (s *SMTPAuthTestSuite) TestBasicIsBoundToTheHost() {
	authenticator, err := New(basicConfig(), "smtp.example.com")
	s.Require().NoError(err)

	session := smtpauthmock.NewSessionMock(s.T())
	mechanism := s.expectAuth(session)
	s.Require().NoError(authenticator.Authenticate(context.Background(), session))
	s.Require().NotNil(*mechanism)

	// PlainAuth refuses to hand credentials to a server it was not bound to, which is what
	// stops a redirected or spoofed connection from collecting them.
	_, _, err = (*mechanism).Start(&smtp.ServerInfo{Name: "evil.example.com", TLS: true})
	s.Require().Error(err)
}

func (s *SMTPAuthTestSuite) TestBasicReturnsTheSessionError() {
	authenticator, err := New(basicConfig(), "smtp.example.com")
	s.Require().NoError(err)

	rejected := errors.New("535 authentication failed")
	session := smtpauthmock.NewSessionMock(s.T())
	session.EXPECT().Auth(mock.Anything).Return(rejected).Once()
	err = authenticator.Authenticate(context.Background(), session)
	s.ErrorIs(err, rejected)
}

func (s *SMTPAuthTestSuite) TestNoneAndZeroConfigPresentNothing() {
	for _, authConfig := range []outboundauth.Config{{Type: outboundauth.TypeNone}, {}} {
		authenticator, err := New(authConfig, "smtp.example.com")
		s.Require().NoError(err)

		// The mock fails the test on any call it was not told to expect, so no AUTH is attempted.
		session := smtpauthmock.NewSessionMock(s.T())
		s.Require().NoError(authenticator.Authenticate(context.Background(), session))
	}
}

func (s *SMTPAuthTestSuite) TestUnsupportedTypeIsRejected() {
	_, err := New(outboundauth.Config{Type: outboundauth.Type("bearer")}, "smtp.example.com")
	s.Require().Error(err)
	s.Contains(err.Error(), "smtp")
	s.Contains(err.Error(), "not supported")
}

// SupportedTypes and New read the same table, so a caller cannot validate against a method the
// transport is unable to build.
func (s *SMTPAuthTestSuite) TestSupportedTypesMatchesWhatNewCanBuild() {
	supported := SupportedTypes()
	s.Require().NotEmpty(supported)

	for _, authType := range supported {
		_, err := New(outboundauth.Config{Type: authType}, "smtp.example.com")
		s.NoError(err, "SupportedTypes advertises %q but New cannot build it", authType)
	}
	s.Len(outboundauth.GetMethods(supported), len(supported),
		"SupportedTypes advertises a method that is not registered")

	// Order is part of the contract: clients offer the methods in this order.
	s.Equal([]outboundauth.Type{outboundauth.TypeNone, outboundauth.TypeBasic}, supported)

	// The returned slice is a copy, so a caller holding it cannot reorder the transport's table.
	supported[0] = outboundauth.Type("tampered")
	s.Equal(outboundauth.TypeNone, SupportedTypes()[0])
}
