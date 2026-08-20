/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package channel

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientConnectsRespondsAndReconnects(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var accepts atomic.Int32
	gotResult := make(chan string, 1)

	// Fake Control Plane: accept, send one Echo request, read the response, then close to force a
	// reconnect on the first connection.
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		n := accepts.Add(1)
		defer func() { _ = c.CloseNow() }()
		_ = wsjson.Write(ctx, c, Request{JSONRPC: Version, ID: "1", Method: "Echo", Params: json.RawMessage(`"ping"`)})
		var resp Response
		if err := wsjson.Read(ctx, c, &resp); err == nil {
			gotResult <- string(resp.Result)
		}
		if n == 1 {
			_ = c.Close(websocket.StatusNormalClosure, "forcing reconnect")
			return
		}
		<-ctx.Done()
	}))
	defer hs.Close()

	router := NewRouter()
	router.Register("Echo", func(_ context.Context, params json.RawMessage) (json.RawMessage, *Error) {
		return params, nil
	})

	client := NewClient(ClientConfig{
		Enabled:          true,
		ID:               "dp-1",
		ControlPlaneURL:  "ws" + strings.TrimPrefix(hs.URL, "http"),
		AuthToken:        "tok",
		PingInterval:     time.Second,
		ReconnectInitial: 10 * time.Millisecond,
		ReconnectMax:     100 * time.Millisecond,
	}, router)
	client.Start(ctx)
	defer client.Stop()

	assert.Equal(t, `"ping"`, <-gotResult)
	require.Eventually(t, func() bool { return accepts.Load() >= 2 }, 3*time.Second, 20*time.Millisecond,
		"client should reconnect after the CP closes the first connection")
}

func TestShouldResetBackoff(t *testing.T) {
	tests := []struct {
		name      string
		uptime    time.Duration
		threshold time.Duration
		want      bool
	}{
		{name: "shorter than threshold keeps growing", uptime: time.Second, threshold: 30 * time.Second, want: false},
		{name: "exactly at threshold resets", uptime: 30 * time.Second, threshold: 30 * time.Second, want: true},
		{name: "longer than threshold resets", uptime: time.Minute, threshold: 30 * time.Second, want: true},
		{name: "immediate drop never resets", uptime: 0, threshold: 30 * time.Second, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldResetBackoff(tt.uptime, tt.threshold))
		})
	}
}

func TestNextBackoffNeverReturnsZero(t *testing.T) {
	assert.Positive(t, nextBackoff(defaultReconnectInitial, 100*time.Millisecond))
}

func TestNextBackoff(t *testing.T) {
	tests := []struct {
		name    string
		current time.Duration
		max     time.Duration
		want    time.Duration
	}{
		{
			name: "doubles below max", current: 10 * time.Millisecond, max: 100 * time.Millisecond,
			want: 20 * time.Millisecond,
		},
		{
			name: "caps at max", current: 80 * time.Millisecond, max: 100 * time.Millisecond,
			want: 100 * time.Millisecond,
		},
		{name: "no max uncapped", current: time.Second, max: 0, want: 2 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, nextBackoff(tt.current, tt.max))
		})
	}
}

func TestJitterWithinBounds(t *testing.T) {
	d := 50 * time.Millisecond
	for range 100 {
		j := jitter(d)
		assert.GreaterOrEqual(t, j, time.Duration(0))
		assert.LessOrEqual(t, j, d)
	}
	assert.Zero(t, jitter(0))
}

func TestStopWithoutStartDoesNotBlock(t *testing.T) {
	client := NewClient(ClientConfig{}, NewRouter())
	done := make(chan struct{})
	go func() {
		client.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop() blocked when Start() was never called")
	}
}

func TestStartCalledTwiceIsIdempotent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient(ClientConfig{ReconnectInitial: time.Hour}, NewRouter())
	client.Start(ctx)
	client.Start(ctx)
	client.Stop()
}

// A Control Plane serving a certificate no public authority signed is reached by naming that
// certificate, which keeps verification on rather than turning it off.
func TestDialClientTrustsTheNamedCertificate(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "server.cert")
	if err := os.WriteFile(certFile, selfSignedPEM(t), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	c := &Client{cfg: ClientConfig{CAFile: certFile}}

	httpClient, err := c.dialClient()

	if err != nil {
		t.Fatalf("dial client: %v", err)
	}
	if httpClient == nil {
		t.Fatal("expected a client configured with the named certificate")
	}
}

// With nothing named, the default client and the system roots are used.
func TestDialClientWithoutACertificateUsesTheDefault(t *testing.T) {
	c := &Client{cfg: ClientConfig{}}

	httpClient, err := c.dialClient()

	if err != nil {
		t.Fatalf("dial client: %v", err)
	}
	if httpClient != nil {
		t.Fatal("expected the library default to be left in place")
	}
}

// A certificate that cannot be read is a misconfiguration worth failing on, not something to fall
// back from: falling back would silently dial with verification the operator did not ask for.
func TestDialClientFailsOnAnUnreadableCertificate(t *testing.T) {
	c := &Client{cfg: ClientConfig{CAFile: filepath.Join(t.TempDir(), "missing.cert")}}

	if _, err := c.dialClient(); err == nil {
		t.Fatal("expected a missing certificate to fail the dial")
	}
}

func TestDialClientFailsOnAFileHoldingNoCertificate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "not-a-cert")
	if err := os.WriteFile(path, []byte("nonsense"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	c := &Client{cfg: ClientConfig{CAFile: path}}

	if _, err := c.dialClient(); err == nil {
		t.Fatal("expected a file with no certificate to fail the dial")
	}
}

// selfSignedPEM builds a throwaway certificate to stand in for a Control Plane's own.
func selfSignedPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("certificate: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
