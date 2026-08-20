# CP-DP Bi-directional WebSocket Channel (v1) - Design

- Status: Approved for planning
- Date: 2026-07-23
- Branch: `feature/cp-dp`
- Related: [Discussion #4271](https://github.com/thunder-id/thunderid/discussions/4271), [Issue #4247](https://github.com/thunder-id/thunderid/issues/4247), builds on [PR #4287](https://github.com/thunder-id/thunderid/pull/4287) (preliminary CP/DP separation)

## 1. Problem and Goal

Data Plane (DP) instances are deployed in private networks or corporate VPCs and cannot accept
inbound traffic from the Control Plane (CP). We need an outbound "phone-home" channel: the DP
initiates and maintains a persistent, bi-directional connection to the CP, over which the CP can
push commands to the DP without the DP exposing any inbound endpoint.

This document specifies the **first increment (v1)**: the channel mechanism plus one concrete use
case. The chosen use case is **Import**: the CP pushes an import request down to the DP, and the DP
applies it locally through its existing import service.

### Goals

1. CP acts as the WebSocket server; DP acts as the WebSocket client.
2. The DP connects to the CP and keeps that connection alive indefinitely (heartbeat + auto-reconnect).
3. Health-check capability to observe and actively probe connectivity.
4. As the initial use case, the DP's Import functionality is invocable from the CP over the channel.

### Non-goals (explicitly deferred to later increments)

- Redis pub/sub or any distributed routing across horizontally scaled CP nodes (v1 is a single CP instance, in-memory registry).
- mTLS and certificate-authority provisioning (v1 uses a shared bearer token; a clean seam is left for mTLS).
- Replay-protection nonces and timestamp windows.
- Signed audit trails beyond normal structured logging.
- Prometheus metrics for the channel.
- Offline command buffering or queueing (v1 fails fast when the target DP is offline).
- Handshake rate limiting.

### Security consequence of the deferred mTLS decision

Read this before enabling the channel. Because v1 authenticates with a single token shared by every
Data Plane, and the Data Plane then asserts its own identity in a request header, the token is
effectively a Data-Plane-impersonation credential. Any holder of it, including a compromised
low-value Data Plane, can connect claiming another Data Plane's id. Doing so evicts the genuine
connection (single-active-socket policy) and makes the impersonator the recipient of every command
the Control Plane subsequently targets at that id. With handshake rate limiting also deferred, an
impersonator can hold the victim offline by reconnecting.

Provision the token per environment through the `file://` form, treat it as a high-value secret, and
do not reuse one token across trust boundaries. The cheap interim hardening, ahead of full mTLS, is
per-Data-Plane tokens: the `Verifier` seam can return the authenticated identity instead of the
handshake trusting the client-supplied header.

### Reachability caveat

Per the programmatic-API-only decision in section 3, v1 ships no operator-facing trigger. A deployed
Control Plane and Data Plane pair will establish and maintain the channel, and `CallImport`, `Ping`,
and `Connections` are callable from Go, but no production code path invokes them yet. A CLI
subcommand or a permission-gated Control Plane endpoint is needed before the channel is usable in a
real deployment.

## 2. Context: the landed CP/DP separation (PR #4287)

The separation currently in `feature/cp-dp` is an API-gating and separate-builds scaffold. It does
**not** include any CP-to-DP communication channel; that is what this design adds. Relevant facts
this design aligns to:

- `server.mode` config field: `hybrid` (default), `cp`, `dp`. It gates only the management HTTP
  surface; public runtime endpoints are always served. An unset or unrecognized value falls open to
  `hybrid`.
- Three binaries:
  - `cmd/server` (hybrid, unchanged).
  - `cmd/cpserver` (defaults `mode=cp`, registers management services only, serves the Console SPA).
  - `cmd/dpserver` (defaults `mode=dp`, registers the full runtime plus management, adds
    `authn`/`oauth`/`flowexec`, serves the Gate SPA).
- Route-to-plane classification lives in `internal/system/security/permissions.go`
  (`managementRoutePlanes`, first-match-wins). A `cp`/`dp` instance returns 404 for the other
  plane's management routes. Public runtime routes are never plane-gated.
- Both `/import` and `/export` are currently classified as Control-Plane routes, and both binaries
  still register the full `importer` and `export` services (the data split is only preliminary and
  is not enforced at the data layer yet).

### Why Import (and not Export) is the v1 use case

The phone-home direction is CP to DP. The natural first command is the CP pushing configuration or
resources **down** to the DP to be applied there. Import fits this direction: the CP sends import
content, and the DP applies it against its local runtime. The intent going forward is for `/import`
to be a Data-Plane responsibility. For v1 the mechanism is what matters; the DP exposes a generic
`Import.Run` method backed by its existing import service, and the CP invokes it programmatically.

## 3. Key Decisions

| Area | Decision |
|---|---|
| Deployment model | Role is determined by the binary: WS server in `cmd/cpserver`, WS client in `cmd/dpserver`. No new mode flag; reuse the existing binary/`server.mode` split. |
| WebSocket library | `github.com/coder/websocket` (new dependency, approved). |
| Handshake auth (v1) | Shared bearer token, validated by the CP. Verifier is an interface so mTLS can be added later without touching the channel core. |
| Message protocol | Minimal hand-rolled JSON-RPC 2.0 envelope (no library dependency). |
| CP-side surface | Programmatic API only: `CallImport(ctx, dpID, req)` and `Ping(ctx, dpID)`. No new HTTP endpoint on the CP. |
| Health check | CP connection registry (per-DP state and last-seen) plus an explicit `Agent.Ping` RPC that round-trips the DP's handler loop. Transport ping/pong keepalive and DP auto-reconnect are always on. |
| Scope | Minimal core only; see the non-goals in section 1. |
| Failure when DP offline | Fail fast; no buffering. |

## 4. Topology

Role is selected by which binary runs, consistent with how PR #4287 structured the split. The
channel starts only when its config section is present for that binary, so `cpserver`/`dpserver`
still run without it until configured (non-breaking).

- `cmd/cpserver` (Control Plane): starts the WS server at `GET /cp/connect`, maintains the
  connection registry, and exposes `CallImport(ctx, dpID, req)` and `Ping(ctx, dpID)` to CP code.
- `cmd/dpserver` (Data Plane): starts the WS client that dials the CP, authenticates, keeps the
  connection alive (ping/pong plus reconnect), and serves inbound RPC by dispatching to local
  services (`Import.Run` to the import service, `Agent.Ping` to a liveness reply).
- `cmd/server` (hybrid): channel off, unchanged.

## 5. Components

New subsystem under the existing `internal/system/` convention. It is a single flat package, matching
the layout of comparable subsystems in this repo (`revocationcache`, `importer`, `export`) rather than
splitting into sub-packages:

```
internal/system/channel/
  jsonrpc.go  JSON-RPC 2.0 envelope types, version and error-code constants
  router.go   Router: method name to handler, dispatch, MethodNotFound
  registry.go CP-side: dpID to conn, connection state and last-seen, single-active-socket-per-dp
  auth.go     handshake Verifier interface plus token implementation (mTLS seam, no core change)
  errors.go   package errors, including the exported ErrDataPlaneNotConnected
  config.go   package-local ServerConfig and ClientConfig, decoupled from system/config
  conn.go     coder/websocket connection wrapper: read, write, ping, close semantics
  server.go   CP: Accept + auth + register; route inbound responses; CallMethod; Connections; Close
  client.go   DP: dial + auth + reconnect/backoff; serve inbound via Router; heartbeat
  methods.go  method-name constants, ImportRunner, CallImport, Ping, ServiceError mapping
  init.go     InitializeServer (CP) and InitializeClient (DP); both return nil when disabled
```

- Import cycle avoidance: `methods.go` imports `internal/system/importer` (which never imports
  `channel`). The DP registers `Import.Run` to `importer.ImportServiceInterface.ImportResources`,
  and `Agent.Ping` to a trivial liveness reply. Wiring happens in `cmd/dpserver` (which already
  imports `importer`).
- Security integration: the CP WS route (`/cp/connect`) is added to `publicPaths` in
  `internal/system/security/permissions.go` so the JWT middleware skips it; the channel does its
  own token auth on the upgrade request. Only `cpserver` registers this route, so no
  `managementRoutePlanes` entry is required.

## 6. Configuration

A `channel` section is added to the aggregate `Config` struct in `internal/system/config/config.go`
(the home of the top-level configuration type; `pkg/thunderidengine/config` holds only shared
sub-structs). Each side carries an explicit `enabled` flag, and both default to false, so the channel
is off unless an operator turns it on. Defaults live in `cmd/server/config/default.json`.

`auth_token` values may use the `file://` form. That is resolved by a global text substitution pass
over the raw configuration (`utils.SubstituteFilePaths`) before parsing, not by any per-field logic,
so it works for any string field.

```yaml
channel:
  server:                            # used by cpserver
    enabled: true
    path: "/cp/connect"              # see the warning below before changing this
    auth_token: "file://config/secrets/cp_dp_token"
    read_limit_bytes: 1048576
    rpc_timeout_seconds: 30          # CP-side: it is the party that issues RPCs
  client:                            # used by dpserver
    enabled: true
    id: "dp-1"                       # DataPlaneID; registry key on the CP
    control_plane_url: "wss://localhost:8090/cp/connect"
    auth_token: "file://config/secrets/cp_dp_token"
    read_limit_bytes: 1048576
    ping_interval_seconds: 30
    reconnect_initial_seconds: 1
    reconnect_max_seconds: 30        # exponential backoff with full jitter
```

Changing `channel.server.path` requires updating two compile-time lists in lockstep: the public-path
allowlist (`publicPaths` in `internal/system/security/permissions.go`) and the Control Plane
access-log exclusion list (`accessLogExcludePaths` in `cmd/cpserver/main.go`). Without both, the
upgrade request is rejected by the JWT middleware or wrapped in a non-hijackable ResponseWriter, and
the channel silently fails. Treat the path as fixed unless you change all three together.

Because the configuration merge only overrides a default when the operator value is non-zero, the
numeric fields above cannot be set back to an explicit `0` from `deployment.yaml`.

## 7. Protocol

A minimal JSON-RPC 2.0 envelope, hand-rolled (no library dependency).

Request (CP to DP):

```json
{ "jsonrpc": "2.0", "id": "<uuid>", "method": "Import.Run", "params": { ... } }
```

Success response (DP to CP):

```json
{ "jsonrpc": "2.0", "id": "<uuid>", "result": { ... } }
```

Error response (DP to CP):

```json
{ "jsonrpc": "2.0", "id": "<uuid>", "error": { "code": -32601, "message": "Method not found" } }
```

v1 methods:

- `Import.Run` - params carry the fields of `importer.ImportRequest` (`content`, `options`);
  result carries `importer.ImportResponse`.
- `Agent.Ping` - no params; result is a small liveness payload (for example DP id and a timestamp).

The DP maintains a method whitelist. Any unknown method returns `MethodNotFound (-32601)`.

## 8. Data Flow

### Handshake

1. `dpserver` boots and the channel client dials `wss://<cp>/cp/connect` with an
   `Authorization: Bearer <token>` header and its DP id.
2. The `cpserver` handler is skipped by the JWT middleware (via `publicPaths`), so the channel
   verifies the token itself, then performs the WebSocket `Accept`.
3. The registry registers the connection keyed by DP id, evicting any existing socket for the same
   id (single-active-socket policy).
4. Persistent read/write pumps with heartbeat begin on both sides.

### Import RPC (CP to DP)

1. CP code calls `CallImport(ctx, dpID, req)`.
2. The server looks up the connection in the registry, builds a JSON-RPC request with a fresh id,
   registers a pending-response waiter keyed by that id, writes the frame, and blocks on the caller
   `ctx` or the configured `rpc_timeout`.
3. The DP reads the request, the Router dispatches `Import.Run`, decodes params into an
   `ImportRequest`, calls `ImportResources`, and writes back `{ id, result }`.
4. The CP reads the response, matches the id, delivers it to the waiter, and `CallImport` returns
   `(*ImportResponse, error)`.

### Ping (CP to DP)

`Ping(ctx, dpID)` sends an `Agent.Ping` request and awaits the reply. This confirms the DP's RPC
loop is alive, which is stronger than a transport-level pong. The registry additionally tracks
connection state and last-seen, updated on transport pong and on any received frame.

### Reconnect (DP)

The client runs a dial loop with exponential backoff and full jitter (from
`reconnect_initial_seconds` to `reconnect_max_seconds`), retrying indefinitely so the connection
stays up per goal 2. The backoff resets only after a connection that survived long enough to count as
healthy, so a Control Plane that accepts and then immediately drops does not pin every Data Plane at
the minimum interval.

## 9. Error Handling and Edge Cases

- Handshake auth failure: CP returns HTTP 401 on the upgrade; the client logs, backs off, and retries.
- Duplicate DP id: CP evicts the prior socket (single-active), logs the event. Eviction closes the
  superseded socket immediately rather than running a close handshake, so a zombie connection cannot
  delay the replacement.
- Oversized frame: the read limit is hit and the connection is closed with the message-too-big close
  code (1009). The waiting caller sees the connection-closed error, not a size-specific one.
- Unknown method: JSON-RPC `MethodNotFound (-32601)`. Undecodable params: `InvalidParams (-32602)`.
- Malformed JSON: the frame cannot be answered, because a frame that fails to decode has no usable
  request id to reply against. The WebSocket library closes the connection on an undecodable frame and
  both read loops treat that as terminal, so the DP reconnects. There is no per-frame `ParseError`
  response.
- Target DP offline: `CallImport` fails fast immediately (no buffering, per scope).
- RPC timeout: `CallImport` returns a timeout error; a late response for that id is discarded.
- Graceful shutdown: the CP closes all sockets; the DP stops the reconnect loop and sends a normal
  closure. Both hook into the existing server shutdown paths.
- In-flight work on a dropped connection: if the connection dies while the DP is running an import,
  the CP reports the call as failed even though the import may have been applied on the DP. Delivery is
  at-most-once from the CP's point of view; there is no idempotency key in v1, so an operator retry
  after an ambiguous failure can apply the same import twice.

## 10. Testing

- Unit:
  - rpc codec and id correlation.
  - registry: register, evict, lookup, last-seen updates.
  - token verifier: valid and invalid token.
  - backoff computation.
- Integration (in-process CP server plus DP client over a real loopback WebSocket, using
  `httptest` and a `coder/websocket` dial):
  - handshake, then a `CallImport` round-trip asserting the `ImportResponse`.
  - `Ping` round-trip.
  - unknown method returns `MethodNotFound`.
  - DP offline returns a fail-fast error.
  - forced close triggers auto-reconnect.
- Existing tests are untouched. The hybrid `cmd/server` path is unaffected because the channel is off.

## 11. Impacted Areas

- New: `internal/system/channel/**`.
- Modified: `internal/system/security/permissions.go` (add `/cp/connect` to `publicPaths`).
- Modified: `pkg/thunderidengine/config/config.go` (add the `channel` config section).
- Modified: `cmd/cpserver` wiring (start the WS server; expose `CallImport`/`Ping`).
- Modified: `cmd/dpserver` wiring (start the WS client; register `Import.Run` and `Agent.Ping`).
- Dependency: add `github.com/coder/websocket` to `go.mod`.

## 12. Forward Compatibility

- mTLS: the handshake `auth.Verifier` interface allows an mTLS implementation to slot in without
  changing the channel core.
- Additional methods: the JSON-RPC Router and `methods/` package accept new commands (for example a
  future policy sync) without transport changes.
- Scaling: the in-memory registry can later be fronted by a shared store (for example Redis pub/sub)
  to route across multiple CP instances, without changing the CP-side programmatic API.
