# Gateway Management and Pipelines Specification

- **Status:** Draft
- **Version:** 0.1
- **Related documents:** [#4281](https://github.com/thunder-id/thunderid/issues/4281) (separate ThunderID into a Control Plane and a Data Plane), [#5582](https://github.com/thunder-id/thunderid/issues/5582) (default gateway), [Control Plane and Data Plane separation specification](https://github.com/thunder-id/thunderid/pull/6804) (this builds on it and changes none of its requirements). API: [gateway](../../api/gateway.yaml). User guide: [centralized configuration](../../docs/content/use-cases/managed-deployments/centralized-configuration.mdx). Threat model: not yet written.

## Summary

A Control Plane (CP) deploys configuration to the gateways (Data Planes) it administers, in an order called the **pipeline**: a deploy enters at the first stage and is promoted to the next. Today the pipeline can only be declared in server configuration (`gateway.pipeline`) and read through `GET /gateways/pipeline`. Changing it needs a configuration change and a restart. A stage that names a gateway that is not registered, or whose name is ambiguous, is dropped with a log line, so an operator sees a shorter pipeline and no error. The CP also cannot say whether a gateway is reachable or behind the version it should hold, and a promotion may name any two gateways, whatever their stages.

This specification makes the pipeline a managed resource, validates it, reports gateway status, and makes promotion follow the stages. It does not change how a version is captured, diffed or applied, or how value capture chooses its gateway.

The governing decision: **the pipeline follows the same rule as the gateway registry.** It is declared in a file, kept in the database, or both, by `gateway.store`, and what a file declares the API cannot change. A pipeline that cannot be resolved is an error an operator sees, not a stage that silently disappears.

In scope: pipeline storage and API, pipeline and registry validation, gateway status, stage-ordered promotion. Out of scope: approval gates, scheduled promotion, value capture across stages, and Console screens beyond the behaviors they rely on (see Open questions).

## Architecture

| Component | Location | Change |
|---|---|---|
| Pipeline | `backend/internal/gateway/pipeline.go` | Resolves stages from the declared or stored list and reports why one cannot be resolved. |
| Pipeline store | `backend/internal/gateway` (new, beside `store.go`) | Holds the stored stage list. Follows the store modes of the gateway registry. |
| Gateway service and handler | `backend/internal/gateway/service.go`, `handler.go` | Pipeline write, gateway status. |
| Version service | `backend/internal/gateway/version_service.go` | `Promote` checks stage order. |
| Server configuration | `backend/internal/system/config` | `gateway.pipeline` is unchanged and gains validation at start. |

Ownership and seams:

- The gateway registry stays the source of which gateways exist. The pipeline only orders them and names the stages.
- Auto apply and value capture keep using the first stage through the same `pipeline` type, so the API and the deploy path agree on what the first stage is.
- A declared gateway has no `GATEWAY` row, so a stage refers to a gateway by name, as `gateway.pipeline` does today, not by id.

## Detailed design

### Pipeline storage and store modes

The pipeline is an ordered list of stages. A stage has a `gateway` (the registered name), a `name` (defaults to the gateway's name) and an `adminConsoleUrl` (defaults to the gateway's base URL plus `/console`), the same fields as `gateway.pipeline`.

`gateway.store` selects where the list lives, with the meaning it has for gateways:

| Mode | Source of the pipeline | API write |
|---|---|---|
| `declarative` | `gateway.pipeline` only | Refused. |
| `mutable` | The database only. `gateway.pipeline` is ignored. | Allowed. |
| `composite` (default) | `gateway.pipeline` when it is not empty, otherwise the database. | Allowed only when `gateway.pipeline` is empty. |

In `composite`, a declared pipeline therefore always wins and is never edited through the API, which keeps a pipeline checked into a repository the single source of truth. When it is empty, the pipeline is whatever the API last wrote, and empty again means the default gateway is the only stage, as now.

The API replaces the whole list in one request. A reorder, an add and a removal are all a replacement, so two operators cannot interleave edits into an order neither wrote.

### Validation

A pipeline is valid when:

- every stage names a registered gateway, and no two registered gateways share that name;
- no gateway appears in two stages, and no two stages share a name;
- it has no more stages than `gateway.max_gateways`.

The same rules apply wherever a pipeline enters: a write through the API is refused with the first problem found, and a declared pipeline that breaks them stops the server from starting, as other file-declared resources do. A pipeline that was valid when written can become invalid when a gateway changes, so the registry protects it:

- deleting a gateway a stage names is refused, and so is renaming it, until the stage is removed or changed;
- the default gateway has no special place in a non-empty pipeline: it need not be a stage.

A pipeline stored in the database is validated again on read. If it has become invalid, for example because a gateway declared in a file was removed from the files, the read reports the stage that cannot be resolved, and a deploy that would enter the pipeline is refused rather than entering at whatever stage is left. Deploying to a first stage the operator did not choose is the failure the current silent skip allows.

### Gateway status

`GET /gateways/{id}/status` reports, for one gateway:

- `reachable`, and when it was checked;
- when it is not reachable, why: unreachable address, certificate not trusted, key refused, or other;
- `appliedVersion`: the version it holds, from `GATEWAY_APPLIED_VERSION`;
- `inSync`: whether it holds the latest captured version, by comparing hashes, not by computing a diff;
- the time and user of the last apply.

The CP checks reachability with one bounded request to the gateway's management API, under the same address rules and timeout as an apply, and reports the result. Status is computed on read and is not stored, and no background poller is added, so a CP that is not being asked does not call its gateways. The result may be reused for a few seconds so a page that shows every stage does not call each gateway once per viewer.

`GET /gateways/pipeline` includes each stage's status, so the Console and the MCP tools show where a deploy would stop before it starts. A gateway that cannot be reached does not fail the pipeline read: its stage reports the reason.

### Promotion follows the stages

Promotion moves what one stage runs to the stage after it. Today `Promote` refuses only a promotion of a gateway to itself. With a pipeline set:

- a promotion whose source and target are not adjacent stages, in that order, is refused, so a version cannot reach production without passing the stages between;
- a promotion to a gateway that is not a stage is refused;
- with no pipeline, promotion between any two registered gateways stays as it is, so a deployment that never set a pipeline is unaffected.

A promotion otherwise follows the rules of an apply: it sends only what the source holds, reports missing values, requires `confirmRemovals` past the threshold, and holds the target's lease.

### Management key rotation

A gateway's key can be replaced today by an update that carries a new key, and the operator must then give the same value to the gateway. Rotation adds one convenience and one rule:

- an update may ask the CP to generate the new key (`rotateKey`), which returns it once, as registration does, so the key is never typed or mailed;
- a gateway declared in a file never returns or rotates its key through the API, as for every other edit.

The DP accepts one management key hash, so rotating the CP's key and the DP's hash are two steps and a window in which calls are refused. This specification does not add a two-key overlap on the DP (see Open questions).

### Data model

One table, in the `configdb` scripts (`backend/dbscripts/configdb/{sqlite,postgres}.sql`).

| Table | Plane | Holds |
|---|---|---|
| `GATEWAY_PIPELINE_STAGE` | CP | One row per stage: `DEPLOYMENT_ID`, `POSITION`, `NAME`, `GATEWAY_NAME`, `ADMIN_CONSOLE_URL`. Primary key `(DEPLOYMENT_ID, POSITION)`. |

There is no foreign key to `GATEWAY`, because a gateway declared in a file has no row. Replacing the pipeline deletes and inserts the rows in one transaction.

### API

- `GET /gateways/pipeline`: unchanged path, each stage gains `status`.
- `PUT /gateways/pipeline`: replaces the stages. Refused in `declarative` mode, and in `composite` mode when `gateway.pipeline` is not empty.
- `GET /gateways/{id}/status`: gateway status.
- `PUT /gateways/{id}`: accepts `rotateKey`.

Each route requires the `system` permission or the child for gateways. New error codes are allocated in `version_error_constants.go` and `error_constants.go` of the gateway module and listed in [gateway.yaml](../../api/gateway.yaml). MCP tools serve the same operations.

### Configuration

No new keys. `gateway.pipeline` keeps its shape, and `gateway.store` now also decides where the pipeline lives. All are deployment level.

## Requirements

Each acceptance criterion is covered by the design above and by a test written with the change.

### R1. The pipeline is managed

**Requirement:** As an administrator, I change the order and stages of the pipeline without restarting the CP, unless it is declared in a file.

**Acceptance criteria:**

- **AC1.1:** Given `mutable` mode, when a valid list is written, then `GET /gateways/pipeline` returns those stages in that order.
- **AC1.2:** Given `declarative` mode, when the list is written through the API, then it is refused.
- **AC1.3:** Given `composite` mode and a non-empty `gateway.pipeline`, when the list is written through the API, then it is refused and the declared pipeline is unchanged.
- **AC1.4:** Given `composite` mode and an empty `gateway.pipeline`, when a list is written, then it is stored and survives a restart.

### R2. An invalid pipeline is an error

**Requirement:** As an administrator, I am told when a pipeline cannot be resolved, and a deploy never enters at a stage I did not choose.

**Acceptance criteria:**

- **AC2.1:** Given a list with a stage naming an unregistered gateway, a repeated gateway, a repeated stage name, or more stages than `gateway.max_gateways`, when it is written, then it is refused and names the problem.
- **AC2.2:** Given a declared pipeline that breaks those rules, when the server starts, then it does not start.
- **AC2.3:** Given a gateway a stage names, when it is deleted or renamed, then it is refused.
- **AC2.4:** Given a stored pipeline whose first stage can no longer be resolved, when a deploy enters the pipeline, then it is refused and no gateway receives it.

### R3. Gateway status is visible

**Requirement:** As an operator, I see whether a gateway is reachable and whether it holds the latest version before I deploy.

**Acceptance criteria:**

- **AC3.1:** Given a reachable gateway that holds the latest version, when its status is read, then it is reachable and in sync.
- **AC3.2:** Given a gateway that holds an older version, when its status is read, then it is not in sync and names the version it holds.
- **AC3.3:** Given a gateway that cannot be reached, when its status is read, then the reason is reported and the request succeeds.
- **AC3.4:** Given a pipeline with an unreachable gateway, when the pipeline is read, then every stage is returned and the unreachable one carries its reason.
- **AC3.5:** Given a CP that is not asked, when time passes, then it makes no status calls to its gateways.

### R4. Promotion follows the stages

**Requirement:** As an operator, I promote one stage at a time and cannot skip one.

**Acceptance criteria:**

- **AC4.1:** Given a pipeline, when a promotion names adjacent stages in order, then it is applied as an apply is.
- **AC4.2:** Given a pipeline, when a promotion skips a stage, goes backwards, or names a gateway that is not a stage, then it is refused.
- **AC4.3:** Given no pipeline, when a promotion names two registered gateways, then it behaves as it does today.

### R5. A gateway key is rotated without typing it

**Requirement:** As an administrator, I rotate a gateway's key and receive the new one once.

**Acceptance criteria:**

- **AC5.1:** Given a registered gateway, when an update asks to rotate its key, then the response returns a new key once, and a later read does not.
- **AC5.2:** Given a gateway declared in a file, when an update asks to rotate its key, then it is refused.

## Open questions

1. **Approval gates.** Whether a stage needs a named approver before a promotion into it, and what records the approval, is undecided. It needs the actor and audit model, and a threat model.
2. **Value capture across stages.** Capture reaches the first gateway only, so a promotion to a later stage reports the values that stage lacks and waits for an operator. Whether capture should follow the pipeline is undecided. A secret the CP does not hold cannot be resent, so a full answer needs a decision on where secrets live between stages.
3. **A two-key overlap on the DP.** The DP accepts one management key hash, so a rotation has a window of refused calls. Accepting two hashes during rotation changes the DP's security model and needs the threat model.
4. **A DP status endpoint.** Reachability is probed with an existing management route. Whether the DP should expose a purpose-built status route, and what it would report, is undecided.
5. **More than one pipeline.** One CP has one pipeline. Per project pipelines are not considered here.
6. **Threat model.** None exists. The status probe and key rotation touch the CP's calls to gateways and the management key, and the docs-internals readme expects one.

## Change log

| Version | Date | Change |
|---|---|---|
| 0.1 | 2026-11-13 | Initial specification, written against `upstream/main` (b3cc4574b). |
