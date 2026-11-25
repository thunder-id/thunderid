// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/constants"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
)

// gatewayFor serves an import over TLS and returns the gateway registered for it, trusting the
// server's certificate the way a registration naming caCertificate does.
func gatewayFor(t *testing.T, handler http.HandlerFunc) *Gateway {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	return &Gateway{ID: "gw-1", BaseURL: server.URL, CACertificate: string(certificate)}
}

// The import is posted to the gateway with its key in the management API key header, verified
// against the certificate authority the registration names.
func TestTheClientPostsTheImportWithTheKey(t *testing.T) {
	var got gatewayImportRequest
	var presented string
	gw := gatewayFor(t, func(w http.ResponseWriter, r *http.Request) {
		presented = r.Header.Get(constants.APIKeyHeaderName)
		assert.Equal(t, "/import/managed", r.URL.Path)
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"summary":{"failed":0}}`))
	})

	answer, plain, err := newGatewayClient().Import(context.Background(), gw, "the-key",
		gatewayImportRequest{Content: "docs", DryRun: true})

	require.NoError(t, err)
	assert.False(t, plain, "the managed import is not a fallback")
	assert.JSONEq(t, `{"summary":{"failed":0}}`, string(answer))
	assert.Equal(t, "the-key", presented)
	assert.Equal(t, "docs", got.Content)
	assert.True(t, got.DryRun)
}

// A gateway that predates the managed import answers 404 for it, so the deploy goes to the plain
// import with the same body and key.
func TestTheClientFallsBackToThePlainImportWhenThereIsNoManagedImport(t *testing.T) {
	var paths []string
	var got gatewayImportRequest
	var presented string
	gw := gatewayFor(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/import/managed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		presented = r.Header.Get(constants.APIKeyHeaderName)
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"summary":{"failed":0}}`))
	})

	answer, plain, err := newGatewayClient().Import(context.Background(), gw, "the-key",
		gatewayImportRequest{Content: "docs"})

	require.NoError(t, err)
	assert.True(t, plain, "a deploy through the plain import is reported")
	assert.Equal(t, []string{"/import/managed", "/import"}, paths)
	assert.JSONEq(t, `{"summary":{"failed":0}}`, string(answer))
	assert.Equal(t, "the-key", presented)
	assert.Equal(t, "docs", got.Content)
}

// Only a missing route falls back: a gateway that refuses the managed import for another reason is
// not asked again through the plain one.
func TestTheClientDoesNotFallBackForOtherRefusals(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		var paths []string
		gw := gatewayFor(t, func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.URL.Path)
			w.WriteHeader(status)
		})

		_, _, err := newGatewayClient().Import(context.Background(), gw, "k", gatewayImportRequest{})

		assert.True(t, errors.Is(err, errGatewayRefused), "status %d", status)
		assert.Equal(t, []string{"/import/managed"}, paths, "status %d", status)
	}
}

// A gateway with neither import is reported as refusing, and nothing was written.
func TestTheClientReportsAGatewayWithNoImport(t *testing.T) {
	gw := gatewayFor(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })

	_, _, err := newGatewayClient().Import(context.Background(), gw, "k", gatewayImportRequest{})

	assert.True(t, errors.Is(err, errGatewayRefused))
	assert.True(t, errors.Is(err, errImportNotWritten))
}

// A gateway answering with anything but success is reported as refusing.
func TestTheClientReportsARefusal(t *testing.T) {
	gw := gatewayFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, _, err := newGatewayClient().Import(context.Background(), gw, "wrong", gatewayImportRequest{})

	assert.True(t, errors.Is(err, errGatewayRefused))
	assert.True(t, errors.Is(err, errImportNotWritten), "a 4xx is answered before anything is written")
}

// Only a failure that shows the gateway received none of the import says so: a server error or a
// connection lost after the request went out leaves open whether the gateway wrote it.
func TestTheClientSaysWhenTheGatewayWroteNothing(t *testing.T) {
	failing := gatewayFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, _, err := newGatewayClient().Import(context.Background(), failing, "k", gatewayImportRequest{})
	assert.True(t, errors.Is(err, errGatewayRefused))
	assert.False(t, errors.Is(err, errImportNotWritten), "a server error may follow a partial write")

	hangingUp := gatewayFor(t, func(w http.ResponseWriter, _ *http.Request) {
		conn, _, hijackErr := w.(http.Hijacker).Hijack()
		require.NoError(t, hijackErr)
		_ = conn.Close()
	})
	_, _, err = newGatewayClient().Import(context.Background(), hangingUp, "k", gatewayImportRequest{})
	assert.Error(t, err)
	assert.False(t, errors.Is(err, errImportNotWritten), "a connection lost after sending may have written")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	closedAddress := listener.Addr().String()
	require.NoError(t, listener.Close())
	_, _, err = newGatewayClient().Import(context.Background(),
		&Gateway{ID: "gw-1", BaseURL: "https://" + closedAddress}, "k", gatewayImportRequest{})
	assert.True(t, errors.Is(err, errImportNotWritten), "a refused connection sent nothing")

	untrusted := *failing
	untrusted.CACertificate = ""
	_, _, err = newGatewayClient().Import(context.Background(), &untrusted, "k", gatewayImportRequest{})
	assert.True(t, errors.Is(err, errImportNotWritten), "a certificate that does not verify sent nothing")

	_, _, err = newGatewayClient().Import(context.Background(), &Gateway{ID: "gw-1", BaseURL: "http://dp.test"}, "k",
		gatewayImportRequest{})
	assert.True(t, errors.Is(err, errImportNotWritten), "a plain HTTP gateway is never called")
}

// A gateway the dialer refuses for its private address was never called, so its import is reported
// as not written and an apply puts back what it saved before sending.
func TestAnImportRefusedForAPrivateAddressWroteNothing(t *testing.T) {
	refusePrivateAddresses(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	_, _, err = newGatewayClient().Import(context.Background(),
		&Gateway{ID: "gw-1", BaseURL: "https://" + listener.Addr().String()}, "k", gatewayImportRequest{})

	require.Error(t, err)
	assert.True(t, errors.Is(err, syshttp.ErrPrivateAddress), "the dialer did not refuse the address")
	assert.True(t, errors.Is(err, errImportNotWritten), "a refused private address sent nothing")
}

// Verification stays on: a gateway whose certificate the registration does not trust is refused,
// and so is a certificate authority that is not a certificate.
func TestTheClientVerifiesTheGateway(t *testing.T) {
	gw := gatewayFor(t, func(http.ResponseWriter, *http.Request) {})

	untrusted := *gw
	untrusted.CACertificate = ""
	_, _, err := newGatewayClient().Import(context.Background(), &untrusted, "k", gatewayImportRequest{})
	assert.Error(t, err)

	broken := *gw
	broken.CACertificate = "not a certificate"
	_, _, err = newGatewayClient().Import(context.Background(), &broken, "k", gatewayImportRequest{})
	assert.Error(t, err)
}

// The key is never sent over plain HTTP: a gateway registered at an http address is refused.
func TestTheClientRefusesAPlainHTTPGateway(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	t.Cleanup(server.Close)

	_, _, err := newGatewayClient().Import(context.Background(), &Gateway{ID: "gw-1", BaseURL: server.URL}, "k",
		gatewayImportRequest{})

	assert.Error(t, err)
	assert.False(t, called, "the key was sent over plain HTTP")
}

// A redirect is not followed, so the key's header is never handed to where it points.
func TestTheClientFollowsNoRedirect(t *testing.T) {
	followed := false
	target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed = true }))
	t.Cleanup(target.Close)
	gw := gatewayFor(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/import/managed", http.StatusTemporaryRedirect)
	})

	_, err := newGatewayClient().ListVariables(context.Background(), gw, "k", StoreListQuery{})

	var refusal *StoreRefusal
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, http.StatusTemporaryRedirect, refusal.Status)
	assert.False(t, followed, "the redirect was followed")
}

// Unless the deployment allows gateways on private addresses, a gateway whose name resolves to one
// is refused as it is dialed, so the key never reaches something inside this plane's own network.
func TestTheClientRefusesAPrivateAddressUnlessAllowed(t *testing.T) {
	refusePrivateAddresses(t)
	called := false
	gw := gatewayFor(t, func(http.ResponseWriter, *http.Request) { called = true })

	_, _, err := newGatewayClient().Import(context.Background(), gw, "k", gatewayImportRequest{})

	require.Error(t, err)
	assert.True(t, errors.Is(err, syshttp.ErrPrivateAddress), "refused for another reason: %v", err)
	assert.False(t, called, "the key was sent to a private address")

	configureGateway(5, true)
	_, _, err = newGatewayClient().Import(context.Background(), gw, "k", gatewayImportRequest{})
	assert.NoError(t, err)
	assert.True(t, called)
}
