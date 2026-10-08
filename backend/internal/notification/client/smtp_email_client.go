// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
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
	// smtpMaxReadBytes caps what a session reads from the server. net/smtp reads a reply with no
	// length limit, so without it a server could stream data into memory until the session
	// deadline. Real replies are a few hundred bytes; the cap also covers the TLS handshake.
	smtpMaxReadBytes = 1 << 20
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

	config, err := parseSMTPConfig(sender.Properties)
	if err != nil {
		return nil, err
	}
	for _, property := range sender.Properties {
		if !isKnownSMTPProperty(property.GetName()) {
			logger.Warn(ctx, "Unknown property for SMTP email client", log.String("property", property.GetName()))
		}
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

	writeHeader(&builder, "From", c.fromHeader())
	writeHeader(&builder, "To", strings.Join(emailData.To, ", "))
	if len(emailData.CC) > 0 {
		writeHeader(&builder, "Cc", strings.Join(emailData.CC, ", "))
	}
	// Q-encoding separates its encoded-words with spaces, so a long non-ASCII subject folds at
	// those like any other.
	writeHeader(&builder, "Subject", mime.QEncoding.Encode("utf-8", emailData.Subject))
	writeHeader(&builder, "Date", time.Now().Format(time.RFC1123Z))
	writeHeader(&builder, "Message-ID", c.newMessageID())
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
	if atIndex := strings.LastIndex(c.config.from, "@"); atIndex >= 0 {
		domain = c.config.from[atIndex+1:]
	}

	return fmt.Sprintf("<%s@%s>", rand.Text(), domain)
}

// sendViaSMTP handles the connection setup, optional TLS, authentication and transmission.
func (c *smtpEmailClient) sendViaSMTP(
	ctx context.Context, serverAddress string, recipients []string, message string) error {
	connection, err := c.dial(ctx, serverAddress)
	if err != nil {
		return fmt.Errorf("smtp connection failed: %w", err)
	}

	// net/smtp sets no deadlines and takes no context, so bound the whole conversation here: by
	// the session timeout or the caller's deadline, whichever is sooner, and by cancellation,
	// which closes the connection to unblock whatever call is in flight.
	deadline := time.Now().Add(smtpSessionTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := connection.SetDeadline(deadline); err != nil {
		_ = connection.Close()
		return fmt.Errorf("smtp connection failed: %w", err)
	}
	stop := context.AfterFunc(ctx, func() {
		_ = connection.Close()
	})
	defer stop()

	// From here the client owns the connection: NewClient closes it on failure, and Quit or
	// Close closes it afterwards.
	smtpClient, err := smtp.NewClient(connection, c.config.host)
	if err != nil {
		return fmt.Errorf("smtp connection failed: %w", err)
	}

	var quitSucceeded bool
	defer func() {
		if !quitSucceeded {
			if closeErr := smtpClient.Close(); closeErr != nil {
				c.logger.Debug(ctx, "Failed to force close SMTP client", log.Error(closeErr))
			}
		}
	}()

	if c.config.tlsMode == common.TLSModeSTARTTLS {
		if ok, _ := smtpClient.Extension("STARTTLS"); !ok {
			return errors.New("smtp connection failed: STARTTLS not supported by server")
		}
		if err := smtpClient.StartTLS(c.tlsConfig()); err != nil {
			return fmt.Errorf("smtp connection failed: %w", err)
		}
	}

	if err := c.auth.Authenticate(ctx, smtpClient); err != nil {
		return replyError("smtp authentication failed", err)
	}

	if err := smtpClient.Mail(c.config.from); err != nil {
		return replyError("email send failed", err)
	}
	for _, recipient := range recipients {
		if err := smtpClient.Rcpt(recipient); err != nil {
			return replyError("email send failed", err)
		}
	}

	writer, err := smtpClient.Data()
	if err != nil {
		return replyError("email send failed", err)
	}
	if _, err := writer.Write([]byte(message)); err != nil {
		return replyError("email send failed", err)
	}
	if err := writer.Close(); err != nil {
		return replyError("email send failed", err)
	}

	// The server already accepted DATA, so a failed QUIT does not fail the send.
	if err := smtpClient.Quit(); err != nil {
		c.logger.Debug(ctx, "Failed to gracefully close SMTP client", log.Error(err))
	} else {
		quitSucceeded = true
	}

	return nil
}

// dial opens the transport, using a direct TLS connection for implicit TLS (SMTPS). The read cap
// sits under TLS rather than over it: net/smtp treats a session as encrypted only when its
// connection is a *tls.Conn, and refuses to send credentials otherwise.
func (c *smtpEmailClient) dial(ctx context.Context, serverAddress string) (net.Conn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, smtpDialTimeout)
	defer cancel()

	rawConn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", serverAddress)
	if err != nil {
		return nil, err
	}
	connection := net.Conn(&readLimitedConn{Conn: rawConn, remaining: smtpMaxReadBytes})

	if c.config.tlsMode == common.TLSModeImplicit {
		tlsConn := tls.Client(connection, c.tlsConfig())
		if err := tlsConn.HandshakeContext(dialCtx); err != nil {
			_ = rawConn.Close()
			return nil, err
		}
		return tlsConn, nil
	}
	return connection, nil
}

// tlsConfig returns the TLS configuration used for both implicit TLS and STARTTLS.
func (c *smtpEmailClient) tlsConfig() *tls.Config {
	return &tls.Config{
		ServerName: c.config.host,
		MinVersion: tls.VersionTLS12,
	}
}

// ValidateSMTPProperties rejects SMTP sender properties that cannot produce a working client. It
// is the one SMTP validator: the sender API runs it when a provider is written, and the client
// runs the same checks when it is built.
func ValidateSMTPProperties(properties []cmodels.Property) error {
	_, err := parseSMTPConfig(properties)
	return err
}

// parseSMTPConfig reads the SMTP configuration out of the sender properties and validates it.
// Every value is checked here rather than at send time, so a sender that cannot deliver is
// rejected when it is configured instead of during a password reset.
func parseSMTPConfig(properties []cmodels.Property) (smtpConfig, error) {
	values := make(map[string]string, len(properties))
	for _, prop := range properties {
		if prop.GetName() == "" {
			return smtpConfig{}, errors.New("properties must have non-empty name")
		}
		value, err := prop.GetValue()
		if err != nil {
			return smtpConfig{}, fmt.Errorf("failed to read property %s", prop.GetName())
		}
		values[prop.GetName()] = value
	}

	host := strings.TrimSpace(values[common.SMTPPropKeyHost])
	if host == "" {
		return smtpConfig{}, errors.New("required property missing for the provider: " + common.SMTPPropKeyHost)
	}

	rawPort := strings.TrimSpace(values[common.SMTPPropKeyPort])
	if rawPort == "" {
		return smtpConfig{}, errors.New("required property missing for the provider: " + common.SMTPPropKeyPort)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 {
		return smtpConfig{}, fmt.Errorf("port must be an integer between 1 and 65535, got: %s", rawPort)
	}

	from := strings.TrimSpace(values[common.SMTPPropKeyFromAddress])
	if from == "" {
		return smtpConfig{}, errors.New(
			"required property missing for the provider: " + common.SMTPPropKeyFromAddress)
	}
	if !common.IsValidEmailAddress(from) {
		return smtpConfig{}, fmt.Errorf("invalid from address: %s", from)
	}

	// The display name is optional, but it lands in the From header, so it must not be able to
	// end that header and start one of its own. It is checked before trimming so a trailing line
	// break cannot be trimmed away.
	if !common.IsValidHeaderText(values[common.SMTPPropKeyFromName]) {
		return smtpConfig{}, errors.New("from name must not contain line breaks")
	}

	tlsMode, ok := common.ParseTLSMode(values[common.SMTPPropKeyTLS])
	if !ok {
		return smtpConfig{}, fmt.Errorf("tls must be one of none, starttls or implicit, got: %s",
			values[common.SMTPPropKeyTLS])
	}

	authConfig := outboundauth.FromValues(values)
	if err := outboundauth.Validate(authConfig, smtpauth.SupportedTypes()); err != nil {
		return smtpConfig{}, err
	}
	// Transport policy, not a generic rule: credentials must not travel in the clear.
	if authConfig.Enabled() && tlsMode == common.TLSModeNone {
		return smtpConfig{}, errors.New("tls must be enabled when authentication is configured")
	}

	return smtpConfig{
		host:     host,
		port:     port,
		from:     from,
		fromName: strings.TrimSpace(values[common.SMTPPropKeyFromName]),
		tlsMode:  tlsMode,
		auth:     authConfig,
	}, nil
}

// isKnownSMTPProperty reports whether name is a property an SMTP sender carries.
func isKnownSMTPProperty(name string) bool {
	switch name {
	case common.SMTPPropKeyHost, common.SMTPPropKeyPort, common.SMTPPropKeyFromAddress,
		common.SMTPPropKeyFromName, common.SMTPPropKeyTLS, common.SenderPropertySupportedChannels:
		return true
	}
	return outboundauth.OwnsPropertyKey(name)
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

// replyError wraps a failure of the given stage of an SMTP session. A server reply keeps only its
// code: its text can echo the envelope or the username, as Postfix does with a rejected
// recipient, and the error is logged.
func replyError(stage string, err error) error {
	var replyErr *textproto.Error
	if errors.As(err, &replyErr) {
		return fmt.Errorf("%s: server replied %d", stage, replyErr.Code)
	}
	return fmt.Errorf("%s: %w", stage, err)
}

// readLimitedConn fails a read once the session has read its byte budget from the server.
type readLimitedConn struct {
	net.Conn
	remaining int64
}

// Read reads from the connection, never past the remaining budget.
func (c *readLimitedConn) Read(p []byte) (int, error) {
	if c.remaining <= 0 {
		return 0, errSMTPReadLimitExceeded
	}
	if int64(len(p)) > c.remaining {
		p = p[:c.remaining]
	}
	n, err := c.Conn.Read(p)
	c.remaining -= int64(n)
	return n, err
}
