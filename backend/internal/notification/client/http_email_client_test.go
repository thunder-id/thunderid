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

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
)

type HTTPEmailClientTestSuite struct {
	suite.Suite
}

func TestHTTPEmailClientTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPEmailClientTestSuite))
}

func (suite *HTTPEmailClientTestSuite) SetupSuite() {
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

func (suite *HTTPEmailClientTestSuite) getValidCustomSenderJSON() common.NotificationSenderDTO {
	return common.NotificationSenderDTO{
		Name:     "Test HTTP Email",
		Provider: common.MessageProviderTypeHTTP,
		Type:     common.NotificationSenderTypeEmail,
		Properties: []cmodels.Property{
			createProperty("url", "https://api.example.com/email", false),
			createProperty("http_method", "POST", false),
			createProperty("content_type", "JSON", false),
			createProperty("http_headers", "Authorization:Bearer token,X-Api-Key:key123", false),
		},
	}
}

func (suite *HTTPEmailClientTestSuite) getValidCustomSenderFORM() common.NotificationSenderDTO {
	return common.NotificationSenderDTO{
		Name:     "Test HTTP Email Form",
		Provider: common.MessageProviderTypeCustom,
		Type:     common.NotificationSenderTypeEmail,
		Properties: []cmodels.Property{
			createProperty("url", "https://api.example.com/email", false),
			createProperty("http_method", "POST", false),
			createProperty("content_type", "FORM", false),
		},
	}
}

func (suite *HTTPEmailClientTestSuite) TestNewHTTPEmailClient_Success() {
	sender := suite.getValidCustomSenderJSON()

	client, err := newHTTPEmailClient(context.Background(), sender)

	suite.NoError(err)
	suite.NotNil(client)
	suite.Equal("Test HTTP Email", client.GetName())
}

func (suite *HTTPEmailClientTestSuite) TestGetName() {
	sender := suite.getValidCustomSenderJSON()
	client, _ := newHTTPEmailClient(context.Background(), sender)

	name := client.GetName()

	suite.Equal("Test HTTP Email", name)
}

func (suite *HTTPEmailClientTestSuite) TestSendEmail_JSON_Success() {
	sender := suite.getValidCustomSenderJSON()
	client, _ := newHTTPEmailClient(context.Background(), sender)

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.Equal(http.MethodPost, r.Method)
		suite.Equal("application/json", r.Header.Get("Content-Type"))
		suite.Equal("Bearer token", r.Header.Get("Authorization"))
		suite.Equal("key123", r.Header.Get("X-Api-Key"))

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"success":true}`)); err != nil {
			suite.T().Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Replace the URL with test server URL
	customClient := client.(*HTTPEmailClient)
	customClient.url = server.URL

	data := common.EmailData{
		Recipient: "user@example.com",
		Subject:   "Test Subject",
		Body:      "Test Body",
		IsHTML:    false,
	}

	err := client.(EmailClientInterface).Send(context.Background(), data)

	suite.NoError(err)
}

func (suite *HTTPEmailClientTestSuite) TestSendEmail_FORM_Success() {
	sender := suite.getValidCustomSenderFORM()
	client, _ := newHTTPEmailClient(context.Background(), sender)

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
	customClient := client.(*HTTPEmailClient)
	customClient.url = server.URL

	data := common.EmailData{
		Recipient: "user@example.com",
		Subject:   "Test Subject",
		Body:      "Test Body",
		IsHTML:    true,
	}

	err := client.(EmailClientInterface).Send(context.Background(), data)

	suite.NoError(err)
}

func (suite *HTTPEmailClientTestSuite) TestSendEmail_Error() {
	sender := suite.getValidCustomSenderJSON()
	client, _ := newHTTPEmailClient(context.Background(), sender)

	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"error":"Invalid request"}`)); err != nil {
			suite.T().Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Replace the URL with test server URL
	customClient := client.(*HTTPEmailClient)
	customClient.url = server.URL

	data := common.EmailData{
		Recipient: "user@example.com",
		Subject:   "Test",
		Body:      "Test",
	}

	err := client.(EmailClientInterface).Send(context.Background(), data)

	suite.Error(err)
	suite.Contains(err.Error(), "status: 400")
}

func (suite *HTTPEmailClientTestSuite) TestSendEmail_NetworkError() {
	sender := suite.getValidCustomSenderJSON()
	client, _ := newHTTPEmailClient(context.Background(), sender)

	// Use an invalid URL to force a network error
	customClient := client.(*HTTPEmailClient)
	customClient.url = "http://invalid-custom-url.local:99999"

	data := common.EmailData{
		Recipient: "user@example.com",
		Subject:   "Test",
		Body:      "Test",
	}

	err := client.(EmailClientInterface).Send(context.Background(), data)

	suite.Error(err)
}

func (suite *HTTPEmailClientTestSuite) TestSendEmail_UnsupportedContentType() {
	sender := common.NotificationSenderDTO{
		Name:     "Test HTTP Email",
		Provider: common.MessageProviderTypeCustom,
		Properties: []cmodels.Property{
			createProperty("url", "https://api.example.com/email", false),
			createProperty("http_method", "POST", false),
			createProperty("content_type", "XML", false),
		},
	}
	client, _ := newHTTPEmailClient(context.Background(), sender)

	data := common.EmailData{
		Recipient: "user@example.com",
		Subject:   "Test",
		Body:      "Test",
	}

	err := client.(EmailClientInterface).Send(context.Background(), data)

	suite.Error(err)
	suite.Contains(err.Error(), "unsupported content type")
}

func (suite *HTTPEmailClientTestSuite) TestGetHeadersFromString_Success() {
	sender := suite.getValidCustomSenderJSON()
	client, _ := newHTTPEmailClient(context.Background(), sender)
	customClient := client.(*HTTPEmailClient)

	headers, err := customClient.getHeadersFromString("Authorization:Bearer token,X-Api-Key:key123")

	suite.NoError(err)
	suite.Equal(2, len(headers))
	suite.Equal("Bearer token", headers["Authorization"])
	suite.Equal("key123", headers["X-Api-Key"])
}

func (suite *HTTPEmailClientTestSuite) TestGetHeadersFromString_InvalidFormat() {
	sender := suite.getValidCustomSenderJSON()
	client, _ := newHTTPEmailClient(context.Background(), sender)
	customClient := client.(*HTTPEmailClient)

	headers, err := customClient.getHeadersFromString("InvalidHeader")

	suite.Error(err)
	suite.Nil(headers)
	suite.Contains(err.Error(), "invalid HTTP header format")
}

func (suite *HTTPEmailClientTestSuite) TestNewHTTPEmailClient_WithUnknownProperty() {
	sender := suite.getValidCustomSenderJSON()
	sender.Properties = append(sender.Properties, createProperty("unknown_prop", "value", false))

	client, err := newHTTPEmailClient(context.Background(), sender)

	// Should succeed and just log a warning for unknown property
	suite.NoError(err)
	suite.NotNil(client)
}

func (suite *HTTPEmailClientTestSuite) TestNewHTTPEmailClient_InvalidHeaders() {
	sender := common.NotificationSenderDTO{
		Name:     "Test HTTP Email",
		Provider: common.MessageProviderTypeCustom,
		Properties: []cmodels.Property{
			createProperty("url", "https://api.example.com/email", false),
			createProperty("http_method", "POST", false),
			createProperty("content_type", "JSON", false),
			createProperty("http_headers", "InvalidHeaderFormat", false),
		},
	}

	client, err := newHTTPEmailClient(context.Background(), sender)

	suite.Error(err)
	suite.Nil(client)
	suite.Contains(err.Error(), "invalid HTTP header format")
}
