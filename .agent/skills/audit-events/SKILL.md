---
name: audit-events
description: Add audit events (audit logs) to a ThunderID management service so each create, update and delete is written as one audit line. Use whenever adding, changing or reviewing audit events for a resource such as roles, groups, agents, user types, connections, resource servers, organization units, flows or server configuration, or wiring the audit event bus into a service.
allowed-tools: Read Write Edit Bash
---

# Audit Events

Audit events are added one management resource per PR, by several people in parallel. Follow this skill so every PR has the same shape. Read it with the `backend`, `backend-tests` and `spec-sync` skills. Templates, the resource table and the PR description text are in [reference.md](reference.md).

## Rules

These are not up for local redesign. If your resource seems to need an exception, stop and ask on #6320.

1. **The domain service publishes, explicitly.** The service where the change happens publishes the event with `eventbus.TryPublish`. Never publish from middleware, a handler wrapper, a route table, a decorator over HTTP handlers or a generic hook: that pattern gets copied onto routes where it does not fit, and real audit cases get skipped because the pattern looked like it covered them. If you think a cross-cutting hook is needed, stop and ask.
2. **Names are the attempted action.** `<resource>.<action>` in lowercase, dotted, present tense (`user.create`). The name never says the result. The outcome does: `succeeded`; `denied` for a refusal raised inside the service, reason `forbidden`; `failed` for anything else, reason derived from the service error code.
3. **One event per attempted action, published once.** Each public mutator wraps a private one that does the work. Internal calls use the private method, so nothing publishes twice.
4. **Subject on `succeeded` and `denied` only.** A `failed` event carries the target and no subject. On a failed update or delete, `target.id` is the caller's path value: untrusted text, capped at 128 characters by the payload.
5. **No new surface.** `audit.enabled` is the only configuration key. Add no configuration key, no envelope field and no line field without a spec change first.
6. **No secrets, no personal data.** Never record secrets, credentials, tokens, client secrets, redirect URIs, attribute values, names or any personal data. The event's entry in `auditedEvents` lists no detail keys unless the spec states why one is needed. An event that is not listed there is dropped and never written.
7. **No runtime imports.** The audit packages must not import runtime packages (`internal/oauth`, `internal/authn`, flow executors). Both binaries link them, and `TestControlPlaneLinksNoRuntime` in `backend/cmd/cpserver/surface_test.go` fails when the Control Plane links a runtime package.
8. **Services get the publish-only bus.** A service takes `eventbus.EventBusInterface`. `eventbus.RegistryInterface` and `eventbus.StopperInterface` stay in the composition root.

## What Is Audited

| Resource | Events | State |
|---|---|---|
| Users | `user.create`, `user.update`, `user.delete` | Merged. The reference implementation. |
| Applications | `application.create`, `application.update`, `application.delete` | PR #6492, the second worked example. Not merged yet. |

Everything else is not audited yet. The table of remaining resources and their planned event names is in [reference.md](reference.md#resources-to-claim).

The data plane events (`client.authenticate`, `token.issue`, `token.revoke`, `user.authenticate`) and the embedder `WithAuditSink` option are a separate track with one owner. Do not add them under this skill.

**Claim a resource before you start.** Comment on the audit delivery issue #6320 with the resource, the package and the event names you plan. One resource service per PR. Wait until nobody else has claimed it.

## How It Fits Together

- `backend/internal/system/eventbus`: the bus. `eventbus.Initialize` returns the bus, the registry and the stopper. `eventbus.NewEvent(ctx, name).With(payload)` builds an event, and `eventbus.TryPublish` publishes it and only logs a failure, so a failed publish never fails the change.
- `backend/internal/system/eventbus/audit`: the audit category. `payload.go` has `NewPayload`, `WithSubject`, `WithDetails` and the outcome constructors. `events.go` has the event name constants, the `TargetType*` constants and the `auditedEvents` allowlist. `actor.go` resolves the actor from the verified security context. `sink.go` has `audit.Subscribe`, which drops any event that fails `validate`.
- `backend/internal/system/eventbus/stdoutsink` writes each event as one JSON line. `backend/internal/system/eventbus/defaultaudit` builds the bus and subscribes that sink when `audit.enabled` is on.
- `backend/internal/system/middleware/client.go` (`ClientMiddleware`) puts the peer address and user agent on the request context with `WithClient` from `backend/internal/system/context`, and `NewPayload` reads them with `GetClient`. A call site never passes the actor or the client.
- `initAuditBus` in `backend/cmd/server/main.go` and `backend/cmd/cpserver/main.go` builds the bus and passes it to `registerServices` as `auditBus`.

`audit.ChangePayload` builds the payload of a change from the service error, sets the subject by rule 4, and maps `tidcommon.ErrorUnauthorized` to `denied`. It comes with PR #6492. In `main` today the user service still has its own `userAuditOutcome` in `backend/internal/user/service.go`, which #6492 replaces. Start your service code after #6492 merges, or write the tests first and switch to `ChangePayload` when you rebase. Never copy `userAuditOutcome` into another package.

## Recipe

Write the tests in the next section before the code. Code templates are in [reference.md](reference.md#code-templates).

1. **Events.** In `backend/internal/system/eventbus/audit/events.go` add the `Event<Resource><Action>` constants, a `TargetType<Resource>` constant, and one `auditedEvents` entry per event with `nil` details. A new action verb gets an `Action` constant in `audit/constants.go` beside `ActionCreate`.
2. **Inject the bus.** Add `bus eventbus.EventBusInterface` as the last parameter of the package's `Initialize` and of its private constructor, and store it on the service struct. Refuse `nil` first thing in `Initialize`, returning `errNilEventBus`, declared in the package's `error_constants.go` exactly as in `backend/internal/user/error_constants.go`. Document the parameter in the `Initialize` comment, as `user.Initialize` does.
3. **Wrap the mutators.** Move the body of each public create, update and delete method into a private method of the same name in lower camel case, with the comment "... without publishing its audit event". The public method calls it, then publishes, then returns the result unchanged. See `CreateUser`, `UpdateUser`, `UpdateUserAttributes` and `DeleteUser` in `backend/internal/user/service.go`.
4. **Publish.** One private `publish<Resource>Event(ctx, name, action, id, svcErr)` per service builds `audit.Ref{Type: audit.TargetType<Resource>, ID: id}` and calls `eventbus.TryPublish(ctx, s.bus, eventbus.NewEvent(ctx, name).With(audit.ChangePayload(ctx, target, action, svcErr)))`. On create, pass the new ID only when `svcErr` is nil (see `publishCreate`). On update and delete, pass the ID the caller sent.
5. **Wire both binaries.** Pass `auditBus` to your `Initialize` in `backend/cmd/server/servicemanager.go` and in `backend/cmd/cpserver/servicemanager.go`. The Control Plane is the priority: every management resource it serves must audit there. Some services are wired in one binary only (for example `project.Initialize` only in the Control Plane, `variablestore.Initialize` only in the server); wire every place the service is initialized. Fix every other caller of `Initialize`, tests included.
6. **Every path goes through the audited methods.** List each path that changes the resource and check that it calls the public method: the management handler, MCP tools (`tools.go` in the package), the importer (`backend/internal/system/importer`), bootstrap, and any other service that calls it. Paths without a caller (bootstrap, flow provisioning) publish with an anonymous actor; that is expected.
7. **Importer upsert.** If the importer updates or creates your resource, it must look the resource up first and then call update or create, never try update and fall back to create, or every create is recorded as a failed update. Copy `importUser` in `backend/internal/system/importer/service_adapters.go` (the `exists` lookup before `UpdateUser` or `CreateUser`).
8. **Limitations, not workarounds.** A path that bypasses the service (declarative or file-backed resources, direct database edits) is recorded as a limitation in the spec and the PR description. Never add special logic for declarative or file-backed resources to close the gap.

## Tests

Write each test from the requirement before the code. Templates are in [reference.md](reference.md#test-templates).

**Unit tests**, in the package, per public mutator:

- `succeeded`: name, target, subject, action and outcome. The actor is the caller from the context.
- `denied`: only if the service itself can refuse (it returns `tidcommon.ErrorUnauthorized`). Subject is set.
- `failed`: a validation error and a store error, each with its reason; no subject.
- Exactly one event per call, including paths that call other mutators inside the service.
- No secret or personal value in the payload: assert `Details()` is empty, and that no value you sent appears.
- A failing bus does not fail the operation (`eventbusmock.NewEventBusInterfaceMock` returning an error, as in `TestUserService_AFailingBusDoesNotFailTheChange`).
- `Initialize` refuses a nil bus (as in `TestInitialize_RefusesANilEventBus` in `backend/internal/user/init_test.go`).
- Add the new events to the cases of `TestValidate` in `backend/internal/system/eventbus/audit/payload_test.go`.

Use `newRecordingBus` (a real bus with a sink subscribed through `audit.Subscribe`, so the allowlist is enforced) to assert what was published, and `discardingBus` for tests the event does not matter to. Both are in `backend/internal/user/service_test.go`; copy them into your package's test file. Never use a `.Maybe()` mock for the bus: it lets a missing publish pass.

**Integration tests**, in `tests/integration/audit`, on `AuditLogTestSuite`, which drives the real management API and reads the server's stdout:

- A lifecycle test like `TestAUserLifecycleWritesOneLinePerChange`: create, update, delete, one line each, then assert actor, target, subject, outcome, client and that no secret or name appears in `line.Raw`.
- A table test like `TestEveryCreateAndUpdatePathRecordsItsOutcome` for failed and denied outcomes and for the other paths (import, MCP where it applies). For a caller scoped to one organization unit, adapt `createScopedUserManager` to the permissions your resource needs.
- Read lines with `linesOf(responses...)` once, after all calls. It sends a fence call and waits for its line, then reads everything once. No fixed sleeps, and no per-call waits.
- A Control Plane test in `tests/integration/controlplane`. None exists in `main` yet; `tests/integration/controlplane/audit_test.go` comes with #6492 and is the pattern to copy.

Lessons that cost a review round:

- Names built in tests must fit their columns. `ROLE.NAME` is `VARCHAR(50)`: Postgres enforces it and SQLite does not. Run the integration test on Postgres too (see [reference.md](reference.md#running-on-postgres)).
- A sign-in is bounded to its application's organization unit. A user outside the default unit signs in through `testutils.OUSignInApps`, as `createScopedUserManager` does.
- `make test_integration` only unpacks the last build. Run `make build_backend` first, and `make build_cp_backend` before the Control Plane suite.

## Spec and Docs

Follow the `spec-sync` skill. The eventing spec is `docs-internals/eventing/spec.md`, added by PR #6296; if your base does not have it yet, update it once that PR merges and do not create it yourself. In the same PR:

- Add a requirement with acceptance criteria for the resource, shaped like R9 (users) and R15 (applications).
- Add the resource and its paths to the Instrumentation list.
- Remove the resource from Known gaps, and add the gaps your resource keeps.
- Bump the version and add a change log row.
- Update the threat model only when a trust boundary or a recorded gap changes. The eventing feature has no `threat-model.md` today.

User docs under `docs/` go in a separate PR.

## PR Checklist

- Branch from `upstream/main` after `git fetch upstream`.
- One commit, signed off (`git commit -s`), short imperative message, `Refs #5591` in the body.
- Fill [.github/pull_request_template.md](../../../.github/pull_request_template.md). Paste the known limitations from [reference.md](reference.md#known-limitations-for-the-pr-description).
- Labels: one of `Type/New Feature`, `Type/Improvement` or `Type/Bug`. Add `trigger-pr-builder` alone and last, or CI skips its checks.
- Run `make verify_mocks`, `make lint_backend`, `go test -race` on the touched packages and on `./internal/system/eventbus/...` (from `backend/`), `make build_backend` then `make test_integration PACKAGE=./audit`, and `make build_cp_backend` then `make test_integration_cp`.
- The "Backend Integration Patch Coverage" check counts only integration runs: 70% of changed lines per package, and a package with fewer than 10 changed coverable lines is exempt. Cover your service through the real API, not with more unit tests.

## Known Limitations

Repeat these in every PR description, adapted to the resource:

- Declarative and file-backed resources, and direct database edits, are not recorded.
- Requests refused by the security layer with 401 or 403, and requests a handler rejects before they reach the service, are not recorded.
- A delete of a missing ID is recorded as `succeeded` when the service's delete is idempotent.
- The bus is best effort and can drop events under overload.

## Review Checklist for Audit PRs

- The event is published in the service, through `eventbus.TryPublish`, and nowhere else.
- Each public mutator publishes exactly once, and no public mutator calls another public mutator.
- Names are `<resource>.<action>` constants in `events.go`, each with an `auditedEvents` entry, and no detail keys without a spec reason.
- The payload is built with `audit.ChangePayload`; there is no local outcome mapping.
- `Initialize` takes `eventbus.EventBusInterface`, refuses nil with `errNilEventBus`, and both `servicemanager.go` files pass `auditBus` wherever the service is initialized.
- Every path into the mutation (handler, MCP, importer, bootstrap, other services) uses the public methods, and the importer looks up before update or create.
- No configuration key, envelope field or line field was added.
- Unit tests use `newRecordingBus` and cover succeeded, failed, denied where possible, one event per call, no secret, a failing bus and a nil bus.
- Integration tests use `linesOf` with no fixed sleep, assert no secret or name in `Raw`, and a Control Plane test exists.
- The spec has the requirement, the instrumentation entry, the updated gaps and a change log row. No `docs/` change in this PR.
