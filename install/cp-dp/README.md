# Control Plane and Data Plane on Kubernetes

Sample `deployment.yaml` files for running ThunderID split into a Control Plane and one or more Data
Planes.

| File | Runs |
|---|---|
| [cp/deployment.yaml](cp/deployment.yaml) | The Control Plane: authoring, versioning, promotion. No runtime traffic. |
| [dp/deployment.yaml](dp/deployment.yaml) | One Data Plane environment: OAuth2/OIDC, flows, the gate. |

Each is a complete config, not a Helm template. Copy it, edit the hostnames and database details,
and mount it over `/opt/thunderid/deployment.yaml`.

## Images

Built from [Dockerfile.cp](../../Dockerfile.cp) and [Dockerfile.dp](../../Dockerfile.dp) at the
repository root. Both carry a default `deployment.yaml`, which the ConfigMap below replaces.

## Provisioning

Loading the schema is the whole of it. Neither plane is seeded, and nothing has to run before or
alongside the pods.

### Load the schema

Each datasource named in `deployment.yaml` gets its schema from the matching script in the
distribution's `dbscripts/`:

```
psql -h "$DB_HOST" -U "$DB_USER" -d configdb           -f dbscripts/configdb/postgres.sql
psql -h "$DB_HOST" -U "$DB_USER" -d entitydb           -f dbscripts/entitydb/postgres.sql
psql -h "$DB_HOST" -U "$DB_USER" -d runtime_transient  -f dbscripts/runtime-transient/postgres.sql
psql -h "$DB_HOST" -U "$DB_USER" -d runtime_persistent -f dbscripts/runtime-persistent/postgres.sql
psql -h "$DB_HOST" -U "$DB_USER" -d environmentdb      -f dbscripts/environmentdb/postgres.sql
```

`environmentdb` is the Control Plane's alone: it holds the environments and captured versions that
promotion compares. A Data Plane runs no environment manager and configures no such datasource, so
it loads the other four.

These scripts create tables unconditionally, so run them against empty databases only.

### Why there is nothing to seed

**The Control Plane's own tenant holds no resources.** The tenant named by
`server.system_deployment_id` is an identity rather than a resource owner. Callers authenticate
against the trusted issuer, which is validated from `deployment.yaml` against the issuer's JWKS and
reads no rows, and the plane issues no tokens of its own. Nothing seeds from it either: a new tenant
copies the oldest tenant of its own organization, or, when there is none, is provisioned from the
`bootstrap/` bundle in the image.

That bundle exists for the tenants created through `/system/tenants`, which provision themselves as
they are created. It stays in the image for that reason, and is never applied to the system tenant.

**A Data Plane holds nothing either, at first.** It is fed by a Control Plane: its organization
units, user types, applications, flows and themes all arrive on the first apply, so seeding any of
them would leave a second copy for that apply to sit alongside.

### What provisioning does not do

**It does not generate key material.** TLS, token signing, and encryption keys are generated and
mounted by whatever provisions the deployment. Every replica of a plane must mount the same ones,
because a token signed by one has to verify on another, and data encrypted under one key cannot be
read under a different one.

**It creates no secrets.** Everything is supplied through the environment: database passwords to the
pods, and on a Data Plane the vault token, from which runtime secrets are resolved at startup and as
they change. The database passwords come from there rather than from `deployment.yaml`, which holds
placeholders the server resolves at startup.

**It does not register a Data Plane with its Control Plane.** That happens on the Control Plane,
which issues the environment's channel token and shows it once. Put that token where
`channel.client.auth_token` reads it from before starting the pods.

## The audience a token binds to

Every access token is bound to exactly one resource server, and that resource server's identifier is
the audience. A client that sends the RFC 8707 `resource` parameter names it; one that does not falls
back to the deployment's `defaultResourceServer`, and with neither the request is refused with
`invalid_target`.

Two things follow, and both are handled for you.

**The audience differs per environment.** Promoted verbatim, every environment would name the audience
of the one the bundle was captured from, and a token minted for dev would name the same audience as
one minted for prod. So a capture replaces the origin of that identifier with a placeholder, and each
apply resolves it from the target's own base URL:

```
dev    →  https://dev.example.com/mcp
stage  →  https://stage.example.com/mcp
```

Set `baseUrl` on each environment's target. The path an operator chose is kept; only the origin is
replaced, and only for the resource server the deployment's own default points at. Any other resource
server is configuration an operator authored and is promoted as it stands.

**The console does not name it.** Leave `configuration.consoleClient.resourceIdentifier` unset on a
Data Plane. The console then sends no `resource`, the server resolves its own default, and there is no
second copy of the value to drift out of step with the one that was promoted.

## Mounting the config

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: thunderid-dp-config
data:
  deployment.yaml: |
    # contents of dp/deployment.yaml
```

```yaml
volumeMounts:
  - name: deployment-yaml
    mountPath: /opt/thunderid/deployment.yaml
    subPath: deployment.yaml
volumes:
  - name: deployment-yaml
    configMap:
      name: thunderid-dp-config
```

### The console's own configuration

The image also carries `apps/console/config.js`, which the browser reads. It ships with development
values and has to be replaced for any real deployment, in the same way as `deployment.yaml`, by
mounting over `/opt/thunderid/apps/console/config.js`.

**Leave `server.public_url` out.** With it and `hostname` both unset the console addresses whatever
host served it, which behind a load balancer is the load balancer. A value hardcoded here is used in
preference, so the console would load from the load balancer and then send every API call somewhere
else, past it, and cross-origin:

```js
window.__THUNDERID_RUNTIME_CONFIG__ = {
  plane: 'cp',
  client: {
    client_id: '<the console client registered at your issuer>',
    resource_identifier: 'https://cp.example.com',
    scopes: ['openid', 'profile', 'email', 'ou', 'tenant_instance:system', /* ... */],
  },
  // No server block: the console follows the host it was served from.
  trusted_issuer: {
    type: 'generic',
    public_url: 'https://idp.example.com/oauth2/token',
    client_id: '<the same client>',
    scopes: ['openid', 'profile', 'email', 'tenant_instance:system'],
  },
};
```

The scopes carry the `tenant_instance` prefix because `security.system_permission_prefix` sets it: a
scope names the instance it grants against, so a bare `system` from the issuer does not satisfy the
permissions this plane checks. The issuer must also emit the deployment id claim named by
`server.deployment_id_claim`; a token without it identifies no tenant and every request is refused.

## Secrets

Nothing secret belongs in the ConfigMap. Two mechanisms carry credentials in, both from the pod's
environment.

**Template placeholders.** A value written as `{{.NAME}}` in `deployment.yaml` is replaced from the
environment when the server loads the file. This is what the samples use for database passwords, the
channel token, and SMTP credentials.

```yaml
env:
  - name: DB_CONFIG_PASSWORD
    valueFrom:
      secretKeyRef: { name: thunderid-db, key: config-password }
  - name: DP_CHANNEL_TOKEN
    valueFrom:
      secretKeyRef: { name: thunderid-dp-channel, key: token }
```

Two things to know about this mechanism. A placeholder that has no matching variable is a startup
failure, not an empty string, so a missing credential surfaces immediately rather than at the first
request that needs it. And substitution scans the whole file including comments, so a placeholder
written in a comment demands a variable just as a real setting does.

**The Direct API secret is opt-in.** `server.security.direct_auth_secret` gates the `/auth`,
`/register/passkey` and `/access` routes on a Data Plane, where an application calls it directly
instead of redirecting a browser to the gate. Callers present it in the `Direct-Auth-Secret` header,
and without it those routes answer 401, which is what a deployment using only the redirect-based
OAuth2 flow wants. The Data Plane sample leaves it out for that reason: add it as a placeholder and
inject it from a Secret if you use those APIs. The Control Plane never reads it at all.

**`THUNDERID_KV_TOKEN`.** The OpenBao token is read straight from this variable, with no placeholder
in the file, and takes precedence over `kv.token`. Everything else about the vault (address, mount,
prefix, namespace) stays in `deployment.yaml`, where it can be read and reviewed. Runtime secrets are
resolved from the vault at startup and as they change.

```yaml
env:
  - name: THUNDERID_KV_TOKEN
    valueFrom:
      secretKeyRef: { name: openbao-dp, key: token }
```

A mounted file works too, as `token: "file:///var/run/secrets/openbao/token"`, which keeps the token
out of the process environment.

## Replicas

**The Data Plane scales.** Every replica dials the Control Plane and holds its own connection, and a
command goes to one of them rather than all, because they share a database. Leave
`channel.client.instance` empty so it defaults to the pod name; setting it to a fixed value would
make every replica present one identity and each new connection would evict the last.

The Data Plane's replicas must share their secret store, which is why the sample uses `kv` mode. On
`file` mode each pod keeps its own file, so a credential pushed to whichever pod the Control Plane
reached would be invisible to the rest. Do not run more than one replica on `file` mode.

**Run the Control Plane with one replica for now.** A Data Plane's connection lives in the memory of
the single Control Plane pod it dialled, and there is no routing between pods yet, so an apply that
arrives at any other pod reports the Data Plane as offline. Its database and environment data are
already shared, so this is the only thing standing in the way of scaling it.

## Storage

Neither plane needs durable storage of its own. The Control Plane keeps its environments and their
captured versions in `environmentdb`, and a Data Plane's configuration comes from the Control Plane,
its secrets from the vault, and its data from Postgres. Both are free to be rescheduled anywhere.

## Ports

| Plane | Port | Notes |
|---|---|---|
| Control Plane | 8095 | Serves `/cp/connect`, which Data Planes dial. |
| Data Plane | 8090 | Runtime traffic. |

The Service and any ingress in front of the Control Plane must allow WebSocket upgrades on
`/cp/connect` and a long idle timeout. The connection is held open, not polled, and an ingress that
times it out will cycle every Data Plane's connection.
