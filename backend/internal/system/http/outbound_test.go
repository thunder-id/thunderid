// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// privateTargets are hosts the outbound guard refuses: a loopback and a private address, the
// link-local metadata address, and a hostname that resolves to loopback.
var privateTargets = []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "localhost"}

type OutboundClientProviderTestSuite struct {
	suite.Suite
}

func TestOutboundClientProviderTestSuite(t *testing.T) {
	suite.Run(t, new(OutboundClientProviderTestSuite))
}

func (suite *OutboundClientProviderTestSuite) SetupSuite() {
	suite.Require().NoError(config.InitializeServerRuntime("", &config.Config{}))
}

// clients returns every HTTP client the provider builds.
func clients(provider OutboundClientProviderInterface) map[string]HTTPClientInterface {
	return map[string]HTTPClientInterface{
		"redirecting":     provider.NewHTTPClient(time.Second),
		"non-redirecting": provider.NewHTTPClientWithoutRedirects(time.Second),
		"check redirect":  provider.NewHTTPClientWithCheckRedirect(time.Second, nil),
		"root CAs":        provider.NewHTTPClientWithRootCAs(time.Second, x509.NewCertPool()),
	}
}

func (suite *OutboundClientProviderTestSuite) TestRefusesPrivateAddressesByDefault() {
	provider := newOutboundClientProvider(false)
	for _, host := range privateTargets {
		for name, client := range clients(provider) {
			resp, err := client.Get("http://" + host + ":8080/")
			if resp != nil {
				_ = resp.Body.Close()
			}
			suite.ErrorIs(err, ErrPrivateAddress, "%s client reached %s", name, host)
		}

		conn, err := provider.NewDialer(time.Second).DialContext(context.Background(), "tcp",
			net.JoinHostPort(host, "25"))
		if conn != nil {
			_ = conn.Close()
		}
		suite.ErrorIs(err, ErrPrivateAddress, "dialer reached %s", host)
	}
}

func (suite *OutboundClientProviderTestSuite) TestReachesPrivateAddressesWhenAllowed() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newOutboundClientProvider(true)
	for _, url := range []string{server.URL, strings.Replace(server.URL, "127.0.0.1", "localhost", 1)} {
		for name, client := range clients(provider) {
			resp, err := client.Get(url)
			suite.Require().NoError(err, "%s client was refused %s", name, url)
			_ = resp.Body.Close()
			suite.Equal(http.StatusOK, resp.StatusCode)
		}

		conn, err := provider.NewDialer(time.Second).DialContext(context.Background(), "tcp",
			strings.TrimPrefix(url, "http://"))
		suite.Require().NoError(err, "dialer was refused %s", url)
		_ = conn.Close()
	}
}

func (suite *OutboundClientProviderTestSuite) TestNonRedirectingClientReturnsTheRedirect() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	defer server.Close()

	resp, err := newOutboundClientProvider(true).NewHTTPClientWithoutRedirects(time.Second).Get(server.URL)
	suite.Require().NoError(err)
	_ = resp.Body.Close()
	suite.Equal(http.StatusFound, resp.StatusCode)
}

// Initialize builds the provider the flag asks for.
func (suite *OutboundClientProviderTestSuite) TestInitialize() {
	suite.Equal(&outboundClientProvider{rejectPrivate: true}, Initialize(false))
	suite.Equal(&outboundClientProvider{rejectPrivate: false}, Initialize(true))
}

// The unguarded client reaches a private address even from a provider that refuses them, for a
// destination the deployment configuration sets.
func (suite *OutboundClientProviderTestSuite) TestUnguardedClientReachesPrivateAddresses() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := Initialize(false).NewUnguardedHTTPClient(time.Second).Get(server.URL)
	suite.Require().NoError(err)
	_ = resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)
}

// The shipped default.json leaves server.allow_private_addresses off, so the provider the service
// managers build from it refuses a private address, and reaches it once a deployment turns it on.
func (suite *OutboundClientProviderTestSuite) TestShippedDefaultRefusesPrivateAddresses() {
	data, err := os.ReadFile("../../../cmd/server/config/default.json")
	suite.Require().NoError(err)
	var cfg config.Config
	suite.Require().NoError(json.Unmarshal(data, &cfg))
	suite.False(cfg.Server.AllowPrivateAddresses)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	suite.Require().NoError(err)
	defer func() { _ = listener.Close() }()
	dial := func(allowPrivateAddresses bool) error {
		conn, err := Initialize(allowPrivateAddresses).NewDialer(time.Second).DialContext(context.Background(),
			"tcp", listener.Addr().String())
		if conn != nil {
			_ = conn.Close()
		}
		return err
	}

	suite.ErrorIs(dial(cfg.Server.AllowPrivateAddresses), ErrPrivateAddress)
	suite.NoError(dial(true))
}
