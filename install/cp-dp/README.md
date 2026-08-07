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

Before a plane's pods start, its database has to be provisioned. There is a script per plane in
[scripts/](scripts/).

| Plane | Script | Run it | Seeds |
|---|---|---|---|
| Control Plane | [scripts/setup-control-plane.sh](scripts/setup-control-plane.sh) | Once, when the Control Plane is first stood up | The baseline, for the root tenant |
| Data Plane | [scripts/setup-data-plane.sh](scripts/setup-data-plane.sh) | Once per environment, when it is first stood up | Nothing. Schema only |

**A Data Plane seeds nothing.** It is fed by a Control Plane: its organization units, user types,
applications, flows and themes all arrive on the first apply, so seeding any of them would leave a
second copy for that apply to sit alongside. Loading the schema is all it needs before the Control
Plane reaches it, which is also why it needs no admin credentials.

These run **from outside the deployment**, from an operator's machine or a platform task. They are
not part of the image and are not meant to run in a pod. What they need is an unpacked distribution
for the plane, holding that deployment's `deployment.yaml`, and reach to its database.

```
tar/unzip the ThunderID Control Plane distribution somewhere, then:

THUNDERID_HOME=/path/to/distribution \
ADMIN_USERNAME=admin \
ADMIN_PASSWORD=... \
DB_CONFIG_PASSWORD=... DB_RUNTIME_TRANSIENT_PASSWORD=... \
DB_ENTITY_PASSWORD=... DB_RUNTIME_PERSISTENT_PASSWORD=... \
  ./scripts/setup-control-plane.sh
```

`deployment.yaml` is read and never written: put the deployment's own configuration in the
distribution first, and the script provisions what it describes. The database passwords come from the
environment rather than from that file, because in a cluster it holds placeholders and the server
resolves them at startup. Pass the same values, from the same Secret.

Provisioning happens once. Re-running after a failure part way through is safe: a schema is loaded
only into an empty database, key material is generated only when absent, and seeding upserts.

### What they do not do

**They do not generate key material.** TLS, token signing, and encryption keys are generated and
mounted by whatever provisions the deployment. Every replica of a plane must mount the same ones,
because a token signed by one has to verify on another, and data encrypted under one key cannot be
read under a different one.

The scripts still need that material reachable, because seeding the baseline loads `deployment.yaml`
and loading it reads every file the configuration points at. Put the same material the pods will
mount under the distribution first; the script checks up front and names anything missing rather than
failing part way through.

**They create no secrets.** Everything else is supplied through the environment: database passwords
to the script and to the pods, and on a Data Plane the vault token, from which runtime secrets are
resolved at startup and as they change.

Everything the scripts do land in the database, so once they have run there is nothing to carry
across.

### What the scripts deliberately leave alone

**The Control Plane's other tenants.** `setup-control-plane.sh` provisions only the tenant named by
`server.system_deployment_id`, because in token mode nothing can call `/system/tenants` to create any
other tenant until that one exists. Every tenant after it is created through that API, which
provisions its baseline itself. Do not rerun the script per tenant.

**Registering a Data Plane with its Control Plane.** That happens on the Control Plane, which issues
the environment's channel token and shows it once. Put that token where `channel.client.auth_token`
reads it from before starting the pods.

PostgreSQL schema loading needs `psql` on the machine running the script. Without it the script says
so and prints the exact command to run instead, rather than leaving a deployment pointed at an empty
database.

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

The Control Plane's `environment_manager.data_dir` holds environments and their captured versions,
which is the history that promotion compares against. Back it with a PersistentVolumeClaim. The Data
Plane needs no durable storage of its own: its configuration comes from the Control Plane, its
secrets from the vault, and its data from Postgres.

## Ports

| Plane | Port | Notes |
|---|---|---|
| Control Plane | 8095 | Serves `/cp/connect`, which Data Planes dial. |
| Data Plane | 8090 | Runtime traffic. |

The Service and any ingress in front of the Control Plane must allow WebSocket upgrades on
`/cp/connect` and a long idle timeout. The connection is held open, not polled, and an ingress that
times it out will cycle every Data Plane's connection.
