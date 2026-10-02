# ThunderID Operator

A Kubernetes operator that manages [ThunderID](https://github.com/thunder-id/thunderid) identity provider deployments declaratively. It lets you spin up ThunderID instances, define auth flows and themes, and register OAuth2 applications — all as Kubernetes custom resources.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Architecture](#architecture)
- [Namespaces](#namespaces)
- [Quick Start](#quick-start)
  - [1. Install CRDs](#1-install-crds)
  - [2. Start ThunderID](#2-start-thunderid)
  - [3. Run the operator](#3-run-the-operator)
  - [4. Register an application](#4-register-an-application)
- [GitOps (Flux)](#gitops-flux)
- [Helm Chart](#helm-chart)
- [Custom Resources](#custom-resources)
  - [ThunderIDInstance](#thunderidinstance)
  - [ThunderIDResource](#thunderidresource)
- [Checking Status](#checking-status)
- [Troubleshooting](#troubleshooting)
- [Development](#development)

---

## Prerequisites

| Tool | Notes |
|------|-------|
| A running Kubernetes 1.28+ cluster | Any distribution works (local or remote), this guide doesn't set one up for you |
| kubectl | Configured to talk to that cluster |
| Go 1.26+ | Needed to run the operator locally (`go run ./cmd/main.go`) |
| make | Optional. Only needed for the [Development](#development) targets (`make manifests`, `make generate`, `make test`) or an in-cluster deploy via `make deploy`. Every Quick Start step below works with plain `kubectl`/`go run` instead |

`kubectl` and `go run` talk to the cluster over the network via your kubeconfig, so they run fine on
whatever OS you're on, including native Windows (PowerShell or Git Bash), even when the cluster
itself is hosted elsewhere (e.g. inside WSL2 on the same machine). Only the cluster's own backing
runtime (Docker, a VM, WSL2, etc.) is typically Linux-based; the client tooling here is not.

Sanity check before continuing:

```bash
kubectl cluster-info
kubectl get nodes
go version
```

Expected: `cluster-info` prints a reachable control-plane URL, `get nodes` lists at least one node
`Ready`, and `go version` reports 1.26 or newer. If any of these fail, fix that before moving on,
since every step below assumes a working cluster connection.

---

## Architecture

```
ThunderIDInstance  ──creates──►  Deployment / Service
      ├──creates──► Ingress (only if spec.domain is set)
      └──creates──► HPA

ThunderIDResource CRD (spec.resource_type: application|flow|theme|organization_unit|
              role|user_type|user|resource_server|group|server_config|
              translation|...)
                       ──writes──►  <instance>-<resource_type-plural> ConfigMap
                                    └── all mounted into ThunderID pod at startup
```

**Key facts:**
- Every `ThunderIDInstance` must pick a persistent backend: either `spec.dataVolumeSize` (PVC-backed SQLite — `repository/database` persists, `setup.sh` runs once on the pod's first-ever start and is skipped on every restart after that) or every `spec.config.database` scope set to `type: postgres` with its own `passwordRef` (external PostgreSQL — see [Database](#database) below; the base seed from `spec.resourcesConfigMap` is imported into the database once via a hash-gated bootstrap Job, not on every restart, and `spec.dataVolumeSize` is ignored since there's no local state to persist). Setting neither is rejected — ephemeral, wipe-on-every-restart SQLite is not a supported configuration.
- The ThunderID image bakes in its own default organization unit, user types, and flows (`default` OU, `Person` user type, `console-app-flow`, `cloud-login-flow`, etc.) — `spec.resourcesConfigMap` is **optional**, only needed if you want to seed additional base resources beyond those defaults.
- Every `resource_type` is loaded at **pod startup** via mounted ConfigMaps, in dependency order — organization units first (everything else can reference `ouId`), resource servers and user types early (roles/users reference them), users before groups (groups reference users by id), groups and resource servers before roles (roles reference both by id), applications last (they can reference themes, flows, and organization units). A config change triggers a rolling restart; it is not applied live. Each `ThunderIDResource` reports `Pending` → `Ready`/`Error` on its `status.phase` while the rollout is verified.
- `spec.domain` is **optional** and unset by default — every local/dev workflow in this repo reaches ThunderID via `kubectl port-forward svc/<name> 8090:8090` → `https://localhost:8090` instead, which is what the operator assumes when no domain is set. Setting `spec.domain` creates a real `Ingress` for external access (a real deployment with real DNS), but the `public_url` value it feeds into ThunderID's own config isn't actually a field ThunderID's schema recognizes (see the image's own `config/default.json`, which has no `public_url` anywhere). ThunderID silently ignores it either way, so this only matters for the `Ingress` routing itself, not anything ThunderID does internally.

---

## Namespaces

The operator's RBAC is a `ClusterRole`, and `cmd/main.go` sets no `--namespace`/cache
restriction — it watches **every namespace** on the cluster by default, not just the one it's
deployed into.

- **A namespace has to exist before you can create anything in it.** `metadata.namespace: foo` in
  a CR only *references* `foo` — it doesn't create it. If `foo` doesn't already exist, `kubectl
  apply` fails with `namespaces "foo" not found`. Create it first:
  ```yaml
  apiVersion: v1
  kind: Namespace
  metadata:
    name: foo
  ```
  (`default`, `kube-system`, `kube-public`, `kube-node-lease` already exist in every cluster, no
  setup needed.)
- **Run multiple isolated instances side by side** by putting each in its own namespace — separate
  teams/environments, or a throwaway namespace for testing without touching a real instance.
- **A `ThunderIDInstance` and everything that belongs to it must share one namespace** — the
  `ThunderIDResource`s labeled with its name, the `EnvironmentValues` (if any) labeled with its name, and every
  Secret named directly in its spec: every `spec.config.database.<scope>.postgres.passwordRef.name`,
  `spec.config.email.smtp.secretRef` (hand-written, same rule). The operator resolves a `ThunderIDResource`'s or
  `EnvironmentValues`'s owning instance by looking it up **in the `ThunderIDResource`'s/`EnvironmentValues`'s own
  namespace** — there's no cross-namespace reference. Splitting an instance's objects across
  namespaces just means the `ThunderIDResource`s/`EnvironmentValues` never find their instance; splitting a
  Secret from its instance means the pod can't find *it* either —
  `envFrom`/Secret volume mounts are namespace-scoped in Kubernetes itself, not an operator
  limitation. A Secret in the wrong namespace fails the pod with `CreateContainerConfigError`, not
  a `kubectl apply`-time error, so it's easy to miss — see
  [Pod never reaches Ready](#pod-never-reaches-ready-after-applying-thunderidinstance).

---

## Quick Start

### 1. Install CRDs

```bash
cd tools/k8s-operator
kubectl apply -f config/crd/bases/
```
(`make install` works too if you have `make` — it does the same thing via kustomize.)

Verify:

```bash
kubectl get crd | grep thunderid
```

Expected output:
```
environmentvalues.apps.thunderid.io
thunderidinstances.apps.thunderid.io
thunderidresources.apps.thunderid.io
```

---

### 2. Start ThunderID

Apply a `ThunderIDInstance` to create and manage a ThunderID deployment:

```yaml
apiVersion: apps.thunderid.io/v1alpha1
kind: ThunderIDInstance
metadata:
  name: thunderid-prod
spec:
  image: ghcr.io/thunder-id/thunderid:latest
  replicas: 1
  dataVolumeSize: "1Gi"
```

```bash
kubectl apply -f my-instance.yaml
```

No base resources ConfigMap needed — the image bakes in its own default
organization unit, user types, and flows (`default` OU, `Person` user type,
`console-app-flow`, `cloud-login-flow`, etc.). If you want to seed
*additional* base resources beyond those defaults, point `spec.resourcesConfigMap`
at a ConfigMap of your own; it's optional, not required.

The `ThunderIDInstance` object now exists, but nothing creates its Deployment/Service/pod
yet — that only happens once the operator (step 3 below) is running and reconciles it.

---

### 3. Run the operator

**Local (development):**

```bash
cd tools/k8s-operator
go run ./cmd/main.go   # or: make run
```

This runs the operator binary against whatever cluster `kubectl` is currently pointing to. Keep this terminal open — it must stay running to reconcile resources.

**In-cluster (production):**

```bash
cd tools/k8s-operator
make deploy IMG=<your-registry>/thunderid-operator:latest
```
or install the [Helm chart](charts/thunderid-operator) instead — see its own README for a `helm install` walkthrough.

Once the operator is running, it picks up the `ThunderIDInstance` applied in step 2. Wait for the pod to be ready:

```bash
kubectl get pod -w
```

You should see a pod reach `1/1 Running`.

---

### 4. Register an application

Create a `ThunderIDResource` with `spec.resource_type: application`. The `thunderid.io/instance` label must match the name of your `ThunderIDInstance`. Everything else under `spec` is forwarded into ThunderID's own application schema essentially as-is (see the [design note](#custom-resources) below) — this example uses literal IDs for the org unit and auth flow, both of which exist in the image's own baked-in defaults (get them with the `kubectl exec ... 01-default-resources.yaml` trick in [Troubleshooting](#troubleshooting) if you're using a different image):

```yaml
apiVersion: apps.thunderid.io/v1alpha1
kind: ThunderIDResource
metadata:
  name: my-console-app
  namespace: default
  labels:
    thunderid.io/instance: thunderid-prod
spec:
  resource_type: application
  name: My Console App
  ouId: "<id of the 'default' OrganizationUnit>"
  authFlowId: "<id of the 'console-app-flow' Flow>"
  type: browser
  isRegistrationFlowEnabled: true
  isRecoveryFlowEnabled: false
  allowedUserTypes:
    - Person
  assertion:
    validityPeriod: 3600
  loginConsent:
    validityPeriod: 0
  inboundAuthConfig:
    - type: oauth2
      config:
        clientId: MY_APP_CLIENT
        redirectUris:
          - https://localhost:8090/callback
        grantTypes:
          - authorization_code
          - refresh_token
        responseTypes:
          - code
        tokenEndpointAuthMethod: none
        pkceRequired: true
        publicClient: true
```

```bash
kubectl apply -f my-app.yaml
```

Check status:

```bash
kubectl get thunderidresources
kubectl describe thunderidresource my-console-app
```

---

## GitOps (Flux)

`ThunderIDInstance` and `ThunderIDResource` CRs work fine under a continuously-reconciling GitOps tool like [Flux](https://fluxcd.io) — commit a CR, `git push`, and it's live with no `kubectl apply` needed. This is entirely optional; steps 1–4 above work standalone with plain `kubectl apply`.

**`EnvironmentValues` is the one exception — never put it under GitOps management.** Its controller mutates the object in place after processing it (masking each synced key's value in `spec.env`). A continuously-reconciling GitOps tool enforces "live must match Git" and will keep reverting that mutation every reconcile interval, fighting the controller forever. `EnvironmentValues` is an ad-hoc, one-time `kubectl apply` resource by design — see [Env vars & `{{.VAR}}` placeholders](#env-vars--var-placeholders) above. It also carries plaintext credentials in its spec, which shouldn't be committed to Git regardless.

This requirement extends to any hand-written `Secret`, too: **always apply `EnvironmentValues` (and any hand-written `Secret`) with `kubectl apply --server-side`**, never plain `kubectl apply` (see above for why). An object already carrying the old `last-applied-configuration` annotation from a prior plain apply needs a one-time manual strip:

```bash
kubectl annotate <kind> <name> kubectl.kubernetes.io/last-applied-configuration- -n <namespace>
```

Caveat: when changing one key in a multi-key `EnvironmentValues`, apply a manifest containing *only* that key — resubmitting the full original file makes every already-masked sibling key conflict at once, and `--force-conflicts` would overwrite all of them simultaneously.

---

## Helm Chart

An alternative to steps 1 and 3 above for a packaged, in-cluster operator deployment — see [charts/thunderid-operator](charts/thunderid-operator) for `helm install` instructions, values, and what it does and doesn't cover.

---

## Custom Resources

### ThunderIDInstance

The top-level resource. Creates and manages a `Deployment`, `Service`, and (optionally) an `Ingress` and/or an `HPA`.

| Field | Required | Description |
|-------|----------|-------------|
| `spec.image` | Yes | ThunderID container image |
| `spec.domain` | No | Externally-reachable hostname. When set, creates an `Ingress` (`Host: spec.domain`, TLS via `spec.tlsSecret` if set) so ThunderID is reachable without `kubectl port-forward` — e.g. a real deployment behind real DNS. When unset (the default — every local/dev workflow in this repo uses this), no `Ingress` is created; reach ThunderID via `kubectl port-forward svc/<name> 8090:8090` → `https://localhost:8090` instead |
| `spec.resourcesConfigMap` | No | ConfigMap containing *additional* base resources YAML, seeded alongside the image's own built-in defaults (default OU, user types, flows). Not required for normal use |
| *(none — see `EnvironmentValues`)* | — | Environment variables come from whichever `EnvironmentValues` in this namespace is labeled `thunderid.io/instance: <this instance's name>`, not a spec field — see [Env vars & `{{.VAR}}` placeholders](#env-vars--var-placeholders) below |
| `spec.config.database` | See note | Mirrors ThunderID's own `database:` section 1:1 — four independent scopes (`config`, `runtimeTransient`, `entity`, `runtimePersistent`), each with `type: sqlite` (default) or `type: postgres` and a matching `sqlite:`/`postgres:` block. Unset means the image's built-in sqlite defaults for all four. See [Database](#database) below. Unless every scope is `type: postgres`, `spec.dataVolumeSize` **must** be set — a `ThunderIDInstance` with neither is rejected |
| `spec.config.database.<scope>.postgres.passwordRef` | No | Required when that scope's `type` is `postgres`: `{name, key}` naming the Secret+key holding that scope's password. Independent per scope — point two scopes at the same Secret with different `key`s for separate passwords, or the same Secret+key to share one, as today's default pattern does |
| `spec.tlsSecret` | No | Kubernetes Secret name for TLS |
| `spec.dataVolumeSize` | See note | e.g. `"1Gi"`. Provisions a ReadWriteOnce PVC for `repository/database` (the SQLite files) and makes `setup.sh` run only once — on every restart after the first, the entrypoint finds a marker file on that volume and skips straight to `start.sh`. Only meaningful with `replicas: 1` (the Deployment switches to a `Recreate` strategy since the PVC can't be mounted by two pods at once). Unnecessary only when every `spec.config.database` scope is `type: postgres` |
| `spec.replicas` | No | Replica count (set to 0 when using HPA, which manages replicas) |
| `spec.resources` | No | CPU/memory requests and limits |
| `spec.autoScaling` | No | HPA settings (`minReplicas`, `maxReplicas`, `targetCPUUtilizationPercentage`, `targetMemoryUtilizationPercentage`) |
| `spec.config.port` | No | Port ThunderID listens on (default `8090`). Drives the container port, all three probes, the Service, and the Ingress backend — set once here. Dual-purpose (Kubernetes wiring too), so it stays flat rather than mirrored under `server:` like the rest of `spec.config` |
| `spec.config.tls.minVersion` | No | Minimum TLS version ThunderID's own HTTPS listener accepts (default `"1.3"`, also accepts `"1.2"`). Unrelated to `spec.tlsSecret`, which is the Ingress's TLS termination |
| `spec.config.jwt.preferredKeyId` | No | Which built-in signing key (`default-key` (RSA) or `ecdsa-key` (ECDSA)) ThunderID signs new tokens with. Default `default-key`; both remain valid for verifying already-issued tokens either way |
| `spec.config.passkey.extraAllowedOrigins` | No | Extra WebAuthn/passkey origins beyond the instance's own public URL — e.g. a separate frontend origin that also needs to register/verify passkeys |
| `spec.config.email` | No | Outbound SMTP config, mirroring ThunderID's own `email:` section — `spec.config.email.smtp.host` / `.port` / `.fromAddress` required, `.username` / `.enableAuthentication` / `.enableStartTLS` optional. Unset means no email section at all — same as the image's own default. `spec.config.email.smtp.secretRef: {name, key}` names the Secret+key holding the SMTP password, required when `smtp.enableAuthentication: true` — can point at the same Secret a database scope's `postgres.passwordRef` already uses, just a different `key` |

#### Public URL / Ingress

By default `spec.domain` is unset and every workflow in this repo reaches
ThunderID via `kubectl port-forward svc/<name> 8090:8090` → `https://localhost:8090`.
Setting `spec.domain` switches to a real `Ingress` instead — useful for a
deployment with actual DNS pointed at the cluster, not local dev.

**Prerequisite:** the `Ingress` the operator creates hardcodes
`nginx.ingress.kubernetes.io/backend-protocol: HTTPS` and
`nginx.ingress.kubernetes.io/ssl-redirect: true` annotations and doesn't set
`spec.ingressClassName`, so it only works with **ingress-nginx installed as
the cluster's default `IngressClass`** — on minikube: `minikube addons
enable ingress`. Other ingress controllers will either ignore those
annotations or not pick up the resource at all.

To enable it:

```yaml
spec:
  image: ghcr.io/thunder-id/thunderid:latest
  domain: id.example.com        # must resolve to your ingress controller's IP
  tlsSecret: id-example-com-tls # optional — see below
```

- **`domain`** becomes the `Ingress`'s `Host` rule and (with no domain-based
  routing of its own) is also what `publicURL` gets set to internally
  (`https://<domain>`) — used only for the `Ingress` routing, since
  ThunderID's own config schema doesn't recognize a `public_url` field at
  all (see the Key Facts note above).
- **`tlsSecret`** (optional) names a pre-existing `Secret` (standard
  `kubernetes.io/tls`, i.e. `tls.crt`/`tls.key`) holding a *real* certificate
  for `domain` — this is what terminates TLS for external clients at the
  `Ingress`. It's separate from ThunderID's own internal self-signed
  cert on the pod itself: the `backend-protocol: HTTPS` annotation just
  tells ingress-nginx to speak HTTPS to the pod on the backend leg
  regardless. Without `tlsSecret`, the `Ingress` has no TLS block and serves
  plain HTTP externally.
- For local testing without real DNS, point `domain` at a hostname you
  control via `/etc/hosts` (or Windows' `C:\Windows\System32\drivers\etc\hosts`)
  mapped to `minikube ip`, rather than a domain that won't resolve — a
  placeholder that resolves nowhere leaves the `Ingress` unreachable.
- **Caveat:** the `Ingress` is only fully built (rules + TLS block) at
  *creation* time — if you set `domain` first and add `tlsSecret` later,
  the existing `Ingress` won't pick up the TLS block on its own. Run
  `kubectl delete ingress <name>` to force the operator to recreate it with
  the current spec on the next reconcile.
- To go back to `kubectl port-forward`-only access, unset `domain` — the
  operator deletes the `Ingress` automatically on the next reconcile.

#### Database

`spec.config.database` mirrors ThunderID's own `database:` section 1:1 — four independent scopes (`config`, `runtimeTransient`, `entity`, `runtimePersistent`), each with its own `type`/`sqlite`/`postgres` block, rather than one connection reused everywhere. By default `spec.config.database` is unset and every scope uses the image's built-in sqlite files — no configuration needed. To use PostgreSQL instead, set every scope's `type: postgres` with its own connection block — ThunderID's own schema has no shortcut for "same connection everywhere," so neither does this, redundancy included — plus a `passwordRef: {name, key}` per scope naming the Secret+key holding that scope's password:

```yaml
apiVersion: apps.thunderid.io/v1alpha1
kind: ThunderIDInstance
metadata:
  name: my-instance
spec:
  image: ghcr.io/thunder-id/thunderid:latest
  config:
    database:
      config:
        type: postgres
        postgres:
          hostname: "postgres.example.com"
          port: "5432"               # optional, defaults to 5432
          username: "thunderid"
          name: "thunderid_config"
          sslmode: "require"         # optional; require (default), verify-full, verify-ca, or disable
          passwordRef:
            name: thunderid-db
            key: password
      entity:
        type: postgres
        postgres:
          hostname: "postgres.example.com"
          port: "5432"
          username: "thunderid"
          name: "thunderid_config"   # same database as config: — see Manual setup below
          sslmode: "require"
          passwordRef:
            name: thunderid-db
            key: password
      runtimeTransient:
        type: postgres
        postgres:
          hostname: "postgres.example.com"
          port: "5432"
          username: "thunderid"
          name: "thunderid_runtime"
          sslmode: "require"
          passwordRef:
            name: thunderid-db
            key: password
      runtimePersistent:
        type: postgres
        postgres:
          hostname: "postgres.example.com"
          port: "5432"
          username: "thunderid"
          name: "thunderid_runtime"  # same database as runtimeTransient: — see Manual setup below
          sslmode: "require"
          passwordRef:
            name: thunderid-db
            key: password
---
apiVersion: v1
kind: Secret
metadata:
  name: thunderid-db
  namespace: default
stringData:
  password: "secret"
```

Each scope's fields are inlined directly into the generated `deployment.yaml` almost verbatim; only `password` stays in the Secret, mounted into the pod as a file and referenced with a `file://` URI, the same way ThunderID resolves `direct_auth_secret` and `crypto.key`. A scope's `sqlite`/`postgres` sub-fields left unset fall back to the same defaults used when `spec.config.database` is omitted entirely.

Every postgres scope needs its own `passwordRef` — there's no shared/top-level fallback. The four scopes above all point at the same Secret+key (`thunderid-db`/`password`), matching a single password reused everywhere, but they don't have to: point two scopes' `passwordRef` at the same Secret with **different** `key` values to give each its own password without a second Secret object, e.g. `config`/`entity` using `key: config-password` and `runtimeTransient`/`runtimePersistent` using `key: runtime-password` on one `thunderid-db` Secret holding both keys.

#### Manual database & schema setup (PostgreSQL)

Unlike SQLite (whose schema is baked into the image's `.db` files at build time), the operator does **not** create databases or run migrations for PostgreSQL — it only tells the pod where to connect. You provision the databases and load the schema yourself, once, before pointing an instance at them. ThunderID's own official Helm chart takes the same approach (its `setup-job.yaml` only imports declarative resources into existing tables — it doesn't create schema either, and its docs point you at the Bitnami PostgreSQL chart for actually standing up postgres).

ThunderID's 4 scopes map to only **2 physical databases** in practice — `config` and `entity` share one (since `entity`'s lookups depend on `ENTITY_TYPES`, which only exists in the config schema), `runtimeTransient` and `runtimePersistent` share the other. Point their `name` fields at the same two database names, as in the example above:

```bash
# 1. Create the two databases (names must match config.name/entity.name and
#    runtimeTransient.name/runtimePersistent.name above)
createdb -U postgres thunderid_config
createdb -U postgres thunderid_runtime

# 2. Load schema into each, from the same scripts ThunderID's own runtime uses
#    (backend/dbscripts/, relative to the repo root — not copied here, so it can't drift
#    out of sync with the schema ThunderID itself ships).
psql -U postgres -d thunderid_config -f ../../backend/dbscripts/configdb/postgres.sql
psql -U postgres -d thunderid_config -f ../../backend/dbscripts/entitydb/postgres.sql
psql -U postgres -d thunderid_runtime -f ../../backend/dbscripts/runtime_persistent/postgres.sql
psql -U postgres -d thunderid_runtime -f ../../backend/dbscripts/runtime_transient/postgres.sql
```

Re-running `psql -f` against an already-initialized database will error on `CREATE TABLE`/`CREATE TYPE` statements for objects that already exist — that's expected on a re-run, not a sign anything's wrong; the schema only needs to be loaded once per database.

---

### ThunderIDResource

The generic ThunderID resource kind. Every ThunderID resource — application, flow, theme, organization unit, role, user type, user, resource server, group, server config, translation, and any future kind ThunderID adds — is the same Kubernetes `kind: ThunderIDResource`, distinguished only by `spec.resource_type`. There's one CRD, one controller; adding a ThunderID resource kind never needs an operator code change.

```yaml
apiVersion: apps.thunderid.io/v1alpha1
kind: ThunderIDResource
metadata:
  name: my-console-app
  namespace: default
  labels:
    thunderid.io/instance: thunderid-prod
spec:
  resource_type: application
  name: My Console App
  ouId: "<id of the 'default' OrganizationUnit>"
  authFlowId: "<id of the 'console-app-flow' Flow>"
  type: browser
  isRegistrationFlowEnabled: true
  allowedUserTypes:
    - Person
  inboundAuthConfig:
    - type: oauth2
      config:
        clientId: MY_APP_CLIENT
        redirectUris:
          - https://localhost:8090/callback
        grantTypes:
          - authorization_code
          - refresh_token
        responseTypes:
          - code
        tokenEndpointAuthMethod: none
        pkceRequired: true
        publicClient: true
```

| Field | Description |
|-------|-------------|
| `metadata.labels['thunderid.io/instance']` | Name of the `ThunderIDInstance` to register with |
| `spec.resource_type` | **Required.** Which ThunderID resource this is — see the table below for the ConfigMap each one lands in |
| everything else under `spec` | ThunderID's own field names for that `resource_type` (`name`, `ouId`, `handle`, `type`, ...) — copy-paste ThunderID's own exported YAML in directly (dropping its `id:` line, which the operator generates itself). Any field can hold a `{{.VAR}}` token — see [Env vars & `{{.VAR}}` placeholders](#env-vars--var-placeholders) below |

**Design note:** Everything under `spec` except `resource_type` is forwarded into ThunderID's own bootstrap schema for that resource type essentially as-is. The operator does **not** resolve handles, validate required fields, or otherwise interpret the content — that's ThunderID's own bootstrap loader's job. This means:
- A mistake (missing required field, a bad ID, malformed JSON) is **not** caught at `kubectl apply` time — it's caught at ThunderID's own bootstrap time, surfaced as a `ThunderIDError`/`ThunderIDCrashed` Event on the owning `ThunderIDInstance`, not on this CR itself (see [Checking Status](#checking-status)).
- Cross-references (`ouId`, `authFlowId`, `themeId`, `resourceServerId`, member/assignment IDs, ...) can be literal ThunderID IDs, the same as if you were editing ThunderID's own `resources.yaml` by hand. Or use `*Handle`-style fields (`ouHandle`, `authFlowHandle`, `registrationFlowHandle`, `recoveryFlowHandle`) — ThunderID's own bootstrap loader resolves them itself. `themeHandle` is the one exception — not supported by ThunderID at all; use `themeId`.
- None of the field names in the table below are enforced by the operator or the CRD schema; get the exact set ThunderID's bootstrap loader accepts from its own docs/schema when in doubt.

#### `resource_type` values

| `resource_type` | ConfigMap | Notes |
|---|---|---|
| `application` | `<instance>-applications` | OAuth2/OIDC application. `type` (not `template`) is required — one of `browser`, `fullstack`, `mobile`, `m2m`, `custom`, or omitting it fails with `APP-1042`. `inboundAuthConfig` is nested YAML/JSON defining the OAuth2 client |
| `flow` | `<instance>-flows` | Auth/registration/recovery/signout flow. `handle`/`name`/`flowType` (`AUTHENTICATION`\|`REGISTRATION`\|`RECOVERY`\|`SIGNOUT`) required. `nodes` is the flow graph — nested YAML matching what ThunderID exports/imports natively, pasted directly (not a string) |
| `theme` | `<instance>-themes` | UI theme. `displayName` required. `theme` is nested YAML/JSON, not a string — a bare `{...}` value is valid YAML flow-mapping syntax (JSON is a YAML subset), so a pasted-in JSON theme export works verbatim without re-indenting |
| `organization_unit` | `<instance>-organizationunits` | Container for users, roles, applications. `handle`/`name` required. `parentId` is an optional literal parent-OU ID |
| `role` | `<instance>-roles` | `name` required. `permissions[].resourceServerId` + `.permissions`, `assignments[].id` + `.type` — literal IDs, not Kubernetes references |
| `user_type` | `<instance>-usertypes` | `category`/`name` required. `schema` (required) is the attribute schema, nested YAML/JSON |
| `user` | `<instance>-users` | `type` (the owning `UserType`'s **name**, e.g. `Person` — a literal string, not a Kubernetes reference) required. `attributes` is a free-form key/value map matching the `UserType`'s `schema`. Passwords/credentials go under `credentials` (e.g. `credentials.password`) — see [Env vars & `{{.VAR}}` placeholders](#env-vars--var-placeholders) to keep the literal value out of this CR entirely |
| `resource_server` | `<instance>-resourceservers` | `name`/`identifier` required — ThunderID rejects one with no identifier at bootstrap, crash-looping the pod. `delimiter` joins a resource/action handle into a permission string (e.g. `ou:view`). `resources[].actions[]` nested |
| `group` | `<instance>-groups` | `name` required. `members[].id` + `.type` — literal IDs, not Kubernetes references |
| `server_config` | `<instance>-serverconfigs` | `name: cors\|session\|defaultResourceServer` + `value: {...}` holding that section's own fields |
| `translation` | `<instance>-translations` | `language` (BCP47, e.g. `en-US`) + `translations` (namespace → key → string) required |

**OAuth2 configuration notes** (`resource_type: application`):
- Public client (browser app, SPA): use `publicClient: true`, `tokenEndpointAuthMethod: none`, `pkceRequired: true`
- Confidential client (server-side): use `publicClient: false`, `tokenEndpointAuthMethod: client_secret_basic`
- `client_credentials` grant type requires a confidential client (`publicClient: false`) with a real auth method — it cannot use `tokenEndpointAuthMethod: none`

#### Env vars & `{{.VAR}}` placeholders

Any field, in any `resource_type` — a user's `credentials.password`, an application's OAuth `clientSecret`, anything — can hold a `{{.VAR}}` token instead of a literal value, so the CR itself never holds the literal secret. ThunderID's own live `resources.yaml` loader resolves these at pod **startup** against the process environment — the same mechanism ThunderID's own bundled bootstrap templates use, just applied to every declarative resource this operator mounts, not only its own bootstrap-time ones.

A `ThunderIDInstance` auto-discovers its env Secret the same way a `ThunderIDResource` finds its owning instance, just in reverse: whichever `EnvironmentValues` in the instance's namespace carries the label `thunderid.io/instance: <instance name>` is the one whose Secret gets mounted whole via `envFrom`. There's no field on `ThunderIDInstanceSpec` naming it — the reference lives on the `EnvironmentValues` side. At most one `EnvironmentValues` may carry a given instance's label; more than one is a config error surfaced as an `InvalidSpec` event rather than picking one arbitrarily. The operator only ever mounts this Secret whole — it never creates, reads individual keys from, or generates values into it. Two ways to author it, both producing exactly the same Secret and both fully supported:

**Apply an `EnvironmentValues`** — a declarative CR: the operator syncs `spec.env` into a Secret of the same name, then overwrites each synced key's value in `spec.env` with a masked marker (`thunderid-masked:<up to 3 leading chars>**********`) so the plaintext — and even its real length — is never visible again via `kubectl get/describe environmentvalues`. The object itself is never deleted — `status.keys` lists which keys are currently synced. To change a value later, re-apply with new plaintext for just that key; `kubectl apply`'s 3-way merge leaves already-masked sibling keys untouched. The plaintext briefly exists in the `EnvironmentValues` object's own `spec` in etcd before it's masked — unlike the Secret it produces, that object gets none of Kubernetes' Secret-specific handling (base64, RBAC scoped to `secrets`, and encryption-at-rest where the cluster has it configured) for that short window. **Always apply `EnvironmentValues` with `kubectl apply --server-side`**, never plain `kubectl apply` — plain apply copies the whole manifest, plaintext included, into the object's `last-applied-configuration` annotation, bypassing the masking entirely (see [GitOps (Flux)](#gitops-flux) below). Never put `EnvironmentValues` under GitOps (Flux) management — a continuously-reconciling GitOps tool will fight the masking:
```yaml
apiVersion: apps.thunderid.io/v1alpha1
kind: EnvironmentValues
metadata:
  name: thunderid-env
  namespace: default
  labels:
    thunderid.io/instance: thunderid-prod
spec:
  env:
    ADMIN2_PASSWORD: change-me
```

**Or hand-write the Secret directly**, plus a thin `EnvironmentValues` stub to carry the label (discovery is purely label-based, so even a hand-written Secret needs an `EnvironmentValues` object pointing at it — its own `spec.env` stays empty and the controller adopts the existing Secret without ever touching its contents). The Secret must also carry `thunderid.io/environment-values: <name of the EnvironmentValues>` — without it the reconcile refuses to touch the Secret at all, since matching by name alone would let anyone who can create an `EnvironmentValues` overwrite keys in any same-named Secret in the namespace, including ones they have no RBAC permission to edit directly:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: thunderid-env
  namespace: default
  labels:
    thunderid.io/environment-values: thunderid-env
type: Opaque
stringData:
  ADMIN2_PASSWORD: change-me
---
apiVersion: apps.thunderid.io/v1alpha1
kind: EnvironmentValues
metadata:
  name: thunderid-env
  namespace: default
  labels:
    thunderid.io/instance: thunderid-prod
```

Either way, reference the key from a `ThunderIDResource`:
```yaml
apiVersion: apps.thunderid.io/v1alpha1
kind: ThunderIDResource
metadata:
  name: admin2
  labels:
    thunderid.io/instance: thunderid-prod
spec:
  resource_type: user
  credentials:
    password: '{{.ADMIN2_PASSWORD}}'
```

A `{{.VAR}}` token with no matching key in the discovered Secret isn't caught at `kubectl apply` time — ThunderID's own boot fails with a clear `environment variable ADMIN2_PASSWORD is not set` error, surfaced the same way any other bootstrap failure is (see [Checking Status](#checking-status)).

ThunderID's declarative loader only reads resources and environment variables at pod **startup** — changing the Secret's content after the fact still requires the ThunderID pod to restart (triggered automatically, the same way editing `spec.config.database`/`spec.config.email` already does) before the new values take effect.

There's no built-in "generate me a password" convenience today — every value is authored by hand into the Secret. If that turns out to matter, it can be layered on top later (the operator writing a generated value into a deterministic key if one isn't already present) without changing anything else about this mechanism.

---

## Checking Status

### Pod health

```bash
kubectl get pod
```

During a rolling update you will see the old pod (`Running`) and a new pod simultaneously. The new pod cycles through:

```
0/1 Error  →  0/1 Running  →  0/1 Error  →  0/1 CrashLoopBackOff
```

This is **normal Kubernetes CrashLoopBackOff behaviour**. Kubernetes introduces an exponential backoff between restarts. If the application config is valid the pod will eventually reach `1/1 Running`.

### ThunderIDResource status

```bash
kubectl get thunderidresources
kubectl get thunderidresources -o custom-columns='NAME:.metadata.name,TYPE:.spec.resource_type,PHASE:.status.phase'   # filter/inspect by resource_type
```

| Phase | Meaning |
|-------|---------|
| `Pending` | Config change detected; waiting for ThunderID pod to apply it |
| `Ready` | ThunderIDResource loaded successfully |
| `Error` | ThunderID rejected the config or the pod is crashing |

```bash
kubectl describe thunderidresource <name>
```

Look at the **Events** section. Event types:

| Reason | Meaning |
|--------|---------|
| `Synced` | Config change detected; ThunderID pod is restarting |
| `Ready` | ThunderIDResource loaded successfully |
| `UnknownType` | `spec.resource_type` isn't set, or isn't one this operator maps to a ConfigMap — see the [`resource_type` values](#resource_type-values) table |
| `InvalidSpec` | `spec.resource_type` couldn't be parsed out of `spec` at all |
| `SyncFailed` | Writing this resource into its ConfigMap failed (e.g. a ConfigMap `Update` conflict) |

Events are per-object, not merged across `resource_type`s — `kubectl describe thunderidresource <name>` only ever shows *that* object's own events, same as before every kind was collapsed into the generic `ThunderIDResource` CRD (one CR per application/flow/user/etc. still exists, `spec.resource_type` just picks the ConfigMap it renders into). For a combined view across every kind at once, `kubectl describe thunderidinstance <name>` is the place to look: a `SyncFailed` also fires a `ThunderIDResourceSyncFailed` event on the owning `ThunderIDInstance` (naming the resource and its `resource_type`), so that one object's Events section becomes an aggregate feed of every dependent `ThunderIDResource`'s sync failures. It additionally shows a `DEGRADED` count summarizing how many dependent `ThunderIDResource`s (of any `resource_type`) currently have a sync failure, without needing to check each one individually.

**Pod-crash diagnostics — `CrashLoopBackOff`, `ThunderIDError`, `ThunderIDCrashed` — always land on the `ThunderIDInstance`, never on a `ThunderIDResource`**, regardless of which resource's reconcile happened to notice the crash:

```bash
kubectl describe thunderidinstance <name>
```

| Reason | Meaning |
|--------|---------|
| `CrashLoopBackOff` | The pod is crash-looping (or stuck resolving a config reference, e.g. a missing Secret) |
| `ThunderIDError` | ThunderID rejected the config — APP error code and message are shown |
| `ThunderIDCrashed` | Pod is in CrashLoopBackOff; no structured error found in logs |

The serving pod (and therefore whatever's currently crashing it) is shared by every `ThunderIDResource` bound to the instance — a crash log has no way to identify which resource's bad spec actually caused it, so attaching it to one resource in particular would just be a guess, and every other resource re-verifying health during the same outage would independently re-diagnose and re-emit the identical message onto itself too. Kubernetes' own event de-duplication (same object + reason + message) collapses repeat diagnoses of one ongoing crash into a single `ThunderIDInstance` Event with an increasing count instead.

> **Tip:** `ThunderIDError` only appears after the pod reaches stable `CrashLoopBackOff`. Wait until `kubectl get pod` shows `CrashLoopBackOff` before running `kubectl describe thunderidinstance`.

---

## Troubleshooting

### APP-1018 — Invalid request format

**Cause:** The `ThunderIDResource` references an OU, auth flow, or theme ID/handle that doesn't actually exist. This is almost always a wrong ID, not a missing resource — the image bakes in its own defaults automatically (see [Key facts](#architecture)), so a fresh `ThunderIDInstance` with no `resourcesConfigMap` at all should still resolve `default`/`console-app-flow`/etc. fine.

**Fix:** Double-check the exact ID or handle you're referencing. For a resource you created yourself via a CR, confirm the CR reached `Ready`. For a baked-in default, get the real ID straight from the image rather than guessing:

```bash
kubectl run inspect-image --image=<image tag in use> --restart=Never --command -- sleep 3600
kubectl exec inspect-image -- cat /opt/thunderid/bootstrap/01-default-resources.yaml
kubectl delete pod inspect-image --wait=false
```
`ouHandle`/`authFlowHandle` resolve fine at bootstrap on the current image — if you'd rather not chase down IDs at all, use the handle instead.

### APP-1024 — Invalid OAuth configuration

**Cause:** Incompatible OAuth2 settings in an `resource_type: application`'s `spec.inboundAuthConfig`. Common examples:

- `client_credentials` grant type with `tokenEndpointAuthMethod: none`
- `client_credentials` with `publicClient: true`

**Fix:** Read the `ThunderIDError` event message on the owning `ThunderIDInstance` (`kubectl describe thunderidinstance <name>`) — it states the exact constraint violated.

### APP-1042 — Application type is required

**Cause:** `spec.type` is unset on a `resource_type: application` `ThunderIDResource`. ThunderID's own bootstrap validation requires an application type — one of `browser`, `fullstack`, `mobile`, `m2m`, `custom`.

**Fix:** Set `spec.type` on the `ThunderIDResource`.

### Phase: Error but no ThunderIDError event

**Cause:** The pod has not yet reached stable `CrashLoopBackOff`. The operator fetches logs only after the pod enters `CrashLoopBackOff` (typically within 30–40 seconds of the first crash).

**Fix:** Wait for `kubectl get pod` to show `CrashLoopBackOff`, then re-run `kubectl describe thunderidinstance`.

### Pod never reaches Ready after applying ThunderIDInstance

**Cause 1:** The operator is not running, or it cannot reach the cluster.

**Fix:** Check that `make run` is still active in your terminal, or check the operator pod logs if deployed in-cluster:

```bash
kubectl logs -n operator-system deployment/operator-controller-manager
```

**Cause 2:** The Secret named by whichever `EnvironmentValues` is labeled for this instance, or by
any `spec.config.database.<scope>.postgres.passwordRef.name`/`spec.config.email.smtp.secretRef`, doesn't exist **in the instance's
own namespace** — see [Namespaces](#namespaces). Most often the Secret does exist, just in the
wrong namespace (e.g. an `EnvironmentValues`-generated one left in `default` while the instance itself
is elsewhere), or the `EnvironmentValues` is missing its `thunderid.io/instance` label entirely.

**Fix:** `kubectl describe pod <pod>` — look for `CreateContainerConfigError` and a
`secret "X" not found` message. `kubectl get events` also shows a `CreateContainerConfigError`
event on the `ThunderIDInstance` naming the missing Secret, and — if a same-named Secret exists
somewhere else in the cluster — which namespace it's actually in. Move or recreate the Secret in
the instance's namespace, or fix the `EnvironmentValues`'s `thunderid.io/instance` label / the
`*SecretRef` field.

### HPA stuck, pod never scales up from 0

**Cause:** If a Deployment was created with `replicas: 0`, the HPA treats it as intentionally stopped and will not scale it up.

**Fix:**

```bash
kubectl delete deployment <instance-name>
# The operator recreates it correctly
```

### Pod restarts are slow (SQLite mode)

**Cause:** `setup.sh` runs on every pod restart to reinitialise the SQLite database. This can only happen if `spec.dataVolumeSize` is unset — the operator rejects `ThunderIDInstance` specs with neither `spec.dataVolumeSize` nor every `spec.config.database` scope set to `type: postgres`, precisely to avoid this.

**Fix:** Set `spec.dataVolumeSize` (e.g. `"1Gi"`) on your `ThunderIDInstance`. This provisions a PVC for `repository/database` and gates `setup.sh` behind a marker file on it, so it only runs on the pod's first-ever start — every restart after that skips straight to `start.sh`. Only use this with `replicas: 1`. If you don't need SQLite at all, setting every `spec.config.database` scope's `type: postgres` avoids `setup.sh` entirely — see [Database](#database).

---

## Development

```bash
cd tools/k8s-operator

# Run tests
make test

# Build the operator binary
make build

# Generate CRD manifests after changing API types in api/v1alpha1/
make manifests

# Regenerate DeepCopy methods after changing API types
make generate

# Install CRDs into the cluster
make install

# Run locally against the current kubeconfig
make run

# Build and push the container image
make docker-build docker-push IMG=<registry>/thunderid-operator:<tag>

# Deploy to cluster
make deploy IMG=<registry>/thunderid-operator:<tag>

# Remove CRDs from the cluster
make uninstall

# Remove operator deployment
make undeploy
```

After changing any type in `api/v1alpha1/`, always run `make manifests generate` before running or deploying.
