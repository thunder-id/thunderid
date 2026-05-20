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
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

type ClientProviderTestSuite struct {
	suite.Suite
	provider notificationClientProviderInterface
}

func TestClientProviderTestSuite(t *testing.T) {
	suite.Run(t, new(ClientProviderTestSuite))
}

func (suite *ClientProviderTestSuite) SetupSuite() {
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

func (suite *ClientProviderTestSuite) SetupTest() {
	suite.provider = newNotificationClientProvider()
}

func (suite *ClientProviderTestSuite) TestNewNotificationClientProvider() {
	provider := newNotificationClientProvider()

	suite.NotNil(provider)
	suite.Implements((*notificationClientProviderInterface)(nil), provider)
}

func (suite *ClientProviderTestSuite) TestGetClient() {
	cases := []struct {
		name     string
		sender   common.NotificationSenderDTO
		expected string
	}{
		{
			name: "twilio",
			sender: common.NotificationSenderDTO{
				Name:     "Test Twilio",
				Provider: common.MessageProviderTypeTwilio,
				Properties: []cmodels.Property{
					createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
					createTestProperty("auth_token", "test-token", true),
					createTestProperty("sender_id", "+15551234567", false),
				},
			},
			expected: "Test Twilio",
		},
		{
			name: "vonage",
			sender: common.NotificationSenderDTO{
				Name:     "Test Vonage",
				Provider: common.MessageProviderTypeVonage,
				Properties: []cmodels.Property{
					createTestProperty("api_key", "test-key", true),
					createTestProperty("api_secret", "test-secret", true),
					createTestProperty("sender_id", "TestSender", false),
				},
			},
			expected: "Test Vonage",
		},
		{
			name: "custom",
			sender: common.NotificationSenderDTO{
				Name:     "Test Custom",
				Provider: common.MessageProviderTypeCustom,
				Properties: []cmodels.Property{
					createTestProperty("url", "https://api.example.com/sms", false),
					createTestProperty("http_method", "POST", false),
					createTestProperty("content_type", "JSON", false),
				},
			},
			expected: "Test Custom",
		},
	}

	for _, tc := range cases {
		suite.T().Run(tc.name, func(t *testing.T) {
			client, err := suite.provider.GetClient(tc.sender)
			suite.Nil(err)
			suite.NotNil(client)
			suite.Equal(tc.expected, client.GetName())
		})
	}
}

func (suite *ClientProviderTestSuite) TestGetClientWithError() {
	makeInvalidSecretProps := func(propName string) []cmodels.Property {
		jsonStr := `[{"name":"` + propName + `","value":"not-encrypted-value","isSecret":true}` + `]`
		props, err := cmodels.DeserializePropertiesFromJSON(jsonStr)
		if err != nil {
			return []cmodels.Property{}
		}
		return props
	}

	cases := []struct {
		name   string
		sender common.NotificationSenderDTO
	}{
		{
			name: "twilio_decryption_error",
			sender: common.NotificationSenderDTO{
				Name:     "Bad Twilio",
				Provider: common.MessageProviderTypeTwilio,
				// account_sid is required and marked secret but value will fail decryption
				Properties: append(makeInvalidSecretProps("account_sid"),
					createTestProperty("auth_token", "test-token", true)),
			},
		},
		{
			name: "vonage_decryption_error",
			sender: common.NotificationSenderDTO{
				Name:     "Bad Vonage",
				Provider: common.MessageProviderTypeVonage,
				Properties: append(makeInvalidSecretProps("api_key"),
					createTestProperty("api_secret", "test-secret", true)),
			},
		},
		{
			name: "custom_decryption_error",
			sender: common.NotificationSenderDTO{
				Name:     "Bad Custom",
				Provider: common.MessageProviderTypeCustom,
				// url is secret here and invalid ciphertext
				Properties: makeInvalidSecretProps("url"),
			},
		},
	}

	for _, tc := range cases {
		suite.T().Run(tc.name, func(t *testing.T) {
			client, err := suite.provider.GetClient(tc.sender)
			suite.NotNil(err)
			if err != nil {
				suite.Equal(serviceerror.InternalServerError.Code, err.Code)
			}
			suite.Nil(client)
		})
	}
}

func (suite *ClientProviderTestSuite) TestGetClient_InvalidProvider() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Sender",
		Provider: "invalid-provider",
	}

	client, err := suite.provider.GetClient(sender)

	suite.Nil(client)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidProvider.Code, err.Code)
}
