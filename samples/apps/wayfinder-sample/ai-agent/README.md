# AI Agent

WebSocket chat agent that authenticates against Thunder with its own identity and switches to a user-context token via authorization-code + PKCE when a tool needs user consent.

See the parent README for end-to-end setup. Configure with `.env.example` in this folder.

## Run

Install the dependencies once from the sample root with `npm install` (see the parent README), not from this folder. Then start the service from here:

```bash
npm run dev
```

Endpoints:

- Chat:   `ws://localhost:8790/chat`
- Health: `http://localhost:8790/health`
