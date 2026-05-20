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
	"testing"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"

	"github.com/stretchr/testify/suite"
)

// FileBasedStoreTestSuite contains comprehensive tests for the file-based notification sender store.
type FileBasedStoreTestSuite struct {
	suite.Suite
	store *notificationFileBasedStore
}

// TestFileBasedStoreTestSuite runs the file-based store test suite.
func TestFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedStoreTestSuite))
}

func (suite *FileBasedStoreTestSuite) SetupSuite() {
	// Create temporary directory and crypto key file
	tempDir := suite.T().TempDir()

	// Initialize server runtime once for all tests
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime(tempDir, testConfig)
}

func (suite *FileBasedStoreTestSuite) TearDownSuite() {
	// Clean up server runtime after all tests
	config.ResetServerRuntime()
}

func (suite *FileBasedStoreTestSuite) SetupTest() {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeNotificationSender)
	suite.store = &notificationFileBasedStore{
		GenericFileBasedStore: genericStore,
	}
}

func (suite *FileBasedStoreTestSuite) createTestSender(id, name string) *common.NotificationSenderDTO {
	properties := []cmodels.Property{}
	prop, _ := cmodels.NewProperty("account_sid", "test_account_sid", false)
	properties = append(properties, *prop)
	prop2, _ := cmodels.NewProperty("auth_token", "test_auth_token", true)
	properties = append(properties, *prop2)
	prop3, _ := cmodels.NewProperty("sender_id", "test_sender_id", false)
	properties = append(properties, *prop3)

	return &common.NotificationSenderDTO{
		ID:          id,
		Name:        name,
		Description: "Test notification sender",
		Type:        common.NotificationSenderTypeMessage,
		Provider:    common.MessageProviderTypeTwilio,
		Properties:  properties,
	}
}

func (suite *FileBasedStoreTestSuite) TestCreateSender_Success() {
	// Arrange
	sender := suite.createTestSender("sender-001", "Twilio Test Sender")

	// Act
	err := suite.store.createSender(context.Background(), *sender)

	// Assert
	suite.NoError(err)

	// Verify sender was created
	actualSender, err := suite.store.getSenderByName(context.Background(), sender.Name)
	suite.NoError(err)
	suite.NotNil(actualSender)
	suite.Equal("sender-001", actualSender.ID)
	suite.Equal("Twilio Test Sender", actualSender.Name)
	suite.Equal(common.MessageProviderTypeTwilio, actualSender.Provider)
}

func (suite *FileBasedStoreTestSuite) TestGetSenderByID_Success() {
	// Arrange
	sender := suite.createTestSender("sender-002", "Vonage Test Sender")
	sender.Provider = common.MessageProviderTypeVonage
	_ = suite.store.createSender(context.Background(), *sender)

	// Act
	retrieved, err := suite.store.getSenderByID(context.Background(), "sender-002")

	// Assert
	suite.NoError(err)
	suite.NotNil(retrieved)
	suite.Equal("sender-002", retrieved.ID)
	suite.Equal("Vonage Test Sender", retrieved.Name)
	suite.Equal(common.MessageProviderTypeVonage, retrieved.Provider)
}

func (suite *FileBasedStoreTestSuite) TestGetSenderByID_NotFound() {
	// Act
	retrieved, err := suite.store.getSenderByID(context.Background(), "non-existent")

	// Assert
	suite.Error(err)
	suite.Nil(retrieved)
}

func (suite *FileBasedStoreTestSuite) TestGetSenderByName_Success() {
	// Arrange
	sender := suite.createTestSender("sender-003", "Custom SMS Sender")
	sender.Provider = common.MessageProviderTypeCustom
	_ = suite.store.createSender(context.Background(), *sender)

	// Act
	retrieved, err := suite.store.getSenderByName(context.Background(), "Custom SMS Sender")

	// Assert
	suite.NoError(err)
	suite.NotNil(retrieved)
	suite.Equal("sender-003", retrieved.ID)
	suite.Equal("Custom SMS Sender", retrieved.Name)
	suite.Equal(common.MessageProviderTypeCustom, retrieved.Provider)
}

func (suite *FileBasedStoreTestSuite) TestGetSenderByName_NotFound() {
	// Act
	actualSender, err := suite.store.getSenderByName(context.Background(), "invalid-name")

	// Assert
	suite.NoError(err)
	suite.Nil(actualSender)
}

func (suite *FileBasedStoreTestSuite) TestListSenders_Empty() {
	// Act
	senders, err := suite.store.listSenders(context.Background())

	// Assert
	suite.NoError(err)
	suite.NotNil(senders)
	suite.Empty(senders)
}

func (suite *FileBasedStoreTestSuite) TestListSenders_MultipleSenders() {
	// Arrange
	sender1 := suite.createTestSender("sender-004", "Sender One")
	sender2 := suite.createTestSender("sender-005", "Sender Two")
	sender3 := suite.createTestSender("sender-006", "Sender Three")

	_ = suite.store.createSender(context.Background(), *sender1)
	_ = suite.store.createSender(context.Background(), *sender2)
	_ = suite.store.createSender(context.Background(), *sender3)

	// Act
	senders, err := suite.store.listSenders(context.Background())

	// Assert
	suite.NoError(err)
	suite.NotNil(senders)
	suite.Len(senders, 3)

	// Verify all senders are present
	senderNames := make(map[string]bool)
	for _, s := range senders {
		senderNames[s.Name] = true
	}
	suite.True(senderNames["Sender One"])
	suite.True(senderNames["Sender Two"])
	suite.True(senderNames["Sender Three"])
}

func (suite *FileBasedStoreTestSuite) TestUpdateSender_ReturnsError() {
	// Act
	err := suite.store.updateSender(context.Background(), "any-id", common.NotificationSenderDTO{})

	// Assert
	suite.Error(err)
	suite.Equal("updateSender is not supported in file-based store", err.Error())
}

func (suite *FileBasedStoreTestSuite) TestDeleteSender_ReturnsError() {
	// Act
	err := suite.store.deleteSender(context.Background(), "any-id")

	// Assert
	suite.Error(err)
	suite.Equal("deleteSender is not supported in file-based store", err.Error())
}

func (suite *FileBasedStoreTestSuite) TestCreateMultipleSenders_WithProperties() {
	// Arrange - Create senders with different property configurations
	twilioSender := suite.createTestSender("twilio-001", "Twilio Production")

	// Vonage sender with different properties
	vonageProps := []cmodels.Property{}
	prop1, _ := cmodels.NewProperty("api_key", "test_api_key", false)
	vonageProps = append(vonageProps, *prop1)
	prop2, _ := cmodels.NewProperty("api_secret", "test_api_secret", true)
	vonageProps = append(vonageProps, *prop2)
	prop3, _ := cmodels.NewProperty("sender_id", "vonage_sender", false)
	vonageProps = append(vonageProps, *prop3)

	vonageSender := &common.NotificationSenderDTO{
		ID:          "vonage-001",
		Name:        "Vonage Production",
		Description: "Vonage notification sender",
		Type:        common.NotificationSenderTypeMessage,
		Provider:    common.MessageProviderTypeVonage,
		Properties:  vonageProps,
	}

	// Act
	err1 := suite.store.createSender(context.Background(), *twilioSender)
	err2 := suite.store.createSender(context.Background(), *vonageSender)

	// Assert
	suite.NoError(err1)
	suite.NoError(err2)

	// Verify both senders exist with correct properties
	twilioRetrieved, err := suite.store.getSenderByID(context.Background(), "twilio-001")
	suite.NoError(err)
	suite.Len(twilioRetrieved.Properties, 3)

	vonageRetrieved, err := suite.store.getSenderByID(context.Background(), "vonage-001")
	suite.NoError(err)
	suite.Len(vonageRetrieved.Properties, 3)
}

func (suite *FileBasedStoreTestSuite) TestNewNotificationFileBasedStore() {
	// Act
	store, _ := newNotificationFileBasedStore()

	// Assert
	suite.NotNil(store)

	// Verify it implements the interface
	var _ notificationStoreInterface = store
}
