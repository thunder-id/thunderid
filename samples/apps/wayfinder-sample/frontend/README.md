# Wayfinder Travel (Frontend)

Vite + React UI for the agent identity sample. Two things matter here:

1. **User sign-in** via the ThunderID JavaScript SDK. Uses the `WAYFINDER` application (a separate OAuth client from the chat agent).
2. **Chat widget** that talks to the agent over WebSocket. The widget also hosts the `/agent-callback` route that captures the auth code from the OBO popup and forwards it back to the agent.

Configure with `.env.example` in this folder.

## Run

Install the dependencies once from the sample root with `npm install` (see the parent README), not from this folder. Then start the service from here:

```bash
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
| `/agent-callback`  | Lands the OBO auth code and posts it back to the chat widget.           |
| `/signin-as-agent` | Deep-link entry that triggers an agent (M2M) sign-in flow.              |

## Build

```bash
npm run build
npm run preview
```
