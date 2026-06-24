# Integrating the new `thunderidengine` into the esignet embedder — approach & plan

> Status: **proposed** — for review before implementation.
> Target embedder: `github.com/mosip/esignet` (`/opt/mosip/ecopy/idp/esignet-service`).
> Target engine: `github.com/thunder-id/thunderid/pkg/thunderidengine` at commit `89bb3c925`
> ("Separate public thunderidengine added").

## 1. Where we are

The new engine is the **low-level core**: `New(...Option)` + `RegisterRoutes(mux)`. It mounts
`GET /flow/meta`, `POST /flow/execute`, and `/oauth2/*`, persists runtime state in a caller-supplied
Redis connection, and requires the embedder to inject eleven providers
(`validateRequiredProviders` in `engine.go`):

```
ActorProvider, AuthnProvider, OUService, AttributeCacheService, AuthZService,
ResourceService, I18nService, IDPService, FlowProvider, ExecutorRegistry, DesignResolveService
```

It currently ships declarative-file defaults for only two of them (`FlowProvider`, `I18nService`,
in `defaults.go`).

esignet was written against an **older, higher-level facade** that no longer exists in the engine:
it imports `pkg/thunderidengine/host`, `pkg/thunderidengine/runtime`, `pkg/thunderidengine/flow`,
and calls `thunderidengine.Initialize(mux, EngineConfig{...})`. None of those packages/symbols are
present at `89bb3c925`.

Two hard constraints discovered while scoping (both confirmed in code):

1. **The required interfaces reference thunder-internal DTO types that the engine does not
   re-export** (`entityprovider.Entity`, `ou.OrganizationUnit`, `resource.ResourceServer`,
   `idp.IDPDTO`, …). Because `github.com/mosip/esignet` is a separate module, Go forbids it from
   importing `internal/*`, so it cannot even name those types to satisfy the interfaces.
2. **`authnprovidermgr.AuthUser` has only unexported fields and no exported constructor.** It is
   passed/returned by every `AuthnProviderManagerInterface` method. An external module cannot
   produce a valid value. (It *can* be built via `json.Unmarshal` because its `UnmarshalJSON`
   maps a JSON proxy onto the private fields, and that proxy's types —
   `authnprovidercm.EntityReference`, `authnprovidercm.AttributesResponse` — are exported. This
   only helps code **inside** the thunder module.)

Conclusion: the composition must happen **inside the thunder module**, where code may import
`internal/*` and construct these types. This plan does that with the smallest, clearly-bounded set
of additions to `pkg/thunderidengine`.

## 2. Guiding principle (the reframe)

**SDK-first.** The engine is designed as a reusable, embeddable SDK for *any* external Go
application — not adapted to match what esignet happens to have today. We define a clean, stable,
DTO-free public surface in `pkg/thunderidengine`; embedders (esignet included) conform their code to
that surface. Where esignet's current providers don't match the SDK, esignet changes — not the SDK.

**File-based (declarative) services are built inside the engine; they are never exposed or
re-exported.** Only things that are genuinely embedder-specific cross the module boundary:

- infrastructure the embedder owns (Redis connection, signing keys),
- configuration (issuer, token lifetimes, declarative resource paths, executor allowlist),
- the embedder's own identity source and custom authentication (its catalog/directory-backed
  actor/authn, and any external integrations such as MOSIP/Sunbird).

## 3. Classification of the eleven required providers

### 3a. Built **inside** the engine from declarative files (no embedder code, no re-export)

Reference: `backend/cmd/server/servicemanager.go` `registerServices` (lines ~145–384) shows the
exact construction order and dependencies. All of these support declarative-resource loading
(they appear in the `exporters` list) and esignet already ships their YAML under
`data/repository/resources/`.

| Provider | Internal constructor (from `servicemanager.go`) | Declarative source dir |
|---|---|---|
| `flowProvider` | `flowmgt.Initialize` (already `buildDefaultFlowProvider`) | `flows/` |
| `i18nService` | `i18nmgt.Initialize` (already `buildDefaultI18nService`) | `i18n/` |
| `ouService` | `ou.Initialize(mux, mcp, cache, sysauthz)` | `organization_units/` |
| `resourceService` | `resource.Initialize(mux, ou, consent)` | `resource_servers/` |
| `idpService` | `idp.Initialize(cache, mux, entitytype)` | `identity_providers/` |
| `designResolveService` | `resolve.Initialize(mux, theme, layout, application)` | `themes/`, `layouts/` |
| `authZService` | `authz.Initialize(role)` | `roles/` |
| `attributeCacheService` | `attributecache.Initialize(config)` | (Redis + config, not a file) |
| `executorRegistry` | `executor.Initialize(deps, flowCfg)` | built-in executors |

These pull in a **supporting sub-graph** that is also built internally and never exposed:
`sysauthz`, `cache`, `consent`, `entitytype`, `entity`, `entityprovider`, `user`, `group`, `role`,
`theme/mgt`, `layout/mgt`, `application`, `notification`, `template`. This is essentially the middle
of `registerServices`, lifted into the engine.

**Route isolation:** each internal `Initialize` that takes a `*http.ServeMux` is given a
**throwaway mux** (`http.NewServeMux()`), exactly like the existing `buildDefaultFlowProvider` /
`buildDefaultI18nService` do, so their management REST routes are never mounted on the embedder's
mux. Only `/flow/meta`, `/flow/execute`, `/oauth2/*` reach the embedder.

### 3b. Supplied by the embedder (esignet) via **simplified host interfaces**

`actorProvider` and `authnProvider` are the embedder's identity source and so are supplied by the
embedding application — but, per constraints §1.1/§1.2, an external module cannot implement the raw
internal interfaces. So the SDK defines **clean, DTO-free host interfaces** as part of its public
surface and adapts them internally to the real interfaces. These interfaces are designed for general
embedders, not reverse-engineered from esignet.

- The SDK defines `host.ActorProvider` and `host.AuthnProvider` as its stable public contract for
  pluggable identity. Any embedder implements them; esignet conforms its providers
  (`internal/host/actors.go`, `catalog_authn.go`, `mosip_authn.go`, `sunbird_authn.go`) to these
  signatures (adjusting them where they differ from the SDK).
- The engine provides adapter types (inside `pkg/thunderidengine`, allowed to import `internal/*`)
  that implement `actorprovider.ActorProviderInterface` and
  `authnprovidermgr.AuthnProviderManagerInterface` by delegating to the host interfaces and
  constructing the internal DTOs (`Entity`, `OAuthClient`, `InboundClient`) and `AuthUser`
  (via the JSON-proxy technique) themselves.
- New options: `WithHostActorProvider(host.ActorProvider)`,
  `WithHostAuthnProvider(host.AuthnProvider)`. The existing raw `WithActorProvider` /
  `WithAuthnProvider` remain for in-tree callers.

### 3c. Supplied by the embedder as **infrastructure / config** (already in the engine API)

- `WithRedis(client, keyPrefix)` — runtime state.
- `WithPKIKey(id, cert, key)` / `WithRuntimeCrypto(...)` — signing material.
- `WithConfig(serverHome, *Config)` — issuer, token lifetimes, declarative paths, executor list,
  cache, translation.
- `WithExecutorDependencies(ExecutorDependencies)` + `WithEnabledExecutors(...)` — **custom
  executors** (MOSIP OTP, Sunbird) are added here. The engine fills the built-in executor deps it
  already holds; esignet supplies only the custom extras. (Chosen over a post-build
  `RegisterCustom` callback.)
- `WithObservability(...)` / `WithConsentEnforcer(...)` — optional.

## 4. Engine changes (in `pkg/thunderidengine`)

These additions define the public SDK surface. They are designed to be general-purpose and stable
for any external embedder; esignet then conforms to them (§5).

1. **`host/host.go`** (new package) — clean, DTO-free SDK interfaces + structs for pluggable
   identity. Designed as the engine's public contract, not copied from esignet:
   - `ActorProvider`: resolve entities/clients (e.g. `IdentifyEntity`, `GetEntity`,
     `SearchEntities`, `GetApplication`, `GetInboundClientByEntityID`,
     `GetInboundClientByClientID`, `GetEntityType`). Finalize the method set as a coherent SDK
     contract rather than esignet's current shape.
   - `AuthnProvider`: authenticate and fetch attributes (e.g. `Authenticate`, `GetAttributes`).
   - DTOs: `Actor`, `Application`, `InboundClient`, `Certificate`, `EntityType`, `AuthnResult`,
     `AuthnMetadata`, `RequestedAttributes`, `GetAttributesMetadata`, `GetAttributesResult` —
     plain SDK structs with no `internal/*` types.
   - (Optionally `AuthorizationProvider` / `ConsentEnforcer` if exposed as pluggable SDK points.)
2. **`runtime/runtime.go`** (new package) — public SDK error sentinel `ErrNotFound` (and any other
   error/contract types host implementations must return). Part of the SDK surface, independent of
   any embedder. (The engine's own runtime stores use `*redis.Client` directly.)
3. **`hostadapter.go`** (package `thunderidengine`) — adapters:
   - `actorAdapter` → `actorprovider.ActorProviderInterface` (maps host `Actor`/`InboundClient` to
     `entityprovider.Entity`, `inboundmodel.OAuthClient`, `inboundmodel.InboundClient`; unsupported
     write methods return a descriptive `EntityProviderError`).
   - `authnAdapter` → `authnprovidermgr.AuthnProviderManagerInterface` (builds `AuthUser` from the
     host `AuthnResult` via the `authUserJSON` proxy shape; `GetUserAttributes` maps the host
     `GetAttributes`).
4. **`defaults_services.go`** (package `thunderidengine`) — `buildDeclarativeServices(...)` that
   constructs the §3a sub-graph on a throwaway mux and returns the providers. Modeled directly on
   `registerServices`.
5. **`options.go`** — add `WithHostActorProvider`, `WithHostAuthnProvider`,
   `WithConsentEnforcer` (if needed).
6. **`engine.go` `New`** — for each required provider not explicitly injected, build it: from the
   host adapter (actor/authn) or from `buildDeclarativeServices` (the rest), gated on
   `declarativeresource.IsDeclarativeModeEnabled()`. `validateRequiredProviders` then passes.

No DTO re-exports. No `internal/*` leaks to esignet.

## 5. esignet changes (esignet conforms to the SDK)

esignet is treated as the first consumer of the SDK and adapts to it; the SDK is not bent to fit
esignet.

- **`go.mod`** — bump the `replace github.com/thunder-id/thunderid => github.com/anushasunkada/thunder/backend`
  pseudo-version to the commit that contains these engine additions. (Per decision: keep the remote
  fork, update the commit.)
- **`internal/host/*`** — conform the provider implementations to the finalized SDK `host`
  interfaces from §4.1. Where esignet's current method signatures differ from the SDK contract,
  esignet changes (rename/adjust methods, map its catalog types onto the SDK structs).
- **`internal/host/executors.go` + `mosip_otp_executor.go`** — keep custom executors; register them
  through `WithExecutorDependencies` (the engine passes its built flow factory + built-in deps),
  conforming to the SDK's executor-dependency types.
- **`cmd/esignet/main.go`** — replace `thunderidengine.Initialize(mux, cfg)` with:
  ```go
  eng, err := thunderidengine.New(
      thunderidengine.WithConfig(serverHome, engineCfg.Config()),
      thunderidengine.WithRedis(redisClient, redisCfg.KeyPrefix),
      thunderidengine.WithPKIKey("default", certFile, keyFile),
      thunderidengine.WithHostActorProvider(embedhost.NewActorProvider(cat)),
      thunderidengine.WithHostAuthnProvider(authnProvider),
      thunderidengine.WithExecutorDependencies(customExecDeps),
      thunderidengine.WithEnabledExecutors(engineCfg.Executors()...),
  )
  if err != nil { logger.Fatal(...) }
  if err := eng.RegisterRoutes(mux); err != nil { logger.Fatal(...) }
  ```
- **`internal/config/engine.go`** — replace `ThunderEngineConfig() thunderidengine.EngineConfig`
  with a builder that returns `*thunderidengine.Config` (alias of internal `config.Config`) plus the
  executor allowlist. Map issuer/audience/lifetimes/gate-client/declarative-paths into the runtime
  `Config`.
- **`internal/store/redis_store.go`** — the engine now owns its Redis runtime stores via
  `WithRedis`, so this custom `runtime.Store` implementation is **no longer needed** for wiring.
  Keep it only if esignet wants its own namespacing; otherwise drop it.

## 6. Build & verify (requires Go 1.26)

This sandbox has no Go toolchain and the Go download hosts are blocked, so the steps below must be
run in a Go-equipped environment.

```bash
# Engine
cd /opt/mosip/github/thunder/backend
go build ./pkg/thunderidengine/...
go vet  ./pkg/thunderidengine/...
go test ./pkg/thunderidengine/...

# Embedder (against the local engine during development)
cd /opt/mosip/ecopy/idp/esignet-service
# temporary local replace for verification only:
#   go mod edit -replace github.com/thunder-id/thunderid=/opt/mosip/github/thunder/backend
go build ./...
go vet  ./...
# then restore the remote-fork replace for commit.
```

Runtime smoke test: start with Redis + Postgres + the declarative `data/` dir, hit
`GET /flow/meta`, run `POST /flow/execute` through the MOSIP OTP flow, and exercise `/oauth2/token`.

## 7. Open items to confirm

- **Declarative mode + DB:** the §3a services run their declarative loaders but several still
  initialize the DB provider singleton. esignet already opens Postgres in `main.go`, so this is
  fine; confirm declarative reads don't require seeded tables.
- **`attributeCacheService`** uses Redis via `attributecacheconfig.FromServerRuntime()`; confirm its
  Redis config is seeded by `WithConfig`.
- **Crypto:** confirm esignet's signing key path maps cleanly to `WithPKIKey` (PEM cert+key) vs the
  old `SystemConfig.SigningKeyPath`.
- **Single-engine-per-process:** the engine seeds the `config.GetServerRuntime()` singleton from
  `WithConfig`; one engine per process (already documented in the engine).
