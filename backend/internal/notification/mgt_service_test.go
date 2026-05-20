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

package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

const (
	testSenderID          = "test-id"
	testSenderOldName     = "Old Name"
	testSenderUpdatedName = "Updated Name"
)

type NotificationSenderMgtServiceTestSuite struct {
	suite.Suite
	mockStore *notificationStoreInterfaceMock
	service   *notificationSenderMgtService
}

func TestNotificationSenderMgtServiceTestSuite(t *testing.T) {
	suite.Run(t, new(NotificationSenderMgtServiceTestSuite))
}

func (suite *NotificationSenderMgtServiceTestSuite) SetupSuite() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *NotificationSenderMgtServiceTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
}

func (suite *NotificationSenderMgtServiceTestSuite) SetupTest() {
	suite.mockStore = newNotificationStoreInterfaceMock(suite.T())
	suite.service = &notificationSenderMgtService{
		notificationStore: suite.mockStore,
		transactioner:     &fakeTransactioner{},
	}
}

func (suite *NotificationSenderMgtServiceTestSuite) TestCreateSender() {
	sender := suite.getValidTwilioSender()

	suite.mockStore.EXPECT().getSenderByName(mock.Anything, sender.Name).Return(nil, nil).Once()
	suite.mockStore.EXPECT().createSender(mock.Anything, mock.MatchedBy(func(s common.NotificationSenderDTO) bool {
		return s.Name == sender.Name && s.ID != ""
	})).Return(nil).Once()

	result, err := suite.service.CreateSender(context.Background(), sender)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(sender.Name, result.Name)
	suite.NotEmpty(result.ID)
}

func (suite *NotificationSenderMgtServiceTestSuite) TestCreateSender_WithFailures() {
	cases := []struct {
		name            string
		inputMod        func(common.NotificationSenderDTO) common.NotificationSenderDTO
		setupMock       func(*notificationStoreInterfaceMock, common.NotificationSenderDTO)
		expectedErrCode string
	}{
		{
			name: "DuplicateName",
			inputMod: func(s common.NotificationSenderDTO) common.NotificationSenderDTO {
				return s
			},
			setupMock: func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {
				existing := s
				existing.ID = "existing-id"
				m.EXPECT().getSenderByName(mock.Anything, s.Name).Return(&existing, nil).Once()
			},
			expectedErrCode: ErrorDuplicateSenderName.Code,
		},
		{
			name: "StoreErrorOnNameCheck",
			inputMod: func(s common.NotificationSenderDTO) common.NotificationSenderDTO {
				return s
			},
			setupMock: func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {
				m.EXPECT().getSenderByName(mock.Anything, s.Name).Return(nil, errors.New("database error")).Once()
			},
			expectedErrCode: serviceerror.InternalServerError.Code,
		},
		{
			name: "StoreErrorOnCreate",
			inputMod: func(s common.NotificationSenderDTO) common.NotificationSenderDTO {
				return s
			},
			setupMock: func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {
				m.EXPECT().getSenderByName(mock.Anything, s.Name).Return(nil, nil).Once()
				m.EXPECT().createSender(mock.Anything, mock.Anything).Return(errors.New("database error")).Once()
			},
			expectedErrCode: serviceerror.InternalServerError.Code,
		},
		{
			name: "InvalidName",
			inputMod: func(s common.NotificationSenderDTO) common.NotificationSenderDTO {
				s.Name = ""
				return s
			},
			setupMock:       func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {},
			expectedErrCode: ErrorInvalidSenderName.Code,
		},
		{
			name: "InvalidProvider",
			inputMod: func(s common.NotificationSenderDTO) common.NotificationSenderDTO {
				s.Provider = "bad"
				return s
			},
			setupMock:       func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {},
			expectedErrCode: ErrorInvalidProvider.Code,
		},
	}

	for _, tc := range cases {
		suite.T().Run(tc.name, func(t *testing.T) {
			mockStore := newNotificationStoreInterfaceMock(t)
			svc := &notificationSenderMgtService{
				notificationStore: mockStore,
				transactioner:     &fakeTransactioner{},
			}

			sender := suite.getValidTwilioSender()
			sender = tc.inputMod(sender)

			if tc.setupMock != nil {
				tc.setupMock(mockStore, sender)
			}

			result, err := svc.CreateSender(context.Background(), sender)
			require := require.New(t)
			require.Nil(result)
			require.NotNil(err)
			require.Equal(tc.expectedErrCode, err.Code)
			mockStore.AssertExpectations(t)
		})
	}
}

func (suite *NotificationSenderMgtServiceTestSuite) TestListSenders() {
	senders := []common.NotificationSenderDTO{
		suite.getValidTwilioSender(),
		suite.getValidVonageSender(),
	}
	senders[0].ID = "id1"
	senders[1].ID = "id2"

	suite.mockStore.EXPECT().listSenders(mock.Anything).Return(senders, nil).Once()

	result, err := suite.service.ListSenders(context.Background())
	suite.Nil(err)
	suite.NotNil(result)
	suite.Len(result, 2)
}

func (suite *NotificationSenderMgtServiceTestSuite) TestListSenders_EmptyList() {
	suite.mockStore.EXPECT().listSenders(mock.Anything).Return([]common.NotificationSenderDTO{}, nil).Once()

	result, err := suite.service.ListSenders(context.Background())
	suite.Nil(err)
	suite.NotNil(result)
	suite.Len(result, 0)
}

func (suite *NotificationSenderMgtServiceTestSuite) TestListSenders_StoreError() {
	suite.mockStore.EXPECT().listSenders(mock.Anything).Return(nil, errors.New("database error")).Once()

	result, err := suite.service.ListSenders(context.Background())
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *NotificationSenderMgtServiceTestSuite) TestGetSender() {
	sender := suite.getValidTwilioSender()
	sender.ID = testSenderID
	suite.mockStore.EXPECT().getSenderByID(mock.Anything, testSenderID).Return(&sender, nil).Once()

	result, err := suite.service.GetSender(context.Background(), testSenderID)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testSenderID, result.ID)
}

func (suite *NotificationSenderMgtServiceTestSuite) TestGetSender_NotFound() {
	suite.mockStore.EXPECT().getSenderByID(mock.Anything, testSenderID).Return(nil, nil).Once()

	result, err := suite.service.GetSender(context.Background(), testSenderID)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorSenderNotFound.Code, err.Code)
}

func (suite *NotificationSenderMgtServiceTestSuite) TestGetSender_EmptyID() {
	result, err := suite.service.GetSender(context.Background(), "")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSenderID.Code, err.Code)
}

func (suite *NotificationSenderMgtServiceTestSuite) TestGetSender_StoreError() {
	suite.mockStore.EXPECT().getSenderByID(mock.Anything, testSenderID).Return(nil, errors.New("database error")).Once()

	result, err := suite.service.GetSender(context.Background(), testSenderID)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

// GetSenderByName Tests
func (suite *NotificationSenderMgtServiceTestSuite) TestGetSenderByName() {
	cases := []struct {
		name     string
		setup    func(*notificationStoreInterfaceMock)
		arg      string
		wantName string
	}{
		{
			name: "SenderFound",
			setup: func(m *notificationStoreInterfaceMock) {
				sender := suite.getValidTwilioSender()
				sender.ID = testSenderID
				m.EXPECT().getSenderByName(mock.Anything, "Test Twilio Sender").Return(&sender, nil).Once()
			},
			arg:      "Test Twilio Sender",
			wantName: "Test Twilio Sender",
		},
	}

	for _, tc := range cases {
		suite.T().Run(tc.name, func(t *testing.T) {
			mockStore := newNotificationStoreInterfaceMock(t)
			svc := &notificationSenderMgtService{
				notificationStore: mockStore,
				transactioner:     &fakeTransactioner{},
			}

			if tc.setup != nil {
				tc.setup(mockStore)
			}

			result, err := svc.GetSenderByName(context.Background(), tc.arg)
			require := require.New(t)
			require.Nil(err)
			require.NotNil(result)
			require.Equal(tc.wantName, result.Name)
			mockStore.AssertExpectations(t)
		})
	}
}

func (suite *NotificationSenderMgtServiceTestSuite) TestGetSenderByName_NotFound() {
	mockStore := newNotificationStoreInterfaceMock(suite.T())
	svc := &notificationSenderMgtService{
		notificationStore: mockStore,
		transactioner:     &fakeTransactioner{},
	}
	mockStore.EXPECT().getSenderByName(mock.Anything, "NonExistent").Return(nil, nil).Once()

	result, err := svc.GetSenderByName(context.Background(), "NonExistent")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorSenderNotFound.Code, err.Code)
}

func (suite *NotificationSenderMgtServiceTestSuite) TestGetSenderByName_WithFailure() {
	cases := []struct {
		name            string
		arg             string
		setup           func(*notificationStoreInterfaceMock)
		expectedErrCode string
	}{
		{
			name:            "EmptyName",
			arg:             "",
			setup:           func(m *notificationStoreInterfaceMock) {},
			expectedErrCode: ErrorInvalidSenderName.Code,
		},
		{
			name: "StoreError",
			arg:  "Test",
			setup: func(m *notificationStoreInterfaceMock) {
				m.EXPECT().getSenderByName(mock.Anything, "Test").Return(nil, errors.New("database error")).Once()
			},
			expectedErrCode: serviceerror.InternalServerError.Code,
		},
	}

	for _, tc := range cases {
		suite.T().Run(tc.name, func(t *testing.T) {
			mockStore := newNotificationStoreInterfaceMock(t)
			svc := &notificationSenderMgtService{
				notificationStore: mockStore,
				transactioner:     &fakeTransactioner{},
			}

			if tc.setup != nil {
				tc.setup(mockStore)
			}

			result, err := svc.GetSenderByName(context.Background(), tc.arg)
			require := require.New(t)
			require.Nil(result)
			require.NotNil(err)
			require.Equal(tc.expectedErrCode, err.Code)
			mockStore.AssertExpectations(t)
		})
	}
}

func (suite *NotificationSenderMgtServiceTestSuite) TestUpdateSender() {
	cases := []struct {
		name      string
		setupMock func(*notificationStoreInterfaceMock, common.NotificationSenderDTO)
	}{
		{
			name: "NoNameChange",
			setupMock: func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {
				existing := s
				existing.ID = testSenderID
				m.EXPECT().getSenderByID(mock.Anything, testSenderID).Return(&existing, nil).Once()
				m.EXPECT().updateSender(mock.Anything, testSenderID, s).Return(nil).Once()
			},
		},
		{
			name: "NameChange",
			setupMock: func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {
				existing := s
				existing.ID = testSenderID
				existing.Name = testSenderOldName
				m.EXPECT().getSenderByID(mock.Anything, testSenderID).Return(&existing, nil).Once()
				m.EXPECT().getSenderByName(mock.Anything, testSenderUpdatedName).Return(nil, nil).Once()
				m.EXPECT().updateSender(mock.Anything, testSenderID, s).Return(nil).Once()
			},
		},
		{
			name: "NameChangeSameID",
			setupMock: func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {
				existing := s
				existing.ID = testSenderID
				existing.Name = testSenderOldName
				same := s
				same.ID = testSenderID
				m.EXPECT().getSenderByID(mock.Anything, testSenderID).Return(&existing, nil).Once()
				m.EXPECT().getSenderByName(mock.Anything, testSenderUpdatedName).Return(&same, nil).Once()
				m.EXPECT().updateSender(mock.Anything, testSenderID, s).Return(nil).Once()
			},
		},
	}

	for _, tc := range cases {
		suite.T().Run(tc.name, func(t *testing.T) {
			mockStore := newNotificationStoreInterfaceMock(t)
			svc := &notificationSenderMgtService{
				notificationStore: mockStore,
				transactioner:     &fakeTransactioner{},
			}

			sender := suite.getValidTwilioSender()
			if tc.name != "NoNameChange" {
				sender.Name = testSenderUpdatedName
			}

			tc.setupMock(mockStore, sender)

			result, err := svc.UpdateSender(context.Background(), testSenderID, sender)
			require := require.New(t)
			require.Nil(err)
			require.NotNil(result)
			mockStore.AssertExpectations(t)
		})
	}
}

func (suite *NotificationSenderMgtServiceTestSuite) TestUpdateSender_WithFailure() {
	cases := []struct {
		name            string
		inputMod        func(common.NotificationSenderDTO) common.NotificationSenderDTO
		setupMock       func(*notificationStoreInterfaceMock, common.NotificationSenderDTO)
		expectedErrCode string
	}{
		{
			name: "DuplicateName",
			inputMod: func(s common.NotificationSenderDTO) common.NotificationSenderDTO {
				s.Name = testSenderUpdatedName
				return s
			},
			setupMock: func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {
				existing := s
				existing.ID = testSenderID
				existing.Name = testSenderOldName
				another := s
				another.ID = "another-id"
				another.Name = testSenderUpdatedName
				m.EXPECT().getSenderByID(mock.Anything, testSenderID).Return(&existing, nil).Once()
				m.EXPECT().getSenderByName(mock.Anything, testSenderUpdatedName).Return(&another, nil).Once()
			},
			expectedErrCode: ErrorDuplicateSenderName.Code,
		},
		{
			name: "SenderNotFound",
			inputMod: func(s common.NotificationSenderDTO) common.NotificationSenderDTO {
				return s
			},
			setupMock: func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {
				m.EXPECT().getSenderByID(mock.Anything, testSenderID).Return(nil, nil).Once()
			},
			expectedErrCode: ErrorSenderNotFound.Code,
		},
		{
			name: "EmptyID",
			inputMod: func(s common.NotificationSenderDTO) common.NotificationSenderDTO {
				return s
			},
			setupMock:       func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {},
			expectedErrCode: ErrorInvalidSenderID.Code,
		},
		{
			name: "TypeChangeNotAllowed",
			inputMod: func(s common.NotificationSenderDTO) common.NotificationSenderDTO {
				s.Type = common.NotificationSenderTypeMessage
				return s
			},
			setupMock: func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {
				existing := s
				existing.ID = testSenderID
				existing.Type = common.NotificationSenderType("legacy-type")
				m.EXPECT().getSenderByID(mock.Anything, testSenderID).Return(&existing, nil).Once()
			},
			expectedErrCode: ErrorSenderTypeUpdateNotAllowed.Code,
		},
		{
			name: "StoreErrorOnUpdate",
			inputMod: func(s common.NotificationSenderDTO) common.NotificationSenderDTO {
				return s
			},
			setupMock: func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {
				existing := s
				existing.ID = testSenderID
				m.EXPECT().getSenderByID(mock.Anything, testSenderID).Return(&existing, nil).Once()
				m.EXPECT().updateSender(mock.Anything, testSenderID, s).Return(errors.New("database error")).Once()
			},
			expectedErrCode: serviceerror.InternalServerError.Code,
		},
		{
			name: "GetSenderByIDError",
			inputMod: func(s common.NotificationSenderDTO) common.NotificationSenderDTO {
				return s
			},
			setupMock: func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {
				m.EXPECT().getSenderByID(mock.Anything, testSenderID).Return(nil, errors.New("database error")).Once()
			},
			expectedErrCode: serviceerror.InternalServerError.Code,
		},
		{
			name: "GetSenderByNameError",
			inputMod: func(s common.NotificationSenderDTO) common.NotificationSenderDTO {
				s.Name = testSenderUpdatedName
				return s
			},
			setupMock: func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {
				existing := s
				existing.ID = testSenderID
				existing.Name = testSenderOldName
				m.EXPECT().getSenderByID(mock.Anything, testSenderID).Return(&existing, nil).Once()
				m.EXPECT().getSenderByName(mock.Anything, testSenderUpdatedName).
					Return(nil, errors.New("database error")).Once()
			},
			expectedErrCode: serviceerror.InternalServerError.Code,
		},
		{
			name: "InvalidValidation",
			inputMod: func(s common.NotificationSenderDTO) common.NotificationSenderDTO {
				s.Provider = "bad"
				return s
			},
			setupMock:       func(m *notificationStoreInterfaceMock, s common.NotificationSenderDTO) {},
			expectedErrCode: ErrorInvalidProvider.Code,
		},
	}

	for _, tc := range cases {
		suite.T().Run(tc.name, func(t *testing.T) {
			mockStore := newNotificationStoreInterfaceMock(t)
			svc := &notificationSenderMgtService{
				notificationStore: mockStore,
				transactioner:     &fakeTransactioner{},
			}

			sender := suite.getValidTwilioSender()
			sender = tc.inputMod(sender)

			if tc.name == "EmptyID" {
				result, err := svc.UpdateSender(context.Background(), "", sender)
				require := require.New(t)
				require.Nil(result)
				require.NotNil(err)
				require.Equal(tc.expectedErrCode, err.Code)
				return
			}

			if tc.setupMock != nil {
				tc.setupMock(mockStore, sender)
			}

			result, err := svc.UpdateSender(context.Background(), testSenderID, sender)
			require := require.New(t)
			require.Nil(result)
			require.NotNil(err)
			require.Equal(tc.expectedErrCode, err.Code)
			mockStore.AssertExpectations(t)
		})
	}
}

func (suite *NotificationSenderMgtServiceTestSuite) TestDeleteSender() {
	suite.mockStore.EXPECT().deleteSender(mock.Anything, testSenderID).Return(nil).Once()
	err := suite.service.DeleteSender(context.Background(), testSenderID)
	suite.Nil(err)
}

func (suite *NotificationSenderMgtServiceTestSuite) TestDeleteSender_EmptyID() {
	err := suite.service.DeleteSender(context.Background(), "")
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSenderID.Code, err.Code)
}

func (suite *NotificationSenderMgtServiceTestSuite) TestDeleteSender_StoreError() {
	suite.mockStore.EXPECT().deleteSender(context.Background(), testSenderID).
		Return(errors.New("database error")).Once()
	err := suite.service.DeleteSender(context.Background(), testSenderID)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

// TestCreateSender_DeclarativeResourcesEnabled tests that CreateSender returns error when declarative resources enabled
func (suite *NotificationSenderMgtServiceTestSuite) TestCreateSender_DeclarativeResourcesEnabled() {
	// Save original config
	originalConfig := config.GetServerRuntime().Config
	defer func() {
		config.GetServerRuntime().Config = originalConfig
	}()

	// Enable declarative resources
	config.GetServerRuntime().Config.DeclarativeResources.Enabled = true

	sender := suite.getValidTwilioSender()
	result, err := suite.service.CreateSender(context.Background(), sender)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal("DCR-1001", err.Code)
}

// TestUpdateSender_DeclarativeResourcesEnabled tests that UpdateSender returns error when declarative resources enabled
func (suite *NotificationSenderMgtServiceTestSuite) TestUpdateSender_DeclarativeResourcesEnabled() {
	// Save original config
	originalConfig := config.GetServerRuntime().Config
	defer func() {
		config.GetServerRuntime().Config = originalConfig
	}()

	// Enable declarative resources
	config.GetServerRuntime().Config.DeclarativeResources.Enabled = true

	sender := suite.getValidTwilioSender()
	result, err := suite.service.UpdateSender(context.Background(), testSenderID, sender)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal("DCR-1002", err.Code)
}

// TestDeleteSender_DeclarativeResourcesEnabled tests that DeleteSender returns error when declarative resources enabled
func (suite *NotificationSenderMgtServiceTestSuite) TestDeleteSender_DeclarativeResourcesEnabled() {
	// Save original config
	originalConfig := config.GetServerRuntime().Config
	defer func() {
		config.GetServerRuntime().Config = originalConfig
	}()

	// Enable declarative resources
	config.GetServerRuntime().Config.DeclarativeResources.Enabled = true

	err := suite.service.DeleteSender(context.Background(), testSenderID)

	suite.NotNil(err)
	suite.Equal("DCR-1003", err.Code)
}

func createTestProperty(name, value string, isSecret bool) cmodels.Property {
	prop, _ := cmodels.NewProperty(name, value, isSecret)
	return *prop
}

func (suite *NotificationSenderMgtServiceTestSuite) getValidTwilioSender() common.NotificationSenderDTO {
	return common.NotificationSenderDTO{
		Name:        "Test Twilio Sender",
		Description: "Test Description",
		Type:        common.NotificationSenderTypeMessage,
		Provider:    common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{
			createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
			createTestProperty("auth_token", "test-auth-token", true),
			createTestProperty("sender_id", "+15551234567", false),
		},
	}
}

func (suite *NotificationSenderMgtServiceTestSuite) getValidVonageSender() common.NotificationSenderDTO {
	return common.NotificationSenderDTO{
		Name:        "Test Vonage Sender",
		Description: "Test Vonage Description",
		Type:        common.NotificationSenderTypeMessage,
		Provider:    common.MessageProviderTypeVonage,
		Properties: []cmodels.Property{
			createTestProperty("api_key", "test-api-key", true),
			createTestProperty("api_secret", "test-api-secret", true),
			createTestProperty("sender_id", "TestSender", false),
		},
	}
}

// fakeTransactioner is a light-weight test double to capture transaction usage without sql mock plumbing.
type fakeTransactioner struct {
	err error
}

func (f *fakeTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	if f.err != nil {
		return f.err
	}
	return txFunc(ctx)
}
