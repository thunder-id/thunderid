// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// privateAllowed lets the clients under test reach the loopback servers the tests start.
var privateAllowed = syshttp.NewOutboundClientProvider(true)

// privateTargets are hosts the outbound guard refuses: a loopback and a private address, the
// link-local metadata address, and a hostname that resolves to loopback.
var privateTargets = []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "localhost"}

type OutboundGuardTestSuite struct {
	suite.Suite
}

func TestOutboundGuardTestSuite(t *testing.T) {
	suite.Run(t, new(OutboundGuardTestSuite))
}

func (suite *OutboundGuardTestSuite) SetupSuite() {
	err := config.InitializeServerRuntime("", &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
	})
	suite.Require().NoError(err)
}

// messageClients builds each HTTP message client with the given provider, all pointed at url.
func (suite *OutboundGuardTestSuite) messageClients(
	clients syshttp.OutboundClientProviderInterface, url string) map[string]MessageClientInterface {
	custom, err := newCustomClient(context.Background(), common.NotificationSenderDTO{
		Name:     "custom",
		Provider: common.NotificationProviderTypeCustom,
		Properties: []cmodels.Property{
			createProperty(common.CustomPropKeyURL, url, false),
			createProperty(common.CustomPropKeyHTTPMethod, "POST", false),
			createProperty(common.CustomPropKeyContentType, "JSON", false),
		},
	}, clients)
	suite.Require().NoError(err)

	twilio, err := newTwilioClient(context.Background(), common.NotificationSenderDTO{
		Name:     "twilio",
		Provider: common.NotificationProviderTypeTwilio,
		Properties: []cmodels.Property{
			createProperty(common.TwilioPropKeyAccountSID, "AC00112233445566778899aabbccddeeff", true),
			createProperty(common.TwilioPropKeyAuthToken, "token", true),
			createProperty(common.TwilioPropKeySenderID, "+15551234567", false),
		},
	}, clients)
	suite.Require().NoError(err)
	twilio.(*TwilioClient).url = url

	vonage, err := newVonageClient(context.Background(), common.NotificationSenderDTO{
		Name:     "vonage",
		Provider: common.NotificationProviderTypeVonage,
		Properties: []cmodels.Property{
			createProperty(common.VonagePropKeyAPIKey, "key", true),
			createProperty(common.VonagePropKeyAPISecret, "secret", true),
			createProperty(common.VonagePropKeySenderID, "ThunderID", false),
		},
	}, clients)
	suite.Require().NoError(err)
	vonage.(*VonageClient).url = url

	return map[string]MessageClientInterface{"custom": custom, "twilio": twilio, "vonage": vonage}
}

func (suite *OutboundGuardTestSuite) TestMessageClients_RefusePrivateAddresses() {
	refusing := syshttp.NewOutboundClientProvider(false)
	for _, host := range privateTargets {
		for name, client := range suite.messageClients(refusing, "http://"+host+":8080/sms") {
			err := client.Send(context.Background(), common.ChannelTypeSMS,
				common.MessageData{Recipient: "+15559876543", Body: `{"message":"hi"}`})
			suite.ErrorIs(err, syshttp.ErrPrivateAddress, "%s client reached %s", name, host)
		}
	}
}

func (suite *OutboundGuardTestSuite) TestMessageClients_ReachPrivateAddressesWhenAllowed() {
	// 202 is what Vonage answers a send with; the custom gateway and Twilio take any 2xx.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"messages":[{"status":"0"}]}`))
	}))
	defer server.Close()

	for _, url := range []string{server.URL, strings.Replace(server.URL, "127.0.0.1", "localhost", 1)} {
		for name, client := range suite.messageClients(privateAllowed, url) {
			err := client.Send(context.Background(), common.ChannelTypeSMS,
				common.MessageData{Recipient: "+15559876543", Body: `{"message":"hi"}`})
			suite.NoError(err, "%s client did not send to %s", name, url)
		}
	}
}

// smtpClient builds an SMTP client with the given provider against host and port.
func (suite *OutboundGuardTestSuite) smtpClient(clients syshttp.OutboundClientProviderInterface, host string,
	port int, tlsMode common.TLSMode) EmailClientInterface {
	client, err := newSMTPEmailClient(context.Background(), common.NotificationSenderDTO{
		Name:     "smtp",
		Type:     common.NotificationSenderTypeEmail,
		Provider: common.NotificationProviderTypeSMTP,
		Properties: []cmodels.Property{
			createProperty(common.SMTPPropKeyHost, host, false),
			createProperty(common.SMTPPropKeyPort, strconv.Itoa(port), false),
			createProperty(common.SMTPPropKeyFromAddress, "noreply@example.com", false),
			createProperty(common.SMTPPropKeyTLS, string(tlsMode), false),
		},
	}, clients)
	suite.Require().NoError(err)
	return client
}

func (suite *OutboundGuardTestSuite) TestSMTPClient_RefusesPrivateAddresses() {
	refusing := syshttp.NewOutboundClientProvider(false)
	email := common.EmailData{To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}
	for _, host := range privateTargets {
		for _, tlsMode := range []common.TLSMode{common.TLSModeNone, common.TLSModeSTARTTLS, common.TLSModeImplicit} {
			err := suite.smtpClient(refusing, host, 2525, tlsMode).Send(context.Background(), email)
			suite.ErrorIs(err, syshttp.ErrPrivateAddress, "reached %s with TLS mode %s", host, tlsMode)
		}
	}
}

func (suite *OutboundGuardTestSuite) TestSMTPClient_ReachesPrivateAddressesWhenAllowed() {
	email := common.EmailData{To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}
	for _, host := range []string{"127.0.0.1", "localhost"} {
		server := newMockSMTPServer(suite.T(), mockSMTPOptions{})
		err := suite.smtpClient(privateAllowed, host, server.port(), common.TLSModeNone).
			Send(context.Background(), email)
		server.close()
		suite.NoError(err, host)
	}
}
