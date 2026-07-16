/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";
import { watch } from "node:fs";
import { createHash, randomBytes, randomUUID } from "node:crypto";

import { ChatAnthropic } from "@langchain/anthropic";
import { ChatGoogleGenerativeAI } from "@langchain/google-genai";
import { createReactAgent } from "@langchain/langgraph/prebuilt";
import { MultiServerMCPClient } from "@langchain/mcp-adapters";
import type { BaseChatModel } from "@langchain/core/language_models/chat_models";
import { DynamicStructuredTool } from "@langchain/core/tools";
import dotenv from "dotenv";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));

dotenv.config({
  path: resolve(__dirname, ".env"),
});

// Local-dev TLS bypass: Thunder ships with a self-signed cert on localhost,
// so fetch() and the Asgardeo JS SDK would otherwise refuse to talk to it.
// We disable Node's TLS verification ONLY when the configured base URL points
// at localhost / 127.0.0.1 to keep production builds safe.
const __thunderBaseUrl = process.env.THUNDER_BASE_URL || "";
if (
  /^https?:\/\/(localhost|127\.0\.0\.1)(:\d+)?(\/|$)/.test(__thunderBaseUrl)
) {
  process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";
  console.warn(
    "[ai-agent] Local Thunder detected — NODE_TLS_REJECT_UNAUTHORIZED set to 0. Do not use this build in production.",
  );
}

const agentConfig = {
  agentID: process.env.AGENT_ID || "",
  agentSecret: process.env.AGENT_SECRET || "",
};

// Separate CIBA-only credentials for the upgrade scheduler.
// Uses wayfinder-ciba-email-flow; kept separate from the main agent because
// auth_flow_handle is shared between authorization_code and CIBA — they need
// different flows (interactive browser login vs backchannel email notification).
const upgradeAgentConfig = {
  agentID: process.env.UPGRADE_AGENT_ID || "",
  agentSecret: process.env.UPGRADE_AGENT_SECRET || "",
};

let LLM_PROVIDER = (process.env.LLM_PROVIDER || "anthropic").toLowerCase();

function createModel(): BaseChatModel {
  if (LLM_PROVIDER === "gemini" || LLM_PROVIDER === "google") {
    return new ChatGoogleGenerativeAI({
      apiKey: process.env.LLM_API_KEY || "",
      model: process.env.MODEL_NAME || "gemini-2.5-flash",
    });
  }

  const anthropicModel = new ChatAnthropic({
    apiKey: process.env.LLM_API_KEY || "",
    model: process.env.MODEL_NAME || "claude-sonnet-4-6",
  });

  // LangChain @langchain/anthropic 0.3.x has special "send only when explicitly
  // set" handling for sonnet-4-5 / haiku-4-5 / opus-4-1, but not for sonnet-4-6.
  // For models outside that list it sends top_p:-1 and top_k:-1 as defaults,
  // which the Anthropic API rejects ("top_p cannot be set to -1", and
  // "temperature and top_p cannot both be specified"). Mutating these to
  // `undefined` makes JSON.stringify omit them from the request body so only
  // the model's own default temperature (1) is sent.
  (anthropicModel as unknown as { topP: number | undefined }).topP = undefined;
  (anthropicModel as unknown as { topK: number | undefined }).topK = undefined;

  return anthropicModel;
}

let model = createModel();

const ENV_PATH = resolve(__dirname, ".env");

function reloadEnv(): void {
  dotenv.config({ path: ENV_PATH, override: true });

  LLM_PROVIDER = (process.env.LLM_PROVIDER || "anthropic").toLowerCase();
  agentConfig.agentID = process.env.AGENT_ID || "";
  agentConfig.agentSecret = process.env.AGENT_SECRET || "";
  upgradeAgentConfig.agentID = process.env.UPGRADE_AGENT_ID || "";
  upgradeAgentConfig.agentSecret = process.env.UPGRADE_AGENT_SECRET || "";

  model = createModel();

  // Clear cached session agents so they pick up the new model on next request.
  for (const session of sessions.values()) {
    session.agent = undefined;
  }

  // Force MCP reconnection on next request.
  mcpTokenFingerprint = "";

  console.log("[ai-agent] .env reloaded — model and session agents refreshed.");
}

let reloadDebounceTimer: ReturnType<typeof setTimeout> | null = null;

watch(ENV_PATH, () => {
  if (reloadDebounceTimer) clearTimeout(reloadDebounceTimer);
  reloadDebounceTimer = setTimeout(() => {
    reloadDebounceTimer = null;
    reloadEnv();
  }, 300);
});

const SYSTEM_PROMPT = `You are the Wayfinder Concierge, a travel assistant for the Wayfinder Travel app.

You MUST call tools to perform any action. Never simulate, skip, or fabricate tool results.

Output formatting — this is critical. The chat UI is plain text. It renders raw newlines but does NOT render markdown tables, headings, bold, italics, or HTML.

Strict rules:
- Never output markdown tables, pipes ("|"), or column separators.
- Never output markdown headings ("#", "##").
- Never output bold ("**...**") or italics ("*...*").
- One fact per line. Use plain text "Label: Value" lines.
- For multiple items (e.g. a list of flights), separate each item with a BLANK LINE between them so they don't visually merge.
- Use a single short emoji at the very start of a section header line if helpful, never inside data lines.
- Be concise. Skip recaps of the user's question and trailing pleasantries unless asked.
- Lead with the key fact the user needs, then the action question. Never pad with itemized price breakdowns unless the user asks for a cost breakdown. Combine related facts into a single sentence where natural.
- NEVER fabricate a booking confirmation, booking reference, or booking status. To create a booking you MUST call the create_booking tool and use only the values it returns. If the tool returns an error, report it to the user — do not invent a success response.

Response length — concise format example:

Instead of:
  A Business class upgrade is available for your Colombo → Dubai flight, but seats are not open for direct upgrade right now. It will be processed asynchronously when a seat becomes available.

  Here are the pricing details:

  Current fare (Economy): USD 289
  Business class fare: USD 620
  Price difference: USD 331

  Would you like to submit an upgrade request? You'll receive an approval notification on your registered device once it's processed.

Write this:
  You are eligible for a Business class upgrade on your Colombo → Dubai flight (+USD 331), but no seats are available right now.

  Would you like to submit a request to be waitlisted?

The rule: one sentence with the essential fact (eligibility + price delta), then the action question. Omit the fare table unless the user asked for it.

Flights and cabin classes:
- Flights come in two cabin classes: Economy and Business.
- Economy flights are the default lower-cost option.
- Business class flights exist on the same routes as Economy flights — same airline, same schedule, higher price.
- When listing flights, always show the cabin class alongside the price so the user knows what they are looking at.

Example of how to render a list of flights — copy this style exactly. ALWAYS include the flight ID on every flight so the user can refer to it later (e.g. "book flight-cmb-sin-01"):

Available flights from Colombo to Singapore

Flight 1 — Meridian Airways
- ID: flight-cmb-sin-03
- Cabin: Economy
- Departure: 01:10
- Arrival: 09:20
- Duration: 5h 40m
- Stops: 1
- Price: USD 276

Flight 2 — Serendib Air
- ID: flight-cmb-sin-01
- Cabin: Economy
- Departure: 08:45
- Arrival: 15:05
- Duration: 3h 50m
- Stops: 0 (non-stop)
- Price: USD 314

When the user says "book flight 1" or "book the cheapest one" after a listing, look up the flight ID from the previous turn's tool result and call create_booking with that ID — do not ask the user for the ID again.

When create_booking returns a successful result, format it using only the returned data:

Booking WF-76855E8F (confirmed)
- Route: Colombo → Singapore
- Airline: Meridian Airways
- Cabin: Economy
- Departure: 01:10
- Arrival: 09:20
- Duration: 5h 40m
- Stops: 1
- Price: USD 276
- Travelers: 1

Flight upgrades:
- Only Economy bookings can be upgraded. If the user asks to upgrade a Business class booking, tell them it is already in the highest cabin class and cannot be upgraded further.
- When the user asks about upgrading an Economy booking, first call get_flight_bookings to confirm the booking is Economy, then call find_upgrade_options with the bookingId to find the matching Business class option for that exact flight.
- find_upgrade_options returns one of three outcomes:
  1. available: false — no Business class option exists for that flight. Tell the user: "There is no Business class upgrade option for this flight."
  2. available: true, canUpgradeDirectly: true — a Business class seat is immediately available. Show the user the price and price difference, confirm with the user, then call upgrade_booking. If upgrade_booking fails for any reason, immediately fall back to request_upgrade and tell the user: "Direct upgrade is not available right now. Your upgrade request has been submitted and will be processed shortly. You will receive an approval notification on your registered device."
  3. available: true, canUpgradeDirectly: false — the Business class seat is not yet available for direct upgrade. Show the user the price and price difference, confirm with the user, then call request_upgrade with only the bookingId. The upgrade scheduler will find the matching Business class flight and process it when a seat becomes available. Tell the user: "Your upgrade request has been submitted and will be processed shortly. You will receive an approval notification on your registered device."
- Show the Business class price and price difference before confirming any upgrade action.
- NEVER call upgrade_booking or request_upgrade without first showing the price details and getting the user's confirmation.
- Always call find_upgrade_options before any upgrade action — never attempt an upgrade without checking availability first.
- request_upgrade only needs the bookingId — do NOT ask the user for a flight ID when queuing an async upgrade.

Some tools require user authorization. The system handles this automatically at the infrastructure level — always call the tool normally and let the system manage consent.`;

// ---------------------------------------------------------------------------
// On-behalf-of (OBO) configuration
// ---------------------------------------------------------------------------
// When the agent calls a tool that mutates user data (booking, cancellation),
// it cannot use its own client-credentials token — it needs a token that
// represents the signed-in user. The frontend obtains an authorization code
// via a popup and submits it to POST /chat/consent. The agent exchanges it
// at THUNDER_BASE_URL/oauth2/token for a user-context access token.

const THUNDER_BASE_URL = process.env.THUNDER_BASE_URL || "";
const AGENT_REDIRECT_URI =
  process.env.AGENT_REDIRECT_URI || "http://localhost:5173/agent-callback";
const MCP_SERVER_URL =
  process.env.MCP_SERVER_URL || "http://localhost:8787/mcp";
const AGENT_ACCESS_SCOPE = process.env.AGENT_ACCESS_SCOPE || "agent:access";
const FRONTEND_ORIGIN = process.env.FRONTEND_ORIGIN || "http://localhost:5173";
// When "id_token_hint", the scheduler sends the user's OIDC ID token to bc-authorize
// instead of their email. The CIBA flow on the server must set loginHintAttribute: "userID"
// so Thunder resolves the hint as an entity ID (see thunderid-config/thunderid.env).
const CIBA_HINT_TYPE = (process.env.CIBA_HINT_TYPE || "login_hint").toLowerCase();
// Upgrade scheduler is opt-in. Set UPGRADE_SCHEDULER_ENABLED=true to start the
// background loop that polls for pending upgrade requests and processes them via CIBA.
const UPGRADE_SCHEDULER_ENABLED = process.env.UPGRADE_SCHEDULER_ENABLED === "true";

const USER_CONTEXT_TOOLS = new Set<string>([
    "create_booking",
    "delete_all_bookings",
    "get_flight_bookings",
    "get_profile",
    "request_upgrade",
    "upgrade_booking",
]);

const OBO_SCOPES =
  "openid booking:read booking:create " +
  "booking:cancel booking:upgrade";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type TokenState = {
  accessToken: string;
  expiresAt: number;
};

type UserToken = TokenState & { idToken?: string };

type MCPLikeTool = {
  name: string;
  description?: string;
  schema?: unknown;
  invoke: (input: unknown, config?: unknown) => Promise<unknown>;
  func: (
    input: unknown,
    runManager?: unknown,
    config?: unknown,
  ) => Promise<unknown>;
};

interface TokenClaims {
  sub?: string;
  client_id?: string;
  aud?: string | string[];
  scope?: string;
  [key: string]: unknown;
}

type SessionState = {
  id: string;
  agent?: ReturnType<typeof createReactAgent>;
  userToken?: UserToken;
  userToolsByName?: Map<string, MCPLikeTool>;
  pendingConsent?: { verifier: string; state: string; requestId: string };
  consentError?: ConsentRequiredError;
  chatMessages: unknown[];
  lastActivity: number;
};

class ConsentRequiredError extends Error {
  authorizeUrl: string;
  state: string;
  requestId: string;
  scope: string;

  constructor(
    authorizeUrl: string,
    state: string,
    requestId: string,
    scope: string,
  ) {
    super("User consent required");
    this.authorizeUrl = authorizeUrl;
    this.state = state;
    this.requestId = requestId;
    this.scope = scope;
  }
}

// ---------------------------------------------------------------------------
// Token utilities
// ---------------------------------------------------------------------------

function decodeTokenClaims(token: string): TokenClaims | null {
  try {
    const parts = token.split(".");
    return JSON.parse(Buffer.from(parts[1], "base64url").toString());
  } catch {
    return null;
  }
}

function base64UrlEncode(input: Buffer): string {
  return input
    .toString("base64")
    .replace(/=+$/g, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_");
}

function generatePkceVerifier(): string {
  return base64UrlEncode(randomBytes(32));
}

function pkceChallengeFromVerifier(verifier: string): string {
  return base64UrlEncode(createHash("sha256").update(verifier).digest());
}

function buildAuthorizeUrl(
  state: string,
  codeChallenge: string,
  scope: string,
): string {
  const params = new URLSearchParams({
    response_type: "code",
    client_id: agentConfig.agentID,
    redirect_uri: AGENT_REDIRECT_URI,
    scope,
    state,
    code_challenge: codeChallenge,
    code_challenge_method: "S256",
  });

  return `${THUNDER_BASE_URL}/oauth2/authorize?${params.toString()}`;
}

async function exchangeCodeForUserToken(
  code: string,
  codeVerifier: string,
): Promise<UserToken> {
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    code,
    redirect_uri: AGENT_REDIRECT_URI,
    client_id: agentConfig.agentID,
    code_verifier: codeVerifier,
  });

  const basicAuth = Buffer.from(
    `${agentConfig.agentID}:${agentConfig.agentSecret}`,
  ).toString("base64");

  const response = await fetch(`${THUNDER_BASE_URL}/oauth2/token`, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      Authorization: `Basic ${basicAuth}`,
      Accept: "application/json",
    },
    body,
  });

  const text = await response.text();

  if (!response.ok) {
    throw new Error(`Token exchange failed (${response.status}): ${text}`);
  }

    let payload: { access_token?: string; id_token?: string; expires_in?: number };
    try {
        payload = JSON.parse(text);
    } catch {
        throw new Error(`Token exchange returned non-JSON response: ${text}`);
    }

  if (!payload.access_token) {
    throw new Error(`Token exchange response missing access_token: ${text}`);
  }

  try {
    const claims = JSON.parse(
      Buffer.from(payload.access_token.split(".")[1], "base64url").toString(
        "utf8",
      ),
    );
    console.log(
      "[obo] user token claims:",
      JSON.stringify({
        sub: claims.sub,
        act: claims.act,
        authorized_permissions: claims.authorized_permissions,
        scope: claims.scope,
        aud: claims.aud,
      }),
    );
  } catch (decodeErr) {
    console.warn("[obo] failed to decode access token for logging", decodeErr);
  }

    return {
        accessToken: payload.access_token,
        idToken: payload.id_token ?? undefined,
        expiresAt: Date.now() + (payload.expires_in ?? 3600) * 1000,
    };
}

// ---------------------------------------------------------------------------
// CIBA helpers — used by the upgrade scheduler to get per-user tokens
// ---------------------------------------------------------------------------

async function initiateCiba(
    hint: string,
    bindingMessage: string,
): Promise<{ authReqId: string; interval: number; expiresIn: number }> {
    const hintParam = CIBA_HINT_TYPE === "id_token_hint" ? "id_token_hint" : "login_hint";
    const body = new URLSearchParams({
        [hintParam]: hint,
        scope: `openid upgrade:process`,
        binding_message: bindingMessage,
    });

  const basicAuth = Buffer.from(
    `${upgradeAgentConfig.agentID}:${upgradeAgentConfig.agentSecret}`,
  ).toString("base64");

  const response = await fetch(`${THUNDER_BASE_URL}/oauth2/bc-authorize`, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      Authorization: `Basic ${basicAuth}`,
      Accept: "application/json",
    },
    body,
  });

  const text = await response.text();

  if (!response.ok) {
    throw new Error(`CIBA bc-authorize failed (${response.status}): ${text}`);
  }

  let payload: {
    auth_req_id?: string;
    expires_in?: number;
    interval?: number;
  };
  try {
    payload = JSON.parse(text);
  } catch {
    throw new Error(`CIBA bc-authorize returned non-JSON: ${text}`);
  }

  if (!payload.auth_req_id) {
    throw new Error(`CIBA bc-authorize response missing auth_req_id: ${text}`);
  }

  return {
    authReqId: payload.auth_req_id,
    interval: payload.interval ?? 5,
    expiresIn: payload.expires_in ?? 120,
  };
}

type CibaPollResult =
  | { status: "approved"; accessToken: string }
  | { status: "pending" }
  | { status: "slow_down" }
  | { status: "expired" }
  | { status: "denied" }
  | { status: "error"; message: string };

async function pollCibaToken(authReqId: string): Promise<CibaPollResult> {
  const body = new URLSearchParams({
    grant_type: "urn:openid:params:grant-type:ciba",
    auth_req_id: authReqId,
  });

  const basicAuth = Buffer.from(
    `${upgradeAgentConfig.agentID}:${upgradeAgentConfig.agentSecret}`,
  ).toString("base64");

  const response = await fetch(`${THUNDER_BASE_URL}/oauth2/token`, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      Authorization: `Basic ${basicAuth}`,
      Accept: "application/json",
    },
    body,
  });

  const text = await response.text();
  let payload: {
    access_token?: string;
    error?: string;
    error_description?: string;
  };

  try {
    payload = JSON.parse(text);
  } catch {
    return {
      status: "error",
      message: `Non-JSON response from token endpoint: ${text}`,
    };
  }

  if (response.ok && payload.access_token) {
    return { status: "approved", accessToken: payload.access_token };
  }

  const errorCode = payload.error;

  if (errorCode === "authorization_pending") return { status: "pending" };
  if (errorCode === "slow_down") return { status: "slow_down" };
  if (errorCode === "expired_token") return { status: "expired" };
  if (errorCode === "access_denied") return { status: "denied" };

  return {
    status: "error",
    message: payload.error_description ?? errorCode ?? "Unknown CIBA error",
  };
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// ---------------------------------------------------------------------------
// User-context MCP tools (OBO)
// ---------------------------------------------------------------------------

async function getUserContextTool(
  session: SessionState,
  toolName: string,
): Promise<MCPLikeTool> {
  if (!session.userToken) {
    throw new Error("No user token available");
  }

    if (!session.userToolsByName) {
        const headers: Record<string, string> = {
            Authorization: `Bearer ${session.userToken.accessToken}`,
        };
        if (session.userToken.idToken) {
            headers["x-id-token"] = session.userToken.idToken;
        }
        const userClient = new MultiServerMCPClient({
            travel: {
                transport: "http",
                url: MCP_SERVER_URL,
                headers,
            },
        });

    const userTools = (await userClient.getTools()) as unknown as MCPLikeTool[];
    session.userToolsByName = new Map(userTools.map((t) => [t.name, t]));
  }

  const userTool = session.userToolsByName.get(toolName);
  if (!userTool) {
    throw new Error(`User-context tool not found on MCP server: ${toolName}`);
  }

  return userTool;
}

function wrapToolForSession(
  originalTool: DynamicStructuredTool,
  session: SessionState,
): DynamicStructuredTool {
  if (!USER_CONTEXT_TOOLS.has(originalTool.name)) {
    return originalTool;
  }

  if (!THUNDER_BASE_URL) {
    return originalTool;
  }

  return new DynamicStructuredTool({
    name: originalTool.name,
    description: originalTool.description,
    schema: originalTool.schema,
    responseFormat: "content_and_artifact",
    func: async (args: Record<string, unknown>) => {
      if (
        !session.userToken ||
        session.userToken.expiresAt <= Date.now() + 5_000
      ) {
        if (!session.pendingConsent) {
          const scope = OBO_SCOPES;
          const requestId = randomUUID();
          const state = randomUUID();
          const verifier = generatePkceVerifier();
          const challenge = pkceChallengeFromVerifier(verifier);
          const authorizeUrl = buildAuthorizeUrl(state, challenge, scope);

          session.pendingConsent = { verifier, state, requestId };
          session.consentError = new ConsentRequiredError(
            authorizeUrl,
            state,
            requestId,
            scope,
          );
          console.log(
            `[obo] ${originalTool.name} → consent required (scope=${scope})`,
          );
        } else {
          // Re-surface the existing consent error so subsequent /chat responses
          // still carry type: "need_user_consent" for the frontend.
          console.log(
            `[obo] ${originalTool.name} → consent already pending, reusing`,
          );
          if (!session.consentError && session.pendingConsent) {
            const { state, requestId } = session.pendingConsent;
            const scope = OBO_SCOPES;
            const authorizeUrl = buildAuthorizeUrl(
              state,
              pkceChallengeFromVerifier(session.pendingConsent.verifier),
              scope,
            );
            session.consentError = new ConsentRequiredError(
              authorizeUrl,
              state,
              requestId,
              scope,
            );
          }
        }

        return [
          "This action requires user authorization. The system will handle the consent flow.",
          null,
        ];
      }

      console.log(`[obo] ${originalTool.name} → reusing cached user token`);
      const userTool = await getUserContextTool(session, originalTool.name);
      return userTool.func(args);
    },
  });
}

// ---------------------------------------------------------------------------
// Session management
// ---------------------------------------------------------------------------

const sessions = new Map<string, SessionState>();
const SESSION_TTL_MS = 30 * 60 * 1000;

function getOrCreateSession(sessionId?: string): SessionState {
  if (sessionId) {
    const existing = sessions.get(sessionId);
    if (existing) {
      existing.lastActivity = Date.now();
      return existing;
    }
  }

  const id = randomUUID();
  const session: SessionState = {
    id,
    chatMessages: [],
    lastActivity: Date.now(),
  };
  sessions.set(id, session);
  return session;
}

setInterval(
  () => {
    const now = Date.now();
    for (const [id] of sessions) {
      const session = sessions.get(id)!;
      if (now - session.lastActivity > SESSION_TTL_MS) {
        sessions.delete(id);
      }
    }
  },
  5 * 60 * 1000,
);

// ---------------------------------------------------------------------------
// Agent M2M token management — tracks expiry and rebuilds the MCP client
// when the token is refreshed.
// ---------------------------------------------------------------------------

let agentTokenState: TokenState | null = null;
let mcpClient: MultiServerMCPClient | null = null;
let mcpBaseTools: DynamicStructuredTool[] = [];
let mcpTokenFingerprint = "";

async function fetchAgentToken(): Promise<TokenState> {
  const body = new URLSearchParams({
    grant_type: "client_credentials",
    scope:
      "booking:recommend upgrade:search",
  });
  const basicAuth = Buffer.from(
    `${agentConfig.agentID}:${agentConfig.agentSecret}`,
  ).toString("base64");

  const response = await fetch(`${THUNDER_BASE_URL}/oauth2/token`, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      Authorization: `Basic ${basicAuth}`,
      Accept: "application/json",
    },
    body,
  });

  const text = await response.text();
  if (!response.ok) {
    throw new Error(`Agent token request failed (${response.status}): ${text}`);
  }

  let payload: { access_token?: string; expires_in?: number };
  try {
    payload = JSON.parse(text);
  } catch {
    throw new Error(`Agent token endpoint returned non-JSON: ${text}`);
  }
  if (!payload.access_token) {
    throw new Error(`Agent token response missing access_token: ${text}`);
  }

  let expiresAt: number;
  try {
    const claims = JSON.parse(
      Buffer.from(payload.access_token.split(".")[1], "base64url").toString(),
    );
    expiresAt = (claims.exp ?? Math.floor(Date.now() / 1000) + 3600) * 1000;
  } catch {
    expiresAt = Date.now() + (payload.expires_in ?? 3600) * 1000;
  }

  return { accessToken: payload.access_token, expiresAt };
}

async function getOrRefreshAgentToken(): Promise<string> {
  if (agentTokenState && agentTokenState.expiresAt > Date.now() + 30_000) {
    return agentTokenState.accessToken;
  }
  agentTokenState = await fetchAgentToken();
  const ttl = Math.round((agentTokenState.expiresAt - Date.now()) / 1000);
  console.log(`[ai-agent] M2M token acquired (expires in ${ttl}s)`);
  return agentTokenState.accessToken;
}

// ---------------------------------------------------------------------------
// Upgrade agent M2M token — separate from the main agent token.
// Uses upgradeAgentConfig credentials with upgrade:read scope so the
// scheduler can call get_pending_upgrade without the main agent's token.
// ---------------------------------------------------------------------------

let upgradeAgentTokenState: TokenState | null = null;

async function fetchUpgradeAgentToken(): Promise<TokenState> {
  const body = new URLSearchParams({
    grant_type: "client_credentials",
    scope:
      "upgrade:read upgrade:search",
  });
  const basicAuth = Buffer.from(
    `${upgradeAgentConfig.agentID}:${upgradeAgentConfig.agentSecret}`,
  ).toString("base64");

  const response = await fetch(`${THUNDER_BASE_URL}/oauth2/token`, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      Authorization: `Basic ${basicAuth}`,
      Accept: "application/json",
    },
    body,
  });

  const text = await response.text();
  if (!response.ok) {
    throw new Error(
      `Upgrade agent token request failed (${response.status}): ${text}`,
    );
  }

  let payload: { access_token?: string; expires_in?: number };
  try {
    payload = JSON.parse(text);
  } catch {
    throw new Error(`Upgrade agent token endpoint returned non-JSON: ${text}`);
  }
  if (!payload.access_token) {
    throw new Error(
      `Upgrade agent token response missing access_token: ${text}`,
    );
  }

  let expiresAt: number;
  try {
    const claims = JSON.parse(
      Buffer.from(payload.access_token.split(".")[1], "base64url").toString(),
    );
    expiresAt = (claims.exp ?? Math.floor(Date.now() / 1000) + 3600) * 1000;
  } catch {
    expiresAt = Date.now() + (payload.expires_in ?? 3600) * 1000;
  }

  return { accessToken: payload.access_token, expiresAt };
}

async function getOrRefreshUpgradeAgentToken(): Promise<string> {
  if (
    upgradeAgentTokenState &&
    upgradeAgentTokenState.expiresAt > Date.now() + 30_000
  ) {
    return upgradeAgentTokenState.accessToken;
  }
  upgradeAgentTokenState = await fetchUpgradeAgentToken();
  const ttl = Math.round(
    (upgradeAgentTokenState.expiresAt - Date.now()) / 1000,
  );
  console.log(`[upgrade-scheduler] M2M token acquired (expires in ${ttl}s)`);
  return upgradeAgentTokenState.accessToken;
}

async function ensureMcpConnection(): Promise<void> {
  if (!THUNDER_BASE_URL || !agentConfig.agentID || !agentConfig.agentSecret) {
    return;
  }
  const token = await getOrRefreshAgentToken();
  if (token === mcpTokenFingerprint && mcpClient) {
    return;
  }
  if (mcpClient) {
    await mcpClient.close().catch(() => {});
  }
  mcpClient = new MultiServerMCPClient({
    travel: {
      transport: "http",
      url: MCP_SERVER_URL,
      headers: { Authorization: `Bearer ${token}` },
    },
  });
  mcpBaseTools = (await mcpClient.getTools()) as DynamicStructuredTool[];
  mcpTokenFingerprint = token;
  console.log(`[ai-agent] MCP client connected (${mcpBaseTools.length} tools)`);
}

async function createAgent() {
  console.log(
    "##########################################################################################################",
  );
  console.log(
    "##      This is an Agent Authentication Flow sample application for authenticating AI agents            ##",
  );
  console.log(
    "##                         using Asgardeo and LangChain framework                                       ##",
  );
  console.log(
    "##########################################################################################################",
  );
  console.log(`[ai-agent] LLM provider: ${LLM_PROVIDER}`);

  if (THUNDER_BASE_URL && agentConfig.agentID && agentConfig.agentSecret) {
    await ensureMcpConnection();
  } else {
    console.log(
      "[ai-agent] Thunder not configured — running without agent authentication.",
    );
    console.log(
      "[ai-agent] Set THUNDER_BASE_URL, AGENT_ID, and AGENT_SECRET to enable OAuth flows.",
    );
    mcpClient = new MultiServerMCPClient({
      travel: { transport: "http", url: MCP_SERVER_URL },
    });
    mcpBaseTools = (await mcpClient.getTools()) as DynamicStructuredTool[];
  }

  const toolProxies = mcpBaseTools.map((tool) => {
    const toolName = tool.name;
    return new DynamicStructuredTool({
      name: toolName,
      description: tool.description,
      schema: tool.schema,
      responseFormat: "content_and_artifact",
      func: async (args: Record<string, unknown>, _runManager, config) => {
        await ensureMcpConnection();
        const currentTool = mcpBaseTools.find((t) => t.name === toolName);
        if (!currentTool) {
          throw new Error(`Tool ${toolName} not available on MCP server`);
        }
        return currentTool.func(args, _runManager, config);
      },
    });
  });

  return toolProxies;
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

function setCorsHeaders(response: ServerResponse): void {
  response.setHeader("Access-Control-Allow-Origin", FRONTEND_ORIGIN);
  response.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  response.setHeader(
    "Access-Control-Allow-Headers",
    "Content-Type, Authorization",
  );
}

function sendJson(
  response: ServerResponse,
  statusCode: number,
  body: Record<string, unknown>,
): void {
  setCorsHeaders(response);
  response.writeHead(statusCode, { "Content-Type": "application/json" });
  response.end(JSON.stringify(body));
}

async function readJsonBody(request: IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = [];

  for await (const chunk of request) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
  }

  if (chunks.length === 0) {
    return {};
  }

  return JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

// ---------------------------------------------------------------------------
// Session agent — lazily created per session, reused across requests
// ---------------------------------------------------------------------------

function getSessionAgent(
  session: SessionState,
  baseTools: DynamicStructuredTool[],
): ReturnType<typeof createReactAgent> {
  if (!session.agent) {
    const wrapped = baseTools.map((t) => wrapToolForSession(t, session));
    session.agent = createReactAgent({
      llm: model,
      tools: wrapped as Parameters<typeof createReactAgent>[0]["tools"],
      prompt: SYSTEM_PROMPT,
    });
  }
  return session.agent;
}

// ---------------------------------------------------------------------------
// POST /chat — main chat endpoint
// ---------------------------------------------------------------------------

async function handleChat(
  request: IncomingMessage,
  response: ServerResponse,
  baseTools: DynamicStructuredTool[],
): Promise<void> {
  if (THUNDER_BASE_URL && agentConfig.agentID) {
    const authHeader = request.headers.authorization;
    if (!authHeader || !authHeader.startsWith("Bearer ")) {
      sendJson(response, 401, { error: "Missing or invalid token" });
      return;
    }

    const claims = decodeTokenClaims(authHeader.slice(7));
    if (!claims) {
      sendJson(response, 401, { error: "Missing or invalid token" });
      return;
    }

    const scopes =
      typeof claims.scope === "string" ? claims.scope.split(" ") : [];
    const requiredScopes = AGENT_ACCESS_SCOPE.split(" ");
    const missingScopes = requiredScopes.filter(
      (s: string) => !scopes.includes(s),
    );

    if (missingScopes.length > 0) {
      console.log(
        `POST /chat | rejected: missing scope ${missingScopes.join(" ")} | sub: ${claims.sub || "-"}`,
      );
      sendJson(response, 403, {
        error: `You do not have permission to access Wayfinder Concierge — missing required scope: ${missingScopes.join(" ")}`,
      });
      return;
    }

    const aud = Array.isArray(claims.aud)
      ? claims.aud.join(",")
      : claims.aud || "-";
    console.log(
      `POST /chat | sub: ${claims.sub || "-"} | aud: ${aud} | scope: ${claims.scope || "-"}`,
    );
  }

  const body = (await readJsonBody(request)) as {
    message?: string;
    session_id?: string;
  };

  if (
    !body.message ||
    typeof body.message !== "string" ||
    !body.message.trim()
  ) {
    sendJson(response, 400, { error: "message is required" });
    return;
  }

  console.log(
    `CHAT | session: ${body.session_id || "new"} | preview: ${body.message.slice(0, 80)}`,
  );

  const session = getOrCreateSession(body.session_id);
  const agent = getSessionAgent(session, baseTools);

  const newMessage = { role: "user" as const, content: body.message };
  const messagesForInvoke = [...session.chatMessages, newMessage];

  try {
    const result = await agent.invoke({ messages: messagesForInvoke });

    if (session.consentError) {
      const consentError = session.consentError;
      session.consentError = undefined;
      sendJson(response, 200, {
        type: "need_user_consent",
        authorize_url: consentError.authorizeUrl,
        state: consentError.state,
        request_id: consentError.requestId,
        scope: consentError.scope,
        session_id: session.id,
      });
      return;
    }

    session.chatMessages = result.messages;
    const finalMessage = result.messages[result.messages.length - 1];

    sendJson(response, 200, {
      type: "response",
      message:
        typeof finalMessage.content === "string"
          ? finalMessage.content
          : JSON.stringify(finalMessage.content),
      session_id: session.id,
    });
  } catch (error) {
    console.error("Error processing chat:", error);
    sendJson(response, 500, {
      error:
        error instanceof Error ? error.message : "Failed to process message",
      session_id: session.id,
    });
  }
}

// ---------------------------------------------------------------------------
// POST /chat/consent — receives the OAuth authorization code from the frontend
// ---------------------------------------------------------------------------

async function handleConsent(
  request: IncomingMessage,
  response: ServerResponse,
): Promise<void> {
  const body = (await readJsonBody(request)) as {
    session_id?: string;
    request_id?: string;
    code?: string;
    state?: string;
  };

  if (!body.session_id || !body.code) {
    sendJson(response, 400, { error: "session_id and code are required" });
    return;
  }

  const session = sessions.get(body.session_id);
  if (!session) {
    sendJson(response, 400, { error: "Invalid or expired session" });
    return;
  }

  if (!session.pendingConsent) {
    sendJson(response, 400, { error: "No pending consent request" });
    return;
  }

  if (body.state && body.state !== session.pendingConsent.state) {
    sendJson(response, 400, { error: "State mismatch" });
    return;
  }

  try {
    const userToken = await exchangeCodeForUserToken(
      body.code,
      session.pendingConsent.verifier,
    );
    session.userToken = userToken;
    session.userToolsByName = undefined;
    session.pendingConsent = undefined;

    sendJson(response, 200, {
      type: "consent_received",
      session_id: session.id,
    });
  } catch (error) {
    console.error("Token exchange failed:", error);
    sendJson(response, 500, {
      error: error instanceof Error ? error.message : "Token exchange failed",
    });
  }
}

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Upgrade scheduler — runs independently of HTTP requests.
// Picks one pending upgrade request at a time, authenticates the user via CIBA,
// calls process_upgrade with the CIBA user token, then immediately checks for
// the next pending request. Sleeps 30s only when there are no pending requests.
// ---------------------------------------------------------------------------

async function processOneUpgrade(): Promise<boolean> {
  // Use the upgrade agent's own M2M token (upgrade:read scope) to call
  // get_pending_upgrade. This is separate from the main agent's mcpBaseTools which use
  // booking:recommend scope.
  const upgradeM2MToken = await getOrRefreshUpgradeAgentToken();

  const upgradeM2MClient = new MultiServerMCPClient({
    travel: {
      transport: "http",
      url: MCP_SERVER_URL,
      headers: { Authorization: `Bearer ${upgradeM2MToken}` },
    },
  });

  let getPendingTool: DynamicStructuredTool | undefined;
  try {
    const tools =
      (await upgradeM2MClient.getTools()) as DynamicStructuredTool[];
    getPendingTool = tools.find((t) => t.name === "get_pending_upgrade");
  } catch (err) {
    console.error(
      "[upgrade-scheduler] Failed to connect upgrade agent MCP client:",
      err,
    );
    await upgradeM2MClient.close().catch(() => {});
    return false;
  }

  if (!getPendingTool) {
    console.warn(
      "[upgrade-scheduler] get_pending_upgrade tool not available on MCP server",
    );
    await upgradeM2MClient.close().catch(() => {});
    return false;
  }

    let pendingResult: {
        pendingCount: number;
        request: {
            id: string;
            userId: string;
            email: string;
            idToken?: string | null;
            bookingId: string;
            fromFlightId: string;
            toFlightId: string;
            priceDifference: number;
            route: { from: string; to: string; airline: string };
            fromCabin: string;
            toCabin: string;
        } | null;
    } | null = null;

  try {
    const raw = await getPendingTool.func({}, undefined, undefined);
    const text = Array.isArray(raw)
      ? (raw[0] as string)
      : typeof raw === "string"
        ? raw
        : JSON.stringify(raw);
    pendingResult = JSON.parse(
      typeof text === "string" ? text : JSON.stringify(text),
    );
  } catch (err) {
    console.error("[upgrade-scheduler] Failed to fetch pending upgrades:", err);
    await upgradeM2MClient.close().catch(() => {});
    return false;
  } finally {
    await upgradeM2MClient.close().catch(() => {});
  }

  if (!pendingResult || !pendingResult.request) {
    console.log(
      `[upgrade-scheduler] No pending upgrades (count: ${pendingResult?.pendingCount ?? 0})`,
    );
    return false;
  }

    const { request, pendingCount } = pendingResult;
    console.log(
        `[upgrade-scheduler] Found ${pendingCount} pending upgrade(s). Processing: ${request.id} for user: ${request.userId}`,
    );

  const priceDiff =
    request.priceDifference > 0
      ? `+$${request.priceDifference.toFixed(0)}`
      : "no extra cost";
  const bindingMessage = `WF-UPG: Approve ${request.route.from}→${request.route.to} ${request.fromCabin}→${request.toCabin} upgrade (${priceDiff})?`;

    const cibaHint = CIBA_HINT_TYPE === "id_token_hint" ? (request.idToken ?? null) : request.email;

    if (!cibaHint) {
        console.error(`[upgrade-scheduler] No ${CIBA_HINT_TYPE} available for upgrade ${request.id} (user: ${request.userId}) — skipping`);
        return false;
    }

    let authReqId: string;
    let pollIntervalSeconds: number;

    try {
        const cibaResponse = await initiateCiba(cibaHint, bindingMessage);
        authReqId = cibaResponse.authReqId;
        pollIntervalSeconds = cibaResponse.interval;
        console.log(
            `[upgrade-scheduler] CIBA initiated for ${request.userId} | auth_req_id: ${authReqId}`,
        );
    } catch (err) {
        console.error(
            `[upgrade-scheduler] CIBA initiation failed for ${request.userId}:`,
            err,
        );
        return false; // Sleep before retrying to avoid overwhelming Thunder
    }

  // Poll until approved, denied, or expired
  let currentIntervalMs = Math.max(pollIntervalSeconds + 1, 6) * 1000;
  let cibaUserToken: string | null = null;

  for (;;) {
    await sleep(currentIntervalMs);

    let pollResult: CibaPollResult;

    try {
      pollResult = await pollCibaToken(authReqId);
    } catch (err) {
      console.error(
        `[upgrade-scheduler] CIBA poll error for ${request.id}:`,
        err,
      );
      return true;
    }

        if (pollResult.status === "approved") {
            cibaUserToken = pollResult.accessToken;
            console.log(
                `[upgrade-scheduler] CIBA approved for ${request.userId}`,
            );
            break;
        }

    if (pollResult.status === "slow_down") {
      currentIntervalMs += 5000;
      console.log(
        `[upgrade-scheduler] CIBA slow_down — increasing poll interval to ${currentIntervalMs}ms`,
      );
      continue;
    }

    if (pollResult.status === "pending") {
      continue;
    }

        // denied / expired / error
        console.log(
            `[upgrade-scheduler] CIBA ${pollResult.status} for upgrade ${request.id} (user: ${request.userId})`,
        );
        return true;
    }

  if (!cibaUserToken) {
    return true;
  }

  // Call process_upgrade using a one-shot MCP client with the CIBA user token
  try {
    const userMcpClient = new MultiServerMCPClient({
      travel: {
        transport: "http",
        url: MCP_SERVER_URL,
        headers: { Authorization: `Bearer ${cibaUserToken}` },
      },
    });

    const userTools =
      (await userMcpClient.getTools()) as DynamicStructuredTool[];
    const processUpgradeTool = userTools.find(
      (t) => t.name === "process_upgrade",
    );

    if (!processUpgradeTool) {
      console.error(
        `[upgrade-scheduler] process_upgrade tool not found with CIBA token`,
      );
      await userMcpClient.close().catch(() => {});
      return true;
    }

    const result = await processUpgradeTool.func(
      { upgradeRequestId: request.id },
      undefined,
      undefined,
    );
    console.log(
      `[upgrade-scheduler] Upgrade ${request.id} processed successfully:`,
      JSON.stringify(result).slice(0, 200),
    );

    await userMcpClient.close().catch(() => {});
  } catch (err) {
    console.error(
      `[upgrade-scheduler] process_upgrade failed for ${request.id}:`,
      err,
    );
  }

  return true;
}

function startUpgradeScheduler(): void {
    if (!UPGRADE_SCHEDULER_ENABLED) {
        console.log("[upgrade-scheduler] Upgrade scheduler is disabled (set UPGRADE_SCHEDULER_ENABLED=true to enable).");
        return;
    }

    if (!THUNDER_BASE_URL || !agentConfig.agentID || !agentConfig.agentSecret) {
        console.log(
            "[upgrade-scheduler] Thunder not configured — upgrade scheduler disabled.",
        );
        return;
    }

  if (!upgradeAgentConfig.agentID || !upgradeAgentConfig.agentSecret) {
    console.log(
      "[upgrade-scheduler] Upgrade agent not configured (UPGRADE_AGENT_ID / UPGRADE_AGENT_SECRET missing) — upgrade scheduler disabled.",
    );
    return;
  }

  console.log(
    "[upgrade-scheduler] Starting upgrade scheduler (30s sleep when idle).",
  );

  async function loop(): Promise<void> {
    for (;;) {
      try {
        const hadWork = await processOneUpgrade();

        if (!hadWork) {
          // No pending requests — sleep 30s before checking again
          await sleep(30_000);
        }
        // If hadWork is true there may be more — loop immediately
      } catch (err) {
        console.error(
          "[upgrade-scheduler] Unexpected error in scheduler loop:",
          err,
        );
        await sleep(30_000);
      }
    }
  }

  loop().catch((err) =>
    console.error("[upgrade-scheduler] Fatal loop error:", err),
  );
}

async function runAgentServer() {
  const baseTools = await createAgent();
  const port = Number(process.env.PORT || process.env.AGENT_PORT || 8790);
  const host = process.env.HOST || "localhost";

  const server = createServer(async (request, response) => {
    const url = new URL(
      request.url || "",
      `http://${request.headers.host || host}`,
    );

    if (request.method === "OPTIONS") {
      setCorsHeaders(response);
      response.writeHead(204);
      response.end();
      return;
    }

    if (url.pathname === "/health" && request.method === "GET") {
      sendJson(response, 200, { status: "ok" });
      return;
    }

    if (url.pathname === "/chat/access" && request.method === "GET") {
      if (THUNDER_BASE_URL && agentConfig.agentID) {
        const authHeader = request.headers.authorization;
        if (!authHeader || !authHeader.startsWith("Bearer ")) {
          sendJson(response, 401, {
            authorized: false,
            error: "Missing or invalid token",
          });
          return;
        }

        const claims = decodeTokenClaims(authHeader.slice(7));
        if (!claims) {
          sendJson(response, 401, {
            authorized: false,
            error: "Missing or invalid token",
          });
          return;
        }

        const scopes =
          typeof claims.scope === "string" ? claims.scope.split(" ") : [];
        const requiredScopes = AGENT_ACCESS_SCOPE.split(" ");
        const missingScopes = requiredScopes.filter(
          (s: string) => !scopes.includes(s),
        );

        if (missingScopes.length > 0) {
          console.log(
            `GET /chat/access | rejected: missing scope ${missingScopes.join(" ")} | sub: ${claims.sub || "-"}`,
          );
          sendJson(response, 403, {
            authorized: false,
            error: `You are not authorized to use Wayfinder Concierge. Your account does not have the required permission.`,
          });
          return;
        }
      }

      sendJson(response, 200, { authorized: true });
      return;
    }

    if (url.pathname === "/chat" && request.method === "POST") {
      try {
        await handleChat(request, response, baseTools);
      } catch (error) {
        console.error("Unhandled error in /chat:", error);
        if (!response.headersSent) {
          sendJson(response, 500, { error: "Internal server error" });
        }
      }
      return;
    }

        if (url.pathname === "/chat/consent" && request.method === "POST") {
            try {
                await handleConsent(request, response);
            } catch (error) {
                console.error("Unhandled error in /chat/consent:", error);
                if (!response.headersSent) {
                    sendJson(response, 500, { error: "Internal server error" });
                }
            }
            return;
        }

        if (url.pathname === "/api/demo/process-upgrades" && request.method === "POST") {
            processOneUpgrade().catch((err) =>
                console.error("[upgrade-scheduler] Background trigger failed:", err)
            );
            sendJson(response, 202, { message: "Upgrade processing triggered in background. One pending upgrade will be processed." });
            return;
        }

    sendJson(response, 404, { error: "Not found" });
  });

  server.listen(port, host, () => {
    console.log(`AI agent API server is running at http://${host}:${port}`);
    console.log(`Chat endpoint: POST http://${host}:${port}/chat`);
    console.log(`Consent endpoint: POST http://${host}:${port}/chat/consent`);
    console.log(`Health check: GET http://${host}:${port}/health`);
    startUpgradeScheduler();
  });

  const shutdown = async () => {
    console.log("Shutting down AI agent server...");
    server.close();
    if (mcpClient) await mcpClient.close();
    process.exit(0);
  };

  process.on("SIGINT", shutdown);
  process.on("SIGTERM", shutdown);
}

runAgentServer().catch(console.error);
