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
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/smtpauth"
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
		isSecret := name == outboundauth.PropertyKey(outboundauth.FieldBasicPassword)
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
	props[outboundauth.PropertyKeyType] = string(outboundauth.TypeBasic)
	if username != "" {
		props[outboundauth.PropertyKey(outboundauth.FieldBasicUsername)] = username
	}
	if password != "" {
		props[outboundauth.PropertyKey(outboundauth.FieldBasicPassword)] = password
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

	auth, err := client.(*smtpEmailClient).auth.Auth(context.Background(), config.host)
	suite.Require().NoError(err)
	suite.Nil(auth)
}

func (suite *SMTPEmailClientTestSuite) TestNewClient_InvalidConfig() {
	cases := []struct {
		name   string
		mutate func(map[string]string)
	}{
		{"missing host", func(p map[string]string) { delete(p, common.SMTPPropKeyHost) }},
		{"missing port", func(p map[string]string) { delete(p, common.SMTPPropKeyPort) }},
		{"non numeric port", func(p map[string]string) { p[common.SMTPPropKeyPort] = "abc" }},
		{"port out of range", func(p map[string]string) { p[common.SMTPPropKeyPort] = "70000" }},
		{"missing from address", func(p map[string]string) { delete(p, common.SMTPPropKeyFromAddress) }},
		{"invalid from address", func(p map[string]string) { p[common.SMTPPropKeyFromAddress] = "not-an-address" }},
		{"display name from address", func(p map[string]string) {
			p[common.SMTPPropKeyFromAddress] = `"Bot" <bot@example.com>`
		}},
		{"invalid tls mode", func(p map[string]string) { p[common.SMTPPropKeyTLS] = "yes" }},
		{"auth without tls", func(p map[string]string) {
			withBasicAuth(p, "u", "p")
		}},
		{"auth without credentials", func(p map[string]string) {
			withBasicAuth(p, "", "")
			p[common.SMTPPropKeyTLS] = string(common.TLSModeSTARTTLS)
		}},
		{"auth without password", func(p map[string]string) {
			withBasicAuth(p, "u", "")
			p[common.SMTPPropKeyTLS] = string(common.TLSModeSTARTTLS)
		}},
		{"unsupported auth type", func(p map[string]string) {
			p[outboundauth.PropertyKeyType] = "bearer"
		}},
		// A line break in the name would end the From header and let the rest be read as
		// headers of its own.
		{"from name with a line break", func(p map[string]string) {
			p[common.SMTPPropKeyFromName] = "Acme\r\nBcc: attacker@evil.test"
		}},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			props := suite.baseProps(587)
			tc.mutate(props)
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

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			client := &smtpEmailClient{config: smtpConfig{from: "noreply@example.com", fromName: tc.fromName}}
			suite.Equal(tc.expected, client.fromHeader())
		})
	}
}

// BCC recipients belong in the envelope only; leaking them into the headers would disclose
// them to every other recipient.
// clientWithAuth builds a client straight from a configuration, bypassing the policy that
// refuses credentials over a plaintext connection. That policy is covered by the validation
// tests; this exists to observe what the configured method actually puts on the wire, which the
// TLS-bearing mock cannot show because its self-signed certificate is rejected before AUTH.
func (suite *SMTPEmailClientTestSuite) clientWithAuth(port int, cfg outboundauth.Config) *smtpEmailClient {
	authenticator, err := smtpauth.New(cfg)
	suite.Require().NoError(err)

	return &smtpEmailClient{
		name: "Test SMTP",
		config: smtpConfig{
			host:    "127.0.0.1",
			port:    port,
			from:    "noreply@example.com",
			tlsMode: common.TLSModeNone,
			auth:    cfg,
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

func (suite *SMTPEmailClientTestSuite) TestSend_NoAuthenticationIssuesNoAuthCommand() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{})
	defer server.close()

	client := suite.clientWithAuth(server.port(), outboundauth.Config{Type: outboundauth.TypeNone})
	suite.Require().NoError(client.Send(context.Background(), suite.emailData()))

	suite.Empty(server.authCommand(), "an unauthenticated sender must not issue AUTH")
}

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

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.Error(client.Send(context.Background(), tc.data))
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
	}{
		{"mail from", "MAIL FROM"},
		{"rcpt to", "RCPT TO"},
		{"data", "DATA"},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			server := newMockSMTPServer(suite.T(), mockSMTPOptions{rejectCommand: tc.rejects})
			defer server.close()

			client := suite.newClient(server.port(), nil)
			err := client.Send(context.Background(), suite.emailData())
			suite.Require().Error(err)
			suite.Contains(err.Error(), "email send failed")
		})
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

// A server that accepts the connection and then goes silent must not hang the caller: the
// legacy client bounded only the dial, so this is the regression this deadline guards.
func (suite *SMTPEmailClientTestSuite) TestSend_StalledServerTimesOut() {
	server := newMockSMTPServer(suite.T(), mockSMTPOptions{stall: true})
	defer server.close()

	original := smtpSessionTimeout
	smtpSessionTimeout = 200 * time.Millisecond
	defer func() { smtpSessionTimeout = original }()

	client := suite.newClient(server.port(), nil)

	done := make(chan error, 1)
	go func() { done <- client.Send(context.Background(), suite.emailData()) }()

	select {
	case err := <-done:
		suite.Error(err)
	case <-time.After(10 * time.Second):
		suite.Fail("Send did not honor the session deadline")
	}
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

// mockSMTPOptions configures the behavior of the in-process SMTP server below.
type mockSMTPOptions struct {
	tlsConfig     *tls.Config
	implicit      bool
	rejectCommand string
	stall         bool
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

func (m *mockSMTPServer) port() int {
	_, port, _ := net.SplitHostPort(m.listener.Addr().String())
	value, _ := strconv.Atoi(port)
	return value
}

func (m *mockSMTPServer) close() {
	m.closeOnce.Do(func() {
		close(m.done)
		_ = m.listener.Close()
	})
}

func (m *mockSMTPServer) message() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data
}

// sender returns the address the client gave in MAIL FROM, which is the envelope sender.
func (m *mockSMTPServer) sender() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mailFrom
}

func (m *mockSMTPServer) recipients() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.rcpt...)
}

// authCommand returns the AUTH command line the client issued, or the empty string when it
// issued none.
func (m *mockSMTPServer) authCommand() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.auth
}

func (m *mockSMTPServer) sawSTARTTLS() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startTLS
}

func (m *mockSMTPServer) quit() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.quitOK
}

func (m *mockSMTPServer) serve() {
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			return
		}
		go m.handle(conn)
	}
}

// handle speaks just enough SMTP to drive the client through a full session.
//
//nolint:gocognit,funlen,cyclop // a line-oriented protocol handler reads better as one switch
func (m *mockSMTPServer) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	if m.opts.stall {
		// Accept the connection and never greet, so only the client's deadline ends it.
		<-m.done
		return
	}

	reader := bufio.NewReader(conn)
	write := func(line string) { _, _ = conn.Write([]byte(line + common.CRLF)) }

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
				m.mu.Lock()
				m.data = body.String()
				m.mu.Unlock()
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
			if m.opts.tlsConfig != nil && !m.opts.implicit {
				write("250-STARTTLS")
			}
			write("250 OK")
		case strings.HasPrefix(upper, "STARTTLS"):
			m.mu.Lock()
			m.startTLS = true
			m.mu.Unlock()
			write("220 Ready to start TLS")
			tlsConn := tls.Server(conn, m.opts.tlsConfig)
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn = tlsConn
			reader = bufio.NewReader(conn)
			write = func(line string) { _, _ = tlsConn.Write([]byte(line + common.CRLF)) }
		case strings.HasPrefix(upper, "AUTH"):
			m.mu.Lock()
			m.auth = line
			m.mu.Unlock()
			write("235 Authentication successful")
		case strings.HasPrefix(upper, "MAIL FROM"):
			if m.opts.rejectCommand == "MAIL FROM" {
				write("550 Rejected")
				continue
			}
			if address, ok := parseAngleAddress(line); ok {
				m.mu.Lock()
				m.mailFrom = address
				m.mu.Unlock()
			}
			write("250 OK")
		case strings.HasPrefix(upper, "RCPT TO"):
			if m.opts.rejectCommand == "RCPT TO" {
				write("550 Rejected")
				continue
			}
			if address, ok := parseAngleAddress(line); ok {
				m.mu.Lock()
				m.rcpt = append(m.rcpt, address)
				m.mu.Unlock()
			}
			write("250 OK")
		case strings.HasPrefix(upper, "DATA"):
			if m.opts.rejectCommand == "DATA" {
				write("554 Rejected")
				continue
			}
			inData = true
			write("354 End data with <CR><LF>.<CR><LF>")
		case strings.HasPrefix(upper, "QUIT"):
			if m.opts.rejectCommand == "QUIT" {
				write("500 Refused")
				return
			}
			m.mu.Lock()
			m.quitOK = true
			m.mu.Unlock()
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

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create a test certificate: %v", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
		MinVersion:   tls.VersionTLS12,
	}
}
