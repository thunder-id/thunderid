/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package notification_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

// NotificationSenderExporterTestSuite tests the notificationSenderExporter.
type NotificationSenderExporterTestSuite struct {
	suite.Suite
	mockService *notificationmock.NotificationSenderMgtSvcInterfaceMock
	exporter    declarativeresource.ResourceExporter
	logger      *log.Logger
}

func TestNotificationSenderExporterTestSuite(t *testing.T) {
	suite.Run(t, new(NotificationSenderExporterTestSuite))
}

func (s *NotificationSenderExporterTestSuite) SetupTest() {
	s.mockService = notificationmock.NewNotificationSenderMgtSvcInterfaceMock(s.T())
	s.exporter = notification.NewNotificationSenderExporterForTest(s.mockService)
	s.logger = log.GetLogger()
}

func (s *NotificationSenderExporterTestSuite) TestNewNotificationSenderExporter() {
	assert.NotNil(s.T(), s.exporter)
}

func (s *NotificationSenderExporterTestSuite) TestGetResourceType() {
	assert.Equal(s.T(), "notification_sender", s.exporter.GetResourceType())
}

func (s *NotificationSenderExporterTestSuite) TestGetParameterizerType() {
	assert.Equal(s.T(), "NotificationSender", s.exporter.GetParameterizerType())
}

func (s *NotificationSenderExporterTestSuite) TestGetAllResourceIDs_Success() {
	expectedSenders := []common.NotificationSenderDTO{
		{ID: "sender1", Name: "Sender 1"},
		{ID: "sender2", Name: "Sender 2"},
	}

	s.mockService.EXPECT().ListSenders(mock.Anything).Return(expectedSenders, nil)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 2)
	assert.Equal(s.T(), "sender1", ids[0])
	assert.Equal(s.T(), "sender2", ids[1])
}

func (s *NotificationSenderExporterTestSuite) TestGetAllResourceIDs_Error() {
	expectedError := &serviceerror.ServiceError{
		Code:  "ERR_CODE",
		Error: i18ncore.I18nMessage{DefaultValue: "test error"},
	}

	s.mockService.EXPECT().ListSenders(mock.Anything).Return(nil, expectedError)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), ids)
	assert.Equal(s.T(), expectedError, err)
}

func (s *NotificationSenderExporterTestSuite) TestGetAllResourceIDs_EmptyList() {
	expectedSenders := []common.NotificationSenderDTO{}

	s.mockService.EXPECT().ListSenders(mock.Anything).Return(expectedSenders, nil)

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	assert.Nil(s.T(), err)
	assert.Len(s.T(), ids, 0)
}

func (s *NotificationSenderExporterTestSuite) TestGetResourceByID_Success() {
	expectedSender := &common.NotificationSenderDTO{
		ID:   "sender1",
		Name: "Test Sender",
	}

	s.mockService.EXPECT().GetSender(mock.Anything, "sender1").Return(expectedSender, nil)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "sender1")

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test Sender", name)
	assert.Equal(s.T(), expectedSender, resource)
}

func (s *NotificationSenderExporterTestSuite) TestGetResourceByID_Error() {
	expectedError := &serviceerror.ServiceError{
		Code:  "ERR_CODE",
		Error: i18ncore.I18nMessage{DefaultValue: "test error"},
	}

	s.mockService.EXPECT().GetSender(mock.Anything, "sender1").Return(nil, expectedError)

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "sender1")

	assert.Nil(s.T(), resource)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), expectedError, err)
}

func (s *NotificationSenderExporterTestSuite) TestValidateResource_Success() {
	prop, _ := cmodels.NewProperty("key1", "value1", false)
	sender := &common.NotificationSenderDTO{
		ID:         "sender1",
		Name:       "Valid Sender",
		Properties: []cmodels.Property{*prop},
	}

	name, err := s.exporter.ValidateResource(sender, "sender1", s.logger)

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Valid Sender", name)
}

func (s *NotificationSenderExporterTestSuite) TestValidateResource_InvalidType() {
	invalidResource := "not a sender"

	name, err := s.exporter.ValidateResource(invalidResource, "sender1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "notification_sender", err.ResourceType)
	assert.Equal(s.T(), "sender1", err.ResourceID)
	assert.Equal(s.T(), "INVALID_TYPE", err.Code)
}

func (s *NotificationSenderExporterTestSuite) TestValidateResource_EmptyName() {
	sender := &common.NotificationSenderDTO{
		ID:   "sender1",
		Name: "",
	}

	name, err := s.exporter.ValidateResource(sender, "sender1", s.logger)

	assert.Empty(s.T(), name)
	assert.NotNil(s.T(), err)
	assert.Equal(s.T(), "notification_sender", err.ResourceType)
	assert.Equal(s.T(), "sender1", err.ResourceID)
	assert.Equal(s.T(), "SENDER_VALIDATION_ERROR", err.Code)
	assert.Contains(s.T(), err.Error, "name is empty")
}

func (s *NotificationSenderExporterTestSuite) TestValidateResource_NoProperties() {
	sender := &common.NotificationSenderDTO{
		ID:         "sender1",
		Name:       "Test Sender",
		Properties: []cmodels.Property{},
	}

	name, err := s.exporter.ValidateResource(sender, "sender1", s.logger)

	// Should still succeed but log a warning
	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "Test Sender", name)
}
