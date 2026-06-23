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
)

type SMTPEmailClientTestSuite struct {
	suite.Suite
}

func TestSMTPEmailClientTestSuite(t *testing.T) {
	suite.Run(t, new(SMTPEmailClientTestSuite))
}

func (suite *SMTPEmailClientTestSuite) TestNewSMTPEmailClient_Valid() {
	props := []cmodels.Property{
		createTestProperty(common.SMTPPropKeyHost, "smtp.example.com", false),
		createTestProperty(common.SMTPPropKeyPort, "587", false),
		createTestProperty(common.SMTPPropKeyUsername, "testuser", false),
		createTestProperty(common.SMTPPropKeyPassword, "testpass", true),
		createTestProperty(common.SMTPPropKeyFromAddress, "no-reply@example.com", false),
		createTestProperty(common.SMTPPropKeyEnableStartTLS, "true", false),
		createTestProperty(common.SMTPPropKeyEnableAuth, "true", false),
	}

	sender := common.NotificationSenderDTO{
		Name:       "Test SMTP",
		Provider:   common.MessageProviderTypeSMTP,
		Type:       common.NotificationSenderTypeEmail,
		Properties: props,
	}

	client, err := newSMTPEmailClient(context.Background(), sender)

	suite.NoError(err)
	suite.NotNil(client)
	suite.Equal("Test SMTP", client.GetName())
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
			name: "missing credentials when auth enabled",
			props: []cmodels.Property{
				createTestProperty(common.SMTPPropKeyHost, "smtp.example.com", false),
				createTestProperty(common.SMTPPropKeyPort, "587", false),
				createTestProperty(common.SMTPPropKeyFromAddress, "test@example.com", false),
				createTestProperty(common.SMTPPropKeyEnableAuth, "true", false),
			},
			errContains: "credentials are required",
		},
	}

	for _, tc := range cases {
		suite.T().Run(tc.name, func(t *testing.T) {
			sender := common.NotificationSenderDTO{
				Name:       "Test SMTP",
				Provider:   common.MessageProviderTypeSMTP,
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
		Provider:   common.MessageProviderTypeSMTP,
		Type:       common.NotificationSenderTypeEmail,
		Properties: props,
	}

	client, err := newSMTPEmailClient(context.Background(), sender)
	suite.NoError(err)

	emailData := common.EmailData{
		Recipient: "",
		Subject:   "Test",
		Body:      "Test Body",
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
		Provider:   common.MessageProviderTypeSMTP,
		Type:       common.NotificationSenderTypeEmail,
		Properties: props,
	}

	client, err := newSMTPEmailClient(context.Background(), sender)
	suite.NoError(err)

	emailData := common.EmailData{
		Recipient: "user@example.com",
		Subject:   "Test\r\nInjected-Header: bad",
		Body:      "Test Body",
	}

	err = client.Send(context.Background(), emailData)
	suite.Error(err)
	suite.Contains(err.Error(), "subject contains invalid characters")
}
