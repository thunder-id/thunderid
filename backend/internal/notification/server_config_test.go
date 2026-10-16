// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
)

type ServerConfigTestSuite struct {
	suite.Suite
	senders *NotificationSenderMgtSvcInterfaceMock
	handler *ServerConfigHandler
}

func TestServerConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ServerConfigTestSuite))
}

func (s *ServerConfigTestSuite) SetupTest() {
	s.senders = NewNotificationSenderMgtSvcInterfaceMock(s.T())
	s.handler = NewServerConfigHandler(s.senders)
}

func (s *ServerConfigTestSuite) TestDecode() {
	value, err := s.handler.Decode(nil)
	s.Require().NoError(err)
	s.Equal(serverConfig{}, value)

	value, err = s.handler.Decode(json.RawMessage(`{"defaultEmailSenderId":"sender-1"}`))
	s.Require().NoError(err)
	s.Equal(serverConfig{DefaultEmailSenderID: "sender-1"}, value)

	_, err = s.handler.Decode(json.RawMessage(`not json`))
	s.Error(err)
}

func (s *ServerConfigTestSuite) TestValidateAcceptsAnEmailSender() {
	s.senders.EXPECT().GetSender(mock.Anything, "sender-1").
		Return(&common.NotificationSenderDTO{ID: "sender-1", Type: common.NotificationSenderTypeEmail}, nil)

	s.NoError(s.handler.Validate(serverConfig{DefaultEmailSenderID: "sender-1"}, serverConfig{}, serverConfig{}))
}

func (s *ServerConfigTestSuite) TestValidateAcceptsClearingTheSender() {
	s.NoError(s.handler.Validate(serverConfig{}, serverConfig{}, serverConfig{}))
}

func (s *ServerConfigTestSuite) TestValidateRejectsAnUnknownSender() {
	s.senders.EXPECT().GetSender(mock.Anything, "missing").Return(nil, &ErrorSenderNotFound)

	s.ErrorIs(s.handler.Validate(serverConfig{DefaultEmailSenderID: "missing"}, nil, nil),
		errUnknownDefaultEmailSender)
}

func (s *ServerConfigTestSuite) TestValidateRejectsAFailedLookup() {
	s.senders.EXPECT().GetSender(mock.Anything, "sender-1").Return(nil, &tidcommon.InternalServerError)

	s.ErrorIs(s.handler.Validate(serverConfig{DefaultEmailSenderID: "sender-1"}, nil, nil),
		errDefaultSenderLookupFailed)
}

func (s *ServerConfigTestSuite) TestValidateRejectsAMessageSender() {
	s.senders.EXPECT().GetSender(mock.Anything, "sms-1").
		Return(&common.NotificationSenderDTO{ID: "sms-1", Type: common.NotificationSenderTypeMessage}, nil)

	s.ErrorIs(s.handler.Validate(serverConfig{DefaultEmailSenderID: "sms-1"}, nil, nil),
		errDefaultSenderNotEmail)
}

// A declarative sender locks the section.
func (s *ServerConfigTestSuite) TestValidateRejectsAWriteOverADeclarativeSender() {
	s.ErrorIs(s.handler.Validate(serverConfig{DefaultEmailSenderID: "sender-2"},
		serverConfig{DefaultEmailSenderID: "sender-1"}, serverConfig{}), errDeclarativeSenderLocked)
}

func (s *ServerConfigTestSuite) TestMerge() {
	declarative := serverConfig{DefaultEmailSenderID: "file"}
	writable := serverConfig{DefaultEmailSenderID: "db"}

	s.Equal(declarative, s.handler.Merge(declarative, writable))
	s.Equal(writable, s.handler.Merge(serverConfig{}, writable))
	s.Equal(serverConfig{}, s.handler.Merge(nil, nil))
}

func (s *ServerConfigTestSuite) TestNewServerConfigHandlerRequiresASenderService() {
	defer func() {
		if recover() == nil {
			s.T().Fatal("expected a panic")
		}
	}()
	NewServerConfigHandler(nil)
}

func (s *ServerConfigTestSuite) TestDefaultEmailSenderID() {
	s.Empty(NewDefaultEmailSender(nil).DefaultEmailSenderID(context.Background()),
		"without a reader no sender is named")

	reader := serverconfigmock.NewServerConfigServiceMock(s.T())
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameNotification)).
		Return(serverConfig{DefaultEmailSenderID: " sender-1 "}, nil).Once()
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameNotification)).
		Return(nil, &tidcommon.InternalServerError).Once()
	sender := NewDefaultEmailSender(reader)

	s.Equal("sender-1", sender.DefaultEmailSenderID(context.Background()))
	s.Empty(sender.DefaultEmailSenderID(context.Background()), "a failed read names no sender")
}
