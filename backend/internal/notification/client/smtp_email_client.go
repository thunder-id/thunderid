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
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	smtpLoggerComponentName = "SMTPEmailClient"
	smtpDialTimeout         = 30 * time.Second
)

// smtpConfig holds the configuration for the SMTP client.
type smtpConfig struct {
	host                 string
	port                 int
	username             string
	password             string
	from                 string
	useTLS               bool
	enableAuthentication bool
}

// SMTPEmailClient implements the EmailClientInterface using SMTP.
type SMTPEmailClient struct {
	name   string
	config smtpConfig
}

// newSMTPEmailClient creates a new instance of SMTPEmailClient.
func newSMTPEmailClient(ctx context.Context, sender common.NotificationSenderDTO) (EmailClientInterface, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, smtpLoggerComponentName))

	config := smtpConfig{}

	for _, prop := range sender.Properties {
		val, err := prop.GetValue()
		if err != nil {
			return nil, fmt.Errorf("failed to get property value for %s: %w", prop.GetName(), err)
		}

		switch prop.GetName() {
		case common.SMTPPropKeyHost:
			config.host = strings.TrimSpace(val)
		case common.SMTPPropKeyPort:
			if port, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
				config.port = port
			}
		case common.SMTPPropKeyUsername:
			config.username = strings.TrimSpace(val)
		case common.SMTPPropKeyPassword:
			config.password = val
		case common.SMTPPropKeyFromAddress:
			config.from = strings.TrimSpace(val)
		case common.SMTPPropKeyEnableStartTLS:
			config.useTLS = strings.TrimSpace(strings.ToLower(val)) == "true"
		case common.SMTPPropKeyEnableAuth:
			config.enableAuthentication = strings.TrimSpace(strings.ToLower(val)) == "true"
		default:
			logger.Warn(ctx, "Unknown property for SMTP Email client", log.String("property", prop.GetName()))
		}
	}

	if config.from == "" {
		return nil, errors.New("from_address is missing")
	}
	if config.host == "" {
		return nil, errors.New("host is missing")
	}
	if config.port <= 0 {
		return nil, errors.New("port is invalid")
	}
	if config.enableAuthentication {
		if config.username == "" || config.password == "" {
			return nil, errors.New("credentials are required when authentication is enabled")
		}
	}

	return &SMTPEmailClient{
		name:   sender.Name,
		config: config,
	}, nil
}

// GetName returns the name of the SMTP email client.
func (c *SMTPEmailClient) GetName() string {
	return c.name
}

// Send dispatches an email notification via SMTP.
func (c *SMTPEmailClient) Send(ctx context.Context, emailData common.EmailData) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, smtpLoggerComponentName))

	recipient := strings.TrimSpace(emailData.Recipient)
	if recipient == "" {
		return errors.New("recipient address cannot be empty")
	}

	if strings.ContainsAny(emailData.Subject, "\r\n") {
		return errors.New("subject contains invalid characters")
	}

	logger.Debug(ctx, "Sending email via SMTP", log.MaskedString("from", c.config.from))

	serverAddress := fmt.Sprintf("%s:%d", c.config.host, c.config.port)

	message := c.buildMessage(emailData, recipient)

	if err := c.sendViaSMTP(ctx, serverAddress, []string{recipient}, message); err != nil {
		return err
	}

	logger.Debug(ctx, "Email sent successfully")
	return nil
}

// buildMessage constructs the raw email message string with headers and body.
func (c *SMTPEmailClient) buildMessage(emailData common.EmailData, recipient string) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("From: %s\r\n", c.config.from))
	builder.WriteString(fmt.Sprintf("To: %s\r\n", recipient))
	// TODO: Support CC and BCC recipients in the future.
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
// optional TLS upgrade, authentication, and message transmission.
func (c *SMTPEmailClient) sendViaSMTP(ctx context.Context, serverAddress string, recipients []string, message string) error {
	conn, err := net.DialTimeout("tcp", serverAddress, smtpDialTimeout)
	if err != nil {
		return fmt.Errorf("smtp connection failed: %w", err)
	}

	client, err := smtp.NewClient(conn, c.config.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp connection failed: %w", err)
	}
	defer func() {
		_ = client.Close()
	}()

	if c.config.useTLS {
		ok, _ := client.Extension("STARTTLS")
		if !ok {
			return errors.New("smtp connection failed: STARTTLS not supported by server")
		}
		tlsConfig := &tls.Config{
			ServerName: c.config.host,
			MinVersion: tls.VersionTLS12,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp connection failed: %w", err)
		}
	}

	if c.config.enableAuthentication && c.config.username != "" && c.config.password != "" {
		if err := client.Auth(smtp.PlainAuth("", c.config.username, c.config.password, c.config.host)); err != nil {
			return fmt.Errorf("smtp authentication failed: %w", err)
		}
	}

	if err := client.Mail(c.config.from); err != nil {
		return fmt.Errorf("email send failed: %w", err)
	}

	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("email send failed: %w", err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("email send failed: %w", err)
	}
	if _, err := writer.Write([]byte(message)); err != nil {
		return fmt.Errorf("email send failed: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("email send failed: %w", err)
	}

	if err := client.Quit(); err != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, smtpLoggerComponentName)).
			Error(ctx, "Failed to gracefully close SMTP client", log.Error(err))
	}

	return nil
}
