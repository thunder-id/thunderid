/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/notification/messagemock"
)

type NotificationSenderServiceTestSuite struct {
	suite.Suite
	mockSenderMgtSvc   *NotificationSenderMgtSvcInterfaceMock
	mockClientProvider *notificationClientProviderInterfaceMock
	service            *notificationSenderService
}

func TestNotificationSenderServiceTestSuite(t *testing.T) {
	suite.Run(t, new(NotificationSenderServiceTestSuite))
}

func (suite *NotificationSenderServiceTestSuite) SetupSuite() {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
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
	suite.mockClientProvider = newNotificationClientProviderInterfaceMock(suite.T())
	suite.service = &notificationSenderService{
		senderMgtService: suite.mockSenderMgtSvc,
		clientProvider:   suite.mockClientProvider,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationSenderService")),
	}
}

func (suite *NotificationSenderServiceTestSuite) getValidSender() *common.NotificationSenderDTO {
	return &common.NotificationSenderDTO{
		ID:       "sender-001",
		Name:     "Test SMS Sender",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.MessageProviderTypeTwilio,
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

	mm := messagemock.NewNotificationClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelTypeSMS).Return(true).Once()
	mm.EXPECT().Send(common.ChannelTypeSMS, mock.Anything).Return(nil).Once()
	suite.mockClientProvider.EXPECT().GetClient(mock.Anything).Return(mm, nil).Once()

	err := suite.service.Send(context.Background(), common.ChannelTypeSMS, "sender-001",
		common.NotificationData{Recipient: "+94714627887", Body: "Test message"})
	suite.Nil(err)
}

func (suite *NotificationSenderServiceTestSuite) TestSendSMS_GetSenderError() {
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "sender-001").
		Return(nil, &ErrorSenderNotFound).Once()

	err := suite.service.Send(context.Background(), common.ChannelTypeSMS, "sender-001",
		common.NotificationData{Recipient: "+94714627887", Body: "Test message"})
	suite.NotNil(err)
	suite.Equal(ErrorSenderNotFound.Code, err.Code)
}

func (suite *NotificationSenderServiceTestSuite) TestSendSMS_WrongSenderType() {
	sender := &common.NotificationSenderDTO{
		ID:   "email-sender-001",
		Type: "EMAIL",
	}
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "email-sender-001").Return(sender, nil).Once()

	err := suite.service.Send(context.Background(), common.ChannelTypeSMS, "email-sender-001",
		common.NotificationData{Recipient: "+94714627887", Body: "Test message"})
	suite.NotNil(err)
	suite.Equal(ErrorRequestedSenderIsNotOfExpectedType.Code, err.Code)
}

func (suite *NotificationSenderServiceTestSuite) TestSendSMS_GetClientError() {
	sender := suite.getValidSender()
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "sender-001").Return(sender, nil).Once()
	suite.mockClientProvider.EXPECT().GetClient(mock.Anything).
		Return(nil, &serviceerror.InternalServerError).Once()

	err := suite.service.Send(context.Background(), common.ChannelTypeSMS, "sender-001",
		common.NotificationData{Recipient: "+94714627887", Body: "Test message"})
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *NotificationSenderServiceTestSuite) TestSendSMS_UnsupportedChannel() {
	sender := suite.getValidSender()
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "sender-001").Return(sender, nil).Once()

	mm := messagemock.NewNotificationClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelType("email")).Return(false).Once()
	suite.mockClientProvider.EXPECT().GetClient(mock.Anything).Return(mm, nil).Once()

	err := suite.service.Send(context.Background(), common.ChannelType("email"), "sender-001",
		common.NotificationData{Recipient: "user@example.com", Body: "Test message"})
	suite.NotNil(err)
	suite.Equal(ErrorUnsupportedChannel.Code, err.Code)
}

func (suite *NotificationSenderServiceTestSuite) TestSendSMS_ClientSendError() {
	sender := suite.getValidSender()
	suite.mockSenderMgtSvc.On("GetSender", mock.Anything, "sender-001").Return(sender, nil).Once()

	mm := messagemock.NewNotificationClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelTypeSMS).Return(true).Once()
	mm.EXPECT().Send(common.ChannelTypeSMS, mock.Anything).Return(errors.New("network error")).Once()
	suite.mockClientProvider.EXPECT().GetClient(mock.Anything).Return(mm, nil).Once()

	err := suite.service.Send(context.Background(), common.ChannelTypeSMS, "sender-001",
		common.NotificationData{Recipient: "+94714627887", Body: "Test message"})
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}
