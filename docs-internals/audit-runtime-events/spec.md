# Audit Runtime Events Specification

- **Status:** Draft
- **Version:** 0.1
- **Related documents:** feature issue #5591, audit design #5601, eventing design #5590, eventing issue #5588, umbrella #5438, eventing and audit logs specification (`docs-internals/eventing/spec.md`, #6296), its threat model (#6297), delivery plans #6319 and #6320

## Summary

ThunderID writes audit events for management changes, client authentication and token issue and revoke. It writes nothing for what a person or a client does through the running system: a sign-in, a session, a locked account, a recovery, a consent decision. An administrator cannot reconstruct an incident that began with a sign-in, and an end user cannot see whether a sign-in on their account was theirs.

This specification adds the runtime audit events and states which reader sees which of them. The governing decision is that a runtime event is an explicit domain event, published by the service that owns the outcome, once for each confirmed attempt. Audit records outcomes. The steps inside a flow stay in diagnostics. Audit events and diagnostic lines of one login are joined by the request trace ID and by two non-secret references added to the record.

Who sees an event is decided on the read side from two facts recorded on the event: the subject, and the organization unit. Nothing is filtered at write time, so an event is recorded once and each reader sees the part they are allowed to.

In scope:

- The runtime events in the catalog below, with their actor, subject, target, outcome, reasons and details.
- The record context that links the events of one login and one session.
- Which reader sees which events through `GET /audit-events` and `GET /users/me/activity`, and the projection an end user sees.

Out of scope, as in #5591: a built-in SIEM, alerting, anomaly detection, real-time streaming or webhook delivery, and external immutable storage. The store, its retention and the two endpoints are designed in #5601. This specification states what they must carry and show for runtime events, and does not redesign them.

## Architecture

The audit package, the event bus and the default stdout sink are specified in the eventing and audit logs specification and are unchanged. This specification adds publishers and extends the catalog.

- **Authentication provider manager.** Publishes `user.authenticate` for each credential check through a decorator around its `AuthenticateUser` path, and publishes the account link event. It already publishes credential enrollment.
- **Flow session service.** Publishes `session.create`, `session.reuse` and `session.end`. Its termination hook already reports every terminated session after the terminating transaction commits.
- **Token services.** Already publish `token.issue` and `token.revoke`. The revocation handler gains the owning user as subject.
- **Identity governance service.** Publishes the lock, unlock, suspend and unsuspend events beside the observability events it publishes today.
- **User service and attribute verification.** Publish the verification events and the credential removal event.
- **Consent executor.** Publishes the consent decision event where it records the decisions.
- **Flow engine.** Publishes the flow outcome event.
- **Audit service (from #5601).** Serves the two endpoints, applies the end-user projection, and publishes the event for a read of the audit log.

The observability pipeline is not changed. The existing observability events, including the identity lock and suspension events, keep their names and payloads.

## Detailed design

### Design principles

- **Explicit publication.** The service that owns an outcome publishes it. No middleware or route table derives runtime events. This continues principle four of the eventing specification.
- **One event for each confirmed attempt by one actor.** The unit is a credential check, a session transition, a token request, a state change or a decision, and the event is published when the outcome is final.
- **Audit records outcomes, diagnostics record steps.** The node-by-node events of a flow stay in the observability pipeline. A reader moves between the two by the trace ID, the flow reference and the session reference.
- **No identifier that works as a credential enters an event.** A session ID, a flow execution ID, a token and an attempted username never appear in a line.
- **Add, never repurpose.** A new event or an extra detail key is additive. An event name, a reason code and a detail key keep their meaning for good.

### Record context

The record context from #5601 gains these fields. Each is set only when it exists, so no field is written empty.

| Field | Content | Source |
|---|---|---|
| `application` | Application ID and display name | The application the request or flow runs for |
| `ou` | ID of the organization unit the event belongs to | The home organization unit of the subject or the target, resolved when the event is published |
| `session_ref` | A reference to the session that is the same on every event of that session, and cannot be used as the session | The flow session service |
| `flow_ref` | A reference to the login flow execution that is the same on every event of that login, and cannot be used as the execution | The flow engine |

`session_ref` and `flow_ref` are the only join keys across requests. A credential check happens before a session exists, so it carries `flow_ref` only. `session.create` carries both, which links the login to the session it created. Every later event of the session carries `session_ref`.

A reference is derived so that a copy of a log line gives no access to the session or the flow. Where the stored record already has an identifier that is distinct from the handle the client presents, that identifier is the reference. Where it does not, the reference is a keyed hash of the handle. Which of the two applies to each is confirmed in the first increment (open question 7).

Display names of the target and the application are copied into the record when the event is published, capped at 128 characters and JSON-escaped, so the record stays readable after the object is deleted.

### Event catalog

Every event follows the audit payload rules of the eventing specification: names are `<resource>.<action>`, an outcome is `succeeded`, `denied` or `failed`, and a reason code is required unless it succeeded. The first group already exists and changes. The second group is new.

Changes to existing events:

| Event | Change |
|---|---|
| `token.revoke` | Carries the owning user as subject when the token belongs to a user. A token that is unknown or not valid carries no subject, so the log never says whether it existed. |
| `token.issue` | A refused request carries the stable reason of the error response. A refresh is recorded with `grant_type` `refresh_token`. |
| `user.credential.update`, `user.credential.enroll` | Gain the detail `credential`, the kind of credential (for example `password`, `pin`, `passkey`). Never a value. A failed update on an account the identifier resolved to carries that user as subject. |

New events:

| Event | Published by | Actor | Subject and target | Outcomes and reasons | Details |
|---|---|---|---|---|---|
| `user.authenticate` | Authentication provider manager, once for each credential check | The authenticated entity on success, anonymous on failure or refusal | The authenticated entity on success. On a failure, the user the identifier resolved to as subject (R3), and the user type as target | `succeeded`. `failed`: `invalid_credentials`, `user_not_found`, `invalid_request`, `ambiguous_user`, `server_error`. `denied`: `account_locked`, `account_suspended`, `subject_not_allowed` | `method` |
| `session.create` | Flow session service | The authenticated user | The user | `succeeded`, or `failed` with the store reason | None |
| `session.reuse` | Flow session service | The user whose session is used | The user, with the application in the record context | `succeeded`, or `denied` when the session does not satisfy the request | None |
| `session.end` | Flow session service, from its termination hook | The user for a sign-out, the administrator or the system for a termination of all sessions | The user | `succeeded` | `end_reason`: `sign_out`, `subject_revocation` |
| `user.lock` | Identity governance service | `system` for an automatic lock | The user | `succeeded` | None |
| `user.unlock` | Identity governance service | The user for a self-unlock through recovery, the administrator otherwise | The user | `succeeded`, `failed` | None |
| `user.suspend`, `user.unsuspend` | Identity governance service | The administrator | The user | `succeeded`, `failed` | None |
| `user.verification.request`, `user.verification.verify`, `user.verification.invalidate` | Attribute verification in the user service | The user, the administrator or the system | The user | `succeeded`, `denied`, `failed` | `attribute`, the name of the attribute and never its value |
| `user.linked_account.create` | Authentication provider manager | The authenticated user | The user | `succeeded`, `denied`, `failed` | `connection`, the connection ID |
| `user.recovery.start` | Recovery flow | Anonymous, or the user when signed in | The user when the identifier resolved to one | `succeeded`, `failed`, `denied` | None |
| `user.credential.remove` | User service | The user or the administrator | The user | `succeeded`, `denied`, `failed` | `credential` |
| `consent.decide` | Consent executor | The user | The user, with the application in the record context | `succeeded`, `failed` | `decision`: `granted` or `declined`. `attributes`: the names of the attributes the decision covers |
| `flow.execution.finish` | Flow engine | The user when known, otherwise anonymous | The flow definition | `succeeded`, `failed` with a stable reason | `flow_type` |
| `audit_log.read` | Audit service | The reader | The audit log, with the scope in `details` | `succeeded`, `denied` | `scope`, `filters` (names only) |

`user.authenticate` is one name for every entity category. On a success the actor type shows whether the entity is a user, an agent or an application. The publisher sets the actor explicitly and never reads the security context of the request, so a bearer token sent to a public endpoint cannot become the actor.

### Credential checks and failed attempts

A sign-in with two credentials, such as a password then a one-time passcode, writes two `user.authenticate` events with the same `flow_ref` and a different `method`. A failed first factor writes one event and the login ends there.

A failed check records the user the identifier resolved to as subject, using the internal ID, so the event appears in that account's own log. The text the person typed is never recorded, because people paste passwords into username fields. When the identifier resolves to no user or to more than one, the event has no subject and the target is the user type with no ID.

This differs from the eventing specification, which records no account on a failed check. The change follows the definition of subject in #5601, the identity whose activity log the event belongs in, and is open question 2.

### Sessions

`session.create` is published after the session commits. `session.reuse` is published when an existing session satisfies an authorization request without a credential check, which is the single sign-on case that no credential event covers. `session.end` is published from the existing termination hook, after the terminating transaction commits.

An expired session has no event, because nothing happens when it expires. A reader derives expiry from the `session.create` time and the configured lifetime. This is a recorded gap.

### Account state and self-service

The lock, unlock, suspend and unsuspend events are published beside the observability events with the same trigger, so each fact is written once to each pipeline from the same code path and the two cannot disagree. A recovery that resets a credential records `user.recovery.start` and, when it completes, `user.credential.update` with the actor set explicitly to the user whose credential was set.

### Consent and flow outcomes

`consent.decide` records a decision at the moment it is made in a flow. It differs from the consent record events, which record that a consent was created or updated. `flow.execution.finish` records only the final outcome of a flow execution. The nodes of the execution are not audit events.

### Example: one sign-in with a password then a one-time passcode

This example shows what one login writes to audit, what stays in diagnostics, and how a reader moves between the two. Names are real event names. The values are illustrative, and the placement of the new context fields is as proposed in this specification.

A login is several requests. The person submits their identifier and password in one request, the passcode in the next, and the application then calls the token endpoint. Each request has its own `trace_id`, so the trace ID cannot join them. `flow_ref` joins the requests of the login, and `session_ref` joins everything after the session exists.

| Step | Request | Audit event written | Diagnostic events written |
|---|---|---|---|
| 1 | The flow starts and shows the password screen | none | `FLOW_STARTED`, `FLOW_USER_INPUT_REQUIRED` |
| 2 | The person submits the password | `user.authenticate`, method `password`, `succeeded`, with `flow_ref` | `FLOW_NODE_EXECUTION_STARTED`, `FLOW_NODE_EXECUTION_COMPLETED`, `FLOW_USER_INPUT_REQUIRED` for the passcode screen |
| 3 | The person submits the passcode | `user.authenticate`, method `otp`, `succeeded`, with the same `flow_ref` | `FLOW_NODE_EXECUTION_STARTED`, `FLOW_NODE_EXECUTION_COMPLETED` |
| 4 | The flow creates the session and ends | `session.create`, with `flow_ref` and `session_ref`, then `flow.execution.finish`, `succeeded` | `FLOW_NODE_EXECUTION_COMPLETED`, `FLOW_COMPLETED` |
| 5 | The application exchanges the code at the token endpoint | `token.issue`, `succeeded`, grant type `authorization_code`, carrying `session_ref` when the request can be tied to the session | `TOKEN_ISSUANCE_STARTED`, `TOKEN_ISSUED` |

The audit events of step 2, an illustrative line for the password check and one for the session:

```json
{"log_type":"AUDIT","schema_version":1,"id":"0193...","time":"2026-11-12T09:14:03.120411Z","event":"user.authenticate","trace_id":"b7e1...","actor":{"type":"user","id":"u42..."},"subject":{"type":"user","id":"u42..."},"target":{"type":"user","id":"u42..."},"action":"authenticate","outcome":{"result":"succeeded"},"client":{"address":"203.0.113.7","user_agent":"Mozilla/5.0 ..."},"application":{"id":"a17...","name":"Customer Portal"},"ou":{"id":"o3..."},"flow_ref":"f9c2...","details":{"method":"password"}}
{"log_type":"AUDIT","schema_version":1,"id":"0193...","time":"2026-11-12T09:14:19.804233Z","event":"session.create","trace_id":"c4a8...","actor":{"type":"user","id":"u42..."},"subject":{"type":"user","id":"u42..."},"target":{"type":"user","id":"u42..."},"action":"create","outcome":{"result":"succeeded"},"client":{"address":"203.0.113.7","user_agent":"Mozilla/5.0 ..."},"application":{"id":"a17...","name":"Customer Portal"},"ou":{"id":"o3..."},"flow_ref":"f9c2...","session_ref":"s5d1..."}
```

What the example shows:

| Observation | What it shows | Where addressed |
|---|---|---|
| Five audit events are written, and eleven diagnostic events. | Audit records the result of each accountable act. The nodes of the flow are not audit events. | R9, AC9.2 |
| Both credential events carry the same `flow_ref`, and the requests that produced them have different trace IDs. | The reference joins a login across requests where the trace ID cannot. | R1, AC1.5 |
| `session.create` carries both references. | It ties the login to the session it created, and every later event of the session carries `session_ref`. | R1, AC1.5 |
| A failed passcode would write `user.authenticate`, method `otp`, `failed`, with the user as subject, and no `session.create`. | The failure is in the account's own log, and the absence of a session is itself the evidence that the login did not complete. | R2, R3 |
| No line contains the password, the passcode, a session ID or the flow execution ID. | The references are not credentials. | R1, AC1.4 |
| An investigator who wants to know which node of the flow failed reads the diagnostic events of the same trace ID or `flow_ref`. | Step detail is in diagnostics, which keep it for a shorter period and may lose it under load. | R9 |

The last row is the cost of the split. Once the diagnostic events expire, the audit record still says what the outcome was and who was involved, and no longer says which node the failure happened in.

### Visibility

A reader sees events by the rule for their role. The rule is applied on the read side, from the `subject`, the `ou` and the event name.

| Event group | Platform administrator | End user | Compliance officer |
|---|---|---|---|
| Management changes | all | those whose subject is the user | all |
| Deployment-wide changes (`server_config.update`, `gateway.*`, `project.*`) | all | none | all |
| Sign-in, session, token | all | those whose subject is the user | all |
| Credential, account state, verification, linking, recovery | all | those whose subject is the user | all |
| Consent decisions | all | those whose subject is the user | all |
| Flow outcomes | all | none | all |
| `audit_log.read` | all | none | all |

A platform administrator and a compliance officer both hold the audit read permission. The compliance officer's permission is read only and does not allow any change. Visibility limited to an organization unit subtree is open question 5.

### Delivery and failure behavior

Runtime events use the delivery of the eventing specification. Until the audit store of #5601 exists, delivery is best effort. When the store exists, audit events are delivered at least once and recorded after the change commits, as chosen in #5590.

A publisher never fails its caller because of audit. A sign-in, a token request and a consent decision complete whether or not the event is recorded. A publish that fails writes one diagnostic line without the event content. Loss on a full queue is detectable through the drop warning and the per-category sequence number of the eventing specification.

Every increment is additive and independent. Removing a publisher stops new events and leaves stored events valid. Renaming or removing an event, a reason code or a detail key is a breaking change for any consumer, and is not allowed after the event ships.

### Data model

No new table. The audit store of #5601 keeps these values of this specification in columns or in its payload: `application_id`, `ou_id`, `session_ref`, `flow_ref` and the subject ID. Its access paths must include the subject within a time range, for the end-user view, and the session reference within a time range, for investigation.

Any audit table or index follows the deployment-scoped rule: a `DEPLOYMENT_ID VARCHAR(255) NOT NULL` column, `PRIMARY KEY (DEPLOYMENT_ID, ID)`, unique indexes scoped by `DEPLOYMENT_ID`, every query filtered by it, and the SQLite and PostgreSQL scripts changed together.

### API

No new endpoint. The two endpoints of #5601 change as follows.

- `GET /audit-events` needs the audit read permission and refuses a caller without it. It gains the filters `sessionRef` and `flowRef`, and returns `application`, `ou`, `session_ref` and `flow_ref`. Its filters are allowlisted. It never accepts a free-form query.
- `GET /users/me/activity` returns the events whose subject is the caller and no other, and applies the end-user projection below.

End-user projection:

| Field | Shown to the end user |
|---|---|
| Time, event label from the catalog, outcome | yes |
| Client address and user agent | yes, of the caller's own events |
| Application display name | yes |
| Actor | the caller, or the words "an administrator" or "the system" when someone else acted. Never the administrator's ID or name |
| Reason | a category the person understands. Never the internal code |
| `trace_id`, `session_ref`, `flow_ref`, target IDs, organization unit | no |

### Configuration

No new key. `audit.enabled` is unchanged.

## Requirements

### R1. Record context

**Requirement:** Every runtime event carries the context a reader needs to say which application, organization unit, login and session it belongs to, and none of it can be used to act as the person.

**Acceptance criteria:**

- **AC1.1:** Given an event published for a request that runs for an application, when it is written, then the line carries the application ID and a display name of at most 128 characters.
- **AC1.2:** Given a subject that belongs to an organization unit, when an event is written, then the line carries that organization unit's ID.
- **AC1.3:** Given an event inside a login flow, when it is written, then it carries `flow_ref`. Given an event inside a session, then it carries `session_ref`. Given neither exists, then neither field is present and none is written empty.
- **AC1.4:** Given any event, when it is written, then no field contains a session ID, a flow execution ID, a token or an attempted username, and a copy of the line does not allow a request to be made as the person.
- **AC1.5:** Given a login that creates a session, when `session.create` is written, then it carries both `flow_ref` and `session_ref`, and the earlier credential events of that login carry the same `flow_ref`.

### R2. Credential checks

**Requirement:** Each credential check during a sign-in is recorded with its outcome and method.

**Acceptance criteria:**

- **AC2.1:** Given a credential check, when it succeeds, is refused or fails, then one `user.authenticate` event is written for that check with the matching outcome and the `method` in the details.
- **AC2.2:** Given a successful check, when the event is written, then the authenticated entity is the actor, the subject and the target.
- **AC2.3:** Given a sign-in with a password then a one-time passcode, when both succeed, then two events are written with the same `flow_ref` and different methods.
- **AC2.4:** Given a refused or failed check, when the reason is known, then it is one of `invalid_credentials`, `user_not_found`, `invalid_request`, `ambiguous_user` or `server_error` for `failed`, and `account_locked`, `account_suspended` or `subject_not_allowed` for `denied`.
- **AC2.5:** Given a bearer token sent to a public sign-in endpoint, when a check fails, then the actor is anonymous and the token's subject is not used.

### R3. Failed attempts name the account

**Requirement:** A failed or refused attempt on an existing account appears in that account's log without recording what the person typed.

**Acceptance criteria:**

- **AC3.1:** Given a failed check whose identifier resolves to exactly one user, when the event is written, then that user is the subject, by internal ID.
- **AC3.2:** Given a failed check whose identifier resolves to no user or to more than one, when the event is written, then it has no subject and the target is the user type with no ID.
- **AC3.3:** Given any failed check, when the event is written, then the text the person typed appears nowhere in the line.

### R4. Session lifecycle

**Requirement:** Establishing, reusing and ending a session are separate events.

**Acceptance criteria:**

- **AC4.1:** Given a login that completes, when the session commits, then one `session.create` event is written for the user.
- **AC4.2:** Given an existing session that satisfies an authorization request without a credential check, when it is used, then one `session.reuse` event is written carrying the application.
- **AC4.3:** Given a sign-out, when the session ends, then `session.end` is written with `end_reason` `sign_out` and the user as actor.
- **AC4.4:** Given an administrative termination of all of a user's sessions, when it commits, then one `session.end` event with `end_reason` `subject_revocation` is written for each session that ended, with the administrator as actor.
- **AC4.5:** Given a session that expires without a sign-out, when the lifetime passes, then no event is written.

### R5. Tokens

**Requirement:** Token events say whose token it was and why a request was refused.

**Acceptance criteria:**

- **AC5.1:** Given a revocation of a token owned by a user, when it succeeds, then `token.revoke` carries that user as subject.
- **AC5.2:** Given a revocation of a token that is unknown or not valid, when it is handled, then the event carries no subject and the response is unchanged.
- **AC5.3:** Given a refused token request, when the event is written, then it carries the stable reason of the error response.
- **AC5.4:** Given a refresh grant, when it succeeds, then `token.issue` is written with `grant_type` `refresh_token`.

### R6. Account state

**Requirement:** A change in the state of an account is recorded when it happens.

**Acceptance criteria:**

- **AC6.1:** Given an automatic lock, when it applies, then `user.lock` is written with the actor `system` and the user as subject.
- **AC6.2:** Given a self-unlock through recovery, when it succeeds, then `user.unlock` is written with the user as actor.
- **AC6.3:** Given an administrator who suspends or unsuspends a user, when it completes, then `user.suspend` or `user.unsuspend` is written with the administrator as actor.
- **AC6.4:** Given a verification of an attribute that is requested, completed or invalidated, when it happens, then the matching `user.verification.*` event is written with the attribute name and never its value.
- **AC6.5:** Given a federated account that is linked, when it succeeds, then `user.linked_account.create` is written with the connection ID in the details.
- **AC6.6:** Given any of these facts, when the observability event for it is published, then both the observability event and the audit event are written from the same code path.

### R7. Self-service credentials

**Requirement:** Recovery and credential changes are recorded, including the ones that fail.

**Acceptance criteria:**

- **AC7.1:** Given a recovery, when it starts, then `user.recovery.start` is written with the subject set when the identifier resolved to a user.
- **AC7.2:** Given a recovery that resets a credential, when it completes, then `user.credential.update` is written with the actor set explicitly to the user whose credential was set.
- **AC7.3:** Given a factor that is removed, when it completes, then `user.credential.remove` is written with the `credential` detail.
- **AC7.4:** Given a credential change that fails on an account the identifier resolved to, when the event is written, then it carries that user as subject and the reason.
- **AC7.5:** Given any credential event, when it is written, then it carries the kind of credential and never the credential, a hash of it or a prompt.

### R8. Consent decisions

**Requirement:** A consent decision is recorded when it is made, with what it covers.

**Acceptance criteria:**

- **AC8.1:** Given a user who grants consent in a flow, when the decision is recorded, then `consent.decide` is written with `decision` `granted`, the user as actor and subject, and the names of the attributes covered.
- **AC8.2:** Given a user who declines, when the decision is recorded, then `consent.decide` is written with `decision` `declined`.
- **AC8.3:** Given a decision, when it is written, then the application is in the record context and no attribute value is in the line.

### R9. Flow outcomes

**Requirement:** The final outcome of a flow execution is recorded, and its steps are not.

**Acceptance criteria:**

- **AC9.1:** Given a flow execution that completes or fails, when it ends, then one `flow.execution.finish` event is written with `flow_type` and a stable reason on failure.
- **AC9.2:** Given a flow execution, when it runs, then no audit event is written for any of its nodes, and the node events stay in the observability pipeline.
- **AC9.3:** Given a flow execution event, when it is written, then its `trace_id` and `flow_ref` match those of the credential events of the same login.

### R10. Details and secrets

**Requirement:** Details hold facts that identify nothing secret.

**Acceptance criteria:**

- **AC10.1:** Given an event, when its details carry a key the event does not allow, then the event is not written.
- **AC10.2:** Given a detail value, when it is written, then it is at most 512 characters and JSON-escaped.
- **AC10.3:** Given any event of this specification, when it is written, then no password, one-time passcode, token, secret, session ID or attribute value is in the line.

### R11. No effect on the request

**Requirement:** Recording an event never changes the outcome of the request.

**Acceptance criteria:**

- **AC11.1:** Given a publisher whose bus is full or has no sink, when a sign-in, a token request or a consent decision completes, then the response is the same as with audit off.
- **AC11.2:** Given a publish that fails, when it fails, then one diagnostic line is written without the event content and the caller is not told.

### R12. End-user visibility

**Requirement:** A user sees the events about their own account and nothing else.

**Acceptance criteria:**

- **AC12.1:** Given a caller, when they request their activity, then every event returned has the caller as subject and no event with another subject is returned.
- **AC12.2:** Given an event where the caller is only the actor on another person's resource, when they request their activity, then it is not returned.
- **AC12.3:** Given an event caused by an administrator, when it is returned, then the actor reads "an administrator", with no ID or name.
- **AC12.4:** Given an event with an internal reason code, when it is returned, then the reason is a category and not the code.
- **AC12.5:** Given the response, when it is returned, then it carries no `trace_id`, `session_ref`, `flow_ref`, target ID or organization unit.

### R13. Administrator visibility

**Requirement:** Reading the audit log is a deliberate permission, and each read is recorded.

**Acceptance criteria:**

- **AC13.1:** Given a caller without the audit read permission, when they request the audit log, then the request is refused and `audit_log.read` is written with outcome `denied`.
- **AC13.2:** Given a caller with the permission, when they request it, then all events of the groups in the visibility table are available, filtered by the allowlisted filters only.
- **AC13.3:** Given a read, when it completes, then `audit_log.read` is written with the filter names and never their values.
- **AC13.4:** Given a request for events of one session, when `sessionRef` is given, then the events of that session and none other are returned, in time order.

### R14. Stable names and reason codes

**Requirement:** Event names, reason codes and detail keys do not change after they ship.

**Acceptance criteria:**

- **AC14.1:** Given every event of this specification, when it is published, then its name is in the audit catalog and follows the name grammar.
- **AC14.2:** Given an outcome other than `succeeded`, when the event is written, then it carries a reason from the stable set of that event.
- **AC14.3:** Given a release, when it adds an event or a detail key, then no existing name, reason or key changes.

### R15. Observability is unchanged

**Requirement:** Adding audit events does not change the observability pipeline.

**Acceptance criteria:**

- **AC15.1:** Given an embedder that implements `ObservabilityProvider`, when this specification is delivered, then it receives the same events with the same names and payloads.
- **AC15.2:** Given a lock, a suspension or a verification, when it happens, then the observability event is still published.

## Open questions

1. **To confirm before approval: one event for each credential check or one for each login.** This specification keeps one for each credential check, as the eventing specification does, because it records the step that failed. #5601 says one actor, one attempt, one event, which could also mean one for each login. A login summary can be derived at read time from the checks, the session and the flow outcome.
2. **To confirm before approval: whether a failed check names the account.** R3 records the resolved user as subject, by the definition of subject in #5601, which differs from the eventing specification. It needs a security review.
3. **Authorization denied.** #5601 lists it as a security decision and the delivery plan #6320 excludes calls refused with 401 or 403. It is not in this specification until one of the two changes.
4. **User data viewed.** #5601 lists disclosure of secrets, with user data viewed as the example. Which reads count is not defined, so it is not in this specification.
5. **Visibility limited to an organization unit subtree.** A proposal in a comment on #5601 records the organization unit as a lineage snapshot and scopes the endpoint by it. This specification records the organization unit on every event, which that proposal needs, and does not decide the scoping or the partner case.
6. **Consent revoked.** #5601 lists it, and the consent service has no revoke operation to publish from. Only decisions and the consent record events are covered.
7. **The session and flow references.** Whether each is an identifier already distinct from the handle the client holds, or a keyed hash of the handle, is confirmed in the first increment and recorded here.
8. **Flow restart and replay.** Whether the flow engine can tell a restarted or replayed execution apart is not confirmed, so no event is specified.
9. **Successful client authentication.** Only a refusal writes `client.authenticate` today. Whether a success should be recorded is not decided.

## Change log

| Version | Date | Change |
|---|---|---|
| 0.1 | 2026-11-12 | Initial draft. Runtime events, record context, visibility by reader, and the end-user projection. |
