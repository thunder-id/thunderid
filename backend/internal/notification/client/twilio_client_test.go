// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
)

// testCryptoKey is the shared key used so secret property encryption works in tests.
const testCryptoKey = "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"

// TestMain wires cmodels' package-level config crypto provider once for the whole test
// binary, so secret Property encryption works regardless of which test's SetupSuite last
// reset the server runtime.
func TestMain(m *testing.M) {
	config.ResetServerRuntime()
	if err := config.InitializeServerRuntime("/tmp/test", &config.Config{
		Crypto: config.CryptoConfig{Encryption: engineconfig.EncryptionConfig{Key: testCryptoKey}},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize server runtime: %v\n", err)
		os.Exit(1)
	}
	_, cfgCryptoSvc, err := defaultkm.Initialize(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize default crypto provider: %v\n", err)
		os.Exit(1)
	}
	cmodels.SetConfigCryptoProvider(cfgCryptoSvc)
	config.ResetServerRuntime()
	os.Exit(m.Run())
}

type TwilioClientTestSuite struct {
	suite.Suite
}

func TestTwilioClientTestSuite(t *testing.T) {
	suite.Run(t, new(TwilioClientTestSuite))
}

func (suite *TwilioClientTestSuite) SetupSuite() {
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

func (suite *TwilioClientTestSuite) getValidTwilioSender() common.NotificationSenderDTO {
	return common.NotificationSenderDTO{
		Name:     "Test Twilio",
		Provider: common.NotificationProviderTypeTwilio,
		Properties: []cmodels.Property{
			createProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
			createProperty("auth_token", "test-auth-token", true),
			createProperty("sender_id", "+15551234567", false),
		},
	}
}

func (suite *TwilioClientTestSuite) TestNewTwilioClient_Success() {
	sender := suite.getValidTwilioSender()

	client, err := newTwilioClient(context.Background(), sender)

	suite.NoError(err)
	suite.NotNil(client)
	suite.Equal("Test Twilio", client.GetName())
}

func (suite *TwilioClientTestSuite) TestGetName() {
	sender := suite.getValidTwilioSender()
	client, _ := newTwilioClient(context.Background(), sender)

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

	client, _ := newTwilioClient(context.Background(), sender)

	// Replace the Twilio URL with test server URL
	twilioClient := client.(*TwilioClient)
	twilioClient.url = server.URL

	data := common.NotificationData{
		Recipient: "+15559876543",
		Body:      "Test message",
	}

	err := client.Send(context.Background(), common.ChannelTypeSMS, data)

	suite.NoError(err)
}

func (suite *TwilioClientTestSuite) TestSendSMS_Error() {
	sender := suite.getValidTwilioSender()
	client, _ := newTwilioClient(context.Background(), sender)

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

	err := client.Send(context.Background(), common.ChannelTypeSMS, data)

	suite.Error(err)
	suite.Contains(err.Error(), "status: 401")
}

func (suite *TwilioClientTestSuite) TestSendSMS_NetworkError() {
	sender := suite.getValidTwilioSender()
	client, _ := newTwilioClient(context.Background(), sender)

	// Use an invalid URL to force a network error
	twilioClient := client.(*TwilioClient)
	twilioClient.url = "http://invalid-twilio-url.local:99999"

	data := common.NotificationData{
		Recipient: "+15559876543",
		Body:      "Test message",
	}

	err := client.Send(context.Background(), common.ChannelTypeSMS, data)

	suite.Error(err)
}

func (suite *TwilioClientTestSuite) TestNewTwilioClient_WithUnknownProperty() {
	sender := suite.getValidTwilioSender()
	sender.Properties = append(sender.Properties, createProperty("unknown_prop", "value", false))

	client, err := newTwilioClient(context.Background(), sender)

	// Should succeed and just log a warning for unknown property
	suite.NoError(err)
	suite.NotNil(client)
}

// Helper function
func createProperty(name, value string, isSecret bool) cmodels.Property {
	prop, _ := cmodels.NewProperty(name, value, isSecret)
	return *prop
}
