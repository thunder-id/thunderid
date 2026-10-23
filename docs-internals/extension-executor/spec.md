# Extension Executor Specification

- **Status:** Draft
- **Version:** 0.8
- **Related documents:**
  - Discovery issue #5555 (ThunderID Extensibility: extension model and authentication)
  - Design discussion #5659 (Extension executor: architecture, response contract and custom authentication)
  - Threat model: [threat-model.md](threat-model.md), added in #5746
  - Flow conditions issue #5145

## Summary

Organizations need ThunderID flows to consult their own services: to stop a sign-in a risk engine rejects, to check a credential against an internal list, to authenticate with a method ThunderID does not implement, or to bring in data that only their systems hold. Today the only outbound call in a flow is the HTTP request executor. It fails open by default, passes upstream error text to users, and has no outcome contract.

This specification adds three things:

- an **extension executor**, a flow step that calls an organization's service and acts on a fixed response contract. The request mirrors the step's own flow context and the response mirrors the step's own result, so an HTTP extension sees and answers what an in-process extension would;
- an **HTTP endpoint connection**, a connection type that holds where and how to reach that service;
- an **extension auth resolver**, an authentication step that turns a service's verified answer into a session for a local user.

**The governing design decision:** the extension executor is blind to identity. It can only add data to the flow, under its own namespace and a closed set of four outcomes. Anything that changes who the user is lives in a separate, explicitly placed step (the resolver), so the power to sign a user in is never a property of an ordinary extension.

**In scope:**

- flows of every type;
- server-to-server calls from ThunderID;
- the console and API to configure them.

**Out of scope** (future specifications):

- asynchronous approval;
- non-blocking notifications;
- surfaces outside flows;
- recording a new authentication method or assurance level for logins through the resolver; such a login claims nothing stronger than the flow recorded (AC9.16);
- OAuth client credentials or mutual TLS on the back channel;
- branching on returned values (#5145).

## Architecture

```mermaid
flowchart LR
  subgraph TID[ThunderID]
    direction LR
    FE[Flow engine] --> EX[Extension executor]
    EX -- SUCCESS --> RS[Extension auth resolver]
    RS --> AP[Authn provider manager]
    EX -. GetResolvedConnection .-> HC[HTTP connection service]
    HC --> ST[(HTTP_CONNECTION)]
    CA[Connections API and console] --> HC
    MT[MCP tools] --> HC
    FM[Flow management] -. usages .-> HC
  end
  EX == "HTTPS POST" ==> SVC[Organization's service]
```

| Component | Owner package | Responsibility | Existing seam used |
|---|---|---|---|
| Extension executor | `internal/flow/executor` | Build the request, call the service, evaluate the response, route the flow, store data | Executor registry and catalog (`executormeta`), `ExecutorDependencies` |
| HTTP connection service | `internal/connection/httpconnection` | Store, validate and resolve HTTP endpoint connections; the connection package gives the importer a writer that applies the API's retention and secret-copy rules | Connection routes, declarative resources, export and import, `cmodels` encryption |
| Extension auth resolver | `internal/flow/executor` | Resolve and authenticate the user a service verified | Entity provider, authn provider manager |
| `extensionVerifiedEntityID` credential type | `internal/authnprovider` | The internal path the resolver authenticates through | `InternalCredentialTypes`, default provider dispatch |
| MCP tools | `internal/connection` | Let agents list, read, create and update HTTP endpoint connections under the same rules as the API (R11) | MCP server (`internal/system/mcp`) |
| Usage tracking | `internal/flow/mgt`, `internal/system/resourcedependency` | Report flows that reference a connection; block its deletion | Dependency registry |
| Console and flow builder | `frontend/packages/configure-connections`, `configure-flows` | Configure connections and the two steps | Connection wizard, executor property panels |

The extension executor is a utility executor and cannot set the authenticated user. The resolver is an authentication executor, which is what allows it to.

## Detailed design

### Extension executor: node properties

| Property | Type | Default | Meaning |
|---|---|---|---|
| `connectionId` | string | required | The HTTP endpoint connection to call. |
| `inputKeys` | string array | empty | The user inputs to send. See *Request construction*. |
| `runtimeDataKeys` | string array | unset | Restricts the runtime data sent. See *Request construction*. |
| `responseKeys` | string array, or a comma-separated string | unset | The response fields to store. Unset stores all fields that are allowed. |
| `maxAttempts` | integer, 1 to 20 | 5 | The most `FAILED` and `INCOMPLETE` answers this step accepts in one execution. See *Attempt limit*. |

Validation:

- The step calls only through an HTTP endpoint connection; there is no inline URL. A flow whose extension step has no `connectionId` is refused at save. A step that still reaches run time without one fails with `ErrExtensionConfigInvalid` (configuration invalid) before any call.
- `inputKeys` and `runtimeDataKeys` must be string arrays. `responseKeys` may be a string array or a comma-separated string.
- A connection that does not exist is refused at save, and so is an empty list, a blank name or a non-string value in `inputKeys`, `runtimeDataKeys` or `responseKeys`; such a value is never read as unset (AC1.4, AC1.5). `connectionId` is compared after trimming surrounding whitespace, at save, at run time and when usages are counted (AC3.4).
- A step that still reaches run time with an unusable property value fails with `ErrExtensionConfigInvalid` before any call, rather than falling back to a default, so a malformed `responseKeys` can never store every field (AC1.6).
- `maxAttempts` outside 1 to 20 is refused at save. The upper bound keeps the limit meaningful for a service that verifies a secret; legitimate retries in one sign-in rarely need more.
- Keeping every endpoint in a connection means credentials are always stored encrypted and the `https` rule (AC2.7) always applies. For local development, use a connection with authentication `none` and an `http` URL.

### Extension executor: endpoint resolution

- The executor calls `GetResolvedConnection` on every execution. That returns the URL, the timeout and the final header map, with the authentication header applied and secrets decrypted in memory only. A step with no connection configured, or whose connection does not exist, fails the node with `ErrExtensionConfigInvalid`, and no call is made. A connection that exists but cannot be resolved is a failure inside ThunderID: the node fails with `ErrExtensionCallFailed`, with `errorKind` `internal` in the audit event, and no call is made.
- The embedded engine (`pkg/thunderidengine`) has no connection resolver, so an extension step run there fails with `ErrExtensionConfigInvalid`.

### Extension executor: request construction

The request is a `POST` with:

- `Content-Type: application/json`;
- `X-Correlation-ID`, the flow's trace ID, unless a configured header supplies it;
- the connection's headers, with its authentication applied.

The body mirrors the step's flow context (`providers.NodeContext`), field for field and under the same names, so that a service written for HTTP and an extension running inside ThunderID see the same information. Only the parts that are safe to leave the server are carried:

```json
{
  "version": "1",
  "executionId": "019a1b2c-3d4e-7f60-8a9b-0c1d2e3f4a5b",
  "flowType": "AUTHENTICATION",
  "currentAction": "<action>",
  "currentNodeId": "<node id>",
  "userInputs":  { "<inputKey>": "<value>" },
  "runtimeData": { "<key>": "<value>" },
  "application": { "id": "<id>", "ouId": "<ou id>", "name": "<name>", "type": "<type>" },
  "authUser": {
    "<provider>": {
      "entityReference": { "entityId": "<id>", "entityCategory": "user", "entityType": "<type>", "ouId": "<ou id>" }
    }
  }
}
```

| Field | Source | Notes |
|---|---|---|
| `version` | contract | Fixed at `"1"` by this specification. A future incompatible contract uses a new value; ThunderID never changes the meaning of a published version. |
| `executionId` | `ExecutionID` | The same on every call within one flow execution, so a service can correlate calls, including the call made when a redirected user returns. |
| `flowType`, `currentAction`, `currentNodeId` | the fields of the same name | `currentAction` is omitted when empty. |
| `userInputs` | `UserInputs` | Filtered by `inputKeys`, below. |
| `runtimeData` | `RuntimeData` | Filtered by class and by `runtimeDataKeys`, below. |
| `application` | `Application` | `id`, `ouId`, `name` and `type` only. The inbound authentication configuration, which carries client secrets, is never sent. |
| `authUser` | `AuthUser` | The same layout, provider name to state, with each state reduced to its `entityReference`. Provider tokens and attributes are never sent. `{}` before anyone is authenticated. |

Never sent: node properties, shared runtime data, forwarded data, the execution history, the executor mode, and the node's declared inputs.

**User inputs:**

- Only the inputs named in `inputKeys` are sent, and only those present in the flow.
- An unset or empty `inputKeys` sends `{}`.
- Credentials (password and OTP inputs) are therefore sent only when an administrator names them on that node, for example on an extension that performs authentication.

**Runtime data**, in three classes:

| Class | Members | Sent |
|---|---|---|
| Secret engine state | `otpSessionToken`, `consent_session_token`, `oauthState`, `oidcNonce`, `magicLinkUsedJti`, `storedInviteToken`, `inviteLink`, `magicLinkURL`, `ssoSessionHandle`, `ssoSessionId`, `tokenFamilyId`, `candidateUsers`, `failureReasonJSON`, `openid4vpVerificationState`, `passkeySessionToken`, `linkingCheckpoint`, `provenSubject`, `password` (a service receives a password only through `inputKeys`), and every SSO checkpoint key (names starting with an SSO session key followed by a checkpoint suffix) | Never, even when named. |
| Other engine keys | Every other key on the reserved list (see *Response storage*), for example `clientId`, `requested_permissions`, `ouId`, `externalIdentity`, `linkingCandidateUserIds`, `linkingVerificationRequested`, `ownerId`, `categoryType`, `defaultOUID` | Only when named in `runtimeDataKeys`. Exception: `applicationId` and `userID` are sent by default when present. |
| Flow data | Every key not on the reserved list, such as `extension.<nodeId>.<field>` keys stored by earlier extensions | Only when named in `runtimeDataKeys`. |

Without `runtimeDataKeys`, a service therefore receives only `applicationId` and `userID`. Data an earlier extension stored never reaches a later one unless an administrator names it on that later step, so one service cannot read another's answers by default.

### Extension executor: the call

- The HTTP client does not follow HTTP redirects. A 3xx response is evaluated like any other status. Following one would resend the request's headers, including its credential, to another host. This is separate from sending the user's browser to a service's page, which the response asks for explicitly (see *Redirect to the service's page*).
- The client uses the server's TLS settings.
- Private and loopback addresses are allowed, because endpoints are set only by administrators. This allows on-premise services.
- The response body is read up to 64 KiB (65 536 bytes). A larger body makes the outcome `ERROR` without being parsed.
- A timeout is not retried.

### Extension executor: response format

A service answers with HTTP status 2xx and a JSON object. This is what a service author implements:

| Field | When | Meaning |
|---|---|---|
| `actionStatus` | Always | `SUCCESS`, `FAILED`, `INCOMPLETE` or `ERROR`. Case-sensitive. |
| `failureReason` | With `FAILED` or `INCOMPLETE`, optional | A machine-readable code, up to 64 characters from `[A-Za-z0-9_.-]`. Passed to the client application as `params.reason`, except for a credential failure (see *Failure information and user messages*). |
| `failureDescription` | With `FAILED` or `INCOMPLETE`, optional | Text for the user, shown as plain text and cut to 256 characters. |
| `redirectUrl` | With `INCOMPLETE`, optional | An absolute `http` or `https` URL to send the user's browser to (see *Redirect to the service's page*). |
| Any other top-level field | Optional | Data for later steps, stored as text under `extension.<nodeId>.<field>` when allowed by `responseKeys`. A stored field must not use a reserved key (see *Response storage*). |

Allow the user through, and give later steps data:

```json
{
  "actionStatus": "SUCCESS",
  "riskScore": "12",
  "riskReason": "known_device"
}
```

Refuse, with a message for the user:

```json
{
  "actionStatus": "FAILED",
  "failureReason": "high_risk",
  "failureDescription": "Sign-in blocked. Contact support."
}
```

Ask for more input through the step's `onIncomplete` prompt:

```json
{
  "actionStatus": "INCOMPLETE",
  "failureReason": "otp_required",
  "failureDescription": "Enter the code we sent you."
}
```

Send the user to the service's own page, then answer `SUCCESS` or `FAILED` when the step calls again for the same `executionId`:

```json
{
  "actionStatus": "INCOMPLETE",
  "redirectUrl": "https://pin.example.com/entry?flowId=019a1b2c-3d4e-7f60-8a9b-0c1d2e3f4a5b"
}
```

Report that the service could not decide; the user sees only a generic error:

```json
{
  "actionStatus": "ERROR",
  "failureReason": "upstream_unavailable"
}
```

An answer that breaks these rules is treated as `ERROR`, as the next section sets out.

### Extension executor: response evaluation

The outcome is decided in this order:

1. The status code is outside 200 to 299: **`ERROR`**.
2. The body is empty, is not a JSON object, or exceeds the size cap: **`ERROR`**.
3. `actionStatus` is missing, is not a string, or is not one of the four values (the comparison is case-sensitive): **`ERROR`**.
4. `actionStatus` is `INCOMPLETE` and `redirectUrl` is present, not empty and not `null`, but is not an absolute `http` or `https` URL: **`ERROR`**.
5. Otherwise, the value of `actionStatus`.

| Outcome | Executor status | Flow continues at | Client error | Runtime data |
|---|---|---|---|---|
| `SUCCESS` | complete | `onSuccess` | none | Response fields stored; earlier `failureReason` and `failureDescription` reset to `""` |
| `FAILED` | failure | `onFailure`, or ends the flow with an error if none is set | `ErrExtensionActionFailed`, or `ErrInvalidCredentials` for a credential failure | `failureReason` and `failureDescription` stored |
| `INCOMPLETE` | user input required | `onIncomplete`, which must lead to a prompt; without `onIncomplete`, the flow ends with ThunderID's internal server error | `ErrExtensionInputRequired` when a message exists, or `ErrInvalidCredentials` for a credential failure | Response fields plus `failureReason` and `failureDescription` stored; nothing is stored when the step has no `onIncomplete` |
| `INCOMPLETE` with `redirectUrl` | external redirection | The user's browser goes to `redirectUrl`; the flow pauses on this step and runs it again when the browser returns | none | Response fields stored |
| `ERROR` | failure | `onFailure`, or ends the flow with an error | `ErrExtensionCallFailed` (generic) | Nothing from the answer is stored, and the step's namespace is cleared (see *Response storage*) |

On `ERROR`, nothing from the response reaches the client. `failureReason` appears only in the audit event (see *Audit event*); `failureDescription` is not recorded.

### Extension executor: redirect to the service's page

**Not built yet; requirements for the custom authentication increment.** Until then, the last rule in this section applies.

Some services collect input on their own page, such as a PIN pad or a document upload, and need the user's browser there. A service asks for that by answering `INCOMPLETE` with `redirectUrl`:

```json
{
  "actionStatus": "INCOMPLETE",
  "redirectUrl": "https://pin.example.com/entry?flowId=<executionId>"
}
```

- The step returns external redirection with `redirectUrl` as the redirect URL, as the federated authentication steps do. ThunderID's client SDK adds a random `state` to the URL and stores it.
- The service sends the browser back to the ThunderID gate's callback, `<gate>/callback?code=<value>&state=<the same state>`. The gate checks `state`, and the SDK resumes the flow with the stored execution ID and the input `code`.
- The step runs again. The new request carries the same `executionId`, so the service can return the outcome it recorded for that execution, which is evaluated as usual. `code` is a user input like any other: it is sent only when `inputKeys` names it, for a service that binds the return to it.
- `redirectUrl` must be an absolute `http` or `https` URL. `javascript:` and `data:` URLs would run in the sign-in page instead of leaving it, so they, relative URLs and non-string values make the outcome `ERROR`. An empty or `null` value means no redirect.
- Any `http` or `https` destination is accepted. The service is configured by an administrator, and services commonly host their page on another origin from their API.
- `redirectUrl` is part of the contract and is never stored as data. With any status other than `INCOMPLETE` it is ignored.
- **Binding the outcome to the person.** The outcome the step receives on its second call must describe the person who completed the service's page in the browser that left the flow, not merely the holder of the `executionId` (AC4.10). The custom authentication increment chooses the mechanism, for example always forwarding `code` and requiring the service to verify that it issued that `code` to the browser that completed its page, or a signed, short-lived token that names the user and the execution.
- **Binding the return to the browser.** Resuming after the redirect must require proof held only by the browser session that left, so that a copied execution ID and challenge value alone cannot resume it (AC4.11).
- Until custom authentication (redirect) is delivered, an `INCOMPLETE` with a non-empty `redirectUrl` is treated as `ERROR`, with `errorKind` `contract`.

### Extension executor: response storage

**Rendering.**

- Top-level response fields are stored as strings.
- Numbers keep their exact text, and booleans become `true` or `false`.
- Objects and arrays are stored as compact JSON.
- `null` values are skipped.
- `actionStatus`, `failureReason`, `failureDescription` and `redirectUrl` are never stored as data. On `FAILED` and `INCOMPLETE` the two failure fields are stored under their own names, as the table above describes.

**Namespace.** Every stored field is written under the step's own namespace, `extension.<nodeId>.<field>`, where `<nodeId>` is the step's node ID in the flow (the `currentNodeId` of the request) and `<field>` is the response field's name. A response field named `riskScore` from the step `risk_check` is stored as `extension.risk_check.riskScore` (AC6.1).

- A returned field therefore never collides with an engine key, a profile attribute or a credential attribute. A field named `email`, `password` or `pin` is stored as `extension.<nodeId>.email` and so on, and is never read as the user's attribute or credential (AC6.7).
- A later step reads extension data only when it names the namespaced key explicitly: a later extension's `runtimeDataKeys`, the resolver's `sourceKey`, or the resolver's `attributeMapping` for provisioning. The authentication assertion and provisioning never take a claim or credential from a namespaced key except through such a mapping.
- Each call replaces the step's namespace. After the request is built, which may send values from the step's own namespace when `runtimeDataKeys` names them, the step removes every key in its namespace and then stores the new answer, so the namespace always holds the latest answer only (AC6.8). This holds for every call that gets an HTTP response, `ERROR` included; a step that gets none (a configuration error, the attempt limit, an unresolvable connection, a timeout or an unreachable service) leaves the namespace as it was, so an earlier answer is not lost to a call the service never answered.
- Extension data is never copied into the SSO session. The session step leaves every `extension.` key, and the `failureReason` and `failureDescription` an extension stored, out of the session it saves, so a later sign-in at another application never receives it; that application's flow calls its own extension if it needs the data (AC6.6).

**Allowlist.** When `responseKeys` is set, only the fields it names are stored. Fields outside it are dropped without error, so upstream tokens never enter the flow.

**Reserved keys.**

- The executor keeps a single list of runtime-data keys that the engine or a built-in step relies on:
  - every `RuntimeKey*` constant in `internal/flow/common`;
  - the identity keys `userID`, `userId` and `ouId`;
  - `groups`, since authorization reads it before the user store;
  - `password`, so that a service cannot plant a credential;
  - `applicationId`;
  - `failureReasonJSON`;
  - `authorized_permissions`;
  - the keys each built-in executor defines for its own state, including `defaultOUID`, `categoryType`, `ownerId`, `idpId`, `senderId` and `passkeySessionToken`;
  - SSO checkpoint keys, matched by prefix because their names carry a checkpoint suffix.
- The namespace is the protection for profile and credential attributes; the reserved list stays as defence in depth for the engine's own keys and to make a misbehaving service visible, since a response that names an engine key shows the service is not following the contract.
- A unit test fails if any `RuntimeKey*` constant in `internal/flow/common`, or any runtime-key constant in `internal/flow/executor`, is missing from the list, so the list cannot fall behind as the engine grows.
- **A response that would store a reserved key is `ERROR`.** The check covers the fields the step stores on `SUCCESS` and `INCOMPLETE`: those `responseKeys` names, or every top-level field when it is empty. A field `responseKeys` leaves out, and the fields of a `FAILED` answer, are neither stored nor checked, so a service can return its own `userId` to a step that does not store it. Nothing from an `ERROR` response is stored.
- The proof of concept showed why this matters: the `{{userId}}` placeholder resolves from `runtimeData.userId` before the authenticated user, so a service that returned its own user ID under that name would be taken for the signed-in user by every later step that uses the placeholder.

### Extension executor: failure information and user messages

- **`failureReason`:**
  - passed as `params.reason`;
  - limited to 64 characters from `[A-Za-z0-9_.-]`;
  - a value outside that is dropped.
- **`failureDescription`:**
  - shown as the error description and passed as `params.description`;
  - control characters and Unicode format characters (category Cf, such as bidirectional overrides and zero-width characters) are removed, and whitespace runs collapsed;
  - cut to 256 characters;
  - always rendered as text, never as markup.
- **Defaults:** if `failureDescription` is missing or empty after cleaning:
  - `FAILED` uses "The request was rejected";
  - `INCOMPLETE` returns no error.
- **Credential failures:**
  - When `failureReason` is `invalid_credentials` or `user_not_found`, with `FAILED` or `INCOMPLETE`, the step returns ThunderID's own `ErrInvalidCredentials`, exactly as the built-in credential steps do, with no params, whatever the service sent.
  - The reason is kept only in runtime data and the audit event.
  - The same error for both cases, identical to a native credential failure, stops an extension from revealing which accounts exist.

### Extension executor: attempt limit

- The step counts its `FAILED` and `INCOMPLETE` answers in one execution. An `INCOMPLETE` that only sends the browser to the service's page is not counted, since it carries no decision; `ERROR` and `SUCCESS` are not counted.
- When the count has reached `maxAttempts`, every later time the flow reaches the step it fails with `ErrExtensionAttemptsExceeded` without calling the service: the flow continues at `onFailure`, as for the OTP step's attempt limit, or ends with that error if none is set (AC4.8).
- The count is executor state under a reserved key outside the `extension.` namespace, so no response can change it, and it is never sent.
- The limit bounds guessing within one execution only. A service that verifies a secret, such as a PIN or a one-time code, must also enforce its own lockout across executions; this is a requirement on the service, stated in the documentation.

### Extension executor: audit event

Every execution publishes one event, `EXTENSION_CALL`, on ThunderID's observability event bus, in the flows category, so that it reaches the deployment's configured outputs (console, file or OpenTelemetry) with the engine's own flow events. The executor receives the observability provider through its dependencies; when observability is disabled, no event is published. The event carries these data fields, reusing the bus's existing keys where one exists:

- `executionId`, `nodeId`, `flowType`;
- `connectionId`;
- `outcome`;
- `failureReason`, when the service sent one in the allowed form;
- `httpStatus`, when a response was received;
- the call's duration, as the bus's existing duration key;
- `errorKind`, for `ERROR`: `timeout`, `transport`, `status`, `body`, `contract`, `reserved_key` or `internal`. It is absent when the service itself answered `ERROR`.

The event never contains URLs, header values, request or response bodies, user inputs, runtime-data values or `failureDescription`. Configuration failures (`ErrExtensionConfigInvalid`) emit the same event with `outcome` set to `CONFIG_ERROR`, and a step that stops at its attempt limit emits it with `outcome` set to `ATTEMPTS_EXCEEDED` (AC8.4).

Observability is disabled in the default configuration, so a deployment publishes `EXTENSION_CALL` only after observability is enabled. The documentation says so in its production guidance.

### Extension executor: errors

Errors are named here by their constant. Numeric codes are assigned from the next free `FET-` and `CON-` ranges at implementation, so that other in-flight changes cannot collide with this spec.

| Error | When | Client sees |
|---|---|---|
| `ErrExtensionConfigInvalid` | No connection configured, or the connection does not exist | "Configuration error", generic description |
| `ErrExtensionCallFailed` | Outcome `ERROR` for any reason, including an unsafe `redirectUrl` and a connection that exists but cannot be resolved | "Extension call failed", generic description |
| `ErrExtensionActionFailed` | Outcome `FAILED` | "Request rejected"; description from `failureDescription`; `params.description`, `params.reason` |
| `ErrInvalidCredentials` (existing) | Outcome `FAILED` or `INCOMPLETE` with `failureReason` `invalid_credentials` or `user_not_found` | ThunderID's own invalid-credentials error, with no params |
| `ErrExtensionInputRequired` | Outcome `INCOMPLETE` with a message | "Additional input required"; `params.description`, `params.reason` |
| `ErrExtensionAttemptsExceeded` | The step reached `maxAttempts` in this execution | "Too many attempts", generic description |

### HTTP endpoint connection

A connection of type `http`. Fields and validation:

| Field | Rule | Error |
|---|---|---|
| `name` | Required, unique per deployment, at most 255 characters | `httpconnection.ErrorInvalidName`; `httpconnection.ErrorAlreadyExists` when duplicate |
| `description` | Optional, at most 500 characters | `httpconnection.ErrorInvalidDescription` |
| `url` | Required, absolute `http` or `https` URL with a valid host and a port from 1 to 65535, and no user name or password. Must be `https` when `authentication.type` is not `none`, so that a stored credential never crosses the network in clear. `http` remains for services that receive no credential, such as a local development service. Stored canonicalized: scheme and host in lower case, default port removed. The URL, including its query string, is not secret: it is stored and returned in clear, so a credential must not be put in it. | `httpconnection.ErrorInvalidURL` |
| `timeoutMs` | 1 to 20 000; `0` or omitted means 10 000 | `httpconnection.ErrorInvalidTimeout` |
| `headers` | `[{name, value}]`; names valid and unique (case-insensitive); values without line breaks. A PUT replaces the whole list, so omitted means none. Names that commonly carry credentials are refused: `Authorization`, `Proxy-Authorization`, `Cookie`, `X-Api-Key`, `Api-Key`, `X-Auth-Token` and `X-Access-Token`. Names the HTTP stack manages are refused: `Host`, `Content-Length`, `Content-Type`, `Transfer-Encoding`, `Connection`, `TE`, `Trailer`, `Upgrade`, `Keep-Alive` and `Expect`. A name may not repeat a header the authentication method sets. A credential belongs only in `authentication`, where it is encrypted and masked; plain header values are stored, returned and exported in clear. | `httpconnection.ErrorInvalidHeaders` |
| `authentication` | Optional outbound authentication object, the shared one other connections use (`internal/system/outboundauth`): `{"type": "none" \| "basic" \| "bearer" \| "api_key", "properties": {...}}`. Omitted means `none`, on update as much as on create. `basic` requires `username` and `password`; for HTTP the username may not contain a colon, since HTTP Basic joins the two with one (RFC 7617). `bearer` requires `token`. `api_key` takes one or more header name and value pairs, each value a secret, validated by the shared HTTP binding. | `httpconnection.ErrorInvalidAuthentication`; `connection.ErrorInvalidAuthenticationType` when the type is not one the server implements |

Authentication reuses the shared outbound authentication model and its HTTP binding (`outboundauth/httpauth`) rather than defining its own, so every connection that dials out with credentials stores, masks and validates them the same way, and the console renders the methods the server reports through `GET /connections/meta`. HTTP endpoint connections offer `none`, `basic`, `bearer` and `api_key` (AC2.9).

**Secrets** are the outbound authentication's credential fields: the `basic` password, the `bearer` token and every `api_key` header value.

- **Storage:** encrypted with the server's configuration key and stored in the connection's properties, never in plain text.
- **Reads:** every read returns each secret as `******`. The basic username and the `api_key` header names are returned in clear.
- **Updates** follow the shared rule for outbound credentials. A secret that is omitted keeps its stored value only while the credential's target is unchanged: the same authentication type, the same non-secret properties (the username) and the same URL **origin** (scheme, host and port, with default ports normalized). Otherwise the stored secret is dropped and must be supplied again, or the update fails with `httpconnection.ErrorInvalidAuthentication`. This stops an editor from pointing a connection at their own server, or changing the user it signs in as, and collecting its stored password.
- **The mask is never stored.** A `basic` password or `bearer` token sent as `******` is refused with `httpconnection.ErrorInvalidAuthentication`; a client omits a secret to keep it (AC2.11). An `api_key` header value sent as `******` on update keeps the stored value of that header, as the shared HTTP binding defines, under the same retention rule; on create it is refused.

**Resolution for calls.** Plain headers first, then the outbound authentication's HTTP binding adds the credential: `Authorization: Basic base64(username:password)` for `basic`, `Authorization: Bearer <token>` for `bearer`, and each configured header for `api_key`. Decrypted values exist only in the outgoing request, for the duration of the call.

**Lifecycle.**

- Delete is refused with `httpconnection.ErrorHasBlockingDependencies` while any flow references the connection through `connectionId`. Flow management reports those references through the dependency registry. A failure to determine usages also refuses the delete.
- Connections defined in declarative YAML are read-only (update and delete fail as declarative resources). With `http_connection.store` set to `declarative`, a create is refused the same way, after the request is validated, so an invalid request gets its validation error.
- Export writes each secret as a `.env` parameter, as for SMTP connections, including every `api_key` header value. An `api_key` header name containing a dot, or two header names that derive the same parameter name, cannot be written as distinct parameters, so the export refuses that connection with an error naming it and the headers, never a value (AC2.13).
- Import accepts the same shape. A document for an existing connection is applied as an update under the same retention rule: an omitted secret is kept only for the same target, and an `api_key` header sent as `******` keeps its stored value. A document for a new connection must carry every secret. A dry run applies the same rules without writing. Header names from imported and declared connections are stored in canonical form. A gateway import does not yet accept a `sec:` reference at an `api_key` header value (#6018).
- Where a copy of a secret is kept for export references to resolve, deleting the connection removes it, and dropping the stored secret under the retention rule removes or replaces it (AC2.12). The copy lives in the default gateway's variable store and is written only on the control plane. API update and delete, MCP updates, import upserts and prunes all release it. A name the connection still refers to, including through a `sec:` reference, is kept; on delete, a reference to the connection's own derived name is released too. Release is best effort and logged: a gateway that has stopped being the default keeps its copy, and concurrent updates of one connection can leave the copies of the earlier write.

### Extension auth resolver

**Not built yet; requirements for the custom authentication increment.**

| Property | Default | Meaning |
|---|---|---|
| `sourceKey` | required | The namespaced key holding the identifier the service verified, `extension.<nodeId>.<field>`, where `<nodeId>` is the extension step that routes to this resolver through `onSuccess`. |
| `matchAttribute` | `username` | The local user attribute to match. `userID` matches by entity ID. |
| `allowAuthenticationWithoutLocalUser` | `false` | When no local user matches, continue so that a provisioning step can create the user. Same name and meaning as on the federated authentication steps. |
| `attributeMapping` | empty | For provisioning: local attribute name to namespaced key, for example `email` to `extension.pin_check.email`. Only these attributes are taken from the extension's answer; a credential attribute cannot be mapped. |

Steps:

1. Read `runtimeData[sourceKey]`. If it's missing or empty, fail with "user not authenticated".
2. Resolve the entity:
   - by ID when `matchAttribute` is `userID`;
   - otherwise through the entity provider, filtered on `matchAttribute`.
   - Not found fails with `ErrEntityNotFound`; more than one match fails with `ErrAmbiguousEntityIdentity`.
   - Not found, in an authentication flow with `allowAuthenticationWithoutLocalUser`, instead sets `runtimeData.userEligibleForProvisioning` to `"true"`, authenticates no one and completes. Only an attribute lookup that matches no entity at all qualifies. A match that is not a user, an ambiguous match and a lookup by `userID` still fail, since a new user could not satisfy them.
3. Load the entity. It must be of category `user` and in state `ACTIVE`. Anything else fails with `ErrEntityNotFound`, the same answer credential authentication gives, so a caller cannot use the resolver to tell which accounts exist.
4. Authenticate through `AuthenticateUser`, with the credential `extensionVerifiedEntityID: <id>`. Failure fails with `ErrEntityAuthFailed`.
5. Complete, and copy the authenticated claims into runtime data.

A missing `sourceKey` property fails with `ErrExtensionAuthResolverConfigInvalid` (configuration invalid).

**Constraints** (AC9.12 to AC9.16):

- **Source.** Flow save refuses a resolver whose `sourceKey` is not in the namespace of the extension step that routes to it through `onSuccess`. Since each call replaces that namespace, the value the resolver reads is the one that extension returned with `SUCCESS` in this pass, never an earlier answer or another step's data.
- **Allowed users.** The resolver signs in only a user the application allows: of an allowed user type and in an organization unit the application serves. Any other match fails as not found.
- **Match attribute.** `matchAttribute` is `userID`, `username`, or an attribute users cannot change through self-service. Flow save refuses any other attribute, so a user cannot set their own attribute to the value a service verifies for someone else.
- **Internal credential types.** Flow save refuses a flow whose steps declare `extensionVerifiedEntityID`, or another internal credential type, as an input, and the credentials step refuses such a type at run time, so the type is reachable only from the resolver.
- **Record.** A sign-in through the resolver publishes an authentication event naming the resolver, and the assertion claims no authentication method or assurance level stronger than the flow recorded.

**Placement.** In a custom authentication flow the order is extension, then resolver, then the authorization step:

```text
Prompt → Extension (onSuccess) → Extension auth resolver → Authorization → Assertion
```

The authorization step requires an authenticated user and fails with `ErrUserNotAuthenticated` without one. The extension executor never authenticates a user, so an extension followed directly by authorization fails there. The resolver is the step that resolves the local user and authenticates them, and authorization then evaluates that user's groups, roles and permissions as it does after any built-in authenticator.

### Custom authentication flow patterns

The proof of concept ran each of these against a PIN service that collects the PIN on its own page, through an HTTP endpoint connection named *PIN Authenticator* with authentication `none`. Step names are the flow builder's labels, with the executor name in brackets. Every step not listed as a route target ends the flow with its error.

**Common to all three.** The service answers the first call for an `executionId` with `INCOMPLETE` and `redirectUrl` (its PIN page). Its page sends the browser to the gate's `/callback` with `code` and the `state` the SDK added. The step then calls again with the same `executionId`, and the service answers `SUCCESS` or `FAILED` once.

The sample responses below are the PIN service's answers from the proof of concept run, with user IDs and addresses replaced by example values. The service's user ID field is shown under the names this specification requires, since the run predates the reserved-key rule.

First call, in every pattern:

```json
{
  "actionStatus": "INCOMPLETE",
  "redirectUrl": "http://localhost:3000/api/pin-entry?flowId=019a1b2c-3d4e-7f60-8a9b-0c1d2e3f4a5b"
}
```

Second call, when the PIN did not match, in every pattern:

```json
{
  "actionStatus": "FAILED",
  "failureReason": "userNotFound",
  "failureDescription": "Unable to find user for given credentials."
}
```

#### Pattern 1: users who exist in ThunderID

| # | Step | Configuration | Routes |
|---|---|---|---|
| 1 | Extension (`ExtensionExecutor`), node `pin_check` | Connection: PIN Authenticator. Inputs to send: none. Stored response fields: `verifiedUserId` | `onSuccess` → 2; `onFailure` → a prompt showing the failure |
| 2 | Extension Auth Resolver (`ExtensionAuthResolver`) | Source key: `extension.pin_check.verifiedUserId`. Match attribute: `userID`. Allow authentication without a local user: off | `onSuccess` → 3; `onFailure` → a prompt showing the failure |
| 3 | Authorization (`AuthorizationExecutor`) | Defaults | `onSuccess` → 4 |
| 4 | Auth Assertion Generator (`AuthAssertExecutor`) | Defaults | `onSuccess` → End |

- The service reads `executionId` and `application.ouId`, and answers `{"actionStatus":"SUCCESS","verifiedUserId":"<ThunderID user ID>"}`. It must know each user's ThunderID ID, or answer a username and use Match attribute `username` instead.
- The identifier must not use a reserved key such as `userID` or `userId`: a response that would store one is `ERROR` (see *Response storage*).

Second call, when the PIN matched:

```json
{
  "actionStatus": "SUCCESS",
  "verifiedUserId": "cea777f1-0000-4000-8000-000000000001",
  "username": "emily@example.com",
  "email": "emily@example.com",
  "given_name": "Emily",
  "family_name": "Ellon"
}
```

Only `extension.pin_check.verifiedUserId` is stored, because Stored response fields names only `verifiedUserId`.

#### Pattern 2: users who do not exist in ThunderID yet

| # | Step | Configuration | Routes |
|---|---|---|---|
| 1 | Extension (`ExtensionExecutor`), node `pin_check` | Connection: PIN Authenticator. Inputs to send: none. Stored response fields: `username`, `email`, `given_name`, `family_name` | `onSuccess` → 2; `onFailure` → a prompt showing the failure |
| 2 | Extension Auth Resolver (`ExtensionAuthResolver`) | Source key: `extension.pin_check.username`. Match attribute: `username`. Allow authentication without a local user: **on**. Attribute mapping: `username`, `email`, `given_name` and `family_name` to the keys of the same field names under `extension.pin_check.` | `onSuccess` → 3; `onFailure` → a prompt showing the failure |
| 3 | Provision User (`ProvisioningExecutor`, mode `user`) | Defaults. Optionally Assign group or Assign role | `onSuccess` → 4 |
| 4 | Authorization (`AuthorizationExecutor`) | Defaults | `onSuccess` → 5 |
| 5 | Auth Assertion Generator (`AuthAssertExecutor`) | Defaults | `onSuccess` → End |

- The service answers `SUCCESS` with flat attribute fields, for example `{"actionStatus":"SUCCESS","username":"…","email":"…","given_name":"…","family_name":"…"}`.
- The application needs an allowed user type whose schema has `username`. Provisioning takes attributes from the extension only through the resolver's attribute mapping, and prompts for any required attribute missing.
- On a user's first sign-in the resolver marks them for provisioning and Provision User creates and signs them in. On later sign-ins the resolver signs in the existing user and provisioning skips. Either way it is one sign-in.
- The service must not return its own user ID under a reserved key such as `userId`. A response that would store a reserved key is `ERROR`; with Stored response fields empty, every field is stored. Use a name of its own, such as `serviceUserId`, if later steps need it.

Second call, when the PIN matched:

```json
{
  "actionStatus": "SUCCESS",
  "serviceUserId": "8a2b49b5-0000-4000-8000-000000000002",
  "username": "peter@example.com",
  "email": "peter@example.com",
  "given_name": "Peter",
  "family_name": "Parker"
}
```

`serviceUserId` is not stored, because Stored response fields leaves it out. On the first sign-in, Provision User creates `peter@example.com` from the four mapped fields.

#### Pattern 3: second factor after password

| # | Step | Configuration | Routes |
|---|---|---|---|
| 1 | Prompt | Inputs `username` and `password`, with a submit action | → 2 |
| 2 | Identifier + Password (`CredentialsAuthExecutor`) | Defaults | `onSuccess` → 3; `onFailure` → 1 |
| 3 | Extension (`ExtensionExecutor`) | Connection: PIN Authenticator. Inputs to send: none. Runtime data to send: default | `onSuccess` → 4; `onFailure` → a prompt showing the failure |
| 4 | Authorization (`AuthorizationExecutor`) | Defaults | `onSuccess` → 5 |
| 5 | Auth Assertion Generator (`AuthAssertExecutor`) | Defaults | `onSuccess` → End |

- No resolver and no provisioning. Step 2 signs the user in; the extension's `SUCCESS` only lets the flow continue, and the extension executor cannot change the user, so the service cannot switch identities.
- The service reads the signed-in user from `runtimeData.userID`, which the default runtime-data selection sends, or from `authUser.<provider>.entityReference.entityId`, and checks the PIN of that user. With no signed-in user it answers `ERROR`.
- The password is not sent: Inputs to send names nothing, and the engine has already cleared it.

Second call, when the PIN matched for the signed-in user:

```json
{
  "actionStatus": "SUCCESS",
  "verifiedUserId": "cea777f1-0000-4000-8000-000000000001",
  "username": "emily@example.com",
  "email": "emily@example.com",
  "given_name": "Emily",
  "family_name": "Ellon"
}
```

The returned fields are stored under the step's namespace, which no later step reads unless it names those keys, so they never change the signed-in user's claims; the flow continues with the user signed in at step 2. Route `onFailure` of a second-factor extension to a path that denies, since `FAILED` and `ERROR` both continue there. When no user is signed in, the service refuses the first call with HTTP `400`, which the step treats as `ERROR`:

```json
{
  "actionStatus": "ERROR",
  "errorMessage": "missingUser",
  "errorDescription": "No authenticated user for second-factor authentication."
}
```

In every pattern ThunderID records neither the method nor the extra factor; that is the assurance work listed as out of scope.

### Internal credential type `extensionVerifiedEntityID`

- The default authn provider accepts `extensionVerifiedEntityID` with a non-empty user ID and returns that ID as the authenticated entity, as it already does for `provisionedEntityID`.
- The type is on `InternalCredentialTypes`, so the credentials authentication API rejects it from API clients. The existing test that every dispatched credential type is on that list covers it. Flows cannot declare it as an input either (see *Extension auth resolver*, Constraints).

### Data model

A new table in the configuration database.

PostgreSQL:

```sql
CREATE TABLE "HTTP_CONNECTION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(36)  NOT NULL,
    NAME          VARCHAR(255) NOT NULL,
    DESCRIPTION   VARCHAR(500),
    PROPERTIES    JSONB        NOT NULL,
    CREATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    UPDATED_AT    TIMESTAMPTZ  DEFAULT NOW(),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);
CREATE UNIQUE INDEX idx_http_connection_name_deployment ON "HTTP_CONNECTION" (DEPLOYMENT_ID, NAME);
```

SQLite:

```sql
CREATE TABLE "HTTP_CONNECTION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(36)  NOT NULL,
    NAME          VARCHAR(255) NOT NULL,
    DESCRIPTION   VARCHAR(500),
    PROPERTIES    TEXT         NOT NULL,
    CREATED_AT    TEXT         DEFAULT (datetime('now')),
    UPDATED_AT    TEXT         DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);
CREATE UNIQUE INDEX idx_http_connection_name_deployment ON "HTTP_CONNECTION" (DEPLOYMENT_ID, NAME);
```

- `PROPERTIES` is a JSON object `{url, timeoutMs, headers, authentication}`. `authentication` is the property array the shared outbound authentication model writes: the authentication type, the username, and each secret (the `basic` password, the `bearer` token and every `api_key` header value) marked secret and encrypted. It is omitted when the connection has no authentication. No other secret is stored.
- The table is added to the SQLite and Postgres configuration scripts. There is no migration framework, so the release notes carry the DDL for existing deployments.
- No other table changes.

### API

All routes require the same authorization as the other connection routes.

| Method and path | Request | Response |
|---|---|---|
| `GET /connections/http` | none | `200`, a list of connection summaries; `400 httpconnection.ErrorResultLimitExceededInCompositeMode` in composite mode when the stored and declared connections together exceed the list limit |
| `POST /connections/http` | `HTTPConnectionCreateRequest` | `201`, `HTTPConnectionResponse` |
| `GET /connections/http/{id}` | none | `200`, `HTTPConnectionResponse`; `404 httpconnection.ErrorNotFound` |
| `PUT /connections/http/{id}` | `HTTPConnectionUpdateRequest` | `200`; `400`, one of the validation errors in the field table above; `404`; `409 httpconnection.ErrorAlreadyExists` |
| `DELETE /connections/http/{id}` | none | `204`; `404`; `409 httpconnection.ErrorHasBlockingDependencies` |
| `GET /connections/http/{id}/usages` | none | `200`, `{totalResults, summary, usages[]}` |
| `GET /connections?category=http` | none | `200`, the standard paginated list |

The request and response bodies have the fields in *HTTP endpoint connection*. Responses carry `id` and `type: "http"`, with secrets masked. `api/connections.yaml` defines these schemas, the `http` category and the `http` type.

### MCP tools

ThunderID's MCP server offers HTTP endpoint connections to agents through four tools (R11):

| Tool | Equivalent API call |
|---|---|
| `thunderid_list_http_connections` | `GET /connections/http` |
| `thunderid_get_http_connection_by_id` | `GET /connections/http/{id}` |
| `thunderid_create_http_connection` | `POST /connections/http` |
| `thunderid_update_http_connection` | `PUT /connections/http/{id}` |

- Each tool goes through the same request mapping and service as its API call, so validation, the `https` rule, refused header names, the retention rule, the refusal of the mask and secret masking are identical.
- There is no delete tool, as for the other resources the MCP server exposes.
- Tools require the same authorization as the connection routes.

### UI

**Connections, create.** "Service Endpoint" in the custom section of the wizard; the API resource stays `/connections/http`, type `http`. The steps are name, then URL and authentication, which offers the same methods as on the detail page and defaults to `none`. Timeout and headers use defaults on create and can be edited afterwards.

**Connections, detail page:**

```text
Service Endpoint · Risk Engine                                [Delete]
┌ General ───────────────────────────────────────────────────────────┐
│ URL      [https://risk.example.com/evaluate                      ] │
│ Timeout  [5000] ms                                                  │
│ Headers  [X-Tenant      ] [acme          ] ×                        │
│          +                                                          │
└────────────────────────────────────────────────────────────────────┘
┌ Authentication ────────────────────────────────────────────────────┐
│ Method   [Username and Password ▾]                                  │
│ Username [svc-extension    ]                                        │
│ Password [••••••••]  (Replace)                                      │
│ Changing the URL host requires re-entering the password.            │
└────────────────────────────────────────────────────────────────────┘
┌ Usages ────────────────────────────────────────────────────────────┐
│ Flow  Default sign-in                                               │
└────────────────────────────────────────────────────────────────────┘
```

When the URL's origin is edited, the Save button stays disabled until every secret field of the current method holds a new value.

**Flow builder, extension step:**

```text
Extension
 Connection        [Risk Engine ▾]                   ← required
 Inputs to send    [username ×] [password ×]  +     ← user inputs; none by default
 Runtime data      [extension.risk_check.riskTier ×]  +  ← application and user ID by default
 Stored fields     [riskScore, challengeId        ]  ← stored under extension.<node ID>.
 Max attempts      [5]
 The service must answer with actionStatus SUCCESS, FAILED, INCOMPLETE or ERROR.
```


**Flow builder, extension auth resolver:** Source key (required, chosen from the stored fields of the extension that leads to it), Match attribute (default `username`), Allow authentication without a local user (off by default) and Attribute mapping.

### Configuration

| Key | Level | Default | Meaning |
|---|---|---|---|
| `http_connection.store` | deployment | `composite` | `mutable`, `declarative` or `composite` storage for HTTP connections, as for other connection types. `composite` matches the default of the other resource types, so HTTP connection files load alongside identity provider files |

The connection's timeout bounds and the 64 KiB cap are constants, not configuration.

## Requirements

### R1. An administrator can call an organization's service from any flow

**Requirement:** An administrator can add an extension step to any flow, pointing at an HTTP endpoint connection, without a server restart.

**Acceptance criteria:**

- **AC1.1:** Given a flow with an extension step that references a connection, when the flow reaches the step, then ThunderID POSTs to the connection's URL with the connection's headers and timeout.
- **AC1.2:** Given a flow whose extension step has no `connectionId`, when the flow is saved, then the save is refused.
- **AC1.3:** Given an extension step that references a deleted connection, when the flow reaches it, then the step fails with `ErrExtensionConfigInvalid` and no request is sent.
- **AC1.4:** Given a flow whose extension step names a `connectionId` that does not exist, when the flow is saved, then the save is refused; the same ID padded with whitespace is accepted only when the trimmed ID exists.
- **AC1.5:** Given an extension step whose `inputKeys`, `runtimeDataKeys` or `responseKeys` is an empty list, contains a blank name or contains a non-string value, or whose `maxAttempts` is outside 1 to 20, when the flow is saved, then the save is refused.
- **AC1.6:** Given such a value that reaches run time, for example through an older flow, when the flow reaches the step, then the step fails with `ErrExtensionConfigInvalid`, no request is sent, and no field is stored.

### R2. Endpoints are reusable and secrets are protected

**Requirement:** An administrator defines a service endpoint once, as an HTTP endpoint connection, with back-channel credentials that ThunderID never discloses.

**Acceptance criteria:**

- **AC2.1:** Given a connection with `basic` authentication, when it is read through any API, then every secret value is `******`, and the stored record contains no secret in plain text.
- **AC2.2:** Given a stored connection, when it is updated with the password omitted, the same authentication type, the same username and the same URL origin, then the next call carries the original secret.
- **AC2.3:** Given a stored connection with `basic` authentication, when its username changes and the password is omitted, then the update fails with `400 httpconnection.ErrorInvalidAuthentication` and the connection is unchanged.
- **AC2.4:** Given a stored connection with authentication other than `none`, when its URL host, port or scheme changes and the password is omitted, then the update fails with `400 httpconnection.ErrorInvalidAuthentication`.
- **AC2.5:** Given a connection that is updated, when a flow next calls it, then the new URL, headers and secrets are used without editing the flow.
- **AC2.6:** Given a connection, when it is exported, then each secret appears as a `.env` parameter and not in the YAML.
- **AC2.7:** Given a connection with authentication other than `none`, when it is created or updated with an `http` URL, then the request fails with `400 httpconnection.ErrorInvalidURL`; with `none`, an `http` URL is accepted.
- **AC2.8:** Given a connection, when it is created or updated with a plain header named `Authorization`, `Cookie`, `Host` or another refused name, in any case, then the request fails with `400 httpconnection.ErrorInvalidHeaders`.
- **AC2.9:** Given a connection created with `bearer` or `api_key` authentication, when it is read, exported and called, then each secret is masked on read, appears as a `.env` parameter on export, and is sent as `Authorization: Bearer <token>` or as each configured API-key header on the call.
- **AC2.10:** Given a connection, when it is created or updated with a plain header named `X-Api-Key`, `api-key`, `X-Auth-Token` or `X-Access-Token`, in any case, then the request fails with `400 httpconnection.ErrorInvalidHeaders`.
- **AC2.11:** Given a connection with `basic` or `bearer` authentication, when it is created or updated with the password or token set to `******`, then the request fails with `400 httpconnection.ErrorInvalidAuthentication` and the stored secret is unchanged.
- **AC2.12:** Given a connection whose secret is kept for export references, when the connection is deleted, or an update drops its secret under the retention rule, then no copy of the old secret remains for references to resolve.
- **AC2.13:** Given a connection with an `api_key` header name containing a dot, or two header names that derive the same parameter name, when it is exported, then the export refuses that connection and no secret appears in the YAML.

### R3. Connections in use cannot be deleted

**Requirement:** An administrator can see which flows use a connection, and cannot delete a connection that a flow uses.

**Acceptance criteria:**

- **AC3.1:** Given a flow whose extension step references a connection, when its usages are requested, then the flow is listed.
- **AC3.2:** Given that reference, when the connection is deleted, then the request fails with `409 httpconnection.ErrorHasBlockingDependencies`.
- **AC3.3:** Given no references, when the connection is deleted, then it returns `204`, and a later read returns `404 httpconnection.ErrorNotFound`.
- **AC3.4:** Given a flow whose step names the connection's ID with surrounding whitespace, when the connection's usages are requested or the connection is deleted, then the flow is counted as a usage.

### R4. A service answers with a fixed contract and the flow routes on it

**Requirement:** A service reports its decision with `actionStatus` (`SUCCESS`, `FAILED`, `INCOMPLETE`, `ERROR`), and the flow continues at the matching path. With `INCOMPLETE` it can send the user's browser to its own page and receive the outcome when the user returns.

**Acceptance criteria:**

- **AC4.1:** Given `SUCCESS`, when the step completes, then the flow continues at `onSuccess`.
- **AC4.2:** Given `FAILED` with `failureReason` and `failureDescription`, when the step has `onFailure`, then the flow continues there. The client receives `ErrExtensionActionFailed` with the description and `params.reason`.
- **AC4.3:** Given `INCOMPLETE` with a `failureDescription`, when the step has `onIncomplete` leading to a prompt, then that prompt is shown with `ErrExtensionInputRequired` and the description.
- **AC4.4:** Given `ERROR`, a missing, non-string or unknown `actionStatus`, a non-2xx status, a 3xx redirect, an empty or non-object body, a body over 64 KiB, a timeout, or an unreachable service, when the step evaluates the answer, then the outcome is `ERROR` with `ErrExtensionCallFailed`. The client response contains nothing from the service's body, and a redirect target is never called.
- **AC4.5:** Given `INCOMPLETE` with an absolute `https` `redirectUrl` (custom authentication increment), when the step evaluates it, then the client receives a redirection to that URL, and when the browser returns through the gate callback, the step calls the service again with the same `executionId`, and with `userInputs.code` only when `inputKeys` names `code`.
- **AC4.6:** Given `INCOMPLETE` with a `redirectUrl` that is `javascript:`, `data:`, relative, scheme-less or not a string, when the step evaluates it, then the outcome is `ERROR` and no redirection is returned.
- **AC4.7:** Given any response carrying `redirectUrl`, when the step stores data, then `redirectUrl` is not stored.
- **AC4.8:** Given an extension step with `maxAttempts: 3` that a flow reaches repeatedly in one execution, when the service has answered `FAILED` or `INCOMPLETE` three times, then the next time the flow reaches the step it fails with `ErrExtensionAttemptsExceeded`, continuing at `onFailure` or ending the flow if none is set, and the service is not called, then or on any later arrival in that execution.
- **AC4.9:** Given a step with no `maxAttempts`, when the service has answered `FAILED` five times in one execution, then the next arrival fails with `ErrExtensionAttemptsExceeded` without calling the service.
- **AC4.10:** Given a redirect to the service's page (custom authentication increment), when the person who completes the page is not the person whose browser started the flow, then the step does not receive that person's outcome as the outcome of this execution.
- **AC4.11:** Given a redirect to the service's page (custom authentication increment), when a different browser presents the execution ID and the latest challenge value, then the flow does not resume.

### R5. Only what the administrator selects is sent

**Requirement:** An extension receives the contract version and the safe parts of the step's flow context, the user inputs the administrator names, and runtime data within the configured selection. Secret engine state, client secrets and provider tokens are never sent.

**Acceptance criteria:**

- **AC5.1:** Given an extension step with no `inputKeys`, when it calls the service, then `userInputs` is `{}`, even when the flow holds a password.
- **AC5.2:** Given `inputKeys: [username, password]`, when it calls the service, then `userInputs` holds exactly those two inputs.
- **AC5.3:** Given runtime data holding `otpSessionToken` and `oauthState`, when any extension step calls the service, even with both named in `runtimeDataKeys`, then neither key is sent.
- **AC5.4:** Given no `runtimeDataKeys`, and runtime data holding `applicationId`, `userID`, `clientId` and `extension.risk_check.riskTier` (stored by an earlier extension), when the step calls the service, then `runtimeData` holds `applicationId` and `userID` only.
- **AC5.5:** Given `runtimeDataKeys: [clientId, extension.risk_check.riskScore]`, when the step calls the service, then `runtimeData` holds only those keys that are present.
- **AC5.6:** Given any call, when the request is sent, then the body carries `"version": "1"`.
- **AC5.7:** Given any call, when the request is sent, then the body carries `executionId`, `flowType`, `currentNodeId`, and `application` with only `id`, `ouId`, `name` and `type`.
- **AC5.8:** Given an application with an OAuth client secret and an authenticated user with provider tokens and attributes, when the step calls the service, then the body contains neither the secret, the tokens nor the attributes, and `authUser` carries each provider's `entityReference` only.

### R6. Returned data is stored safely

**Requirement:** Data a service returns is available to later steps, within an allowlist and under the step's own namespace, and can never overwrite state the engine trusts, change a user's attributes or credentials, or outlive the flow.

**Acceptance criteria:**

- **AC6.1:** Given a step with node ID `risk_check` and `SUCCESS` with fields `risk` and `claims`, when the step completes, then a later step's `runtimeData` holds `extension.risk_check.risk` as text and `extension.risk_check.claims` as compact JSON. `actionStatus` is not stored, and no key named `risk` or `claims` is added.
- **AC6.2:** Given `responseKeys: [username]` and a response that also carries `accessToken`, when the step completes, then only `extension.<nodeId>.username` is stored.
- **AC6.3:** Given a `SUCCESS` or `INCOMPLETE` response containing a reserved key (for example `mapped_role_ids` or `userID`) as a top-level field, when `responseKeys` is empty or names that key, then the outcome is `ERROR` and nothing from the response is stored; when `responseKeys` leaves it out, then the step continues and the field is not stored.
- **AC6.4:** Given a reserved-key list in the executor, when a `RuntimeKey*` constant in `internal/flow/common` or a runtime-key constant in `internal/flow/executor` is missing from it, then the unit test suite fails.
- **AC6.5:** Given `FAILED` followed in the same execution by `SUCCESS`, when a later step reads runtime data, then `failureReason` and `failureDescription` are empty.
- **AC6.6:** Given an extension that stores fields before the session step, when another application later joins the same SSO session, then that application's flow and assertion receive no `extension.` key, `failureReason` or `failureDescription` from the earlier sign-in.
- **AC6.7:** Given a user signed in by password, followed by an extension that answers `SUCCESS` with `email` and `pin`, when the assertion is generated and when provisioning runs, then the assertion carries the user's stored `email`, and no credential or attribute is taken from the answer.
- **AC6.8:** Given an extension that answered `INCOMPLETE` with `challengeId` and then `SUCCESS` without it, when a later step reads runtime data, then `extension.<nodeId>.challengeId` is absent.

### R7. User-facing failure text is bounded and cannot reveal accounts

**Requirement:** Text a service sends for users is plain and bounded, and a credential failure returns ThunderID's own invalid-credentials error.

**Acceptance criteria:**

- **AC7.1:** Given a `failureDescription` of 300 characters containing control characters, when it is shown, then the user sees at most 256 characters, with no control characters.
- **AC7.2:** Given `FAILED` with `failureReason` `user_not_found`, and separately with `invalid_credentials`, when each is returned, then both return the same `ErrInvalidCredentials`, without `params.reason`.
- **AC7.3:** Given `FAILED` with no `failureDescription`, when it is shown, then the description is "The request was rejected".
- **AC7.4:** Given a `failureReason` containing characters outside `[A-Za-z0-9_.-]` or longer than 64 characters, when the error is returned, then `params.reason` is absent.
- **AC7.5:** Given a `failureDescription` containing a right-to-left override or a zero-width space, when it is shown, then neither character is present.

### R8. Every call is audited without sensitive data

**Requirement:** When observability is enabled, each extension execution leaves one record an operator can use to tell a refusal from a broken service.

**Acceptance criteria:**

- **AC8.1:** Given observability is enabled, when any execution ends, then exactly one `EXTENSION_CALL` event is published in the flows category with `executionId`, `nodeId`, `connectionId`, `outcome`, the duration, and `httpStatus` when a response was received.
- **AC8.2:** Given an `ERROR` caused by a timeout, when the event is emitted, then `errorKind` is `timeout`.
- **AC8.3:** Given any execution, when the event is emitted, then it contains no URL, header value, request or response body, user input, runtime-data value or `failureDescription`.
- **AC8.4:** Given observability is enabled and a step that stops at its attempt limit, when the flow ends, then one `EXTENSION_CALL` event is published with `outcome` `ATTEMPTS_EXCEEDED`.

### R9. A service can authenticate a user

**Requirement:** An administrator can let a service perform authentication, so that the local user the service verified is resolved before authorization, and the flow ends with a normal session for that user.

**Acceptance criteria:**

- **AC9.1:** Given an extension with node ID `pin_check` that answers `SUCCESS` with `authenticatedUsername: jdoe`, followed by a resolver with `sourceKey: extension.pin_check.authenticatedUsername`, when the flow completes, then the assertion's subject is the local user `jdoe`, even though the user's stored password was never checked.
- **AC9.2:** Given the same flow, when the service answers `FAILED`, then no user is authenticated, and the flow follows `onFailure`.
- **AC9.3:** Given a resolver whose source value is missing, matches no user, matches more than one, matches a non-user entity, or matches a user that is not `ACTIVE`, when it runs, then it fails and authenticates no one. The not-found, non-user and inactive cases give the same error.
- **AC9.4:** Given the credentials authentication API, when a client sends credential type `extensionVerifiedEntityID`, then the request is rejected.
- **AC9.5:** Given a resolver with no `sourceKey` property, when it runs, then it fails with `ErrExtensionAuthResolverConfigInvalid`.
- **AC9.6:** Given an extension, a resolver and then an authorization step, when the flow runs, then authorization evaluates the resolved user's groups and roles, and the assertion carries the permissions granted to that user.
- **AC9.7:** Given an extension followed directly by an authorization step, with no resolver, when the service answers `SUCCESS`, then the authorization step fails with `ErrUserNotAuthenticated` and no session is created.
- **AC9.8:** Given a resolver with `allowAuthenticationWithoutLocalUser` and an `attributeMapping`, followed by Provision User, when the service answers `SUCCESS` for a username that no local user has, then one sign-in creates that user from the mapped attributes only and authenticates them.
- **AC9.9:** Given the same flow and a username that a local user already has, when the user signs in, then the resolver authenticates that user and provisioning creates nothing.
- **AC9.10:** Given `allowAuthenticationWithoutLocalUser`, when the identifier matches a non-user entity, matches more than one entity, or is looked up by `userID`, then the resolver fails and does not mark the user for provisioning.
- **AC9.11:** Given a user signed in by password, followed by an extension, when the service answers `SUCCESS`, then the flow continues with the same user, and the request carried that user's ID in `runtimeData.userID` and `authUser`.
- **AC9.12:** Given a resolver whose `sourceKey` is not in the namespace of the extension that routes to it through `onSuccess`, when the flow is saved, then the save is refused.
- **AC9.13:** Given a service that answers `SUCCESS` with the username of a user whose user type or organization unit the application does not allow, when the resolver runs, then it fails as not found and authenticates no one.
- **AC9.14:** Given a resolver whose `matchAttribute` is an attribute users can change through self-service, when the flow is saved, then the save is refused.
- **AC9.15:** Given a flow whose step declares `extensionVerifiedEntityID` or another internal credential type as an input, when the flow is saved, then the save is refused; when such an input reaches the credentials step at run time, then it is refused.
- **AC9.16:** Given a sign-in through the resolver, when the flow completes, then an authentication event names the resolver, and the assertion claims no authentication method or assurance level the flow did not record.

### R10. Administrators configure everything in the console

**Requirement:** An administrator can create and edit HTTP endpoint connections and both steps without writing JSON.

**Acceptance criteria:**

- **AC10.1:** Given the connection wizard, when an administrator chooses Service Endpoint and enters a name and URL, then a connection is created with timeout 10 000 ms and authentication `none`.
- **AC10.2:** Given a connection's detail page, when the administrator edits the URL origin, then Save stays disabled until each secret of the current method is re-entered.
- **AC10.3:** Given the flow builder, when an administrator adds an extension step, then a connection must be selected before the flow can be saved, and the step offers no URL, headers or timeout fields.
- **AC10.4:** Given the flow builder, when an administrator adds inputs to send, then `inputKeys` stores them. With none chosen, `inputKeys` is empty.
- **AC10.5:** Given the flow builder, when an administrator ticks Allow authentication without a local user on a resolver, then `allowAuthenticationWithoutLocalUser` is stored as `true`.
- **AC10.6:** Given the connection wizard and detail page, when an administrator opens the authentication methods, then None, Username and Password, Bearer token and API key are offered.

### R11. Agents manage connections under the same rules

**Requirement:** An agent connected to ThunderID's MCP server can list, read, create and update HTTP endpoint connections, and every rule that protects a connection through the API applies equally to the tools.

**Acceptance criteria:**

- **AC11.1:** Given the MCP server, when an agent lists its tools, then `thunderid_list_http_connections`, `thunderid_get_http_connection_by_id`, `thunderid_create_http_connection` and `thunderid_update_http_connection` are listed, and no tool deletes a connection.
- **AC11.2:** Given a connection with a stored secret, when an agent reads, lists or updates it through the tools, then no tool output contains the secret.
- **AC11.3:** Given an update through the tool that omits the password and changes the URL origin, when it is called, then it fails as the API does, and an update that sends `******` as the password fails as AC2.11 requires.

## Change log

| Version | Date | Change |
|---|---|---|
| 0.1 | 2026-10-09 | Initial specification, from design discussion #5659 and the proof of concept. |
| 0.2 | 2026-10-12 | The request mirrors the flow context, with `inputKeys` and `runtimeDataKeys` as filters. Added the redirect to a service's page (moved into scope), provisioning after the resolver, and the custom authentication flow patterns with the step-by-step setup of each, from the proof of concept's run against a PIN service. |
| 0.3 | 2026-10-13 | Review fixes: pattern 1 uses `verifiedUserId`, since `userId` is reserved; a service must not return reserved keys; a connection with authentication requires an `https` URL (AC2.7); separate PostgreSQL and SQLite schemas; flow data is sent only when named in `runtimeDataKeys` (AC5.4); a response format section for service authors; sample PIN service responses for each custom authentication pattern. |
| 0.4 | 2026-10-13 | The inline endpoint is removed; an extension step calls only through an HTTP endpoint connection (AC1.1 to AC1.3, AC10.3). The audit event is published on the observability event bus as `EXTENSION_CALL` (R8). The never-sent and reserved lists add `passkeySessionToken`, executor-owned keys and SSO checkpoint keys (AC6.4). |
| 0.5 | 2026-10-13 | Connection authentication reuses the shared outbound authentication model (`none` and `basic` today; bearer token and API key follow in a separate feature), with its secret-retention rule and the URL origin as the credential target. Plain headers refuse credential-carrying and HTTP-managed names (AC2.8). URL host and port are validated and the URL stored canonicalized. A separate error for an invalid description. The URL and its query string are documented as not secret. An omitted secret is kept under the shared rule; `******` is not treated as a kept value. |
| 0.6 | 2026-10-14 | Aligned with the HTTP endpoint connection implementation: `PROPERTIES` stores the shared model's authentication property array, not a separate `secrets` array; connection errors are named by their `httpconnection.Error*` constants, and an unsupported authentication type returns the existing `connection.ErrorInvalidAuthenticationType`; the create wizard includes authentication; the primary key is `(DEPLOYMENT_ID, ID)`, as in the other configuration tables. |
| 0.7 | 2026-10-15 | Aligned with the extension executor implementation: `groups` and `password` are reserved, while profile attributes stay storable; a credential failure returns ThunderID's own `ErrInvalidCredentials` with no params (AC7.2); a connection that exists but cannot be resolved is `ErrExtensionCallFailed` with `errorKind` `internal`; the audit event carries `failureReason` but never `failureDescription`; a flow whose extension step has no connection is refused at save (AC1.2); property types for `inputKeys`, `runtimeDataKeys` and `responseKeys`; an `INCOMPLETE` without `onIncomplete` ends the flow with an internal server error; a redirect is treated as `ERROR` until custom authentication is delivered; the embedded engine has no connection resolver; the never-sent and engine-key lists add the linking, ownership and OU keys the executor classifies. |
| 0.8 | 2026-10-18 | `http_connection.store` defaults to `composite`, as the other resource types do. Connections offer `bearer` and `api_key` (AC2.9), refuse common credential header names (AC2.10) and refuse a masked secret (AC2.11), and secret copies kept for export follow the connection's lifecycle, through the API, MCP and import (AC2.12). Import applies the retention rule, and export refuses `api_key` header names it cannot write as distinct parameters (AC2.13). The console names the type Service Endpoint (AC10.1, AC10.6), and MCP tools manage connections under the API's rules (R11). Returned fields are stored under `extension.<nodeId>.<field>`, replaced on each call, never taken as user attributes or credentials and never copied into the SSO session with the failure fields (R6, AC6.6 to AC6.8); `maxAttempts` bounds `FAILED` and `INCOMPLETE` answers per execution (AC4.8, AC4.9, AC8.4). Flow save refuses a missing connection and malformed key lists (AC1.4 to AC1.6, AC3.4); format characters are removed from `failureDescription` (AC7.5); the reserved-key check covers only the fields a step stores (AC6.3), `password` and `provenSubject` are never sent, and a call without an HTTP response leaves the step's namespace as it was; `EXTENSION_CALL` requires observability to be enabled (R8). The resolver and the redirect are marked as requirements for the custom authentication increment, adding source binding, allowed users, non-editable match attributes, refused internal credential inputs and an authentication record (AC9.12 to AC9.16), and binding of the outcome and the return (AC4.5, AC4.10, AC4.11). |
