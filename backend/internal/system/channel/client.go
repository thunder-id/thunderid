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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	clientLoggerComponent = "ChannelClient"
	pingTimeout           = 10 * time.Second

	// defaultReconnectInitial is used when the configured initial backoff is not positive, so a
	// misconfigured client cannot spin in a hot dial loop.
	defaultReconnectInitial = time.Second

	// writeTimeout bounds writing a single JSON-RPC response back to the Control Plane. Dispatch runs
	// in its own goroutine against a per-connection context that otherwise has no deadline of its
	// own, so without this bound a stalled write would hold the write mutex (see
	// wsConn.writeMessage) indefinitely, wedging every later response on the connection.
	writeTimeout = 10 * time.Second

	// healthyConnectionThreshold is the minimum time a connection must stay up before its end is
	// treated as recovering from a healthy connection and resets the reconnect backoff to its initial
	// value. A connection that dies before this elapses (a Control Plane rolling restart, or two Data
	// Planes racing for the same id) keeps growing the backoff instead of repeatedly pinning the
	// client at the fastest retry.
	healthyConnectionThreshold = 30 * time.Second
)

// instance names this replica. The host name is the pod name under Kubernetes, which is stable across
// this process's lifetime and distinct per replica, so a reconnect replaces this pod's own socket
// rather than a sibling's.
func (c *Client) instance() string {
	if configured := strings.TrimSpace(c.cfg.Instance); configured != "" {
		return configured
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		return host
	}
	return defaultInstance
}

// dialClient builds the HTTP client the handshake is made with. With no CA file configured it is
// nil, which leaves coder/websocket to use its default and the system roots.
//
// The certificate is read on each dial rather than cached, so replacing a rotated one takes effect on
// the next reconnect instead of needing a restart.
func (c *Client) dialClient() (*http.Client, error) {
	if strings.TrimSpace(c.cfg.CAFile) == "" {
		return nil, nil
	}
	pem, err := os.ReadFile(c.cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read the control plane certificate %s: %w", c.cfg.CAFile, err)
	}
	// Starting from the system roots rather than an empty pool, so naming one certificate adds to
	// what is trusted instead of replacing it.
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%s holds no certificate", c.cfg.CAFile)
	}
	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}},
	}, nil
}

// Client is the Data Plane end of the channel: it dials the Control Plane, keeps the connection
// alive with a heartbeat, reconnects with backoff, and serves inbound JSON-RPC requests.
type Client struct {
	cfg      ClientConfig
	router   *Router
	logger   *log.Logger
	mu       sync.Mutex
	started  bool
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
}

// NewClient builds a Data Plane channel client bound to the given router.
func NewClient(cfg ClientConfig, router *Router) *Client {
	return &Client{
		cfg:    cfg,
		router: router,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, clientLoggerComponent)),
		done:   make(chan struct{}),
	}
}

// Start launches the reconnect loop in the background. It returns immediately. Calling Start more
// than once is a no-op after the first call.
func (c *Client) Start(ctx context.Context) {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return
	}
	c.started = true
	ctx, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()
	go c.run(ctx)
}

// Stop cancels the reconnect loop and waits for it to exit. Safe to call multiple times and safe
// to call even if Start was never called.
func (c *Client) Stop() {
	c.stopOnce.Do(func() {
		c.mu.Lock()
		started := c.started
		cancel := c.cancel
		c.mu.Unlock()
		if !started {
			return
		}
		if cancel != nil {
			cancel()
		}
		<-c.done
	})
}

func (c *Client) run(ctx context.Context) {
	defer close(c.done)
	initialBackoff := c.cfg.ReconnectInitial
	if initialBackoff <= 0 {
		initialBackoff = defaultReconnectInitial
	}
	backoff := initialBackoff
	for {
		if ctx.Err() != nil {
			return
		}
		dialed, uptime, err := c.connectAndServe(ctx)
		if ctx.Err() != nil {
			return
		}
		if dialed {
			if shouldResetBackoff(uptime, healthyConnectionThreshold) {
				backoff = initialBackoff
			}
			c.logger.Warn(ctx, "Control plane connection lost; reconnecting", log.Error(err))
		} else {
			c.logger.Warn(ctx, "Control plane dial failed; retrying", log.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(jitter(backoff)):
		}
		backoff = nextBackoff(backoff, c.cfg.ReconnectMax)
	}
}

// shouldResetBackoff reports whether a connection that stayed up for uptime should be treated as
// healthy, resetting the reconnect backoff to its initial value rather than continuing to grow it.
func shouldResetBackoff(uptime, threshold time.Duration) bool {
	return uptime >= threshold
}

// connectAndServe dials once and runs the read loop until the connection ends. The first return
// value reports whether the dial succeeded; the second reports how long the connection stayed up
// after a successful dial, which the caller uses to decide whether to reset the reconnect backoff.
func (c *Client) connectAndServe(ctx context.Context) (bool, time.Duration, error) {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.cfg.AuthToken)
	header.Set(HeaderDataPlaneID, c.cfg.ID)
	header.Set(HeaderDataPlaneInstance, c.instance())

	httpClient, err := c.dialClient()
	if err != nil {
		return false, 0, err
	}
	ws, _, err := websocket.Dial(ctx, c.cfg.ControlPlaneURL, &websocket.DialOptions{
		HTTPHeader: header,
		HTTPClient: httpClient,
	})
	if err != nil {
		return false, 0, err
	}
	conn := newWSConn(ws, c.cfg.ReadLimit)
	defer func() { _ = conn.closeNow() }()
	c.logger.Info(ctx, "Connected to control plane", log.String("url", c.cfg.ControlPlaneURL))
	connectedAt := time.Now()

	// connCtx is scoped to this one connection. Canceling it (deferred below) both stops the
	// heartbeat and signals any in-flight dispatch/response-write goroutines that the connection is
	// gone, instead of leaving them running against the long-lived run context after the socket has
	// already died.
	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()
	go c.heartbeat(connCtx, conn)

	for {
		var req Request
		if err := conn.readMessage(connCtx, &req); err != nil {
			if ctx.Err() != nil {
				// Deliberate shutdown (Stop was called): send a normal closure frame so the Control
				// Plane records a clean disconnect instead of an abnormal 1006 closure. The deferred
				// closeNow above remains the safety net if this graceful close itself stalls or fails.
				_ = conn.close(websocket.StatusNormalClosure, "data plane shutting down")
			}
			return true, time.Since(connectedAt), err
		}
		// Dispatch and write the response using the per-connection context rather than the run
		// context, so a dropped connection cancels in-flight work instead of it running indefinitely
		// against the local database. Because the Control Plane has already failed the caller by the
		// time the connection drops, an import already applied on the Data Plane may still be
		// reported as failed; at-most-once delivery of the RPC result is not guaranteed.
		go func(req Request) {
			resp := c.router.Dispatch(connCtx, req)
			wctx, cancel := context.WithTimeout(connCtx, writeTimeout)
			defer cancel()
			if err := conn.writeMessage(wctx, resp); err != nil {
				c.logger.Warn(connCtx, "Failed to write RPC response", log.String("id", req.ID), log.Error(err))
			}
		}(req)
	}
}

// heartbeat sends a transport ping on an interval. A failed ping closes the connection, which ends
// the read loop and triggers a reconnect. Ping runs concurrently with the read loop, which is
// required by coder/websocket (Ping is satisfied by a concurrent Reader).
func (c *Client) heartbeat(ctx context.Context, conn *wsConn) {
	if c.cfg.PingInterval <= 0 {
		return
	}
	ticker := time.NewTicker(c.cfg.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pctx, cancel := context.WithTimeout(ctx, pingTimeout)
			err := conn.ping(pctx)
			cancel()
			if err != nil {
				c.logger.Warn(ctx, "Heartbeat ping failed; closing connection", log.Error(err))
				_ = conn.closeNow()
				return
			}
		}
	}
}

// jitter applies full jitter to a backoff duration.
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	//nolint:gosec // Reconnect backoff jitter, not security sensitive
	return time.Duration(rand.Int64N(int64(d) + 1))
}

// nextBackoff doubles the backoff up to the configured maximum.
func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if max > 0 && next > max {
		return max
	}
	return next
}
