# ThunderID Architecture Reference

Open-source IAM stack (`github.com/thunder-id/thunderid`). One Go module builds two binaries: the server (REST API + two React SPAs, `/gate` and `/console`) and the Control Plane (`cmd/cpserver`: management APIs and gateway deployment, no runtime).

## Structure

```text
backend/cmd/server/
  main.go               # startup
  servicemanager.go     # calls every internal/*/init.go to register routes
backend/cmd/cpserver/
  servicemanager.go     # the Control Plane's own wiring: management services only; kept in step with cmd/server by hand
  surface_test.go       # fails if the Control Plane binary links a runtime package
  bootstrap/flows/      # JSON auth/registration flow definitions (auto-seeded)
  repository/           # configdb.db · runtime_transient.db · runtime_persistent.db · entitydb.db created at runtime in the configured data directory (SQLite or Postgres)
backend/internal/
  authn/                # credential / OTP / passkey / social login
  oauth/                # OAuth 2.0 + OIDC server (authorize, token, introspect, userinfo, JWKS, DCR)
  flow/flowexec/        # flow execution engine  →  POST /flow/execute
  flow/executor/        # one file per executor; all names in constants.go; registered in init.go
  flow/core/            # ExecutorInterface, node/graph model
  flow/mgt/             # flow CRUD API
  consent/ application/ user/ group/ role/ ou/ idp/   # management domains
  gateway/              # gateways a Control Plane deploys to; configuration versions, apply lease, exclusions
  variablestore/        # variables and secrets that var:/sec: references resolve to
  system/               # config · database · cache · jose/jwt · security · mcp · log · i18n · deployment (tenant scope) · export · importer
frontend/apps/
  gate/         # login/registration SPA  (@thunderid/react — app-native mode)
  console/      # admin SPA               (@thunderid/react — redirect mode)
  console-cp/   # the console the Control Plane serves: console + @thunderid/configure-gateways, no flow tryouts
frontend/packages/      # @thunderid/contexts · design · hooks · i18n · utils · types · logger
samples/apps/           # react-sdk-sample · react-api-based-sample · vanilla-sample · wayfinder-sample
```

## Backend patterns

- Each domain package: `handler → service → store`, single `Initialize(mux, …)` in `init.go`.
- Public paths (no JWT): `/auth/**`, `/flow/execute/**`, `/oauth2/**`, `/.well-known/openid-configuration/**`, `/.well-known/oauth-authorization-server/**`, `/.well-known/oauth-protected-resource`, `/gate/**`, `/console/**`, `/mcp/**`. Full list in `system/security/permissions.go`.
- Errors: `serviceerror.ServiceError` internally; `sysutils.WriteErrorResponse(w, status, errConst)` for HTTP.

## Control Plane and gateways

A Control Plane holds configuration and deploys it; a server beside it is a **gateway** that runs what is deployed. A version is a capture of a deployment's whole configuration, applied to a gateway through the gateway's import API (upserts, then deletions, the full configuration every time). Values are referenced (`var:NAME`, `sec:NAME`) and resolved from each gateway's variable store at import. Every row is scoped by `DEPLOYMENT_ID`, read from the context with `deployment.Resolve(ctx)`. Rules and pitfalls: [.agent/skills/control-plane/SKILL.md](.agent/skills/control-plane/SKILL.md).

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
