# Component architecture

How the client configuration endpoint relates to the components it reuses. The dynamic client
registration component owns the protocol surface only; client state, credentials and signing keys
stay with their existing owners.

```mermaid
flowchart TB
  subgraph client [Registered client]
    C[Client holding a<br/>registration access token]
  end

  subgraph dcr [Dynamic client registration]
    REG[Registration endpoint<br/>POST]
    CFG[Client configuration endpoint<br/>GET / PUT / DELETE]
    AUTHZ[Authorization:<br/>token subject or system permission]
    SVC[Registration service:<br/>metadata translation,<br/>token issue and verify]
  end

  subgraph reused [Reused components]
    APP[Application lifecycle<br/>read / replace / delete]
    JWT[Token signing service]
    PERM[System permission check]
  end

  C -->|register| REG
  C -->|manage| CFG
  CFG --> AUTHZ
  AUTHZ --> SVC
  REG --> SVC
  SVC -->|resolve, replace, delete| APP
  SVC -->|issue and verify token| JWT
  AUTHZ -->|administrative caller| PERM

  REG -.->|token and configuration URI| C
```

## Ownership

| Component | Owns |
|---|---|
| Dynamic client registration | The protocol surface: RFC 7591 metadata translation, what a client may change, whether a registration access token is issued at all, and token issuance and validation |
| Application lifecycle | Client state. The single owner of the registered record |
| Token signing service | Signing keys, algorithm selection, issuer identity |
| System permission check | Administrative authorization |
