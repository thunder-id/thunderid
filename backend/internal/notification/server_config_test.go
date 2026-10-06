// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type ServerConfigTestSuite struct {
	suite.Suite
	senders *NotificationSenderMgtSvcInterfaceMock
	handler *serverConfigHandler
}

func TestServerConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ServerConfigTestSuite))
}

func (s *ServerConfigTestSuite) SetupTest() {
	s.senders = NewNotificationSenderMgtSvcInterfaceMock(s.T())
	s.handler = newServerConfigHandler(s.senders)
}

func (s *ServerConfigTestSuite) TestDecode() {
	value, err := s.handler.Decode(nil)
	s.Require().NoError(err)
	s.Equal(ServerConfig{}, value)

	value, err = s.handler.Decode(json.RawMessage(`{"defaultEmailSenderId":"sender-1"}`))
	s.Require().NoError(err)
	s.Equal(ServerConfig{DefaultEmailSenderID: "sender-1"}, value)

	_, err = s.handler.Decode(json.RawMessage(`not json`))
	s.Error(err)
}

func (s *ServerConfigTestSuite) TestDecodeRejectsAnUnknownField() {
	_, err := s.handler.Decode(json.RawMessage(`{"defaultEmailSenderId":"x","extra":1}`))
	s.Require().Error(err)
	s.Contains(err.Error(), `unknown field "extra"`)
}

// The declarative load validates with no layers, so the lookup must resolve the sender there too.
func (s *ServerConfigTestSuite) TestValidateLooksUpTheNamedSender() {
	cases := []struct {
		name     string
		senderID string
		lookedUp string
		sender   *common.NotificationSenderDTO
		svcErr   *tidcommon.ServiceError
		want     error
	}{
		{name: "an email sender", senderID: "sender-1", lookedUp: "sender-1",
			sender: &common.NotificationSenderDTO{ID: "sender-1", Type: common.NotificationSenderTypeEmail}},
		{name: "a padded ID", senderID: " sender-1 ", lookedUp: "sender-1",
			sender: &common.NotificationSenderDTO{ID: "sender-1", Type: common.NotificationSenderTypeEmail}},
		{name: "an unknown sender", senderID: "missing", lookedUp: "missing",
			svcErr: &ErrorSenderNotFound, want: errUnknownDefaultEmailSender},
		{name: "a failed lookup", senderID: "sender-1", lookedUp: "sender-1",
			svcErr: &tidcommon.InternalServerError, want: errDefaultSenderLookupFailed},
		{name: "a message sender", senderID: "sms-1", lookedUp: "sms-1",
			sender: &common.NotificationSenderDTO{ID: "sms-1", Type: common.NotificationSenderTypeMessage},
			want:   errDefaultSenderNotEmail},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			senders := NewNotificationSenderMgtSvcInterfaceMock(s.T())
			senders.EXPECT().GetSender(mock.Anything, tc.lookedUp).Return(tc.sender, tc.svcErr).Once()

			err := newServerConfigHandler(senders).Validate(ServerConfig{DefaultEmailSenderID: tc.senderID}, nil, nil)

			if tc.want == nil {
				s.NoError(err)
				return
			}
			s.ErrorIs(err, tc.want)
		})
	}
}

func (s *ServerConfigTestSuite) TestValidateAcceptsClearingTheSender() {
	s.NoError(s.handler.Validate(ServerConfig{}, ServerConfig{}, ServerConfig{}))
}

// A declarative sender locks the section.
func (s *ServerConfigTestSuite) TestValidateRejectsAWriteOverADeclarativeSender() {
	s.ErrorIs(s.handler.Validate(ServerConfig{DefaultEmailSenderID: "sender-2"},
		ServerConfig{DefaultEmailSenderID: "sender-1"}, ServerConfig{}), errDeclarativeSenderLocked)
}

func (s *ServerConfigTestSuite) TestMerge() {
	declarative := ServerConfig{DefaultEmailSenderID: "file"}
	writable := ServerConfig{DefaultEmailSenderID: "db"}

	s.Equal(declarative, s.handler.Merge(declarative, writable))
	s.Equal(writable, s.handler.Merge(ServerConfig{}, writable))
	s.Equal(ServerConfig{}, s.handler.Merge(nil, nil))
}
