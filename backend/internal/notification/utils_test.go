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
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (suite *UtilsTestSuite) SetupSuite() {
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

func (suite *UtilsTestSuite) TestValidateNotificationSender_EmptyName() {
	sender := common.NotificationSenderDTO{
		Name:     "",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.MessageProviderTypeTwilio,
	}

	err := validateNotificationSender(sender)

	suite.NotNil(err)
	suite.Equal(ErrorInvalidSenderName.Code, err.Code)
}

func (suite *UtilsTestSuite) TestValidateNotificationSender_InvalidType() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Sender",
		Type:     "INVALID_TYPE",
		Provider: common.MessageProviderTypeTwilio,
	}

	err := validateNotificationSender(sender)

	suite.NotNil(err)
	suite.Equal(ErrorInvalidSenderType.Code, err.Code)
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_EmptyProvider() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Sender",
		Type:     common.NotificationSenderTypeMessage,
		Provider: "",
	}

	err := validateMessageNotificationSender(sender)

	suite.NotNil(err)
	suite.Equal(ErrorInvalidProvider.Code, err.Code)
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_InvalidProvider() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Sender",
		Type:     common.NotificationSenderTypeMessage,
		Provider: "invalid-provider",
	}

	err := validateMessageNotificationSender(sender)

	suite.NotNil(err)
	suite.Equal(ErrorInvalidProvider.Code, err.Code)
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_Twilio() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Twilio",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{
			createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
			createTestProperty("auth_token", "test-token", true),
			createTestProperty("sender_id", "+15551234567", false),
		},
	}

	err := validateMessageNotificationSender(sender)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_Vonage() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Vonage",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.MessageProviderTypeVonage,
		Properties: []cmodels.Property{
			createTestProperty("api_key", "test-key", true),
			createTestProperty("api_secret", "test-secret", true),
			createTestProperty("sender_id", "TestSender", false),
		},
	}

	err := validateMessageNotificationSender(sender)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_Custom() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Custom",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.MessageProviderTypeCustom,
		Properties: []cmodels.Property{
			createTestProperty("url", "https://api.example.com/sms", false),
			createTestProperty("http_method", "POST", false),
			createTestProperty("content_type", "JSON", false),
		},
	}

	err := validateMessageNotificationSender(sender)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateTwilioProperties() {
	properties := []cmodels.Property{
		createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
		createTestProperty("auth_token", "test-token", true),
		createTestProperty("sender_id", "+15551234567", false),
	}

	err := validateTwilioProperties(properties)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateTwilioProperties_MissingAccountSID() {
	properties := []cmodels.Property{
		createTestProperty("auth_token", "test-token", true),
		createTestProperty("sender_id", "+15551234567", false),
	}

	err := validateTwilioProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "account_sid")
}

func (suite *UtilsTestSuite) TestValidateTwilioProperties_MissingAuthToken() {
	properties := []cmodels.Property{
		createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
		createTestProperty("sender_id", "+15551234567", false),
	}

	err := validateTwilioProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "auth_token")
}

func (suite *UtilsTestSuite) TestValidateTwilioProperties_MissingSenderID() {
	properties := []cmodels.Property{
		createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
		createTestProperty("auth_token", "test-token", true),
	}

	err := validateTwilioProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "sender_id")
}

func (suite *UtilsTestSuite) TestValidateTwilioProperties_InvalidAccountSIDFormat() {
	properties := []cmodels.Property{
		createTestProperty("account_sid", "invalid-sid", true),
		createTestProperty("auth_token", "test-token", true),
		createTestProperty("sender_id", "+15551234567", false),
	}

	err := validateTwilioProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "account SID format")
}

func (suite *UtilsTestSuite) TestValidateVonageProperties() {
	properties := []cmodels.Property{
		createTestProperty("api_key", "test-key", true),
		createTestProperty("api_secret", "test-secret", true),
		createTestProperty("sender_id", "TestSender", false),
	}

	err := validateVonageProperties(properties)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateVonageProperties_MissingAPIKey() {
	properties := []cmodels.Property{
		createTestProperty("api_secret", "test-secret", true),
		createTestProperty("sender_id", "TestSender", false),
	}

	err := validateVonageProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "api_key")
}

func (suite *UtilsTestSuite) TestValidateVonageProperties_MissingAPISecret() {
	properties := []cmodels.Property{
		createTestProperty("api_key", "test-key", true),
		createTestProperty("sender_id", "TestSender", false),
	}

	err := validateVonageProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "api_secret")
}

func (suite *UtilsTestSuite) TestValidateVonageProperties_MissingSenderID() {
	properties := []cmodels.Property{
		createTestProperty("api_key", "test-key", true),
		createTestProperty("api_secret", "test-secret", true),
	}

	err := validateVonageProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "sender_id")
}

func (suite *UtilsTestSuite) TestValidateCustomProperties() {
	properties := []cmodels.Property{
		createTestProperty("url", "https://api.example.com/sms", false),
		createTestProperty("http_method", "POST", false),
		createTestProperty("content_type", "JSON", false),
	}

	err := validateCustomProperties(properties)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateCustomProperties_MissingURL() {
	properties := []cmodels.Property{
		createTestProperty("http_method", "POST", false),
		createTestProperty("content_type", "JSON", false),
	}

	err := validateCustomProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "URL")
}

func (suite *UtilsTestSuite) TestValidateCustomProperties_InvalidHTTPMethod() {
	properties := []cmodels.Property{
		createTestProperty("url", "https://api.example.com/sms", false),
		createTestProperty("http_method", "DELETE", false),
	}

	err := validateCustomProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "HTTP method")
}

func (suite *UtilsTestSuite) TestValidateCustomProperties_InvalidContentType() {
	properties := []cmodels.Property{
		createTestProperty("url", "https://api.example.com/sms", false),
		createTestProperty("http_method", "POST", false),
		createTestProperty("content_type", "XML", false),
	}

	err := validateCustomProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "content type")
}

func (suite *UtilsTestSuite) TestValidateCustomProperties_EmptyPropertyName() {
	properties := []cmodels.Property{
		createTestProperty("url", "https://api.example.com/sms", false),
		createTestProperty("", "value", false),
	}

	err := validateCustomProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "non-empty name")
}

func (suite *UtilsTestSuite) TestValidateSenderProperties() {
	properties := []cmodels.Property{
		createTestProperty("prop1", "value1", false),
		createTestProperty("prop2", "value2", false),
	}
	requiredProps := map[string]bool{
		"prop1": false,
		"prop2": false,
	}

	err := validateSenderProperties(properties, requiredProps)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateSenderProperties_MissingRequired() {
	properties := []cmodels.Property{
		createTestProperty("prop1", "value1", false),
	}
	requiredProps := map[string]bool{
		"prop1": false,
		"prop2": false,
	}

	err := validateSenderProperties(properties, requiredProps)

	suite.NotNil(err)
	suite.Contains(err.Error(), "prop2")
}

func (suite *UtilsTestSuite) TestValidateSenderProperties_EmptyName() {
	properties := []cmodels.Property{
		createTestProperty("", "value1", false),
	}
	requiredProps := map[string]bool{}

	err := validateSenderProperties(properties, requiredProps)

	suite.NotNil(err)
	suite.Contains(err.Error(), "non-empty name")
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_EmptyProperties() {
	sender := common.NotificationSenderDTO{
		Name:       "Test Twilio Empty Props",
		Type:       common.NotificationSenderTypeMessage,
		Provider:   common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{},
	}

	err := validateMessageNotificationSender(sender)

	suite.NotNil(err)
	suite.Equal(ErrorInvalidRequestFormat.Code, err.Code)
	suite.Contains(err.ErrorDescription.DefaultValue, "message notification sender properties cannot be empty")
}

func (suite *UtilsTestSuite) TestValidateNotificationSender() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Sender",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{
			createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
			createTestProperty("auth_token", "test-token", true),
			createTestProperty("sender_id", "+15551234567", false),
		},
	}

	err := validateNotificationSender(sender)
	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSenderProperties_UnsupportedProvider() {
	sender := common.NotificationSenderDTO{
		Provider:   "unsupported-provider",
		Properties: []cmodels.Property{createTestProperty("k", "v", false)},
	}

	err := validateMessageNotificationSenderProperties(sender)
	suite.NotNil(err)
	suite.Contains(err.Error(), "unsupported message notification sender")
}

func (suite *UtilsTestSuite) TestValidateTwilioProperties_RegexError() {
	// patch matchString to return an error
	orig := matchString
	matchString = func(pattern, s string) (bool, error) {
		return false, errors.New("regex fail")
	}
	defer func() { matchString = orig }()

	properties := []cmodels.Property{
		createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
		createTestProperty("auth_token", "test-token", true),
		createTestProperty("sender_id", "+15551234567", false),
	}

	err := validateTwilioProperties(properties)
	suite.NotNil(err)
	suite.Contains(err.Error(), "failed to validate Twilio account SID")
}
