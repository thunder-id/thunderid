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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startTestServer mounts a channel Server on an httptest server and returns it plus the ws:// URL.
func startTestServer(t *testing.T, cfg ServerConfig) (*Server, string) {
	t.Helper()
	if cfg.Path == "" {
		cfg.Path = "/cp/connect"
	}
	s := NewServer(cfg, newTokenVerifier(cfg.AuthToken))
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+cfg.Path, s.HandleConnect)
	hs := httptest.NewServer(mux)
	t.Cleanup(func() { s.Close(); hs.Close() })
	return s, "ws" + strings.TrimPrefix(hs.URL, "http") + cfg.Path
}

// dialAsDataPlane dials the server as a fake Data Plane and runs a loop that echoes every request's
// params back as the result. It returns once the connection closes.
func dialAsDataPlane(t *testing.T, ctx context.Context, url, token, dpID string) {
	t.Helper()
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	header.Set(HeaderDataPlaneID, dpID)
	c, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: header})
	require.NoError(t, err)
	go func() {
		defer func() { _ = c.CloseNow() }()
		for {
			var req Request
			if err := wsjson.Read(ctx, c, &req); err != nil {
				return
			}
			_ = wsjson.Write(ctx, c, Response{JSONRPC: Version, ID: req.ID, Result: req.Params})
		}
	}()
}

func TestServerCallMethodRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s, url := startTestServer(t, ServerConfig{Enabled: true, AuthToken: "tok"})
	dialAsDataPlane(t, ctx, url, "tok", "dp-1")

	// Wait for registration.
	require.Eventually(t, func() bool { return len(s.Connections()) == 1 }, 2*time.Second, 10*time.Millisecond)

	result, err := s.CallMethod(ctx, "dp-1", "Echo", map[string]string{"hello": "world"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"hello":"world"}`, string(result))
}

func TestServerCallMethodUnknownDataPlane(t *testing.T) {
	s, _ := startTestServer(t, ServerConfig{Enabled: true, AuthToken: "tok"})
	_, err := s.CallMethod(context.Background(), "missing", "Echo", nil)
	assert.ErrorIs(t, err, ErrDataPlaneNotConnected)
}

func TestServerRejectsBadToken(t *testing.T) {
	_, url := startTestServer(t, ServerConfig{Enabled: true, AuthToken: "tok"})
	header := http.Header{}
	header.Set("Authorization", "Bearer wrong")
	header.Set(HeaderDataPlaneID, "dp-1")
	_, resp, err := websocket.Dial(context.Background(), url, &websocket.DialOptions{HTTPHeader: header})
	assert.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
