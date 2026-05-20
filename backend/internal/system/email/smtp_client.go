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

import (
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	smtpLoggerComponentName = "SMTPEmailClient"
	smtpDialTimeout         = 30 * time.Second
)

// The newSMTPClient creates a new instance of smtpClient.
// It validates the configuration at creation time to avoid runtime errors.
func newSMTPClient(config smtpConfig) (EmailClientInterface, error) {
	sender := strings.TrimSpace(config.from)
	if sender == "" {
		return nil, ErrorInvalidSender
	}
	if !IsValidEmail(sender) {
		return nil, ErrorInvalidSender
	}
	config.from = sender
	if strings.TrimSpace(config.host) == "" {
		return nil, ErrorInvalidHost
	}
	if config.port <= 0 {
		return nil, ErrorInvalidPort
	}
	if config.enableAuthentication {
		if strings.TrimSpace(config.username) == "" || strings.TrimSpace(config.password) == "" {
			return nil, ErrorInvalidCredentials
		}
	}
	return &smtpClient{
		config: config,
	}, nil
}

// NewSMTPClientFromConfig creates a new smtpClient using the global server configuration.
// It reads the email.smtp section from the server runtime config.
// Returns an error if the configuration is invalid (e.g., missing sender address)
// or if the runtime is not initialized.
func NewSMTPClientFromConfig() (EmailClientInterface, error) {
	emailConfig := config.GetServerRuntime().Config.Email.SMTP

	enableStartTLS := true
	if emailConfig.EnableStartTLS != nil {
		enableStartTLS = *emailConfig.EnableStartTLS
	}

	enableAuth := true
	if emailConfig.EnableAuthentication != nil {
		enableAuth = *emailConfig.EnableAuthentication
	}

	return newSMTPClient(smtpConfig{
		host:                 emailConfig.Host,
		port:                 emailConfig.Port,
		username:             emailConfig.Username,
		password:             emailConfig.Password,
		from:                 emailConfig.FromAddress,
		useTLS:               enableStartTLS,
		enableAuthentication: enableAuth,
	})
}

// smtpClient implements the EmailClientInterface using SMTP.
func (c *smtpClient) Send(emailData EmailData) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, smtpLoggerComponentName))

	// 1. Validate, sanitize in place, and extract the flat envelope list
	allRecipients, err := c.validateAndProcessRecipients(&emailData)
	if err != nil {
		return err
	}

	logger.Debug("Sending email via SMTP",
		log.MaskedString("from", c.config.from),
		log.Int("recipientCount", len(emailData.To)))

	serverAddress := fmt.Sprintf("%s:%d", c.config.host, c.config.port)

	// 2. Build the message headers (now using the trimmed emailData.To and emailData.CC arrays)
	message := c.buildMessage(emailData)

	// 3. Send via SMTP
	if err := c.sendViaSMTP(serverAddress, allRecipients, message); err != nil {
		return err
	}

	logger.Debug("Email sent successfully")
	return nil
}

// validateAndProcessRecipients validates the recipient email addresses in the To, CC, and BCC fields.
func (c *smtpClient) validateAndProcessRecipients(emailData *EmailData) ([]string, error) {
	var allRecipients []string
	hasRecipient := false

	// Inline helper to validate and clean a specific group of addresses
	processGroup := func(addresses []string) ([]string, error) {
		var cleaned []string
		for _, address := range addresses {
			trimmed := strings.TrimSpace(address)
			if trimmed == "" {
				return nil, fmt.Errorf("%w: recipient address cannot be empty", ErrorInvalidRecipient)
			}
			if !IsValidEmail(trimmed) {
				return nil, fmt.Errorf("%w: invalid recipient address '%s'", ErrorInvalidRecipient, trimmed)
			}
			cleaned = append(cleaned, trimmed)
			allRecipients = append(allRecipients, trimmed)
			hasRecipient = true
		}
		return cleaned, nil
	}

	var err error
	if emailData.To, err = processGroup(emailData.To); err != nil {
		return nil, err
	}
	if emailData.CC, err = processGroup(emailData.CC); err != nil {
		return nil, err
	}
	if emailData.BCC, err = processGroup(emailData.BCC); err != nil {
		return nil, err
	}

	if !hasRecipient {
		return nil, ErrorInvalidRecipient
	}

	// Reject CR/LF in Subject to prevent header injection.
	if strings.ContainsAny(emailData.Subject, "\r\n") {
		return nil, ErrorInvalidSubject
	}

	return allRecipients, nil
}

// buildMessage constructs the raw email message string with headers and body.
func (c *smtpClient) buildMessage(emailData EmailData) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("From: %s\r\n", c.config.from))

	if len(emailData.To) > 0 {
		builder.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(emailData.To, ", ")))
	} else {
		builder.WriteString("To: undisclosed-recipients:;\r\n")
	}

	if len(emailData.CC) > 0 {
		builder.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(emailData.CC, ", ")))
	}

	builder.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("utf-8", emailData.Subject)))
	builder.WriteString("MIME-Version: 1.0\r\n")

	if emailData.IsHTML {
		builder.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	} else {
		builder.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	}

	builder.WriteString("\r\n")
	builder.WriteString(emailData.Body)

	return builder.String()
}

// sendViaSMTP handles the low-level SMTP communication, connection setup,
// optional TLS upgrade, authentication, and message transmission
func (c *smtpClient) sendViaSMTP(serverAddress string, recipients []string, message string) error {
	conn, err := net.DialTimeout("tcp", serverAddress, smtpDialTimeout)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrorSMTPConnection, err)
	}

	client, err := smtp.NewClient(conn, c.config.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("%w: %w", ErrorSMTPConnection, err)
	}
	defer func() {
		_ = client.Close()
	}()

	if c.config.useTLS {
		ok, _ := client.Extension("STARTTLS")
		if !ok {
			return fmt.Errorf("%w: STARTTLS not supported by server", ErrorSMTPConnection)
		}
		tlsConfig := &tls.Config{
			ServerName: c.config.host,
			MinVersion: tls.VersionTLS12,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("%w: %w", ErrorSMTPConnection, err)
		}
	}

	if c.config.enableAuthentication && c.config.username != "" && c.config.password != "" {
		if err := client.Auth(smtp.PlainAuth("", c.config.username, c.config.password, c.config.host)); err != nil {
			return fmt.Errorf("%w: %w", ErrorSMTPAuth, err)
		}
	}

	if err := client.Mail(c.config.from); err != nil {
		return fmt.Errorf("%w: %w", ErrorEmailSendFailed, err)
	}

	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("%w: %w", ErrorEmailSendFailed, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrorEmailSendFailed, err)
	}
	if _, err := writer.Write([]byte(message)); err != nil {
		return fmt.Errorf("%w: %w", ErrorEmailSendFailed, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("%w: %w", ErrorEmailSendFailed, err)
	}

	if err := client.Quit(); err != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, smtpLoggerComponentName)).
			Error("Failed to gracefully close SMTP client", log.Error(err))
	}

	return nil
}
