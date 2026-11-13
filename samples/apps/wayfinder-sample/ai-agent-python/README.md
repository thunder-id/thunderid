# Python Wayfinder Concierge

This sample replaces Wayfinder's TypeScript AI-agent process with an async Python service. It uses the ThunderID Python SDK for machine tokens, interactive authorization, OBO token retrieval, and CIBA approval. FastAPI serves the existing frontend contract. LangChain's `create_agent`, Gemini integration, and native MCP adapter provide the model/tool runtime.

Python 3.11 or later is required. Framework dependency versions are pinned in `pyproject.toml`. The ThunderID SDK is unreleased: install its reviewed local checkout together with this sample rather than installing an unverified public package with the same name.

## Install

Run these commands from this directory:

```bash
export THUNDERID_PYTHON_SDK_PATH=/path/to/python-sdks  # local thunder-id/python-sdks checkout
uv venv .venv
uv pip install --python .venv/bin/python -e "$THUNDERID_PYTHON_SDK_PATH" -e '.[dev]'
cp .env.example .env
```

Fill the environment file for your deployment. Keep populated files private and untracked. Supply credentials through your normal local secret provisioning; never print tokens or paste credentials into shell history.

## Configure

| Setting | Purpose |
| --- | --- |
| `THUNDERID_BASE_URL` | Exact issuer URL of the isolated ThunderID instance. |
| `THUNDERID_CA_FILE` | CA certificate that issued the instance's server certificate, or the server certificate itself when it is self-signed. TLS checks remain enabled. |
| `AGENT_ID`, `AGENT_SECRET` | Confidential Concierge client, with client credentials, authorization code, and refresh grants configured. |
| `MCP_SERVER_URL` | Booking backend's `/mcp` URL and registered booking resource-server identifier. |
| `AGENT_REDIRECT_URI` | Exact registered redirect URI used by the frontend consent popup, ending in `/agent-callback`. |
| `AGENT_CHAT_URL` | This service's `/chat` URL and registered chat resource-server identifier. |
| `FRONTEND_ORIGIN` | Exact allowed browser origin. |
| `AGENT_PORT` or `PORT`, `HOST` | Listener configuration. `AGENT_PORT` takes precedence over `PORT`. The port must match `AGENT_CHAT_URL`. |
| `AGENT_ACCESS_SCOPE` | Required inbound scope, default `agent:access`. |
| `LLM_PROVIDER` | `google`, the supported provider in this sample. |
| `LLM_API_KEY`, `MODEL_NAME` | Gemini Developer API credential and an available model ID. Both are required for chat generation. |
| `UPGRADE_SCHEDULER_ENABLED` | Explicitly enables the guarded CIBA demo processor; default `false`. |
| `UPGRADE_AGENT_ID`, `UPGRADE_AGENT_SECRET` | Separate CIBA client, configured with client credentials and `urn:openid:params:grant-type:ciba`. |
| `CIBA_HINT_TYPE` | `login_hint` or `id_token_hint`, matching the imported CIBA flow. |

Import the Wayfinder redirect bundle into an isolated ThunderID instance. Set the bundle's frontend origins, redirect URIs, booking resource identifier, chat resource identifier, and SMTP connection to the same values used by your stack. Register the Concierge's callback explicitly. Grant its own identity `booking:recommend` and `upgrade:search`; delegated users need the booking permissions for their actions. Configure the separate upgrade client with `upgrade:read` and `upgrade:search` and a working CIBA approval flow.

Set these frontend variables before starting Vite:

```text
VITE_AI_FEATURES_ENABLED=true
VITE_AGENT_CHAT_URL=<same value as AGENT_CHAT_URL>
VITE_API_BASE_URL=<booking backend origin>
VITE_THUNDER_BASE_URL=<same value as THUNDERID_BASE_URL>
```

Use separate service ports when another Wayfinder stack is running. Start services individually; a shared `PORT` in the aggregate Node launcher would affect multiple services. The booking backend must trust the local issuer certificate through `NODE_EXTRA_CA_CERTS`, as described in its README. Do not disable TLS verification in Python or Node.

## Run

```bash
uv run --no-sync --env-file .env wayfinder-agent
```

The process validates SDK configuration and initializes the clients. `/health` reports `model_configured: false` when the Gemini credential or model ID is missing. Chat returns `503 MODEL_NOT_CONFIGURED` in that state; the sample does not simulate model output.

Once configured, sign in to the existing Wayfinder frontend with an account holding `agent:access`. Try a recommendation first. A request to inspect or change the user's bookings starts the OAuth consent popup. The same account must approve that authorization.

| Route | Contract |
| --- | --- |
| `GET /health` | Readiness/configuration flags and a coarse CIBA job status; no credentials or user details. |
| `GET /chat/access` | Requires a verified chat-audience bearer token and the configured access scope. Returns `authorized`. |
| `POST /chat` | Same bearer requirement. Accepts `message` and optional `session_id`. Returns `response` or `need_user_consent` using the existing frontend shapes. |
| `POST /chat/consent` | Accepts `session_id`, `code`, and mandatory `state`; optional `request_id`/`error`. The frontend sends no bearer here. SDK state validation and matching authenticated user bind the response. |
| `POST /api/demo/process-upgrades` | Requires the chat bearer and enabled feature. Starts one background approval attempt. Concurrent triggers return `409`. |

## Authorization Boundaries

- Incoming chat access is verified by the sample's resource API policy: asymmetric JWT signature, exact issuer/chat audience, lifetime, and required scope. This policy is separate from the SDK's client-side operations.
- Every chat belongs to its authenticated subject. Another account cannot reuse its session identifier.
- Machine tools obtain their current token through `get_agent_token`. User tools obtain their current authorization through `get_obo_token`. Native MCP interceptors set the headers for each tool invocation; discovery also obtains a fresh SDK helper result.
- The graph checkpoints before an entire tool batch. When a batch requires consent, none of its tools execute. The frontend's retry resumes that checkpoint without appending the same user message or replaying completed mutations.
- Native LangChain messages stay in the checkpoint history, including Gemini thought-signature metadata. Only the final text reaches the frontend.
- The language model cannot call scheduler tools. The optional CIBA processor uses a separate SDK client, verifies that the pending upgrade belongs to the authenticated caller, waits through the SDK's polling helper, then calls `process_upgrade` with the returned user token. Backend ownership checks remain authoritative.

## Try CIBA Approval

Queue an upgrade through the Concierge, then trigger the guarded demo endpoint with the same user's valid chat bearer. The processor reads the next pending request from the booking MCP server; it does not accept a user identity or hint from the HTTP body. Requests belonging to another user are refused before approval begins.

Approve the request through the configured channel. The job has a 120-second local deadline. `/health` exposes only a coarse status: `running` while a job waits, then `processed`, `upgrade_failed`, `no_pending_upgrade`, `failed`, or `canceled`. A `202` response means the job started, not that the upgrade succeeded. No automatic scheduler or repeated approval attempts run in the background.

## Storage and Limits

This sample explicitly opts into `MemoryStorage` for transient chat and machine contexts. Tokens and checkpoint history are process local and are lost on restart. Sessions expire after 30 minutes of inactivity. Do not use a singleton client to combine unrelated users or turn on tracing that captures credentials, user messages, or booking data.

The sample supports the Wayfinder tool set and Gemini provider only. It does not provide a generic tool/plugin framework, production session persistence, cross-process checkpoints, or automatic remote cancellation of CIBA requests. Calls that fail after an external side effect require application-specific reconciliation; the sample does not automatically replay failed mutations.

## Validate

```bash
uv run --no-sync pytest
uv run --no-sync ruff check .
uv run --no-sync ruff format --check .
```

Offline tests use focused transport, policy, and model doubles. They exercise the real native graph checkpoint/resume machinery without a Gemini service. Live Gemini, browser consent, MCP, and SMTP/CIBA verification require the configured runtime stack and an actual API credential; offline success does not stand in for those checks.

Runtime API references: [LangChain agents](https://docs.langchain.com/oss/python/langchain/agents), [Gemini integration](https://docs.langchain.com/oss/python/integrations/chat/google_generative_ai), and [MCP adapter](https://reference.langchain.com/python/langchain-mcp-adapters/client).
