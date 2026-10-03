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
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/smtpauth"
)

const (
	// smtpDialTimeout bounds establishing the TCP (and, for implicit TLS, the handshake).
	smtpDialTimeout = 30 * time.Second
	// smtpSessionTimeout bounds the whole SMTP conversation after the dial. Without it a server
	// that accepts the connection and then stalls blocks the calling goroutine indefinitely, since
	// net/smtp sets no deadlines of its own.
	smtpSessionTimeout = 60 * time.Second
	// headerFoldLength is the header line length RFC 5322 recommends. Header values are folded
	// at whitespace to stay near it, and so well within the 998-octet hard limit.
	headerFoldLength = 78
)

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
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SMTPEmailClient"))

	config, err := parseSMTPConfig(ctx, sender, logger)
	if err != nil {
		return nil, err
	}

	authenticator, err := smtpauth.New(config.auth, config.host)
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

// GetName returns the name of the sender backing this client.
func (emailClient *smtpEmailClient) GetName() string {
	return emailClient.name
}

// Send dispatches an email notification via SMTP.
func (emailClient *smtpEmailClient) Send(ctx context.Context, emailData common.EmailData) error {
	if err := emailData.Validate(); err != nil {
		return err
	}

	emailClient.logger.Debug(ctx, "Sending email via SMTP", log.MaskedString("from", emailClient.config.from))

	recipients := make([]string, 0, len(emailData.To)+len(emailData.CC)+len(emailData.BCC))
	recipients = append(recipients, emailData.To...)
	recipients = append(recipients, emailData.CC...)
	recipients = append(recipients, emailData.BCC...)

	serverAddress := net.JoinHostPort(emailClient.config.host, strconv.Itoa(emailClient.config.port))

	if err := emailClient.sendViaSMTP(ctx, serverAddress, recipients, emailClient.buildMessage(emailData)); err != nil {
		return err
	}

	emailClient.logger.Debug(ctx, "Email sent successfully")
	return nil
}

// buildMessage constructs the raw RFC 5322 message with headers and body. BCC is deliberately
// absent: those recipients travel in the envelope only.
func (emailClient *smtpEmailClient) buildMessage(emailData common.EmailData) string {
	var builder strings.Builder

	writeHeader(&builder, "From", emailClient.fromHeader())
	writeHeader(&builder, "To", strings.Join(emailData.To, ", "))
	if len(emailData.CC) > 0 {
		writeHeader(&builder, "Cc", strings.Join(emailData.CC, ", "))
	}
	// Q-encoding separates its encoded-words with spaces, so a long non-ASCII subject folds at
	// those like any other.
	writeHeader(&builder, "Subject", mime.QEncoding.Encode("utf-8", emailData.Subject))
	writeHeader(&builder, "Date", time.Now().Format(time.RFC1123Z))
	writeHeader(&builder, "Message-ID", emailClient.newMessageID())
	builder.WriteString("MIME-Version: 1.0" + serverconst.CRLF)

	if emailData.IsHTML {
		builder.WriteString(`Content-Type: text/html; charset="utf-8"` + serverconst.CRLF)
	} else {
		builder.WriteString(`Content-Type: text/plain; charset="utf-8"` + serverconst.CRLF)
	}
	// Quoted-printable keeps a UTF-8 body within the 7-bit, 998-octet line limits SMTP relays
	// enforce, rather than relying on every hop accepting raw 8-bit data.
	builder.WriteString("Content-Transfer-Encoding: quoted-printable" + serverconst.CRLF)

	builder.WriteString(serverconst.CRLF)
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
func (emailClient *smtpEmailClient) fromHeader() string {
	if emailClient.config.fromName == "" {
		return emailClient.config.from
	}
	return (&mail.Address{Name: emailClient.config.fromName, Address: emailClient.config.from}).String()
}

// newMessageID builds a Message-ID whose domain part is the from address' domain. Receivers
// score messages without one as more likely to be spam.
func (emailClient *smtpEmailClient) newMessageID() string {
	domain := emailClient.config.host
	if atIndex := strings.LastIndex(emailClient.config.from, "@"); atIndex >= 0 {
		domain = emailClient.config.from[atIndex+1:]
	}

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return fmt.Sprintf("<%d@%s>", time.Now().UnixNano(), domain)
	}
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(random), domain)
}

// sendViaSMTP handles the connection setup, optional TLS, authentication and transmission.
func (emailClient *smtpEmailClient) sendViaSMTP(
	ctx context.Context, serverAddress string, recipients []string, message string) error {
	connection, err := emailClient.dial(ctx, serverAddress)
	if err != nil {
		return fmt.Errorf("smtp connection failed: %w", err)
	}

	defer func() {
		if closeErr := connection.Close(); closeErr != nil {
			emailClient.logger.Debug(ctx, "Failed to close SMTP connection", log.Error(closeErr))
		}
	}()

	// net/smtp sets no deadlines and takes no context, so bound the whole conversation here: by
	// the session timeout or the caller's deadline, whichever is sooner, and by cancellation,
	// which closes the connection to unblock whatever call is in flight.
	deadline := time.Now().Add(smtpSessionTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return fmt.Errorf("smtp connection failed: %w", err)
	}
	stop := context.AfterFunc(ctx, func() {
		_ = connection.Close()
	})
	defer stop()

	smtpClient, err := smtp.NewClient(connection, emailClient.config.host)
	if err != nil {
		return fmt.Errorf("smtp connection failed: %w", err)
	}

	var quitSucceeded bool
	defer func() {
		if !quitSucceeded {
			if closeErr := smtpClient.Close(); closeErr != nil {
				emailClient.logger.Debug(ctx, "Failed to force close SMTP client", log.Error(closeErr))
			}
		}
	}()

	if emailClient.config.tlsMode == common.TLSModeSTARTTLS {
		if ok, _ := smtpClient.Extension("STARTTLS"); !ok {
			return errors.New("smtp connection failed: STARTTLS not supported by server")
		}
		if err := smtpClient.StartTLS(emailClient.tlsConfig()); err != nil {
			return fmt.Errorf("smtp connection failed: %w", err)
		}
	}

	if err := emailClient.auth.Authenticate(ctx, smtpClient); err != nil {
		return fmt.Errorf("smtp authentication failed: %w", err)
	}

	if err := smtpClient.Mail(emailClient.config.from); err != nil {
		return fmt.Errorf("email send failed: %w", err)
	}
	for _, recipient := range recipients {
		if err := smtpClient.Rcpt(recipient); err != nil {
			return fmt.Errorf("email send failed: %w", err)
		}
	}

	writer, err := smtpClient.Data()
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
	if err := smtpClient.Quit(); err != nil {
		emailClient.logger.Debug(ctx, "Failed to gracefully close SMTP client", log.Error(err))
	} else {
		quitSucceeded = true
	}

	return nil
}

// dial opens the transport, using a direct TLS connection for implicit TLS (SMTPS).
func (emailClient *smtpEmailClient) dial(ctx context.Context, serverAddress string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: smtpDialTimeout}
	if emailClient.config.tlsMode == common.TLSModeImplicit {
		tlsDialer := &tls.Dialer{NetDialer: dialer, Config: emailClient.tlsConfig()}
		return tlsDialer.DialContext(ctx, "tcp", serverAddress)
	}
	return dialer.DialContext(ctx, "tcp", serverAddress)
}

// tlsConfig returns the TLS configuration used for both implicit TLS and STARTTLS.
func (emailClient *smtpEmailClient) tlsConfig() *tls.Config {
	return &tls.Config{
		ServerName: emailClient.config.host,
		MinVersion: tls.VersionTLS12,
	}
}

// parseSMTPConfig reads the SMTP configuration out of the sender properties and validates it.
// Declaratively loaded senders are not validated on load, so for them this is the only check.
func parseSMTPConfig(ctx context.Context, sender common.NotificationSenderDTO,
	logger *log.Logger) (smtpConfig, error) {
	config := smtpConfig{tlsMode: common.TLSModeSTARTTLS}

	authConfig, err := outboundauth.FromProperties(sender.Properties)
	if err != nil {
		return config, err
	}
	config.auth = authConfig

	for _, property := range sender.Properties {
		// Authentication properties were read above; skipping them here keeps the unknown
		// property warning from firing on keys this client has no reason to know.
		if outboundauth.OwnsPropertyKey(property.GetName()) {
			continue
		}

		value, err := property.GetValue()
		if err != nil {
			return config, fmt.Errorf("failed to get property value for %s: %w", property.GetName(), err)
		}

		switch property.GetName() {
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
			logger.Warn(ctx, "Unknown property for SMTP email client", log.String("property", property.GetName()))
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

// writeHeader writes one header field, folding its value at spaces so that each line stays
// within headerFoldLength where the value allows it. A fold only inserts a CRLF before an
// existing space, so unfolding restores the value exactly.
func writeHeader(builder *strings.Builder, name, value string) {
	builder.WriteString(name)
	builder.WriteString(":")
	lineLength := len(name) + 1
	for wordIndex, word := range strings.Split(value, " ") {
		// Folding before an empty word would leave a line of whitespace only, which RFC 5322
		// forbids.
		if wordIndex > 0 && word != "" && lineLength+1+len(word) > headerFoldLength {
			builder.WriteString(serverconst.CRLF)
			lineLength = 0
		}
		builder.WriteString(" ")
		builder.WriteString(word)
		lineLength += 1 + len(word)
	}
	builder.WriteString(serverconst.CRLF)
}
