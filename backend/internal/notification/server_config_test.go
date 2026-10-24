// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	notificationconfig "github.com/thunder-id/thunderid/internal/notification/config"
)

type ServerConfigTestSuite struct {
	suite.Suite
	handler *serverConfigHandler
}

func TestServerConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ServerConfigTestSuite))
}

func (s *ServerConfigTestSuite) SetupTest() {
	s.handler = newServerConfigHandler()
}

func (s *ServerConfigTestSuite) TestDecode() {
	value, err := s.handler.Decode(nil)
	s.Require().NoError(err)
	s.Equal(notificationconfig.NotificationSectionConfig{}, value)

	value, err = s.handler.Decode(json.RawMessage(`{"defaultEmailSenderId":"sender-1"}`))
	s.Require().NoError(err)
	s.Equal(notificationconfig.NotificationSectionConfig{DefaultEmailSenderID: "sender-1"}, value)

	_, err = s.handler.Decode(json.RawMessage(`not json`))
	s.Error(err)
}

func (s *ServerConfigTestSuite) TestDecodeRejectsAnUnknownField() {
	_, err := s.handler.Decode(json.RawMessage(`{"defaultEmailSenderId":"x","extra":1}`))
	s.Require().Error(err)
	s.Contains(err.Error(), `unknown field "extra"`)
}

func (s *ServerConfigTestSuite) TestValidateDoesNotResolveTheSender() {
	s.NoError(s.handler.Validate(
		notificationconfig.NotificationSectionConfig{DefaultEmailSenderID: "missing"}, nil, nil))
}

func (s *ServerConfigTestSuite) TestValidateAcceptsClearingTheSender() {
	empty := notificationconfig.NotificationSectionConfig{}
	s.NoError(s.handler.Validate(empty, empty, empty))
}

// A declarative sender locks the section.
func (s *ServerConfigTestSuite) TestValidateRejectsAWriteOverADeclarativeSender() {
	s.ErrorIs(s.handler.Validate(notificationconfig.NotificationSectionConfig{DefaultEmailSenderID: "sender-2"},
		notificationconfig.NotificationSectionConfig{DefaultEmailSenderID: "sender-1"},
		notificationconfig.NotificationSectionConfig{}), errDeclarativeSenderLocked)
}

func (s *ServerConfigTestSuite) TestMerge() {
	declarative := notificationconfig.NotificationSectionConfig{DefaultEmailSenderID: "file"}
	writable := notificationconfig.NotificationSectionConfig{DefaultEmailSenderID: "db"}

	s.Equal(declarative, s.handler.Merge(declarative, writable))
	s.Equal(writable, s.handler.Merge(notificationconfig.NotificationSectionConfig{}, writable))
	s.Equal(notificationconfig.NotificationSectionConfig{}, s.handler.Merge(nil, nil))
}
