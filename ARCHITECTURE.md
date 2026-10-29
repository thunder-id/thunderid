# ThunderID Architecture Reference

Open-source IAM stack (`github.com/thunder-id/thunderid`). Two builds of one codebase, each a single Go binary serving a REST API and React SPAs:

- **Data plane** (`backend/cmd/server`): where users authenticate. Issues tokens, runs the login flows, and serves `/gate` (login), `/console` and, for a data plane a control plane manages, the Admin Console. It also takes configuration pushed by a control plane through its management API (`/import`, `/variables`, `/secrets`).
- **Control plane** (`backend/cmd/cpserver`): where configuration is authored. Serves the management APIs and `/console` (`console-cp`), and serves no authentication traffic. It captures versions of its configuration and applies them to the gateways (data planes) it manages. Its console signs in through a data plane acting as a trusted issuer.

A control plane can manage many data planes. See [docs/static/assets/images/readme/architecture.svg](docs/static/assets/images/readme/architecture.svg), the editable source of the diagram in the README.

## Structure

```text
backend/cmd/server/     # data plane
  main.go               # startup
  servicemanager.go     # calls every internal/*/init.go to register routes
  bootstrap/flows/      # JSON auth/registration flow definitions (auto-seeded)
  repository/           # configdb.db · runtime_transient.db · runtime_persistent.db · entitydb.db created at runtime in the configured data directory (SQLite or Postgres)
backend/cmd/cpserver/   # control plane: management APIs, configuration versions, gateways
backend/internal/
  authn/                # credential / OTP / passkey / social login
  oauth/                # OAuth 2.0 + OIDC server (authorize, token, introspect, userinfo, JWKS, DCR)
  flow/flowexec/        # flow execution engine  →  POST /flow/execute
  flow/executor/        # one file per executor; all names in constants.go; registered in init.go
  flow/core/            # ExecutorInterface, node/graph model
  flow/mgt/             # flow CRUD API
  consent/ application/ user/ group/ role/ ou/ idp/   # management domains
  system/               # config · database · cache · jose/jwt · security · mcp · log · i18n
frontend/apps/
  gate/         # login/registration SPA  (@thunderid/react — app-native mode)
  console/      # admin SPA               (@thunderid/react — redirect mode)
  console-cp/   # control plane admin SPA (runs every operation as a direct API call)
  admin-console/# data plane admin SPA    (users, agents, groups, roles)
frontend/packages/      # @thunderid/contexts · design · hooks · i18n · utils · types · logger
samples/apps/           # react-sdk-sample · react-api-based-sample · vanilla-sample · wayfinder-sample
```

## Backend patterns

- Each domain package: `handler → service → store`, single `Initialize(mux, …)` in `init.go`.
- Public paths (no JWT): `/auth/**`, `/flow/execute/**`, `/oauth2/**`, `/.well-known/openid-configuration/**`, `/.well-known/oauth-authorization-server/**`, `/.well-known/oauth-protected-resource`, `/gate/**`, `/console/**`, `/mcp/**`. Full list in `system/security/permissions.go`.
- Errors: `serviceerror.ServiceError` internally; `sysutils.WriteErrorResponse(w, status, errConst)` for HTTP.

## Flow engine

Authentication/registration are JSON node graphs (`START → PROMPT → TASK → DECISION → COMPLETE`). The engine steps through nodes, persisting state in `runtime_transient` across requests. Each `TASK` node names an executor (e.g. `"CredentialsAuthExecutor"`). To add one: implement `core.ExecutorInterface`, add name to `executor/constants.go`, register in `executor/init.go`.

## ThunderID React SDK

| Mode | `ThunderIDProvider` props | Used in |
|------|--------------------------|---------|
| Redirect (ThunderID-hosted login) | `clientId` + `baseUrl` | `Console`, `react-sdk-sample` |
| App-native (Flow API) | `applicationId` + `baseUrl` | `Gate`, `react-api-based-sample` |

`clientId` vs `applicationId` is the critical distinction. Common primitives: `useThunderID()`, `<SignedIn/Out>`, `<SignInButton/SignOutButton>`, `<ProtectedRoute>` (`@thunderid/react-router`).

## Auth Flow 

### (Mode 1)

```text
Client → GET /oauth2/authorize → 302 /gate?executionId=…
Gate SPA → POST /flow/execute (loop) → 302 redirect_uri?code=…
Client → POST /oauth2/token → { access_token, id_token }
```

### Mode 2

Client posts directly to `POST /flow/execute` with `applicationId` (first call) then `executionId` until `status: COMPLETE`.
