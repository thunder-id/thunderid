// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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

// OutboundClientProviderInterface builds the HTTP clients and dialers for calls whose destination is
// configured rather than fixed, such as a federated identity provider, a notification provider, an
// HTTP request step in a flow, a client's JWKS URI or a gateway. Unless the provider allows private
// addresses, every client and dialer it builds refuses to connect to a loopback, link-local, private,
// reserved or unspecified address, checked on the address actually connected to, so a hostname that
// resolves to one is refused too.
type OutboundClientProviderInterface interface {
	// NewHTTPClient returns a client with the given timeout that follows redirects. Every hop is
	// dialed through the same guard.
	NewHTTPClient(timeout time.Duration) HTTPClientInterface
	// NewHTTPClientWithoutRedirects returns a client with the given timeout that returns a 3xx
	// response instead of following it. Use it when a request carries a credential a redirect
	// would resend to another host.
	NewHTTPClientWithoutRedirects(timeout time.Duration) HTTPClientInterface
	// NewHTTPClientWithCheckRedirect returns a client with the given timeout that applies checkRedirect
	// to every redirect, as http.Client.CheckRedirect does. Every hop is dialed through the same guard.
	NewHTTPClientWithCheckRedirect(timeout time.Duration,
		checkRedirect func(*http.Request, []*http.Request) error) HTTPClientInterface
	// NewHTTPClientWithRootCAs returns a client with the given timeout that trusts only rootCAs and
	// returns a 3xx response instead of following it. Use it to reach a server whose certificate a
	// private authority issued, with verification kept on.
	NewHTTPClientWithRootCAs(timeout time.Duration, rootCAs *x509.CertPool) HTTPClientInterface
	// NewUnguardedHTTPClient returns a client with the given timeout that follows redirects and reaches
	// any address, whether or not the provider allows private ones. Use it only for a destination the
	// deployment configuration sets, such as the trusted issuer's JWKS URL, never for one an
	// administrator or a client configures.
	NewUnguardedHTTPClient(timeout time.Duration) HTTPClientInterface
	// NewDialer returns a dialer whose connection attempts are bounded by the given timeout. Zero
	// leaves the bound to the context a dial is given.
	NewDialer(timeout time.Duration) DialerInterface
}

// outboundClientProvider is the default implementation of OutboundClientProviderInterface.
type outboundClientProvider struct {
	rejectPrivate bool
}

// newOutboundClientProvider creates the provider of clients and dialers for configured destinations.
// With allowPrivateAddresses they may reach loopback, link-local and private addresses.
func newOutboundClientProvider(allowPrivateAddresses bool) OutboundClientProviderInterface {
	return &outboundClientProvider{rejectPrivate: !allowPrivateAddresses}
}

// NewHTTPClient returns a client with the given timeout that follows redirects.
func (p *outboundClientProvider) NewHTTPClient(timeout time.Duration) HTTPClientInterface {
	return newClient(timeout, p.NewDialer(dialTimeout), nil, nil)
}

// NewHTTPClientWithoutRedirects returns a client with the given timeout that does not follow redirects.
func (p *outboundClientProvider) NewHTTPClientWithoutRedirects(timeout time.Duration) HTTPClientInterface {
	return newClient(timeout, p.NewDialer(dialTimeout), nil, refuseRedirects)
}

// NewHTTPClientWithCheckRedirect returns a client with the given timeout and redirect policy.
func (p *outboundClientProvider) NewHTTPClientWithCheckRedirect(timeout time.Duration,
	checkRedirect func(*http.Request, []*http.Request) error) HTTPClientInterface {
	return newClient(timeout, p.NewDialer(dialTimeout), nil, checkRedirect)
}

// NewHTTPClientWithRootCAs returns a client with the given timeout that trusts only rootCAs and does
// not follow redirects.
func (p *outboundClientProvider) NewHTTPClientWithRootCAs(timeout time.Duration,
	rootCAs *x509.CertPool) HTTPClientInterface {
	return newClient(timeout, p.NewDialer(dialTimeout), rootCAs, refuseRedirects)
}

// NewUnguardedHTTPClient returns a client with the given timeout that follows redirects and reaches any
// address.
func (p *outboundClientProvider) NewUnguardedHTTPClient(timeout time.Duration) HTTPClientInterface {
	return newClient(timeout, newDialer(dialTimeout, false), nil, nil)
}

// NewDialer returns a dialer whose connection attempts are bounded by the given timeout.
func (p *outboundClientProvider) NewDialer(timeout time.Duration) DialerInterface {
	return newDialer(timeout, p.rejectPrivate)
}

// dialTimeout bounds the connection attempts of a client's dialer.
const dialTimeout = 10 * time.Second

// newClient returns a client with the given timeout and redirect policy whose transport dials through
// dialer and, when rootCAs is set, trusts only those authorities. The transport sets no proxy, so the
// guard cannot be bypassed through HTTP_PROXY.
func newClient(timeout time.Duration, dialer DialerInterface, rootCAs *x509.CertPool,
	checkRedirect func(*http.Request, []*http.Request) error) HTTPClientInterface {
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DialContext: dialer.DialContext,
				// #nosec G402 -- Min TLS version is TLS 1.2 or higher based on config
				TLSClientConfig: &tls.Config{
					MinVersion: GetTLSVersion(config.GetServerRuntime().Config),
					RootCAs:    rootCAs,
				},
			},
			CheckRedirect: checkRedirect,
		},
	}
}

// refuseRedirects is the redirect policy that returns a 3xx response instead of following it.
func refuseRedirects(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}
