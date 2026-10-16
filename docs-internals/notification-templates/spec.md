# Notification Templates Specification

- **Status:** Draft
- **Version:** 0.1
- **Related documents:** [Feature issue #5337](https://github.com/thunder-id/thunderid/issues/5337), [design discussion #5388](https://github.com/thunder-id/thunderid/discussions/5388)

## Summary

ThunderID ships fixed email and SMS templates. Changing their content requires editing server files and restarting the service. Administrators cannot manage or preview templates through the Console or an API. This specification adds runtime template management and live previews, with language and design support: a template references translation keys and a design rather than embedding its own copy or branding. 

Each template stores one content definition rather than separate content per language, and it can reference translation keys so the same template localizes through the Translation feature. The Design feature supplies the design themes when an notification is rendered. The template stores a color scheme selection and message content refers design tokens rather than embedding the design itself.

A flow node selects the template to send, and notifications dispatched outside a flow use the same managed templates: the SMS OTP authentication endpoint. During execution the renderer resolves placeholders in a fixed order: first the `{{t(...)}}` translation keys for the requested language (falling back to the system language), then the `{{design(...)}}` tokens against the theme (email body only, so SMS skips this step), and finally the `{{ctx(...)}}` runtime values. The rendered content is then handed to the sender.

## Architecture

A template belongs to a notification channel, currently email or SMS. An administrator selects a template for a flow’s notification node, and the corresponding channel executor uses it. An email executor cannot use an SMS template, or vice versa.

Template content belongs to an organization unit and is independent of language and design. Its message content can reference translation keys; the resolved translation text, which can contain `{{ctx(...)}}` placeholders, is produced by the Translation feature at render time. The template stores the selected color scheme; the Design feature resolev the design tokens when the notification is rendered or previewed.

The Notification Templates module manages templates and coordinates rendering. It uses the Translation feature to resolve localized text and the Design feature to apply the relevant design. Runtime consumers invoke the template renderer, which returns the fully resolved notification content ready for sending. They do not call the Translation or Design features themselves to resolve template placeholders; the renderer has already applied them.

Templates required by flows, which are themselves created at bootstrap, are imported through the existing bootstrap path. After import they are managed like any other template: editable in `mutable` store mode, and read-only when file-declared. Features installed or enabled later create any additional templates they require.

```mermaid
flowchart TB
    subgraph clients [Clients]
        Console["ThunderID Console<br/>template editor + preview"]
        ApiClient["API consumers<br/>SDKs / GitOps"]
    end

    subgraph core [ ]
        direction LR
        subgraph nt [Notification Templates module]
            API["Notification Templates API"]
            MgmtSvc["Template management<br/>(create / read / update / delete)"]
            Renderer["Template renderer<br/>resolves translations, design, and<br/>{{ctx(...)}}; returns content for sending"]
            Store[("Template store<br/>DB, mutable, language-neutral content")]
        end

        subgraph install [Install-time]
            Bootstrap[("Bootstrap bundle")]
            Importer["Import service"]
        end
    end

    subgraph reused [Reused features]
        Translation["Translation feature<br/>resolve translation keys for locale"]
        Design["Design feature<br/>theme / color scheme (email only)"]
    end

    subgraph consumers [Runtime consumers]
        Flow["Flow email/SMS executors<br/>+ out-of-flow callers (SMS OTP)"]
        Sender["Notification senders (unchanged)"]
    end
    Recipient["Recipient"]

    Console --> API
    ApiClient --> API
    API -->|manage / preview| MgmtSvc
    MgmtSvc --> Store
    Bootstrap -->|seed templates at install| Importer --> Store
    Renderer -->|read template| Store
    Renderer -->|resolve keys for locale| Translation
    Renderer -->|apply design to email| Design
    Flow -->|render for send| Renderer
    Renderer -. rendered notification .-> Flow
    Flow -->|deliver| Sender --> Recipient

    Store ~~~ Translation
    nt ~~~ reused
```

| Component | Responsibility |
|---|---|
| Notification Templates module | Manage templates and coordinate rendering |
| Template renderer | Return fully resolved notification content (translations and design applied) for sending, so runtime consumers do not call the Translation or Design features directly |
| Template store | Persist templates and their content |
| Declarative resources | Seed the default templates as declarative resources (`resource_type: notification_template`) at bootstrap, matched by handle for idempotent upsert |
| Translation feature | Resolve `notification`-namespace translation keys for the target language. |
| Design feature | Apply the design theme based and color scheme. |
| Flow executors | Select a template and, through the template renderer, supply the design theme, the runtime (flow) context, the locale. |
| Notification senders | Deliver rendered email or SMS |
| Console | Manage templates and show the template-editor preview |

## Detailed design

### Module and interfaces

The feature lives in the `internal/notificationtemplate` package (a single flat package with the API handler, service, DB store, cache-backed store, and renderer). It reuses `internal/system/i18n/mgt` for translations, `internal/design/resolve` for design, `internal/notification` for delivery, and `internal/system/declarative_resource` for declarative-resource seeding and export.

Management goes through the service interface; runtime consumers (flow executors and out-of-flow callers) depend only on the narrow renderer interface, never on the management service directly.

```go
// Management surface.
type NotificationTemplateServiceInterface interface {
    ListTemplates(ctx, channel ChannelType, limit, offset int) (*TemplateListResponse, *ServiceError)
    CreateTemplate(ctx, channel ChannelType, req CreateTemplateRequest) (*Template, *ServiceError)
    GetTemplate(ctx, channel ChannelType, id string) (*Template, *ServiceError)
    GetTemplateByHandle(ctx, channel ChannelType, handle string) (*Template, *ServiceError)
    ValidateTemplate(ctx, channel ChannelType, req CreateTemplateRequest) *ServiceError
    UpdateTemplate(ctx, channel ChannelType, id string, req UpdateTemplateRequest) (*Template, *ServiceError)
    DeleteTemplate(ctx, channel ChannelType, id string) *ServiceError
    // Injected after construction to avoid a cyclic import with the dependency registry.
    SetDependencyRegistry(r resourcedependency.Registry)
}

// Runtime renderer: the only surface flow executors and out-of-flow callers use.
type TemplateRendererInterface interface {
    Resolve(ctx, channel ChannelType, handle string, in RenderInput) (*ResolvedContent, *ServiceError)
    // Injected after construction because the design resolve service is built after the renderer.
    SetDesignResolver(resolver designResolver)
}
```

DTO and model shapes:

```go
type Template struct {
    ID, OUID, Handle, DisplayName, Description string
    IsReadOnly bool             // true for file-declared templates (read-only via the API)
    Design     *TemplateDesign  // email only; nil for SMS
    Content    TemplateContent
}
type TemplateContent struct { Subject, Body string } // Subject empty for SMS
type TemplateDesign  struct { ColorScheme string }   // "light" | "dark"

type CreateTemplateRequest struct { // OUID is required and immutable, set only at creation
    OUID, Handle, DisplayName, Description string
    Design  *TemplateDesign
    Content TemplateContent
}
type UpdateTemplateRequest struct { // Handle omitted: it is immutable
    DisplayName, Description string
    Design  *TemplateDesign
    Content TemplateContent
}

// Render input supplied by the caller; ResolvedContent is returned ready to send.
type RenderInput    struct { Locale string; Data map[string]string; ThemeID string }
type ResolvedContent struct { Subject, Body string }
```

`channel` is taken from the request path, not the body. Delivery clients expose their own provider interfaces in `internal/notification/client` (`EmailClientInterface`, `MessageClientInterface`).

### Template configuration and management

Each template belongs to an organization unit (`ouId`, required and immutable). Attaching a template to an organization unit records ownership; access control by organization unit is not enforced in this phase. The handle is unique per channel across the deployment, so a flow's reference by handle resolves to a single template; per-organization-unit or per-application overrides of the same handle are not supported in this phase.

Editability depends on the template store mode, selected by `notification.template.store` (default `mutable`; also `declarative` and `composite`). In `mutable` mode the bootstrap templates are seeded into the database and are ordinary editable templates. In `declarative` and `composite` modes, templates declared in files are read-only (`isReadOnly: true`): a create, update, or delete that targets a file-declared template returns `403` (`NTM-1022`), and in `declarative`-only mode creating any template through the API is refused with `403` (`NTM-1024`). A read-only template can still be read, listed, rendered, and previewed.

Administrators can create, list, read, update, and delete editable templates through the API or Console.

The creation wizard offers sample templates (hardcoded in the Console and kept in sync with the bootstrap defaults) alongside a **Start from scratch** option. A sample supplies starting content for a new template and has no special status after creation.

### Content and localization

An email template has a subject and body. An SMS template has a body only. A content field is a string that can embed three kinds of placeholder, none, one, or many, alongside literal text:

- `{{t(...)}}`: a translation key, resolved to localized text by the Translation feature.
- `{{ctx(...)}}`: a runtime value the sender injects, substituted at send time (see below). The Console token catalog offers these tokens for authoring. Write-time validation checks only that the placeholder is well-formed, not that its key is in this set; a `{{ctx(...)}}` key the runtime does not supply is caught at send time, where the render fails closed (see Rendering and delivery).
- `{{design(...)}}`: a design token (for example `{{design(palette.primary.main)}}`), resolved against the applicable design. Allowed only in the email body, never in a subject or in SMS.

Placeholders are validated when a template is created or updated. A malformed or unsupported placeholder (`NTM-1020`), or a `{{design(...)}}` placeholder in a subject or in SMS (`NTM-1021`), is rejected with `400` before the template is stored, so a template whose placeholder grammar would fail to render cannot be saved. The validator checks placeholder syntax and design placement; it does not verify that a `{{ctx(...)}}` key is one the runtime injects, so those are resolved (and can fail closed) at render time.

The renderer resolves `{{t(...)}}` keys through the Translation feature (`internal/system/i18n/mgt`) under a dedicated `notification` translation namespace (`ResolveTranslationsForKey(ctx, language, "notification", key)`), and `{{design(...)}}` tokens through the Design feature, so a template is localized and themed at render time. The default templates seeded at bootstrap are authored this way: their copy is `{{t(...)}}` keys in the `notification` namespace (for example `otp.email.subject`, `otp.email.heading`, `cibaNotification.email.*`), their email colors are `{{design(...)}}` tokens, and `{{ctx(...)}}` placeholders carry the runtime values. An administrator authors or edits translation keys from the editor's token panel (its inline creates a `notification` key) or from the Console Translations page.

The resolved translation text can contain `{{ctx(...)}}` placeholders for values supplied at runtime. A flow is one such caller, but not the only one: an out-of-flow sender such as the SMS OTP endpoint supplies the same values. These placeholders are substituted when the notification is sent, and remain visible in previews because a preview has no runtime context.

Notification layouts are outside the scope of this phase; the resolved body provides the full content.

### Rendering and delivery

When a flow reaches a notification node, its executor calls the template renderer (`Resolve`) with the channel, the template handle, the runtime context values (`Data`), and the theme ID; the locale. The renderer resolves translation keys, resolves the design tokens against the theme (email body only), and substitutes context placeholders, then returns the fully resolved content ready for sending. The executor therefore never calls the Translation or Design features itself; it receives finished content and hands it to the existing notification sender for delivery.

Delivery reuses the existing notification sender service (`internal/notification`) unchanged. Both executors hand the rendered content, plus the provider (sender) named on the node, to the sender service: the email executor (`internal/flow/executor/email_executor.go`) calls `SendEmail`, and the SMS executor (`internal/flow/executor/sms_executor.go`) calls `SendMessage`. The sender service resolves the managed sender by ID and dispatches through the matching client in `internal/notification/client`: the SMTP client for email, or Twilio, Vonage, or a custom provider for SMS. Only the content these executors send changes; the transport packages are not modified.

Rendering is fail-closed: the notification is **not** sent when a referenced translation key (when fallback to matching language also failed), a required `{{ctx(...)}}` value, or a `{{design(...)}}` token cannot be resolved, or when rendering otherwise fails. In those cases the error is surfaced to the caller (the flow execution, or the out-of-flow sender) and no partially rendered message is delivered.

### Data model

A template is identified by a server-assigned UUID and belongs to an organization unit recorded in its `ouId`, set at creation and immutable. Each template also carries a unique, immutable `handle` and a human-readable `displayName`, following the same convention as themes, layouts, and flows. It stores a channel, an optional description, and channel-specific content definition. A single flat entity holds a template and its content; content is language-neutral, so there are no per-locale rows.

The `handle` is unique per channel across the deployment: creating a second template with an existing handle in the same channel returns `409`, regardless of organization unit, so a handle resolves to a single template and there is no per-organization-unit override in this phase. The `displayName` is not unique and can be edited freely.

Flows reference a template by its `handle` rather than its UUID. Because the handle is stable and deployment-independent, bootstrapped flows stay valid without fixed UUIDs, and a flow exported from one deployment remains valid when imported into another.

Templates persist in the **config database** (the store uses the config DB client; the table ships in `dbscripts/configdb`), replacing the current read-only template file store. A template is a single flat row, with content and design held as JSON columns, so there are no per-locale rows:

```sql
CREATE TABLE NOTIFICATION_TEMPLATE (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(36)  NOT NULL,            -- server-assigned UUIDv7
    CHANNEL       VARCHAR(16)  NOT NULL,            -- email | sms
    HANDLE        VARCHAR(255) NOT NULL,
    OU_ID         VARCHAR(36)  NOT NULL,            -- owning organization unit
    DISPLAY_NAME  VARCHAR(255) NOT NULL,
    DESCRIPTION   VARCHAR(512),
    CONTENT       JSONB        NOT NULL,            -- { subject, body }
    DESIGN        JSONB        NOT NULL DEFAULT '{}', -- { colorScheme }, email only
    CREATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    UPDATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, ID),
    UNIQUE (DEPLOYMENT_ID, CHANNEL, HANDLE)
);
```

Scoping columns are `DEPLOYMENT_ID` and the owning `OU_ID`; the `UNIQUE (DEPLOYMENT_ID, CHANNEL, HANDLE)` constraint keeps a handle unique per channel across the deployment, and the runtime lookup is by channel and handle, not by organization unit.

Templates are seeded and kept up to date as **declarative resources** (`resource_type: notification_template`) through the declarative-resource system (`internal/system/declarative_resource`), matched by handle for idempotent upsert. The runtime lookup (`GetTemplateByHandle`) is served through a read-through cache keyed by `channel:handle`; writes keep it coherent (create and update re-populate the entry, delete invalidates it), while list and get-by-id bypass the cache. Migration preserve existing shipped templates and valid flow references.

### API

The Notification Templates API manages templates by channel. The `channel` path parameter accepts `email` or `sms`, and `id` is the UUID assigned by the server. Read operations (list, get) require the `system:notificationtemplate:view` scope; write operations (create, update, delete) require the `system:notificationtemplate` scope.

| Method | Path | Description |
|---|---|---|
| `GET` | `/notification-templates/{channel}/templates` | List template summaries for the channel, with `limit` and `offset` pagination. |
| `POST` | `/notification-templates/{channel}/templates` | Create a template with its content. |
| `GET` | `/notification-templates/{channel}/templates/{id}` | Get a template. |
| `PUT` | `/notification-templates/{channel}/templates/{id}` | Update a template. The `handle` is immutable and cannot be changed. |
| `DELETE` | `/notification-templates/{channel}/templates/{id}` | Delete a template. Idempotent. |

A template carries a required, immutable `ouId` (the owning organization unit), a unique, immutable `handle`, a `displayName`, an optional `description`, a read-only `isReadOnly` flag (true for file-declared templates), and channel-specific content. Email content has a `subject` and `body`; SMS content has a `body` only (plain text, no subject or design configuration). Each content field is a string that may embed `{{t(...)}}` translation placeholders and `{{ctx(...)}}` context placeholders, so a field can reference none, one, or multiple translation keys alongside literal text. An email template may also carry a `design` with a `light` or `dark` `colorScheme`, composed on top of the content when the email is rendered and ignored for SMS. The `handle` and `ouId` are set at creation and are not accepted on update; a `handle` must be a lowercase kebab-case identifier, and an email template requires a `subject`.

An email template response (the seeded `otp` default, authored with `{{t(...)}}` keys, `{{design(...)}}` tokens, and a runtime `{{ctx(...)}}` value):

```json
{
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "ouId": "a1b2c3d4-0000-4000-8000-000000000001",
  "handle": "otp",
  "displayName": "Email OTP Verification",
  "description": "One-time passcode sent to verify a user's email address.",
  "isReadOnly": false,
  "design": {
    "colorScheme": "light"
  },
  "content": {
    "subject": "{{t(otp.email.subject)}}",
    "body": "<h2>{{t(otp.email.heading)}}</h2><p>{{t(otp.email.intro)}}</p><p style=\"color: {{design(palette.primary.main)}};\">{{ctx(otpCode)}}</p><p>{{t(otp.email.expiry)}}</p>"
  }
}
```

An SMS template response (no subject or design; body authored with a `{{t(...)}}` key and a runtime `{{ctx(...)}}` value):

```json
{
  "id": "0db4e64e-57aa-49cb-9b8e-1f3044ed5e17",
  "ouId": "a1b2c3d4-0000-4000-8000-000000000001",
  "handle": "otp",
  "displayName": "SMS OTP Verification",
  "description": "One-time passcode sent to verify a user's phone number.",
  "isReadOnly": false,
  "content": {
    "body": "{{t(otp.sms.message)}} {{ctx(otpCode)}}"
  }
}
```

Invalid requests return `400`, unauthorized requests `401`, forbidden requests `403`, and missing templates `404`. Placeholder validation runs on create and update: a malformed or unsupported placeholder (`NTM-1020`) or a `{{design(...)}}` placeholder in a subject or SMS (`NTM-1021`) returns `400` and the template is not stored. A write that targets a file-declared, read-only template returns `403` (`NTM-1022`), and a create attempted while the store runs in declarative-only mode returns `403` (`NTM-1024`). Creating a template whose handle already exists in the channel returns `409` (`NTM-1009`). Delete is idempotent: deleting a template that does not exist still returns `204`. Errors use the shared `Error` response shape with the `NTM-XXXX` code prefix for template-specific failures.

Previews are produced in the Console (see Preview).

### Preview

Previews show how a notification will appear for a selected language and applicable design, leaving `{{ctx(...)}}` placeholders visible. Generating a preview never sends a notification.

Preview is available in the **template editor**; it updates as the administrator edits the template or changes the preview language and design theme. A flow-editor preview of the selected template is future work.

The preview resolves the content, translations, and design the same way as delivery, and additionally flags tokens that would fail a real send (an unsupported or malformed token, a missing translation, or a `{{ctx(...)}}` key outside the catalog) so the administrator can correct them before saving.

### UI

The Console surfaces notification templates under **Customize → Notifications**, split by channel into an **Email Templates** page and an **SMS Templates** page, with a sibling **Translations** page (**Design** is its own Customize page). A channel page lists its templates by display name with edit and delete actions and pagination, and a **New template** action starts the creation wizard; on a deployment with more than one organization unit the wizard begins with an organization-unit step (skipped for a single OU), then offers a **Start from scratch** option and sample templates (hardcoded in the Console and kept in sync with the bootstrap defaults). The shipped defaults appear in the list (Email OTP Verification, Magic Link Sign-In Email, CIBA Authentication Notification Email, Password Recovery Email, User Invitation Email, Self Registration Invite Email). An **Import / Export** action covers declarative (GitOps) round-tripping.

![Email Templates list page: templates by display name with edit and delete actions, a New template button, and pagination](assets/email-templates-list.png)

The editor shows the template's display name and description, both editable inline, and for email a **Design** panel with a **Color scheme** toggle (light or dark) that the preview uses. It provides the **Subject** field (email only) and the **Body** field, with the email body authored as HTML. A token panel inserts a placeholder into the focused field: a searchable list of **Translation tokens** (`{{t(...)}}`, with an inline **+** to create a new `notification`-namespace key), **Design tokens** (`{{design(...)}}`, for example primary color or text primary), and the `{{ctx(...)}}` context tokens.

On the right, a **Preview** panel renders the template for a selected **Language** and **Design theme**: it resolves the translation and design tokens and shows the formatted subject and body, leaving `{{ctx(...)}}` placeholders visible (for example `{{ctx(otpCode)}}`). The preview validates the content live and flags any placeholder that would not resolve: an unsupported or malformed token, a design token used where design does not apply or one that does not match the selected theme, a missing translation, or a context key outside the catalog. While any such warning is shown, **Save** (editor) and **Create** (wizard) are disabled, so a template that would fail to send cannot be saved; this complements the server-side write validation, which enforces placeholder syntax and design placement.

![Email template editor: a Design color-scheme toggle, Subject and HTML Body fields, the translation and design token panel, and a live Preview with language and design-theme selectors](assets/email-template-editor.png)

The flow editor lists templates for the selected notification node’s channel so an administrator can pick one.

## Requirements

### R1. Manage notification templates at runtime

**Requirement:** Privileged (authorized) users can manage notification templates without editing server files or restarting ThunderID.

**Acceptance criteria:**

- **AC1.1:** Given a privileged, authorized user, when they create or update a template through the API or Console, then later reads and sends use its saved content without a restart.
- **AC1.2:** Given a bootstrap template seeded in `mutable` store mode, when an administrator edits it, then it is updated like an administrator-created template, with no special classification.
- **AC1.3:** Given a create or update whose content has a malformed or unsupported placeholder, or a design token in a subject or SMS, when it is submitted, then the API returns `400` and nothing is saved.
- **AC1.4:** Given a file-declared, read-only template, when a write targets it (or a create is attempted in declarative-only mode), then the API returns `403` and nothing is changed.

### R2. Localize content through translations

**Requirement:** A single template definition can produce content in a target language using translation keys. This phase renders in the system default language.

**Acceptance criteria:**

- **AC2.1:** Given a template whose subject and body reference translation keys, when it is rendered for a language with corresponding translations, then the output contains the resolved text without changing the stored template.
- **AC2.2:** Given a key with no translation for the target language but a value in the system language, when rendering runs, then the notification is sent using the system-language fallback.
- **AC2.3:** Given a referenced key with no translation in any language, or a required `{{ctx(...)}}` or `{{design(...)}}` value that cannot be resolved, when rendering runs, then the notification is not sent and the error is surfaced.

### R3. Apply design at send time

**Requirement:** Emails apply the applicable design at render time, composing the selected theme with the template's stored `colorScheme` and resolving any `{{design(...)}}` tokens in the body, without embedding the design itself in the content. Design is resolved against the theme supplied at render time.

**Acceptance criteria:**

- **AC3.1:** Given the same template rendered with two different themes, when each sends an email, then each email uses the design of its theme.
- **AC3.2:** Given a theme's design changes, when a later email is rendered with that theme, then it reflects the change without updating the template.
- **AC3.3:** Given an SMS template, when it is rendered, then its output is plain text without design markup.

### R4. Preview notifications

**Requirement:** Administrators can inspect notifications in the template editor. A flow-editor preview of the selected template is future work.

**Acceptance criteria:**

- **AC4.1:** Given a template and selected language and design context, when its preview loads in the template editor, then it shows the resolved translations and applicable design while leaving `{{ctx(...)}}` placeholders visible, and no notification is sent.
- **AC4.2:** Given an email or SMS notification node, when its template picker opens, then it lists templates for that node’s channel.
- **AC4.3:** Given the editor preview reports a warning (an unresolved or unsupported token, a missing translation, or a context key outside the catalog), when the administrator attempts to save or create, then the action is blocked until the warning is resolved.

### R5. Provision templates used by flows

**Requirement:** Flows, out-of-flow senders, and installed features that require a notification template have a corresponding manageable template.

**Acceptance criteria:**

- **AC5.1:** Given a fresh installation, when bootstrap runs, then it seeds the default templates required by its shipped flows and notification endpoints as declarative resources (`resource_type: notification_template`), idempotently upserted by handle.
- **AC5.2:** Given a feature that requires a new template, when it is installed or enabled, then the corresponding template is created through the same declarative resource import path.

### R6. Deliver rendered notifications through the selected provider

**Requirement:** A fully rendered notification is delivered through the provider selected for the sending node, reusing the existing notification sender service and clients.

**Acceptance criteria:**

- **AC6.1:** Given a node with a selected provider, when a template renders successfully, then the sender service dispatches the rendered content through that provider's client (SMTP for email; Twilio, Vonage, or custom for SMS).
- **AC6.2:** Given a sending node that names no usable provider, when it executes, then delivery fails and the error is surfaced, rather than silently dropping the notification.

## Change log

| Version | Date | Change |
|---|---|---|
| 0.1 | 2026-09-24 | Initial specification based on the notification templates design discussion. |
