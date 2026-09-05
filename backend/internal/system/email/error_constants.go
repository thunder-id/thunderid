// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
