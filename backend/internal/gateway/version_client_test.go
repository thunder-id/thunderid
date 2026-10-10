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
		assert.Equal(t, "/import", r.URL.Path)
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"summary":{"failed":0}}`))
	})

	answer, err := newGatewayClient().Import(context.Background(), gw, "the-key",
		gatewayImportRequest{Content: "docs", DryRun: true})

	require.NoError(t, err)
	assert.JSONEq(t, `{"summary":{"failed":0}}`, string(answer))
	assert.Equal(t, "the-key", presented)
	assert.Equal(t, "docs", got.Content)
	assert.True(t, got.DryRun)
}

// A gateway answering with anything but success is reported as refusing.
func TestTheClientReportsARefusal(t *testing.T) {
	gw := gatewayFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := newGatewayClient().Import(context.Background(), gw, "wrong", gatewayImportRequest{})

	assert.True(t, errors.Is(err, errGatewayRefused))
	assert.True(t, errors.Is(err, errImportNotWritten), "a 4xx is answered before anything is written")
}

// Only a failure that shows the gateway received none of the import says so: a server error or a
// connection lost after the request went out leaves open whether the gateway wrote it.
func TestTheClientSaysWhenTheGatewayWroteNothing(t *testing.T) {
	failing := gatewayFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := newGatewayClient().Import(context.Background(), failing, "k", gatewayImportRequest{})
	assert.True(t, errors.Is(err, errGatewayRefused))
	assert.False(t, errors.Is(err, errImportNotWritten), "a server error may follow a partial write")

	hangingUp := gatewayFor(t, func(w http.ResponseWriter, _ *http.Request) {
		conn, _, hijackErr := w.(http.Hijacker).Hijack()
		require.NoError(t, hijackErr)
		_ = conn.Close()
	})
	_, err = newGatewayClient().Import(context.Background(), hangingUp, "k", gatewayImportRequest{})
	assert.Error(t, err)
	assert.False(t, errors.Is(err, errImportNotWritten), "a connection lost after sending may have written")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	closedAddress := listener.Addr().String()
	require.NoError(t, listener.Close())
	_, err = newGatewayClient().Import(context.Background(), &Gateway{ID: "gw-1", BaseURL: "https://" + closedAddress},
		"k", gatewayImportRequest{})
	assert.True(t, errors.Is(err, errImportNotWritten), "a refused connection sent nothing")

	untrusted := *failing
	untrusted.CACertificate = ""
	_, err = newGatewayClient().Import(context.Background(), &untrusted, "k", gatewayImportRequest{})
	assert.True(t, errors.Is(err, errImportNotWritten), "a certificate that does not verify sent nothing")

	_, err = newGatewayClient().Import(context.Background(), &Gateway{ID: "gw-1", BaseURL: "http://dp.test"}, "k",
		gatewayImportRequest{})
	assert.True(t, errors.Is(err, errImportNotWritten), "a plain HTTP gateway is never called")
}

// Verification stays on: a gateway whose certificate the registration does not trust is refused,
// and so is a certificate authority that is not a certificate.
func TestTheClientVerifiesTheGateway(t *testing.T) {
	gw := gatewayFor(t, func(http.ResponseWriter, *http.Request) {})

	untrusted := *gw
	untrusted.CACertificate = ""
	_, err := newGatewayClient().Import(context.Background(), &untrusted, "k", gatewayImportRequest{})
	assert.Error(t, err)

	broken := *gw
	broken.CACertificate = "not a certificate"
	_, err = newGatewayClient().Import(context.Background(), &broken, "k", gatewayImportRequest{})
	assert.Error(t, err)
}

// The key is never sent over plain HTTP: a gateway registered at an http address is refused.
func TestTheClientRefusesAPlainHTTPGateway(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	t.Cleanup(server.Close)

	_, err := newGatewayClient().Import(context.Background(), &Gateway{ID: "gw-1", BaseURL: server.URL}, "k",
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
		http.Redirect(w, r, target.URL+"/import", http.StatusTemporaryRedirect)
	})

	_, err := newGatewayClient().ListVariables(context.Background(), gw, "k", StoreListQuery{})

	var refusal *StoreRefusal
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, http.StatusTemporaryRedirect, refusal.Status)
	assert.False(t, followed, "the redirect was followed")
}
