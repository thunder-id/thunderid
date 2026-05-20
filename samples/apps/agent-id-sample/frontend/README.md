# Wayfinder Travel (Frontend)

Vite + React UI for the agent identity sample. Two things matter here:

1. **User sign-in** via the Asgardeo JavaScript SDK pointed at Thunder. Uses Thunder's `WAYFINDER` application (a separate OAuth client from the chat agent).
2. **Chat widget** that talks to the agent over WebSocket. The widget also hosts the `/agent-callback` route that captures the auth code from the OBO popup and forwards it back to the agent.

Configure with `.env.example` in this folder.

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
| `/agent-callback`  | Lands the OBO auth code and posts it back to the chat widget.           |

## Build

```bash
npm run build
npm run preview
```
