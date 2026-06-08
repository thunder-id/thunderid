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

package email

import "errors"

// Client errors for email service
var (
	// ErrorInvalidRecipient is returned when the email has no valid recipients.
	ErrorInvalidRecipient = errors.New("invalid recipient: the email must have at least one valid recipient address")
	// ErrorInvalidSender is returned when the sender (From) address is empty.
	ErrorInvalidSender = errors.New("invalid sender: the sender email address is invalid or empty")
	// ErrorInvalidSubject is returned when the email subject contains invalid characters.
	ErrorInvalidSubject = errors.New("invalid subject: the email subject contains invalid characters")
	// ErrorInvalidHost is returned when the SMTP host is empty.
	ErrorInvalidHost = errors.New("invalid host: the SMTP host cannot be empty")
	// ErrorInvalidPort is returned when the SMTP port is zero or negative.
	ErrorInvalidPort = errors.New("invalid port: the SMTP port must be greater than zero")
	// ErrorInvalidCredentials is returned when the SMTP username or password is empty but authentication is enabled.
	ErrorInvalidCredentials = errors.New("invalid credentials: username and password cannot be empty " +
		"when authentication is enabled")
	// ErrorUnreachableOrigin is returned when the SMTP origin cannot be reached using a TCP dial.
	ErrorUnreachableOrigin = errors.New("unreachable origin: unable to reach SMTP origin using TCP dial")
)

// Server errors for email service
var (
	// ErrorSMTPConnection is returned when the SMTP connection cannot be established.
	ErrorSMTPConnection = errors.New("smtp connection failed")
	// ErrorSMTPAuth is returned when SMTP authentication fails.
	ErrorSMTPAuth = errors.New("smtp authentication failed")
	// ErrorEmailSendFailed is returned when the email fails to send.
	ErrorEmailSendFailed = errors.New("email sending failed")
)
