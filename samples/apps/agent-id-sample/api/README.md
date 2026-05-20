# Wayfinder Travel API

REST API for the Wayfinder Travel app. Verifies Thunder-issued JWTs and enforces scopes per route.

Configure with `.env.example` in this folder.

## Run

```bash
npm install
npm run seed
npm start
```

The API runs on `http://localhost:8787`.

## Endpoints

| Method | Route                       | Required scope     | Notes                                  |
| ------ | --------------------------- | ------------------ | -------------------------------------- |
| GET    | `/health`                   | —                  |                                        |
| GET    | `/api/flights`              | —                  | `?from=Colombo&to=Singapore`           |
| GET    | `/api/hotels`               | —                  | `?location=Singapore`                  |
| GET    | `/api/trips`                | —                  |                                        |
| GET    | `/api/locations`            | —                  | `?category=flights`                    |
| POST   | `/api/bookings`             | `booking:create`   |                                        |
| GET    | `/api/bookings/flights`     | `booking:read`     |                                        |
| DELETE | `/api/bookings/flights`     | `booking:cancel`   |                                        |
| GET    | `/api/me`                   | —                  | Requires a valid JWT but no scope.     |

OpenAPI documentation is available in `openapi.yaml`.
