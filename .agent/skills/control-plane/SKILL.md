---
name: control-plane
description: Control Plane and gateway rules for ThunderID. Use whenever a change touches cmd/cpserver, internal/gateway, configuration versions, the variable store or value references (var:/sec:), import deletions, deployment scoping (DEPLOYMENT_ID, deployment.Resolve), the console the control plane serves (frontend/apps/console-cp, @thunderid/configure-gateways), or wires a new service, executor, or resource type into the server.
allowed-tools: Read Write Edit Bash
---

# Control Plane and Gateways

ThunderID ships as two binaries built from one module:

| Binary | Entry point | Runs |
|---|---|---|
| Server | `backend/cmd/server` | Everything: management APIs plus the runtime (login, flow execution, token issuance, authorization). On its own it is just the server. Beside a Control Plane it is a **gateway**: it runs the configuration the Control Plane deploys to it. |
| Control Plane (CP) | `backend/cmd/cpserver` | Management APIs only, plus gateway registration and configuration versions. No runtime. |

There is no third mode and no `isControlPlane` flag inside a service. A behaviour the Control Plane needs is wired in `cmd/cpserver`; a behaviour only the server needs is wired in `cmd/server`. Packages under `internal/` stay plane-agnostic.

Product docs: [centralized configuration](../../../docs/content/use-cases/managed-deployments/centralized-configuration.mdx), [configuration versions](../../../docs/content/guides/declarative-configurations/configuration-versions.mdx), [value references](../../../docs/content/guides/declarative-configurations/value-references.mdx).

## Wording

- Upstream says **gateway** for the server a version is applied to. Never introduce "environment", "data plane deployment", or "DP" in user-facing text, API fields, or identifiers; "data plane" is fine in code comments and test names that describe the plane split.
- `DEPLOYMENT_ID` and `deployment.Resolve` name the **tenant scope** of a row (one Control Plane may administer several deployments on one database). They do not name a gateway.

## Wiring: Two Service Managers

`cmd/server/servicemanager.go` and `cmd/cpserver/servicemanager.go` each build the server by hand. Nothing checks that they agree, so a service, dependency, or setter added to one and not the other compiles and fails only at runtime on the other plane.

Whenever you touch `cmd/server/servicemanager.go`:

1. Open `cmd/cpserver/servicemanager.go` and decide, for the same change, whether the Control Plane needs it. Management (CRUD, stores, validation, export, import, gateways) does; runtime (token issuance, login, flow execution, authorization evaluation, credential issuance) does not.
2. A dependency a service needs only at runtime must still be satisfied on the Control Plane, by an explicit no-op the service exposes, never by leaving a field nil. A service with an unset dependency must refuse the operation rather than skip it, so a plane that forgot the wiring fails closed.
3. `cmd/cpserver/surface_test.go` (`TestControlPlaneLinksNoRuntime`) fails when the Control Plane binary links a runtime package. Do not add a package to `allowedRuntimeLeaves` to make it pass; move the types the Control Plane needs into a package without an `Initialize`.
4. Build both: `make build_backend build_cp_backend`.

The Control Plane validates flow definitions from the static executor catalog in `internal/flow/executormeta`, not from a live registry. A new executor registers in `executor/init.go` **and** in `executormeta/catalog.go`; `TestCatalogMatchesRegistry` in `internal/flow/executor` keeps them aligned. A new flow type goes into the catalog entries of every executor that supports it. The Control Plane validates executor metadata only; do not plumb live services into its validation.

## Deployment Scoping

- Every table has `DEPLOYMENT_ID`, every query filters by it and binds it last, and every store reads it with `deployment.Resolve(ctx)` from `internal/system/deployment`. The id is put on the context at the edge; start-up tasks, background workers and CLI tooling fall back to the configured id. Never read the configured id in a store, and never pass a deployment id as a function parameter when a context is available.
- Caches that hold rows are keyed by deployment too. A cache keyed by id alone leaks across deployments on a shared database.
- Gateway tables (`GATEWAY`, `CONFIGURATION_VERSION`, `GATEWAY_APPLIED_VERSION`, `GATEWAY_APPLY_LEASE`, `GATEWAY_EXCLUDED_RESOURCE`) follow the same rules; `internal/gateway/change_capture_test.go` shows the per-deployment capture expectation.
- Schema rules are in the `db` skill.

## Gateways and Configuration Versions (`internal/gateway`)

- A gateway is registered through the API (DB store), declared in `config/resources/gateways` (file store, reconciled on every start) or both (composite); `gateway.store` picks the mode and falls back to the global declarative setting. A declared gateway has no `GATEWAY` row, so the version tables carry no foreign key to it.
- A **version** is a capture of this deployment's whole configuration, named by the hash of what it captured, so the same configuration is always the same version. `configurationExport` in `version_service.go` lists every exported type, users and groups included. **A new exportable resource type must be added there**, with the other registrations in the `backend` skill's `resource.md`, or the type is silently absent from every version.
- Apply pushes the version to the gateway through the gateway's import API and reports the import's outcomes. Applies to one gateway are serialised by `GATEWAY_APPLY_LEASE` (one apply at a time, the latest requested version queued). Resources excluded for a gateway (`GATEWAY_EXCLUDED_RESOURCE`) are neither written nor removed there until someone selects them again; a removal that was held back is offered once no version names the resource.
- An apply always carries the **full** configuration, changed or not. Any service behaviour that runs on update must therefore be a no-op for an unchanged resource (no secret rewrite, no revocation, no re-issued credential). Compare against the stored state before acting.
- The user who captured or applied a version is stored as an id only; names are looked up at the trusted issuer when read. Never store usernames.
- Credentials in a version are exported as references the gateway fills in from its own store, never as values.

## Variable Store and Value References

- `internal/variablestore` holds this deployment's variables and secrets. A resource field may hold a reference, `var:NAME` or `sec:NAME`, instead of a value; `variablestore.Lookup` resolves it at import (a variable as it is, a secret decrypted).
- A `sec:` reference may stand only in a field the resource's exporter lists as secret. A new resource with secret fields declares them to the exporter; otherwise a secret reference there is rejected and an export writes the value out.
- On a Control Plane a value is not resolved; it is captured for the gateway. A value written on the Control Plane is pushed to the gateway's store in the background, with retries (`value_capture.go`). Do not resolve references on the Control Plane and do not make the Control Plane depend on holding the value.
- Exports parameterise values into references (`internal/system/export/parameterizer.go`); templates carry neither values nor references. Test both directions when you add a field that can hold a reference.

## Import Deletions

- An import request may carry `deletions` as well as content. Upserts run first, deletions after, in reverse dependency order (`internal/system/importer/delete_runtime.go`). Every resource type that can be deleted through an import registers in `deleterFor`.
- A delete through the import API is a real delete: whatever the service does on `DELETE /<resource>/{id}` (dependency checks, revocation, cleanup) must happen on this path too. Put that behaviour in the service, never in the handler, so every entry point (REST, import, DCR, an applied version) shares it.
- Deleting a resource the server does not hold is reported as already absent, not as a removal that happened. A deleter that spans several services (see `deleteConnection`) reads before it deletes, so a delete that succeeds for an unknown id is not reported as a removal.

## Integration Tests

- Code only the Control Plane runs is tested in `tests/integration/controlplane`, run with `make test_integration_cp` after `make build_backend build_cp_backend`. The target sets `INTEGRATION_PLANE=control`, swaps the Control Plane binary into the distribution and boots the server beside it as a gateway on port 8096 (`testutils.DataPlaneURL`). `make test_integration` leaves that package out. Patch coverage is enforced on every pull request; code only the Control Plane reaches is covered by tests in `controlplane`, never by lowering a coverage bar or adding a label.
- Gateway API, version, reference and apply-safety tests that run on the all-in-one server live in `tests/integration/gateway`.
- Stop any locally running Control Plane (8095) or server (8090) before an integration run; the suites bind those ports.

## Console

- `frontend/apps/console-cp` is the console the Control Plane serves: the same shell as `frontend/apps/console`, composed from the same `@thunderid/configure-*` packages, plus `@thunderid/configure-gateways` (gateway list and detail, the Deploy page, the gateway explorer and values pages) and without the flow tryouts. It runs every administration operation natively (`AdministrationModes.NATIVE`) because the Control Plane executes no flows.
- Gateway and deploy UI lives in `@thunderid/configure-gateways` only, and is mounted only by `console-cp` (`frontend/apps/console-cp/src/App.tsx`, `configs/RouteConfig.ts`). Do not mount gateway or deploy routes in `frontend/apps/console`; the server has no gateways to deploy to.
- Both consoles have their own `RouteConfig`. A new console route is added to both unless the feature is plane-specific; a change to a shared feature package is verified against both apps (`pnpm --filter @thunderid/console ...` and `pnpm --filter @thunderid/console-cp ...`).
