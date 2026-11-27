# SSO Session Management Threat Model

This model covers SSO session management in ThunderID: the device and IP recorded on each SSO session, the resolution of the client IP behind trusted proxies, the session management API for administrators and for signed-in users, and the Console Sessions views.

## Overview

Session management lets people see and end SSO sessions. Every session records the User-Agent and IP it was used from. Administrators list and revoke any user's sessions through `/sessions`, and a signed-in user lists and revokes their own through `/users/me/sessions`. A revocation runs the same termination as sign-out, so it revokes the session's grants, deletes the session, and triggers back-channel logout.

The security-relevant shift is that sessions, which no API exposed before, can now be read and ended by id. The self-service routes must never let one user see or end another user's session, the administrator routes must stay behind `system:session:view` and `system:session:delete`, and a revocation must leave nothing that can still back SSO or refresh a token. The IP and User-Agent are stored on the session, and the IP is also written to the access log. Both are client supplied and personal data, so nothing may trust them, they must not be able to inject content into the database or the log, and the stored copy must go with the session.

Cross-cutting concerns covered elsewhere: access token validation and the permission check in the security middleware, the login flow that creates and reuses sessions, the session cookie transport, token family revocation, and back-channel logout delivery. These are trust inputs here, not re-analysed.

## Scope

This model covers:

- Client IP resolution in `RequestMetadataMiddleware` and the `clientip` package, and validation of `server.trusted_proxies` at startup.
- Recording `userAgent`, `creationIp`, and `lastActiveIp` on a session.
- The client IP added to the access log.
- `GET /sessions`, `DELETE /sessions`, and `DELETE /sessions/{sessionId}` for administrators.
- `GET /users/me/sessions`, `DELETE /users/me/sessions`, and `DELETE /users/me/sessions/{sessionId}` for signed-in users.
- The Console Sessions tab on the user and application pages.

Out of scope (see the referenced companion models or the owning area):

- Delivery of the logout token after a revocation. Owned by [back-channel-logout/threat-model.md](../back-channel-logout/threat-model.md), whose scope names the sign-out (`Terminate`), subject revocation (`TerminateBySubject`) and session management API (`RevokeSession` and `RevokeSubjectSessions`) paths. Revocation through this API reaches the same listener with the reason `session_revocation`.
- Session establishment, reuse, the session cookie handle, the idle and absolute deadlines, and the cleanup procedures. Owned by the session area.
- The other paths that end sessions. Sign-out runs the same termination as a revocation through this API, so it leaves the same state. User suspension and user delete first revoke every token issued to the subject, then delete the sessions with `TerminateBySubject`, so nothing is left for a later revoke-all to end, and a suspended user is refused at session reuse and at the refresh grant as well. An automatic lock after failed sign-ins is different: it refuses new authentication only and ends no session or token, so that failed guesses cannot sign a user out. The live sessions of a locked user stay listed, and revoke-all is the way to end them. Application delete revokes that application's token families and removes its participation with `DetachApplication`.
- Token family revocation itself. Owned by the token revocation area, and used here unchanged through the criteria revoker.
- Access token validation and the permission table. Trust inputs here.
- The SDK components that call the self-service API. Owned by [sdk-development/threat-model.md](../sdk-development/threat-model.md).

## Architecture

```mermaid
flowchart LR
  subgraph Untrusted ["Untrusted"]
    U["User's<br/>browser or app"]
    A["Administrator<br/>client or Console"]
  end
  subgraph Operator ["Operator network"]
    P["Reverse proxy<br/>listed as trusted"]
  end
  subgraph Trusted ["Trust boundary: ThunderID deployment"]
    MW["Request<br/>Metadata<br/>Middleware"]
    SEC["Security<br/>middleware<br/>token and<br/>permission"]
    FE["Flow execution<br/>and authorize<br/>endpoints"]
    API["sessionmgt<br/>API"]
    SS["session.<br/>Service"]
    AL["Access<br/>log"]
    DB[("SESSION<br/>SESSION_<br/>PARTICIPANT")]
    REV["Criteria<br/>revoker"]
    BCL["Termination<br/>listeners"]
  end
  U -->|"HTTPS,<br/>headers set<br/>by client"| P
  A -->|"HTTPS,<br/>bearer token"| P
  P -->|"adds<br/>X-Forwarded-For"| MW
  U -.->|"direct,<br/>no proxy"| MW
  MW --> AL
  MW --> FE
  MW --> SEC
  SEC --> API
  FE --> SS
  API --> SS
  SS --> DB
  SS --> REV
  SS -->|after commit| BCL
```

The trust boundary sits at the deployment edge. Two things cross it that matter here. Request headers come from the client and are untrusted. The one exception is the forwarding header set for a trusted proxy, which is read only when the request comes from that proxy's range. The User-Agent is always the client's, so it is cleaned and capped whatever the source. Session ids leave through the API and come back in revocation requests, so every self-service call is bound to the subject in the access token rather than to the id it names.

### Components

| Component | Task |
| --- | --- |
| `clientip` resolver | Matches the connecting address against the parsed trusted ranges and reads a forwarding header only for a match. Walks `X-Forwarded-For` right to left |
| `RequestMetadataMiddleware` | Puts the request metadata, the resolved IP and the User-Agent, on the request context for every request |
| Session step | Builds `ClientInfo` from the request metadata, dropping invalid UTF-8 and capping the User-Agent at 512 characters |
| Security middleware | Validates the access token, requires `system:session:view` on `GET /sessions` and `system:session:delete` on the `DELETE /sessions` routes, and lets any signed-in user reach `/users/me/sessions`. The Control Plane declares no session permission and leaves the `/sessions` routes out |
| `sessionmgt` handler | Validates the request, takes the self-service user from the token, marks the current session from the token's `tfid`, and resolves names for administrators |
| `session.Service` and store | List live sessions with parameterized queries, check ownership, and revoke sessions in one transaction |
| Criteria revoker | Revokes the token families of a revoked session's participants |
| Termination listeners | Called after a revocation commits, with the session id, subject id, and participants. Back-channel logout is one |
| Console Sessions panel | Calls the administrator API with the signed-in administrator's token |

### Actors

#### Actors

| Actor | Description | Roles or permissions |
| --- | --- | --- |
| End user | Signs in through a login flow, and manages their own sessions | A valid access token |
| Administrator | Lists and revokes any user's sessions, through the API, the Console or the Admin Console | `system:session:view` to list, `system:session:delete` to revoke. A role that holds `system` has both |
| Attacker | Any caller able to reach ThunderID or a proxy, able to set any header. May hold a valid token for their own account | None, or their own access token |
| Trusted reverse proxy | Forwards requests and reports the client IP in a header | Listed in `server.trusted_proxies` |
| Operator | Configures `server.trusted_proxies`, and reads the access log and database | Deployment configuration |

#### Entitlement matrix

| Actor | Set the recorded client IP | Set the recorded User-Agent | List own sessions | List any user's sessions | Revoke own sessions | Revoke any user's sessions | Change trusted proxies |
| --- | --- | --- | --- | --- | --- | --- | --- |
| End user | [No] | [Yes], on their own sessions | [Yes] | [No] | [Yes] | [No] | [No] |
| Administrator with both permissions | [No] | [No] | [Yes] | [Yes] | [Yes] | [Yes] | [No] |
| Administrator with only `system:session:view` | [No] | [No] | [Yes] | [Yes] | [Yes] | [No] | [No] |
| Administrator with only `system:session:delete` | [No] | [No] | [Yes] | [No] | [Yes] | [Yes] | [No] |
| Attacker | [No], unless a trusted range covers the address they connect from (threat 4) | [Yes], on their own sessions | [Yes], their own account only | [No] | [Yes], their own account only | [No] | [No] |
| Trusted reverse proxy | [Yes] | [No] | [No] | [No] | [No] | [No] | [No] |
| Operator | [No] | [No] | [No] | [No] | [No] | [No] | [Yes] |

### External Dependencies (not owned)

| Dependency | Description (usage, purpose, authentication, authorization, security) |
| --- | --- |
| Reverse proxy | Operator owned. Must overwrite or append to the forwarding header so a client cannot place an address to the right of its own. ThunderID trusts it only by its connecting address |
| Runtime persistent database | Stores `SESSION` and `SESSION_PARTICIPANT`. Access control, encryption at rest, and scheduling the PostgreSQL cleanup procedure are the operator's |
| User and application services | Resolve display names for the administrator listing. A failed lookup leaves the name out |

## Threats and mitigations

### Out-of-scope interactions and risks

- Session hijacking through a stolen session cookie. Owned by the session transport. The recorded IP is not used to bind a session, so it neither helps nor hurts here.
- Session fixation through the session cookie. Owned by the session area. The server mints the handle only when a user's first authentication is saved, so there is no handle before authentication to fix. A cookie value the server did not mint resolves to no session and is replaced by a new handle. An authentication by a different user is not attached to an existing session. A re-authentication by the same user keeps the handle and moves the authentication time forward. Replacing the handle at that point is tracked in #ISSUE-HANDLE-ROTATION.
- A stolen access token used against `/users/me/sessions`. The token is validated by the security middleware, and what it can do here is no more than it can already do over the account.
- Failure or delay of logout token delivery after a revocation. Owned by the back-channel logout model.
- The deployment's network path between the client and a trusted proxy, and between the proxy and ThunderID.

### Interactions

#### 01: Resolving the client IP

**Description**

`RequestMetadataMiddleware` resolves the client IP for every request. A forwarding header is read only when the connecting address is in a trusted range. `X-Forwarded-For` is walked right to left to the first untrusted address. A single-IP header must be sent exactly once. A missing header, or an invalid address reached before the first untrusted one, falls back to the connecting address. Addresses further left are never read, and if every address is trusted the leftmost one is used.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| Client | Trusted reverse proxy, optional | `RequestMetadataMiddleware` |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant C as Client
  participant P as Trusted proxy
  participant M as RequestMetadataMiddleware
  C->>P: request, X-Forwarded-For: spoofed
  P->>M: X-Forwarded-For: spoofed, client
  M->>M: connecting address in trusted range?
  M->>M: walk right to left, first untrusted = client
  M->>M: put client IP and User-Agent on context
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-Medium] | An IP address is personal data |
| Communication medium | [M-NT] | |
| Transport security | [TLS] | TLS ends at the proxy or at ThunderID, as deployed |
| Authentication | None | Applies to every request |
| Accessibility | [Public] | |
| Authorization and Access Control | Trusted ranges | Only a trusted connecting address may report a client IP |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 1 | [Spoofing] | A client connecting directly sends `X-Forwarded-For` to record a chosen IP | [No] | Headers are read only when the connecting address is in a configured trusted range. The default list is empty |
| 2 | [Spoofing] | A client behind a trusted proxy prepends a fake address to `X-Forwarded-For` | [No] | The header is walked right to left and stops at the first untrusted address, which the proxy appended. Addresses to its left are never read |
| 3 | [Spoofing] | A client sends a single-IP header such as `X-Real-IP` twice, hoping its line is read | [No] | A single-IP header sent more than once is rejected, and the connecting address is used |
| 4 | [Security Risk] | An operator trusts a range that untrusted callers can connect from, such as `0.0.0.0/0` or a private range that also holds internal users, or a proxy that passes the client's header through unchanged. With the default empty list every session behind a proxy records the proxy's address, which pushes operators to widen the list | [Yes] | Bounded: the IP is for display and logging only, so a spoofed value cannot grant access. Startup does not refuse a wide range, since a proxy inside a wide private range is a valid setup. The configuration reference asks for the narrowest ranges and for the proxy to set the header itself. Recorded as a residual |
| 5 | [Denial of Service] | A long `X-Forwarded-For` header costs parsing time on every request | [No] | The whole header is split, and when every hop is trusted each one is matched against every trusted entry. The cost is bounded by Go's default 1 MB header limit, since the server does not set `MaxHeaderBytes`, times the number of trusted entries. Parsing stops at the first untrusted or invalid address |
| 6 | [Operational Risk] | A trusted proxy is misconfigured and sends no header, so every request records the proxy's IP. Logging each fallback as a warning would let a client behind the proxy fill the log with headers that give no valid IP | [No] | The fallback is logged at debug level only, so a client cannot raise warnings, and an operator finds the proxy by turning on debug logging |
| 7 | [Operational Risk] | An invalid trusted proxy entry is ignored, so the operator believes a proxy is trusted when it is not | [No] | Invalid CIDRs, IPv4-mapped ranges shorter than `/96`, invalid header names, and `forwarded` fail at startup |

#### 02: Recording client metadata on a session

**Description**

The session step reads the client IP and the User-Agent from the request metadata and passes them to the session service, on every path that reaches it: the flow execution endpoint and a `prompt=none` authorization request. The session service writes `userAgent` and `creationIp` once on creation, and refreshes `lastActiveIp` when a login flow reuses the session and refreshes `LAST_ACTIVE_AT`, or the user authenticates again inside it. A `refresh_token` grant leaves the properties as they are.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| Client | `RequestMetadataMiddleware`, session step, session service | `SESSION.PROPERTIES` |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant C as Client
  participant M as RequestMetadataMiddleware
  participant F as Session step
  participant S as Session service
  participant D as Database
  C->>M: POST /flow/execute or GET /oauth2/authorize, User-Agent
  M->>F: request metadata on context
  F->>F: drop invalid UTF-8, cap at 512 characters
  F->>S: ClientInfo
  S->>D: INSERT or UPDATE PROPERTIES (parameterized)
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-Medium] | IP addresses and User-Agent strings are personal data |
| Communication medium | [M-DB] | |
| Transport security | [TLS] | To the database, as configured by the operator |
| Authentication | Login flow | Metadata is written only by a flow that creates or reuses a session |
| Accessibility | [Internal] | Read back through the session management API, and by operators with database access |
| Authorization and Access Control | N/A | |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 8 | [Tampering] | A crafted User-Agent injects SQL into the session write | [No] | All writes use parameterized queries, and the value is JSON encoded |
| 9 | [Denial of Service] | A very large User-Agent grows the session row on every login | [No] | The User-Agent is capped at 512 characters |
| 10 | [Tampering] | Invalid UTF-8 in the User-Agent breaks JSON encoding or a later display | [No] | Invalid UTF-8 is dropped before the value is recorded |
| 11 | [Denial of Service] | A corrupted `PROPERTIES` value stops a session from loading, blocking SSO or sign-out | [No] | An unreadable value decodes to empty properties and the session loads normally |
| 12 | [Spoofing] | An app-native flow run from the application's backend records the application server's IP and User-Agent instead of the user's | [Yes] | Bounded: a display inaccuracy only. Recorded as an open question in the specification and as a residual |
| 13 | [Privacy Risk] | IP and User-Agent are kept longer than needed | [Yes] | Partly bounded: they are deleted with the session row on sign-out or revocation. There is no bounded default for expired rows. On PostgreSQL the cleanup procedure removes them only when the operator schedules it, and SQLite has no cleanup, so with nothing scheduled every sign-in that is never ended leaves two IPs and a User-Agent in the database. Recorded as a residual |

#### 03: Writing the client IP to the access log

**Description**

The access log adds the resolved client IP as the last field of each line, or `-` when there is none. The connecting address stays the first field.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| Client | `RequestMetadataMiddleware` | Access log |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant C as Client
  participant M as RequestMetadataMiddleware
  participant L as Access log
  C->>M: request
  M->>L: connecting address, ..., correlation ID, client IP
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-Medium] | The connecting address was already logged. Behind a trusted proxy, the new field adds the client's own IP |
| Communication medium | [M-FS] | |
| Transport security | N/A | |
| Authentication | None | |
| Accessibility | [Restricted] | Operator only |
| Authorization and Access Control | Operator log access | |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 14 | [Tampering] | A forged header injects text or new lines into the access log | [No] | Only a value that parses as an IP is logged, in canonical form, so no header text reaches the log |
| 15 | [Repudiation] | A spoofed client IP misleads an investigation | [No] | The connecting address, which cannot be spoofed, stays the first field |
| 16 | [Repudiation] | The audit log and the session name different addresses for one request, so an investigator does not know which to rely on | [Yes] | The audit `client` (#6580) is recorded only when the connecting address is public, while the session and the access log's last field use the address from a trusted proxy's header. Behind a proxy the two can differ. The access log has both the connecting address and the client IP, so it is the record to correlate them. Using one rule for both is recorded as a residual |

#### 04: Listing sessions as an administrator

**Description**

An administrator calls `GET /sessions` with exactly one of `userId` and `appId`, directly or from the Console Sessions tab. The security middleware requires the `system:session:view` permission. The service returns one page of live sessions with their participants, and the handler resolves user and application names.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| Administrator | Security middleware, `sessionmgt` handler, session service | `SESSION`, `SESSION_PARTICIPANT`, user and application services |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant A as Administrator
  participant M as Security middleware
  participant H as sessionmgt handler
  participant S as Session service
  A->>M: GET /sessions?userId=..., bearer token
  M->>M: validate token, require system:session:view
  M->>H: request
  H->>H: validate filter, limit, offset
  H->>S: ListLiveBySubject(userId, now, limit, offset)
  S->>H: page of live sessions with participants
  H->>H: resolve user and application names
  H->>A: 200, sessions without handles or tfids
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-Medium] | User names, IPs, User-Agents, and application names |
| Communication medium | [M-NT] | |
| Transport security | [TLS] | |
| Authentication | Bearer access token | |
| Accessibility | [Restricted] | |
| Authorization and Access Control | `system:session:view` permission | Enforced by the security middleware before the handler runs |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 17 | [Elevation of Privilege] | A user without `system:session:view` lists another user's sessions through `/sessions` | [No] | `GET /sessions` has its own permission entry, so the security middleware requires `system:session:view` and returns `403`. A holder of `system:session:view` reads the sessions of every user, which is the intended reach |
| 18 | [Information Disclosure] | A listing leaks a value that lets the reader take over a session, such as its handle or token family id | [No] | The response model carries no handle, checkpoint context, or token family id. The session `id` is the internal `SESSION_ID`, the same value as the `sid` claim. No flow or SSO lookup accepts it, so it can name a session but not resume one |
| 19 | [Denial of Service] | A large page or a deep offset makes each listing expensive | [No] | `limit` is capped at 100, and the queries are backed by `idx_session_subject` and `idx_session_participant_app`. Each page runs a count first, and an `offset` at or past that count is answered from the count alone, so a deep offset costs no scan. Participants for a page are read in one query, with one user and one application lookup per distinct id. Request volume is left to the front door (checklist 2) |
| 20 | [Information Disclosure] | An expired session, or a session that is not an SSO session, is shown as live, so an administrator misjudges what can still be used | [No] | Only `ACTIVE` SSO sessions within both deadlines are listed. The listing queries filter on `TYPE = 'SSO'`, so `DIRECT` sessions planned in #6322 are not listed. Revoking all of a user's sessions reads every type, so no session of the user is left behind |

#### 05: Listing my own sessions

**Description**

A signed-in user calls `GET /users/me/sessions`. The user is taken from the access token, and no user id is accepted from the request. Each session is marked `isCurrent` when one of its participants holds the token family in the access token's `tfid` claim.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| End user | Security middleware, `sessionmgt` handler, session service | The caller's rows in `SESSION` and `SESSION_PARTICIPANT` |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant U as End user
  participant M as Security middleware
  participant H as sessionmgt handler
  participant S as Session service
  U->>M: GET /users/me/sessions, bearer token
  M->>M: validate token
  M->>H: request, subject and tfid from token
  H->>S: ListLiveBySubject(subject, now, limit, offset)
  S->>H: page of the caller's live sessions
  H->>H: isCurrent = a participant holds tfid
  H->>U: 200
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-Medium] | The caller's own IPs, User-Agents, and applications |
| Communication medium | [M-NT] | |
| Transport security | [TLS] | |
| Authentication | Bearer access token | |
| Accessibility | [Public] | Any signed-in user |
| Authorization and Access Control | Subject from the token | The route accepts no user id |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 21 | [Information Disclosure] | A user lists another user's sessions by passing a user id | [No] | The handler reads the subject from the access token and ignores any user id. The query filters on that subject. Any access token the REST gate accepts reaches the listing, including one issued to another application or a delegated token, and it shows only that subject's sessions |
| 22 | [Spoofing] | A client marks a session of its choice as current, to steer a later `exceptCurrent` revocation | [No] | `isCurrent` comes from the token's `tfid` claim, which is signed into the token. The request carries no session id for it |
| 23 | [Information Disclosure] | The self-service listing reveals the names of other users or internal ids | [No] | User names are not resolved on this route, and the response carries no handle or token family id |

#### 06: Revoking sessions as an administrator

**Description**

An administrator calls `DELETE /sessions/{sessionId}` or `DELETE /sessions?userId=`, directly or from the Console. The service runs the sign-out termination for each session in one transaction: it reads the participants, revokes their token families, deletes the session, its contexts and its participants, and notifies termination listeners after commit.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| Administrator | Security middleware, `sessionmgt` handler, session service | `SESSION`, `SESSION_PARTICIPANT`, token families, termination listeners |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant A as Administrator
  participant M as Security middleware
  participant S as Session service
  participant D as Database
  participant R as Criteria revoker
  participant L as Termination listeners
  A->>M: DELETE /sessions/{id}, bearer token
  M->>M: validate token, require system:session:delete
  M->>S: RevokeSession(id)
  S->>D: begin, read participants
  S->>R: revoke each participant's token family
  S->>D: delete session, contexts, participants, commit
  S->>L: notify, reason session_revocation
  S->>A: 204
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-Low] | No data is returned |
| Communication medium | [M-NT], [M-DB] | |
| Transport security | [TLS] | |
| Authentication | Bearer access token | |
| Accessibility | [Restricted] | |
| Authorization and Access Control | `system:session:delete` permission | |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 24 | [Elevation of Privilege] | A user without `system:session:delete` revokes another user's sessions through `/sessions` | [No] | The security middleware requires `system:session:delete` on `DELETE /sessions` and `DELETE /sessions/{id}`, so an administrator who can only view sessions can't revoke them |
| 25 | [Security Risk] | A revoked session keeps working because its tokens stay valid, or a crash between steps leaves live tokens for a deleted session | [Yes] | Bounded: token families are revoked first, in the same transaction that deletes the session, so either both happen or neither does. The `204` means the session is deleted and its families are revoked in the database. The refresh grant and introspection read the database, so they refuse the tokens at once. The REST gate reads a cache that syncs every `server.security.token_revocation.sync_interval_seconds`, 60 by default, so it can accept an access token for up to that long. A relying party that validates access tokens itself accepts them until they expire. With token revocation turned off, only the session is removed. Documented in the API reference |
| 26 | [Security Risk] | An application keeps a local session or live tokens after its SSO session is revoked | [Yes] | Bounded: termination listeners are notified after commit, and back-channel logout sends a logout token to each participant that registered an endpoint. A notification dropped on a full queue or at shutdown is recorded as a residual in the back-channel logout threat model. Under read committed, a participant that joins between the participant read and the delete is neither notified nor revoked: its row is deleted with the session, but its token family stays valid until its tokens expire. Two revocations of one session both revoke the families, and only the one whose delete removed the row notifies listeners. Recorded as a residual |
| 27 | [Security Risk] | Tokens from an earlier grant of the same application, or tokens not issued through a session, survive the revocation | [Yes] | Bounded: every sign-in through the session, a silent `prompt=none` sign-in included, starts a new token family, and the participant row keeps only the latest. A revocation reaches only that one, and the earlier families expire on their own lifetime. There is no `offline_access` exemption: a refresh token from an authorization code grant carries its sign-in's family and ends with it. Tokens from the device and CIBA grants carry no token family, and neither does an exchanged token unless `oauth.token_exchange.token_family` is `inherit`, so a revocation does not reach them, their refresh tokens keep working, and the listing does not show them. Client credentials tokens belong to the application, not the user. User suspension, user delete and the token revocation endpoint do reach them. Documented in the API reference, and tracked in #3403 |
| 28 | [Repudiation] | An administrator revokes a user's sessions without a record of who did it | [Yes] | Not mitigated: the access log line has the path, without the query string, and no caller, so neither the administrator nor the target user of `DELETE /sessions?userId=` is in it. Termination listeners receive the `session_revocation` reason without the caller. Listing and revoking are not recorded as audit events. Recorded as a residual |

#### 07: Revoking my own sessions

**Description**

A signed-in user calls `DELETE /users/me/sessions/{sessionId}`, or `DELETE /users/me/sessions` with an optional `exceptCurrent=true`. The user is taken from the access token, and a delegated token is refused. Revoking one session checks that it belongs to the caller. Keeping the current session uses the token's `tfid`, and when no session of the caller holds it, nothing is revoked.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| End user | Security middleware, `sessionmgt` handler, session service | The caller's sessions, their token families, termination listeners |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant U as End user
  participant H as sessionmgt handler
  participant S as Session service
  U->>H: DELETE /users/me/sessions?exceptCurrent=true, bearer token
  H->>H: subject and tfid from token
  H->>S: RevokeSubjectSessions(subject, tfid)
  S->>S: find the session holding tfid
  alt no session holds tfid
    S->>H: ErrTokenFamilyNotInSession
    H->>U: 400 SSM-1006, nothing revoked
  else found
    S->>S: terminate every other session
    H->>U: 204
  end
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-Low] | No data is returned |
| Communication medium | [M-NT], [M-DB] | |
| Transport security | [TLS] | |
| Authentication | Bearer access token | |
| Accessibility | [Public] | Any signed-in user |
| Authorization and Access Control | Ownership | The session must belong to the subject in the token |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 29 | [Elevation of Privilege] | A user revokes another user's session by guessing or reusing its id | [No] | The service revokes only a session whose subject matches the token's subject, so knowing an id is not enough (threat 36) |
| 30 | [Information Disclosure] | The response to revoking another user's session reveals that it exists | [No] | A session of another user and a session that doesn't exist both return `404` with `SSM-1007` |
| 31 | [Elevation of Privilege] | The self-service permission entries open a broader route, such as `DELETE /users/me` | [No] | Only `DELETE /users/me/sessions` and `DELETE /users/me/sessions/**` map to no permission. Tests confirm `DELETE /users/me` still needs the user permission |
| 32 | [Denial of Service] | A user asking to stay signed in is signed out of every session, because their token identifies no session | [No] | When no session holds the token's `tfid`, the request fails with `SSM-1006` and nothing is revoked |
| 33 | [Security Risk] | A stolen or left-behind access token is used to sign the real user out of their other sessions while keeping its own | [Yes] | Bounded: the token already gives that reach over the account. The real user can sign in again, and revoking the stolen token's session ends its grant. The routes ask for no recent authentication, which ASVS 5.0 V7.5.2 asks for at level 2, and an access token carries no `auth_time` to check. Recorded as a residual |
| 34 | [Elevation of Privilege] | A delegated token, issued to a client acting for the user, ends the user's sessions | [No] | `DELETE /users/me/sessions` and `DELETE /users/me/sessions/{id}` refuse a delegated token with `403` and `SSM-1008`, as `/users/me/update-credentials` does since #6702 |
| 35 | [Security Risk] | An access token whose session was just revoked still calls the REST API until the gate's revocation view refreshes | [Yes] | Bounded: for up to the cache sync interval, 60 seconds by default, the token reaches every route its permissions allow, including `/users/me/sessions`, `/users/me/update-credentials` and, for an administrator's token, `/sessions`. In that window the person who was signed out can list and end the user's other sessions. `exceptCurrent=true` revokes nothing, since no session holds the token's family any more. The token cannot be refreshed. With token revocation turned off, the window lasts until the token expires. Recorded as a residual |
| 36 | [Elevation of Privilege] | A relying party learns the session id from the `sid` claim in its ID token, which equals the session `id` these routes accept, and uses it to end the user's session | [No] | The id alone authorizes nothing: `DELETE /sessions/{id}` needs `system:session:delete`, and `/users/me/sessions/{id}` needs the user's own access token and checks ownership. With those, the relying party can only cause the sign-out it could already start through the end session endpoint |

#### 08: Managing sessions in the Console

**Description**

The Sessions tab on a user or application page calls the administrator API with the signed-in administrator's token. The Console of a standalone server shows it on both pages, and the Admin Console on the user page. The Control Plane console shows no Sessions tab. It describes each session's device from its User-Agent, and asks for confirmation before a revocation.

**Assets involved**

| Initiator | Intermediate | Target |
| --- | --- | --- |
| Administrator | Console | Session management API |

**Data flow**

```mermaid
sequenceDiagram
  autonumber
  participant A as Administrator
  participant C as Console Sessions tab
  participant API as Session management API
  A->>C: open the user's Sessions tab
  C->>API: GET /sessions?userId=..., Console token
  API->>C: page of sessions
  A->>C: Revoke, then confirm
  C->>API: DELETE /sessions/{id}
  API->>C: 204, list refreshed
```

**Security considerations**

| Area | Response | Comments |
| --- | --- | --- |
| Data confidentiality | [C-Medium] | Shows user names, IPs, and devices |
| Communication medium | [M-NT] | |
| Transport security | [TLS] | |
| Authentication | The Console's access token | |
| Accessibility | [Restricted] | |
| Authorization and Access Control | `system:session:view` to list and `system:session:delete` to revoke, enforced by the API | The Console grants no access of its own |

**Threat assessment**

| ID | Category | Threat | Materializable | Mitigation / comment |
| --- | --- | --- | --- | --- |
| 37 | [Tampering] | A crafted User-Agent injects markup or script into the Console, or into another client that shows sessions | [No] | The Console parses the User-Agent into fixed browser, OS, and device names, and React renders every value as text. The API returns the raw `userAgent`, which the API reference marks as client supplied and to be rendered as text only. SDK components are covered by the SDK threat model |
| 38 | [Spoofing] | A user copies the victim's User-Agent, so a session that is not the victim's looks like their own device | [Yes] | Bounded: the device name is a hint, not proof. The sign-in IP, the last active IP, the sign-in time and the applications are shown with it, and are harder to copy. Recorded as a residual |
| 39 | [Elevation of Privilege] | Hiding the tab is relied on to keep sessions from a user without `system:session:view` | [No] | The API enforces the permission on every call, and the tab shows a no-permission message when it refuses one. The tab is hidden on applications users can't sign in to only to avoid an empty view |
| 40 | [Process Risk] | An administrator revokes the wrong sessions by mistake | [No] | Each revocation asks for confirmation first, and the dialog says what it does: the user is signed out on that device, or on every device for Revoke all. Revoke all is offered only on a user's own page |

## Security Review Checklist

A review aid that complements the threat models and the self-assessment. Guidance follows the [OWASP Top 10 Proactive Controls](https://top10proactive.owasp.org/).

### Security considerations

| # | Consideration | State | Comments |
| --- | --- | --- | --- |
| 1 | Are all inputs and outputs validated (syntactic and semantic)? | [Yes] | IPs are parsed, the User-Agent is sanitized and capped, config is validated at startup, and filter, `limit`, `offset` and `exceptCurrent` are validated |
| 2 | Are rate limits in place where necessary? | [No] | ThunderID applies no rate limit to its APIs, and the production guidelines place route-specific limits at the API gateway, ingress or front door. Listing is paged and capped, and an `offset` past the total costs only the count (threat 19). Revocation is idempotent |
| 3 | Are permissions, roles, and entitlements defined on the principle of least privilege and business need? | [Yes] | Listing needs `system:session:view` and revoking needs `system:session:delete`, so a role can view sessions without revoking them. Users reach only their own sessions |
| 4 | Are authentication and authorization validated at both the UI and API layers, front end and back end, before granting access to resources? | [Yes] | The API enforces them on every call. The Console relies on the API |
| 5 | Are proper isolations in place between components to ensure least-privilege access and reduce the blast radius against lateral movement? | [Yes] | The client IP is never read by a security decision. Self-service routes take the subject only from the token |
| 6 | Have any default credentials been changed, and are default superuser or root accounts not in use (when using third-party components)? | [N/A] | |
| 7 | Has the implementation followed best-practice guidelines (OWASP, Kubernetes, vendor, or technology provider)? | [Yes] | Right-to-left `X-Forwarded-For` parsing with a trusted proxy list, and object-level authorization on the self-service routes |
| 8 | Are secrets, credentials, and internal-only material kept out of the public source tree and its git history? | [Yes] | |
| 9 | Was a security-focused code review conducted for this change, and have the findings been addressed? | [Partial] | #5575 is reviewed and merged. #6150, #6152, and #6159 are still under review |
| 10 | Is Static Analysis (SAST) or IaC scanning conducted, and are findings addressed? | [Yes] | Repository CI |
| 11 | Is Software Composition Analysis (SCA) conducted or integrated into the repository, and are findings addressed (for example FOSSA, Trivy)? | [Yes] | Repository CI. No new dependency |
| 12 | Is Dynamic (DAST) or API scanning conducted on a non-production setup, and are findings addressed? | [No] | Covered by integration tests only |
| 13 | Are audit logs generated in a standardized format for critical functionality, and available to authorized users to trace critical events and aid incident response? Note the retention period in Comments. | [No] | The access log records each request with the client IP, but not the caller or the query string, so it does not show who revoked whose sessions (threat 28). No audit event is emitted for a listing or a revocation. Retention is the operator's |
| 14 | Do audit logs for critical configuration changes record the difference between the old and new versions? | [N/A] | Deployment config only |
| 15 | Are data in transit and at rest encrypted? | [Partial] | In transit with TLS. The properties are stored in plain text, so at rest depends on the operator's database encryption |
| 16 | Are sensitive values such as credentials and keys stored in a secret store or vault? | [N/A] | |
| 17 | Is personal, sensitive, or confidential data kept out of logs? | [Partial] | The access log gets the resolved client IP as a new field. Behind a proxy that is the client's own IP, which was not logged before. Its retention and access are the operator's. No User-Agent is logged |
| 18 | Have users been given clear instructions for secure usage? | [Yes] | The configuration reference and the Docker production guide cover `server.trusted_proxies`, and the sessions guide covers revocation |

### Business impact and resilience

For an open-source component, most of these are shared with the operator who deploys it.

| # | Consideration | State | Comments |
| --- | --- | --- | --- |
| 1 | Has a business impact analysis been done to identify resilience requirements (maximum tolerable downtime, uptime, RPO, RTO)? | [Partial] | No new availability requirement, but the upgrade has a window. The renamed `SESSION` tables with the required `TYPE` column (#6331) and the `PROPERTIES` column must exist before the new version starts. A node on the old version fails to write to the new schema, and a node on the new version fails on the old one. Either way every session query fails, the default Console sign-in included, until the schema and every node match. The schema change and the version switch must happen together, with no mixed versions serving traffic |

### Dependency and component health

| # | Consideration | State | Comments |
| --- | --- | --- | --- |
| 1 | Are dependencies, base images, and runtimes monitored for known vulnerabilities and kept current (for example automated dependency scanning), and are findings addressed? | [Yes] | Uses the standard library and the existing `golang.org/x/net` |
| 2 | Are any End-of-Life or End-of-Service components in use? | [No] | |
| 3 | Is hardening guidance published for operators who deploy the project (optional)? | [Yes] | Docker production guide |

### Privacy considerations

| # | Consideration | State | Comments |
| --- | --- | --- | --- |
| 1 | Is the purpose and legal basis for processing personal data clearly defined? | [Partial] | The purpose is defined: letting users and administrators recognize and end sessions. The legal basis is for the deploying organization to assess, and security is a possible legitimate interest (GDPR Recital 49) |
| 2 | Are the collection, storage, processing, sharing, archival, and disposal of personal data aligned with the data minimization principle? | [Yes] | Only the User-Agent and two IPs. No geolocation |
| 3 | Is personal data stored securely? | [Yes] | In the runtime persistent database, with the operator's access control |
| 4 | Are privacy notices updated to reflect any new processing or changes to purpose and legal basis? | [N/A] | The deploying organization's notice |
| 5 | Is access to personal data granted on a need-to-know basis? | [Yes] | Users see only their own sessions. Administrators need `system:session:view` |
| 6 | Are data retention requirements considered? | [Partial] | Deleted with the session on sign-out or revocation. Revoking all of a user's sessions also removes their expired rows. Otherwise expired rows are removed by the PostgreSQL cleanup procedure, when the operator schedules it |
| 7 | Is there a process to dispose of personal data on request in a timely manner while meeting retention requirements? | [Yes] | Revoking the user's sessions deletes the rows and the metadata on them |
| 8 | Are records of personal-data processing maintained in the project's data inventory or records of processing? | [N/A] | The deploying organization's records |

## Residual risks (open items)

- **A trusted range that is too wide lets a client choose the recorded IP (threat 4).** So does a proxy that passes the client's header through. Bounded to display and logging, since no decision reads the IP. The configuration reference asks for the narrowest ranges. A startup warning for `0.0.0.0/0` and `::/0` is tracked in #ISSUE-WIDE-RANGE.
- **The audit log and the session can name different addresses for one request (threat 16).** The access log has both. One rule for both is tracked in #ISSUE-CLIENT-IP-RULE.
- **App-native flows called from an application backend record the backend's IP and User-Agent (threat 12).** A display inaccuracy only, recorded as an open question in the specification (tracking: [#4130](https://github.com/thunder-id/thunderid/discussions/4130#discussioncomment-19390574)).
- **Expired sessions keep their metadata until they are cleaned up (threat 13).** There is no bounded default. On PostgreSQL, the cleanup procedure removes sessions past their absolute deadline only when the operator schedules it. SQLite has no cleanup procedure, and an idle-expired session on either database waits for its absolute deadline. Expired sessions are never listed. Cleanup runs outside the server by design, so the gap is a cleanup for SQLite and guidance on scheduling the procedure, tracked in #ISSUE-RETENTION.
- **A revocation may not reach every token (threats 25, 26, 27 and 35).** The REST gate can accept an access token for up to the cache sync interval, on every route the token's permissions allow, and a relying party that validates tokens itself until they expire. A participant that joins between the participant read and the delete keeps its token family. Only each application's latest grant in the session is revoked, and earlier grants stay valid until they expire. Tokens from the device and CIBA grants, and exchanged tokens that do not inherit the family, are not reached at all. Shared with sign-out, and tracked in #3403.
- **No audit event records who listed or revoked a session (threat 28).** The access log has neither the caller nor the target user. Session events with the trigger, the actor and the participants are tracked in #5205, under the audit log work in #5591.
- **A stolen access token can sign the user out of their other sessions (threat 33).** No recent authentication is asked for before a self-service revocation. Access tokens carry no `auth_time` yet, so the check depends on the step-up support tracked in #5846.
- **A copied User-Agent makes a session look like the victim's device (threat 38).** The IPs, the sign-in time and the applications shown with it help tell sessions apart.

## Appendix

- Sample configuration:

  ```yaml
  server:
    trusted_proxies:
      - cidr: "10.0.1.0/24"
        header: "x-real-ip"
      - cidr: "172.28.0.10/32"
  ```

- References: [MDN X-Forwarded-For](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For), [RFC 7239](https://www.rfc-editor.org/rfc/rfc7239), [OWASP API Security Top 10, API1 Broken Object Level Authorization](https://owasp.org/API-Security/editions/2023/en/0xa1-broken-object-level-authorization/), [OWASP Top 10 Proactive Controls](https://top10proactive.owasp.org/), and the companion model [back-channel-logout/threat-model.md](../back-channel-logout/threat-model.md).
- Design discussions: [Session visibility](https://github.com/thunder-id/thunderid/discussions/4130), and [Session tracking, token lifecycle and session management](https://github.com/thunder-id/thunderid/discussions/6322). 
- Feature issue [#3400](https://github.com/thunder-id/thunderid/issues/3400).

## Change log

| Version | Date | Change |
|---|---|---|
| 0.1 | 2026-11-04 | Initial threat model, from design discussion #4130, as built in #5575, #6150, #6152 and #6159. |
