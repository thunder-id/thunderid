// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"context"
	"errors"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/notification/clientmock"
)

type NotificationSenderServiceTestSuite struct {
	suite.Suite
	mockSenderMgtSvc  *NotificationSenderMgtSvcInterfaceMock
	mockClientFactory *clientmock.ClientFactoryInterfaceMock
	service           *notificationSenderService
}

func TestNotificationSenderServiceTestSuite(t *testing.T) {
	suite.Run(t, new(NotificationSenderServiceTestSuite))
}

func (suite *NotificationSenderServiceTestSuite) SetupSuite() {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
	}
	if err := config.InitializeServerRuntime("", testConfig); err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *NotificationSenderServiceTestSuite) SetupTest() {
	suite.mockSenderMgtSvc = NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	suite.mockClientFactory = clientmock.NewClientFactoryInterfaceMock(suite.T())
	suite.service = &notificationSenderService{
		senderMgtService: suite.mockSenderMgtSvc,
		clientFactory:    suite.mockClientFactory,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationSenderService")),
	}
}

func (suite *NotificationSenderServiceTestSuite) getValidSender() *common.NotificationSenderDTO {
	return &common.NotificationSenderDTO{
		ID:       "sender-001",
		Name:     "Test SMS Sender",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.NotificationProviderTypeTwilio,
		Properties: []cmodels.Property{
			createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
			createTestProperty("auth_token", "test-token", true),
			createTestProperty("sender_id", "+15551234567", false),
		},
	}
}

func (suite *NotificationSenderServiceTestSuite) TestSendSMS_Success() {
	sender := suite.getValidSender()
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "sender-001").Return(sender, nil).Once()

	mm := clientmock.NewMessageClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelTypeSMS).Return(true).Once()
	mm.EXPECT().Send(mock.Anything, common.ChannelTypeSMS, mock.Anything).Return(nil).Once()
	suite.mockClientFactory.EXPECT().GetClient(mock.Anything, mock.Anything).Return(mm, nil).Once()

	err := suite.service.SendMessage(context.Background(), common.ChannelTypeSMS, "sender-001",
		common.MessageData{Recipient: "+94714627887", Body: "Test message"})
	suite.Nil(err)
}

func (suite *NotificationSenderServiceTestSuite) TestSendSMS_GetSenderError() {
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "sender-001").
		Return(nil, &ErrorSenderNotFound).Once()

	err := suite.service.SendMessage(context.Background(), common.ChannelTypeSMS, "sender-001",
		common.MessageData{Recipient: "+94714627887", Body: "Test message"})
	suite.NotNil(err)
	suite.Equal(ErrorSenderNotFound.Code, err.Code)
}

func (suite *NotificationSenderServiceTestSuite) TestSendMessage_WrongSenderType() {
	sender := &common.NotificationSenderDTO{
		ID:       "email-sender-001",
		Type:     common.NotificationSenderTypeEmail,
		Provider: common.NotificationProviderTypeSMTP,
	}
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "email-sender-001").Return(sender, nil).Once()

	emailClient := clientmock.NewEmailClientInterfaceMock(suite.T())
	suite.mockClientFactory.EXPECT().GetClient(mock.Anything, mock.Anything).Return(emailClient, nil).Once()

	err := suite.service.SendMessage(context.Background(), common.ChannelTypeSMS, "email-sender-001",
		common.MessageData{Recipient: "+94714627887", Body: "Test message"})
	suite.NotNil(err)
	suite.Equal(ErrorRequestedSenderIsNotOfExpectedType.Code, err.Code)
}

func (suite *NotificationSenderServiceTestSuite) TestSendSMS_GetClientError() {
	sender := suite.getValidSender()
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "sender-001").Return(sender, nil).Once()
	suite.mockClientFactory.EXPECT().GetClient(mock.Anything, mock.Anything).
		Return(nil, &tidcommon.InternalServerError).Once()

	err := suite.service.SendMessage(context.Background(), common.ChannelTypeSMS, "sender-001",
		common.MessageData{Recipient: "+94714627887", Body: "Test message"})
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *NotificationSenderServiceTestSuite) TestSendSMS_UnsupportedChannel() {
	sender := suite.getValidSender()
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "sender-001").Return(sender, nil).Once()

	mm := clientmock.NewMessageClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelType("email")).Return(false).Once()
	suite.mockClientFactory.EXPECT().GetClient(mock.Anything, mock.Anything).Return(mm, nil).Once()

	err := suite.service.SendMessage(context.Background(), common.ChannelType("email"), "sender-001",
		common.MessageData{Recipient: "user@example.com", Body: "Test message"})
	suite.NotNil(err)
	suite.Equal(ErrorUnsupportedChannel.Code, err.Code)
}

func (suite *NotificationSenderServiceTestSuite) TestSendSMS_ClientSendError() {
	sender := suite.getValidSender()
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "sender-001").Return(sender, nil).Once()

	mm := clientmock.NewMessageClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelTypeSMS).Return(true).Once()
	mm.EXPECT().Send(mock.Anything, common.ChannelTypeSMS, mock.Anything).Return(errors.New("network error")).Once()
	suite.mockClientFactory.EXPECT().GetClient(mock.Anything, mock.Anything).Return(mm, nil).Once()

	err := suite.service.SendMessage(context.Background(), common.ChannelTypeSMS, "sender-001",
		common.MessageData{Recipient: "+94714627887", Body: "Test message"})
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *NotificationSenderServiceTestSuite) getValidEmailSender() *common.NotificationSenderDTO {
	return &common.NotificationSenderDTO{
		ID:       "email-sender-001",
		Name:     "Test Email Sender",
		Type:     common.NotificationSenderTypeEmail,
		Provider: common.NotificationProviderTypeSMTP,
		Properties: []cmodels.Property{
			createTestProperty(common.SMTPPropKeyHost, "smtp.example.com", false),
			createTestProperty(common.SMTPPropKeyPort, "587", false),
			createTestProperty(common.SMTPPropKeyFromAddress, "noreply@example.com", false),
		},
	}
}

func (suite *NotificationSenderServiceTestSuite) emailData() common.EmailData {
	return common.EmailData{To: []string{"user@example.com"}, Subject: "Hi", Body: "Body"}
}

func (suite *NotificationSenderServiceTestSuite) TestSendEmail_Success() {
	sender := suite.getValidEmailSender()
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "email-sender-001").Return(sender, nil).Once()

	emailClient := clientmock.NewEmailClientInterfaceMock(suite.T())
	emailClient.EXPECT().Send(mock.Anything, mock.Anything).Return(nil).Once()
	suite.mockClientFactory.EXPECT().GetClient(mock.Anything, mock.Anything).Return(emailClient, nil).Once()

	err := suite.service.SendEmail(context.Background(), "email-sender-001", suite.emailData())
	suite.Nil(err)
}

// A provider is always named explicitly. Without one there is nothing to fall back to, so the
// send is rejected before any lookup rather than dispatching through an arbitrary sender.
func (suite *NotificationSenderServiceTestSuite) TestSendEmail_EmptySenderIDIsRejected() {
	for _, senderID := range []string{"", "  "} {
		err := suite.service.SendEmail(context.Background(), senderID, suite.emailData())
		suite.Require().NotNil(err)
		suite.Equal(ErrorEmailSenderNotSpecified.Code, err.Code)
	}

	suite.mockSenderMgtSvc.AssertNotCalled(suite.T(), "GetSender", mock.Anything, mock.Anything)
	suite.mockSenderMgtSvc.AssertNotCalled(suite.T(), "GetSenderByName", mock.Anything, mock.Anything)
}

func (suite *NotificationSenderServiceTestSuite) TestSendEmail_WrongSenderType() {
	sender := suite.getValidSender()
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "sender-001").Return(sender, nil).Once()

	messageClient := clientmock.NewMessageClientInterfaceMock(suite.T())
	suite.mockClientFactory.EXPECT().GetClient(mock.Anything, mock.Anything).Return(messageClient, nil).Once()

	err := suite.service.SendEmail(context.Background(), "sender-001", suite.emailData())
	suite.NotNil(err)
	suite.Equal(ErrorRequestedSenderIsNotOfExpectedType.Code, err.Code)
}

func (suite *NotificationSenderServiceTestSuite) TestSendEmail_ClientSendError() {
	sender := suite.getValidEmailSender()
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "email-sender-001").Return(sender, nil).Once()

	emailClient := clientmock.NewEmailClientInterfaceMock(suite.T())
	emailClient.EXPECT().Send(mock.Anything, mock.Anything).Return(errors.New("smtp failure")).Once()
	suite.mockClientFactory.EXPECT().GetClient(mock.Anything, mock.Anything).Return(emailClient, nil).Once()

	err := suite.service.SendEmail(context.Background(), "email-sender-001", suite.emailData())
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}
