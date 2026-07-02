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
	smtpDialTimeout = 30 * time.Second
)

// smtpConfig holds the configuration for the SMTP client.
type smtpConfig struct {
	host                 string
	port                 int
	username             string
	password             string
	from                 string
	tlsMode              common.TLSMode
	enableAuthentication bool
}

// smtpEmailClient implements the EmailClientInterface using SMTP.
type smtpEmailClient struct {
	name   string
	config smtpConfig
	logger *log.Logger
}

// newSMTPEmailClient creates a new instance of smtpEmailClient.
func newSMTPEmailClient(ctx context.Context, sender common.NotificationSenderDTO) (EmailClientInterface, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SMTPEmailClient"))

	config, err := parseConfig(ctx, sender, logger)
	if err != nil {
		return nil, err
	}

	return &smtpEmailClient{
		name:   sender.Name,
		config: config,
		logger: logger,
	}, nil
}

// parseConfig parses the configuration for the SMTP client.
func parseConfig(ctx context.Context, sender common.NotificationSenderDTO, logger *log.Logger) (smtpConfig, error) {
	config := smtpConfig{
		tlsMode: common.TLSModeNone,
	}
	for _, prop := range sender.Properties {
		val, err := prop.GetValue()
		if err != nil {
			return config, fmt.Errorf("failed to get property value for %s: %w", prop.GetName(), err)
		}

		switch prop.GetName() {
		case common.SMTPPropKeyHost:
			config.host = strings.TrimSpace(val)
		case common.SMTPPropKeyPort:
			port, err := strconv.Atoi(strings.TrimSpace(val))
			if err != nil || port <= 0 {
				return config, errors.New("port is invalid")
			}
			config.port = port
		case common.SMTPPropKeyUsername:
			config.username = strings.TrimSpace(val)
		case common.SMTPPropKeyPassword:
			config.password = val
		case common.SMTPPropKeyFromAddress:
			config.from = strings.TrimSpace(val)
		case common.SMTPPropKeyTLS:
			switch strings.TrimSpace(strings.ToLower(val)) {
			case string(common.TLSModeSTARTTLS):
				config.tlsMode = common.TLSModeSTARTTLS
			case string(common.TLSModeImplicit):
				config.tlsMode = common.TLSModeImplicit
			default:
				config.tlsMode = common.TLSModeNone
			}
		case common.SMTPPropKeyEnableAuth:
			config.enableAuthentication = strings.TrimSpace(strings.ToLower(val)) == "true"
		case common.SenderPropertySupportedChannels:
			// Ignored here as it is a generic sender property
		default:
			logger.Warn(ctx, "Unknown property for SMTP Email client", log.String("property", prop.GetName()))
		}
	}

	if config.from == "" {
		return config, errors.New("from_address is missing")
	}
	if config.host == "" {
		return config, errors.New("host is missing")
	}
	if config.port == 0 {
		return config, errors.New("port is invalid")
	}

	if config.enableAuthentication {
		if config.tlsMode == common.TLSModeNone {
			return config, errors.New("TLS must be enabled when authentication is enabled")
		}
		if config.username == "" || config.password == "" {
			return config, errors.New("credentials are required when authentication is enabled")
		}
	}

	return config, nil
}

// GetName returns the name of the SMTP email client.
func (c *smtpEmailClient) GetName() string {
	return c.name
}

// Send dispatches an email notification via SMTP.
func (c *smtpEmailClient) Send(ctx context.Context, emailData common.EmailData) error {
	if err := emailData.Validate(); err != nil {
		return err
	}

	if strings.ContainsAny(c.config.from, common.CRLF) {
		return errors.New("from address contains invalid characters")
	}

	c.logger.Debug(ctx, "Sending email via SMTP", log.MaskedString("from", c.config.from))

	serverAddress := fmt.Sprintf("%s:%d", c.config.host, c.config.port)

	message := c.buildMessage(emailData)

	allRecipients := append([]string{}, emailData.To...)
	allRecipients = append(allRecipients, emailData.CC...)
	allRecipients = append(allRecipients, emailData.BCC...)

	if err := c.sendViaSMTP(ctx, serverAddress, allRecipients, message); err != nil {
		return err
	}

	c.logger.Debug(ctx, "Email sent successfully")
	return nil
}

// buildMessage constructs the raw email message string with headers and body.
func (c *smtpEmailClient) buildMessage(emailData common.EmailData) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("From: %s"+common.CRLF, c.config.from))
	builder.WriteString(fmt.Sprintf("To: %s"+common.CRLF, strings.Join(emailData.To, ", ")))
	if len(emailData.CC) > 0 {
		builder.WriteString(fmt.Sprintf("Cc: %s"+common.CRLF, strings.Join(emailData.CC, ", ")))
	}
	builder.WriteString(fmt.Sprintf("Subject: %s"+common.CRLF, mime.QEncoding.Encode("utf-8", emailData.Subject)))
	builder.WriteString("MIME-Version: 1.0" + common.CRLF)

	if emailData.IsHTML {
		builder.WriteString("Content-Type: text/html; charset=\"utf-8\"" + common.CRLF)
	} else {
		builder.WriteString("Content-Type: text/plain; charset=\"utf-8\"" + common.CRLF)
	}

	builder.WriteString(common.CRLF)
	builder.WriteString(emailData.Body)

	return builder.String()
}

// sendViaSMTP handles the low-level SMTP communication, connection setup,
// optional TLS upgrade, authentication, and message transmission.
func (c *smtpEmailClient) sendViaSMTP(
	ctx context.Context, serverAddress string, recipients []string, message string) error {
	var conn net.Conn
	var err error

	if c.config.tlsMode == common.TLSModeImplicit {
		tlsConfig := &tls.Config{
			ServerName: c.config.host,
			MinVersion: tls.VersionTLS12,
		}
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: smtpDialTimeout}, "tcp", serverAddress, tlsConfig)
	} else {
		conn, err = net.DialTimeout("tcp", serverAddress, smtpDialTimeout)
	}

	if err != nil {
		return fmt.Errorf("smtp connection failed: %w", err)
	}

	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			c.logger.Error(ctx, "Failed to close connection", log.Error(closeErr))
		}
	}()

	client, err := smtp.NewClient(conn, c.config.host)
	if err != nil {
		return fmt.Errorf("smtp connection failed: %w", err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			c.logger.Error(ctx, "Failed to force close SMTP client", log.Error(closeErr))
		}
	}()

	if c.config.tlsMode == common.TLSModeSTARTTLS {
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
		if c.config.tlsMode == common.TLSModeNone {
			return errors.New("smtp connection failed: TLS must be enabled when authentication is enabled")
		}
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
		c.logger.Error(ctx, "Failed to gracefully close SMTP client", log.Error(err))
	}

	return nil
}
