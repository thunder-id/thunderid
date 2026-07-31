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
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

type SMTPEmailClientTestSuite struct {
	suite.Suite
}

func TestSMTPEmailClientTestSuite(t *testing.T) {
	suite.Run(t, new(SMTPEmailClientTestSuite))
}

func (suite *SMTPEmailClientTestSuite) SetupSuite() {
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

func (suite *SMTPEmailClientTestSuite) TestNewSMTPEmailClient_Valid() {
	props := []cmodels.Property{
		createTestProperty(common.SMTPPropKeyHost, "smtp.example.com", false),
		createTestProperty(common.SMTPPropKeyPort, "587", false),
		createTestProperty(common.SMTPPropKeyUsername, "testuser", false),
		createTestProperty(common.SMTPPropKeyPassword, "testpass", true),
		createTestProperty(common.SMTPPropKeyFromAddress, "no-reply@example.com", false),
		createTestProperty(common.SMTPPropKeyTLS, string(common.TLSModeSTARTTLS), false),
		createTestProperty(common.SMTPPropKeyEnableAuth, "true", false),
	}

	sender := common.NotificationSenderDTO{
		Name:       "Test SMTP",
		Provider:   common.NotificationProviderTypeSMTP,
		Type:       common.NotificationSenderTypeEmail,
		Properties: props,
	}

	client, err := newSMTPEmailClient(context.Background(), sender)

	suite.NoError(err)
	suite.NotNil(client)
	suite.Equal("Test SMTP", client.GetName())
	suite.Equal(common.TLSModeSTARTTLS, client.(*smtpEmailClient).config.tlsMode)
}

func (suite *SMTPEmailClientTestSuite) TestNewSMTPEmailClient_ValidImplicitTLS() {
	props := []cmodels.Property{
		createTestProperty(common.SMTPPropKeyHost, "smtp.example.com", false),
		createTestProperty(common.SMTPPropKeyPort, "465", false),
		createTestProperty(common.SMTPPropKeyUsername, "testuser", false),
		createTestProperty(common.SMTPPropKeyPassword, "testpass", true),
		createTestProperty(common.SMTPPropKeyFromAddress, "no-reply@example.com", false),
		createTestProperty(common.SMTPPropKeyTLS, string(common.TLSModeImplicit), false),
		createTestProperty(common.SMTPPropKeyEnableAuth, "true", false),
	}

	sender := common.NotificationSenderDTO{
		Name:       "Test SMTP",
		Provider:   common.NotificationProviderTypeSMTP,
		Type:       common.NotificationSenderTypeEmail,
		Properties: props,
	}

	client, err := newSMTPEmailClient(context.Background(), sender)

	suite.NoError(err)
	suite.NotNil(client)
	suite.Equal("Test SMTP", client.GetName())
	suite.Equal(common.TLSModeImplicit, client.(*smtpEmailClient).config.tlsMode)
}

func (suite *SMTPEmailClientTestSuite) TestNewSMTPEmailClient_ValidNoneTLS() {
	props := []cmodels.Property{
		createTestProperty(common.SMTPPropKeyHost, "smtp.example.com", false),
		createTestProperty(common.SMTPPropKeyPort, "25", false),
		createTestProperty(common.SMTPPropKeyFromAddress, "no-reply@example.com", false),
		// Intentionally omitting TLS and Auth keys to test plaintext
	}

	sender := common.NotificationSenderDTO{
		Name:       "Test SMTP Plaintext",
		Provider:   common.NotificationProviderTypeSMTP,
		Type:       common.NotificationSenderTypeEmail,
		Properties: props,
	}

	client, err := newSMTPEmailClient(context.Background(), sender)

	suite.NoError(err)
	suite.NotNil(client)
	suite.Equal("Test SMTP Plaintext", client.GetName())
	suite.Equal(common.TLSModeNone, client.(*smtpEmailClient).config.tlsMode)
}

func (suite *SMTPEmailClientTestSuite) TestNewSMTPEmailClient_InvalidConfig() {
	cases := []struct {
		name        string
		props       []cmodels.Property
		errContains string
	}{
		{
			name: "missing host",
			props: []cmodels.Property{
				createTestProperty(common.SMTPPropKeyPort, "587", false),
				createTestProperty(common.SMTPPropKeyFromAddress, "test@example.com", false),
			},
			errContains: "host is missing",
		},
		{
			name: "missing from address",
			props: []cmodels.Property{
				createTestProperty(common.SMTPPropKeyHost, "smtp.example.com", false),
				createTestProperty(common.SMTPPropKeyPort, "587", false),
			},
			errContains: "from_address is missing",
		},
		{
			name: "invalid port",
			props: []cmodels.Property{
				createTestProperty(common.SMTPPropKeyHost, "smtp.example.com", false),
				createTestProperty(common.SMTPPropKeyPort, "invalid", false),
				createTestProperty(common.SMTPPropKeyFromAddress, "test@example.com", false),
			},
			errContains: "port is invalid",
		},
		{
			name: "missing TLS when auth enabled",
			props: []cmodels.Property{
				createTestProperty(common.SMTPPropKeyHost, "smtp.example.com", false),
				createTestProperty(common.SMTPPropKeyPort, "587", false),
				createTestProperty(common.SMTPPropKeyFromAddress, "test@example.com", false),
				createTestProperty(common.SMTPPropKeyEnableAuth, "true", false),
			},
			errContains: "TLS must be enabled",
		},
		{
			name: "missing credentials when auth enabled",
			props: []cmodels.Property{
				createTestProperty(common.SMTPPropKeyHost, "smtp.example.com", false),
				createTestProperty(common.SMTPPropKeyPort, "587", false),
				createTestProperty(common.SMTPPropKeyFromAddress, "test@example.com", false),
				createTestProperty(common.SMTPPropKeyEnableAuth, "true", false),
				createTestProperty(common.SMTPPropKeyTLS, string(common.TLSModeSTARTTLS), false),
			},
			errContains: "credentials are required",
		},
	}

	for _, tc := range cases {
		suite.T().Run(tc.name, func(t *testing.T) {
			sender := common.NotificationSenderDTO{
				Name:       "Test SMTP",
				Provider:   common.NotificationProviderTypeSMTP,
				Type:       common.NotificationSenderTypeEmail,
				Properties: tc.props,
			}

			client, err := newSMTPEmailClient(context.Background(), sender)

			suite.Error(err)
			suite.Nil(client)
			suite.Contains(err.Error(), tc.errContains)
		})
	}
}

func (suite *SMTPEmailClientTestSuite) TestSend_EmptyRecipient() {
	props := []cmodels.Property{
		createTestProperty(common.SMTPPropKeyHost, "smtp.example.com", false),
		createTestProperty(common.SMTPPropKeyPort, "587", false),
		createTestProperty(common.SMTPPropKeyFromAddress, "no-reply@example.com", false),
	}

	sender := common.NotificationSenderDTO{
		Name:       "Test SMTP",
		Provider:   common.NotificationProviderTypeSMTP,
		Type:       common.NotificationSenderTypeEmail,
		Properties: props,
	}

	client, err := newSMTPEmailClient(context.Background(), sender)
	suite.NoError(err)

	emailData := common.EmailData{
		To:      []string{""},
		Subject: "Test",
		Body:    "Test Body",
	}

	err = client.Send(context.Background(), emailData)
	suite.Error(err)
	suite.Contains(err.Error(), "recipient address cannot be empty")
}

func (suite *SMTPEmailClientTestSuite) TestSend_InvalidSubject() {
	props := []cmodels.Property{
		createTestProperty(common.SMTPPropKeyHost, "smtp.example.com", false),
		createTestProperty(common.SMTPPropKeyPort, "587", false),
		createTestProperty(common.SMTPPropKeyFromAddress, "no-reply@example.com", false),
	}

	sender := common.NotificationSenderDTO{
		Name:       "Test SMTP",
		Provider:   common.NotificationProviderTypeSMTP,
		Type:       common.NotificationSenderTypeEmail,
		Properties: props,
	}

	client, err := newSMTPEmailClient(context.Background(), sender)
	suite.NoError(err)

	emailData := common.EmailData{
		To:      []string{"test@example.com"},
		Subject: "Test\r\nInjected-Header: bad",
		Body:    "Test Body",
	}

	err = client.Send(context.Background(), emailData)
	suite.Error(err)
	suite.Contains(err.Error(), "subject contains invalid characters")
}

func (suite *SMTPEmailClientTestSuite) TestSend_InvalidRecipient() {
	props := []cmodels.Property{
		createTestProperty(common.SMTPPropKeyHost, "smtp.example.com", false),
		createTestProperty(common.SMTPPropKeyPort, "587", false),
		createTestProperty(common.SMTPPropKeyFromAddress, "no-reply@example.com", false),
	}

	sender := common.NotificationSenderDTO{
		Name:       "Test SMTP",
		Provider:   common.NotificationProviderTypeSMTP,
		Type:       common.NotificationSenderTypeEmail,
		Properties: props,
	}

	client, err := newSMTPEmailClient(context.Background(), sender)
	suite.NoError(err)

	emailData := common.EmailData{
		To:      []string{"test@example.com\r\nBcc: evil@example.com"},
		Subject: "Test",
		Body:    "Test Body",
	}

	err = client.Send(context.Background(), emailData)
	suite.Error(err)
	suite.Contains(err.Error(), "recipient address contains invalid characters")

	// Test CC
	emailData.To = []string{"test@example.com"}
	emailData.CC = []string{"cc@example.com\r\nBcc: evil@example.com"}
	err = client.Send(context.Background(), emailData)
	suite.Error(err)
	suite.Contains(err.Error(), "recipient address contains invalid characters")
}

func (suite *SMTPEmailClientTestSuite) TestBuildMessage_RecipientCombinations() {
	client := &smtpEmailClient{
		config: smtpConfig{
			from: "sender@example.com",
		},
	}

	testCases := []struct {
		name        string
		emailData   common.EmailData
		expected    []string
		notExpected []string
	}{
		{
			name: "Only TO",
			emailData: common.EmailData{
				To:      []string{"to1@example.com", "to2@example.com"},
				Subject: "Test",
				Body:    "Body",
			},
			expected: []string{
				"To: to1@example.com, to2@example.com\r\n",
				"From: sender@example.com\r\n",
			},
			notExpected: []string{"Cc:", "Bcc:"},
		},
		{
			name: "TO and CC",
			emailData: common.EmailData{
				To:      []string{"to1@example.com"},
				CC:      []string{"cc1@example.com", "cc2@example.com"},
				Subject: "Test",
				Body:    "Body",
			},
			expected: []string{
				"To: to1@example.com\r\n",
				"Cc: cc1@example.com, cc2@example.com\r\n",
			},
			notExpected: []string{"Bcc:"},
		},
		{
			name: "TO and BCC",
			emailData: common.EmailData{
				To:      []string{"to1@example.com"},
				BCC:     []string{"bcc1@example.com"},
				Subject: "Test",
				Body:    "Body",
			},
			expected: []string{
				"To: to1@example.com\r\n",
			},
			notExpected: []string{"Cc:", "Bcc:"},
		},
		{
			name: "TO, CC, and BCC",
			emailData: common.EmailData{
				To:      []string{"to1@example.com"},
				CC:      []string{"cc1@example.com"},
				BCC:     []string{"bcc1@example.com", "bcc2@example.com"},
				Subject: "Test",
				Body:    "Body",
			},
			expected: []string{
				"To: to1@example.com\r\n",
				"Cc: cc1@example.com\r\n",
			},
			notExpected: []string{"Bcc:"},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			msg := client.buildMessage(tc.emailData)

			for _, exp := range tc.expected {
				suite.Contains(msg, exp)
			}
			for _, nexp := range tc.notExpected {
				suite.NotContains(msg, nexp)
			}
		})
	}
}
