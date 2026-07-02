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

package common

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ValidatorTestSuite struct {
	suite.Suite
}

func TestValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(ValidatorTestSuite))
}

func (suite *ValidatorTestSuite) TestNotificationProviderType_Valid() {
	cases := []struct {
		name     string
		provider NotificationProviderType
		expected bool
	}{
		{"Vonage", NotificationProviderTypeVonage, true},
		{"Twilio", NotificationProviderTypeTwilio, true},
		{"Custom", NotificationProviderTypeCustom, true},
		{"SMTP", NotificationProviderTypeSMTP, true},
		{"HTTP", NotificationProviderTypeHTTP, true},
		{"Invalid", NotificationProviderType("invalid"), false},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.Equal(tc.expected, tc.provider.Valid())
		})
	}
}

func (suite *ValidatorTestSuite) TestNotificationSenderType_Valid() {
	cases := []struct {
		name     string
		sender   NotificationSenderType
		expected bool
	}{
		{"Message", NotificationSenderTypeMessage, true},
		{"Email", NotificationSenderTypeEmail, true},
		{"Invalid", NotificationSenderType("invalid"), false},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.Equal(tc.expected, tc.sender.Valid())
		})
	}
}

func (suite *ValidatorTestSuite) TestEmailData_Validate() {
	cases := []struct {
		name        string
		emailData   *EmailData
		expectError bool
		errorString string
		checkFunc   func(*EmailData) // Optional function to verify trimmed data
	}{
		{
			name: "Success with trimming",
			emailData: &EmailData{
				To:      []string{"user1@example.com", " user2@example.com "},
				CC:      []string{" cc@example.com "},
				BCC:     []string{"bcc@example.com"},
				Subject: " Test Subject ",
			},
			expectError: false,
			checkFunc: func(e *EmailData) {
				suite.Equal("user2@example.com", e.To[1])
				suite.Equal("cc@example.com", e.CC[0])
				suite.Equal("Test Subject", e.Subject)
			},
		},
		{
			name: "Empty To array",
			emailData: &EmailData{
				To:      []string{},
				Subject: "Test",
			},
			expectError: true,
			errorString: "recipient address cannot be empty",
		},
		{
			name: "Empty To address",
			emailData: &EmailData{
				To:      []string{""},
				Subject: "Test",
			},
			expectError: true,
			errorString: "recipient address cannot be empty",
		},
		{
			name: "Invalid characters in To",
			emailData: &EmailData{
				To:      []string{"user@\r\nexample.com"},
				Subject: "Test",
			},
			expectError: true,
			errorString: "recipient address contains invalid characters",
		},
		{
			name: "Invalid characters in Subject",
			emailData: &EmailData{
				To:      []string{"user@example.com"},
				Subject: "Test\r\nSubject",
			},
			expectError: true,
			errorString: "subject contains invalid characters",
		},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			err := tc.emailData.Validate()
			if tc.expectError {
				suite.Error(err)
				if err != nil {
					suite.Contains(err.Error(), tc.errorString)
				}
			} else {
				suite.NoError(err)
				if tc.checkFunc != nil {
					tc.checkFunc(tc.emailData)
				}
			}
		})
	}
}
