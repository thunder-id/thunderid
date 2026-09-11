# Registration lifecycle

The full lifecycle a dynamically registered client can drive on its own, from registration through
management to deletion, and what happens to the registration access token at each step.

```mermaid
stateDiagram-v2
  [*] --> Registered: POST register<br/>(RFC 7591)

  state Registered {
    [*] --> AdminOnly: issuance disabled<br/>(default)
    [*] --> Active: issuance enabled
    Active --> Active: GET read
    Active --> Active: PUT replace metadata<br/>(identity preserved)
    AdminOnly --> AdminOnly: GET / PUT<br/>(administrative caller)
  }

  Registered --> Deleted: DELETE
  Registered --> Unmanageable: token expires<br/>(only when issuance enabled)

  Deleted --> [*]

  note right of Registered
    Issuance disabled by default:
    no token, administrative access only.
    When enabled the client also holds
    registration_access_token and
    registration_client_uri.
  end note

  note right of Unmanageable
    Client still works for OAuth flows.
    Only self-management is lost.
    Recovery: administrative API.
  end note

  note right of Deleted
    Token still verifies but
    resolves to no client,
    so it authorizes nothing.
  end note
```

## Token state at each stage

| Stage | Token verifies | Token authorizes | Client usable for OAuth |
|---|---|---|---|
| Registered, issuance disabled | No token exists | Administrative caller only | Yes |
| Registered, within validity | Yes | Yes, its own registration only | Yes |
| Registered, past validity | No, expired | No | Yes. Only self-management is lost |
| Deleted | Yes, cryptographically | No, resolves to no client | No |

The first row is the default. Disabling issuance after tokens have been handed out does not move a
client into it: the setting governs issuance alone, so an existing token keeps working until it
expires or its client is deleted.

The third row is the important one: deletion is not enforced by invalidating the token, but by the
client no longer existing. Every operation resolves the client before authorizing, so a token for a
deleted client has nothing to act on.
