# ThunderID Helm Chart

This repository contains the Helm chart for ThunderID, a lightweight user and identity management system designed for modern application development.

## Configuration Value Types

ThunderID's configuration system supports multiple value formats for **any parameter** in the configuration:

1. **Direct Values** - Static values specified directly in YAML:
   ```yaml
   server:
     hostname: "localhost"
     port: 8090
   ```

2. **Environment Variables** - Use Go template syntax `{{.VARIABLE_NAME}}` to reference environment variables:
   ```yaml
   database:
     config:
       password: "{{.DB_PASSWORD}}"
   server:
     publicUrl: "{{.PUBLIC_URL}}"
   ```

3. **File References** - Use `file://` protocol to load content from files:
   ```yaml
   crypto:
     encryption:
       key: "file://repository/resources/security/crypto.key"
   ```
   Supports both quoted and unquoted paths:
   - `file://path/to/file` - Unquoted path (no spaces)
   - `file://"path/with spaces"` - Quoted path (with spaces allowed)
   - `file:///absolute/path` - Absolute paths
   - `file://relative/path` - Relative paths (resolved from the ThunderID installation directory)

## Prerequisites

### Infrastructure

- Running Kubernetes cluster ([minikube](https://kubernetes.io/docs/tasks/tools/#minikube) or an alternative cluster)
- **For Ingress-based deployment:** Kubernetes ingress controller ([NGINX Ingress](https://github.com/kubernetes/ingress-nginx) recommended)
- **For Gateway API deployment:** Gateway API implementation ([Envoy Gateway](https://gateway.envoyproxy.io/) recommended)

### Tools
| Tool          | Installation Guide | Version Check Command |
|---------------|--------------------|-----------------------|
| Git           | [Install Git](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git) | `git --version` |
| Helm          | [Install Helm](https://helm.sh/docs/intro/install/) | `helm version` |
| Docker        | [Install Docker](https://docs.docker.com/engine/install/) | `docker --version` |
| `kubectl`     | [Install kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) | `kubectl version` |

## Quick Start Guide

Follow these steps to deploy ThunderID in your Kubernetes cluster:

### 1. Install the ThunderID Helm chart

```bash
# Pull and install from GitHub Container Registry
helm install my-thunderid oci://ghcr.io/thunder-id/helm-charts/thunderid
```

If you wish to install another version, use the command below to specify the desired version.

```bash
helm install my-thunderid oci://ghcr.io/thunder-id/helm-charts/thunderid --version <VERSION>
```

> To see which chart versions are available, you can:
> - Visit the [ThunderID Helm Chart Registry](https://github.com/thunder-id/thunderid/pkgs/container/helm-charts%2Fthunderid) on GitHub Container Registry.

If you want to customize the installation, create a `custom-values.yaml` file with your configurations and use:

```bash
helm install my-thunderid oci://ghcr.io/thunder-id/helm-charts/thunderid -f custom-values.yaml
```

The command deploys ThunderID on the Kubernetes cluster with the default configuration. The [Parameters](#parameters) section lists the available parameters that can be configured during installation.

If you want to install ThunderID with SQLite databases, use the following command:

```bash
helm install my-thunderid oci://ghcr.io/thunder-id/helm-charts/thunderid \
  --set configuration.database.config.type=sqlite \
  --set configuration.database.runtime.type=sqlite \
  --set configuration.database.user.type=sqlite
```

**Note:** When using SQLite:
- **Persistence is automatically enabled** when any database is configured to use SQLite
- The setup job's init container will automatically copy SQLite databases from the image to a PVC
- Database files will persist across pod restarts

### 2. Get the External IP

After deploying ThunderID, you need to find its external IP address to access it outside the cluster. Run the following command to list the Ingress resources:

```bash
kubectl get ingress
```
**Output Fields:**

- **HOSTS** – hostname (e.g., `thunderid.local`)
- **ADDRESS** – External IP
- **PORTS** – Exposed ports (typically 80, 443)

After the installation is complete, you can access ThunderID via the Ingress hostname.

By default, ThunderID will be available at `http://thunderid.local`. You may need to add this hostname to your local hosts file or configure your DNS accordingly.

### Uninstalling the Chart

To uninstall/delete the `my-thunderid` deployment:

```bash
helm uninstall my-thunderid
```

This command removes all the Kubernetes components associated with the chart and deletes the release.

## Gateway API Setup (Alternative to Ingress)

ThunderID supports Kubernetes Gateway API as a modern alternative to Ingress. Enable it by setting `gateway.enabled=true` and `httproute.enabled=true` when installing the chart.

### Gateway API Prerequisites
- A TLS certificate stored as a Kubernetes Secret named `thunderid-tls` in your deployment `namespace`

### Create a TLS Certificate

Generate a self-signed certificate and create the Kubernetes Secret:

```bash
# Generate a self-signed certificate for your hostname (e.g., thunderid.local)
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout tls.key -out tls.crt \
  -subj "/CN=thunderid.local" \
  -addext "subjectAltName = DNS:thunderid.local"

# Create the TLS secret (must be named 'thunderid-tls' in your deployment namespace)
kubectl create secret tls thunderid-tls \
  --cert=tls.crt \
  --key=tls.key \
  -n <your-namespace>
```


## Parameters

The following table lists the configurable parameters of the ThunderID chart and their default values.

### Global Parameters

| Name                      | Description                                     | Default                                                 |
| ------------------------- | ----------------------------------------------- | ------------------------------------------------------- |
| `nameOverride`            | String to partially override common.names.fullname | `""`                                                  |
| `fullnameOverride`        | String to fully override common.names.fullname  | `""`                                                    |

### Deployment Parameters

| Name                                    | Description                                                                             | Default                        |
| --------------------------------------- | --------------------------------------------------------------------------------------- | ------------------------------ |
| `deployment.replicaCount`               | Number of ThunderID replicas                                                              | `2`                            |
| `deployment.strategy.rollingUpdate.maxSurge` | Maximum number of pods that can be created over the desired number during an update | `1`                           |
| `deployment.strategy.rollingUpdate.maxUnavailable` | Maximum number of pods that can be unavailable during an update              | `0`                           |
| `deployment.image.registry`             | ThunderID image registry                                                                  | `ghcr.io/thunder-id`             |
| `deployment.image.repository`           | ThunderID image repository                                                                | `thunderid`                      |
| `deployment.image.tag`                  | ThunderID image tag                                                                       | `latest`                       |
| `deployment.image.digest`               | ThunderID image digest (use either tag or digest)                                         | `""`                           |
| `deployment.image.pullPolicy`           | ThunderID image pull policy                                                               | `Always`                       |
| `deployment.terminationGracePeriodSeconds` | Pod termination grace period in seconds                                              | `10`                           |
| `deployment.container.port`             | ThunderID container port                                                                  | `8090`                         |
| `deployment.startupProbe.initialDelaySeconds` | Startup probe initial delay seconds                                               | `1`                            |
| `deployment.startupProbe.periodSeconds` | Startup probe period seconds                                                            | `2`                            |
| `deployment.startupProbe.failureThreshold` | Startup probe failure threshold                                                      | `30`                           |
| `deployment.livenessProbe.periodSeconds` | `Liveness` probe period seconds                                                        | `10`                           |
| `deployment.readinessProbe.initialDelaySeconds` | Readiness probe initial delay seconds                                           | `1`                            |
| `deployment.readinessProbe.periodSeconds` | Readiness probe period seconds                                                        | `10`                           |
| `deployment.resources.limits.cpu`       | CPU resource limits                                                                     | `200m`                         |
| `deployment.resources.limits.memory`    | Memory resource limits                                                                  | `100Mi`                        |
| `deployment.resources.requests.cpu`     | CPU resource requests                                                                   | `100m`                         |
| `deployment.resources.requests.memory`  | Memory resource requests                                                                | `50Mi`                         |
| `deployment.securityContext.readOnlyRootFilesystem` | Enable read-only root filesystem (must be false for SQLite)                     | `true`                         |
| `deployment.securityContext.enableRunAsUser` | Enforce user ID via pod security context                                               | `true`                         |
| `deployment.securityContext.runAsUser`  | User ID to run the container                                                            | `10001`                        |
| `deployment.securityContext.enableRunAsGroup` | Enable setting group ID for the container process                                 | `true`                         |
| `deployment.securityContext.runAsGroup` | Group ID to run the container                                                           | `10001`                        |
| `deployment.securityContext.enableFsGroup` | Enable setting `fsGroup` for volume ownership                                        | `true`                         |
| `deployment.securityContext.fsGroup`    | Group ID for mounted volumes (fixes SQLite permission issues on cloud platforms)        | `10001`                        |
| `deployment.securityContext.seccompProfile.enabled` | Enable `seccomp` profile                                                    | `false`                        |
| `deployment.securityContext.seccompProfile.type` | `Seccomp` profile type                                                         | `RuntimeDefault`               |
| `deployment.env`                        | Additional environment variables with plain values                                     | `[]`                           |
| `deployment.secretEnv`                  | Additional environment variables sourced from Kubernetes Secrets                         | `[]`                           |

### HPA Parameters

| Name                              | Description                                                      | Default                       |
| --------------------------------- | ---------------------------------------------------------------- | ----------------------------- |
| `hpa.enabled`                     | Enable Horizontal Pod `Autoscaler`                               | `true`                        |
| `hpa.maxReplicas`                 | Maximum number of replicas                                       | `10`                          |
| `hpa.averageUtilizationCPU`       | Target CPU usage percentage                                      | `65`                          |
| `hpa.averageUtilizationMemory`    | Target Memory usage percentage                                   | `75`                          |

### Service Parameters

| Name                             | Description                                                       | Default                      |
| -------------------------------- | ----------------------------------------------------------------- | ---------------------------- |
| `service.port`                   | ThunderID service port                                              | `8090`                       |

### Service Account Parameters

| Name                         | Description                                                | Default                       |
| ---------------------------- | ---------------------------------------------------------- | ----------------------------- |
| `serviceAccount.create`      | Enable creation of ServiceAccount                          | `true`                        |
| `serviceAccount.name`        | Name of the service account to use                         | `thunderid-service-account`     |

### PDB Parameters

| Name                        | Description                                                 | Default                       |
| --------------------------- | ----------------------------------------------------------- | ----------------------------- |
| `pdb.minAvailable`          | Minimum number of pods that must be available               | `50%`                         |

### Ingress Parameters

| Name                                  | Description                                                     | Default                      |
| ------------------------------------- | --------------------------------------------------------------- | ---------------------------- |
| `ingress.enabled`                     | Enable Ingress resource                                         | `true`                       |
| `ingress.className`                   | Ingress controller class                                        | `nginx`                      |
| `ingress.hostname`                    | Default host for the ingress resource                           | `thunderid.local`              |
| `ingress.paths[0].path`               | Path for the ingress resource                                   | `/`                          |
| `ingress.paths[0].pathType`           | Path type for the ingress resource                              | `Prefix`                     |
| `ingress.tlsSecretsName`              | TLS secret name for HTTPS                                       | `thunderid-tls`                |
| `ingress.commonAnnotations`           | Common annotations for ingress                                  | See values.yaml              |
| `ingress.customAnnotations`           | Custom annotations for ingress                                  | `{}`                         |

### `HTTPRoute` Parameters

| Name                                  | Description                                                                  | Default                      |
| ------------------------------------- | ---------------------------------------------------------------------------- | ---------------------------- |
| `httproute.enabled`                   | Enable Gateway API `HTTPRoute` resource (alternative to Ingress)             | `false`                      |
| `httproute.annotations`               | Annotations for the `HTTPRoute` resource                                     | `{}`                         |
| `httproute.parentRefs`                | Gateway references this route attaches to (required when enabled)            | `[]`                         |
| `httproute.hostnames`                 | `Hostnames` this route responds to                                           | `[]`                         |

### Gateway Parameters

| Name                                  | Description                                                                  | Default                      |
| ------------------------------------- | ---------------------------------------------------------------------------- | ---------------------------- |
| `gateway.enabled`                     | Enable Gateway API Gateway resource (alternative to Ingress)                 | `false`                      |
| `gateway.name`                        | Override the name of the Gateway resource                                    | `""`                         |
| `gateway.className`                   | Gateway class name                                                           | `eg` (Envoy default name)    |
| `gateway.tls.enabled`                 | Enable TLS listener on the Gateway                                           | `true`                       |
| `gateway.tls.secretName`              | TLS secret name for HTTPS listener                                           | `thunderid-tls`                |
| `gateway.tls.mode`                    | TLS reference mode                                                           | `Terminate`                  |

### Database Password Management

ThunderID provides flexible password management for database connections with automatic Kubernetes Secret integration.

#### Security Warning

⚠️ **Storing passwords as plaintext in values.yaml is NOT recommended for production.** Use Kubernetes Secrets or `--set` flags to store sensitive credentials securely.

#### How Password Management Works

ThunderID uses intelligent password detection based on the `password` and `passwordRef` fields:

1. **If `passwordRef.key` is set** → Uses external Secret (production pattern)
2. **If `password` has a value but `passwordRef.key` is empty** → Auto-converts to Helm-managed Secret (development/test pattern)
3. **If both are empty** → No password (SQLite-only deployments)

The auto-created Secret is created as a Helm pre-install/pre-upgrade hook to ensure it exists before the main deployment and setup job run.

#### Pattern 1: Auto-Convert to Helm-Managed Secret (For Development/Testing)

Provide passwords directly in the `password` field. Helm automatically creates a Secret named `<release-name>-db-credentials`:

```yaml
configuration:
  database:
    config:
      postgres:
        password: "my-secret-password-1"  # Auto-converted to Secret!
    runtime:
      postgres:
        password: "my-secret-password-2"
      redis:
        password: "my-runtime-redis-password"
    user:
      postgres:
        password: "my-secret-password-3"
```

**Best Practice:** Use `--set` flags to avoid committing passwords:
```bash
helm install my-thunderid oci://ghcr.io/thunder-id/helm-charts/thunderid \
  --set configuration.database.config.postgres.password=mypass1 \
  --set configuration.database.runtime.postgres.password=mypass2 \
  --set configuration.database.runtime.redis.password=myredispass \
  --set configuration.database.user.postgres.password=mypass3
```

Helm automatically:
- Creates `<release-name>-db-credentials` Secret as a pre-install/pre-upgrade hook
- Injects environment variables (`DB_CONFIG_PASSWORD`, `DB_RUNTIME_PASSWORD`, `DB_RUNTIME_REDIS_PASSWORD`, `DB_USER_PASSWORD`) into pods
- Updates pods when passwords change (via checksum annotations)

#### Pattern 2: External Secret (For Production - Recommended)

Reference a pre-existing Kubernetes Secret (created manually or by external-secrets-operator):

**Step 1:** Create your Secret:
```bash
kubectl create secret generic my-db-secrets \
  --from-literal=config-password=secret1 \
  --from-literal=runtime-password=secret2 \
  --from-literal=user-password=secret3
```

**Step 2:** Configure Helm to reference the external Secret:
```yaml
configuration:
  database:
    config:
      postgres:
        passwordRef:
          name: "my-db-secrets"      # Your Secret name
          key: "config-password"      # Key within Secret
    runtime:
      postgres:
        passwordRef:
          name: "my-db-secrets"
          key: "runtime-password"
      redis:
        passwordRef:
          name: "my-db-secrets"
          key: "runtime-redis-password"
    user:
      postgres:
        passwordRef:
          name: "my-db-secrets"
          key: "user-password"
```

When `passwordRef.key` is set, the `password` field is ignored and Helm uses your external Secret.

**Important:** The checksum annotation used to trigger pod `rollouts` is only computed for auto-generated Secrets. When you use an external Secret via `passwordRef` (Pattern 2), changes to that Secret will **not** automatically restart pods. You must either manually restart the pods or use a tool to watch for Secret changes and trigger pod `rollouts`.

**Important:** When you *do not* use `passwordRef.key` (i.e., you rely on the auto-generated Secret), the Helm chart will
base64-encode the `password` value directly into a Kubernetes Secret. In this mode, values like `"{{.DB_PASSWORD}}"` or
`"file:///secrets/pass"` are stored as literal strings in the Secret and **are not** resolved as environment variables or
file references by Helm. Environment variable placeholders (`{{.VAR}}`) and `file://` references are only resolved
when ThunderID reads configuration directly via its application config loader (e.g., from a ConfigMap or file).
They are not resolved when the value is first converted into a Kubernetes Secret by this chart.

#### Password Field Options
Password fields are available in `configuration.database.config.postgres`, `configuration.database.runtime.postgres`, `configuration.database.runtime.redis`, and `configuration.database.user.postgres`:

| Field                  | Description                                                                                                                                    | Example                      |
| ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------- |
| `password`             | Direct password value. When ThunderID reads config directly, this may also be an environment variable placeholder (`{{.VAR}}`) or file reference (`file://path`). When using the auto-generated Secret, the value is stored **as-is** in the Secret and such placeholders are **not** resolved. | `"mypassword"` or `"{{.DB_PASSWORD}}"` or `"file:///secrets/pass"` |
| `passwordRef.name`     | Kubernetes Secret name (optional, defaults to `<release-name>-db-credentials` for auto-convert)                                               | `"my-db-secrets"`            |
| `passwordRef.key`      | Secret key name. When set, `password` field is ignored and external Secret is used                                                            | `"config-password"`          |

### ThunderID Configuration Parameters

| Name                                              | Description                                                                                                                                             | Default                      |
|---------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------| ---------------------------- |
| `configuration.server.port`                       | ThunderID server port                                                                                                                                     | `8090`                       |
| `configuration.server.httpOnly`                   | Whether the server should run in HTTP-only mode                                                                                                         | `false`                      |
| `configuration.server.publicURL`                  | Public URL of the ThunderID server                                                                                                                        | `https://thunderid.local`      |
| `configuration.gateClient.hostname`               | Gate client hostname                                                                                                                                    | `thunderid.local`              |
| `configuration.gateClient.port`                   | Gate client port                                                                                                                                        | `443`                       |
| `configuration.gateClient.scheme`                 | Gate client scheme                                                                                                                                      | `https`                      |
| `configuration.gateClient.path`                   | Gate client base path                                                                                                                                   | `/gate`                      |
| `configuration.consoleClient.path`                | Console client base path                                                                                                                                | `/console`                   |
| `configuration.consoleClient.clientId`            | Console client ID                                                                                                                                       | `CONSOLE`                    |
| `configuration.consoleClient.scopes`              | Console client scopes                                                                                                                                   | `['openid', 'profile', 'email', 'system']` |
| `configuration.tls.minVersion`                    | Minimum TLS version                                                                                                                                     | `1.3`                        |
| `configuration.tls.certFile`                      | Server TLS certificate file path                                                                                                                        | `repository/resources/security/server.cert` |
| `configuration.tls.keyFile`                       | Server TLS key file path                                                                                                                                | `repository/resources/security/server.key`  |
| `configuration.crypto.encryption.key`             | Crypto encryption key (change the default key with a 32-byte (64 character) hex string in production)                                                   | `file://repository/resources/security/crypto.key` |
| `configuration.crypto.passwordHashing.algorithm`  | Password hashing algorithm                                            | `PBKDF2`                     |
| `configuration.crypto.passwordHashing.pbkdf2.salt_size` | PBKDF2 salt size                                                | `16`                         |
| `configuration.crypto.passwordHashing.pbkdf2.iterations` | PBKDF2 iterations                                              | `600000`                     |
| `configuration.crypto.passwordHashing.pbkdf2.key_size` | PBKDF2 key size                                                  | `32`                         |
| `configuration.crypto.passwordHashing.argon2id.salt_size` | Argon2id salt size                                            | `16`                         |
| `configuration.crypto.passwordHashing.argon2id.iterations` | Argon2id iterations                                          | `2`                          |
| `configuration.crypto.passwordHashing.argon2id.key_size` | Argon2id key size                                              | `32`                         |
| `configuration.crypto.passwordHashing.argon2id.memory` | Argon2id memory                                                  | `19456`                      |
| `configuration.crypto.passwordHashing.argon2id.parallelism` | Argon2id parallelism                                        | `1`                          |
| `configuration.crypto.passwordHashing.sha256.salt_size` | SHA256 salt size                                                | `16`                         |
| `configuration.crypto.keys[].id`                  | Signing key identifier                                                                                                                                  | `default-key`                |
| `configuration.crypto.keys[].certFile`            | Signing certificate file path                                                                                                                           | `repository/resources/security/signing.cert` |
| `configuration.crypto.keys[].keyFile`             | Signing key file path                                                                                                                                   | `repository/resources/security/signing.key`  |
| `configuration.database.config.type`            | Config database type (postgres or sqlite)                                                                                                               | `postgres`                   |
| `configuration.database.config.sqlite.path`      | SQLite database path (for SQLite only)                                                                                                                  | `repository/database/configdb.db` |
| `configuration.database.config.sqlite.options`   | SQLite options (for SQLite only)                                                                                                                        | `_journal_mode=WAL&_busy_timeout=5000&_pragma=foreign_keys(1)` |
| `configuration.database.config.sqlite.max_open_conns` | Maximum number of open connections for SQLite                                                                                                      | `500`                        |
| `configuration.database.config.sqlite.max_idle_conns` | Maximum number of idle SQLite connections                                                                                                          | `100`                        |
| `configuration.database.config.sqlite.conn_max_lifetime` | Maximum SQLite connection lifetime in seconds                                                                                                    | `3600`                       |
| `configuration.database.config.postgres.name`            | Postgres database name (for postgres only)                                                                                                              | `configdb`                  |
| `configuration.database.config.postgres.hostname`        | Postgres hostname (for postgres only)                                                                                                                   | `localhost` |
| `configuration.database.config.postgres.port`            | Postgres port (for postgres only)                                                                                                                       | `5432`                       |
| `configuration.database.config.postgres.username`        | Postgres username (for postgres only)                                                                                                                   | `dbuser`                   |
| `configuration.database.config.postgres.password`        | Config Postgres password - supports plaintext. When `passwordRef.key` is set, this field is ignored and the external Secret is used instead.           | `dbpassword`    |
| `configuration.database.config.postgres.passwordRef.name` | Kubernetes Secret name for config Postgres password                                                                                                     | `""`    |
| `configuration.database.config.postgres.passwordRef.key`  | Kubernetes Secret key for config Postgres password                                                                                                      | `""`    |
| `configuration.database.config.postgres.sslmode`         | Postgres SSL mode (for postgres only)                                                                                                                   | `require`                    |
| `configuration.database.config.postgres.max_open_conns`  | Maximum number of open connections to the database                                                                                                      | `500`                        |
| `configuration.database.config.postgres.max_idle_conns`  | Maximum number of idle connections in the pool                                                                                                          | `100`                        |
| `configuration.database.config.postgres.conn_max_lifetime` | Maximum lifetime of a connection in seconds                                                                                                             | `3600`                       |
| `configuration.database.runtime.type`             | Runtime database type (`postgres`, `sqlite`, or `redis`)                                                                                               | `postgres`                   |
| `configuration.database.runtime.sqlite.path`       | SQLite database path (for SQLite only)                                                                                                                  | `repository/database/runtimedb.db` |
| `configuration.database.runtime.sqlite.options`    | SQLite options (for SQLite only)                                                                                                                        | `_journal_mode=WAL&_busy_timeout=5000&_pragma=foreign_keys(1)` |
| `configuration.database.runtime.sqlite.max_open_conns` | Maximum number of open connections for SQLite                                                                                                      | `500`                        |
| `configuration.database.runtime.sqlite.max_idle_conns` | Maximum number of idle SQLite connections                                                                                                          | `100`                        |
| `configuration.database.runtime.sqlite.conn_max_lifetime` | Maximum SQLite connection lifetime in seconds                                                                                                    | `3600`                       |
| `configuration.database.runtime.postgres.name`             | Postgres database name (for postgres only)                                                                                                              | `runtimedb`                  |
| `configuration.database.runtime.postgres.hostname`         | Postgres hostname (for postgres only)                                                                                                                   | `localhost` |
| `configuration.database.runtime.postgres.port`             | Postgres port (for postgres only)                                                                                                                       | `5432`                      |
| `configuration.database.runtime.postgres.username`         | Postgres username (for postgres only)                                                                                                                   | `dbuser`                   |
| `configuration.database.runtime.postgres.password`         | Runtime Postgres password - supports plaintext. When `passwordRef.key` is set, this field is ignored and the external Secret is used instead.          | `dbpassword`     |
| `configuration.database.runtime.postgres.passwordRef.name`  | Kubernetes Secret name for runtime Postgres password                                                                                                    | `""`    |
| `configuration.database.runtime.postgres.passwordRef.key`   | Kubernetes Secret key for runtime Postgres password                                                                                                     | `""`    |
| `configuration.database.runtime.postgres.sslmode`          | Postgres SSL mode (for postgres only)                                                                                                                   | `require`                    |
| `configuration.database.runtime.postgres.max_open_conns`   | Maximum number of open connections to the database                                                                                                      | `500`                        |
| `configuration.database.runtime.postgres.max_idle_conns`   | Maximum number of idle connections in the pool                                                                                                          | `100`                        |
| `configuration.database.runtime.postgres.conn_max_lifetime` | Maximum lifetime of a connection in seconds                                                                                                             | `3600`                       |
| `configuration.database.runtime.redis.address`     | Redis server address in `host:port` format (for Redis only)                                                                                             | `""`                         |
| `configuration.database.runtime.redis.username`    | Redis username (for Redis only)                                                                                                                          | `""`                         |
| `configuration.database.runtime.redis.password`    | Runtime Redis password. When `passwordRef.key` is set, this field is ignored and the external Secret is used instead.                                  | `""`                         |
| `configuration.database.runtime.redis.passwordRef.name` | Kubernetes Secret name for runtime Redis password                                                                                                      | `""`                         |
| `configuration.database.runtime.redis.passwordRef.key` | Kubernetes Secret key for runtime Redis password                                                                                                      | `""`                         |
| `configuration.database.runtime.redis.db`          | Redis logical database index (0–15) (for Redis only)                                                                                                   | `0`                          |
| `configuration.database.runtime.redis.key_prefix`   | Prefix applied to all Redis keys written by ThunderID (for Redis only)                                                                                   | `""`                         |
| `configuration.database.user.type`                | User database type (postgres or sqlite)                                                                                                                 | `postgres`                   |
| `configuration.database.user.sqlite.path`          | SQLite database path (for SQLite only)                                                                                                                  | `repository/database/userdb.db` |
| `configuration.database.user.sqlite.options`       | SQLite options (for SQLite only)                                                                                                                        | `_journal_mode=WAL&_busy_timeout=5000&_pragma=foreign_keys(1)` |
| `configuration.database.user.sqlite.max_open_conns` | Maximum number of open connections for SQLite                                                                                                        | `500`                        |
| `configuration.database.user.sqlite.max_idle_conns` | Maximum number of idle SQLite connections                                                                                                            | `100`                        |
| `configuration.database.user.sqlite.conn_max_lifetime` | Maximum SQLite connection lifetime in seconds                                                                                                      | `3600`                       |
| `configuration.database.user.postgres.name`                | Postgres database name (for postgres only)                                                                                                              | `userdb`                     |
| `configuration.database.user.postgres.hostname`            | Postgres hostname (for postgres only)                                                                                                                   | `localhost` |
| `configuration.database.user.postgres.port`                | Postgres port (for postgres only)                                                                                                                       | `5432`                       |
| `configuration.database.user.postgres.username`            | Postgres username (for postgres only)                                                                                                                   | `dbuser`                   |
| `configuration.database.user.postgres.password`            | User Postgres password - supports plaintext. When `passwordRef.key` is set, this field is ignored and the external Secret is used instead.              | `dbpassword`        |
| `configuration.database.user.postgres.passwordRef.name`     | Kubernetes Secret name for user Postgres password                                                                                                       | `""`    |
| `configuration.database.user.postgres.passwordRef.key`      | Kubernetes Secret key for user Postgres password                                                                                                        | `""`    |
| `configuration.database.user.postgres.sslmode`             | Postgres SSL mode (for postgres only)                                                                                                                   | `require`                    |
| `configuration.database.user.postgres.max_open_conns`      | Maximum number of open connections to the database                                                                                                      | `500`                        |
| `configuration.database.user.postgres.max_idle_conns`      | Maximum number of idle connections in the pool                                                                                                          | `100`                        |
| `configuration.database.user.postgres.conn_max_lifetime`   | Maximum lifetime of a connection in seconds                                                                                                             | `3600`                       |
| `configuration.cache.disabled`                    | Disable cache                                                                                                                                           | `true`                       |
| `configuration.cache.type`                        | Cache type                                                                                                                                              | `inmemory`                   |
| `configuration.cache.size`                        | Cache size                                                                                                                                              | `1000`                       |
| `configuration.cache.ttl`                         | Cache TTL in seconds                                                                                                                                    | `3600`                       |
| `configuration.cache.evictionPolicy`              | Cache eviction policy                                                                                                                                   | `LRU`                        |
| `configuration.cache.cleanupInterval`             | Cache cleanup interval in seconds                                                                                                                       | `300`                        |
| `configuration.cache.redis.address`               | Redis server address (host:port). Required when type is `redis`                                                                                         |                              |
| `configuration.cache.redis.username`              | Redis authentication username                                                                                                                           | `""`                        |
| `configuration.cache.redis.password`              | Redis authentication password. For production, avoid plaintext in values.yaml and use Kubernetes Secrets (or pass via `--set`) instead.                | `""`                        |
| `configuration.cache.redis.passwordRef.name`      | Kubernetes Secret name for Redis password. Leave empty to use auto-created `<release-name>-db-credentials` Secret when password field is set          | `""`                        |
| `configuration.cache.redis.passwordRef.key`       | Kubernetes Secret key for Redis password. When set, overrides `password` field and uses external Secret                                                | `""`                        |
| `configuration.cache.redis.db`                    | Redis database number                                                                                                                                   | `0`                          |
| `configuration.cache.redis.keyPrefix`             | Prefix for all Redis cache keys                                                                                                                         | `thunderid`                  |
| `configuration.jwt.issuer`                        | JWT issuer (derived from server.publicUrl if not set)                                                                                                   | derived                      |
| `configuration.jwt.validityPeriod`                | JWT validity period in seconds                                                                                                                          | `3600`                       |
| `configuration.jwt.audience`                      | Default audience for auth assertions                                                                                                                    | `application`                |
| `configuration.jwt.preferredKeyId`                | Preferred key ID for signing JWTs (must match a key in configuration.crypto.keys)                                                                       | `default-key`                |
| `configuration.oauth.refreshToken.renewOnGrant`   | Renew refresh token on grant                                                                                                                            | `false`                      |
| `configuration.oauth.refreshToken.validityPeriod` | Refresh token validity period in seconds                                                                                                                | `86400`                      |
| `configuration.flow.defaultAuthFlowHandle`        | Default authentication flow handle                                                                                                                      | `default-basic-flow`         |
| `configuration.flow.maxVersionHistory`            | Maximum flow version history to retain                                                                                                                  | `3`                          |
| `configuration.flow.autoInferRegistration`        | Enable auto-infer registration flow                                                                                                                     | `true`                       |
| `configuration.cors.allowedOrigins`               | CORS allowed origins                                                                                                                                    | See values.yaml              |
| `configuration.passkey.allowedOrigins`            | Passkey allowed origins                                                                                                                                 | `[]`                         |
| `configuration.consent.enabled`                   | Enable consent service                                                                                                                                  | `false`                      |
| `configuration.consent.baseUrl`                   | Base URL of the consent service                                                                                                                         | `""`                         |
| `configuration.consent.timeout`                   | Timeout for consent service API calls in seconds                                                                                                        | `5`                          |
| `configuration.consent.maxRetries`                | Max retry attempts for transient errors when calling consent service API                                                                                | `3`                          |

### Persistence Parameters

Persistence is **automatically enabled** when using SQLite as the database type for any database (config, runtime, or user). It creates a PersistentVolumeClaim to store SQLite database files.

| Name                                   | Description                                                     | Default                      |
| -------------------------------------- | --------------------------------------------------------------- | ---------------------------- |
| `persistence.enabled`                  | Enable persistence for SQLite databases (auto-enabled for SQLite) | `false`                    |
| `persistence.storageClass`             | Storage class name (use "-" for no storage class)               | `""`                         |
| `persistence.accessMode`               | PVC access mode                                                 | `ReadWriteOnce`              |
| `persistence.size`                     | PVC storage size                                                | `1Gi`                        |
| `persistence.annotations`              | Additional annotations for PVC                                  | `{}`                         |

**Note:** 
- When any database is configured to use SQLite, a PersistentVolumeClaim (PVC) is **always created** to store the database files, regardless of the `persistence.enabled` or `setup.enabled` settings.
- The PVC is mounted by the setup job's init container (if `setup.enabled` is true) to initialize the database, and by the main ThunderID deployment for ongoing operation.
- You can customize the storage size and storage class for the PVC using the `persistence.size` and `persistence.storageClass` values.

### Declarative Resources Parameters

Declarative resources can be mounted into ThunderID's `repository/resources` directory from either a ConfigMap or Secret.

| Name                                   | Description                                                     | Default                      |
| -------------------------------------- | --------------------------------------------------------------- | ---------------------------- |
| `declarativeResources.enabled`         | Enable declarative resources mount                              | `false`                      |
| `declarativeResources.mountPath`       | Mount path inside container                                     | `/opt/thunderid/repository/resources` |
| `declarativeResources.readOnly`        | Mount declarative resources as read-only                        | `true`                       |
| `declarativeResources.configMap.name`  | Existing ConfigMap name containing declarative resources        | `""`                        |
| `declarativeResources.configMap.items` | ConfigMap items to mount (string or `{key,path}`; empty = all keys) | `[]`                    |
| `declarativeResources.secret.name`     | Existing Secret name containing declarative resources           | `""`                        |
| `declarativeResources.secret.items`    | Secret items to mount (string or `{key,path}`; empty = all keys) | `[]`                      |

**Validation rules:**
- When `declarativeResources.enabled=true`, set exactly one source: `declarativeResources.configMap.name` or `declarativeResources.secret.name`.
- Setting both sources at once fails template rendering.

When `declarativeResources.enabled` is set to `true`, the generated ThunderID `deployment.yaml` also sets:

```yaml
declarative_resources:
  enabled: true
```

Example using a ConfigMap:

```yaml
declarativeResources:
  enabled: true
  mountPath: /opt/thunderid/repository/resources
  configMap:
    name: thunderid-declarative-resources
    items:
      - organizations/default/organization.yaml
      - identity-providers/google.yaml
```

### Declarative Resources Mounting Guide

When declarative resources are enabled, the chart mounts the same volume into both:
- ThunderID deployment container
- Setup job container

This ensures resources are available during initialization and at runtime.

#### How directory mapping works

Each entry in `declarativeResources.configMap.items` (or `declarativeResources.secret.items`) supports two formats:

- String format: uses the same value for both `key` and `path`
  - Example: `applications/application1.yaml`
- Object format: explicit `key -> path` mapping
  - Example: `{ key: app1, path: applications/application1.yaml }`

Use object format when you need to mount a source key to a different directory/file path under `declarativeResources.mountPath`.

When `items` are provided, the chart mounts declarative resources file-by-file using `subPath`. This preserves existing files already present in ThunderID's `repository/resources` directory.

Resulting file path example:
- With `path: applications/application1.yaml`, file is mounted at `/opt/thunderid/repository/resources/applications/application1.yaml`

#### End-to-end example with ConfigMap

Create a ConfigMap where keys represent the target directory structure:

```bash
kubectl create configmap thunderid-declarative-resources \
  --from-file=applications/application1.yaml=./declarative-resources/applications/application1.yaml \
  --from-file=organizations/default/organization.yaml=./declarative-resources/organizations/default/organization.yaml \
  --from-file=identity-providers/google.yaml=./declarative-resources/identity-providers/google.yaml
```

Configure Helm values:

```yaml
declarativeResources:
  enabled: true
  mountPath: /opt/thunderid/repository/resources
  readOnly: true
  configMap:
    name: thunderid-declarative-resources
    items:
      - key: applications/application1.yaml
        path: applications/application1.yaml
      - key: organizations/default/organization.yaml
        path: organizations/default/organization.yaml
      - key: identity-providers/google.yaml
        path: identity-providers/google.yaml

# Example with explicit key/path remapping
declarativeResources:
  enabled: true
  configMap:
    name: thunderid-declarative-resources
    items:
      - key: app1
        path: applications/application1.yaml
      - key: idp-google
        path: identity-providers/google.yaml
```

Install or upgrade:

```bash
helm upgrade --install my-thunderid oci://ghcr.io/thunder-id/helm-charts/thunderid -f custom-values.yaml
```

#### Example with Secret Source

Use this when resource files contain sensitive values.

```yaml
declarativeResources:
  enabled: true
  secret:
    name: thunderid-declarative-resources-secret
    items:
      - key: app1
        path: applications/application1.yaml
      - key: idp-google
        path: identity-providers/google.yaml
```

#### Mount All Keys From Source

If `items` is empty, all keys from the ConfigMap/Secret are mounted:

```yaml
declarativeResources:
  enabled: true
  configMap:
    name: thunderid-declarative-resources
    items: []
```

  Note: With empty `items`, Kubernetes mounts the source at the directory level. This can hide existing files in `repository/resources` during pod runtime. To preserve bundled files and add only selected declarative resources, configure explicit `items`.

#### Runtime Configuration Sync

Setting `declarativeResources.enabled: true` also updates generated ThunderID config:

```yaml
declarative_resources:
  enabled: true
```

#### Verify Mounted Files and Config

```bash
# Check mounted files inside ThunderID pod
kubectl exec -it deploy/my-thunderid -- ls -R /opt/thunderid/repository/resources

# Confirm declarative_resources.enabled in generated deployment config
kubectl exec -it deploy/my-thunderid -- grep -n "declarative_resources\|enabled" /opt/thunderid/conf/deployment.yaml
```

#### Common configuration errors

- Error: `Invalid declarativeResources configuration: set only one source, declarativeResources.configMap.name or declarativeResources.secret.name.`
  - Cause: Both `configMap.name` and `secret.name` are set.
  - Fix: Keep only one source.

- Error: `Invalid declarativeResources configuration: set declarativeResources.configMap.name or declarativeResources.secret.name when declarativeResources.enabled=true.`
  - Cause: Declarative resources enabled without a source.
  - Fix: Set either `declarativeResources.configMap.name` or `declarativeResources.secret.name`.

### Setup Job Parameters

The setup job runs `setup.sh` as a one-time Helm pre-install hook to initialize ThunderID with default resources (admin user, organization, etc.).

| Name                                   | Description                                                     | Default                      |
| -------------------------------------- | --------------------------------------------------------------- | ---------------------------- |
| `setup.enabled`                        | Enable setup job (runs on install via Helm hook)                | `true`                       |
| `setup.backoffLimit`                   | Number of retries if setup fails                                | `3`                          |
| `setup.preserveJob`                    | Preserve job after completion (false = delete on success)       | `false`                      |
| `setup.ttlSecondsAfterFinished`        | Time to keep failed jobs (only if preserveJob=false)            | `86400` (24 hours)           |
| `setup.debug`                          | Enable debug mode for setup                                     | `false`                      |
| `setup.args`                           | Additional command-line arguments for setup.sh                  | `[]`                         |
| `setup.env`                            | Additional environment variables for setup job                  | `[]`                         |
| `setup.secretEnv`                      | Additional environment variables sourced from Kubernetes Secrets | `[]`                         |
| `setup.resources.requests.cpu`         | CPU request for setup job                                       | `100m`                       |
| `setup.resources.requests.memory`      | Memory request for setup job                                    | `50Mi`                       |
| `setup.resources.limits.cpu`           | CPU limit for setup job                                         | `200m`                       |
| `setup.resources.limits.memory`        | Memory limit for setup job                                      | `100Mi`                      |
| `setup.extraVolumeMounts`              | Additional volume mounts for setup job                          | `[]`                         |
| `setup.extraVolumes`                   | Additional volumes for setup job                                | `[]`                         |

Environment variable item structure for plain value environment variables in `deployment.env` and `setup.env`:

- `name`: Environment variable name in the container.
- `value`: Plain value from Helm values.

Example:

```yaml
deployment:
  env:
    - name: LOG_LEVEL
      value: debug
    - name: EXTERNAL_API_BASE_URL
      value: https://api.example.com
```

Environment variable item structure for secret-backed environment variables in `deployment.secretEnv` and `setup.secretEnv`:

- `name`: Environment variable name in the container.
- `secretName`: Kubernetes Secret resource name.
- `secretKey`: Key in the Secret data map.
- `optional`: Optional `boolean` passed to `secretKeyRef.optional`.

**Job Retention Behavior:**
- When `preserveJob=false` (default): Successful jobs are deleted immediately. Failed jobs are kept for `ttlSecondsAfterFinished` (24 hours) to allow debugging.
- When `preserveJob=true`: Job is kept indefinitely regardless of success/failure status. Use this for troubleshooting or audit purposes.

### Bootstrap Script Parameters

Bootstrap scripts extend ThunderID's setup process by adding your own initialization logic. These scripts run as part of the setup job.

#### Understanding Default Bootstrap Scripts

ThunderID provides these default bootstrap scripts in `/opt/thunderid/bootstrap/`:
- **`common.sh`** - Helper functions for logging (`log_info`, `log_success`, `log_warning`, `log_error`) and API calls (`thunderid_api_call`)
- **`01-default-resources.sh`** - Creates admin user, default organization, and Person user type
- **`02-sample-resources.sh`** - Creates sample resources for testing

#### Configuration Parameters

| Name                        | Description                                                                      | Default |
| --------------------------- | -------------------------------------------------------------------------------- | ------- |
| `bootstrap.scripts`         | Inline custom bootstrap scripts (key: filename, value: content)                 | `{}`    |
| `bootstrap.configMap.name`  | Name of external ConfigMap containing bootstrap scripts                          | `""`    |
| `bootstrap.configMap.files` | List of script filenames to mount from ConfigMap (empty = mount entire ConfigMap) | `[]`    |

#### Three Bootstrap Patterns

**Pattern 1: Add Inline Scripts** (Preserves Defaults)

Use `bootstrap.scripts` to define scripts directly in values.yaml. These scripts are added to the default bootstrap scripts.

```yaml
bootstrap:
  scripts:
    30-custom-users.sh: |
      #!/bin/bash
      set -e
      SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]:-$0}")"
      source "${SCRIPT_DIR}/common.sh"

      log_info "Creating custom user..."
      thunderid_api_call POST "/users" '{"type":"person","attributes":{"username":"alice","password":"alice123","sub":"alice","email":"alice@example.com"}}'
      log_success "User created"
```

- ✅ Preserves ThunderID's default scripts (`common.sh`, `01-*`, `02-*`)
- ✅ Can use helper functions from `common.sh`
- ✅ No additional configuration needed

---

**Pattern 2: Add External ConfigMap Scripts** (Preserves Defaults)

Use `bootstrap.configMap` with a `files` list to mount specific scripts from an external ConfigMap.

Create your ConfigMap:
```bash
kubectl create configmap my-bootstrap \
  --from-file=30-users.sh=./30-users.sh \
  --from-file=40-apps.sh=./40-apps.sh
```

Configure Helm values:
```yaml
bootstrap:
  configMap:
    name: "my-bootstrap"
    files:
      - 30-users.sh
      - 40-apps.sh
```

- ✅ Preserves ThunderID's default scripts
- ✅ Can use helper functions from `common.sh`
- ✅ Scripts managed separately from Helm chart

---

**Pattern 3: Replace All Scripts with ConfigMap** (Complete Replacement)

⚠️ **WARNING**: This entirely replaces ThunderID's default bootstrap scripts. Use only if you need complete control.

Use `bootstrap.configMap` **without** specifying `files` to mount the entire ConfigMap and replace all defaults.

Create your complete ConfigMap (must include `common.sh`):
```bash
kubectl create configmap complete-bootstrap \
  --from-file=common.sh=./common.sh \
  --from-file=01-my-setup.sh=./01-my-setup.sh
```

Configure Helm values:
```yaml
bootstrap:
  configMap:
    name: "complete-bootstrap"
    # No files list = mounts entire ConfigMap (replaces all defaults)
```

- ⚠️ **Removes ALL default scripts** (`common.sh`, `01-default-resources.sh`, `02-sample-resources.sh`)
- ⚠️ You MUST provide your own `common.sh` with required helper functions
- ⚠️ No default admin user, organization, or schemas will be created
- ✅ Complete control over bootstrap process

**For comprehensive examples, helper function documentation, and best practices, see:** [Custom Bootstrap Guide](../../docs/guides/setup/custom-bootstrap.md)

### Custom Configuration

The ThunderID configuration file (deployment.yaml) can be customized by overriding the default values in the values.yaml file.
Or you can directly update the values in conf/deployment.yaml before deploying the Helm chart.

### Database Configuration

ThunderID supports both sqlite and postgres databases. By default, postgres is configured.

Make sure to create the necessary databases and users in your Postgres instance before deploying ThunderID. The values.yaml should be overridden with the required database configurations for the DB created.

Note: Use sqlite only if you are running a single pod.
