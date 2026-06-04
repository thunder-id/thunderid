# Wayfinder Travel (Frontend)

Vite + React UI for the agent identity sample. Two things matter here:

1. **User sign-in** via the Asgardeo JavaScript SDK pointed at Thunder. Uses Thunder's `WAYFINDER` application (a separate OAuth client from the chat agent).
2. **Chat widget** that talks to the agent over WebSocket. The widget also hosts the `/agent-callback` route that captures the auth code from the OBO popup and forwards it back to the agent.

Configure with `.env.example` in this folder.

## Auth Mode Config

Two environment variables in `.env` control which auth mode is active:

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_AUTH_IS_REDIRECT_BASED` | `true` | `true` — standard OAuth2 redirect to ThunderID Login Gate. `false` — embedded app-native flow via `/flow/execute`. |
| `VITE_AUTH_IS_VERBOSE` | `false` | Only applies when `VITE_AUTH_IS_REDIRECT_BASED=false`. `true` — SDK-driven embedded flow (component trees). `false` — step-by-step embedded flow (raw `inputs[]`/`actions[]`). |

These values can also be overridden at startup via `start.sh` flags (`--redirect-based`, `--verbose`) without editing `.env`. See the top-level `README.md` for details.

The two auth mode families require different ThunderID resource bundles — import from `thunderid-config/redirect/` for redirect mode or `thunderid-config/app-native/` for app-native modes.

## Run

```bash
npm install
npm run dev
```

The app opens on `http://localhost:5173/`.

## Routes

| Route              | Purpose                                                                 |
| ------------------ | ----------------------------------------------------------------------- |
| `/`                | Travel UI + chat widget.                                                |
| `/flights`         | Public landing page (flights). Search panel is visible by default.      |
| `/hotels`          | Hotels landing page.                                                    |
| `/trips`           | Trip-ideas landing page.                                                |
| `/results`         | Flight search results.                                                  |
| `/bookings`        | Signed-in user's flight bookings.                                       |
| `/profile`         | Signed-in user's profile — account details, attribute edits, password change. Calls Thunder's `/users/me` directly. |
| `/signin`          | Sign-in page (app-native modes only — redirect mode uses the Login Gate). |
| `/signup`          | Sign-up page (app-native modes only).                                   |
| `/recovery`        | Password recovery page (app-native modes only — recovery email links land here). |
| `/agent-callback`  | Lands the OBO auth code and posts it back to the chat widget.           |
| `/signin-as-agent` | Deep-link entry that triggers an agent (M2M) sign-in flow.              |

## Build

```bash
npm run build
npm run preview
```
