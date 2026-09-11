# Dynamic Client Registration Management Specification

- **Status:** Draft
- **Version:** 0.2
- **Related documents:**
  - Feature issue: [#5150](https://github.com/thunder-id/thunderid/issues/5150)
  - [RFC 7592](https://www.rfc-editor.org/rfc/rfc7592) OAuth 2.0 Dynamic Client Registration Management Protocol
  - [RFC 7591](https://www.rfc-editor.org/rfc/rfc7591) OAuth 2.0 Dynamic Client Registration Protocol
  - [RFC 6750](https://www.rfc-editor.org/rfc/rfc6750) OAuth 2.0 Bearer Token Usage
  - [threat-model.md](threat-model.md)

## Summary

ThunderID supports Dynamic Client Registration (RFC 7591): a client creates its OAuth registration
with a single HTTP request to the registration endpoint. Once created, that registration is frozen
from the client's point of view. There is no standards-based way for the client to read back what
was registered, change a redirect URI, or remove itself. The only available route is a
ThunderID-specific administrative API, so a client provisioned through an open standard must switch
protocols and obtain administrative authorization to manage what it already owns.

This specification adds the RFC 7592 Client Configuration Endpoint, which completes the lifecycle
that dynamic registration starts. When registration access tokens are enabled, a successful
registration also returns a `registration_access_token` and a `registration_client_uri`. Presenting
that token at that URI lets the client read its current registration, replace its metadata, or
delete itself.

Token issuance is off by default. In the default configuration the endpoint exists and is reachable,
but only an administrative caller can use it, and registration responses carry no token. Self-service
management is something a deployment turns on deliberately. The reasoning is in
[Registration access token issuance](#registration-access-token-issuance) below.

In scope: the three configuration operations, the token that authorizes them, the setting that
governs whether tokens are issued at all, the registration response fields, and the configuration
that governs token lifetime.

Out of scope: returning the client secret on read, and expressing ThunderID-specific application
properties such as permitted user types through dynamic registration. Two further capabilities are
designed but deliberately deferred, with their reasoning recorded in
[Deferred: single-use tokens](#deferred-single-use-tokens): revoking an individual token, and
consuming a token on update.

## Architecture

The feature extends the existing dynamic client registration component rather than introducing a new
one. The registration endpoint and the client configuration endpoint share the same service, the same
client metadata model, and the same error vocabulary, so a registration response and a client
information response are the same shape.

Three seams already existed and are reused unchanged:

- **Application lifecycle.** Reading, replacing and deleting a registered client are expressed as
  operations on the underlying application record. The configuration endpoint owns no storage of its
  own; the application component remains the single owner of client state.
- **Token issuance and verification.** The registration access token is minted and verified through
  the same signing service that issues every other token this deployment produces, so it inherits the
  deployment's signing key, algorithm selection and issuer identity.
- **Request authorization.** The administrative permission check already used by the registration
  endpoint is reused as a second accepted credential at the configuration endpoint.

Ownership boundary: the dynamic client registration component owns the protocol surface, which means
translating RFC 7591 metadata to and from the application model, deciding what a client may change,
and issuing and validating the registration access token. It does not own client state, credential
storage, or signing keys.

Diagrams:

- [Component architecture](assets/architecture.md), including ownership boundaries.
- [Registration lifecycle](assets/lifecycle.md), including the token's state at each stage.
- [Authorization decision flow](assets/authorization-flow.md), including why the checks run in the
  order they do.

## Detailed design

### Registration access token issuance

Whether a registration returns a management token is a deployment decision, and the default is not to
issue one.

A registration access token is a long-lived bearer credential that authorizes reading, replacing and
deleting a registration. It cannot be revoked individually (see the deviations below), so a
deployment that issues tokens accepts an exposure window bounded only by the token's lifetime. Most
deployments do not need self-service management: their clients are provisioned once and administered
through the existing administrative API. Issuing a durable credential to every registrant by default
would spend that risk on their behalf without their asking.

Turning issuance off does not remove the endpoint. All three operations remain available to an
administrative caller holding the system permission, which is the same credential that already
governs the registration endpoint. What changes is only whether a client is handed a credential of
its own.

The setting governs **issuance alone, never validation**. A token minted while issuance was enabled
keeps working if issuance is later turned off, until it expires or its client is deleted. Turning the
setting off is a decision about new registrations, not a way to invalidate credentials already in the
field; a deployment that needs the latter deletes the affected registrations. This also means the
setting can be turned off without breaking clients that are actively managing themselves.

Conformance testing is the clearest case for turning it on: the OpenID Connect conformance suite
registers its relying parties dynamically and then manages them through the configuration endpoint,
so a conformance deployment enables issuance in the same way it enables anonymous registration.

### Registration access token

The token is a signed JWT. Its subject is the `client_id` it manages, which is what binds it to a
single registration. Its audience is that client's configuration endpoint URI, so the token names the
resource it is good for. Its type header marks it as a registration access token, distinct from an
access token.

Validation runs in a fixed order, and the order is deliberate:

1. **Type first.** The token is rejected unless its type header marks it as a registration access
   token. Checking this before anything else means an ordinary access token is refused as
   unauthorized rather than being carried further into the flow.
2. **Signature, expiry and issuer.** Verified through the standard signing service.
3. **Subject last.** The subject must equal the client identifier in the request path.

The audience is intentionally **not** asserted during verification, even though it is present in the
token. Asserting it would make a token issued for another client fail as malformed, reporting the
wrong condition. Leaving it to the subject check produces a correct "you may not manage this client"
result instead. The audience remains in the token as a statement of intended scope for any relying
party that inspects it.

Failure behavior: a missing, malformed, expired or unverifiable token is unauthorized. A valid token
whose subject names a different client is forbidden. The distinction matters because the two
conditions call for different client behavior, retry with a correct token versus stop.

Lifecycle: the token is issued at registration, when issuance is enabled, and returned again
unchanged on every read and update, so a client that has the current token always has a usable one.
It is never rotated and never consumed, so a read is safe to repeat and a client that fails to
persist a response is not locked out of its own registration. RFC 7592 permits rotating the token on
a read or update; the reasoning for not doing so, and the design that would, is recorded in
[Deferred: single-use tokens](#deferred-single-use-tokens). The token expires after a configurable
period. It is not revocable individually; see the deviations below.

The token is not the client secret and confers no capability beyond managing the one registration it
names. In particular it cannot be used at the token endpoint, and an access token cannot be used at
the configuration endpoint.

### Client resolution and authorization

Every operation follows the same sequence: resolve the client named in the request path, then
authorize the caller against that specific client, then act. The full decision flow, and the
reasoning behind the order of checks, is in
[assets/authorization-flow.md](assets/authorization-flow.md).

Resolving before authorizing is what makes a token inert once its client is gone. A deleted client no
longer resolves to anything, so its token has nothing left to authorize against and every operation
reports the client as not found. This is how the requirement that the token stop working after
deletion is satisfied without storing the token.

Two credentials are accepted:

- the client's own registration access token, which authorizes management of that client only;
- an administrative caller holding the system permission, which authorizes management of any client.

The administrative path is reached only after registration access token validation fails for a reason
other than "this token belongs to another client". A token that is valid but names a different client
is a permission failure and stops there. It must not fall through to the administrative check,
because doing so would let a valid client token be evaluated against a completely different
authorization rule.

### Read

Returns the client's currently registered metadata, together with the registration access token and
the configuration endpoint URI, so a single read gives the client everything it needs to continue
managing itself.

The response is assembled from two sources: the OAuth client record supplies the protocol fields
(redirect URIs, grant and response types, authentication method, scopes, keys), and the application
record supplies the human-readable metadata (name, home page, logo, terms, policy, contacts). Both
are needed because neither holds the complete picture.

The client secret is absent. See the deviations below.

### Update

An update **replaces** the registered metadata rather than merging it, because the underlying
application record is replaced wholesale. Any field the client omits is cleared, not retained. The
client is therefore expected to send the complete set of metadata it wants to end up with, which is
consistent with RFC 7592 treating the update as a full replacement of client metadata.

Three properties are exempt from replacement and are carried forward from the existing registration,
because losing them would break the registration rather than change it:

| Property | Why it is carried forward |
|---|---|
| Client identifier | The registration must keep its identity. Allowing it to lapse would issue a new identifier and orphan the client's own configuration URI and any registered key material. |
| Client secret | A metadata change must not silently invalidate the client's credentials. The update deliberately supplies no secret, which is what preserves the stored one. |
| Owning organization unit and application type | Neither is expressible through RFC 7591 metadata, and the application type is immutable, so both are inherited rather than reasserted. |

The client name is also inherited when the request omits it, because a name is required on the
underlying record and an omitted name would otherwise fail validation rather than clear the field.

Server-managed registration fields are ignored if the client sends them: the registration access
token, the configuration URI, the issuance timestamp and the secret expiry. They describe the
registration rather than configure it, so accepting them would let a client assert things the server
alone determines.

A `client_id` in the request body is permitted and, per RFC 7592, expected. When present it must
identify the client being updated. A mismatch is rejected as invalid client metadata rather than
ignored, because it signals that the client believes it is updating something other than what the URI
names.

Metadata that fails validation is rejected with the same errors the registration endpoint already
uses, so a client sees consistent behavior whether it is registering or updating.

### Delete

Removes the registration. The client is resolved and authorized first, so deleting an unknown client
reports not found rather than succeeding silently. On success the response carries no body.

After deletion the registration access token still verifies cryptographically, because nothing about
it has changed, but it no longer resolves to a client and therefore authorizes nothing. Every
subsequent operation reports the client as not found.

### Deferred: single-use tokens

A registration access token that is consumed when used cannot be replayed, which closes most of the
exposure a long-lived bearer credential carries. The design is settled and recorded here, but is not
part of this feature.

**The design.** An update consumes the presented token and returns a fresh one in the same response.
The consumed token is recorded on the platform's existing single-token deny list, the same mechanism
that already enforces single-use refresh tokens, and validation consults it before accepting a token.
No new storage is required: the deny list is keyed by token identifier, already carries an expiry
per entry, and is already swept by the existing cleanup routine. The only schema change is admitting
a new revocation reason.

**Consume before acting.** The write to the deny list is what makes the operation exclusive, so it
has to happen before the update is applied rather than after. Two concurrent replays then resolve
deterministically: whichever request records the token first proceeds, and the other is rejected. The
consequence is that a failed update still spends the token, which is the correct trade, since the
alternative leaves a window in which a token can be used twice.

**Reads are exempt.** Only updates consume a token. RFC 7592 permits rotation on a read as well, but
a read that invalidates the caller's credential is not safely repeatable, and a client that polls its
own registration would spend a token each time. A replayed read returns data its caller already holds
and changes nothing; a replayed update reapplies a full metadata replacement and can silently revert
a later legitimate change. Single-use is therefore spent where it prevents something and skipped
where it would only cost availability.

**Why it is deferred.** It introduces a failure mode this feature currently does not have: an update
whose response is lost leaves the client having spent its token without receiving the replacement,
and self-management is then recoverable only administratively. RFC 7592 permits server-side discard
of a rotated token precisely because it assumes the replacement reaches the client. That assumption
does not survive a lost response.

With issuance off by default, the exposure single-use would mitigate is not present in a default
deployment, so the availability cost buys correspondingly less. Enabling it will also require
confirming that clients which manage registrations in practice, including the OpenID Connect
conformance suite, follow a rotated token rather than reusing the one they were issued. When it
lands it will be a setting of its own, defaulting off, so that a deployment adopts replay protection
and the lockout risk together and deliberately.

### Protocol deviations

Points where the implementation knowingly departs from RFC 7592. Each is a consequence of a design
decision recorded above, not an oversight. The first applies whenever the endpoint is used; the rest
apply only to a deployment that has enabled token issuance, since a deployment that has not issues no
tokens to deviate over.

**Registration does not always return the management fields.** RFC 7592 section 3 marks
`registration_access_token` and `registration_client_uri` as required members of a client information
response. With issuance disabled, which is the default, both are absent and a registrant has no
self-service management route. A deployment that needs conformance with RFC 7592, including one being
certified against the OpenID Connect dynamic provider profile, enables issuance; the fields are then
present and the deviation does not apply.

**A submitted client secret is ignored rather than verified.** RFC 7592 section 2.2 says that a
`client_secret` included in an update must match the currently issued secret. ThunderID cannot
compare it, because the stored secret has no read path, so the field is ignored instead. The same
section forbids a client choosing its own secret, which ignoring the field satisfies by construction:
there is no path from the submitted value to storage. Failing this way is the safe direction, since
it never rotates a credential and never rejects a client that correctly echoed its own secret.

**The client secret is not returned on read or update.** RFC 7592's example client information
response includes `client_secret`. ThunderID stores the client secret write-only: it is set once at
registration and there is no read path anywhere in the platform. The secret is therefore omitted
rather than fabricated. A client that loses its secret has no recovery route through this endpoint
and must register again. Adding a credential read path was considered and rejected, because it would
open credential reads across the whole platform to serve one endpoint.

**The registration access token expires.** RFC 7592 says the token SHOULD NOT expire. A
self-contained token has to carry an expiry, so it does. The default is long (90 days) and the
period is deployment-configurable. A client that goes quiet for longer than the configured period
loses the ability to manage its registration.

Removing the expiry was considered and rejected. It is not expressible without changing the shared
token service, which stamps an expiry on every token it issues and rejects a token that lacks one;
that invariant holds for every token type in the platform and is not worth weakening for one. Expiry
is also what bounds an unrevocable token's exposure, and it is what would make a consumed token's
deny-list entry reclaimable were single-use adopted. A very long expiry remains available to an
operator who wants it, at the cost of a correspondingly long exposure window.

**An individual token cannot be revoked.** Deleting the client makes its token inert, which satisfies
the RFC's requirement that the token stop working when the registration is deleted. A token that
leaks while its client still exists, however, stays valid until it expires. Containing such a leak
means deleting the registration, which ends a working integration in order to withdraw a credential.

Closing this gap does not require new storage. The platform already maintains a single-token deny
list, keyed by token identifier and swept by expiry, which the same design sketched in
[Deferred: single-use tokens](#deferred-single-use-tokens) would use. Revocation and single-use are
the same mechanism applied to different triggers, and are most sensibly adopted together.

### API

Two changes: additional fields on the existing registration response, and a new per-client
configuration endpoint.

#### Registration response additions

A successful registration response gains three fields:

| Field | Description | Present |
|---|---|---|
| `registration_access_token` | Authorizes management of this registration. Bound to this client alone. Not the client secret and not recoverable if lost. | Only when issuance is enabled |
| `registration_client_uri` | The client configuration endpoint for this registration. | Only when issuance is enabled |
| `client_id_issued_at` | Time the client identifier was issued. | Always |

The two management fields are omitted together. A registrant either receives both and can manage
itself, or receives neither and cannot; there is no state in which it holds a token without knowing
where to present it.

#### Client configuration endpoint

Addressed per client, under the existing registration path:

```
/oauth2/dcr/register/{client_id}
```

The issue text illustrates `/oauth2/register/{client_id}`. The established ThunderID registration
endpoint is `/oauth2/dcr/register`, and relocating it would be a breaking change outside the scope of
this feature, so the configuration endpoint extends the existing path instead.

| Operation | Method | Success | Body |
|---|---|---|---|
| Read registration | `GET` | `200` | Current client metadata |
| Update registration | `PUT` | `200` | Updated client metadata |
| Delete registration | `DELETE` | `204` | None |

Authorization for all three: the `registration_access_token` as an RFC 6750 Bearer token, or an
administrative caller holding the system permission. The endpoint is registered regardless of whether
token issuance is enabled; with issuance off, the administrative credential is the only one that can
authorize a request, because no client holds a token.

Error responses:

| Status | Condition | Notes |
|---|---|---|
| `400` | Invalid client metadata, or a `client_id` that does not match the request path | Same error vocabulary as registration |
| `401` | Registration access token missing, malformed, expired or unverifiable | Carries a `WWW-Authenticate` challenge naming an invalid token |
| `403` | Token is valid but was issued for a different client | |
| `404` | No such registration, including one already deleted | |
| `500` | Server failure | |

The `401` and `403` responses share the RFC 6750 `invalid_token` error code, as that specification
prescribes; the status distinguishes them.

### Configuration

Two deployment-level settings, alongside the existing dynamic client registration settings:

| Key | Default | Description |
|---|---|---|
| `oauth.dcr.registration_access_token_enabled` | `false` | Whether registration issues a registration access token, and with it a configuration URI. |
| `oauth.dcr.registration_access_token_validity_period` | `7776000` (90 days) | Lifetime in seconds of a registration access token. |

Deployment-level only; there is no organization-level override.

Issuance defaults to off for the reasons in
[Registration access token issuance](#registration-access-token-issuance). It governs issuance alone:
tokens already issued keep working after it is turned off.

The validity period applies only when issuance is enabled. A long default is chosen because RFC 7592
says the token SHOULD NOT expire, and because the token is not individually revocable, the period is
also the only bound on a leaked token's exposure. Shortening it narrows that window at the cost of a
client that goes quiet for longer having to re-register. Lengthening it approaches the RFC's
preference at the cost of a longer exposure window.

Neither setting has any effect when dynamic client registration is disabled, since nothing is
registered.

## Requirements

### R1. Registration returns what a client needs to manage itself

**Requirement:** When token issuance is enabled, a client that registers dynamically receives, in the
registration response, a credential and a URI sufficient to read, update and delete its own
registration without administrative involvement.

**Acceptance criteria:**

- **AC1.1:** Given token issuance is enabled, when a client registers successfully, then the response
  contains a `registration_access_token` and a `registration_client_uri` addressing that client.
- **AC1.2:** Given a successful registration, when the response is inspected, then the response
  contains a `client_id_issued_at` timestamp, whether or not issuance is enabled.
- **AC1.3:** Given a successful registration, when the response is inspected, then the
  `registration_access_token` differs from the `client_secret`.
- **AC1.4:** Given a successful registration, when the returned `registration_access_token` is
  presented at the returned `registration_client_uri`, then the request is authorized.

### R1a. Token issuance is a deployment decision, and defaults to off

**Requirement:** A deployment controls whether registration issues management credentials. The
default is not to issue them, and the endpoint remains administratively usable either way.

**Acceptance criteria:**

- **AC1a.1:** Given no explicit configuration, when a client registers successfully, then the response
  contains neither a `registration_access_token` nor a `registration_client_uri`.
- **AC1a.2:** Given token issuance is disabled, when an administrative caller reads, updates or
  deletes a registration, then the request is authorized and succeeds.
- **AC1a.3:** Given token issuance is disabled, when a request carrying no credential is made to the
  client configuration endpoint, then it is rejected with `401`.
- **AC1a.4:** Given a token issued while issuance was enabled, when issuance is subsequently disabled,
  then that token continues to authorize its own registration until it expires or its client is
  deleted.

### R2. A client can read its own registration

**Requirement:** A client can retrieve its currently registered metadata using its registration
access token.

**Acceptance criteria:**

- **AC2.1:** Given a registered client, when it reads its registration with a valid registration
  access token, then the response returns `200` with the metadata as currently registered.
- **AC2.2:** Given a registered client, when it reads its registration, then the response contains no
  `client_secret`.
- **AC2.3:** Given a registered client, when it reads its registration, then the response contains
  the registration access token and configuration URI, so the client can continue managing itself
  from the read alone.

### R3. A client can update its own registration

**Requirement:** A client can replace its supported registered metadata using its registration access
token, without losing its identity or credentials.

**Acceptance criteria:**

- **AC3.1:** Given a registered client, when it updates its metadata with a valid registration access
  token, then the response returns `200` and the changed metadata is reflected in a subsequent read.
- **AC3.2:** Given a registered client, when its metadata is updated, then the `client_id` in the
  response and in a subsequent read is unchanged.
- **AC3.3:** Given a confidential client, when its metadata is updated, then the client secret issued
  at registration continues to authenticate the client afterwards.
- **AC3.4:** Given a registered client, when it submits an update whose body names a different
  `client_id` than the request path, then the request is rejected with `400`.
- **AC3.5:** Given a registered client, when it submits an update containing invalid client metadata,
  then the request is rejected with `400` and the registration is left unchanged.
- **AC3.6:** Given a registered client, when it submits an update that omits `client_name`, then the
  registered name is retained rather than cleared.

### R4. A client can delete its own registration

**Requirement:** A client can remove its registration using its registration access token, and the
token stops working once it has.

**Acceptance criteria:**

- **AC4.1:** Given a registered client, when it deletes its registration with a valid registration
  access token, then the response returns `204`.
- **AC4.2:** Given a client whose registration has been deleted, when its registration access token
  is presented again for that client, then the request is rejected with `404`.

### R5. A registration access token authorizes exactly one registration

**Requirement:** A registration access token permits management only of the client it was issued for,
and is validated independently of the access tokens issued to applications.

**Acceptance criteria:**

- **AC5.1:** Given two registered clients, when one client's registration access token is presented
  for the other client, then the request is rejected with `403` and the other client's registration is
  neither disclosed nor modified.
- **AC5.2:** Given a registered client, when a request carries no credential, then it is rejected with
  `401` and the registration is not disclosed.
- **AC5.3:** Given a registered client, when a request carries a malformed or unverifiable
  registration access token, then it is rejected with `401` and the registration is not disclosed.
- **AC5.4:** Given a valid OAuth access token that is not a registration access token, when it is
  presented at the client configuration endpoint, then it is not accepted as a registration access
  token.
- **AC5.5:** Given a rejected request at the client configuration endpoint, when the response is
  inspected, then it carries a `WWW-Authenticate` challenge consistent with Bearer token usage.

### R6. Token lifetime is a deployment decision

**Requirement:** The lifetime of a registration access token is configurable, and every issued token
carries an expiry.

**Acceptance criteria:**

- **AC6.1:** Given no explicit configuration, when a client registers with issuance enabled, then the
  issued registration access token is valid for the documented default period.
- **AC6.2:** Given a configured validity period, when a client registers with issuance enabled, then
  the issued registration access token expires after that period.
- **AC6.3:** Given an expired registration access token, when it is presented at the client
  configuration endpoint, then the request is rejected with `401`.
- **AC6.4:** Given any issued registration access token, when it is inspected, then it carries an
  expiry claim.

## Change log

| Version | Date | Change |
|---|---|---|
| 0.2 | 2026-09-10 | Token issuance made configurable and defaulted to off. Single-use tokens designed and deferred, with revocation recorded as the same mechanism. Expiry retained, and the alternative of omitting it rejected. Added the deviation covering absent management fields and the one covering an ignored client secret on update. |
| 0.1 | 2026-09-07 | Initial specification. |
