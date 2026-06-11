# Wayfinder Sample

End-to-end sample of an AI agent that holds its own ThunderID-managed identity.

The agent uses **its own access token (client_credentials grant)** for browsing tools. When a tool needs the user's consent (booking, cancellation, reading the user's own data), it switches to a **user-context token**. That token is obtained via **OAuth 2.0 authorization-code + PKCE**.

The sample is a travel booking app called Wayfinder. A chat widget in the UI talks to a LangChain agent that calls REST tools through an MCP server. The REST API and the MCP server share one Node process and one set of service modules. `/api/*` is the REST surface and `/mcp` is the MCP surface, both enforcing the same `booking:*` scopes against Thunder-issued tokens.

## Architecture

```mermaid
graph LR
    subgraph Frontend["Frontend (React + Vite)"]
        UI["Web App"]
        Chat["Chat Widget"]
    end

    subgraph Backend["Backend Services"]
        Agent["AI Agent<br/>(HTTP API)"]
        Wayfinder["Wayfinder Server<br/>(REST + MCP + SQLite)"]
    end

    Thunder["ThunderID<br/>(Identity Provider)"]

    UI -- "user token<br/>(WAYFINDER app)<br/>→ /api/*" --> Wayfinder
    Chat -- "POST /chat<br/>+ user token<br/>(agent:access)" --> Agent
    Agent -- "M2M token<br/>(client_credentials)<br/>→ /mcp" --> Wayfinder
    Agent -. "OBO token<br/>(authorization_code + PKCE)<br/>→ /mcp" .-> Wayfinder

    Thunder -- "user sign-in<br/>(authorization_code)" --> UI
    Thunder -- "M2M token" --> Agent
    Thunder -. "OBO consent popup" .-> Chat
```text

### Token Flow

The sample uses two OAuth clients and three token types:

| Token | OAuth Client | Grant | Purpose |
|-------|-------------|-------|---------|
| **User token** | `WAYFINDER` | `authorization_code` | Frontend sign-in, API calls, chat API auth (`agent:access` scope) |
| **M2M token** | `WAYFINDER-CONCIERGE` | `client_credentials` | Agent's own identity for browsing tools (search flights, hotels) via MCP |
| **OBO token** | `WAYFINDER-CONCIERGE` | `authorization_code` + PKCE | Implicit on-behalf-of user-context token for mutating tools (booking, cancellation) via MCP. Because the client is an agent, the issued token automatically carries an `act` claim identifying the agent — no separate token exchange needed. |

**How it works:**

1. The user signs in to the Wayfinder web app via the `WAYFINDER` OAuth application. The issued token carries `agent:access` (from the Chat User role).
2. When the user sends a chat message, the frontend calls `POST /chat` on the AI Agent API with the user's access token in the `Authorization` header. The AI Agent validates the token has the `agent:access` scope before processing the message.
3. For browsing tools (search flights, search hotels), the AI Agent uses its own M2M token (obtained via `client_credentials` with the `WAYFINDER-CONCIERGE` credentials) to call the Wayfinder server's `/mcp` endpoint.
4. For mutating tools (create booking, cancel booking), the AI Agent returns a `need_user_consent` response. The frontend opens a consent popup, the user signs in and picks which booking permissions to grant (`booking:read`, `booking:create`, `booking:cancel`), and the authorization code is submitted to `POST /chat/consent`. The agent exchanges it for a user-context token, and the frontend retries the original message. Because `WAYFINDER-CONCIERGE` is a ThunderID *agent*, the issued user-context token automatically includes an `act` claim with the agent's entity ID — an implicit on-behalf-of token. The Wayfinder server logs this delegation (`sub` = the user, `act.sub` = the agent) without any explicit token-exchange step.
5. The Wayfinder server validates the JWT on every request and enforces scopes per route — browsing endpoints/tools are open, booking endpoints/tools require the matching `booking:*` scope. The MCP layer and the REST layer share the same scope guards because they share the same service modules.

## What This Demonstrates

- A ThunderID **agent** acting as an autonomous principal — distinct from a ThunderID user.
- The agent's **machine-to-machine (M2M) token** used for read-only browsing tools (search flights, search hotels, recommend flights, etc.).
- **Scope-based access control** on the AI Agent's HTTP API — only users with `agent:access` can use the chat. Users without this scope (e.g. `jane.smith`) can browse and book through the UI but cannot use the Wayfinder Concierge.
- A **typed user model** — `Customer` user type for consumers, `Staff` user type for internal team — with self sign-up, password recovery, and staff invitation flows backing the B2C use-case story.
- An **on-behalf-of (OBO)** flow triggered from inside a chat session: the agent returns a consent request, the frontend opens a popup where the user picks which booking permissions to grant, and the issued user-context token only carries the approved subset. The token is an **implicit OBO token** — it carries an `act` claim naming the agent that mediated the login, so the resource server sees both the user (`sub`) and the acting agent (`act.sub`) from a single token.
- A REST API that **verifies the JWT** and **enforces scopes per route** (`booking:read`, `booking:create`, `booking:cancel`, `booking:recommend`).
- A **self-service profile page** at `/profile` that calls Thunder's `/users/me` directly with the `WAYFINDER` user token to read account details, edit attributes, and change the password.
- **Multi-LLM support** — the Wayfinder Concierge works with both **Anthropic Claude** and **Google Gemini**, selectable via an environment variable.
- **CIBA-based flight upgrade** — a background upgrade scheduler uses CIBA (Client-Initiated Backchannel Authentication) to authenticate the customer out-of-band via email or SMS notification. The customer approves the upgrade on their own device; the scheduler then processes it with a CIBA-issued token carrying `upgrade:process` scope.

## Project Structure

```text
wayfinder-sample/
├── frontend/          React + Vite UI. Hosts the chat widget and the
│                      /agent-callback route used by the consent popup.
├── backend/           Node server backed by SQLite. Hosts both the REST API
│                      (/api/*) and the MCP server (/mcp), validates JWTs,
│                      enforces scopes per route and per MCP tool.
├── smtp-server/       Sample SMTP server with a web inbox UI.
│                      Captures emails sent by ThunderID flows (recovery,
│                      onboarding, CIBA). No external email relay required.
├── ai-agent/          HTTP Wayfinder Concierge API (LangChain + Claude/Gemini).
├── thunderid-config/  Importable YAML config for ThunderID setup.
└── README.md
```

Each subdirectory has its own README with the environment variables it reads.

## Prerequisites

- Node.js 20+
- A running ThunderID backend on `https://localhost:8090` (self-signed cert is fine).
- **One** of the following LLM API keys:
  - Anthropic API key from [console.anthropic.com](https://console.anthropic.com), **or**
  - Google Gemini API key from [aistudio.google.com](https://aistudio.google.com).

### Allow the frontend origin in Thunder

The Wayfinder web app runs on `http://localhost:5173` and calls Thunder directly for `/oauth2/authorize`, `/oauth2/token`, and `/users/me`. Browsers block these cross-origin calls unless Thunder's CORS allow-list includes the frontend origin.

Edit `backend/cmd/server/repository/conf/deployment.yaml` and add `http://localhost:5173` under `cors.allowed_origins`. Leave any existing entries in place — they belong to other samples.

```yaml
cors:
  allowed_origins:
    # ...existing entries...
    - "http://localhost:5173"
```

Restart the ThunderID server after the change. If you serve the frontend from a different host or port, add that origin instead.

## ThunderID Setup

The `thunderid-config/` directory contains a single importable YAML that creates all required ThunderID resources — resource servers, roles, users, the OAuth application, and the AI agent.

### Import Resources

Import the bundle:

1. Start ThunderID and open the Console.
2. On the **welcome screen** (shown on first login, or accessible from the user profile menu), choose **Open** and upload `thunderid-config/thunderid-config.yaml`. Then for environment variables, upload `thunderid-config/thunderid.env`.

The import creates:

| Resource | Type | What it creates |
|----------|------|-----------------|
| `Customer` | User type | Consumer schema (`username`, `email`, `password`, `given_name`, `family_name`, `mobileNumber`, `sub`) with self-registration enabled |
| `Staff` | User type | Internal team schema (`username`, `email`, `password`, `displayName`) |
| `wayfinder-agent` | Resource server | `agent:access` permission |
| `wayfinder-booking` | Resource server | `booking:read`, `booking:create`, `booking:cancel`, `booking:recommend`, `booking:upgrade`, `upgrade:read`, `upgrade:search`, `upgrade:process` permissions. Protects both `/api/*` (REST) and `/mcp` (MCP tools) on the Wayfinder server. |
| `WAYFINDER` | Application | Public OAuth app (PKCE, redirect to `http://localhost:5173`) with registration and recovery flows enabled |
| `WAYFINDER-CONCIERGE` | Agent | Confidential OAuth client with `client_credentials` + `authorization_code` grants |
| `WAYFINDER-UPGRADE-AGENT` | Agent | CIBA-only confidential client used by the upgrade scheduler to authenticate customers out-of-band |
| `wayfinder-registration-flow` | Flow | Self sign-up flow (REGISTRATION). Assigns the `Traveler` role on completion. |
| `wayfinder-recovery-flow` | Flow | Email-link password recovery flow (RECOVERY) |
| `wayfinder-onboarding-flow` | Flow | Staff onboarding flow (USER_ONBOARDING) with Support/DestinationsAdmin role-selection branches |
| `wayfinder-agent-auth-flow` | Flow | Authentication flow with consent screen (assigned to the AI chat agent) |
| `wayfinder-ciba-email-flow` | Flow | CIBA authentication flow that notifies the user via email (used by the upgrade scheduler by default) |
| `wayfinder-ciba-sms-flow` | Flow | CIBA authentication flow that notifies the user via SMS (switch to this in Console to try the SMS channel) |
| `Traveler` | Role | Booking permissions, assigned to `john.doe` and `jane.smith` |
| `Support` | Role | Staff role for consumer support workflows |
| `DestinationsAdmin` | Role | Staff role for curating featured destinations |
| `OpsAdmin` | Role | Staff role for managing other staff, assigned to `alex.carter` |
| `Wayfinder Chat User` | Role | `agent:access`, `booking:upgrade`, `upgrade:process` permissions, assigned to `john.doe` |
| `Recommender` | Role | `booking:recommend`, `upgrade:search` permissions (assigned to the Wayfinder Concierge) |
| `Upgrade Scheduler` | Role | `upgrade:read`, `upgrade:search` permissions (assigned to the upgrade scheduler agent) |
| `john.doe` / `john.doe` | User | Demo user with Concierge access, booking, and upgrade permissions |
| `jane.smith` / `jane.smith` | User | Demo user with booking permissions but **no** Concierge access |
| `alex.carter` / `alex.carter` | User | Staff with the `OpsAdmin` role for inviting other staff |

The agent's client secret defaults to `wayfinder-agent-secret` (set in `thunderid.env`). Change it in the environment file before importing if you prefer a different value.

### Environment variables in `thunderid.env`

| Variable | Required | Description |
|---|---|---|
| `WAYFINDER_CLIENT_ID` | Yes | Client ID for the `WAYFINDER` application |
| `AGENT_CLIENT_ID` | Yes | Client ID for the `WAYFINDER-CONCIERGE` agent |
| `AGENT_CLIENT_SECRET` | Yes | Client secret for the Wayfinder Concierge |
| `UPGRADE_AGENT_CLIENT_ID` | Yes | Client ID for the `WAYFINDER-UPGRADE-AGENT` |
| `UPGRADE_AGENT_CLIENT_SECRET` | Yes | Client secret for the upgrade scheduler agent |
| `JOHN_DOE_PASSWORD` | Yes | Password for the `john.doe` demo user |
| `JANE_SMITH_PASSWORD` | Yes | Password for the `jane.smith` demo user |
| `ALEX_CARTER_PASSWORD` | Yes | Password for the `alex.carter` demo user |
| `CIBA_SMS_SENDER_ID` | **SMS only** | Notification sender ID for the CIBA SMS flow. Required only if you switch the upgrade agent to `wayfinder-ciba-sms-flow`. Obtain the sender ID after running the SMS setup script (see [CIBA Upgrade — SMS channel](#ciba-upgrade--sms-channel) below). |

### Manual Setup

After the import, update `deployment.yaml` with two additions and restart the server:

- Activate the Wayfinder onboarding flow. ThunderID permits only one `USER_ONBOARDING` flow at a time, selected by handle:

  ```yaml
  flow:
    user_onboarding_flow_handle: "wayfinder-onboarding-flow"
  ```

- Configure SMTP so recovery and invitation emails can be delivered. The sample ships with a built-in local SMTP server (`smtp-server/`) that listens on `127.0.0.1:2525` — no external relay required. The defaults below match its credentials exactly, so no further editing is needed for local development:

  ```yaml
  email:
    smtp:
      host: "127.0.0.1"
      port: 2525
      username: "dev"
      password: "dev"
      from_address: "noreply@thunderid.dev"
      enable_start_tls: false
      enable_authentication: true
  ```

  Once the sample is running, open `http://localhost:8788` to view captured emails in the inbox UI.

## Configure the Sample

`backend/`, `smtp-server/`, `ai-agent/`, and `frontend/` each ship with a `.env.example` listing only the variables you actually need to set. In each of those folders, copy it to `.env` and fill the placeholders. The `smtp-server/.env.example` defaults already match the `email.smtp` settings in `deployment.yaml`, so it works as-is.

The only placeholder you must replace is in `ai-agent/.env`:

- `ANTHROPIC_API_KEY=` (or `GOOGLE_API_KEY=`) — your LLM API key.

The agent secret defaults to `wayfinder-agent-secret` (matching `thunderid.env`). Everything else in the examples is local development defaults that match the run instructions below.

## Run

From the sample root, install all workspace dependencies:

```bash
cd backend     && npm install && npm run seed && npm start   # http://localhost:8787 (REST + /mcp)
cd smtp-server && npm install && npm run dev                 # SMTP :2525 | Inbox http://localhost:8788
cd ai-agent    && npm install && npm start                   # http://localhost:8790/chat
cd frontend    && npm install && npm run dev                 # http://localhost:5173
```

The Wayfinder server hosts both the REST API on `/api/*` and the MCP server on `/mcp`. `npm run seed` initializes the local SQLite database with sample flights, hotels, and trips. Run it once on first setup.

The SMTP server captures all emails sent by ThunderID (password recovery, staff invitations, CIBA upgrade notifications) and displays them at `http://localhost:8788`. It accepts any username/password, matching the `deployment.yaml` defaults (`dev` / `dev`).

## Try It

Open `http://localhost:5173`, sign in as `john.doe` / `john.doe`, open the chat widget, and try:

```
What flights are there from Colombo to Singapore?
```text

These browsing tools use the agent's M2M token — no popup beyond the initial sign-in.

You can also ask the agent for general recommendations — for example:

```
Suggest a few flight deals.
```text

This calls the `recommend_bookings` MCP tool, which serves `GET /api/bookings/recommended` from the same service module with the agent's M2M token. The tool requires the `booking:recommend` scope, which is granted to the `WAYFINDER-CONCIERGE` agent via its **Recommender** role assignment.

Then:

```
Book flight 2
```text

The agent will pause and ask for your permission. A popup opens and you sign in as the demo user via the agent's OAuth application. You pick which booking permissions to grant in the consent screen. The booking then succeeds — or returns 403 if you denied `booking:create`.

### Manage Your Profile

Click your name in the top-right corner and pick **Profile** to view your account details, edit profile attributes, or change your password. The page calls Thunder's `/users/me`, `PUT /users/me`, and `POST /users/me/update-credentials` directly with the `WAYFINDER` user token — no scope beyond a valid user JWT is required.

### No Chat Access

Sign out and sign in as `jane.smith` / `jane.smith`. Jane can browse flights, hotels, and manage bookings through the web UI. Sending a chat message returns a 403 error — her token lacks the `agent:access` scope because she is not assigned the **Chat User** role.

### Flight Upgrade via CIBA

The Wayfinder Concierge supports requesting cabin upgrades. The upgrade is processed asynchronously by a background scheduler that authenticates the customer out-of-band using **CIBA (Client-Initiated Backchannel Authentication)**.

**Before trying this feature**, update `john.doe`'s profile in Thunder Console with a real email address and mobile number — the CIBA notification is delivered to these contacts.

Sign in as `john.doe` and try:

```
What flights are available from Colombo to Singapore?
Book flight-cmb-sin-01 for 1 traveler
I want to upgrade my Colombo to Singapore booking to business class. What are my options?
Upgrade my booking to flight-cmb-sin-01-biz
```text

The agent submits the upgrade request and tells you that a notification will be sent. Sign out — within 30 seconds the upgrade scheduler picks up the request and emails you an approval link. Open the link, enter your password, approve the consent screen (first time only), and the upgrade is processed automatically.

Sign back in and ask:

```
Check my bookings
```text

The booking now shows Business class.

#### CIBA Upgrade — SMS channel

By default the upgrade agent uses `wayfinder-ciba-email-flow`. To switch to SMS notifications:

1. **Set up an SMS sender.** Register a notification sender via the Thunder API:

   ```
   POST https://localhost:8090/notification-senders/message
   ```

   Pass your SMS provider credentials in the request body. Thunder returns the created sender's `id`. Copy that value into `thunderid-config/thunderid.env`:

   ```env
   CIBA_SMS_SENDER_ID=<sender-id-from-api>
   ```

   Re-import `thunderid-config.yaml` so the CIBA SMS flow picks up the new sender ID.

2. **Switch the upgrade agent's flow.** In Thunder Console go to **Applications → WAYFINDER-UPGRADE-AGENT** and change the Authentication Flow to `wayfinder-ciba-sms-flow`. Save. (You may see a brief error on save — this is a known Console issue; the change is persisted. Navigate away and back to confirm.)

3. **Update `john.doe`'s mobile number** in Thunder Console (Customer user type, `mobileNumber` field, E.164 format e.g. `+15550101`).

Now request another upgrade on a different route (e.g. Colombo → Dubai). The notification arrives as an SMS instead of an email. Because you already approved the consent during the email flow, the consent screen is skipped — just enter your password and the upgrade is processed.

> See [CIBA_DEMO_GUIDE.md](CIBA_DEMO_GUIDE.md) for a full end-to-end walkthrough of both channels.
