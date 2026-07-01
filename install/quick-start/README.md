# ThunderID Quick Start

Run ThunderID locally using Docker Compose. This is the fastest way to get ThunderID up and running with all dependencies configured.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose installed
- Terminal access

---

## Running ThunderID

Start all services with a single command:

```bash
docker compose up
```

This will automatically:
1. **Initialize** the database from the image
2. **Run setup** — bootstraps default resources (admin user, sample apps, etc.)
3. **Start the server** — ThunderID is ready to serve requests

Once running, ThunderID is available at:

| URL | Description |
|------|-------------|
| `https://localhost:8090` | ThunderID Server |
| `https://localhost:8090/console` | ThunderID Console |

> **Default credentials:** `admin` / `admin`

---

## Services

The Compose file defines three services:

| Service | Description |
|---|---|
| `thunderid-db-init` | One-shot container that copies the initial database files to a shared volume |
| `thunderid-setup` | One-shot container that bootstraps default resources via `setup.sh` |
| `thunderid` | The ThunderID server — starts after setup completes |

The `thunderid-db-init` and `thunderid-setup` services run once and exit. Only `thunderid` stays running.

---

## Stopping ThunderID

```bash
# Stop and keep data
docker compose down

# Stop and remove all data (fresh start)
docker compose down -v
```

---

## Custom Host and Port

By default, ThunderID runs on `localhost:8090`. To run it on a different hostname or port — for example a custom domain, a server IP, or a local alias like `thunderid.local` — you need to override three configuration files.

### How It Works

The Docker image bakes in default configuration files. You can override them with volume mounts — no image rebuild required.

| File in container | Purpose |
|---|---|
| `/opt/thunderid/deployment.yaml` | Backend server — bind address, public URL, Gate client redirect |
| `/opt/thunderid/apps/console/config.js` | Management Console frontend |
| `/opt/thunderid/apps/gate/config.js` | Gate login app frontend |

### Step 1: Create Your Configuration Files

Create the following three files in the same directory as `docker-compose.yml`:

```text
.
├── docker-compose.yml
├── deployment.yaml       ← backend configuration
├── console-config.js     ← ThunderID Console configuration
└── gate-config.js        ← Gate login app configuration
```

#### `deployment.yaml`

```yaml
server:
  hostname: "0.0.0.0"                            # Keep as-is — binds to all interfaces
  port: <your-port>                              # e.g. 8090
  public_url: "https://<your-host>:<your-port>" # e.g. https://thunderid.local:8090

gate_client:
  hostname: "<your-host>"
  port: <your-port>
  scheme: "https"
  path: "/gate"

passkey:
  allowed_origins:
    - "https://<your-host>:<your-port>"  # e.g. https://thunderid.local:8090

# Other configurations...
```

> **CORS allowed origins** live in the server-config `cors` section, not in `deployment.yaml`. Add them to `config/resources/server_configs/cors.yaml`, or update them at runtime with `PUT /server-config/cors`:
>
> ```yaml
> name: cors
> value:
>   allowedOrigins:
>     - "https://<your-host>:<your-port>"  # e.g. https://thunderid.local:8090
> ```

#### `console-config.js`

```js
window.__THUNDERID_RUNTIME_CONFIG__ = {
  client: {
    base: '/console',
    client_id: 'CONSOLE',
    scopes: ['openid', 'profile', 'email', 'system'],
  },
  server: {
    public_url: 'https://<your-host>:<your-port>', // e.g. https://thunderid.local:8090
  },
};
```

#### `gate-config.js`

```js
window.__THUNDERID_RUNTIME_CONFIG__ = {
  client: {
    base: '/gate',
  },
  server: {
    public_url: 'https://<your-host>:<your-port>', // e.g. https://thunderid.local:8090
  },
};
```

### Step 2: Add Volume Mounts to `docker-compose.yml`

Add the following volume mounts to the `thunderid-setup` and `thunderid` services:

```yaml
services:
  thunderid-setup:
    # ...
    volumes:
      # ...
      - ./deployment.yaml:/opt/thunderid/deployment.yaml:ro

  thunderid:
    # ...
    ports:
      - "<your-port>:<your-port>"  # Update if changing the port, e.g. 9090:9090
    volumes:
      # ...
      - ./deployment.yaml:/opt/thunderid/deployment.yaml:ro
      - ./console-config.js:/opt/thunderid/apps/console/config.js:ro
      - ./gate-config.js:/opt/thunderid/apps/gate/config.js:ro
```

> **Note:** `deployment.yaml` must be mounted into `thunderid-setup` too, because the setup process starts a temporary server to bootstrap resources. The frontend `config.js` files only need to be in the `thunderid` service.
>
> The `ports` mapping only needs updating if you change the port number. If you are only changing the hostname, leave it as `8090:8090`.

### Step 3: Start ThunderID

```bash
docker compose up
```

### Example: Using `thunderid.local`

First, add the alias to your hosts file:

**macOS / Linux:**
```bash
echo "127.0.0.1 thunderid.local" | sudo tee -a /etc/hosts
```

**Windows (run as Administrator):**
```powershell
Add-Content -Path "C:\Windows\System32\drivers\etc\hosts" -Value "127.0.0.1 thunderid.local"
```

Then replace `<your-host>` with `thunderid.local` and `<your-port>` with `8090` (or your chosen port) in all three configuration files.

---

## Running with Redis Cache

By default, ThunderID cache is disabled. To use Redis, start a Redis instance and configure ThunderID to point to it.

### Step 1: Start Redis

From the repository root, use the Redis Docker Compose file provided under `install/local-development/redis`:

```bash
docker compose -f ./install/local-development/redis/docker-compose.yml up -d
```

This starts Redis with a persistent volume.

### Step 2: Configure ThunderID to Use Redis

Add the following cache section to your `deployment.yaml` override file (see [Custom Host and Port](#custom-host-and-port) for how to create and mount it), then start ThunderID with `docker compose up`.

```yaml
cache:
  type: "redis"
  redis:
    address: "<your-redis-host>:<your-redis-port>"
    username: "<your-redis-username>"
    password: "<your-redis-password>"
    db: <your-redis-db>
    key_prefix: "thunderid:"
```

---

## Troubleshooting

**`yaml: unmarshal errors` on startup**
Your `deployment.yaml` contains an unrecognized field. Ensure the config schema matches the image version you are running.

**Frontend still redirects to `localhost` or the wrong port**
Make sure all three files are mounted correctly. A hard refresh (`Ctrl+Shift+R`) may be needed to clear the browser cache.

**CORS errors in the browser**
Ensure your full origin (host + port) is listed in the server-config `cors` section. Add it to `config/resources/server_configs/cors.yaml`, or apply it at runtime with `PUT /server-config/cors`.

**Connection refused on the new port**
Ensure the `ports` mapping in `docker-compose.yml` matches the port set in `deployment.yaml`.
