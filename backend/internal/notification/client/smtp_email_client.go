// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/smtpauth"
)

const (
	smtpLoggerComponentName = "SMTPEmailClient"
	// smtpDialTimeout bounds establishing the TCP (and, for implicit TLS, the handshake).
	smtpDialTimeout = 30 * time.Second
)

// smtpSessionTimeout bounds the whole SMTP conversation after the dial. Without it a server
// that accepts the connection and then stalls blocks the calling goroutine indefinitely, since
// net/smtp sets no deadlines of its own. A variable so tests can shorten it.
var smtpSessionTimeout = 60 * time.Second

// smtpConfig holds the resolved configuration for an SMTP email client.
type smtpConfig struct {
	host     string
	port     int
	from     string
	fromName string
	tlsMode  common.TLSMode
	auth     outboundauth.Config
}

// smtpEmailClient implements EmailClientInterface over SMTP.
type smtpEmailClient struct {
	name   string
	config smtpConfig
	auth   smtpauth.Authenticator
	logger *log.Logger
}

// newSMTPEmailClient creates a new instance of smtpEmailClient.
func newSMTPEmailClient(ctx context.Context, sender common.NotificationSenderDTO) (EmailClientInterface, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, smtpLoggerComponentName))

	config, err := parseSMTPConfig(ctx, sender, logger)
	if err != nil {
		return nil, err
	}

	authenticator, err := smtpauth.New(config.auth)
	if err != nil {
		return nil, err
	}

	return &smtpEmailClient{
		name:   sender.Name,
		config: config,
		auth:   authenticator,
		logger: logger,
	}, nil
}

// parseSMTPConfig reads the SMTP configuration out of the sender properties. The sender was
// already validated on write, so anything rejected here is a stored value that no longer
// parses.
func parseSMTPConfig(ctx context.Context, sender common.NotificationSenderDTO,
	logger *log.Logger) (smtpConfig, error) {
	config := smtpConfig{tlsMode: common.TLSModeSTARTTLS}

	authConfig, err := outboundauth.FromProperties(sender.Properties)
	if err != nil {
		return config, err
	}
	config.auth = authConfig

	for _, prop := range sender.Properties {
		// Authentication properties were read above; skipping them here keeps the unknown
		// property warning from firing on keys this client has no reason to know.
		if outboundauth.OwnsPropertyKey(prop.GetName()) {
			continue
		}

		value, err := prop.GetValue()
		if err != nil {
			return config, fmt.Errorf("failed to get property value for %s: %w", prop.GetName(), err)
		}

		switch prop.GetName() {
		case common.SMTPPropKeyHost:
			config.host = strings.TrimSpace(value)
		case common.SMTPPropKeyPort:
			port, convErr := strconv.Atoi(strings.TrimSpace(value))
			if convErr != nil {
				return config, fmt.Errorf("invalid port: %s", value)
			}
			config.port = port
		case common.SMTPPropKeyFromAddress:
			config.from = strings.TrimSpace(value)
		case common.SMTPPropKeyFromName:
			config.fromName = strings.TrimSpace(value)
		case common.SMTPPropKeyTLS:
			mode, ok := common.ParseTLSMode(value)
			if !ok {
				return config, fmt.Errorf("invalid tls mode: %s", value)
			}
			config.tlsMode = mode
		case common.SenderPropertySupportedChannels:
			// Ignored here as it is a generic sender property.
		default:
			logger.Warn(ctx, "Unknown property for SMTP email client", log.String("property", prop.GetName()))
		}
	}

	if err := validateSMTPConfig(config); err != nil {
		return config, err
	}

	return config, nil
}

// validateSMTPConfig rejects a configuration that cannot produce a working connection.
func validateSMTPConfig(config smtpConfig) error {
	if config.host == "" {
		return errors.New("host is missing")
	}
	if config.port <= 0 || config.port > 65535 {
		return fmt.Errorf("port is out of range: %d", config.port)
	}
	if !common.IsValidEmailAddress(config.from) {
		return errors.New("from address is missing or invalid")
	}
	if !common.IsValidHeaderText(config.fromName) {
		return errors.New("from name must not contain line breaks")
	}
	if err := outboundauth.Validate(config.auth, common.SMTPSupportedAuthTypes); err != nil {
		return err
	}
	// Transport policy, not a generic rule: credentials must not travel in the clear.
	if config.auth.Enabled() && config.tlsMode == common.TLSModeNone {
		return errors.New("TLS must be enabled when authentication is configured")
	}
	return nil
}

// GetName returns the name of the sender backing this client.
func (c *smtpEmailClient) GetName() string {
	return c.name
}

// Send dispatches an email notification via SMTP.
func (c *smtpEmailClient) Send(ctx context.Context, emailData common.EmailData) error {
	if err := emailData.Validate(); err != nil {
		return err
	}

	c.logger.Debug(ctx, "Sending email via SMTP", log.MaskedString("from", c.config.from))

	recipients := make([]string, 0, len(emailData.To)+len(emailData.CC)+len(emailData.BCC))
	recipients = append(recipients, emailData.To...)
	recipients = append(recipients, emailData.CC...)
	recipients = append(recipients, emailData.BCC...)

	serverAddress := net.JoinHostPort(c.config.host, strconv.Itoa(c.config.port))

	if err := c.sendViaSMTP(ctx, serverAddress, recipients, c.buildMessage(emailData)); err != nil {
		return err
	}

	c.logger.Debug(ctx, "Email sent successfully")
	return nil
}

// buildMessage constructs the raw RFC 5322 message with headers and body. BCC is deliberately
// absent: those recipients travel in the envelope only.
func (c *smtpEmailClient) buildMessage(emailData common.EmailData) string {
	var builder strings.Builder

	builder.WriteString("From: " + c.fromHeader() + common.CRLF)
	builder.WriteString("To: " + strings.Join(emailData.To, ", ") + common.CRLF)
	if len(emailData.CC) > 0 {
		builder.WriteString("Cc: " + strings.Join(emailData.CC, ", ") + common.CRLF)
	}
	builder.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", emailData.Subject) + common.CRLF)
	builder.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + common.CRLF)
	builder.WriteString("Message-ID: " + c.newMessageID() + common.CRLF)
	builder.WriteString("MIME-Version: 1.0" + common.CRLF)

	if emailData.IsHTML {
		builder.WriteString(`Content-Type: text/html; charset="utf-8"` + common.CRLF)
	} else {
		builder.WriteString(`Content-Type: text/plain; charset="utf-8"` + common.CRLF)
	}
	// Quoted-printable keeps a UTF-8 body within the 7-bit, 998-octet line limits SMTP relays
	// enforce, rather than relying on every hop accepting raw 8-bit data.
	builder.WriteString("Content-Transfer-Encoding: quoted-printable" + common.CRLF)

	builder.WriteString(common.CRLF)
	// Writes to a strings.Builder cannot fail, so neither can the encoder writing into it.
	bodyWriter := quotedprintable.NewWriter(&builder)
	_, _ = bodyWriter.Write([]byte(emailData.Body))
	_ = bodyWriter.Close()

	return builder.String()
}

// fromHeader renders the value of the From header. With a display name configured, mail.Address
// quotes it and MIME encodes it where needed, so a name carrying a comma, a quote or non-ASCII
// still renders as one valid address. The envelope sender and the Message-ID domain keep reading
// the bare address, which is what they require.
func (c *smtpEmailClient) fromHeader() string {
	if c.config.fromName == "" {
		return c.config.from
	}
	return (&mail.Address{Name: c.config.fromName, Address: c.config.from}).String()
}

// newMessageID builds a Message-ID whose domain part is the from address' domain. Receivers
// score messages without one as more likely to be spam.
func (c *smtpEmailClient) newMessageID() string {
	domain := c.config.host
	if at := strings.LastIndex(c.config.from, "@"); at >= 0 {
		domain = c.config.from[at+1:]
	}

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return fmt.Sprintf("<%d@%s>", time.Now().UnixNano(), domain)
	}
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(random), domain)
}

// sendViaSMTP handles the connection setup, optional TLS, authentication and transmission.
func (c *smtpEmailClient) sendViaSMTP(
	ctx context.Context, serverAddress string, recipients []string, message string) error {
	conn, err := c.dial(ctx, serverAddress)
	if err != nil {
		return fmt.Errorf("smtp connection failed: %w", err)
	}

	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			c.logger.Debug(ctx, "Failed to close SMTP connection", log.Error(closeErr))
		}
	}()

	// net/smtp sets no deadlines and takes no context, so bound the whole conversation here: by
	// the session timeout or the caller's deadline, whichever is sooner, and by cancellation,
	// which closes the connection to unblock whatever call is in flight.
	deadline := time.Now().Add(smtpSessionTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("smtp connection failed: %w", err)
	}
	stop := context.AfterFunc(ctx, func() {
		_ = conn.Close()
	})
	defer stop()

	client, err := smtp.NewClient(conn, c.config.host)
	if err != nil {
		return fmt.Errorf("smtp connection failed: %w", err)
	}

	var quitSucceeded bool
	defer func() {
		if !quitSucceeded {
			if closeErr := client.Close(); closeErr != nil {
				c.logger.Debug(ctx, "Failed to force close SMTP client", log.Error(closeErr))
			}
		}
	}()

	if c.config.tlsMode == common.TLSModeSTARTTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("smtp connection failed: STARTTLS not supported by server")
		}
		if err := client.StartTLS(c.tlsConfig()); err != nil {
			return fmt.Errorf("smtp connection failed: %w", err)
		}
	}

	auth, err := c.auth.Auth(ctx, c.config.host)
	if err != nil {
		return fmt.Errorf("smtp authentication failed: %w", err)
	}
	if auth != nil {
		if err := client.Auth(auth); err != nil {
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

	// The server already accepted DATA, so a failed QUIT does not fail the send.
	if err := client.Quit(); err != nil {
		c.logger.Debug(ctx, "Failed to gracefully close SMTP client", log.Error(err))
	} else {
		quitSucceeded = true
	}

	return nil
}

// dial opens the transport, using a direct TLS connection for implicit TLS (SMTPS).
func (c *smtpEmailClient) dial(ctx context.Context, serverAddress string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: smtpDialTimeout}
	if c.config.tlsMode == common.TLSModeImplicit {
		tlsDialer := &tls.Dialer{NetDialer: dialer, Config: c.tlsConfig()}
		return tlsDialer.DialContext(ctx, "tcp", serverAddress)
	}
	return dialer.DialContext(ctx, "tcp", serverAddress)
}

// tlsConfig returns the TLS configuration used for both implicit TLS and STARTTLS.
func (c *smtpEmailClient) tlsConfig() *tls.Config {
	return &tls.Config{
		ServerName: c.config.host,
		MinVersion: tls.VersionTLS12,
	}
}
