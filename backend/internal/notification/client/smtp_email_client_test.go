// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"io"
	"math/big"
	"mime"
	"mime/quotedprintable"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/smtpauth"
	"github.com/thunder-id/thunderid/tests/mocks/outboundauth/smtpauthmock"
)

type SMTPEmailClientTestSuite struct {
	suite.Suite
}

func TestSMTPEmailClientTestSuite(t *testing.T) {
	suite.Run(t, new(SMTPEmailClientTestSuite))
}

func (suite *SMTPEmailClientTestSuite) SetupSuite() {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
	}
	if err := config.InitializeServerRuntime("", testConfig); err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

// senderFor builds an SMTP sender DTO from a name->value property map.
func (suite *SMTPEmailClientTestSuite) senderFor(values map[string]string) common.NotificationSenderDTO {
	props := make([]cmodels.Property, 0, len(values))
	for name, value := range values {
		isSecret := name == "authentication_password"
		prop, err := cmodels.NewProperty(name, value, isSecret)
		suite.Require().NoError(err)
		props = append(props, *prop)
	}
	return common.NotificationSenderDTO{
		Name:       "Test SMTP",
		Type:       common.NotificationSenderTypeEmail,
		Provider:   common.NotificationProviderTypeSMTP,
		Properties: props,
	}
}

func (suite *SMTPEmailClientTestSuite) baseProps(port int) map[string]string {
	return map[string]string{
		common.SMTPPropKeyHost:        "127.0.0.1",
		common.SMTPPropKeyPort:        strconv.Itoa(port),
		common.SMTPPropKeyFromAddress: "noreply@example.com",
		common.SMTPPropKeyTLS:         string(common.TLSModeNone),
	}
}

// withBasicAuth configures the basic authentication method on a property map and returns it.
func withBasicAuth(props map[string]string, username, password string) map[string]string {
	props["authentication_type"] = string(outboundauth.TypeBasic)
	if username != "" {
		props["authentication_username"] = username
	}
	if password != "" {
		props["authentication_password"] = password
	}
	return props
}

// newClient builds a client against the given port, overriding the base properties.
func (suite *SMTPEmailClientTestSuite) newClient(port int, overrides map[string]string) *smtpEmailClient {
	props := suite.baseProps(port)
	for name, value := range overrides {
		props[name] = value
	}
	client, err := newSMTPEmailClient(context.Background(), suite.senderFor(props))
	suite.Require().NoError(err)
	return client.(*smtpEmailClient)
}

func (suite *SMTPEmailClientTestSuite) emailData() common.EmailData {
	return common.EmailData{
		To:      []string{"user@example.com"},
		Subject: "Hello",
		Body:    "Body text",
	}
}

func (suite *SMTPEmailClientTestSuite) TestNewClient_DefaultsToSTARTTLS() {
	props := suite.baseProps(587)
	delete(props, common.SMTPPropKeyTLS)

	client, err := newSMTPEmailClient(context.Background(), suite.senderFor(props))
	suite.Require().NoError(err)
	suite.Equal("Test SMTP", client.GetName())
	suite.Equal(common.TLSModeSTARTTLS, client.(*smtpEmailClient).config.tlsMode)
}

func (suite *SMTPEmailClientTestSuite) TestNewClient_ParsesAllProperties() {
	props := suite.baseProps(2525)
	props[common.SMTPPropKeyTLS] = string(common.TLSModeImplicit)
	props = withBasicAuth(props, "user", "pass")
	props[common.SenderPropertySupportedChannels] = string(common.ChannelTypeEmail)

	client, err := newSMTPEmailClient(context.Background(), suite.senderFor(props))
	suite.Require().NoError(err)

	config := client.(*smtpEmailClient).config
	suite.Equal("127.0.0.1", config.host)
	suite.Equal(2525, config.port)
	suite.Equal("noreply@example.com", config.from)
	suite.Equal(common.TLSModeImplicit, config.tlsMode)
	suite.Equal(outboundauth.TypeBasic, config.auth.Type)
	suite.Equal("user", config.auth.Get(outboundauth.FieldBasicUsername))
	suite.Equal("pass", config.auth.Get(outboundauth.FieldBasicPassword))
}

// Authentication properties are read by the outboundauth package, not this client's own
// property loop, so they must not trip its unknown-property warning.
func (suite *SMTPEmailClientTestSuite) TestNewClient_DoesNotWarnOnAuthenticationProperties() {
	props := withBasicAuth(suite.baseProps(2525), "user", "pass")
	props[common.SMTPPropKeyTLS] = string(common.TLSModeSTARTTLS)

	client, err := newSMTPEmailClient(context.Background(), suite.senderFor(props))
	suite.Require().NoError(err)
	suite.Equal(outboundauth.TypeBasic, client.(*smtpEmailClient).config.auth.Type)
}

// An omitted authentication block leaves the client presenting no credentials at all.
func (suite *SMTPEmailClientTestSuite) TestNewClient_NoAuthenticationConfigured() {
	client, err := newSMTPEmailClient(context.Background(), suite.senderFor(suite.baseProps(2525)))
	suite.Require().NoError(err)

	config := client.(*smtpEmailClient).config
	suite.False(config.auth.Enabled())

	// The mock fails the test on any call it was not told to expect, so no AUTH is attempted.
	session := smtpauthmock.NewSessionMock(suite.T())
	suite.Require().NoError(client.(*smtpEmailClient).auth.Authenticate(context.Background(), session))
}

func (suite *SMTPEmailClientTestSuite) TestNewClient_InvalidConfig() {
	cases := []struct {
		name   string
		mutate func(map[string]string)
	}{
		{"missing host", func(properties map[string]string) { delete(properties, common.SMTPPropKeyHost) }},
		{"missing port", func(properties map[string]string) { delete(properties, common.SMTPPropKeyPort) }},
		{"non numeric port", func(properties map[string]string) { properties[common.SMTPPropKeyPort] = "abc" }},
		{"port out of range", func(properties map[string]string) { properties[common.SMTPPropKeyPort] = "70000" }},
		{"missing from address", func(properties map[string]string) {
			delete(properties, common.SMTPPropKeyFromAddress)
		}},
		{"invalid from address", func(properties map[string]string) {
			properties[common.SMTPPropKeyFromAddress] = "not-an-address"
		}},
		{"display name from address", func(properties map[string]string) {
			properties[common.SMTPPropKeyFromAddress] = `"Bot" <bot@example.com>`
		}},
		{"invalid tls mode", func(properties map[string]string) { properties[common.SMTPPropKeyTLS] = "yes" }},
		{"auth without tls", func(properties map[string]string) {
			withBasicAuth(properties, "u", "p")
		}},
		{"auth without credentials", func(properties map[string]string) {
			withBasicAuth(properties, "", "")
			properties[common.SMTPPropKeyTLS] = string(common.TLSModeSTARTTLS)
		}},
		{"auth without password", func(properties map[string]string) {
			withBasicAuth(properties, "u", "")
			properties[common.SMTPPropKeyTLS] = string(common.TLSModeSTARTTLS)
		}},
		{"unsupported auth type", func(properties map[string]string) {
			properties["authentication_type"] = "bearer"
		}},
		// A line break in the name would end the From header and let the rest be read as
		// headers of its own.
		{"from name with a line break", func(properties map[string]string) {
			properties[common.SMTPPropKeyFromName] = "Acme\r\nBcc: attacker@evil.test"
		}},
		// A trailing line break must be rejected rather than trimmed away.
		{"from name with a trailing line break", func(properties map[string]string) {
			properties[common.SMTPPropKeyFromName] = "Acme\r\n"
		}},
	}

	for _, testCase := range cases {
		suite.Run(testCase.name, func() {
			props := suite.baseProps(587)
			testCase.mutate(props)
			_, err := newSMTPEmailClient(context.Background(), suite.senderFor(props))
			suite.Error(err)
		})
	}
}

func (suite *SMTPEmailClientTestSuite) TestSend_Success() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{})
	defer server.close()

	client := suite.newClient(server.port(), nil)
	suite.Require().NoError(client.Send(context.Background(), suite.emailData()))

	message := server.message()
	suite.Contains(message, "From: noreply@example.com")
	suite.Contains(message, "To: user@example.com")
	suite.Contains(message, "Subject: Hello")
	suite.Contains(message, `Content-Type: text/plain; charset="utf-8"`)
	// Date and Message-ID are required for deliverability; receivers score their absence.
	suite.Contains(message, "Date: ")
	suite.Contains(message, "Message-ID: <")
	suite.Contains(message, "@example.com>")
	suite.Contains(message, "Body text")
	suite.Equal([]string{"user@example.com"}, server.recipients())
	suite.Equal("noreply@example.com", server.sender())
	suite.True(server.quit())
}

// The display name belongs to the From header only. The envelope sender and the Message-ID
// domain must stay the bare address, or delivery and SPF alignment break.
func (suite *SMTPEmailClientTestSuite) TestSend_WithFromNameKeepsEnvelopeBare() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{})
	defer server.close()

	client := suite.newClient(server.port(), map[string]string{common.SMTPPropKeyFromName: "Acme Support"})
	suite.Require().NoError(client.Send(context.Background(), suite.emailData()))

	suite.Contains(server.message(), `From: "Acme Support" <noreply@example.com>`)
	suite.Equal("noreply@example.com", server.sender())
	suite.Contains(server.message(), "@example.com>")
}

// A name is attacker-visible configuration in the From header, so it has to survive the
// characters that would otherwise split the address or the header itself.
func (suite *SMTPEmailClientTestSuite) TestFromHeaderQuotesAndEncodesName() {
	cases := []struct {
		name     string
		fromName string
		expected string
	}{
		{"plain name is quoted", "Acme Support", `"Acme Support" <noreply@example.com>`},
		{"comma cannot split the address", "Acme, Inc.", `"Acme, Inc." <noreply@example.com>`},
		{"quote is escaped", `Acme "Support"`, `"Acme \"Support\"" <noreply@example.com>`},
		{"non-ascii is mime encoded", "Café", "=?utf-8?q?Caf=C3=A9?= <noreply@example.com>"},
		{"no name leaves the bare address", "", "noreply@example.com"},
	}

	for _, testCase := range cases {
		suite.Run(testCase.name, func() {
			client := &smtpEmailClient{config: smtpConfig{from: "noreply@example.com", fromName: testCase.fromName}}
			suite.Equal(testCase.expected, client.fromHeader())
		})
	}
}

// clientWithAuth builds a client straight from a configuration, bypassing the policy that
// refuses credentials over a plaintext connection. That policy is covered by the validation
// tests; this exists to observe what the configured method actually puts on the wire, which the
// TLS-bearing mock cannot show because its self-signed certificate is rejected before AUTH.
func (suite *SMTPEmailClientTestSuite) clientWithAuth(port int, authConfig outboundauth.Config) *smtpEmailClient {
	authenticator, err := smtpauth.New(authConfig, "127.0.0.1")
	suite.Require().NoError(err)

	return &smtpEmailClient{
		name: "Test SMTP",
		config: smtpConfig{
			host:    "127.0.0.1",
			port:    port,
			from:    "noreply@example.com",
			tlsMode: common.TLSModeNone,
			auth:    authConfig,
		},
		auth:   authenticator,
		logger: log.GetLogger(),
	}
}

func (suite *SMTPEmailClientTestSuite) TestSend_BasicAuthIssuesPlainOnTheWire() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{})
	defer server.close()

	client := suite.clientWithAuth(server.port(), outboundauth.Config{
		Type: outboundauth.TypeBasic,
		Properties: map[string]string{
			outboundauth.FieldBasicUsername: "mailer",
			outboundauth.FieldBasicPassword: "s3cret",
		},
	})
	suite.Require().NoError(client.Send(context.Background(), suite.emailData()))

	authCommand := server.authCommand()
	suite.Require().NotEmpty(authCommand, "the client should have authenticated")
	suite.True(strings.HasPrefix(strings.ToUpper(authCommand), "AUTH PLAIN"), authCommand)

	// The credential is the SASL PLAIN triple, so a wrong encoding would not round-trip.
	encoded := strings.Fields(authCommand)
	suite.Require().Len(encoded, 3)
	decoded, err := base64.StdEncoding.DecodeString(encoded[2])
	suite.Require().NoError(err)
	suite.Equal("\x00mailer\x00s3cret", string(decoded))
}

// A rejected AUTH fails the send before any mail is submitted. The error is logged, so it keeps
// only the reply code and never the server's text, which can echo the username.
func (suite *SMTPEmailClientTestSuite) TestSend_RejectedAuthentication() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{rejectCommand: "AUTH"})
	defer server.close()

	client := suite.clientWithAuth(server.port(), outboundauth.Config{
		Type: outboundauth.TypeBasic,
		Properties: map[string]string{
			outboundauth.FieldBasicUsername: "mailer",
			outboundauth.FieldBasicPassword: "s3cret",
		},
	})
	err := client.Send(context.Background(), suite.emailData())
	suite.Require().Error(err)
	suite.Equal("smtp authentication failed: server replied 535", err.Error())
	suite.NotContains(err.Error(), "mailer")
	suite.NotEmpty(server.authCommand(), "the client should have attempted AUTH")
	suite.Empty(server.sender(), "no mail may be submitted after a rejected AUTH")
	suite.Empty(server.message())
}

func (suite *SMTPEmailClientTestSuite) TestSend_NoAuthenticationIssuesNoAuthCommand() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{})
	defer server.close()

	client := suite.clientWithAuth(server.port(), outboundauth.Config{Type: outboundauth.TypeNone})
	suite.Require().NoError(client.Send(context.Background(), suite.emailData()))

	suite.Empty(server.authCommand(), "an unauthenticated sender must not issue AUTH")
}

// BCC recipients belong in the envelope only; leaking them into the headers would disclose
// them to every other recipient.
func (suite *SMTPEmailClientTestSuite) TestSend_BCCIsEnvelopeOnly() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{})
	defer server.close()

	client := suite.newClient(server.port(), nil)
	data := suite.emailData()
	data.CC = []string{"cc@example.com"}
	data.BCC = []string{"bcc@example.com"}
	suite.Require().NoError(client.Send(context.Background(), data))

	message := server.message()
	suite.Contains(message, "Cc: cc@example.com")
	suite.NotContains(message, "bcc@example.com")
	suite.ElementsMatch([]string{"user@example.com", "cc@example.com", "bcc@example.com"}, server.recipients())
}

func (suite *SMTPEmailClientTestSuite) TestSend_HTMLContentType() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{})
	defer server.close()

	client := suite.newClient(server.port(), nil)
	data := suite.emailData()
	data.IsHTML = true
	suite.Require().NoError(client.Send(context.Background(), data))

	suite.Contains(server.message(), `Content-Type: text/html; charset="utf-8"`)
}

func (suite *SMTPEmailClientTestSuite) TestSend_EncodesNonASCIISubject() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{})
	defer server.close()

	client := suite.newClient(server.port(), nil)
	data := suite.emailData()
	data.Subject = "Überprüfung"
	suite.Require().NoError(client.Send(context.Background(), data))

	suite.Contains(server.message(), "Subject: =?utf-8?q?")
}

// The Message-ID is unique per message and carries the from address' domain.
func (suite *SMTPEmailClientTestSuite) TestNewMessageID() {
	client := &smtpEmailClient{config: smtpConfig{host: "127.0.0.1", from: "noreply@example.com"}}

	first := client.newMessageID()
	suite.Regexp(`^<[A-Z2-7]{26}@example\.com>$`, first)
	suite.NotEqual(first, client.newMessageID())
}

// A header line may not exceed 998 octets, and mime.QEncoding joins its encoded-words with a
// plain space, so a long subject has to be folded while still decoding to the original text.
func (suite *SMTPEmailClientTestSuite) TestBuildMessage_FoldsLongSubject() {
	cases := []struct {
		name    string
		subject string
	}{
		{"non-ascii", strings.Repeat("é", 440)},
		{"ascii", strings.TrimSpace(strings.Repeat("verification ", 150))},
	}

	for _, testCase := range cases {
		suite.Run(testCase.name, func() {
			client := &smtpEmailClient{config: smtpConfig{host: "127.0.0.1", from: "noreply@example.com"}}
			data := suite.emailData()
			data.Subject = testCase.subject
			message := client.buildMessage(data)

			header, _, found := strings.Cut(message, serverconst.CRLF+serverconst.CRLF)
			suite.Require().True(found)
			lines := strings.Split(header, serverconst.CRLF)
			for _, line := range lines {
				suite.LessOrEqual(len(line), 998)
			}

			var subject string
			for lineIndex, line := range lines {
				if !strings.HasPrefix(line, "Subject: ") {
					continue
				}
				subject = strings.TrimPrefix(line, "Subject: ")
				for _, next := range lines[lineIndex+1:] {
					if !strings.HasPrefix(next, " ") {
						break
					}
					subject += next
				}
				break
			}
			suite.Greater(strings.Count(header, serverconst.CRLF+" "), 0, "the subject should have been folded")

			decoded, err := new(mime.WordDecoder).DecodeHeader(subject)
			suite.Require().NoError(err)
			suite.Equal(testCase.subject, decoded)
		})
	}
}

// The body is declared and sent as quoted-printable, so non-ASCII text and long lines survive
// relays that only accept 7-bit data.
func (suite *SMTPEmailClientTestSuite) TestSend_EncodesBodyAsQuotedPrintable() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{})
	defer server.close()

	client := suite.newClient(server.port(), nil)
	data := suite.emailData()
	data.Body = "Ihr Bestätigungscode lautet 1234. " + strings.Repeat("x", 200)
	suite.Require().NoError(client.Send(context.Background(), data))

	message := server.message()
	suite.Contains(message, "Content-Transfer-Encoding: quoted-printable")
	suite.Contains(message, "Best=C3=A4tigungscode")
	suite.NotContains(message, "Bestätigungscode")
	for _, line := range strings.Split(message, "\n") {
		suite.LessOrEqual(len(line), 78)
	}

	header, body, found := strings.Cut(message, "\n\n")
	suite.Require().True(found, "message must separate headers from body: %q", header)
	decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(body)))
	suite.Require().NoError(err)
	suite.Equal(data.Body, strings.TrimRight(string(decoded), "\n"))
}

func (suite *SMTPEmailClientTestSuite) TestSend_RejectsInvalidPayload() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{})
	defer server.close()

	client := suite.newClient(server.port(), nil)

	cases := []struct {
		name string
		data common.EmailData
	}{
		{"no recipient", common.EmailData{Subject: "s", Body: "b"}},
		{"blank recipient", common.EmailData{To: []string{"  "}, Subject: "s", Body: "b"}},
		{"malformed recipient", common.EmailData{To: []string{"nope"}, Subject: "s", Body: "b"}},
		{"header injection in recipient", common.EmailData{
			To: []string{"user@example.com\r\nBcc: attacker@evil.test"}, Subject: "s", Body: "b"}},
		{"header injection in subject", common.EmailData{
			To: []string{"user@example.com"}, Subject: "s\r\nBcc: attacker@evil.test", Body: "b"}},
	}

	for _, testCase := range cases {
		suite.Run(testCase.name, func() {
			suite.Error(client.Send(context.Background(), testCase.data))
		})
	}

	// Nothing reached the wire.
	suite.Empty(server.message())
}

// The client must fail closed rather than silently delivering in the clear when the server
// does not offer the STARTTLS upgrade the sender was configured for.
func (suite *SMTPEmailClientTestSuite) TestSend_STARTTLSNotSupportedFailsClosed() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{})
	defer server.close()

	client := suite.newClient(server.port(), map[string]string{
		common.SMTPPropKeyTLS: string(common.TLSModeSTARTTLS),
	})

	err := client.Send(context.Background(), suite.emailData())
	suite.Require().Error(err)
	suite.Contains(err.Error(), "STARTTLS not supported")
	suite.Empty(server.message())
}

// The client verifies the server certificate: no InsecureSkipVerify, no custom roots. Against
// a self-signed server the STARTTLS handshake must fail and nothing must be delivered.
func (suite *SMTPEmailClientTestSuite) TestSend_STARTTLSVerifiesCertificate() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{tlsConfig: newSelfSignedTLSConfig(suite.T())})
	defer server.close()

	client := suite.newClient(server.port(), map[string]string{
		common.SMTPPropKeyTLS: string(common.TLSModeSTARTTLS),
	})

	err := client.Send(context.Background(), suite.emailData())
	suite.Require().Error(err)
	suite.Contains(err.Error(), "smtp connection failed")
	suite.True(server.sawSTARTTLS(), "the client should have attempted the STARTTLS upgrade")
	suite.Empty(server.message())
}

func (suite *SMTPEmailClientTestSuite) TestSend_ImplicitTLSVerifiesCertificate() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{
		tlsConfig: newSelfSignedTLSConfig(suite.T()),
		implicit:  true,
	})
	defer server.close()

	client := suite.newClient(server.port(), map[string]string{
		common.SMTPPropKeyTLS: string(common.TLSModeImplicit),
	})

	err := client.Send(context.Background(), suite.emailData())
	suite.Require().Error(err)
	suite.Contains(err.Error(), "smtp connection failed")
	suite.Empty(server.message())
}

func (suite *SMTPEmailClientTestSuite) TestSend_ServerRejections() {
	cases := []struct {
		name    string
		rejects string
		code    string
	}{
		{"mail from", "MAIL FROM", "550"},
		{"rcpt to", "RCPT TO", "550"},
		{"data", "DATA", "554"},
	}

	for _, testCase := range cases {
		suite.Run(testCase.name, func() {
			server := newMockSMTPServer(suite.T(), mockSMTPOptions{rejectCommand: testCase.rejects})
			defer server.close()

			client := suite.newClient(server.port(), nil)
			err := client.Send(context.Background(), suite.emailData())
			suite.Require().Error(err)
			suite.Equal("email send failed: server replied "+testCase.code, err.Error())
		})
	}
}

// The error is logged, so a reply that echoes the recipient must not carry the address into it.
func (suite *SMTPEmailClientTestSuite) TestSend_RejectedRecipientIsNotInError() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{rejectCommand: "RCPT TO"})
	defer server.close()

	client := suite.newClient(server.port(), nil)
	emailData := suite.emailData()
	err := client.Send(context.Background(), emailData)
	suite.Require().Error(err)
	for _, recipient := range emailData.To {
		suite.NotContains(err.Error(), recipient)
	}
}

// The server accepted DATA, so a refused QUIT must not fail the send.
func (suite *SMTPEmailClientTestSuite) TestSend_RejectedQuitStillSucceeds() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{rejectCommand: "QUIT"})
	defer server.close()

	client := suite.newClient(server.port(), nil)
	suite.NoError(client.Send(context.Background(), suite.emailData()))
	suite.Contains(server.message(), "Body text")
}

func (suite *SMTPEmailClientTestSuite) TestSend_ConnectionRefused() {
	// Bind and immediately release a port so the dial is refused deterministically.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	suite.Require().NoError(err)
	port := listener.Addr().(*net.TCPAddr).Port
	suite.Require().NoError(listener.Close())

	client := suite.newClient(port, nil)
	sendErr := client.Send(context.Background(), suite.emailData())
	suite.Require().Error(sendErr)
	suite.Contains(sendErr.Error(), "smtp connection failed")
}

func (suite *SMTPEmailClientTestSuite) TestSend_CancelledContextAbortsDial() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{})
	defer server.close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := suite.newClient(server.port(), nil)
	err := client.Send(ctx, suite.emailData())
	suite.Require().Error(err)
	suite.Contains(err.Error(), "smtp connection failed")
}

// Canceling the caller's context after the connection is up must end the session at once rather
// than leaving it to the session timeout, since net/smtp itself never reads the context.
func (suite *SMTPEmailClientTestSuite) TestSend_CancelledContextAbortsStalledSession() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{stall: true})
	defer server.close()

	client := suite.newClient(server.port(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- client.Send(ctx, suite.emailData()) }()
	time.AfterFunc(100*time.Millisecond, cancel)

	select {
	case err := <-done:
		suite.Error(err)
	case <-time.After(10 * time.Second):
		suite.Fail("Send did not honor context cancellation after the dial")
	}
}

// A caller's deadline sooner than the session timeout bounds the session.
func (suite *SMTPEmailClientTestSuite) TestSend_ContextDeadlineBoundsStalledSession() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{stall: true})
	defer server.close()

	client := suite.newClient(server.port(), nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- client.Send(ctx, suite.emailData()) }()

	select {
	case err := <-done:
		suite.Error(err)
	case <-time.After(10 * time.Second):
		suite.Fail("Send did not honor the context deadline after the dial")
	}
}

// net/smtp reads a reply with no length limit, so a server that never ends its greeting must
// fail the send once the session's read budget is spent, rather than fill memory until the
// session deadline.
func (suite *SMTPEmailClientTestSuite) TestSend_StopsReadingAnOversizedReply() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{flood: true})
	defer server.close()

	client := suite.newClient(server.port(), nil)
	err := client.Send(context.Background(), suite.emailData())
	suite.Require().Error(err)
	suite.ErrorIs(err, errSMTPReadLimitExceeded)
	suite.Empty(server.message())
}

// The budget covers the whole session, and a read past it fails without touching the connection.
func (suite *SMTPEmailClientTestSuite) TestReadLimitedConn_CapsTotalBytesRead() {
	clientSide, serverSide := net.Pipe()
	defer func() { _ = serverSide.Close() }()
	go func() { _, _ = serverSide.Write([]byte("0123456789")) }()

	limited := &readLimitedConn{Conn: clientSide, remaining: 4}
	buffer := make([]byte, 10)
	n, err := limited.Read(buffer)
	suite.Require().NoError(err)
	suite.Equal("0123", string(buffer[:n]))

	_, err = limited.Read(buffer)
	suite.ErrorIs(err, errSMTPReadLimitExceeded)
	_ = clientSide.Close()
}

// mockSMTPOptions configures the behavior of the in-process SMTP server below.
type mockSMTPOptions struct {
	tlsConfig     *tls.Config
	implicit      bool
	rejectCommand string
	stall         bool
	// flood sends an endless greeting line, as a hostile server would to exhaust memory.
	flood bool
}

// mockSMTPServer is a minimal SMTP server that records what it received.
type mockSMTPServer struct {
	listener net.Listener
	opts     mockSMTPOptions
	done     chan struct{}

	mu        sync.Mutex
	data      string
	mailFrom  string
	rcpt      []string
	startTLS  bool
	auth      string
	quitOK    bool
	closeOnce sync.Once
}

func newMockSMTPServer(t *testing.T, opts mockSMTPOptions) *mockSMTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	if opts.implicit {
		listener = tls.NewListener(listener, opts.tlsConfig)
	}

	server := &mockSMTPServer{listener: listener, opts: opts, done: make(chan struct{})}
	go server.serve()
	return server
}

func (server *mockSMTPServer) port() int {
	_, port, _ := net.SplitHostPort(server.listener.Addr().String())
	value, _ := strconv.Atoi(port)
	return value
}

func (server *mockSMTPServer) close() {
	server.closeOnce.Do(func() {
		close(server.done)
		_ = server.listener.Close()
	})
}

func (server *mockSMTPServer) message() string {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.data
}

// sender returns the address the client gave in MAIL FROM, which is the envelope sender.
func (server *mockSMTPServer) sender() string {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.mailFrom
}

func (server *mockSMTPServer) recipients() []string {
	server.mu.Lock()
	defer server.mu.Unlock()
	return append([]string{}, server.rcpt...)
}

// authCommand returns the AUTH command line the client issued, or the empty string when it
// issued none.
func (server *mockSMTPServer) authCommand() string {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.auth
}

func (server *mockSMTPServer) sawSTARTTLS() bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.startTLS
}

func (server *mockSMTPServer) quit() bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.quitOK
}

func (server *mockSMTPServer) serve() {
	for {
		connection, err := server.listener.Accept()
		if err != nil {
			return
		}
		go server.handle(connection)
	}
}

// handle speaks just enough SMTP to drive the client through a full session.
//
//nolint:gocognit,funlen,cyclop // a line-oriented protocol handler reads better as one switch
func (server *mockSMTPServer) handle(connection net.Conn) {
	defer func() { _ = connection.Close() }()

	if server.opts.stall {
		// Accept the connection and never greet, so only the client's deadline ends it.
		<-server.done
		return
	}

	if server.opts.flood {
		chunk := []byte("220-" + strings.Repeat("a", 64*1024))
		for {
			if _, err := connection.Write(chunk); err != nil {
				return
			}
		}
	}

	reader := bufio.NewReader(connection)
	write := func(line string) { _, _ = connection.Write([]byte(line + serverconst.CRLF)) }

	write("220 mock.local ESMTP")

	inData := false
	var body strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false
				server.mu.Lock()
				server.data = body.String()
				server.mu.Unlock()
				write("250 OK")
				continue
			}
			body.WriteString(line + "\n")
			continue
		}

		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			write("250-mock.local")
			write("250-AUTH PLAIN LOGIN")
			if server.opts.tlsConfig != nil && !server.opts.implicit {
				write("250-STARTTLS")
			}
			write("250 OK")
		case strings.HasPrefix(upper, "STARTTLS"):
			server.mu.Lock()
			server.startTLS = true
			server.mu.Unlock()
			write("220 Ready to start TLS")
			tlsConn := tls.Server(connection, server.opts.tlsConfig)
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			connection = tlsConn
			reader = bufio.NewReader(connection)
			write = func(line string) { _, _ = tlsConn.Write([]byte(line + serverconst.CRLF)) }
		case strings.HasPrefix(upper, "AUTH"):
			server.mu.Lock()
			server.auth = line
			server.mu.Unlock()
			if server.opts.rejectCommand == "AUTH" {
				// Echo the username as some servers do, so a test can tell whether it leaks.
				write("535 5.7.8 Authentication credentials invalid for mailer")
				continue
			}
			write("235 Authentication successful")
		case strings.HasPrefix(upper, "MAIL FROM"):
			if server.opts.rejectCommand == "MAIL FROM" {
				write("550 Rejected")
				continue
			}
			if address, ok := parseAngleAddress(line); ok {
				server.mu.Lock()
				server.mailFrom = address
				server.mu.Unlock()
			}
			write("250 OK")
		case strings.HasPrefix(upper, "RCPT TO"):
			address, ok := parseAngleAddress(line)
			if server.opts.rejectCommand == "RCPT TO" {
				// Echo the recipient as Postfix does, so a test can tell whether it leaks.
				write("550 5.1.1 <" + address + ">: Recipient address rejected")
				continue
			}
			if ok {
				server.mu.Lock()
				server.rcpt = append(server.rcpt, address)
				server.mu.Unlock()
			}
			write("250 OK")
		case strings.HasPrefix(upper, "DATA"):
			if server.opts.rejectCommand == "DATA" {
				write("554 Rejected")
				continue
			}
			inData = true
			write("354 End data with <CR><LF>.<CR><LF>")
		case strings.HasPrefix(upper, "QUIT"):
			if server.opts.rejectCommand == "QUIT" {
				write("500 Refused")
				return
			}
			server.mu.Lock()
			server.quitOK = true
			server.mu.Unlock()
			write("221 Bye")
			return
		default:
			write("250 OK")
		}
	}
}

// parseAngleAddress extracts the address from an SMTP command such as "RCPT TO:<a@b.test>".
func parseAngleAddress(line string) (string, bool) {
	start := strings.Index(line, "<")
	if start < 0 {
		return "", false
	}
	end := strings.Index(line[start:], ">")
	if end <= 0 {
		return "", false
	}
	return line[start+1 : start+end], true
}

// newSelfSignedTLSConfig returns a server TLS config with a certificate for "localhost" that
// no system root trusts, which is what makes it useful for asserting the client verifies.
func newSelfSignedTLSConfig(t *testing.T) *tls.Config {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate a test key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create a test certificate: %v", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{certificateDER}, PrivateKey: privateKey}},
		MinVersion:   tls.VersionTLS12,
	}
}
