# `pkg/thunderidengine` — Import-Graph-Clean Embeddable Engine: Implementation Plan

## 1. Goal & constraints

Ship a public Go package `github.com/thunder-id/thunderid/pkg/thunderidengine` that an external Go application can import to mount three endpoint groups on its own `http.ServeMux`:

- `GET /flow/meta`
- `POST /flow/execute` (+ `OPTIONS`)
- `/oauth2/*` (authorize, token, introspect, userinfo, JWKS, discovery, PAR, CIBA) — DCR is **out of scope for v1** (see §7)

Hard constraints (from requirements):

1. Core and default implementations stay in `backend/internal`. Only interfaces and composition/`Initialize` logic may move to `pkg/thunderidengine`.
2. **No database dependency** reaches the embedding application — neither at runtime nor in the import graph / `go.mod`. Redis is allowed, and the embedder passes the Redis connection config.
3. The engine can initialize the executor registry with external custom executors and optionally select which built-in executors to register.
4. The embedder passes configuration; wherever possible the engine falls back to declarative resources.
5. These dependencies are overridable by the embedder: `actorProvider`, `flowProvider`, `authnProvider`, `i18nService`, `designResolveService`, `ouService`, `observabilitySvc`, `runtimeCryptoProvider`, `authZService`.

**Definition of "import-graph-clean" (the acceptance test):**

```
go list -deps ./backend/pkg/thunderidengine | grep -E 'lib/pq|modernc/sqlite|database/sql'
```

must return nothing. This is enforced in CI (Phase 6).

## 2. Why import-clean (the contamination root)

`backend/internal/system/database/provider` is a single package whose `dbclient.go` blank-imports the SQL drivers:

```go
import (
    _ "github.com/lib/pq"
    _ "modernc.org/sqlite"
)
```

`redisprovider.go` lives in the **same package**, so importing it for Redis-only use still compiles `dbclient.go` and links both drivers. Every runtime store today imports this package on **both** the SQL and Redis branches:

- `flow/flowexec`: `init.go`, `store.go` (SQL), `redis_store.go`
- `oauth/oauth2/jti`: `store.go` (SQL), `redis_store.go`
- `oauth/oauth2/authz`: `auth_code_store.go` + `auth_code_redis_store.go`, `auth_req_store.go` + `auth_req_redis_store.go`
- `oauth/oauth2/ciba`: `store.go` + `store_redis.go`
- `oauth/oauth2/par`: `store.go` + `redis_store.go`

(`oauth/oauth2/dcr` also uses `GetRuntimeDBTransactioner()` directly, but DCR is out of scope for v1 — see §7.)

Consequence of *not* fixing this: `lib/pq` + `modernc/sqlite` (+ the large `modernc/libc` subtree) land in the embedder's `go.mod`, and — the real bug — if the embedder also imports either driver, `database/sql` panics at `init()` with `sql: Register called twice for driver pq`, which cannot be recovered. Import-clean removes the footgun.

## 3. Conceptual model

The engine owns three things only:

1. **Protocol + orchestration logic** — flow execution engine, flow metadata, OAuth2/OIDC server.
2. **Runtime state** — short-lived state (flow execution state, auth codes, JTI replay cache, CIBA/PAR requests) persisted in **Redis only**.
3. **A composition root** — the equivalent of today's `servicemanager.go`, minus DB init, exposed as `New(...Option)`.

Everything that is a **system of record** (users, OUs, roles, registered clients, flow definitions-as-config, themes/layouts, translations) is **injected** by the embedder via the nine override points. The engine ships *optional* declarative-file-backed defaults (reading `bootstrap/flows`, `i18n`, `themes`, and `resources/`) for the providers where a file-backed default is feasible. Where no clean default exists, the provider is a **required** injection.

### Classification of the nine overridable dependencies

| Dependency | Default impl today | Import-clean default? | Plan |
|---|---|---|---|
| `observabilitySvc` | `observability.Initialize()` | ✅ clean | Default provided |
| `runtimeCryptoProvider` | `kmprovider.Initialize(pki)` | ✅ clean (file/PKI) | Default provided |
| `flowProvider` | `flowmgt` (DB) | ⚠️ via declarative | Default = declarative reader over `bootstrap/flows`; else inject |
| `actorProvider` | wraps `inboundclient`(DB)+`entityprovider`(DB) | ❌ DB-backed | **Required injection** (or declarative client reader) |
| `ouService` | `ou.Initialize` (DB) | ❌ DB-backed | **Required injection** (or declarative OU reader) |
| `authZService` | `authz.Initialize(role)` → `role`(DB) | ❌ DB-backed | **Required injection** (or declarative role reader) |
| `i18nService` | `i18nmgt` (DB) | ⚠️ via declarative | Default = declarative reader over `i18n/`; else inject |
| `designResolveService` | `resolve.Initialize` → theme/layout mgt (DB) | ⚠️ via declarative | Default = declarative reader over `themes/`; else inject |
| `authnProvider` | `authnprovidermgr` over entity/passkey/otp/... (DB) | ❌ DB-backed | **Required injection** |

The interfaces for all nine already exist and are already the parameters to `flowexec.Initialize` / `flowmeta.Initialize` / `oauth.Initialize`. No new abstractions are needed for the injection points themselves — only re-export and default construction.

## 4. Target package layout

```text
backend/pkg/thunderidengine/
  engine.go            # Engine type; New(...Option) (*Engine, error); RegisterRoutes(mux); Handler()
  options.go           # functional options + defaulting logic
  config.go            # public Config (Redis conn, declarative resource paths, feature toggles, DeploymentID)
  providers.go         # public interface aliases re-exporting internal provider interfaces
  executors.go         # BuiltinExecutors(...names) / CustomExecutors(...) builders
  defaults.go          # declarative-backed default constructors (flow, i18n, design)
  doc.go               # package docs + the import-clean contract

backend/internal/system/database/
  redisstore/          # NEW: redisprovider.go moved here; no SQL imports
  sqlstore/            # NEW: dbclient.go, dbprovider.go, retry.go (the driver-importing files)
  dbtypes/             # NEW: neutral constants (DataSourceTypeRedis/SQLite/Postgres), shared types

backend/internal/flow/flowexec/
  store.go             # KEEPS only FlowStoreInterface (no impl, no provider import)
  redisstore/          # NEW: newRedisFlowStore  (imports redisstore only)
  sqlstore/            # NEW: newFlowStore       (imports sqlstore only)

backend/internal/oauth/oauth2/{jti,authz,ciba,par}/
  interface.go         # store interface only
  redisstore/          # NEW: Redis impls
  sqlstore/            # NEW: SQL impls
```

External apps cannot import `internal/*`; `pkg/thunderidengine` can, because it shares the module root. The `pkg` package is the single public façade.

## 5. Public API surface (target)

```go
package thunderidengine

type Engine struct { /* unexported */ }

func New(opts ...Option) (*Engine, error)

func (e *Engine) RegisterRoutes(mux *http.ServeMux)   // primary
func (e *Engine) Handler() http.Handler               // convenience wrapper
func (e *Engine) Shutdown(ctx context.Context) error  // closes Redis, observability

// --- options ---
func WithConfig(cfg Config) Option
func WithRedis(client *redis.Client, keyPrefix string) Option   // embedder-supplied connection

// nine override points; omit => declarative default OR error if no clean default
func WithActorProvider(ActorProvider) Option
func WithFlowProvider(FlowProvider) Option
func WithAuthnProvider(AuthnProvider) Option
func WithI18nService(I18nService) Option
func WithDesignResolveService(DesignResolveService) Option
func WithOUService(OUService) Option
func WithObservability(ObservabilityService) Option
func WithRuntimeCrypto(RuntimeCryptoProvider) Option
func WithAuthZService(AuthZService) Option

// executor registry control
func WithExecutors(specs ...ExecutorSpec) Option
func BuiltinExecutors(names ...string) ExecutorSpec   // opt-in allowlist
func CustomExecutors(execs ...core.ExecutorInterface) ExecutorSpec
```

`providers.go` aliases the existing internal interfaces so embedders never import `internal`:

```go
type ActorProvider        = actorprovider.ActorProviderInterface
type FlowProvider          = flowexec.FlowProviderInterface
type AuthnProvider         = authncommon.AuthnProviderInterface
// ... etc.
```

(If any aliased interface transitively references a DB type in its method signatures, that type is also re-exported or replaced with a DTO — verified in Phase 4.)

## 6. Phased work plan

### Phase 0 — Guardrails & baseline (no behavior change)
- Add the CI import-clean check script (initially allowed to fail / informational) so progress is measurable from day one.
- Snapshot current `go list -deps` for the three target packages to know the starting contamination set.
- Confirm `make lint` / `make test` green baseline.

### Phase 1 — Split the database provider package
- Move `redisprovider.go` → `internal/system/database/redisstore` (package `redisstore`). It already imports no SQL — only `redis`, `config`, `log`, `transaction`. Verify `transaction` does not pull a driver (it imports `database/sql` from stdlib only — acceptable, but confirm the engine path needs the Redis transactioner, which is a no-op type).
- Move `dbclient.go`, `dbprovider.go`, `retry.go` → `internal/system/database/sqlstore` (package `sqlstore`).
- Move `DataSourceTypeRedis`/`Postgres`/`SQLite` constants + shared types → `internal/system/database/dbtypes` (no driver imports), so callers can branch on store type without importing either store package.
- Update all references repo-wide (`provider.GetRedisProvider` → `redisstore.GetProvider`, etc.). Mechanical; covered by compiler + tests.
- **Risk check:** `database/sql` in `transaction` and the Redis no-op transactioner. `database/sql` (stdlib) is fine — the CI grep targets the *drivers*, not stdlib. Keep the grep to `lib/pq|modernc/sqlite`. If we also want to forbid `database/sql`, the Redis transactioner must not reference `*sql.DB`; confirm and, if needed, introduce a `transaction.Transactioner` interface with a Redis-only no-op impl in `redisstore`.

### Phase 2 — Extract store implementations behind interfaces
For each of the six runtime stores, keep the interface in the domain package and move SQL/Redis impls into sibling sub-packages so importing the domain core no longer compiles the SQL impl:

1. `flowexec`: `FlowStoreInterface` stays; `newFlowStore`→`flowexec/sqlstore`, `newRedisFlowStore`→`flowexec/redisstore`.
2. `jti`: interface stays; SQL/Redis impls split.
3. `authz`: **two** stores — auth-code and auth-request — each split (SQL + Redis).
4. `ciba`: split `store.go`/`store_redis.go`.
5. `par`: split `store.go`/`redis_store.go`.

Replace the in-package `if cfg.RuntimeDBType == redis { ... } else { ... }` selection with **store injection**: each domain `Initialize` accepts the already-constructed store interface instead of choosing internally. The chooser logic moves up to the composition root (`servicemanager.go` for the standalone server, `engine.New` for the embeddable engine). This is what lets the engine import only the `redisstore` sub-packages.

### Phase 3 — De-singletonize the Redis connection
- Add `redisstore.New(client *redis.Client, keyPrefix string) Provider` so the engine injects the embedder's `*redis.Client` instead of the global `GetProvider()` (which reads `config.GetServerRuntime()`).
- Keep the singleton path for the standalone server (it can call `redisstore.New` from its own config too — preferred, to converge both paths).
- Decision to record: the broader `config.GetServerRuntime()` singleton remains for v1 (engine seeds it from `WithConfig`). Full config threading is out of scope; note the single-engine-per-process limitation in `doc.go`.

### Phase 4 — Build `pkg/thunderidengine`
- Implement `New` as a DB-free analogue of `registerServices`: construct Redis stores (Phase 2/3), build observability + runtime crypto defaults, then build each injectable provider — using the supplied override if present, else the declarative default (Phase 5), else return a descriptive error for the required-injection providers (`actorProvider`, `authnProvider`, `ouService`, `authZService`).
- Wire `flowexec.Initialize`, `flowmeta.Initialize`, and `oauth.Initialize` onto the supplied mux. Replace `log.Fatal` with returned errors. (DCR is not wired in v1 — see §7.)
- Implement `providers.go` aliases; verify no aliased interface signature leaks a DB type (else introduce DTOs).
- Implement executor options over the existing `executor.Initialize(deps, cfg)` (which already supports config-driven builtin selection): add a custom-executor inject and an explicit builtin allowlist.

### Phase 5 — Declarative-resource-backed default providers
- Implement read-only, file-backed defaults for `flowProvider` (over `bootstrap/flows`), `i18nService` (over `i18n/`), and `designResolveService` (over `themes/`), reusing `internal/system/declarative_resource/loader.go`.
- These defaults must themselves be import-clean (no DB). Verify with the Phase 0 grep extended to the default constructors.
- Document that `actorProvider`, `authnProvider`, `ouService`, `authZService` have no clean default and must be injected.

### Phase 6 — Dogfood: refactor the standalone server
- Rewrite `backend/cmd/server/servicemanager.go` to consume `thunderidengine` for the three endpoint groups (passing the DB-backed providers + SQL stores as the "overrides"), proving the seam works and preventing drift between the two composition roots.
- Flip the CI import-clean check to **blocking**.

### Phase 7 — Verification
- `go list -deps ./backend/pkg/thunderidengine | grep -E 'lib/pq|modernc/sqlite'` → empty (CI gate).
- Unit tests for each split store package (move existing `*_test.go` alongside impls).
- A minimal **sample embedder app** under `samples/` that imports only `pkg/thunderidengine` + a Redis client, injects stub providers, and serves the three endpoints — its `go.mod` is asserted to contain no SQL driver.
- `make lint` + `make test` green; OAuth conformance / flow e2e suites pass against the engine-wired server.

## 7. Resolved decisions

1. **DCR — excluded, permanently (decided).** `dcr.Initialize` uses `GetRuntimeDBTransactioner()` directly and depends on `applicationService` (DB-backed, system-of-record for clients); dynamic *client registration* inherently writes to a client store, which is fundamentally at odds with the DB-free engine boundary. It is **not** part of the engine and is not planned for any future engine version. Embedders needing DCR run the full standalone server.
2. **Interface signatures leaking DB types (resolved — handled in Phase 2/4).** The risk is not a literal `*sql.Tx` in a signature; it is a re-exported interface whose signature references a type whose *home package* is DB-contaminated. Confirmed example: `FlowProviderInterface.GetFlowByHandle` returns `*flowmgt.CompleteFlowDefinition`, defined in `flow/mgt/model.go` — the same package as `flow/mgt/store.go`, which imports the SQL provider. An embedder implementing `FlowProvider` must import `flowmgt` to build the return value, dragging the drivers back in. The Phase 2 store extraction fixes this (moving `flow/mgt/store.go` → `flow/mgt/sqlstore` makes the `flowmgt` package driver-free). Phase 4 audit: for each of the nine interfaces, confirm every type appearing in its signatures lives in a now-clean package; if any does not, move that DTO into a leaf `model`/`types` package. (Internal store interfaces like `FlowStoreInterface`/`FlowContextDB` are not re-exported and need no treatment.)
3. **`transaction` package & `database/sql` (resolved).** `transaction` imports stdlib `database/sql`, but that is **not** a driver — it links no `lib/pq`/`modernc/sqlite` and adds nothing to the embedder's `go.mod`. The Redis path uses the package's existing `NewNoOpTransactioner()` (the Redis provider's `GetTransactioner()` returns it), so the engine never constructs a `*sql.DB`; the `Transactioner` interface itself (`Transact(ctx, func) error`) is clean. **Decision:** the CI gate forbids the drivers only (`lib/pq`, `modernc/sqlite`), not stdlib `database/sql`. Splitting the SQL transactioner impl into a sub-package to remove `database/sql` from the graph is optional cosmetic cleanup, not required.
4. **Config singleton (decided — accepted).** v1 keeps `config.GetServerRuntime()` global, seeded by the engine from `WithConfig` → one engine instance per process. Multi-instance embedding is **not** a v1 requirement; the limitation is documented in `doc.go`.
5. **Naming/placement (decided).** The engine lives at `backend/pkg/thunderidengine`, import path `github.com/thunder-id/thunderid/backend/pkg/thunderidengine`.

## 8. Suggested sequencing / parallelism

Phases 1 → 2 → 3 are sequential (each depends on the previous). Phase 5 (declarative defaults) can proceed in parallel with Phase 2/3 once interfaces are fixed. Phase 4 depends on 2+3. Phases 6 and 7 are last. Phase 0 guardrails go in immediately so every PR shows movement of the contamination set toward empty.

## 9. Phase 2 outcome & reassessment (added after implementation)

### What was completed

Phase 1 (split `redisstore`/`dbtypes` out of the driver-importing `provider` package) and Phase 2 (extract the **runtime** stores behind injected interfaces) are implemented and pass `make build_backend` / `make test` / `make lint`:

- `redisstore` + `dbtypes` packages created; `provider` keeps the SQL drivers.
- Runtime stores split into `*/sqlstore` sub-packages with the Redis impls kept in-package and the SQL-vs-Redis selection lifted to the composition root (`servicemanager.buildOAuthRuntimeStores` / `buildFlowRuntimeStore`): **jti, ciba, par, flowexec, authz** (auth code + auth request).
- `oauth.RuntimeStores` bundle injects them into `oauth.Initialize`; `flowexec.Initialize` receives its store + transactioner.
- Two internal types were exported to allow the impls to move out (`par.PushedAuthorizationRequest`, `authz.AuthRequestContext`) plus `authz.ErrAuthorizationCodeNotFound`.

### The finding: runtime stores were necessary but not sufficient

Verifying the goal exposed a second, larger contamination layer:

```
go list -deps ./internal/oauth ./internal/flow/flowexec ./internal/flow/flowmeta \
  | grep -E 'lib/pq|modernc.org/sqlite'     # STILL non-empty
```

The engine's three entry packages still transitively import the SQL drivers — **not** through any runtime store, but because they import **system-of-record service packages for their injected interface *types*,** and those packages each contain a DB store that blank-imports the drivers. Confirmed dirty dependencies:

- `oauth` → `resource`, `ou`, `idp`, `attributecache`, `system/i18n/mgt` (imported for `ResourceServiceInterface`, `OrganizationUnitServiceInterface`, `IDPServiceInterface`, `AttributeCacheServiceInterface`, `I18nServiceInterface`).
- `flowmeta` → `ou`, `system/i18n/mgt`.
- `flowexec` → **`flowmgt`**: `flowexec/interface.go` declares `FlowProviderInterface.GetFlowByHandle` returning `*flowmgt.CompleteFlowDefinition`. Importing `flowexec` for that interface pulls `flowmgt`, whose `store.go` imports `provider`. So `flowexec` is **not** actually clean despite its own store being extracted — this is the §2 DTO-home-package leak, occurring in the engine's own package.

(`actorprovider`, `authnprovider/manager`, and the authorization `authz` service are already driver-free.)

### Why "just extend Phase 2" does not work

Applying the store-extraction pattern to `resource`/`ou`/`idp`/`attributecache`/`i18n`/`flowmgt` does not terminate: `resource` imports `ou` + `consent`; `ou`/`idp` import `entity`/`entitytype`/`role`; etc. Roughly 17 internal domains import `provider`. Cleaning the transitive closure means de-driver-ing most of the backend — not a bounded change, and not the intent.

### Revised strategy — contract packages (interface + DTO relocation)

The engine must depend only on **interfaces declared in driver-free packages**, never on the concrete DB-backed service packages. For each injected dependency the engine entry packages require:

1. Declare the interface (and every DTO type in its method signatures) in a driver-free leaf package — either a new `<domain>/contract` package, or a shared `pkg/thunderidengine/contracts` package the engine owns.
2. Change `oauth.Initialize` / `flowexec.Initialize` / `flowmeta.Initialize` parameters to use the contract interfaces.
3. The concrete service packages implement the contract (structurally, or via a thin adapter). The **full-server composition root** is the only place that imports the concrete packages and passes them in — it is allowed to link the drivers.

Bounded set to relocate (the engine's actual injected surface): `ResourceServiceInterface`, `OrganizationUnitServiceInterface`, `IDPServiceInterface`, `AttributeCacheServiceInterface`, `I18nServiceInterface`, `DesignResolveServiceInterface`, `FlowProviderInterface` (+ `CompleteFlowDefinition` and any graph/model DTOs it exposes), and an audit of `authnProvider`. Each interface's referenced DTOs must travel with it into the clean package.

### Decision: Option B — engine-owned minimal contracts (chosen)

**Decided.** The engine declares the minimal interfaces it actually calls in an engine-owned, driver-free `contracts` package; the engine entry packages (`oauth`, `flowexec`, `flowmeta`) take those contract types as parameters. The concrete DB-backed services are wrapped in thin adapters at the **full-server composition root** (`servicemanager`), which is the only place allowed to import the heavy domain packages and link the drivers. Domain packages stay essentially untouched. The `FlowProvider`/`CompleteFlowDefinition` case is handled by relocating the flow-definition DTOs to a driver-free leaf package (the engine's `FlowProviderInterface` is already engine-owned; only its return DTO leaks), since duplicating the full node/graph model into a contract would be worse than moving it.

(Rejected: Option A — per-domain `contract` packages — because moving full fat interfaces plus all their DTOs across `resource`/`ou`/`idp`/`i18n`/etc. spreads the churn across many domain packages and their callers, the exact cascade this refactor avoids.)

### Original framing of the decision (for reference)

The key choice was: **where do the contract interfaces live, and do we relocate the existing interfaces or define fresh engine-owned ones?**

- **Option A — per-domain `contract` sub-packages.** Move each interface + its DTOs into `<domain>/contract`; the domain service package imports its own contract. Keeps interfaces near their domain; touches many packages' interface declarations and DTO definitions.
- **Option B — engine-owned `pkg/thunderidengine/contracts`.** The engine defines the minimal interfaces it needs; concrete services are adapted at the composition root. Fewest changes to existing domain packages, but the engine restates method sets and DTOs (or imports DTOs from clean leaf packages).

Either way the DTO types in the signatures (e.g. `CompleteFlowDefinition`, resource/ou/idp models) must end up in driver-free packages. This is the substance of the remaining Phase 4 work and should be settled explicitly before implementation.

### Phase 2.5 execution plan (Option B, concrete)

Refinements learned while scoping:

- **The engine references some domain DTOs directly, not only via interfaces** (e.g. `granthandlers`/`userinfo` use `attributecache.AttributeCache`; `flowexec.FlowProviderInterface` returns `*flowmgt.CompleteFlowDefinition`). Those DTOs must move to driver-free leaf packages regardless of A/B — that relocation is the unavoidable core; the contracts/adapters sit on top.
- **Alias-method constraint:** a type alias cannot carry methods defined in another package. Any DTO with methods (e.g. `NodeDefinition.MarshalYAML/UnmarshalYAML`) must move *together with its methods* into the leaf package; the origin package then aliases the type back (`type X = leaf.X`) so its own code and other callers stay unchanged.

Ordered slices (each independently build/test/lint-verifiable):

1. **flowdef (FlowProvider).** New leaf `internal/flow/flowdef` holds the flow-definition model graph (`CompleteFlowDefinition`, `NodeDefinition` + its YAML marshaling + `nodeDefinitionAlias`, `NodeLayout/Size/Position`, `PromptDefinition`, `InputDefinition`, `ValidationRuleDefinition`, `ActionDefinition`, `ExecutorDefinition`, `ConditionDefinition`). `flowmgt` aliases them back (so `flowmgt`, `importer`, and flowmgt's graph/inference code are unchanged). `flowexec/interface.go` references `flowdef.CompleteFlowDefinition` and drops the `flowmgt` import → `flowexec` becomes driver-free. No contract/adapter needed (the interface is already engine-owned).
2. **engine `contracts` package** (`backend/pkg/thunderidengine/contracts`, or interim `internal/oauth/contracts`) — define the minimal interfaces the engine calls, plus any small DTOs not already in clean leaf packages.
3. **attributecache** — relocate `AttributeCache` DTO to a clean leaf; define `contracts.AttributeCacheService` (the ~4 methods the engine uses); adapter at root.
4. **i18n** — engine uses a small read subset; define `contracts.I18nResolver`; relocate `LanguageTranslationsResponse`/`TranslationResponse` (or convert in adapter); adapter at root.
5. **ou**, 6. **idp**, 7. **resource** — same pattern; scope each by the methods/DTOs the engine entry packages actually invoke.
8. Re-point `oauth.Initialize` / `flowmeta.Initialize` / `flowexec.Initialize` params to the contract types; add adapters in `servicemanager`; verify `go list -deps ./internal/oauth ./internal/flow/flowexec ./internal/flow/flowmeta | grep -E 'lib/pq|modernc.org/sqlite'` is empty.

### Scope reality confirmed while implementing slice 1 (flowdef)

The `flowdef` slice (slice 1) is done and cuts `flowexec → flowmgt`. But verifying exposed that the contamination surface for the three engine entry packages is wider than the initial §9 list — it is essentially the whole **injected-service-interface surface**:

- **`flowexec` → `flow/executor`.** `flowexec` imports `executor` for `ExecutorRegistryInterface` plus helpers (`GetDefaultInputs`, `GetName`, `GetType`) and error values. The `executor` package aggregates ~7 DB-backed service interfaces in `ExecutorDependencies` (`attributecache`, `entitytype`, `group`, `idp`, `notification`, `ou`, `role`). Fix (same pattern as flowdef): relocate `ExecutorRegistryInterface` + the helper funcs/errors `flowexec` uses into a driver-free leaf (e.g. `flow/core`), so `flowexec` stops importing `executor`; the builtin executors + `ExecutorDependencies` stay in `executor`, imported only by the composition root.
- **`flowmeta` →** `ou`, `i18n/mgt`.
- **`oauth` →** `resource`, `ou`, `idp`, `attributecache`, `i18n/mgt`.

Net remaining (each a bounded, independently verifiable slice like flowdef): one `executor`-registry relocation, plus engine-owned contracts + adapters for the union of injected services — `resource`, `ou`, `idp`, `attributecache`, `i18n/mgt` — and their signature DTOs. Realistically ~8–12 more slices. Large but mechanical and tractable; each ends with the `go list -deps … | grep` set shrinking.

### Decision: actorProvider descoped (accept drivers via actor path)

**Decided.** `actorProvider` is **not** contract-ified. It stays injected as `actorprovider.ActorProviderInterface` from its real package. Rationale: it cascades the worst — `actorprovider → inboundclient → provider` and `actorprovider → entityprovider → entity → provider`, and its interface returns DTOs (`entityprovider.Entity`, `entityprovider.EntityGroup`, `inboundmodel.OAuthClient/InboundClient`) whose home packages are themselves driver-backed. Relocating that DTO web is disproportionate.

**Consequence (important):** `actorProvider` is imported by all three engine entry packages (`oauth`, `flowexec`, `flowmeta`). Because it transitively imports `provider` (which blank-imports `modernc.org/sqlite` and whose `retry.go` imports `lib/pq`), **the engine binary links the SQL drivers regardless of any other contract work.** The strict `go list -deps … | grep -E 'lib/pq|modernc.org/sqlite'` == empty goal is therefore **not achievable** while actorProvider is descoped. Contract-ifying the remaining services (`ou`/`idp`/`resource`/`attributecache`/`i18n`) would not change the grep result on its own.

Net: the import-clean goal is effectively relaxed to **runtime-Redis-only** (the engine never opens SQL; embedder passes Redis config) plus a documented caveat that embedders must not also blank-import `lib/pq`/`modernc.org/sqlite` (to avoid the duplicate `database/sql` driver-registration panic). A build-tag on the driver registration could later remove the *linkage* if desired, but that is a separate, optional effort.

The completed, still-valuable work stands: Phase 1 (provider/redisstore/dbtypes split), Phase 2 (runtime stores injected, Redis-capable), and the flowdef + executor-registry relocations.

### Revised phase order

Phase 2 ✅ (runtime stores) → **Phase 2.5 (new): relocate injected service interfaces + DTOs to driver-free contract packages; re-point `oauth`/`flowexec`/`flowmeta` at them; verify `go list -deps … | grep -E 'lib/pq|modernc.org/sqlite'` is empty** → Phase 3 (Redis injection) → Phase 4 (engine) → Phase 5/6/7.
