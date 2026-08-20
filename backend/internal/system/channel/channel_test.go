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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/importer"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// fakeImportRunner is a test double for the DP import service.
type fakeImportRunner struct {
	lastContent string
	resp        *importer.ImportResponse
	svcErr      *tidcommon.ServiceError
}

func (f *fakeImportRunner) ImportResources(
	_ context.Context, req *importer.ImportRequest,
) (*importer.ImportResponse, *tidcommon.ServiceError) {
	f.lastContent = req.Content
	return f.resp, f.svcErr
}

// wireEndToEnd starts a real Server behind httptest and a real Client dialing it, and waits for the
// connection to register.
func wireEndToEnd(t *testing.T, ctx context.Context, runner *fakeImportRunner) *Server {
	t.Helper()
	cfg := ServerConfig{Enabled: true, Path: "/cp/connect", AuthToken: "tok"}
	mux := http.NewServeMux()
	s := InitializeServer(mux, cfg, nil)
	hs := httptest.NewServer(mux)

	client := InitializeClient(ClientConfig{
		Enabled:          true,
		ID:               "dp-1",
		ControlPlaneURL:  "ws" + strings.TrimPrefix(hs.URL, "http") + "/cp/connect",
		AuthToken:        "tok",
		PingInterval:     time.Second,
		ReconnectInitial: 10 * time.Millisecond,
		ReconnectMax:     100 * time.Millisecond,
	}, runner, nil)
	client.Start(ctx)

	t.Cleanup(func() { client.Stop(); s.Close(); hs.Close() })
	require.Eventually(t, func() bool { return len(s.Connections()) == 1 }, 3*time.Second, 20*time.Millisecond)
	return s
}

func TestCallImportRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runner := &fakeImportRunner{resp: &importer.ImportResponse{
		Summary: &importer.ImportSummary{TotalDocuments: 2, Imported: 2},
	}}
	s := wireEndToEnd(t, ctx, runner)

	resp, err := s.CallImport(ctx, "dp-1", &importer.ImportRequest{Content: "resource_type: application"})
	require.NoError(t, err)
	require.NotNil(t, resp.Summary)
	assert.Equal(t, 2, resp.Summary.Imported)
	assert.Equal(t, "resource_type: application", runner.lastContent)
}

func TestCallImportPropagatesServiceError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runner := &fakeImportRunner{svcErr: &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "IMP-1001",
		Error: tidcommon.I18nMessage{DefaultValue: "invalid import request"},
	}}
	s := wireEndToEnd(t, ctx, runner)

	_, err := s.CallImport(ctx, "dp-1", &importer.ImportRequest{Content: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid import request")
}

func TestPingRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s := wireEndToEnd(t, ctx, &fakeImportRunner{})
	assert.NoError(t, s.Ping(ctx, "dp-1"))
}

// TestServerLastSeenAdvancesOnTransportPing verifies that an idle Data Plane (no JSON-RPC traffic at
// all) still keeps its LastSeen advancing, driven purely by the client's transport-level ping
// heartbeat surfaced through AcceptOptions.OnPingReceived on the Control Plane.
func TestServerLastSeenAdvancesOnTransportPing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := ServerConfig{Enabled: true, Path: "/cp/connect", AuthToken: "tok"}
	mux := http.NewServeMux()
	s := InitializeServer(mux, cfg, nil)
	hs := httptest.NewServer(mux)

	client := InitializeClient(ClientConfig{
		Enabled:          true,
		ID:               "dp-1",
		ControlPlaneURL:  "ws" + strings.TrimPrefix(hs.URL, "http") + "/cp/connect",
		AuthToken:        "tok",
		PingInterval:     20 * time.Millisecond,
		ReconnectInitial: 10 * time.Millisecond,
		ReconnectMax:     100 * time.Millisecond,
	}, &fakeImportRunner{}, nil)
	client.Start(ctx)
	t.Cleanup(func() { client.Stop(); s.Close(); hs.Close() })

	require.Eventually(t, func() bool { return len(s.Connections()) == 1 }, 3*time.Second, 20*time.Millisecond)
	connectedAt := s.Connections()[0].LastSeen

	require.Eventually(t, func() bool {
		conns := s.Connections()
		return len(conns) == 1 && conns[0].LastSeen.After(connectedAt)
	}, 2*time.Second, 20*time.Millisecond, "LastSeen should advance from transport pings alone")
}

func TestCallImportOfflineFailsFast(t *testing.T) {
	s := InitializeServer(http.NewServeMux(), ServerConfig{Enabled: true, Path: "/cp/connect", AuthToken: "tok"}, nil)
	defer s.Close()
	_, err := s.CallImport(context.Background(), "dp-1", &importer.ImportRequest{Content: "x"})
	assert.ErrorIs(t, err, ErrDataPlaneNotConnected)
}

func TestInitializeServerDisabledReturnsNilAndRegistersNoRoute(t *testing.T) {
	mux := http.NewServeMux()
	s := InitializeServer(mux, ServerConfig{Enabled: false, Path: "/cp/connect"}, nil)
	assert.Nil(t, s)

	req := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	_, pattern := mux.Handler(req)
	assert.Empty(t, pattern)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestInitializeClientDisabledReturnsNil(t *testing.T) {
	c := InitializeClient(ClientConfig{Enabled: false}, &fakeImportRunner{}, nil)
	assert.Nil(t, c)
}

func TestInitializeServerNormalizesPathMissingLeadingSlash(t *testing.T) {
	mux := http.NewServeMux()
	s := InitializeServer(mux, ServerConfig{Enabled: true, Path: "cp/connect", AuthToken: "tok"}, nil)
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	_, pattern := mux.Handler(req)
	assert.NotEmpty(t, pattern)
}
