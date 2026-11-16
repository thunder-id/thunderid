// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// OutboundClientFactoryInterface builds the HTTP clients and dialers for outbound calls, such as to a
// federated identity provider, a notification provider, an HTTP request step in a flow, a client's
// JWKS URI or a gateway. Unless the factory allows private addresses, every client and dialer it
// builds refuses to connect to a loopback, link-local, private, reserved or unspecified address,
// checked on the address actually connected to, so a hostname that resolves to one is refused too.
type OutboundClientFactoryInterface interface {
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
	// private authority issued, with verification kept on. The client has its own transport, so
	// release it with CloseIdleConnections once done.
	NewHTTPClientWithRootCAs(timeout time.Duration, rootCAs *x509.CertPool) HTTPClientInterface
	// NewUnguardedHTTPClient returns a client with the given timeout that follows redirects and reaches
	// any address, whether or not the factory allows private ones. Use it only for a destination the
	// deployment configuration sets, such as the trusted issuer's JWKS URL, never for one an
	// administrator or a client configures.
	NewUnguardedHTTPClient(timeout time.Duration) HTTPClientInterface
	// NewDialer returns a dialer whose connection attempts are bounded by the given timeout. Zero
	// leaves the bound to the context a dial is given.
	NewDialer(timeout time.Duration) DialerInterface
}

// outboundClientFactory is the default implementation of OutboundClientFactoryInterface. Its clients
// share one guarded and one unguarded transport, built on first use, so they reuse connections.
type outboundClientFactory struct {
	rejectPrivate bool
	once          sync.Once
	guarded       *http.Transport
	unguarded     *http.Transport
}

// newOutboundClientFactory creates the factory of clients and dialers for outbound calls. With
// allowPrivateAddresses they may reach loopback, link-local and private addresses.
func newOutboundClientFactory(allowPrivateAddresses bool) OutboundClientFactoryInterface {
	return &outboundClientFactory{rejectPrivate: !allowPrivateAddresses}
}

// NewHTTPClient returns a client with the given timeout that follows redirects.
func (f *outboundClientFactory) NewHTTPClient(timeout time.Duration) HTTPClientInterface {
	return newClient(timeout, f.guardedTransport(), nil)
}

// NewHTTPClientWithoutRedirects returns a client with the given timeout that does not follow redirects.
func (f *outboundClientFactory) NewHTTPClientWithoutRedirects(timeout time.Duration) HTTPClientInterface {
	return newClient(timeout, f.guardedTransport(), refuseRedirects)
}

// NewHTTPClientWithCheckRedirect returns a client with the given timeout and redirect policy.
func (f *outboundClientFactory) NewHTTPClientWithCheckRedirect(timeout time.Duration,
	checkRedirect func(*http.Request, []*http.Request) error) HTTPClientInterface {
	return newClient(timeout, f.guardedTransport(), checkRedirect)
}

// NewHTTPClientWithRootCAs returns a client with the given timeout that trusts only rootCAs and does
// not follow redirects.
func (f *outboundClientFactory) NewHTTPClientWithRootCAs(timeout time.Duration,
	rootCAs *x509.CertPool) HTTPClientInterface {
	return newClient(timeout, newTransport(f.NewDialer(dialTimeout), rootCAs), refuseRedirects)
}

// NewUnguardedHTTPClient returns a client with the given timeout that follows redirects and reaches any
// address.
func (f *outboundClientFactory) NewUnguardedHTTPClient(timeout time.Duration) HTTPClientInterface {
	f.buildTransports()
	return newClient(timeout, f.unguarded, nil)
}

// NewDialer returns a dialer whose connection attempts are bounded by the given timeout.
func (f *outboundClientFactory) NewDialer(timeout time.Duration) DialerInterface {
	return newDialer(timeout, f.rejectPrivate)
}

// guardedTransport returns the transport shared by the clients that dial through the guard.
func (f *outboundClientFactory) guardedTransport() *http.Transport {
	f.buildTransports()
	return f.guarded
}

// buildTransports builds the shared transports once. They read the TLS settings from the server
// runtime, which is loaded by the time a client is built.
func (f *outboundClientFactory) buildTransports() {
	f.once.Do(func() {
		f.unguarded = newTransport(newDialer(dialTimeout, false), nil)
		f.guarded = f.unguarded
		if f.rejectPrivate {
			f.guarded = newTransport(newDialer(dialTimeout, true), nil)
		}
	})
}

// newTransport returns a transport that dials through dialer and, when rootCAs is set, trusts only
// those authorities. It sets no proxy, so the guard cannot be bypassed through HTTP_PROXY.
func newTransport(dialer DialerInterface, rootCAs *x509.CertPool) *http.Transport {
	return &http.Transport{
		DialContext: dialer.DialContext,
		// #nosec G402 -- Min TLS version is TLS 1.2 or higher based on config
		TLSClientConfig: &tls.Config{
			MinVersion: GetTLSVersion(config.GetServerRuntime().Config),
			RootCAs:    rootCAs,
		},
	}
}

// newClient returns a client with the given timeout and redirect policy that sends through transport.
func newClient(timeout time.Duration, transport *http.Transport,
	checkRedirect func(*http.Request, []*http.Request) error) HTTPClientInterface {
	return &HTTPClient{
		client: &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: checkRedirect},
	}
}

// refuseRedirects is the redirect policy that returns a 3xx response instead of following it.
func refuseRedirects(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}
