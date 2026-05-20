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

package message

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
)

type TwilioClientTestSuite struct {
	suite.Suite
}

func TestTwilioClientTestSuite(t *testing.T) {
	suite.Run(t, new(TwilioClientTestSuite))
}

func (suite *TwilioClientTestSuite) SetupSuite() {
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

func (suite *TwilioClientTestSuite) getValidTwilioSender() common.NotificationSenderDTO {
	return common.NotificationSenderDTO{
		Name:     "Test Twilio",
		Provider: common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{
			createProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
			createProperty("auth_token", "test-auth-token", true),
			createProperty("sender_id", "+15551234567", false),
		},
	}
}

func (suite *TwilioClientTestSuite) TestNewTwilioClient_Success() {
	sender := suite.getValidTwilioSender()

	client, err := NewTwilioClient(sender)

	suite.NoError(err)
	suite.NotNil(client)
	suite.Equal("Test Twilio", client.GetName())
}

func (suite *TwilioClientTestSuite) TestGetName() {
	sender := suite.getValidTwilioSender()
	client, _ := NewTwilioClient(sender)

	name := client.GetName()

	suite.Equal("Test Twilio", name)
}

func (suite *TwilioClientTestSuite) TestSendSMS_Success() {
	sender := suite.getValidTwilioSender()

	// Create a test server to mock Twilio API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Equal(http.MethodPost, r.Method)

		// Check authorization
		user, pass, ok := r.BasicAuth()
		suite.True(ok)
		suite.Equal("AC00112233445566778899aabbccddeeff", user)
		suite.Equal("test-auth-token", pass)

		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(`{"sid":"SM1234567890","status":"queued"}`)); err != nil {
			suite.T().Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Update sender to use test server URL
	accountSID := "AC00112233445566778899aabbccddeeff"
	sender.Properties = []cmodels.Property{
		createProperty("account_sid", accountSID, true),
		createProperty("auth_token", "test-auth-token", true),
		createProperty("sender_id", "+15551234567", false),
	}

	client, _ := NewTwilioClient(sender)

	// Replace the Twilio URL with test server URL
	twilioClient := client.(*TwilioClient)
	twilioClient.url = server.URL

	data := common.NotificationData{
		Recipient: "+15559876543",
		Body:      "Test message",
	}

	err := client.Send(common.ChannelTypeSMS, data)

	suite.NoError(err)
}

func (suite *TwilioClientTestSuite) TestSendSMS_Error() {
	sender := suite.getValidTwilioSender()
	client, _ := NewTwilioClient(sender)

	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		if _, err := w.Write([]byte(`{"code":20003,"message":"Authenticate","status":401}`)); err != nil {
			suite.T().Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Replace the Twilio URL with test server URL
	twilioClient := client.(*TwilioClient)
	twilioClient.url = server.URL

	data := common.NotificationData{
		Recipient: "+15559876543",
		Body:      "Test message",
	}

	err := client.Send(common.ChannelTypeSMS, data)

	suite.Error(err)
	suite.Contains(err.Error(), "status: 401")
}

func (suite *TwilioClientTestSuite) TestSendSMS_NetworkError() {
	sender := suite.getValidTwilioSender()
	client, _ := NewTwilioClient(sender)

	// Use an invalid URL to force a network error
	twilioClient := client.(*TwilioClient)
	twilioClient.url = "http://invalid-twilio-url.local:99999"

	data := common.NotificationData{
		Recipient: "+15559876543",
		Body:      "Test message",
	}

	err := client.Send(common.ChannelTypeSMS, data)

	suite.Error(err)
}

func (suite *TwilioClientTestSuite) TestNewTwilioClient_WithUnknownProperty() {
	sender := suite.getValidTwilioSender()
	sender.Properties = append(sender.Properties, createProperty("unknown_prop", "value", false))

	client, err := NewTwilioClient(sender)

	// Should succeed and just log a warning for unknown property
	suite.NoError(err)
	suite.NotNil(client)
}

// Helper function
func createProperty(name, value string, isSecret bool) cmodels.Property {
	prop, _ := cmodels.NewProperty(name, value, isSecret)
	return *prop
}
