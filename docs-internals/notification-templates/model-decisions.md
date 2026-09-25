# Notification Templates — Model & Validation Decisions

Status: Draft · Scope: `backend/internal/notificationtemplate` + `api/notification-templates.yaml` + `spec.md`

This records the modeling decisions agreed while designing how the notification template
feature represents email-vs-SMS data, validates it, and exposes it through the API. It
complements `spec.md` (the feature spec) and `discussion.md` (the design discussion).

**Phase note:** This phase implements the CRUD management API and its channel-generic backend
only. Rendering — resolving translation keys, composing branding, substituting `{{ctx(...)}}` —
and the preview endpoint and runtime `TemplateProvider` that would consume it are **not** built
yet. Decisions below that concern resolved/rendered output (D6, D12) are recorded for when that
phase lands; they do not describe shipped code.

## Guiding principle

A value that is "email or SMS at runtime" is modeled as **one tagged struct with a `channel`
discriminator**, never as a Go interface. In Go, interfaces express *behavior*; data is a plain
struct. Runtime variance is handled by a `switch channel` (a factory), not by polymorphic data
types. This matches the repo convention (`internal/system/template/model.go` `TemplateDTO`,
`internal/notification/client/factory.go`).

## Decisions

### D1 — Tagged struct, not an interface (data model)
`Template` and the resolved notification are single structs carrying `channel`. No
`NotificationTemplate` interface with `EmailTemplate`/`SMSTemplate` implementations. Interfaces
are reserved for behavior (service, store, provider, channel handlers).

### D2 — `channel` is addressed by path, not carried in the DTO body
`channel` is a route/path parameter and is reflected in each response's `self` link
(`/notification-templates/{channel}/templates/{id}`); it is **not** a JSON field on `Template` or
`TemplateSummary`, matching the OpenAPI contract (D11). The store DAO carries `channel` internally
(it keys every query), so the service can still `switch` on it, but the wire contract does not
duplicate what the path already states. (Superseded the earlier plan to add a `channel` field to the
response DTOs; when the render phase adds `ResolvedNotification`, that type does carry `channel`
because it has no addressing path of its own.)

### D3 — `channel` in the path, echoed on responses, not in request bodies
`channel` is a route/path parameter for addressing (`/notification-templates/{channel}/...`) and
is echoed on responses. It is **not** a field on `CreateTemplateRequest`/`UpdateTemplateRequest`
— the path is the single source of truth.

### D4 — Two model families, converted in the service
- Storage: `templateDAO` (unexported), persisted across two tables: `NOTIFICATION_TEMPLATE` holds the
  identity/name/description plus a single **`CONTENT` JSON column** carrying the whole
  `TemplateContent` document, and a companion `NOTIFICATION_TEMPLATE_DESIGN` table holds the optional
  design (one row per template, none for channels without a design). Content is **not** exploded into
  flat per-field columns (no `SUBJECT_KEY`/`BODY_KEY` columns) — see the DB-design note below.
- API DTOs: `Template`, `TemplateContent` (`subject`/`body`), `TemplateDesign`, `TemplateSummary`,
  request/response types.
The service converts (`toValidatedDAO` request→DAO, `daoToTemplate` DAO→response); the store owns the
JSON (de)serialization of the `CONTENT` column. There is no third "domain" layer. The handler only
decodes request DTOs and serializes responses.

**Why JSON content, not flat columns.** Storing content as one JSON document (validated per-channel at
the service layer, not by the schema) lets a new channel introduce its own content fields without a
DB migration — the exact generality this module is built for. The access pattern never queries content
by sub-field: a template is always fetched whole and rendered, so the one advantage of per-field
columns (or an EAV child table) is never exercised. This matches the DB design agreed in the design
discussion.

### D5 — One common model, not channel-specific models
A single `TemplateContent`/`Template` serves both channels. No `EmailContent`/`SMSContent` types.
Channel-dependent fields are handled by:
- flat field + `omitempty` for stray fields (e.g. `subject`, email-only), and
- an optional nested pointer for a cohesive channel-specific cluster (`Design *TemplateDesign`,
  `nil` for SMS — the nil-ness encodes "not applicable").
Consequence: the model gives **no** validation control; the type can express field *presence*
per channel at best, never value *correctness*.

### D6 — Resolved content is a distinct type
`ResolvedContent` is separate from `TemplateContent` even though the fields match: `TemplateContent`
holds translation *keys*; `ResolvedContent` holds rendered *text* (with `{{ctx(...)}}` left literal).
Distinct types prevent confusing a key for a value.

### D7 — Content is language-neutral (translation keys)
`subject`/`body` are single translation-key references; localized text (which may contain
`{{ctx(...)}}`) is resolved from the Translation feature at render time. `contentType` is
`text/html`|`text/plain`; SMS is forced to `text/plain`, with no subject and no design.

### D8 — Identity & uniqueness
Server-assigned **UUID is the sole identity**; flows reference a template by UUID, and
bootstrap-seeded templates use fixed UUIDs so references stay valid across installs. **`name` is
unique per channel**; a duplicate create returns `409`.

### D9 — Validation is manual, imperative, in the service
Go has no built-in declarative/annotation validation (no `@Valid`/`@NotNull` equivalent). JSON
decoding is structural only (missing fields → zero values; enums unchecked). All business
validation is hand-written in the **service layer** (not the handler), so non-HTTP callers
(providers, bootstrap import) are validated too. The tag-based `go-playground/validator` and
OpenAPI server codegen are **not** used here; validation is hand-rolled by convention.

### D10 — Per-channel behavior via a switch-based factory
Channel-specific rules live behind a `channelHandler` interface (`validate`, `writeContent`,
`resolve`), selected by a `handlerFor(channel)` factory (`switch channel`). This keeps the single
`switch` in one place instead of scattering it across `toValidatedDAO`, `daoToTemplate`, the
validator, and the resolver. A registry map (as in `flow/executor`) is intentionally **not** used
— a switch fits a small fixed set of two channels.

### D11 — API contract: one common schema, channel in path
The OpenAPI uses **one** `Template`/`TemplateContent` schema; email-only fields and the
strict/lenient behavior are described in field descriptions. `oneOf`/`discriminator` is **not**
used: `channel` is in the path (a body discriminator would be awkward/duplicative), and since the
server is not generated from the spec, `oneOf` would buy documentation precision, not enforcement,
at the cost of drift with the single Go struct. The YAML is descriptive; **Go enforces manually**.

### D12 — Provider surfaces
- Management API (handlers) → CRUD DTOs (`Template`, `CreateTemplateRequest`, …).
- Runtime consumers (flow executors) → a narrow `TemplateProvider` returning `ResolvedNotification`,
  not the full CRUD service.

## Resolved decisions

### O1 — Strict vs lenient for channel-mismatched fields → **strict**
When a request sends fields that don't apply to the channel, the channel handler **rejects** the
request rather than silently dropping, so persisted objects are always valid for their channel at
runtime: `subject` on SMS → `400 NTM-1012`, `design` on SMS → `400 NTM-1013`. `contentType` is the
exception — it is server-derived (email always `text/html`, SMS always `text/plain`), so any supplied
value is overridden, not rejected. Implemented in `smsHandler.validate` / `emailHandler.normalize`
(channel.go).

### O2 — Where name-uniqueness is enforced → **both**
A service-level pre-check inside the write transaction (`IsNameExists`) returns a friendly `409`
(`NTM-1011`), and the DB `UNIQUE (DEPLOYMENT_ID, CHANNEL, NAME)` constraint is the race-safe backstop.
The pre-check and the insert share one transaction so a concurrent create cannot slip past the check.

## References
- Convention precedents: `internal/system/template/model.go`, `internal/notification/client/factory.go`,
  `internal/flow/executor/register.go`; JSON-column storage: `internal/inboundclient/store.go`;
  transactional write + name pre-check: `internal/notification/mgt_service.go`.
- Current code: `internal/notificationtemplate/{model,channel,service,store,store_constants,handler,init,error_constants}.go`.
- Schema: `backend/dbscripts/configdb/{postgres,sqlite}.sql` (`NOTIFICATION_TEMPLATE`, `NOTIFICATION_TEMPLATE_DESIGN`).
- Contract: `api/notification-templates.yaml`; feature spec: `spec.md`.
