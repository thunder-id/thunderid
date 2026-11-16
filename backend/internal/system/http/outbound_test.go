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

type OutboundClientFactoryTestSuite struct {
	suite.Suite
}

func TestOutboundClientFactoryTestSuite(t *testing.T) {
	suite.Run(t, new(OutboundClientFactoryTestSuite))
}

func (suite *OutboundClientFactoryTestSuite) SetupSuite() {
	suite.Require().NoError(config.InitializeServerRuntime("", &config.Config{}))
}

// clients returns every HTTP client the factory builds.
func clients(factory OutboundClientFactoryInterface) map[string]HTTPClientInterface {
	return map[string]HTTPClientInterface{
		"redirecting":     factory.NewHTTPClient(time.Second),
		"non-redirecting": factory.NewHTTPClientWithoutRedirects(time.Second),
		"check redirect":  factory.NewHTTPClientWithCheckRedirect(time.Second, nil),
		"root CAs":        factory.NewHTTPClientWithRootCAs(time.Second, x509.NewCertPool()),
	}
}

func (suite *OutboundClientFactoryTestSuite) TestRefusesPrivateAddressesByDefault() {
	factory := newOutboundClientFactory(false)
	for _, host := range privateTargets {
		for name, client := range clients(factory) {
			resp, err := client.Get("http://" + host + ":8080/")
			if resp != nil {
				_ = resp.Body.Close()
			}
			suite.ErrorIs(err, ErrPrivateAddress, "%s client reached %s", name, host)
		}

		conn, err := factory.NewDialer(time.Second).DialContext(context.Background(), "tcp",
			net.JoinHostPort(host, "25"))
		if conn != nil {
			_ = conn.Close()
		}
		suite.ErrorIs(err, ErrPrivateAddress, "dialer reached %s", host)
	}
}

func (suite *OutboundClientFactoryTestSuite) TestReachesPrivateAddressesWhenAllowed() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	factory := newOutboundClientFactory(true)
	for _, url := range []string{server.URL, strings.Replace(server.URL, "127.0.0.1", "localhost", 1)} {
		for name, client := range clients(factory) {
			resp, err := client.Get(url)
			suite.Require().NoError(err, "%s client was refused %s", name, url)
			_ = resp.Body.Close()
			suite.Equal(http.StatusOK, resp.StatusCode)
		}

		conn, err := factory.NewDialer(time.Second).DialContext(context.Background(), "tcp",
			strings.TrimPrefix(url, "http://"))
		suite.Require().NoError(err, "dialer was refused %s", url)
		_ = conn.Close()
	}
}

func (suite *OutboundClientFactoryTestSuite) TestNonRedirectingClientReturnsTheRedirect() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	defer server.Close()

	resp, err := newOutboundClientFactory(true).NewHTTPClientWithoutRedirects(time.Second).Get(server.URL)
	suite.Require().NoError(err)
	_ = resp.Body.Close()
	suite.Equal(http.StatusFound, resp.StatusCode)
}

// Initialize builds the factory the flag asks for.
func (suite *OutboundClientFactoryTestSuite) TestInitialize() {
	suite.Equal(&outboundClientFactory{rejectPrivate: true}, Initialize(false))
	suite.Equal(&outboundClientFactory{rejectPrivate: false}, Initialize(true))
}

// The unguarded client reaches a private address even from a factory that refuses them, for a
// destination the deployment configuration sets.
func (suite *OutboundClientFactoryTestSuite) TestUnguardedClientReachesPrivateAddresses() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := Initialize(false).NewUnguardedHTTPClient(time.Second).Get(server.URL)
	suite.Require().NoError(err)
	_ = resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)
}

// The shipped default.json leaves server.allow_private_addresses off, so the factory the service
// managers build from it refuses a private address, and reaches it once a deployment turns it on.
func (suite *OutboundClientFactoryTestSuite) TestShippedDefaultRefusesPrivateAddresses() {
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

// transportOf returns the transport a client from the factory sends through.
func transportOf(client HTTPClientInterface) http.RoundTripper {
	return client.(*HTTPClient).client.Transport
}

// The guarded clients share one transport, so they reuse connections; the unguarded client shares it
// only where the factory allows private addresses, and a client with its own root CAs has its own.
func (suite *OutboundClientFactoryTestSuite) TestClientsShareTheFactoryTransport() {
	refusing := newOutboundClientFactory(false)
	guarded := transportOf(refusing.NewHTTPClient(time.Second))
	suite.Same(guarded, transportOf(refusing.NewHTTPClient(time.Minute)))
	suite.Same(guarded, transportOf(refusing.NewHTTPClientWithoutRedirects(time.Second)))
	suite.Same(guarded, transportOf(refusing.NewHTTPClientWithCheckRedirect(time.Second, nil)))
	suite.NotSame(guarded, transportOf(refusing.NewUnguardedHTTPClient(time.Second)))
	suite.NotSame(guarded, transportOf(refusing.NewHTTPClientWithRootCAs(time.Second, x509.NewCertPool())))

	allowing := newOutboundClientFactory(true)
	suite.Same(transportOf(allowing.NewHTTPClient(time.Second)),
		transportOf(allowing.NewUnguardedHTTPClient(time.Second)))
}
