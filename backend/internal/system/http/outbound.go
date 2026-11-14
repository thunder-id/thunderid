// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// DialerInterface opens network connections.
type DialerInterface interface {
	// DialContext connects to addr on the named network.
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// OutboundClientProviderInterface builds the HTTP clients and dialers for calls whose destination an
// administrator configures, such as a federated identity provider, a notification provider or an
// HTTP request step in a flow. Unless the deployment allows private addresses, every client and
// dialer it builds refuses to connect to a loopback, link-local, private or unspecified address,
// checked on the address actually connected to, so a hostname that resolves to one is refused too.
type OutboundClientProviderInterface interface {
	// NewHTTPClient returns a client with the given timeout that follows redirects. Every hop is
	// dialed through the same guard.
	NewHTTPClient(timeout time.Duration) HTTPClientInterface
	// NewHTTPClientWithoutRedirects returns a client with the given timeout that returns a 3xx
	// response instead of following it. Use it when a request carries a credential a redirect
	// would resend to another host.
	NewHTTPClientWithoutRedirects(timeout time.Duration) HTTPClientInterface
	// NewDialer returns a dialer whose connection attempts are bounded by the given timeout.
	NewDialer(timeout time.Duration) DialerInterface
}

// outboundClientProvider is the default implementation of OutboundClientProviderInterface.
type outboundClientProvider struct {
	rejectPrivate bool
}

// NewOutboundClientProvider creates the provider of clients and dialers for administrator-configured
// destinations. With allowPrivateAddresses they may reach loopback, link-local and private addresses.
func NewOutboundClientProvider(allowPrivateAddresses bool) OutboundClientProviderInterface {
	return &outboundClientProvider{rejectPrivate: !allowPrivateAddresses}
}

// NewHTTPClient returns a client with the given timeout that follows redirects.
func (p *outboundClientProvider) NewHTTPClient(timeout time.Duration) HTTPClientInterface {
	return &HTTPClient{client: &http.Client{Timeout: timeout, Transport: p.transport()}}
}

// NewHTTPClientWithoutRedirects returns a client with the given timeout that does not follow redirects.
func (p *outboundClientProvider) NewHTTPClientWithoutRedirects(timeout time.Duration) HTTPClientInterface {
	return &HTTPClient{
		client: &http.Client{
			Timeout:       timeout,
			Transport:     p.transport(),
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// NewDialer returns a dialer whose connection attempts are bounded by the given timeout.
func (p *outboundClientProvider) NewDialer(timeout time.Duration) DialerInterface {
	return newDialer(timeout, p.rejectPrivate)
}

// transport returns a transport that dials through the provider's dialer.
func (p *outboundClientProvider) transport() *http.Transport {
	return &http.Transport{
		DialContext: p.NewDialer(10 * time.Second).DialContext,
		// #nosec G402 -- Min TLS version is TLS 1.2 or higher based on config
		TLSClientConfig: &tls.Config{
			MinVersion: GetTLSVersion(config.GetServerRuntime().Config),
		},
	}
}
