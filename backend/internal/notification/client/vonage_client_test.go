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

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
)

type VonageClientTestSuite struct {
	suite.Suite
}

func TestVonageClientTestSuite(t *testing.T) {
	suite.Run(t, new(VonageClientTestSuite))
}

func (suite *VonageClientTestSuite) SetupSuite() {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *VonageClientTestSuite) getValidVonageSender() common.NotificationSenderDTO {
	return common.NotificationSenderDTO{
		Name:     "Test Vonage",
		Provider: common.MessageProviderTypeVonage,
		Properties: []cmodels.Property{
			createProperty("api_key", "test-api-key", true),
			createProperty("api_secret", "test-api-secret", true),
			createProperty("sender_id", "TestSender", false),
		},
	}
}

func (suite *VonageClientTestSuite) TestNewVonageClient_Success() {
	sender := suite.getValidVonageSender()

	client, err := newVonageClient(context.Background(), sender)

	suite.NoError(err)
	suite.NotNil(client)
	suite.Equal("Test Vonage", client.GetName())
}

func (suite *VonageClientTestSuite) TestGetName() {
	sender := suite.getValidVonageSender()
	client, _ := newVonageClient(context.Background(), sender)

	name := client.GetName()

	suite.Equal("Test Vonage", name)
}

func (suite *VonageClientTestSuite) TestSendSMS_Success() {
	sender := suite.getValidVonageSender()
	client, _ := newVonageClient(context.Background(), sender)

	// Create a test server to mock Vonage API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Equal(http.MethodPost, r.Method)
		suite.Equal("application/json", r.Header.Get("Content-Type"))
		suite.Equal("application/json", r.Header.Get("Accept"))

		// Check authorization
		user, pass, ok := r.BasicAuth()
		suite.True(ok)
		suite.Equal("test-api-key", user)
		suite.Equal("test-api-secret", pass)

		w.WriteHeader(http.StatusAccepted)
		if _, err := w.Write([]byte(`{"message_uuid":"abc123"}`)); err != nil {
			suite.T().Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Replace the Vonage URL with test server URL
	vonageClient := client.(*VonageClient)
	vonageClient.url = server.URL

	data := common.NotificationData{
		Recipient: "+15559876543",
		Body:      "Test message",
	}

	err := client.Send(context.Background(), common.ChannelTypeSMS, data)

	suite.NoError(err)
}

func (suite *VonageClientTestSuite) TestSendSMS_PropagatesCorrelationID() {
	sender := suite.getValidVonageSender()
	client, _ := newVonageClient(context.Background(), sender)

	var gotCorrelationID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCorrelationID = r.Header.Get("X-Correlation-ID")
		w.WriteHeader(http.StatusAccepted)
		if _, err := w.Write([]byte(`{"message_uuid":"abc123"}`)); err != nil {
			suite.T().Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	vonageClient := client.(*VonageClient)
	vonageClient.url = server.URL

	data := common.NotificationData{
		Recipient: "+15559876543",
		Body:      "Test message",
	}

	ctx := sysContext.WithTraceID(context.Background(), "trace-xyz")
	err := client.Send(ctx, common.ChannelTypeSMS, data)

	suite.NoError(err)
	suite.Equal("trace-xyz", gotCorrelationID)
}

func (suite *VonageClientTestSuite) TestSendSMS_Error() {
	sender := suite.getValidVonageSender()
	client, _ := newVonageClient(context.Background(), sender)

	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		if _, err := w.Write([]byte(
			`{"type":"https://www.nexmo.com/messages/Errors#InvalidParams","title":"Invalid Params"}`,
		)); err != nil {
			suite.T().Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Replace the Vonage URL with test server URL
	vonageClient := client.(*VonageClient)
	vonageClient.url = server.URL

	data := common.NotificationData{
		Recipient: "+15559876543",
		Body:      "Test message",
	}

	err := client.Send(context.Background(), common.ChannelTypeSMS, data)

	suite.Error(err)
	suite.Contains(err.Error(), "status: 401")
}

func (suite *VonageClientTestSuite) TestSendSMS_NetworkError() {
	sender := suite.getValidVonageSender()
	client, _ := newVonageClient(context.Background(), sender)

	// Use an invalid URL to force a network error
	vonageClient := client.(*VonageClient)
	vonageClient.url = "http://invalid-vonage-url.local:99999"

	data := common.NotificationData{
		Recipient: "+15559876543",
		Body:      "Test message",
	}

	err := client.Send(context.Background(), common.ChannelTypeSMS, data)

	suite.Error(err)
}

func (suite *VonageClientTestSuite) TestFormatPhoneNumber_WithPlus() {
	sender := suite.getValidVonageSender()
	client, _ := newVonageClient(context.Background(), sender)
	vonageClient := client.(*VonageClient)

	formatted := vonageClient.formatPhoneNumber("+15559876543")

	suite.Equal("15559876543", formatted)
}

func (suite *VonageClientTestSuite) TestFormatPhoneNumber_WithDoubleZero() {
	sender := suite.getValidVonageSender()
	client, _ := newVonageClient(context.Background(), sender)
	vonageClient := client.(*VonageClient)

	formatted := vonageClient.formatPhoneNumber("0015559876543")

	suite.Equal("15559876543", formatted)
}

func (suite *VonageClientTestSuite) TestFormatPhoneNumber_NoPrefix() {
	sender := suite.getValidVonageSender()
	client, _ := newVonageClient(context.Background(), sender)
	vonageClient := client.(*VonageClient)

	formatted := vonageClient.formatPhoneNumber("15559876543")

	suite.Equal("15559876543", formatted)
}

func (suite *VonageClientTestSuite) TestNewVonageClient_WithUnknownProperty() {
	sender := suite.getValidVonageSender()
	sender.Properties = append(sender.Properties, createProperty("unknown_prop", "value", false))

	client, err := newVonageClient(context.Background(), sender)

	// Should succeed and just log a warning for unknown property
	suite.NoError(err)
	suite.NotNil(client)
}
