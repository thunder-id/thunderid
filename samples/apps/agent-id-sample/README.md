# Agent Identity Sample (Wayfinder)

End-to-end sample of an AI agent that holds its own ThunderID-managed identity.

The agent uses **its own access token (client_credentials grant)** for browsing tools and switches to a **user-context token via OAuth 2.0 authorization-code + PKCE** when a tool needs the user's consent (booking, cancellation, viewing the user's own data).

The sample is a travel booking app called Wayfinder. A chat widget in the UI talks to a LangChain agent that calls REST tools through an MCP server.

## What This Demonstrates

- A ThunderID **agent** acting as an autonomous principal — distinct from a ThunderID user.
- The agent's **machine-to-machine (M2M) token** used for read-only browsing tools (search flights, search hotels, etc.).
- An **on-behalf-of (OBO)** flow triggered from inside a chat session: the agent asks for the user's consent in a popup, the user picks which booking permissions to grant, and the issued user-context token only carries the approved subset.
- A REST API that **verifies the JWT** and **enforces scopes per route** (`booking:read`, `booking:create`, `booking:cancel`).
- An in-app **Agent Portal** page (`/agent-portal`) where a signed-in admin can list, create, and delete agents and assign ThunderID roles to them — calling ThunderID's admin APIs directly from the browser with the user's `system`-scoped access token.

## Project Structure

```text
agent-id-sample/
├── frontend/   React + Vite UI. Hosts the chat widget and the
│               /agent-callback route used by the consent popup.
├── api/        Node REST API backed by SQLite. Validates JWTs
│               and enforces scopes on booking routes.
├── mcp/        Streamable-HTTP MCP server that wraps the REST API.
├── ai-agent/   WebSocket chat agent (LangChain + Claude).
└── README.md
```

Each subdirectory has its own README with the environment variables it reads and a `npm start` command.

## Prerequisites

- Node.js 22+
- A running ThunderID backend on `https://localhost:8090` (self-signed cert is fine).
- An Anthropic API key from [console.anthropic.com](https://console.anthropic.com) **or** a Google Gemini API key from [aistudio.google.com](https://aistudio.google.com/app/apikey).

## ThunderID Setup

Before running the sample, create the following entities in ThunderID. You can use the Console UI or the management APIs.

### 1. Resource Server With Permissions

Create a resource server **`booking-api`** (identifier `booking-api`) with a resource handle `booking` and three actions:

| Action handle | Resulting permission |
| ------------- | -------------------- |
| `read`        | `booking:read`       |
| `create`      | `booking:create`     |
| `cancel`      | `booking:cancel`     |

### 2. Demo User

Create a user (e.g. username `john.doe`, password `john.doe`) under the default OU. This will be the resource owner who consents in the popup.

### 3. Roles Granting the Demo User Permissions

Create a role (e.g. **Wayfinder Admin**) that grants the three `booking:read`, `booking:create`, `booking:cancel` permissions on the `booking-api` resource server. Assign the role to the demo user.

Also assign the demo user the built-in **Administrator** role (or any role that grants the `system` permission on the `system` resource server). This is what lets the in-app Agent Portal page call ThunderID's admin APIs. Without it, sign-in still works but the Agent Portal will see authorization errors when listing or creating agents.

### 4. Wayfinder Web Application

Create an OAuth application **`WAYFINDER`** with redirect URI `http://localhost:5173`. Assign it an authentication flow that runs the **AuthorizationExecutor** so the user's role permissions are evaluated into the access token (the same flow shape as the agent flow below works). This is what the frontend uses for user sign-in.

### 5. Agent and Agent Authentication Flow

Create an **agent authentication flow** with the following node sequence and register it for the agent below:

```
start → prompt_credentials → basic_auth → authorization_check → auth_assert → end
```

The `authorization_check` node uses the **AuthorizationExecutor**; the rest are standard.

Create an **agent** named **`WAYFINDER-CHAT-AGENT`** with:

- Redirect URI: `http://localhost:5173/agent-callback`
- Allowed grants: `authorization_code` (for OBO) and `client_credentials` (for M2M)
- `accessToken.userAttributes`: `given_name`, `family_name`, `email`, `groups`
- The agent authentication flow created above

ThunderID prints the agent's **client secret only once** at creation. Capture it for `ai-agent/.env`.

## Configure the Sample

`api/`, `ai-agent/`, and `frontend/` each ship with a `.env.example` listing only the variables you actually need to set. In each of those folders, copy it to `.env` and fill the placeholders. The `mcp/` server has no required configuration.

The placeholder values you must replace are in `ai-agent/.env`:

- `AGENT_SECRET=` — the agent client secret captured at agent creation.
- `ANTHROPIC_API_KEY=` — your Anthropic API key (when using the default `MODEL_PROVIDER=anthropic`).

To use Gemini instead, set:

- `MODEL_PROVIDER=google`
- `GEMINI_API_KEY=` — your Google Gemini API key.
- `MODEL_NAME=` — optionally override the model (default: `gemini-3-flash-preview`).

Everything else in the examples is local development defaults that match the run instructions below.

## Run

In four terminals:

```bash
cd api      && npm install && npm run seed && npm start   # http://localhost:8787
cd mcp      && npm install && npm start                   # http://localhost:8000/mcp
cd ai-agent && npm install && npm start                   # ws://localhost:8790/chat
cd frontend && npm install && npm run dev                 # http://localhost:5173
```

`npm run seed` initializes the local SQLite database with sample flights, hotels, and trips. Run it once on first setup.

## Try It

Open `http://localhost:5173`, open the chat widget, and try:

```
What flights are there from Colombo to Singapore?
```

These browsing tools use the agent's M2M token — no popup, no user sign-in.

Then:

```
Book flight 2
```

The agent will pause and ask for your permission. A popup opens, you sign in as the demo user, you pick which booking permissions to grant in the consent screen, and the booking succeeds (or returns 403 if you denied `booking:create`).

To exercise the **Agent Portal**, sign in to the Wayfinder web app itself (using the same demo user, who now has the `system` permission) and open the account menu → **Agent Portal**. From there you can create more agents, delete them, and assign roles to them — all calling ThunderID's admin APIs with the user's access token.
