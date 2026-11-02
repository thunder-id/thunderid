// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
)

type CustomClientTestSuite struct {
	suite.Suite
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestCustomClientTestSuite(t *testing.T) {
	suite.Run(t, new(CustomClientTestSuite))
}

func (suite *CustomClientTestSuite) SetupSuite() {
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

func (suite *CustomClientTestSuite) getValidCustomSenderJSON() common.NotificationSenderDTO {
	return common.NotificationSenderDTO{
		Name:     "Test Custom",
		Provider: common.NotificationProviderTypeCustom,
		Properties: []cmodels.Property{
			createProperty("url", "https://api.example.com/sms", false),
			createProperty("http_method", "POST", false),
			createProperty("content_type", "JSON", false),
			createProperty(common.CustomPropKeyHTTPHeaders,
				"X-Api-Key: key123, X-Tenant: tenant-1", true),
		},
	}
}

func (suite *CustomClientTestSuite) getValidCustomSenderFORM() common.NotificationSenderDTO {
	return common.NotificationSenderDTO{
		Name:     "Test Custom Form",
		Provider: common.NotificationProviderTypeCustom,
		Properties: []cmodels.Property{
			createProperty("url", "https://api.example.com/sms", false),
			createProperty("http_method", "POST", false),
			createProperty("content_type", "FORM", false),
		},
	}
}

func (suite *CustomClientTestSuite) TestNewCustomClient_Success() {
	sender := suite.getValidCustomSenderJSON()

	client, err := newCustomClient(context.Background(), sender)

	suite.NoError(err)
	suite.NotNil(client)
	suite.Equal("Test Custom", client.GetName())
}

func (suite *CustomClientTestSuite) TestGetName() {
	sender := suite.getValidCustomSenderJSON()
	client, _ := newCustomClient(context.Background(), sender)

	name := client.GetName()

	suite.Equal("Test Custom", name)
}

func (suite *CustomClientTestSuite) TestSendSMS_JSON_Success() {
	sender := suite.getValidCustomSenderJSON()
	client, _ := newCustomClient(context.Background(), sender)

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Equal(http.MethodPost, r.Method)
		suite.Equal("application/json", r.Header.Get("Content-Type"))
		suite.Equal("key123", r.Header.Get("X-Api-Key"))
		suite.Equal("tenant-1", r.Header.Get("X-Tenant"))

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"success":true}`)); err != nil {
			suite.T().Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Replace the URL with test server URL
	customClient := client.(*CustomClient)
	customClient.url = server.URL

	data := common.MessageData{
		Recipient: "+15559876543",
		Body:      `{"message":"Test message"}`,
	}

	err := client.Send(context.Background(), common.ChannelTypeSMS, data)

	suite.NoError(err)
}

func (suite *CustomClientTestSuite) TestSendSMSLegacyHTTPHeadersSuccess() {
	for _, test := range []struct {
		name     string
		isSecret bool
	}{{name: "plaintext"}, {name: "encrypted", isSecret: true}} {
		suite.Run(test.name, func() {
			sender := suite.getValidCustomSenderJSON()
			sender.Properties[len(sender.Properties)-1] = createProperty(
				"http_headers", "X-API-Key: legacy-key, X-Tenant: legacy-tenant", test.isSecret,
			)
			client, err := newCustomClient(context.Background(), sender)
			suite.Require().NoError(err)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				suite.Equal("legacy-key", r.Header.Get("X-API-Key"))
				suite.Equal("legacy-tenant", r.Header.Get("X-Tenant"))
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client.(*CustomClient).url = server.URL
			err = client.Send(context.Background(), common.ChannelTypeSMS, common.MessageData{
				Recipient: "+15559876543", Body: `{"message":"Test message"}`,
			})
			suite.NoError(err)
		})
	}
}

func (suite *CustomClientTestSuite) TestSendSMSStringHeadersAndAuthentication() {
	sender := suite.getValidCustomSenderFORM()
	sender.Properties = append(sender.Properties,
		createProperty(common.CustomPropKeyHTTPHeaders,
			"X-Campaign: campaign-1", false),
	)
	authProperties, err := outboundauth.ToProperties(outboundauth.Config{
		Type: outboundauth.TypeAPIKey, Properties: map[string]string{"X-Api-Key": "secret"},
	})
	suite.Require().NoError(err)
	sender.Properties = append(sender.Properties, authProperties...)
	client, err := newCustomClient(context.Background(), sender)
	suite.Require().NoError(err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Equal("campaign-1", r.Header.Get("X-Campaign"))
		suite.Equal("secret", r.Header.Get("X-Api-Key"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client.(*CustomClient).url = server.URL
	err = client.Send(context.Background(), common.ChannelTypeSMS, common.MessageData{Body: "text"})
	suite.NoError(err)
}

func (suite *CustomClientTestSuite) TestSendSMSBearerAndBasicAuthentication() {
	for _, test := range []struct {
		name   string
		config outboundauth.Config
		check  func(*http.Request)
	}{
		{"bearer", outboundauth.Config{Type: outboundauth.TypeBearer,
			Properties: map[string]string{outboundauth.FieldBearerToken: "token"}},
			func(request *http.Request) { suite.Equal("Bearer token", request.Header.Get("Authorization")) }},
		{"basic", outboundauth.Config{Type: outboundauth.TypeBasic,
			Properties: map[string]string{outboundauth.FieldBasicUsername: "user",
				outboundauth.FieldBasicPassword: "password"}},
			func(request *http.Request) {
				username, password, ok := request.BasicAuth()
				suite.True(ok)
				suite.Equal("user", username)
				suite.Equal("password", password)
			}},
	} {
		suite.Run(test.name, func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				test.check(request)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			sender := suite.getValidCustomSenderJSON()
			authProperties, err := outboundauth.ToProperties(test.config)
			suite.Require().NoError(err)
			sender.Properties = append(sender.Properties, authProperties...)
			client, err := newCustomClient(context.Background(), sender)
			suite.Require().NoError(err)
			client.(*CustomClient).url = server.URL
			suite.NoError(client.Send(context.Background(), common.ChannelTypeSMS,
				common.MessageData{Body: `{"message":"test"}`}))
		})
	}
}

func (suite *CustomClientTestSuite) TestRejectsGeneralAuthorizationWithBearerOrBasic() {
	for _, config := range []outboundauth.Config{
		{Type: outboundauth.TypeBearer, Properties: map[string]string{outboundauth.FieldBearerToken: "token"}},
		{Type: outboundauth.TypeBasic, Properties: map[string]string{
			outboundauth.FieldBasicUsername: "user", outboundauth.FieldBasicPassword: "password"}},
	} {
		sender := suite.getValidCustomSenderJSON()
		sender.Properties = append(sender.Properties,
			createProperty(common.CustomPropKeyHTTPHeaders, "authorization: legacy", false))
		authProperties, err := outboundauth.ToProperties(config)
		suite.Require().NoError(err)
		sender.Properties = append(sender.Properties, authProperties...)
		_, err = newCustomClient(context.Background(), sender)
		suite.ErrorContains(err, "also used for authentication")
	}
}

func (suite *CustomClientTestSuite) TestSendSMSLegacyHeadersAllowRemoteHTTP() {
	sender := suite.getValidCustomSenderJSON()
	sender.Properties[len(sender.Properties)-1] = createProperty(
		"http_headers", "X-API-Key: legacy-key", false,
	)
	client, err := newCustomClient(context.Background(), sender)
	suite.Require().NoError(err)
	customClient := client.(*CustomClient)
	customClient.url = "http://internal.example/send"
	called := false
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		called = true
		suite.Equal("legacy-key", request.Header.Get("X-API-Key"))
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})
	customClient.httpClient = &http.Client{Transport: transport}

	err = client.Send(context.Background(), common.ChannelTypeSMS, common.MessageData{
		Recipient: "+15559876543", Body: `{"message":"Test message"}`,
	})
	suite.NoError(err)
	suite.True(called)
}

func (suite *CustomClientTestSuite) TestSendSMSLegacyHeadersFollowRedirect() {
	forwarded := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		forwarded = true
		suite.Equal("campaign-1", request.Header.Get("X-Campaign"))
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	sender := suite.getValidCustomSenderJSON()
	sender.Properties[len(sender.Properties)-1] = createProperty(
		common.CustomPropKeyHTTPHeaders, "X-Campaign: campaign-1", false,
	)
	client, err := newCustomClient(context.Background(), sender)
	suite.Require().NoError(err)
	client.(*CustomClient).url = redirect.URL

	err = client.Send(context.Background(), common.ChannelTypeSMS, common.MessageData{Body: `{"message":"test"}`})
	suite.NoError(err)
	suite.True(forwarded)
}

func (suite *CustomClientTestSuite) TestSendSMSAuthenticationDoesNotFollowRedirect() {
	forwarded := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		forwarded = true
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	sender := suite.getValidCustomSenderFORM()
	authProperties, err := outboundauth.ToProperties(outboundauth.Config{
		Type: outboundauth.TypeAPIKey, Properties: map[string]string{"X-API-Key": "secret"},
	})
	suite.Require().NoError(err)
	sender.Properties = append(sender.Properties, authProperties...)
	client, err := newCustomClient(context.Background(), sender)
	suite.Require().NoError(err)
	client.(*CustomClient).url = redirect.URL

	err = client.Send(context.Background(), common.ChannelTypeSMS, common.MessageData{Body: "message=test"})
	suite.ErrorContains(err, "status: 307")
	suite.False(forwarded)
}

func (suite *CustomClientTestSuite) TestSendSMS_FORM_Success() {
	sender := suite.getValidCustomSenderFORM()
	client, _ := newCustomClient(context.Background(), sender)

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Equal(http.MethodPost, r.Method)
		suite.Equal("application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`OK`)); err != nil {
			suite.T().Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Replace the URL with test server URL
	customClient := client.(*CustomClient)
	customClient.url = server.URL

	data := common.MessageData{
		Recipient: "+15559876543",
		Body:      "to=+15559876543\nmessage=Test message",
	}

	err := client.Send(context.Background(), common.ChannelTypeSMS, data)

	suite.NoError(err)
}

func (suite *CustomClientTestSuite) TestSendSMS_Error() {
	sender := suite.getValidCustomSenderJSON()
	client, _ := newCustomClient(context.Background(), sender)

	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"error":"Invalid request"}`)); err != nil {
			suite.T().Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Replace the URL with test server URL
	customClient := client.(*CustomClient)
	customClient.url = server.URL

	data := common.MessageData{
		Recipient: "+15559876543",
		Body:      `{"message":"Test"}`,
	}

	err := client.Send(context.Background(), common.ChannelTypeSMS, data)

	suite.Error(err)
	suite.Contains(err.Error(), "status: 400")
}

func (suite *CustomClientTestSuite) TestSendSMS_NetworkError() {
	sender := suite.getValidCustomSenderJSON()
	client, _ := newCustomClient(context.Background(), sender)

	// Use an invalid URL to force a network error
	customClient := client.(*CustomClient)
	customClient.url = "http://invalid-custom-url.local:99999"

	data := common.MessageData{
		Recipient: "+15559876543",
		Body:      `{"message":"Test"}`,
	}

	err := client.Send(context.Background(), common.ChannelTypeSMS, data)

	suite.Error(err)
}

func (suite *CustomClientTestSuite) TestSendSMS_UnsupportedContentType() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Custom",
		Provider: common.NotificationProviderTypeCustom,
		Properties: []cmodels.Property{
			createProperty("url", "https://api.example.com/sms", false),
			createProperty("http_method", "POST", false),
			createProperty("content_type", "XML", false),
		},
	}
	client, _ := newCustomClient(context.Background(), sender)

	data := common.MessageData{
		Recipient: "+15559876543",
		Body:      `<message>Test</message>`,
	}

	err := client.Send(context.Background(), common.ChannelTypeSMS, data)

	suite.Error(err)
	suite.Contains(err.Error(), "unsupported content type")
}

func (suite *CustomClientTestSuite) TestNewCustomClient_WithUnknownProperty() {
	sender := suite.getValidCustomSenderJSON()
	sender.Properties = append(sender.Properties, createProperty("unknown_prop", "value", false))

	client, err := newCustomClient(context.Background(), sender)

	// Should succeed and just log a warning for unknown property
	suite.NoError(err)
	suite.NotNil(client)
}

func (suite *CustomClientTestSuite) TestNewCustomClient_InvalidAPIKeyHeaders() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Custom",
		Provider: common.NotificationProviderTypeCustom,
		Properties: []cmodels.Property{
			createProperty("url", "https://api.example.com/sms", false),
			createProperty("http_method", "POST", false),
			createProperty("content_type", "JSON", false),
			createProperty(common.CustomPropKeyHTTPHeaders, "InvalidHeaderFormat", true),
		},
	}

	client, err := newCustomClient(context.Background(), sender)

	suite.Error(err)
	suite.Nil(client)
	suite.Contains(err.Error(), "failed to parse legacy HTTP headers")
}
