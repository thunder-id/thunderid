# Dynamic Client Registration Management Specification

- **Status:** Final
- **Version:** 1.0
- **Related documents:**
  - Feature issue: [#5150](https://github.com/thunder-id/thunderid/issues/5150)
  - [RFC 7592](https://www.rfc-editor.org/rfc/rfc7592) OAuth 2.0 Dynamic Client Registration Management Protocol
  - [RFC 7591](https://www.rfc-editor.org/rfc/rfc7591) OAuth 2.0 Dynamic Client Registration Protocol
  - [RFC 6750](https://www.rfc-editor.org/rfc/rfc6750) OAuth 2.0 Bearer Token Usage
  - [threat-model.md](threat-model.md)

## Summary

ThunderID supports Dynamic Client Registration (RFC 7591): a client creates its OAuth registration with a single HTTP request to the registration endpoint. Once created, that registration is frozen from the protocol's point of view. There is no standards-based way to read back what was registered, change a redirect URI, or remove the client. The only available route is a ThunderID-specific administrative API, with a request shape and an error vocabulary unrelated to the one the client was registered through.

This specification adds the RFC 7592 Client Configuration Endpoint, which completes the lifecycle that dynamic registration starts. A caller addresses a registration at its own URI and reads, replaces or deletes it, using the same metadata vocabulary registration already uses.

This is phase 1, and it is deliberately administrative. Every operation on the configuration endpoint is authorized by the system permission, the same credential that already governs the registration endpoint. No per-client management credential is issued, and a registered client cannot manage itself. The reasoning is in [Authorization](#authorization) below.

In scope: the three configuration operations, their authorization model, and the registration response fields.

Out of scope: returning the client secret on read, and expressing ThunderID-specific application properties such as permitted user types through dynamic registration.

## Architecture

The feature extends the existing dynamic client registration component rather than introducing a new one. The registration endpoint and the client configuration endpoint share the same service, the same client metadata model, and the same error vocabulary, so a registration response and a client information response are the same shape.

Three seams already existed and are reused unchanged:

- **Application lifecycle.** Reading, replacing and deleting a registered client are expressed as operations on the underlying application record. The configuration endpoint owns no storage of its own; the application component remains the single owner of client state.
- **Request authorization.** The administrative permission check already used by the registration endpoint is the only credential the configuration endpoint accepts.
- **Credential verification.** Checking a submitted client secret reuses the same verification path the token endpoint uses to authenticate a confidential client. The configuration endpoint verifies through it and never writes a credential.

Ownership boundary: the dynamic client registration component owns the protocol surface, which means translating RFC 7591 metadata to and from the application model and deciding what a caller may change. It does not own client state or credential storage, and it verifies a submitted secret through the existing credential path rather than reading or writing one itself.

See [Component architecture](assets/architecture.md) for the component diagram and ownership boundaries.

## Detailed design

### Authorization

Management of a registration is administrative. A caller holding the system permission may read, replace or delete any dynamically registered client.

The driving consumer is what settles this. An API or application portal registers many clients dynamically and governs all of them on behalf of their owners. It already holds one administrative credential, and one credential is the right shape for what it does: a per-client credential would oblige it to store and rotate N secrets to perform the operations a single secret already authorizes, and would make its own store of those secrets the thing worth attacking. Phase 1 therefore gives the portal the endpoint it needs without giving it a credential inventory to manage.

Every operation follows the same sequence: authorize the caller, then resolve the client named in the request path, then act. Authorizing first means an unauthorized caller cannot learn which client identifiers exist, since every unauthorized request fails the same way whether or not the client is real.

Failure behavior: a caller without the system permission is unauthorized. The response is `401` and carries an RFC 6750 `WWW-Authenticate` challenge naming an invalid token, so a caller that presented a credential learns the credential was the problem. There is no `403` path. A caller either holds the permission that governs every registration or holds nothing, so there is no state in which a caller is authenticated for one registration and refused another.

The permission is checked per request and nothing about it is bound to a particular client, which is also why deleting a client does not need to invalidate any credential. A deleted client no longer resolves, so every subsequent operation on it reports the client as not found.

### Read

Returns the client's currently registered metadata.

The response is assembled from two sources: the OAuth client record supplies the protocol fields (redirect URIs, grant and response types, authentication method, scopes, keys), and the application record supplies the human-readable metadata (name, home page, logo, terms, policy, contacts). Both are needed because neither holds the complete picture.

The client secret is absent. See the deviations below.

### Update

An update **replaces** the registered metadata rather than merging it, because the underlying application record is replaced wholesale. Any field the caller omits is cleared, not retained. The caller is therefore expected to send the complete set of metadata it wants to end up with, which is consistent with RFC 7592 treating the update as a full replacement of client metadata. A caller that sends a partial body will find unrelated fields cleared, so the safe pattern is to read, modify the result, and send it back.

Three properties are exempt from replacement and are carried forward from the existing registration, because losing them would break the registration rather than change it:

| Property | Why it is carried forward |
|---|---|
| Client identifier | The registration must keep its identity. Allowing it to lapse would issue a new identifier, orphan the client's own configuration URI, and drop any registered key material. |
| Client secret | A metadata change must not silently invalidate the client's credentials. The update supplies no new secret to storage, which is what preserves the stored one. A `client_secret` in the request body is a value to be checked, never a value to be written; see below. |
| Owning organization unit and application type | Neither is expressible through RFC 7591 metadata, and the application type is immutable, so both are inherited rather than reasserted. |

The client name is updated like any other field when the request supplies one. It is inherited only when the request omits it entirely, because a name is required on the underlying record and an omitted name would otherwise fail validation rather than clear the field. A request is treated as omitting the name only when it carries neither `client_name` nor a localized variant of it, so supplying only a localized name still names the client. The update response reports the name that was actually applied, which is the inherited one in that case.

A field's translations are replaced along with the field, so a translation the request omits is removed rather than kept. A field the request does not mention keeps its translations, so changing one field does not discard another's. Sending only translations changes only those: a field's own value is replaced when the request supplies it, not when it supplies a translation of it. A read returns a field with its translations, so a caller can see what an update would replace and can send back what it read.

RFC 7592 section 2.2 requires an update request to carry its `client_id`, and its value must identify the client being updated. A request that omits it, or that names a different client, is rejected as invalid client metadata rather than ignored, because either signals that the caller believes it is updating something other than what the URI names.

The same section governs `client_secret`: the field is optional, but if the request carries it, its value must match the client's currently issued secret. ThunderID enforces this. The secret is stored hashed, so the submitted value is verified the same way the token endpoint verifies a client authenticating itself: the presented value is hashed and compared against what is stored. A mismatch is rejected as invalid client metadata and the registration is left unchanged. A request that omits the field is unaffected, and in either case the stored secret is preserved: the submitted value is only ever compared, never written, which is what satisfies the section's prohibition on a client choosing its own secret.

Metadata that fails validation is rejected with the same errors the registration endpoint already uses, so a caller sees consistent behavior whether it is registering or updating.

### Delete

Removes the registration. The client is resolved and authorized first, so deleting an unknown client reports not found rather than succeeding silently. On success the response carries no body.

### Deferred: client self-management

RFC 7592 authorizes the configuration endpoint with a per-client registration access token issued at registration. Phase 1 does not implement that, and this section records why, so that the question is settled rather than re-derived.

**Why it is deferred, not abandoned.** The registration access token is RFC 7592's own authorization model, so a deployment that wants registrants to manage themselves without an operator in the loop will eventually need it. A later phase may reintroduce it alongside the administrative path rather than in place of it, since the two serve different consumers: a portal governing many clients, and a client governing itself. Reintroducing it brings back the design questions phase 1 does not have to answer, including token lifetime against an RFC that says the token SHOULD NOT expire, individual revocation, and whether a token is consumed on use.

Anything that reopens this decision should reopen it on the strength of a consumer that needs self-management, not on a conformance requirement that does not exist.

### Protocol deviations

Points where the implementation knowingly departs from RFC 7592. Each is a consequence of a design decision recorded above, not an oversight.

**Registration does not return the management fields.** RFC 7592 section 3 marks `registration_access_token` and `registration_client_uri` as required members of a client information response. Phase 1 issues no per-client token, so neither field is present in a registration response or in a client information response, and a registrant has no self-service management route. The configuration endpoint is nonetheless reachable at the URI that `registration_client_uri` would have named, to an administrative caller. Closing this deviation is the deferred scope described above.

**The client secret is not returned on read or update.** RFC 7592's example client information response includes `client_secret`. ThunderID stores the client secret hashed, alongside the algorithm and parameters used to derive it. The platform can verify a secret a caller presents, by hashing the presented value and comparing, but it cannot recover the original from what it stores. The secret is therefore omitted rather than fabricated. A client that loses its secret has no recovery route through this endpoint and must be registered again, since the only alternative would be to store secrets reversibly across the whole platform to serve one endpoint.

### API

This feature adds a per-client configuration endpoint. The registration endpoint and its response are unchanged, so clients already using dynamic registration are unaffected.

#### Client configuration endpoint

Addressed per client, under the existing registration path:

```
/oauth2/dcr/register/{client_id}
```

The issue text illustrates `/oauth2/register/{client_id}`. The established ThunderID registration endpoint is `/oauth2/dcr/register`, and relocating it would be a breaking change outside the scope of this feature, so the configuration endpoint extends the existing path instead.

| Operation | Method | Success | Body |
|---|---|---|---|
| Read registration | `GET` | `200` | Current client metadata |
| Update registration | `PUT` | `200` | Updated client metadata |
| Delete registration | `DELETE` | `204` | None |

Authorization for all three: an administrative caller holding the system permission.

Error responses:

| Status | Condition | Notes |
|---|---|---|
| `400` | Invalid client metadata, a `client_id` that does not match the request path, or a `client_secret` that does not match the issued secret | Same error vocabulary as registration |
| `401` | Caller does not hold the system permission | Carries a `WWW-Authenticate` challenge naming an invalid token |
| `404` | No such registration, including one already deleted | |
| `500` | Server failure | |

There is no `403`. A caller either holds the permission that governs every registration or is unauthorized, so there is no condition that a forbidden status would describe.

### Configuration

This feature adds no configuration keys. The existing dynamic client registration settings, `oauth.dcr.enabled` and `oauth.dcr.insecure`, are unchanged and are not consulted by the configuration endpoint, which is registered and authorized the same way regardless of them.

## Requirements

### R1. A registration can be read through the protocol it was created with

**Requirement:** An administrative caller can retrieve a dynamically registered client's currently registered metadata from the client configuration endpoint, in the same shape the registration response uses.

**Acceptance criteria:**

- **AC1.1:** Given a registered client, when an administrative caller reads its registration, then the response returns `200` with the metadata as currently registered.
- **AC1.2:** Given a registered client, when its registration is read, then the response contains no `client_secret`.
- **AC1.3:** Given a client with localized variants of a field, when its registration is read, then the response reports those variants and the base field carries a resolved value rather than an i18n reference, including when the client was registered with no base value for that field.

### R2. A registration can be updated without losing its identity

**Requirement:** An administrative caller can replace a client's supported registered metadata, leaving the client's identifier and credentials intact. A `client_secret` carried in the request is verified against the issued secret rather than written.

**Acceptance criteria:**

- **AC2.1:** Given a registered client, when an administrative caller updates its metadata, then the response returns `200` and the changed metadata is reflected in a subsequent read.
- **AC2.2:** Given a registered client, when its metadata is updated, then the `client_id` in the response and in a subsequent read is unchanged.
- **AC2.3:** Given a confidential client, when its metadata is updated, then the client secret issued at registration continues to authenticate the client afterwards.
- **AC2.4:** Given a registered client, when an update body omits `client_id` or names a different one than the request path, then the request is rejected with `400`.
- **AC2.5:** Given a registered client, when an update body carries a `client_secret` that does not match the currently issued secret, then the request is rejected with `400` and the registration is left unchanged.
- **AC2.6:** Given a registered client, when an update body carries a `client_secret` that matches the currently issued secret, then the update is applied and the stored secret is unchanged.
- **AC2.7:** Given a registered client, when an update contains invalid client metadata, then the request is rejected with `400` and the registration is left unchanged.
- **AC2.8:** Given a registered client, when an update omits `client_name`, then the registered name is retained rather than cleared.
- **AC2.9:** Given a registered client, when an update omits an optional field that was previously set, then that field is cleared, because the update is a full replacement.
- **AC2.10:** Given a client with localized variants of a field, when an update supplies that field but omits one of its variants, then the omitted variant is removed rather than retained.
- **AC2.11:** Given a client with localized variants of a field, when an update does not mention that field at all, then its existing variants are retained.
- **AC2.12:** Given a client with a base value and localized variants of a field, when an update supplies only variants of that field, then the base value is retained.

### R3. A registration can be deleted

**Requirement:** An administrative caller can remove a registration, and the registration is not reachable afterwards.

**Acceptance criteria:**

- **AC3.1:** Given a registered client, when an administrative caller deletes its registration, then the response returns `204` with no body.
- **AC3.2:** Given a deleted registration, when any operation is attempted against it, then the request is rejected with `404`.
- **AC3.3:** Given a client identifier that was never registered, when any operation is attempted against it, then the request is rejected with `404` rather than succeeding silently.

### R4. One administrative credential governs every registration

**Requirement:** The client configuration endpoint is authorized by the system permission alone, and that permission authorizes management of any dynamically registered client.

**Acceptance criteria:**

- **AC4.1:** Given two registered clients, when an administrative caller reads, updates or deletes either of them, then the request is authorized, without a per-client credential.
- **AC4.2:** Given a request carrying no credential, when it reaches the client configuration endpoint, then it is rejected with `401` and the registration is neither disclosed nor modified.
- **AC4.3:** Given a caller authenticated without the system permission, when it reaches the client configuration endpoint, then it is rejected with `401`, not `403`.
- **AC4.4:** Given a rejected request at the client configuration endpoint, when the response is inspected, then it carries a `WWW-Authenticate` challenge consistent with Bearer token usage.
- **AC4.5:** Given a successful registration, when the response is inspected, then it contains neither a `registration_access_token` nor a `registration_client_uri`.

## Change log

| Version | Date | Change |
|---|---|---|
| 1.0 | 2026-09-21 | Initial specification. |
