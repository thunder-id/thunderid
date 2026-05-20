# AI Agent

WebSocket chat agent that authenticates against Thunder with its own identity and switches to a user-context token via authorization-code + PKCE when a tool needs user consent.

See the parent README for end-to-end setup. Configure with `.env.example` in this folder.

## Run

```bash
npm install
npm start
```

Endpoints:

- Chat:   `ws://localhost:8790/chat`
- Health: `http://localhost:8790/health`
