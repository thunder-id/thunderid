# Wayfinder Travel Backend

Node backend for the Wayfinder Travel app. Hosts the REST API on `/api/*` and the MCP server on `/mcp` in a single process. Verifies Thunder-issued JWTs, enforces scopes per route and per MCP tool, and serves the OAuth protected-resource metadata document for MCP authorization discovery.

Configure with `.env.example` in this folder.

## Run

```bash
npm install
npm run seed
npm start
```

The backend runs on `http://localhost:8787`.

## Endpoints

### REST

| Method | Route                       | Required scope     | Notes                                  |
| ------ | --------------------------- | ------------------ | -------------------------------------- |
| GET    | `/health`                   | —                  |                                        |
| GET    | `/api/flights`              | —                  | `?from=Colombo&to=Singapore`           |
| GET    | `/api/bookings/recommended` | `booking:recommend` | `?limit=3` (1-10, default 3). Random picks. |
| GET    | `/api/hotels`               | —                  | `?location=Singapore`                  |
| GET    | `/api/trips`                | —                  |                                        |
| GET    | `/api/locations`            | —                  | `?category=flights`                    |
| POST   | `/api/bookings`             | `booking:create`   |                                        |
| GET    | `/api/bookings/flights`     | `booking:read`     |                                        |
| DELETE | `/api/bookings/flights`     | `booking:cancel`   |                                        |
| GET    | `/api/me`                   | —                  | Requires a valid JWT but no scope.     |

### MCP

| Method | Route                                       | Notes                                                        |
| ------ | ------------------------------------------- | ------------------------------------------------------------ |
| POST   | `/mcp`                                      | Streamable-HTTP MCP endpoint. Validates JWT and enforces per-tool scopes. Returns 401 with `WWW-Authenticate: Bearer resource_metadata=...` on missing or invalid token. |
| GET    | `/.well-known/oauth-protected-resource`     | RFC 9728 protected-resource metadata document for MCP client discovery. |

#### MCP Tools and Scopes

| Tool                  | Required scope        | Backing endpoint              |
| --------------------- | --------------------- | ----------------------------- |
| `search_flights`      | —                     | `GET /api/flights`            |
| `recommend_bookings`  | `booking:recommend`   | `GET /api/bookings/recommended` |
| `search_hotels`       | —                     | `GET /api/hotels`             |
| `get_trips`           | —                     | `GET /api/trips`              |
| `get_locations`       | —                     | `GET /api/locations`          |
| `get_profile`         | —                     | `GET /api/me`                 |
| `get_flight_bookings` | `booking:read`        | `GET /api/bookings/flights`   |
| `create_booking`      | `booking:create`      | `POST /api/bookings`          |
| `delete_all_bookings` | `booking:cancel`      | `DELETE /api/bookings/flights` |

OpenAPI documentation is available in `openapi.yaml`.
