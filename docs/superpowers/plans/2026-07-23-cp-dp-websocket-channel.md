# CP-DP Bi-directional WebSocket Channel (v1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a phone-home WebSocket channel so the Control Plane (`cmd/cpserver`) can push reverse JSON-RPC commands to a Data Plane (`cmd/dpserver`) over a persistent, DP-initiated connection, with the first command being `Import.Run`.

**Architecture:** A new `internal/system/channel` package holds a JSON-RPC 2.0 codec, a method router, a shared-token handshake verifier, a CP-side connection registry, a `coder/websocket` connection wrapper, a CP `Server` (accepts connections, calls methods, awaits tagged responses) and a DP `Client` (dials, authenticates, reconnects with backoff, heartbeats, serves inbound RPCs). The CP `Server` is wired into `cmd/cpserver`; the DP `Client` into `cmd/dpserver`. Role is chosen by the binary; the channel starts only when its config section is enabled.

**Tech Stack:** Go 1.26, `github.com/coder/websocket` v1.8.15 (+ `wsjson`), testify (suite + assert), the repo's `internal/system/log`, `internal/system/utils`, and `pkg/thunderidengine/common` (`ServiceError`).

## Global Constraints

- Product name: always `ThunderID`; never bare `thunder`/`Thunder`/`THUNDER` except in import paths (verbatim from AGENTS.md).
- No em dashes or double hyphens in copy/UI strings (verbatim from AGENTS.md).
- Do not add dependencies or modify Makefiles/CI/build without explicit approval. `coder/websocket` was explicitly approved by the user in brainstorming; no other new dependency is permitted. Do NOT modify `build.sh`, `Makefile`, or CI.
- Every new `.go` file begins with the Apache 2.0 header, `Copyright (c) 2026, WSO2 LLC.`, using the two-space form after `implied.` (see Task 2 for the verbatim block).
- Tests target 80%+ coverage for new code (verbatim from AGENTS.md, root line 49).
- New backend packages follow the `Initialize(...)` + unexported `newX` + `XServiceInterface`/`XInterface` convention; long-lived components expose `Start(ctx)`/`Stop()` or `Close()` and their lifecycle is owned by `cmd/*server` main.
- One commit per PR: commit per task during execution (frequent commits), then squash to a single commit before opening the PR. Commit messages are short imperative sentences with no conventional-commit prefix (no `feat:`/`fix:`), referencing `Refs #4247` where relevant.
- Every `git commit` in this plan (each per-task commit AND the final squash) MUST end with a blank line followed by this trailer, even where the per-task command blocks below omit it for brevity:
  ```
  Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
  ```
- Toolchain note: `backend/go.mod` requires `go 1.26` while some environments have `go1.25.1` with `GOTOOLCHAIN=auto`; `go get`/`go mod tidy` may auto-download the 1.26 toolchain (needs network). Ensure this works before Task 1.
- Inner-loop test command (single package): `cd backend && go test -race -count=1 ./internal/system/channel/...`. Authoritative gate: `make pr_checks`.
- `internal/system/*` packages must never import `cmd/*`. The channel receives its dependencies (config, `ImportRunner`) by injection.

---

## File Structure

**New package `backend/internal/system/channel/` (flat, single package):**

| File | Responsibility |
|---|---|
| `jsonrpc.go` | JSON-RPC 2.0 `Request`/`Response`/`Error` types, version + error-code constants, `Error.Error()`. No ws dependency. |
| `jsonrpc_test.go` | Codec + `Error` unit tests. |
| `router.go` | `HandlerFunc`, `Router` (register/dispatch, MethodNotFound). No ws dependency. |
| `router_test.go` | Router unit tests. |
| `auth.go` | `Verifier` interface + `tokenVerifier` (constant-time bearer check; mTLS seam). |
| `auth_test.go` | Verifier unit tests. |
| `errors.go` | Package errors: `ErrDataPlaneNotConnected` (exported), `errUnauthorized`, `errAuthNotConfigured`, `errMissingDataPlaneID`. |
| `registry.go` | Generic CP-side `Registry[T ConnEntry]`: register (evict duplicate), unregister-by-identity, get, list. No ws dependency. |
| `registry_test.go` | Registry unit tests with a fake `ConnEntry`. |
| `config.go` | Package-local `ServerConfig`/`ClientConfig` (decoupled from `system/config`). |
| `conn.go` | `wsConn`: `coder/websocket` wrapper with a write mutex (one-writer rule), read/ping/close. |
| `server.go` | CP `Server`: `serverConn`, `HandleConnect`, `CallMethod`, `Close`. |
| `server_test.go` | CP server integration test (fake DP client). |
| `client.go` | DP `Client`: `Start`/`Stop`, dial + auth, reconnect + backoff, read loop, heartbeat. |
| `client_test.go` | DP client integration test (fake CP server). |
| `methods.go` | Method-name constants, `ImportRunner` interface, `RegisterDataPlaneMethods`, `Server.CallImport`, `Server.Ping`, `serviceErrorToRPC`. |
| `init.go` | `InitializeServer(mux, cfg)` and `InitializeClient(cfg, importRunner)`. |
| `channel_test.go` | End-to-end integration: real `Server` + real `Client`, `CallImport`/`Ping`. |

**Modified files:**

| File | Change |
|---|---|
| `backend/go.mod`, `backend/go.sum` | Add `github.com/coder/websocket` (Task 1). |
| `backend/internal/system/security/permissions.go` | Add `"/cp/connect"` to `publicPaths` (Task 10). |
| `backend/cmd/cpserver/main.go` | Add `"/cp/connect"` to `accessLogExcludePaths` (Task 10). |
| `backend/internal/system/config/config.go` | Add `Channel ChannelConfig` section + structs (Task 11). |
| `backend/cmd/server/config/default.json` | Add disabled `channel` defaults (Task 11). |
| `backend/cmd/cpserver/servicemanager.go` | Package var + `channel.InitializeServer` + `Close` in `unregisterServices` (Task 12). |
| `backend/cmd/dpserver/main.go` | Build client, `Start(ctx)`, `Stop()` in `gracefulShutdown` (Task 13). |

---

## Task 1: Add the coder/websocket dependency

**Files:**
- Modify: `backend/go.mod`, `backend/go.sum`

- [ ] **Step 1: Add the dependency**

Run:
```bash
cd backend && go get github.com/coder/websocket@v1.8.15 && go mod tidy
```
Expected: `go.mod` gains `github.com/coder/websocket v1.8.15`; `go.sum` gains its checksums. The library is zero-dependency, so no other requires appear.

- [ ] **Step 2: Verify it builds**

Run:
```bash
cd backend && go build ./... 
```
Expected: success (no compile errors). This confirms the toolchain resolved and the module is usable.

- [ ] **Step 3: Commit**

```bash
git add backend/go.mod backend/go.sum
git commit -m "Add coder/websocket dependency for the CP-DP channel

Refs #4247"
```

---

## Task 2: JSON-RPC 2.0 envelope and codec

**Files:**
- Create: `backend/internal/system/channel/jsonrpc.go`
- Test: `backend/internal/system/channel/jsonrpc_test.go`

**Interfaces:**
- Produces: `Version` const (`"2.0"`); error-code consts `CodeParseError=-32700`, `CodeInvalidRequest=-32600`, `CodeMethodNotFound=-32601`, `CodeInvalidParams=-32602`, `CodeInternalError=-32603`; types `Request{JSONRPC,ID,Method string; Params json.RawMessage}`, `Response{JSONRPC,ID string; Result json.RawMessage; Error *Error}`, `Error{Code int; Message string; Data json.RawMessage}` implementing `error`; constructor `NewError(code int, message string) *Error`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/system/channel/jsonrpc_test.go`:
```go
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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestRoundTrips(t *testing.T) {
	req := Request{JSONRPC: Version, ID: "abc", Method: "Import.Run", Params: json.RawMessage(`{"content":"x"}`)}
	raw, err := json.Marshal(req)
	assert.NoError(t, err)

	var got Request
	assert.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "2.0", got.JSONRPC)
	assert.Equal(t, "abc", got.ID)
	assert.Equal(t, "Import.Run", got.Method)
	assert.JSONEq(t, `{"content":"x"}`, string(got.Params))
}

func TestResponseOmitsEmptyFields(t *testing.T) {
	raw, err := json.Marshal(Response{JSONRPC: Version, ID: "1", Result: json.RawMessage(`{"ok":true}`)})
	assert.NoError(t, err)
	assert.NotContains(t, string(raw), "error")

	raw, err = json.Marshal(Response{JSONRPC: Version, ID: "1", Error: NewError(CodeMethodNotFound, "nope")})
	assert.NoError(t, err)
	assert.NotContains(t, string(raw), "result")
}

func TestErrorImplementsError(t *testing.T) {
	var err error = NewError(CodeInvalidParams, "bad params")
	assert.Contains(t, err.Error(), "-32602")
	assert.Contains(t, err.Error(), "bad params")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/system/channel/ -run TestRequestRoundTrips`
Expected: FAIL (package/types not defined).

- [ ] **Step 3: Write the implementation**

Create `backend/internal/system/channel/jsonrpc.go`:
```go
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

// Package channel implements the phone-home WebSocket channel between the Control Plane
// (WebSocket server) and Data Plane (WebSocket client), carrying reverse JSON-RPC 2.0 commands.
package channel

import (
	"encoding/json"
	"fmt"
)

// Version is the JSON-RPC protocol version string used on every frame.
const Version = "2.0"

// JSON-RPC 2.0 standard error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Request is a JSON-RPC 2.0 request frame sent by the Control Plane to a Data Plane.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response frame returned by a Data Plane. Exactly one of Result or
// Error is set.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is a JSON-RPC 2.0 error object. It implements the error interface so it can be returned
// directly from CallMethod.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// NewError builds a JSON-RPC error with the given code and message.
func NewError(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Error renders the JSON-RPC error as a string.
func (e *Error) Error() string {
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/system/channel/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/system/channel/jsonrpc.go backend/internal/system/channel/jsonrpc_test.go
git commit -m "Add JSON-RPC 2.0 envelope for the CP-DP channel

Refs #4247"
```

---

## Task 3: RPC method router

**Files:**
- Create: `backend/internal/system/channel/router.go`
- Test: `backend/internal/system/channel/router_test.go`

**Interfaces:**
- Consumes: `Request`, `Response`, `Error`, `NewError`, `CodeMethodNotFound`, `Version` (Task 2).
- Produces: `HandlerFunc func(ctx context.Context, params json.RawMessage) (json.RawMessage, *Error)`; `Router` with `NewRouter() *Router`, `(*Router).Register(method string, h HandlerFunc)`, `(*Router).Dispatch(ctx context.Context, req Request) Response`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/system/channel/router_test.go` (include the Apache header from Task 2):
```go
package channel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRouterDispatchesRegisteredMethod(t *testing.T) {
	r := NewRouter()
	r.Register("Echo", func(_ context.Context, params json.RawMessage) (json.RawMessage, *Error) {
		return params, nil
	})

	resp := r.Dispatch(context.Background(), Request{JSONRPC: Version, ID: "1", Method: "Echo", Params: json.RawMessage(`"hi"`)})
	assert.Equal(t, "1", resp.ID)
	assert.Nil(t, resp.Error)
	assert.JSONEq(t, `"hi"`, string(resp.Result))
}

func TestRouterUnknownMethodReturnsMethodNotFound(t *testing.T) {
	resp := NewRouter().Dispatch(context.Background(), Request{JSONRPC: Version, ID: "2", Method: "Nope"})
	assert.NotNil(t, resp.Error)
	assert.Equal(t, CodeMethodNotFound, resp.Error.Code)
	assert.Equal(t, "2", resp.ID)
}

func TestRouterPropagatesHandlerError(t *testing.T) {
	r := NewRouter()
	r.Register("Fail", func(_ context.Context, _ json.RawMessage) (json.RawMessage, *Error) {
		return nil, NewError(CodeInvalidParams, "bad")
	})
	resp := r.Dispatch(context.Background(), Request{JSONRPC: Version, ID: "3", Method: "Fail"})
	assert.Equal(t, CodeInvalidParams, resp.Error.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/system/channel/ -run TestRouter`
Expected: FAIL (Router undefined).

- [ ] **Step 3: Write the implementation**

Create `backend/internal/system/channel/router.go` (include the Apache header):
```go
package channel

import (
	"context"
	"encoding/json"
	"sync"
)

// HandlerFunc handles one JSON-RPC method. params is the raw request params; the return values are
// the raw result or a JSON-RPC error (exactly one is non-nil).
type HandlerFunc func(ctx context.Context, params json.RawMessage) (json.RawMessage, *Error)

// Router maps JSON-RPC method names to handlers on the Data Plane.
type Router struct {
	mu       sync.RWMutex
	handlers map[string]HandlerFunc
}

// NewRouter creates an empty router.
func NewRouter() *Router {
	return &Router{handlers: make(map[string]HandlerFunc)}
}

// Register binds a handler to a method name. A later registration for the same method replaces the
// earlier one.
func (r *Router) Register(method string, h HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[method] = h
}

// Dispatch routes req to its handler and builds the response frame. Unknown methods return a
// MethodNotFound error response.
func (r *Router) Dispatch(ctx context.Context, req Request) Response {
	r.mu.RLock()
	h, ok := r.handlers[req.Method]
	r.mu.RUnlock()
	if !ok {
		return Response{JSONRPC: Version, ID: req.ID, Error: NewError(CodeMethodNotFound, "method not found: "+req.Method)}
	}
	result, rpcErr := h(ctx, req.Params)
	if rpcErr != nil {
		return Response{JSONRPC: Version, ID: req.ID, Error: rpcErr}
	}
	return Response{JSONRPC: Version, ID: req.ID, Result: result}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/system/channel/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/system/channel/router.go backend/internal/system/channel/router_test.go
git commit -m "Add JSON-RPC method router for the CP-DP channel

Refs #4247"
```

---

## Task 4: Handshake auth verifier and package errors

**Files:**
- Create: `backend/internal/system/channel/errors.go`
- Create: `backend/internal/system/channel/auth.go`
- Test: `backend/internal/system/channel/auth_test.go`

**Interfaces:**
- Produces: exported `ErrDataPlaneNotConnected error`; unexported `errUnauthorized`, `errAuthNotConfigured`, `errMissingDataPlaneID`; `Verifier interface { Verify(r *http.Request) error }`; `newTokenVerifier(token string) *tokenVerifier`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/system/channel/auth_test.go` (include the Apache header):
```go
package channel

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenVerifierAcceptsMatchingBearer(t *testing.T) {
	v := newTokenVerifier("s3cret")
	r := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	r.Header.Set("Authorization", "Bearer s3cret")
	assert.NoError(t, v.Verify(r))
}

func TestTokenVerifierRejectsWrongOrMissingBearer(t *testing.T) {
	v := newTokenVerifier("s3cret")

	r := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	r.Header.Set("Authorization", "Bearer nope")
	assert.ErrorIs(t, v.Verify(r), errUnauthorized)

	r2 := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	assert.ErrorIs(t, v.Verify(r2), errUnauthorized)
}

func TestTokenVerifierRejectsWhenNotConfigured(t *testing.T) {
	v := newTokenVerifier("")
	r := httptest.NewRequest(http.MethodGet, "/cp/connect", nil)
	r.Header.Set("Authorization", "Bearer anything")
	assert.ErrorIs(t, v.Verify(r), errAuthNotConfigured)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/system/channel/ -run TestTokenVerifier`
Expected: FAIL (newTokenVerifier undefined).

- [ ] **Step 3: Write the implementations**

Create `backend/internal/system/channel/errors.go` (include the Apache header):
```go
package channel

import "errors"

// ErrDataPlaneNotConnected is returned by CallMethod when no active connection exists for the
// target Data Plane id.
var ErrDataPlaneNotConnected = errors.New("data plane not connected")

// errUnauthorized is returned by the handshake verifier when the bearer token is absent or wrong.
var errUnauthorized = errors.New("unauthorized channel handshake")

// errAuthNotConfigured is returned by the handshake verifier when no shared token is configured, so
// the server refuses all connections (secure by default).
var errAuthNotConfigured = errors.New("channel auth token not configured")

// errMissingDataPlaneID is returned when the handshake request omits the Data Plane id header.
var errMissingDataPlaneID = errors.New("missing data plane id header")
```

Create `backend/internal/system/channel/auth.go` (include the Apache header):
```go
package channel

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// HeaderDataPlaneID is the request header the Data Plane sends its id in during the handshake.
const HeaderDataPlaneID = "X-Data-Plane-ID"

// Verifier authenticates an inbound Data Plane handshake request. Implementations must not mutate r.
// A token implementation is provided; an mTLS implementation can be added later without changing the
// channel server.
type Verifier interface {
	Verify(r *http.Request) error
}

// tokenVerifier checks a shared bearer token presented in the Authorization header.
type tokenVerifier struct {
	token string
}

// newTokenVerifier builds a Verifier that compares against the configured shared token.
func newTokenVerifier(token string) *tokenVerifier {
	return &tokenVerifier{token: token}
}

// Verify returns nil when the request carries the configured bearer token, errAuthNotConfigured when
// no token is configured, and errUnauthorized otherwise. The comparison is constant-time.
func (v *tokenVerifier) Verify(r *http.Request) error {
	if v.token == "" {
		return errAuthNotConfigured
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(v.token)) != 1 {
		return errUnauthorized
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/system/channel/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/system/channel/errors.go backend/internal/system/channel/auth.go backend/internal/system/channel/auth_test.go
git commit -m "Add shared-token handshake verifier for the CP-DP channel

Refs #4247"
```

---

## Task 5: CP connection registry

**Files:**
- Create: `backend/internal/system/channel/registry.go`
- Test: `backend/internal/system/channel/registry_test.go`

**Interfaces:**
- Produces: `ConnEntry interface { ID() string; LastSeen() time.Time; Close(reason string) }`; `ConnInfo struct { ID string; LastSeen time.Time }`; generic `Registry[T ConnEntry]` with `NewRegistry[T ConnEntry]() *Registry[T]`, `Register(c T)` (evicts an existing entry with the same id by calling its `Close`), `Unregister(id string, c T)` (removes only if the current entry is c), `Get(id string) (T, bool)`, `List() []ConnInfo`, `entries() []T`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/system/channel/registry_test.go` (include the Apache header):
```go
package channel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeConn struct {
	id       string
	seen     time.Time
	closed   bool
	closeMsg string
}

func (f *fakeConn) ID() string          { return f.id }
func (f *fakeConn) LastSeen() time.Time { return f.seen }
func (f *fakeConn) Close(reason string) { f.closed = true; f.closeMsg = reason }

func TestRegistryRegisterGetUnregister(t *testing.T) {
	r := NewRegistry[*fakeConn]()
	c := &fakeConn{id: "dp-1"}
	r.Register(c)

	got, ok := r.Get("dp-1")
	assert.True(t, ok)
	assert.Same(t, c, got)

	r.Unregister("dp-1", c)
	_, ok = r.Get("dp-1")
	assert.False(t, ok)
}

func TestRegistryEvictsDuplicateID(t *testing.T) {
	r := NewRegistry[*fakeConn]()
	old := &fakeConn{id: "dp-1"}
	r.Register(old)
	fresh := &fakeConn{id: "dp-1"}
	r.Register(fresh)

	assert.True(t, old.closed, "old connection should be closed on duplicate register")
	got, _ := r.Get("dp-1")
	assert.Same(t, fresh, got)
}

func TestRegistryUnregisterOnlyRemovesMatchingEntry(t *testing.T) {
	r := NewRegistry[*fakeConn]()
	fresh := &fakeConn{id: "dp-1"}
	r.Register(fresh)
	stale := &fakeConn{id: "dp-1"}

	r.Unregister("dp-1", stale) // stale is not the current entry; must be a no-op
	got, ok := r.Get("dp-1")
	assert.True(t, ok)
	assert.Same(t, fresh, got)
}

func TestRegistryListSnapshots(t *testing.T) {
	r := NewRegistry[*fakeConn]()
	r.Register(&fakeConn{id: "dp-1", seen: time.Unix(10, 0)})
	r.Register(&fakeConn{id: "dp-2", seen: time.Unix(20, 0)})
	assert.Len(t, r.List(), 2)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/system/channel/ -run TestRegistry`
Expected: FAIL (Registry undefined).

- [ ] **Step 3: Write the implementation**

Create `backend/internal/system/channel/registry.go` (include the Apache header):
```go
package channel

import (
	"sync"
	"time"
)

// ConnEntry is a registered Data Plane connection tracked by the registry.
type ConnEntry interface {
	ID() string
	LastSeen() time.Time
	Close(reason string)
}

// ConnInfo is a point-in-time snapshot of a registered connection, used for observability.
type ConnInfo struct {
	ID       string
	LastSeen time.Time
}

// Registry tracks active Data Plane connections on the Control Plane, keyed by Data Plane id, with a
// single-active-socket-per-id policy.
type Registry[T ConnEntry] struct {
	mu    sync.RWMutex
	conns map[string]T
}

// NewRegistry creates an empty registry.
func NewRegistry[T ConnEntry]() *Registry[T] {
	return &Registry[T]{conns: make(map[string]T)}
}

// Register stores c under its id, evicting and closing any existing connection for that id.
func (r *Registry[T]) Register(c T) {
	r.mu.Lock()
	old, existed := r.conns[c.ID()]
	r.conns[c.ID()] = c
	r.mu.Unlock()
	if existed {
		old.Close("superseded by a new data plane connection")
	}
}

// Unregister removes the entry for id only if it is still c, avoiding a race where a newer
// connection replaced c between its read loop ending and this call.
func (r *Registry[T]) Unregister(id string, c T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cur, ok := r.conns[id]; ok && any(cur) == any(c) {
		delete(r.conns, id)
	}
}

// Get returns the active connection for id, if any.
func (r *Registry[T]) Get(id string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.conns[id]
	return c, ok
}

// List returns a snapshot of all active connections.
func (r *Registry[T]) List() []ConnInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ConnInfo, 0, len(r.conns))
	for _, c := range r.conns {
		out = append(out, ConnInfo{ID: c.ID(), LastSeen: c.LastSeen()})
	}
	return out
}

// entries returns a snapshot of all active connections for internal fan-out (for example, closing
// every connection during shutdown).
func (r *Registry[T]) entries() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]T, 0, len(r.conns))
	for _, c := range r.conns {
		out = append(out, c)
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/system/channel/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/system/channel/registry.go backend/internal/system/channel/registry_test.go
git commit -m "Add CP connection registry for the CP-DP channel

Refs #4247"
```

---

## Task 6: Package-local config types

**Files:**
- Create: `backend/internal/system/channel/config.go`

**Interfaces:**
- Produces: `ServerConfig{Enabled bool; Path string; AuthToken string; ReadLimit int64}`; `ClientConfig{Enabled bool; ID string; ControlPlaneURL string; AuthToken string; ReadLimit int64; PingInterval time.Duration; RPCTimeout time.Duration; ReconnectInitial time.Duration; ReconnectMax time.Duration}`.

There is no separate test; these are plain data structs consumed by later tasks (compilation is verified by `go build`). This is folded here because `server.go` and `client.go` reference them.

- [ ] **Step 1: Write the implementation**

Create `backend/internal/system/channel/config.go` (include the Apache header):
```go
package channel

import "time"

// ServerConfig configures the Control Plane channel server. It is decoupled from system/config; the
// cmd wiring maps the deployment config into it.
type ServerConfig struct {
	// Enabled turns the channel server on. When false, InitializeServer registers nothing.
	Enabled bool
	// Path is the route the WebSocket server is mounted on (for example "/cp/connect").
	Path string
	// AuthToken is the shared bearer token a Data Plane must present during the handshake.
	AuthToken string
	// ReadLimit is the max single-message size in bytes. Zero uses the coder/websocket default.
	ReadLimit int64
	// RPCTimeout bounds a single CallMethod round-trip when the caller context has no deadline.
	// Zero means no fallback timeout (the caller context governs).
	RPCTimeout time.Duration
}

// ClientConfig configures the Data Plane channel client.
type ClientConfig struct {
	// Enabled turns the channel client on. When false, InitializeClient returns nil.
	Enabled bool
	// ID is this Data Plane's identifier, sent in the handshake and used as the CP registry key.
	ID string
	// ControlPlaneURL is the wss:// (or ws://) endpoint of the Control Plane channel server.
	ControlPlaneURL string
	// AuthToken is the shared bearer token presented during the handshake.
	AuthToken string
	// ReadLimit is the max single-message size in bytes. Zero uses the coder/websocket default.
	ReadLimit int64
	// PingInterval is how often the client sends a transport ping to keep the connection alive.
	PingInterval time.Duration
	// ReconnectInitial and ReconnectMax bound the exponential-with-jitter reconnect backoff.
	ReconnectInitial time.Duration
	ReconnectMax     time.Duration
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./internal/system/channel/`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/system/channel/config.go
git commit -m "Add channel config types for the CP-DP channel

Refs #4247"
```

---

## Task 7: Connection wrapper and CP server

**Files:**
- Create: `backend/internal/system/channel/conn.go`
- Create: `backend/internal/system/channel/server.go`
- Test: `backend/internal/system/channel/server_test.go`

**Interfaces:**
- Consumes: `Request`, `Response`, `Error`, `Version`, `CodeInternalError`, `Registry`, `ConnEntry`, `Verifier`, `newTokenVerifier`, `ServerConfig`, `ErrDataPlaneNotConnected`, `HeaderDataPlaneID`, `errUnauthorized`, `errMissingDataPlaneID` (Tasks 2-6); `utils.GenerateUUID()` from `github.com/thunder-id/thunderid/internal/system/utils`; `log` from `github.com/thunder-id/thunderid/internal/system/log`; `github.com/coder/websocket` + `.../wsjson`.
- Produces: `wsConn` (with `writeMessage`, `readMessage`, `ping`, `close`, `closeNow`); `Server` with `NewServer(cfg ServerConfig, verifier Verifier) *Server`, `(*Server).HandleConnect(w http.ResponseWriter, r *http.Request)`, `(*Server).CallMethod(ctx context.Context, dpID, method string, params any) (json.RawMessage, error)`, `(*Server).Connections() []ConnInfo`, `(*Server).Close()`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/system/channel/server_test.go` (include the Apache header):
```go
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
		defer c.CloseNow()
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/system/channel/ -run TestServer`
Expected: FAIL (NewServer undefined).

- [ ] **Step 3: Write conn.go**

Create `backend/internal/system/channel/conn.go` (include the Apache header):
```go
package channel

import (
	"context"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// wsConn wraps a coder/websocket connection. coder/websocket permits only one concurrent writer, so
// writeMessage serializes all writes behind a mutex; reads happen only from a single read loop.
type wsConn struct {
	ws      *websocket.Conn
	writeMu sync.Mutex
}

func newWSConn(ws *websocket.Conn, readLimit int64) *wsConn {
	if readLimit > 0 {
		ws.SetReadLimit(readLimit)
	}
	return &wsConn{ws: ws}
}

func (c *wsConn) writeMessage(ctx context.Context, v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return wsjson.Write(ctx, c.ws, v)
}

func (c *wsConn) readMessage(ctx context.Context, v any) error {
	return wsjson.Read(ctx, c.ws, v)
}

func (c *wsConn) ping(ctx context.Context) error {
	return c.ws.Ping(ctx)
}

func (c *wsConn) close(code websocket.StatusCode, reason string) error {
	return c.ws.Close(code, reason)
}

func (c *wsConn) closeNow() error {
	return c.ws.CloseNow()
}
```

- [ ] **Step 4: Write server.go**

Create `backend/internal/system/channel/server.go` (include the Apache header):
```go
package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const serverLoggerComponent = "ChannelServer"

// serverConn is one accepted Data Plane connection on the Control Plane. It tracks in-flight
// CallMethod requests by id so responses can be matched to their waiters.
type serverConn struct {
	*wsConn
	dpID         string
	lastSeenNano atomic.Int64
	pendingMu    sync.Mutex
	pending      map[string]chan *Response
}

func newServerConn(wc *wsConn, dpID string) *serverConn {
	sc := &serverConn{wsConn: wc, dpID: dpID, pending: make(map[string]chan *Response)}
	sc.touch()
	return sc
}

func (sc *serverConn) ID() string          { return sc.dpID }
func (sc *serverConn) LastSeen() time.Time  { return time.Unix(0, sc.lastSeenNano.Load()) }
func (sc *serverConn) Close(reason string)  { _ = sc.wsConn.close(websocket.StatusNormalClosure, reason) }
func (sc *serverConn) touch()               { sc.lastSeenNano.Store(time.Now().UnixNano()) }

func (sc *serverConn) registerPending(id string) chan *Response {
	ch := make(chan *Response, 1)
	sc.pendingMu.Lock()
	sc.pending[id] = ch
	sc.pendingMu.Unlock()
	return ch
}

func (sc *serverConn) unregisterPending(id string) {
	sc.pendingMu.Lock()
	delete(sc.pending, id)
	sc.pendingMu.Unlock()
}

func (sc *serverConn) deliver(resp *Response) {
	sc.pendingMu.Lock()
	ch, ok := sc.pending[resp.ID]
	sc.pendingMu.Unlock()
	if ok {
		select {
		case ch <- resp:
		default:
		}
	}
}

// failAllPending unblocks every waiting caller when the connection drops.
func (sc *serverConn) failAllPending() {
	sc.pendingMu.Lock()
	defer sc.pendingMu.Unlock()
	for id, ch := range sc.pending {
		select {
		case ch <- &Response{JSONRPC: Version, ID: id, Error: NewError(CodeInternalError, "data plane connection closed")}:
		default:
		}
	}
}

// Server is the Control Plane end of the channel: it accepts Data Plane connections and issues
// reverse JSON-RPC requests to them.
type Server struct {
	cfg      ServerConfig
	verifier Verifier
	registry *Registry[*serverConn]
	logger   *log.Logger
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewServer builds a channel server. Its base context is cancelled by Close to stop all read loops.
func NewServer(cfg ServerConfig, verifier Verifier) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		cfg:      cfg,
		verifier: verifier,
		registry: NewRegistry[*serverConn](),
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, serverLoggerComponent)),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// HandleConnect authenticates the handshake, upgrades to WebSocket, registers the connection, and
// runs the response read loop until the connection closes or the server shuts down.
func (s *Server) HandleConnect(w http.ResponseWriter, r *http.Request) {
	if err := s.verifier.Verify(r); err != nil {
		s.logger.Warn(r.Context(), "Rejected data plane handshake", log.Error(err))
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	dpID := r.Header.Get(HeaderDataPlaneID)
	if dpID == "" {
		http.Error(w, errMissingDataPlaneID.Error(), http.StatusBadRequest)
		return
	}

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// The Data Plane is a non-browser client and sends no Origin header.
		InsecureSkipVerify: true,
	})
	if err != nil {
		return // Accept has already written the error response.
	}
	sc := newServerConn(newWSConn(ws, s.cfg.ReadLimit), dpID)
	s.registry.Register(sc)
	s.logger.Info(s.ctx, "Data plane connected", log.String("dpID", dpID))

	defer func() {
		s.registry.Unregister(dpID, sc)
		sc.failAllPending()
		_ = sc.closeNow()
		s.logger.Info(s.ctx, "Data plane disconnected", log.String("dpID", dpID))
	}()

	for {
		var resp Response
		if err := sc.readMessage(s.ctx, &resp); err != nil {
			if s.ctx.Err() == nil {
				s.logger.Debug(s.ctx, "Data plane read loop ended", log.String("dpID", dpID), log.Error(err))
			}
			return
		}
		sc.touch()
		sc.deliver(&resp)
	}
}

// CallMethod sends a JSON-RPC request to the given Data Plane and waits for its tagged response. It
// fails fast with ErrDataPlaneNotConnected when the Data Plane is not connected.
func (s *Server) CallMethod(ctx context.Context, dpID, method string, params any) (json.RawMessage, error) {
	sc, ok := s.registry.Get(dpID)
	if !ok {
		return nil, ErrDataPlaneNotConnected
	}

	// Apply the configured RPC timeout as a fallback only when the caller set no deadline.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && s.cfg.RPCTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.cfg.RPCTimeout)
		defer cancel()
	}

	paramsRaw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	id := utils.GenerateUUID()
	respCh := sc.registerPending(id)
	defer sc.unregisterPending(id)

	if err := sc.writeMessage(ctx, Request{JSONRPC: Version, ID: id, Method: method, Params: paramsRaw}); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

// Connections returns a snapshot of connected Data Planes for observability/health.
func (s *Server) Connections() []ConnInfo {
	return s.registry.List()
}

// Close cancels all read loops and closes every active connection. Safe to call once.
func (s *Server) Close() {
	s.cancel()
	for _, c := range s.registry.entries() {
		c.Close("control plane shutting down")
	}
}
```

Note: `CallMethod` uses `s.cfg.RPCTimeout` (the field added to `ServerConfig` in Task 6) only as a fallback when the caller's context carries no deadline; the future CP caller normally passes a context with its own deadline. There is exactly one `CallMethod` definition, above.

- [ ] **Step 5: Run test to verify it passes**

Run: `cd backend && go test -race ./internal/system/channel/ -run TestServer`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/system/channel/conn.go backend/internal/system/channel/server.go backend/internal/system/channel/server_test.go
git commit -m "Add CP channel server and connection wrapper for the CP-DP channel

Refs #4247"
```

---

## Task 8: DP client

**Files:**
- Create: `backend/internal/system/channel/client.go`
- Test: `backend/internal/system/channel/client_test.go`

**Interfaces:**
- Consumes: `Request`, `Response`, `Router`, `wsConn`, `ClientConfig` (Tasks 2-7); `coder/websocket`; `log`.
- Produces: `Client` with `NewClient(cfg ClientConfig, router *Router) *Client`, `(*Client).Start(ctx context.Context)`, `(*Client).Stop()`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/system/channel/client_test.go` (include the Apache header):
```go
package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		defer c.CloseNow()
		_ = wsjson.Write(ctx, c, Request{JSONRPC: Version, ID: "1", Method: "Echo", Params: json.RawMessage(`"ping"`)})
		var resp Response
		if err := wsjson.Read(ctx, c, &resp); err == nil {
			gotResult <- string(resp.Result)
		}
		if n == 1 {
			c.Close(websocket.StatusNormalClosure, "forcing reconnect")
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/system/channel/ -run TestClient`
Expected: FAIL (NewClient undefined).

- [ ] **Step 3: Write the implementation**

Create `backend/internal/system/channel/client.go` (include the Apache header):
```go
package channel

import (
	"context"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	clientLoggerComponent = "ChannelClient"
	pingTimeout           = 10 * time.Second
)

// Client is the Data Plane end of the channel: it dials the Control Plane, keeps the connection
// alive with a heartbeat, reconnects with backoff, and serves inbound JSON-RPC requests.
type Client struct {
	cfg      ClientConfig
	router   *Router
	logger   *log.Logger
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

// Start launches the reconnect loop in the background. It returns immediately.
func (c *Client) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	go c.run(ctx)
}

// Stop cancels the reconnect loop and waits for it to exit. Safe to call once.
func (c *Client) Stop() {
	c.stopOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		<-c.done
	})
}

func (c *Client) run(ctx context.Context) {
	defer close(c.done)
	backoff := c.cfg.ReconnectInitial
	for {
		if ctx.Err() != nil {
			return
		}
		dialed, err := c.connectAndServe(ctx)
		if ctx.Err() != nil {
			return
		}
		if dialed {
			backoff = c.cfg.ReconnectInitial // reset after a healthy connection
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

// connectAndServe dials once and runs the read loop until the connection ends. The bool reports
// whether the dial succeeded (used to reset backoff).
func (c *Client) connectAndServe(ctx context.Context) (bool, error) {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.cfg.AuthToken)
	header.Set(HeaderDataPlaneID, c.cfg.ID)

	ws, _, err := websocket.Dial(ctx, c.cfg.ControlPlaneURL, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		return false, err
	}
	conn := newWSConn(ws, c.cfg.ReadLimit)
	defer conn.closeNow()
	c.logger.Info(ctx, "Connected to control plane", log.String("url", c.cfg.ControlPlaneURL))

	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()
	go c.heartbeat(hbCtx, conn)

	for {
		var req Request
		if err := conn.readMessage(ctx, &req); err != nil {
			return true, err
		}
		// Dispatch in a goroutine so a slow handler does not block reading (and pong processing).
		go func(req Request) {
			resp := c.router.Dispatch(ctx, req)
			if err := conn.writeMessage(ctx, resp); err != nil {
				c.logger.Warn(ctx, "Failed to write RPC response", log.String("id", req.ID), log.Error(err))
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test -race ./internal/system/channel/ -run TestClient`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/system/channel/client.go backend/internal/system/channel/client_test.go
git commit -m "Add DP channel client with heartbeat and reconnect for the CP-DP channel

Refs #4247"
```

---

## Task 9: Import method binding, ping, and Initialize entrypoints

**Files:**
- Create: `backend/internal/system/channel/methods.go`
- Create: `backend/internal/system/channel/init.go`
- Test: `backend/internal/system/channel/channel_test.go`

**Interfaces:**
- Consumes: `Router`, `Server`, `Client`, `ServerConfig`, `ClientConfig`, `Error`, codes, `Version` (Tasks 2-8); `importer.ImportRequest`, `importer.ImportResponse`, `importer.ImportServiceInterface` from `github.com/thunder-id/thunderid/internal/system/importer`; `tidcommon` from `github.com/thunder-id/thunderid/pkg/thunderidengine/common`.
- Produces: consts `MethodImportRun="Import.Run"`, `MethodAgentPing="Agent.Ping"`; `ImportRunner` interface; `RegisterDataPlaneMethods(router *Router, importer ImportRunner, dpID string)`; `(*Server).CallImport(ctx, dpID string, req *importer.ImportRequest) (*importer.ImportResponse, error)`; `(*Server).Ping(ctx, dpID string) error`; `InitializeServer(mux *http.ServeMux, cfg ServerConfig) *Server`; `InitializeClient(cfg ClientConfig, runner ImportRunner) *Client`.

- [ ] **Step 1: Write the failing end-to-end test**

Create `backend/internal/system/channel/channel_test.go` (include the Apache header):
```go
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

func (f *fakeImportRunner) ImportResources(_ context.Context, req *importer.ImportRequest) (*importer.ImportResponse, *tidcommon.ServiceError) {
	f.lastContent = req.Content
	return f.resp, f.svcErr
}

// wireEndToEnd starts a real Server behind httptest and a real Client dialing it, and waits for the
// connection to register. It returns the server and the fake runner the DP uses.
func wireEndToEnd(t *testing.T, ctx context.Context, runner *fakeImportRunner) *Server {
	t.Helper()
	cfg := ServerConfig{Enabled: true, Path: "/cp/connect", AuthToken: "tok"}
	mux := http.NewServeMux()
	s := InitializeServer(mux, cfg)
	hs := httptest.NewServer(mux)

	client := InitializeClient(ClientConfig{
		Enabled:          true,
		ID:               "dp-1",
		ControlPlaneURL:  "ws" + strings.TrimPrefix(hs.URL, "http") + "/cp/connect",
		AuthToken:        "tok",
		PingInterval:     time.Second,
		ReconnectInitial: 10 * time.Millisecond,
		ReconnectMax:     100 * time.Millisecond,
	}, runner)
	client.Start(ctx)

	t.Cleanup(func() { client.Stop(); s.Close(); hs.Close() })
	require.Eventually(t, func() bool { return len(s.Connections()) == 1 }, 3*time.Second, 20*time.Millisecond)
	return s
}

func TestCallImportRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runner := &fakeImportRunner{resp: &importer.ImportResponse{Summary: &importer.ImportSummary{TotalDocuments: 2, Imported: 2}}}
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

func TestCallImportOfflineFailsFast(t *testing.T) {
	s := InitializeServer(http.NewServeMux(), ServerConfig{Enabled: true, Path: "/cp/connect", AuthToken: "tok"})
	defer s.Close()
	_, err := s.CallImport(context.Background(), "dp-1", &importer.ImportRequest{Content: "x"})
	assert.ErrorIs(t, err, ErrDataPlaneNotConnected)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/system/channel/ -run TestCallImport`
Expected: FAIL (InitializeServer/CallImport undefined).

- [ ] **Step 3: Write methods.go**

Create `backend/internal/system/channel/methods.go` (include the Apache header):
```go
package channel

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/system/importer"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Channel RPC method names.
const (
	MethodImportRun = "Import.Run"
	MethodAgentPing = "Agent.Ping"
)

// ImportRunner is the subset of the importer service the channel invokes on the Data Plane. It is
// satisfied by importer.ImportServiceInterface.
type ImportRunner interface {
	ImportResources(ctx context.Context, request *importer.ImportRequest) (*importer.ImportResponse, *tidcommon.ServiceError)
}

// pingResult is the Agent.Ping reply payload.
type pingResult struct {
	DataPlaneID string `json:"dataPlaneId"`
}

// RegisterDataPlaneMethods registers the Data Plane's inbound RPC handlers on the router.
func RegisterDataPlaneMethods(router *Router, runner ImportRunner, dpID string) {
	router.Register(MethodImportRun, func(ctx context.Context, params json.RawMessage) (json.RawMessage, *Error) {
		var req importer.ImportRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, NewError(CodeInvalidParams, "invalid import params: "+err.Error())
		}
		resp, svcErr := runner.ImportResources(ctx, &req)
		if svcErr != nil {
			return nil, serviceErrorToRPC(svcErr)
		}
		raw, err := json.Marshal(resp)
		if err != nil {
			return nil, NewError(CodeInternalError, err.Error())
		}
		return raw, nil
	})

	router.Register(MethodAgentPing, func(_ context.Context, _ json.RawMessage) (json.RawMessage, *Error) {
		raw, err := json.Marshal(pingResult{DataPlaneID: dpID})
		if err != nil {
			return nil, NewError(CodeInternalError, err.Error())
		}
		return raw, nil
	})
}

// serviceErrorToRPC maps a ThunderID ServiceError to a JSON-RPC error, preserving the human message
// and carrying the service error code in Data.
func serviceErrorToRPC(svcErr *tidcommon.ServiceError) *Error {
	code := CodeInternalError
	if svcErr.Type == tidcommon.ClientErrorType {
		code = CodeInvalidParams
	}
	data, _ := json.Marshal(map[string]string{"serviceCode": svcErr.Code})
	return &Error{Code: code, Message: svcErr.Error.DefaultValue, Data: data}
}

// CallImport pushes an import request to the given Data Plane and decodes its response.
func (s *Server) CallImport(ctx context.Context, dpID string, req *importer.ImportRequest) (*importer.ImportResponse, error) {
	raw, err := s.CallMethod(ctx, dpID, MethodImportRun, req)
	if err != nil {
		return nil, err
	}
	var resp importer.ImportResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Ping actively round-trips the Agent.Ping RPC to confirm the Data Plane's handler loop is alive.
func (s *Server) Ping(ctx context.Context, dpID string) error {
	_, err := s.CallMethod(ctx, dpID, MethodAgentPing, nil)
	return err
}
```

- [ ] **Step 4: Write init.go**

Create `backend/internal/system/channel/init.go` (include the Apache header):
```go
package channel

import "net/http"

// InitializeServer builds the Control Plane channel server and, when enabled, registers its
// WebSocket route on mux. It returns the server (whose Close the caller invokes at shutdown), or nil
// when disabled.
func InitializeServer(mux *http.ServeMux, cfg ServerConfig) *Server {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Path == "" {
		cfg.Path = "/cp/connect"
	}
	s := NewServer(cfg, newTokenVerifier(cfg.AuthToken))
	mux.HandleFunc("GET "+cfg.Path, s.HandleConnect)
	return s
}

// InitializeClient builds the Data Plane channel client with the Import and Ping handlers
// registered, or returns nil when disabled. The caller owns Start/Stop.
func InitializeClient(cfg ClientConfig, runner ImportRunner) *Client {
	if !cfg.Enabled {
		return nil
	}
	router := NewRouter()
	RegisterDataPlaneMethods(router, runner, cfg.ID)
	return NewClient(cfg, router)
}
```

Note: the end-to-end test calls `InitializeServer` with `Enabled: true` and a non-empty `Path`, so the route registers. Keep `Path` set in the test (it is).

- [ ] **Step 5: Run test to verify it passes**

Run: `cd backend && go test -race ./internal/system/channel/`
Expected: PASS (all channel tests).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/system/channel/methods.go backend/internal/system/channel/init.go backend/internal/system/channel/channel_test.go
git commit -m "Add Import.Run binding, Ping, and Initialize entrypoints for the CP-DP channel

Refs #4247"
```

---

## Task 10: Security wiring (public path + access-log exclusion)

**Files:**
- Modify: `backend/internal/system/security/permissions.go`
- Modify: `backend/cmd/cpserver/main.go`
- Test: `backend/internal/system/security/service_test.go` (add a case)

**Why both edits:** Adding `/cp/connect` to `publicPaths` exempts the WebSocket upgrade from JWT auth (the channel does its own token check). Adding `/cp/connect` to `cmd/cpserver`'s `accessLogExcludePaths` bypasses the `loggingResponseWriter` wrapper, which implements neither `Hijack()` nor `Unwrap()` and would otherwise prevent `coder/websocket` from hijacking the connection and clearing the 10s `WriteTimeout`.

- [ ] **Step 1: Write the failing test**

IMPORTANT: the public-path suite builds its security service from a test-local fixture slice `testPublicPaths` (declared near `service_test.go:34`), NOT the production `publicPaths`. So this task edits both: the fixture (to turn the new test row green) and production `publicPaths` (the real wiring).

In `backend/internal/system/security/service_test.go`, find the `TestProcess_PublicPaths` (or equivalent public-path table test) `testCases` slice and add an entry:
```go
		{"CP channel connect", "/cp/connect"},
```
This row asserts `Process` returns no error (runtime context) for `/cp/connect`; it FAILS until `/cp/connect` is added to the `testPublicPaths` fixture in Step 3.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/system/security/ -run TestProcess_PublicPaths`
Expected: FAIL for the `/cp/connect` case (path not in the fixture yet).

- [ ] **Step 3: Add /cp/connect to both the fixture and production publicPaths**

First, in `backend/internal/system/security/service_test.go`, add `"/cp/connect",` to the `testPublicPaths` fixture slice (near `service_test.go:34`). This is what turns the Step 1 row green.

Then, in `backend/internal/system/security/permissions.go`, add to the production `publicPaths` slice (next to the `/mcp/**` entry, mirroring its comment style):
```go
	"/cp/connect", // Control Plane phone-home WebSocket; authenticated by a shared token in the channel handler.
```

Give the production change its own coverage by adding a standalone test to `service_test.go` (a top-level func, not part of the suite):
```go
func TestProductionPublicPathsIncludesChannelConnect(t *testing.T) {
	assert.Contains(t, publicPaths, "/cp/connect")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/system/security/ -run TestProcess_PublicPaths`
Expected: PASS.

- [ ] **Step 5: Exclude /cp/connect from the CP access log**

In `backend/cmd/cpserver/main.go`, in `accessLogExcludePaths`, change the seed slice to include the channel route:
```go
func accessLogExcludePaths(configured []string) []string {
	paths := []string{"/console/", "/cp/connect"}
	for _, prefix := range configured {
		if prefix != "" && prefix != "/" {
			paths = append(paths, prefix)
		}
	}
	return paths
}
```

- [ ] **Step 6: Verify the build and security tests**

Run: `cd backend && go build ./cmd/cpserver/ && go test ./internal/system/security/`
Expected: success + PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/system/security/permissions.go backend/internal/system/security/service_test.go backend/cmd/cpserver/main.go
git commit -m "Make the CP channel route public and hijackable for the CP-DP channel

Refs #4247"
```

---

## Task 11: Global config section and defaults

**Files:**
- Modify: `backend/internal/system/config/config.go`
- Modify: `backend/cmd/server/config/default.json`
- Test: `backend/internal/system/config/config_test.go` (add a case)

**Interfaces:**
- Produces: `config.ChannelConfig{Server ChannelServerConfig; Client ChannelClientConfig}` on `config.Config` as field `Channel` (yaml/json `channel`), with `ChannelServerConfig{Enabled bool; Path string; AuthToken string; ReadLimitBytes int64}` and `ChannelClientConfig{Enabled bool; ID string; ControlPlaneURL string; AuthToken string; ReadLimitBytes int64; PingIntervalSeconds int; RPCTimeoutSeconds int; ReconnectInitialSeconds int; ReconnectMaxSeconds int}`.

- [ ] **Step 1: Write the failing test**

`config_test.go` is in package `config` (an internal test), so it calls `LoadConfig` unqualified, mirroring the existing load tests in that file (which use `LoadConfig(userFile, "", tempDir)` with a temp deployment.yaml). Add this test:
```go
func TestLoadConfigParsesChannelSection(t *testing.T) {
	dir := t.TempDir()
	userFile := filepath.Join(dir, "deployment.yaml")
	require.NoError(t, os.WriteFile(userFile, []byte(`
channel:
  server:
    enabled: true
    path: "/cp/connect"
    auth_token: "tok"
    rpc_timeout_seconds: 30
  client:
    enabled: true
    id: "dp-1"
    control_plane_url: "wss://cp.example/cp/connect"
    auth_token: "tok"
    ping_interval_seconds: 30
`), 0o600))

	cfg, err := LoadConfig(userFile, "", dir)
	require.NoError(t, err)
	assert.True(t, cfg.Channel.Server.Enabled)
	assert.Equal(t, "/cp/connect", cfg.Channel.Server.Path)
	assert.Equal(t, 30, cfg.Channel.Server.RPCTimeoutSeconds)
	assert.Equal(t, "dp-1", cfg.Channel.Client.ID)
	assert.Equal(t, 30, cfg.Channel.Client.PingIntervalSeconds)
}
```
Ensure `config_test.go` imports `os`, `path/filepath`, and testify `assert`/`require` (the existing load tests already use this pattern, so most imports are present). The point of the test: a `channel:` block parses under the strict (`KnownFields(true)`) YAML decoder, proving the new struct field exists. If `LoadConfig` with only a `channel:` block trips an unrelated section `Validate()`, copy the minimal valid deployment.yaml body from the nearest sibling load test and add the `channel:` block to it.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/system/config/ -run TestLoadConfigParsesChannelSection`
Expected: FAIL (Channel field / strict-yaml unknown key `channel`).

- [ ] **Step 3: Add the config structs**

In `backend/internal/system/config/config.go`, add the field to the `Config` struct (aligned with the surrounding tags):
```go
	Channel              ChannelConfig                    `yaml:"channel"               json:"channel"`
```
And add the struct definitions (near the other section structs such as `CryptoConfig`):
```go
// ChannelConfig configures the CP-DP phone-home WebSocket channel. The Server block is used by the
// Control Plane (cpserver); the Client block by the Data Plane (dpserver).
type ChannelConfig struct {
	Server ChannelServerConfig `yaml:"server" json:"server"`
	Client ChannelClientConfig `yaml:"client" json:"client"`
}

// ChannelServerConfig configures the Control Plane channel WebSocket server.
type ChannelServerConfig struct {
	Enabled           bool   `yaml:"enabled"            json:"enabled"`
	Path              string `yaml:"path"               json:"path"`
	AuthToken         string `yaml:"auth_token"         json:"auth_token"`
	ReadLimitBytes    int64  `yaml:"read_limit_bytes"   json:"read_limit_bytes"`
	RPCTimeoutSeconds int    `yaml:"rpc_timeout_seconds" json:"rpc_timeout_seconds"`
}

// ChannelClientConfig configures the Data Plane channel WebSocket client.
type ChannelClientConfig struct {
	Enabled                 bool   `yaml:"enabled"                   json:"enabled"`
	ID                      string `yaml:"id"                        json:"id"`
	ControlPlaneURL         string `yaml:"control_plane_url"         json:"control_plane_url"`
	AuthToken               string `yaml:"auth_token"                json:"auth_token"`
	ReadLimitBytes          int64  `yaml:"read_limit_bytes"          json:"read_limit_bytes"`
	PingIntervalSeconds     int    `yaml:"ping_interval_seconds"     json:"ping_interval_seconds"`
	ReconnectInitialSeconds int    `yaml:"reconnect_initial_seconds" json:"reconnect_initial_seconds"`
	ReconnectMaxSeconds     int    `yaml:"reconnect_max_seconds"     json:"reconnect_max_seconds"`
}
```

- [ ] **Step 4: Add disabled defaults**

In `backend/cmd/server/config/default.json`, add a top-level `channel` block (disabled by default so nothing changes unless an operator opts in):
```json
  "channel": {
    "server": {
      "enabled": false,
      "path": "/cp/connect",
      "auth_token": "",
      "read_limit_bytes": 1048576,
      "rpc_timeout_seconds": 30
    },
    "client": {
      "enabled": false,
      "id": "",
      "control_plane_url": "",
      "auth_token": "",
      "read_limit_bytes": 1048576,
      "ping_interval_seconds": 30,
      "reconnect_initial_seconds": 1,
      "reconnect_max_seconds": 30
    }
  },
```
(Insert as a sibling of the existing top-level keys; mind the trailing comma so the JSON stays valid.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd backend && go test ./internal/system/config/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/system/config/config.go backend/internal/system/config/config_test.go backend/cmd/server/config/default.json
git commit -m "Add channel config section and disabled defaults for the CP-DP channel

Refs #4247"
```

---

## Task 12: Wire the channel server into cpserver

**Files:**
- Modify: `backend/cmd/cpserver/servicemanager.go`

**Interfaces:**
- Consumes: `channel.InitializeServer`, `channel.ServerConfig`, `config.GetServerRuntime().Config.Channel.Server` (Tasks 9, 11).

Note: `cmd/cpserver` has no `*_test.go` harness (only `cmd/server` does), so this is thin glue validated by `build` + `vet`, not a failing-test-first cycle. The logic it wires (`InitializeServer`) is already covered by Task 9. This deviation from strict TDD is intentional and matches repo convention.

- [ ] **Step 1: Add the package var and Initialize call**

In `backend/cmd/cpserver/servicemanager.go`, near the existing `var observabilitySvc ...` declaration, add:
```go
// channelServer is the CP-DP phone-home WebSocket server. It is held here so gracefulShutdown can
// close it. Nil when the channel is disabled.
var channelServer *channel.Server
```
Add the import `"github.com/thunder-id/thunderid/internal/system/channel"`.

Inside `registerServices`, just before the `return jwtService, runtimeCryptoSvc, importService` line, add:
```go
	// Initialize the CP-DP channel server (phone-home WebSocket). No-op when disabled.
	chCfg := config.GetServerRuntime().Config.Channel.Server
	channelServer = channel.InitializeServer(mux, channel.ServerConfig{
		Enabled:    chCfg.Enabled,
		Path:       chCfg.Path,
		AuthToken:  chCfg.AuthToken,
		ReadLimit:  chCfg.ReadLimitBytes,
		RPCTimeout: time.Duration(chCfg.RPCTimeoutSeconds) * time.Second,
	})
```

- [ ] **Step 2: Close it on shutdown**

In the same file, in `unregisterServices`, add:
```go
	if channelServer != nil {
		channelServer.Close()
	}
```

- [ ] **Step 3: Verify build and compile-time test of the package**

Run: `cd backend && go build ./cmd/cpserver/ && go vet ./cmd/cpserver/`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/cpserver/servicemanager.go
git commit -m "Start the CP-DP channel server in cpserver

Refs #4247"
```

---

## Task 13: Wire the channel client into dpserver

**Files:**
- Modify: `backend/cmd/dpserver/main.go`

**Interfaces:**
- Consumes: `channel.InitializeClient`, `channel.ClientConfig`, the `importService` returned by `registerServices`, `config.GetServerRuntime().Config.Channel.Client` (Tasks 9, 11).

Note: `cmd/dpserver` has no `*_test.go` harness, so this is thin glue validated by `build` + `vet`, not a failing-test-first cycle. The logic it wires (`InitializeClient`) is already covered by Task 9. This deviation from strict TDD is intentional and matches repo convention.

- [ ] **Step 1: Build and start the client after the bootstrap check**

In `backend/cmd/dpserver/main.go`, add the import `"github.com/thunder-id/thunderid/internal/system/channel"` and `"time"` (if not already imported).

After `revocationSyncer.Start(ctx)` (the non-bootstrap path), add:
```go
	// Start the CP-DP channel client (phone-home WebSocket). Nil when disabled.
	channelClient := initChannelClient(importService)
	if channelClient != nil {
		channelClient.Start(ctx)
	}
```
Change the `gracefulShutdown(ctx, logger, server, cacheManager, revocationSyncer)` call to pass the client:
```go
	gracefulShutdown(ctx, logger, server, cacheManager, revocationSyncer, channelClient)
```

- [ ] **Step 2: Add the mapping helper and the shutdown param**

Add a helper in the same file:
```go
// initChannelClient builds the CP-DP channel client from config, or returns nil when disabled.
func initChannelClient(importService importer.ImportServiceInterface) *channel.Client {
	c := config.GetServerRuntime().Config.Channel.Client
	return channel.InitializeClient(channel.ClientConfig{
		Enabled:          c.Enabled,
		ID:               c.ID,
		ControlPlaneURL:  c.ControlPlaneURL,
		AuthToken:        c.AuthToken,
		ReadLimit:        c.ReadLimitBytes,
		PingInterval:     time.Duration(c.PingIntervalSeconds) * time.Second,
		ReconnectInitial: time.Duration(c.ReconnectInitialSeconds) * time.Second,
		ReconnectMax:     time.Duration(c.ReconnectMaxSeconds) * time.Second,
	}, importService)
}
```
Add the import `"github.com/thunder-id/thunderid/internal/system/importer"` if it is not already imported by main.go (it is used only for the type here). Update `gracefulShutdown` to accept and stop the client:
```go
func gracefulShutdown(
	ctx context.Context,
	logger *log.Logger,
	server *http.Server,
	cacheManager cache.CacheManagerInterface,
	revocationSyncer revocationcache.Syncer,
	channelClient *channel.Client,
) {
	// ... existing body ...
	revocationSyncer.Stop()

	if channelClient != nil {
		channelClient.Stop()
	}
	// ... rest unchanged ...
}
```

- [ ] **Step 3: Verify build**

Run: `cd backend && go build ./cmd/dpserver/ && go vet ./cmd/dpserver/`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/dpserver/main.go
git commit -m "Start the CP-DP channel client in dpserver

Refs #4247"
```

---

## Task 14: Full validation and squash

**Files:** none (validation only).

- [ ] **Step 1: Run the channel package tests with race detection**

Run: `cd backend && go test -race -count=1 ./internal/system/channel/...`
Expected: PASS.

- [ ] **Step 2: Build all three server binaries**

Run: `cd backend && go build ./cmd/cpserver ./cmd/dpserver ./cmd/server`
Expected: success (confirms all wiring compiles).

- [ ] **Step 3: Run the authoritative gate**

Run: `make pr_checks`
Expected: PASS (verify_mocks, lint, format_check, unit + integration tests, builds). If `verify_mocks` flags the channel package, it means an interface was configured for mockery unexpectedly; this plan adds no `.mockery.*.yml` entries, so there should be nothing to generate. If lint flags the new files, fix in place (for example unused imports, comment formatting) and re-run.

- [ ] **Step 4: Squash to a single commit**

Per AGENTS.md (one commit per PR), squash the task commits into one before opening the PR:
```bash
git reset --soft $(git merge-base HEAD origin/feature/cp-dp)
git commit -m "Add CP-DP phone-home WebSocket channel with Import.Run

Adds internal/system/channel: a JSON-RPC 2.0 reverse-command channel where the
Data Plane (dpserver) dials the Control Plane (cpserver) over coder/websocket,
authenticates with a shared token, keeps a persistent connection with heartbeat
and reconnect, and serves inbound commands. First command is Import.Run, invoked
from the CP via Server.CallImport. Adds a connection registry with an Agent.Ping
health probe.

Refs #4247

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
Expected: `git log --oneline origin/feature/cp-dp..HEAD` shows exactly one commit.

---

## Self-Review

**Spec coverage (against `docs/superpowers/specs/2026-07-23-cp-dp-websocket-channel-design.md`):**

- Topology (CP server in cpserver, DP client in dpserver, hybrid off): Tasks 12, 13; config-gated via Task 11. Covered.
- Components (`rpc`, `transport`, `registry`, `auth`, server/client, methods): realized as a flat `internal/system/channel` package (Tasks 2-9). Delta from the spec's sub-package layout, chosen to match the repo's flat-package convention (revocationcache/export/importer); noted in File Structure.
- Config section: Task 11. Covered.
- Protocol (JSON-RPC 2.0): Tasks 2-3. Covered.
- Data flow (handshake, Import RPC, ping, reconnect): Tasks 7, 8, 9. Covered.
- Error handling (auth 401, duplicate-id eviction, read limit, MethodNotFound, offline fail-fast, timeout, graceful shutdown): Tasks 4, 5, 7, 8, 9, 12, 13. Covered. (ParseError/InvalidRequest classification of malformed inbound frames is minimal in v1: the DP read loop surfaces decode failures as read errors; per-frame ParseError responses are not emitted because a frame with no id cannot be answered. Acceptable for v1; noted here.)
- Testing (unit + integration): Tasks 2-9. Covered.
- Use case (Import, not Export): the spec and this plan both specify Import, and both record why in the spec's "Why Import (and not Export) is the v1 use case" section. No divergence.

**Delta from spec worth surfacing to the user:**
1. Flat package layout instead of sub-packages (convention-driven).
2. The access-log `Hijack`/`Unwrap` blocker and 10s `WriteTimeout` were not in the spec; Task 10 handles both by excluding `/cp/connect` from the CP access log. This is the single most important correctness fix.
3. No use-case divergence: spec and plan both specify Import.
4. Config keys differ from spec section 6: `reconnect` is flattened to `reconnect_initial_seconds`/`reconnect_max_seconds` (not nested), `rpc_timeout_seconds` lives under the `server` block (the CP is the caller) rather than the client, and the spec's server-side `ping_interval_seconds`/`write_timeout_seconds` are not implemented (the DP owns the heartbeat; the CP write deadline is cleared by the access-log exclusion). The plan's keys are authoritative.
5. Config lives in `internal/system/config/config.go` (the home of the aggregate `Config` struct), not `pkg/thunderidengine/config` as spec sections 6 and 11 state. The plan's location is correct; the spec text is imprecise.

These deltas mean the committed design spec (`docs/superpowers/specs/2026-07-23-cp-dp-websocket-channel-design.md`) was partly stale on component layout, config keys, and config location. The spec has since been updated to match what was built.

**Adversarial verification:** This plan was checked by four independent critics (codebase-accuracy, Go compilability, coder/websocket API correctness, spec/process). All confirmed defects were fixed inline: the unused `encoding/json` import in `server_test.go`, the `TestCallImportOfflineFailsFast` empty-`Path` panic (plus a defensive `Path` default in `InitializeServer`), the duplicate `CallMethod` versions (now one, honoring `RPCTimeout` as a fallback), the `testPublicPaths` fixture in Task 10, the undefined `loadTestConfigWithChannel` helper (now concrete), the dead `rpc_timeout` config (now consumed CP-side), and the missing `Co-Authored-By` trailer. The coder/websocket API usage was confirmed correct with no findings.

**Placeholder scan:** No TBD/TODO. Task 11's remaining conditional ("if `LoadConfig` trips an unrelated section `Validate()`, copy the minimal valid deployment.yaml from the nearest sibling load test") is a genuine codebase-shape contingency, not a missing artifact; the primary path is concrete.

**Type consistency:** `Request`/`Response`/`Error`/`Version`/codes are defined in Task 2 and used unchanged in Tasks 3, 7, 8, 9. `Registry[*serverConn]` (Task 5/7), `ConnEntry` methods `ID()/LastSeen()/Close(reason)` implemented by `serverConn` (Task 7). `ImportRunner` (Task 9) matches `importer.ImportServiceInterface.ImportResources` exactly. `ServerConfig` (with `RPCTimeout`) and `ClientConfig` (Task 6) fields match their use in Tasks 7-9 and the cmd mapping in Tasks 12-13. There is exactly one `CallMethod` definition, using the `s.cfg.RPCTimeout` field (not a method call).
