# Travel MCP Server

Streamable-HTTP MCP server that wraps the Wayfinder Travel REST API and forwards the incoming `Authorization` header unchanged. No configuration required for local development.

## Tools

- `search_flights` — `GET /api/flights`
- `search_hotels` — `GET /api/hotels`
- `get_trips` — `GET /api/trips`
- `get_locations` — `GET /api/locations`
- `create_booking` — `POST /api/bookings` (requires `booking:create`)
- `get_flight_bookings` — `GET /api/bookings/flights` (requires `booking:read`)
- `delete_all_bookings` — `DELETE /api/bookings/flights` (requires `booking:cancel`)
- `get_profile` — `GET /api/me`

Scope enforcement happens at the REST API, not here.

## Run

```bash
npm install
npm start
```

Endpoints:

- MCP:    `http://localhost:8000/mcp`
- Health: `http://localhost:8000/health`
