/*
Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { createServer } from "node:http";
import { createHash, randomBytes, randomUUID } from "node:crypto";
import type { Duplex } from "node:stream";

import { ChatAnthropic } from "@langchain/anthropic";
import { ChatGoogleGenerativeAI } from "@langchain/google-genai";
import type { BaseChatModel } from "@langchain/core/language_models/chat_models";
import { createReactAgent } from "@langchain/langgraph/prebuilt";
import { MultiServerMCPClient } from "@langchain/mcp-adapters";
import dotenv from "dotenv";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));

dotenv.config({
    path: resolve(__dirname, ".env"),
});

// Local-dev TLS bypass: Thunder ships with a self-signed cert on localhost,
// so fetch() and the ThunderID JS SDK would otherwise refuse to talk to it.
// We disable Node's TLS verification ONLY when the configured base URL points
// at localhost / 127.0.0.1 to keep production builds safe.
const __thunderBaseUrl = process.env.THUNDER_BASE_URL || "";
if (/^https?:\/\/(localhost|127\.0\.0\.1)(:\d+)?(\/|$)/.test(__thunderBaseUrl)) {
    process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";
    console.warn("[ai-agent] Local Thunder detected — NODE_TLS_REJECT_UNAUTHORIZED set to 0. Do not use this build in production.");
}

const agentConfig = {
    agentID: process.env.AGENT_ID || "",
    agentSecret: process.env.AGENT_SECRET || "",
};

const MODEL_PROVIDER = (process.env.MODEL_PROVIDER || "anthropic").toLowerCase();

if (MODEL_PROVIDER !== "google" && MODEL_PROVIDER !== "anthropic") {
    throw new Error(`Unsupported MODEL_PROVIDER "${MODEL_PROVIDER}". Must be "google" or "anthropic".`);
}

let model: BaseChatModel;
if (MODEL_PROVIDER === "google") {
    if (!process.env.GEMINI_API_KEY) {
        throw new Error("GEMINI_API_KEY is required when MODEL_PROVIDER=google");
    }
    model = new ChatGoogleGenerativeAI({
        apiKey: process.env.GEMINI_API_KEY,
        model: process.env.MODEL_NAME || "gemini-3-flash-preview",
    });
} else {
    if (!process.env.ANTHROPIC_API_KEY) {
        throw new Error("ANTHROPIC_API_KEY is required when MODEL_PROVIDER=anthropic");
    }
    const anthropicModel = new ChatAnthropic({
        apiKey: process.env.ANTHROPIC_API_KEY,
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
    model = anthropicModel;
}

// System prompt for the chat agent. Kept short and focused on formatting so
// answers render cleanly inside a narrow chat widget. Tables and complex
// markdown render as raw text in the widget today.
const SYSTEM_PROMPT = `You are the Wayfinder Chat Agent, a travel assistant for the Wayfinder Travel app.

Use the available tools to answer travel questions and manage the user's bookings.

Output formatting — this is critical. The chat UI is plain text. It renders raw newlines but does NOT render markdown tables, headings, bold, italics, or HTML.

Strict rules:
- Never output markdown tables, pipes ("|"), or column separators.
- Never output markdown headings ("#", "##").
- Never output bold ("**...**") or italics ("*...*").
- One fact per line. Use plain text "Label: Value" lines.
- For multiple items (e.g. a list of flights), separate each item with a BLANK LINE between them so they don't visually merge.
- Use a single short emoji at the very start of a section header line if helpful, never inside data lines.
- Be concise. Skip recaps of the user's question and trailing pleasantries unless asked.

Example of how to render a list of flights — copy this style exactly. ALWAYS include the flight ID on every flight so the user can refer to it later (e.g. "book flight-cmb-sin-01"):

Available flights from Colombo to Singapore

Flight 1 — Meridian Airways
- ID: flight-cmb-sin-03
- Departure: 01:10
- Arrival: 09:20
- Duration: 5h 40m
- Stops: 1
- Price: USD 276

Flight 2 — Serendib Air
- ID: flight-cmb-sin-01
- Departure: 08:45
- Arrival: 15:05
- Duration: 3h 50m
- Stops: 0 (non-stop)
- Price: USD 314

When the user says "book flight 1" or "book the cheapest one" after a listing, look up the flight ID from the previous turn's tool result and call create_booking with that ID — do not ask the user for the ID again.

Example of a good single booking summary:

Booking WF-76855E8F (confirmed)
- Route: Colombo → Singapore
- Airline: Meridian Airways
- Departure: 01:10
- Arrival: 09:20
- Duration: 5h 40m
- Stops: 1
- Price: USD 276
- Travelers: 1

When you need information you can call the tools. When a tool requires the user's permission to act on their behalf, the system handles that automatically — do not mention it in the chat.`;

// ---------------------------------------------------------------------------
// On-behalf-of (OBO) configuration
// ---------------------------------------------------------------------------
// When the agent calls a tool that mutates user data (booking, cancellation),
// it cannot use its own client-credentials token — it needs a token that
// represents the signed-in user. We obtain one with a standard OAuth 2.0
// authorization-code flow with PKCE, triggered inside the chat session.
//
// The agent sends `need_user_consent` to the chat widget, which opens a popup
// at THUNDER_BASE_URL/oauth2/authorize. After the user signs in, the popup
// posts the auth code back to the agent via the WebSocket, which exchanges it
// at THUNDER_BASE_URL/oauth2/token for a user-context access token.

const THUNDER_BASE_URL = process.env.THUNDER_BASE_URL || "";
const AGENT_REDIRECT_URI = process.env.AGENT_REDIRECT_URI || "http://localhost:5173/agent-callback";
const MCP_SERVER_URL = process.env.MCP_SERVER_URL || "http://localhost:8000/mcp";

// Tools that require a user-context access token. For these, the agent will
// initiate the consent flow if it doesn't already have a user token cached
// for the current chat session.
const USER_CONTEXT_TOOLS = new Set<string>([
    "create_booking",
    "cancel_booking",
    "delete_all_bookings",
    "get_flight_bookings",
    "get_profile",
]);

// Scope to request for each user-context tool. Booking tools share one OBO popup
// requesting all three permissions; the user picks which to grant in the consent
// screen and the issued token carries only the approved subset. Per-route API
// checks (booking:create/read/cancel) then decide whether each tool call succeeds.
const ALL_BOOKING_SCOPES = "booking:read booking:create booking:cancel";
const USER_CONTEXT_SCOPES: Record<string, string> = {
    create_booking: ALL_BOOKING_SCOPES,
    cancel_booking: ALL_BOOKING_SCOPES,
    delete_all_bookings: ALL_BOOKING_SCOPES,
    get_flight_bookings: ALL_BOOKING_SCOPES,
    get_profile: "profile email",
};

const USER_CONSENT_TIMEOUT_MS = 120_000;

type UserToken = {
    accessToken: string;
    expiresAt: number;
};

type PendingConsent = {
    resolve: (data: { type: string; code?: string; state?: string; error?: string; error_description?: string }) => void;
    reject: (err: Error) => void;
    verifier: string;
    state: string;
    timeoutHandle: ReturnType<typeof setTimeout>;
};

type SessionState = {
    socket: Duplex;
    isClosed: boolean;
    userToken?: UserToken;
    userToolsByName?: Map<string, MCPLikeTool>;
    pendingConsents: Map<string, PendingConsent>;
    // When multiple tools are invoked in parallel (common with Gemini), only
    // one consent flow should run at a time. Concurrent callers share this
    // promise so a single popup is shown and a single token is acquired.
    consentInProgress?: Promise<void>;
    // Accumulated conversation history for this WebSocket session. We feed
    // the full thread (including prior tool calls and assistant replies) into
    // each invoke() so the agent remembers what it just said. Without this
    // every turn starts from a blank slate.
    chatMessages: unknown[];
};

type MCPLikeTool = {
    name: string;
    description?: string;
    schema?: unknown;
    invoke: (input: unknown, config?: unknown) => Promise<unknown>;
};

function base64UrlEncode(input: Buffer): string {
    return input.toString("base64").replace(/=+$/g, "").replace(/\+/g, "-").replace(/\//g, "_");
}

function generatePkceVerifier(): string {
    return base64UrlEncode(randomBytes(32));
}

function pkceChallengeFromVerifier(verifier: string): string {
    return base64UrlEncode(createHash("sha256").update(verifier).digest());
}

function buildAuthorizeUrl(state: string, codeChallenge: string, scope: string): string {
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

async function exchangeCodeForUserToken(code: string, codeVerifier: string): Promise<UserToken> {
    const body = new URLSearchParams({
        grant_type: "authorization_code",
        code,
        redirect_uri: AGENT_REDIRECT_URI,
        client_id: agentConfig.agentID,
        code_verifier: codeVerifier,
    });

    const basicAuth = Buffer.from(`${agentConfig.agentID}:${agentConfig.agentSecret}`).toString("base64");

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

    let payload: { access_token?: string; expires_in?: number };
    try {
        payload = JSON.parse(text);
    } catch {
        throw new Error(`Token exchange returned non-JSON response: ${text}`);
    }

    if (!payload.access_token) {
        throw new Error(`Token exchange response missing access_token: ${text}`);
    }

    return {
        accessToken: payload.access_token,
        expiresAt: Date.now() + (payload.expires_in ?? 3600) * 1000,
    };
}

async function requestUserConsent(session: SessionState, scope: string): Promise<void> {
    const requestId = randomUUID();
    const state = randomUUID();
    const verifier = generatePkceVerifier();
    const challenge = pkceChallengeFromVerifier(verifier);
    const authorizeUrl = buildAuthorizeUrl(state, challenge, scope);

    const ok = sendJson(session.socket, {
        type: "need_user_consent",
        authorize_url: authorizeUrl,
        state,
        request_id: requestId,
        scope,
    });

    if (!ok) {
        throw new Error("WebSocket is closed; cannot request user consent");
    }

    const response = await new Promise<{ type: string; code?: string; state?: string; error?: string; error_description?: string }>(
        (resolve, reject) => {
            const timeoutHandle = setTimeout(() => {
                if (session.pendingConsents.delete(requestId)) {
                    reject(new Error("User did not respond to the consent prompt in time."));
                }
            }, USER_CONSENT_TIMEOUT_MS);

            session.pendingConsents.set(requestId, { resolve, reject, verifier, state, timeoutHandle });
        },
    );

    if (response.type === "user_consent_error" || response.error) {
        throw new Error(`User declined consent: ${response.error || "unknown"}`);
    }

    if (response.type !== "user_code" || !response.code) {
        throw new Error("Received an unexpected consent response from the client.");
    }

    if (response.state && response.state !== state) {
        throw new Error("Consent response state did not match. Aborting.");
    }

    const userToken = await exchangeCodeForUserToken(response.code, verifier);
    session.userToken = userToken;
    // Force the user-context MCP tools cache to be rebuilt with the new token.
    session.userToolsByName = undefined;
}

async function getUserContextTool(session: SessionState, toolName: string): Promise<MCPLikeTool> {
    if (!session.userToken) {
        throw new Error("No user token available");
    }

    if (!session.userToolsByName) {
        const userClient = new MultiServerMCPClient({
            travel: {
                transport: "http",
                url: MCP_SERVER_URL,
                headers: {
                    Authorization: `Bearer ${session.userToken.accessToken}`,
                },
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

function wrapToolForSession(originalTool: MCPLikeTool, session: SessionState): MCPLikeTool {
    if (!USER_CONTEXT_TOOLS.has(originalTool.name)) {
        return originalTool;
    }

    const wrapped: MCPLikeTool = Object.create(Object.getPrototypeOf(originalTool) as object | null);
    Object.assign(wrapped, originalTool);

    wrapped.invoke = async (input: unknown, config?: unknown) => {
        if (!session.userToken || session.userToken.expiresAt <= Date.now() + 5_000) {
            if (!session.consentInProgress) {
                const scope = USER_CONTEXT_SCOPES[originalTool.name] || "booking";
                session.consentInProgress = requestUserConsent(session, scope).finally(() => {
                    session.consentInProgress = undefined;
                });
            }
            try {
                await session.consentInProgress;
            } catch (err) {
                console.error(`[obo] ${originalTool.name} → consent failed:`, err);
                throw err;
            }
        }
        const userTool = await getUserContextTool(session, originalTool.name);
        try {
            return await userTool.invoke(input, config);
        } catch (err) {
            console.error(`[tool] ${originalTool.name} invocation failed:`, err);
            throw err;
        }
    };

    return wrapped;
}

type ChatMessage = {
    role: "user" | "assistant" | "system";
    content: string;
};

type ChatRequest = {
    message?: unknown;
    messages?: unknown;
};

type WebSocketFrame = {
    opcode: number;
    payload: Buffer<ArrayBufferLike>;
};

const WEB_SOCKET_GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11";

function parseChatRequest(payload: string): ChatMessage[] {

    try {
        const request = JSON.parse(payload) as ChatRequest;

        if (typeof request.message === "string" && request.message.trim()) {
            return [{ role: "user", content: request.message }];
        }

        if (Array.isArray(request.messages)) {
            const messages = request.messages.filter((message): message is ChatMessage => {
                if (typeof message !== "object" || message === null) {
                    return false;
                }

                const candidate = message as Partial<ChatMessage>;

                return (
                    typeof candidate.content === "string" &&
                    ["user", "assistant", "system"].includes(candidate.role || "")
                );
            });

            if (messages.length > 0) {
                return messages;
            }
        }
    } catch {
        if (payload.trim()) {
            return [{ role: "user", content: payload }];
        }
    }

    throw new Error("Send a non-empty text message or JSON payload with a `message` field.");
}

function getResponseContent(content: unknown): string {
    if (typeof content === "string") {
        return content;
    }

    return JSON.stringify(content);
}

function createWebSocketAcceptKey(key: string): string {
    return createHash("sha1")
        .update(`${key}${WEB_SOCKET_GUID}`)
        .digest("base64");
}

function encodeWebSocketFrame(payload: string, opcode = 0x1): Buffer {
    const payloadBuffer = Buffer.from(payload);
    const payloadLength = payloadBuffer.length;

    if (payloadLength <= 125) {
        return Buffer.concat([
            Buffer.from([0x80 | opcode, payloadLength]),
            payloadBuffer,
        ]);
    }

    if (payloadLength <= 65535) {
        const header = Buffer.alloc(4);
        header[0] = 0x80 | opcode;
        header[1] = 126;
        header.writeUInt16BE(payloadLength, 2);

        return Buffer.concat([header, payloadBuffer]);
    }

    const header = Buffer.alloc(10);
    header[0] = 0x80 | opcode;
    header[1] = 127;
    header.writeBigUInt64BE(BigInt(payloadLength), 2);

    return Buffer.concat([header, payloadBuffer]);
}

function parseWebSocketFrame(
    buffer: Buffer<ArrayBufferLike>
): { frame: WebSocketFrame; remaining: Buffer<ArrayBufferLike> } | null {
    if (buffer.length < 2) {
        return null;
    }

    const opcode = buffer[0] & 0x0f;
    const isMasked = (buffer[1] & 0x80) === 0x80;
    let payloadLength = buffer[1] & 0x7f;
    let offset = 2;

    if (payloadLength === 126) {
        if (buffer.length < offset + 2) {
            return null;
        }

        payloadLength = buffer.readUInt16BE(offset);
        offset += 2;
    } else if (payloadLength === 127) {
        if (buffer.length < offset + 8) {
            return null;
        }

        const extendedPayloadLength = buffer.readBigUInt64BE(offset);

        if (extendedPayloadLength > BigInt(Number.MAX_SAFE_INTEGER)) {
            throw new Error("WebSocket message is too large.");
        }

        payloadLength = Number(extendedPayloadLength);
        offset += 8;
    }

    const maskOffset = offset;

    if (isMasked) {
        offset += 4;
    }

    if (buffer.length < offset + payloadLength) {
        return null;
    }

    const payload = Buffer.from(buffer.subarray(offset, offset + payloadLength));

    if (isMasked) {
        const mask = buffer.subarray(maskOffset, maskOffset + 4);

        for (let index = 0; index < payload.length; index += 1) {
            payload[index] = payload[index] ^ mask[index % 4];
        }
    }

    return {
        frame: { opcode, payload },
        remaining: buffer.subarray(offset + payloadLength),
    };
}

function isSocketWritable(socket: Duplex) {
    return !socket.destroyed && !socket.writableEnded;
}

function writeFrame(socket: Duplex, frame: Buffer) {
    if (!isSocketWritable(socket)) {
        return false;
    }

    try {
        socket.write(frame);

        return true;
    } catch (error) {
        console.warn("Unable to write WebSocket frame:", error instanceof Error ? error.message : error);

        return false;
    }
}

function sendJson(socket: Duplex, payload: Record<string, unknown>) {
    return writeFrame(socket, encodeWebSocketFrame(JSON.stringify(payload)));
}

function closeWebSocket(socket: Duplex) {
    if (isSocketWritable(socket)) {
        try {
            socket.end(encodeWebSocketFrame("", 0x8));
        } catch (err) {
            console.error("[ws] failed to send close frame, destroying socket:", err);
            socket.destroy();
        }
    }
}

async function getAgentTokenViaClientCredentials(): Promise<string> {
    const body = new URLSearchParams({ grant_type: "client_credentials" });
    const basicAuth = Buffer.from(`${agentConfig.agentID}:${agentConfig.agentSecret}`).toString("base64");

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

    let payload: { access_token?: string };
    try {
        payload = JSON.parse(text);
    } catch {
        throw new Error(`Agent token endpoint returned non-JSON: ${text}`);
    }
    if (!payload.access_token) {
        throw new Error(`Agent token response missing access_token: ${text}`);
    }
    return payload.access_token;
}

async function createAgent() {
    console.log("##########################################################################################################");
    console.log("##      This is an Agent Authentication Flow sample application for authenticating AI agents            ##");
    console.log("##                         using ThunderID and LangChain framework                                      ##");
    console.log("##########################################################################################################");

    // Call Thunder's /oauth2/token directly with the client_credentials grant.
    const agentAccessToken = await getAgentTokenViaClientCredentials();

    const client = new MultiServerMCPClient({
        travel: {
            transport: "http",
            url: MCP_SERVER_URL,
            headers: {
                Authorization: `Bearer ${agentAccessToken}`,
            },
        },
    });

    // Base tools use the agent's autonomous token. They are shared across
    // sessions for non-user-context calls. For user-context tools we will
    // wrap each tool per-session so it can trigger the OAuth code flow and
    // route the call through a separate MCP client that uses the user token.
    const baseTools = (await client.getTools()) as unknown as MCPLikeTool[];

    return { baseTools, client };
}

async function runAgentServer() {
    const { baseTools, client } = await createAgent();
    const port = Number(process.env.PORT || process.env.AGENT_PORT || 8790);
    const host = process.env.HOST || "localhost";

    const server = createServer((request, response) => {
        if (request.url === "/health") {
            response.writeHead(200, { "Content-Type": "application/json" });
            response.end(JSON.stringify({ status: "ok" }));

            return;
        }

        response.writeHead(404, { "Content-Type": "application/json" });
        response.end(JSON.stringify({ error: "Not found" }));
    });

    const handleConnection = (socket: Duplex) => {
        const session: SessionState = {
            socket,
            isClosed: false,
            pendingConsents: new Map(),
            chatMessages: [],
        };

        // Lazily-built ReAct agent for this WebSocket session. The agent wraps
        // user-context tools so that they trigger the OAuth code flow + token
        // exchange transparently from inside the agent's tool-call loop.
        let sessionAgent: ReturnType<typeof createReactAgent> | null = null;
        const getSessionAgent = () => {
            if (!sessionAgent) {
                const wrapped = baseTools.map((t) => wrapToolForSession(t, session));
                sessionAgent = createReactAgent({
                    llm: model,
                    tools: wrapped as unknown as Parameters<typeof createReactAgent>[0]["tools"],
                    prompt: SYSTEM_PROMPT,
                });
            }
            return sessionAgent;
        };

        const cleanupPendingConsents = (reason: string) => {
            for (const pending of session.pendingConsents.values()) {
                clearTimeout(pending.timeoutHandle);
                pending.reject(new Error(reason));
            }
            session.pendingConsents.clear();
        };

        socket.on("close", () => {
            session.isClosed = true;
            cleanupPendingConsents("WebSocket closed before consent completed.");
        });

        socket.on("end", () => {
            session.isClosed = true;
            cleanupPendingConsents("WebSocket ended before consent completed.");
        });

        socket.on("error", (error) => {
            session.isClosed = true;
            console.warn("WebSocket client disconnected:", error.message);
            cleanupPendingConsents(`WebSocket error: ${error.message}`);
        });

        sendJson(socket, {
            type: "ready",
            message: "Connected to the ThunderID AI agent.",
        });

        let queue = Promise.resolve();
        let buffer: Buffer<ArrayBufferLike> = Buffer.alloc(0);

        socket.on("data", (data) => {
            buffer = Buffer.concat([buffer, data]);

            try {
                let parsed = parseWebSocketFrame(buffer);

                while (parsed) {
                    buffer = parsed.remaining;

                    if (parsed.frame.opcode === 0x8) {
                        closeWebSocket(socket);

                        return;
                    }

                    if (parsed.frame.opcode === 0x9) {
                        writeFrame(socket, encodeWebSocketFrame(parsed.frame.payload.toString(), 0xA));
                    }

                    if (parsed.frame.opcode === 0x1) {
                        const payload = parsed.frame.payload.toString("utf8");

                        // Intercept consent responses BEFORE the per-message queue,
                        // because the queue may be blocked on the chat message that
                        // requested this consent in the first place.
                        let preParsed: { type?: unknown; request_id?: unknown } | undefined;
                        try {
                            preParsed = JSON.parse(payload) as { type?: unknown; request_id?: unknown };
                        } catch (err) {
                            console.error("[ws] failed to parse incoming message as JSON:", err);
                            preParsed = undefined;
                        }

                        if (preParsed && (preParsed.type === "user_code" || preParsed.type === "user_consent_error")) {
                            const requestId = typeof preParsed.request_id === "string" ? preParsed.request_id : "";
                            const pending = session.pendingConsents.get(requestId);
                            if (pending) {
                                clearTimeout(pending.timeoutHandle);
                                session.pendingConsents.delete(requestId);
                                pending.resolve(preParsed as { type: string; code?: string; state?: string; error?: string });
                            }
                            parsed = parseWebSocketFrame(buffer);
                            continue;
                        }

                        queue = queue.then(async () => {
                            if (session.isClosed) {
                                return;
                            }

                            const newMessages = parseChatRequest(payload);

                            if (!sendJson(socket, { type: "processing" })) {
                                session.isClosed = true;
                                return;
                            }

                            // Append the new user turn to the session's running thread.
                            // The agent.invoke() call receives the FULL history so it can
                            // refer back to its own earlier replies (e.g. resolve "flight 1"
                            // to a flight ID it listed two turns ago).
                            session.chatMessages = [...session.chatMessages, ...newMessages];

                            const agent = getSessionAgent();
                            const result = await agent.invoke({ messages: session.chatMessages });
                            // Persist the updated thread (now includes any new tool calls and
                            // the assistant reply) back to session state.
                            session.chatMessages = result.messages;
                            const finalResponse = result.messages[result.messages.length - 1];

                            if (session.isClosed) {
                                return;
                            }

                            sendJson(socket, {
                                type: "response",
                                message: getResponseContent(finalResponse.content),
                            });
                        }).catch((error: unknown) => {
                            if (session.isClosed) {
                                return;
                            }

                            console.error("Error handling chat message:", error);
                            sendJson(socket, {
                                type: "error",
                                message: error instanceof Error ? error.message : "Failed to process chat message.",
                            });
                        });
                    }

                    parsed = parseWebSocketFrame(buffer);
                }
            } catch (error) {
                console.error("Error parsing WebSocket frame:", error);
                sendJson(socket, {
                    type: "error",
                    message: error instanceof Error ? error.message : "Invalid WebSocket message.",
                });
                closeWebSocket(socket);
            }
        });
    };

    server.on("upgrade", (request, socket, head) => {
        socket.on("error", (error) => {
            console.warn("WebSocket upgrade socket error:", error.message);
        });

        try {
            const url = new URL(request.url || "", `http://${request.headers.host || host}`);
            const key = request.headers["sec-websocket-key"];

            if (url.pathname !== "/chat" || typeof key !== "string") {
                if (!socket.destroyed && !socket.writableEnded) {
                    socket.write("HTTP/1.1 404 Not Found\r\n\r\n");
                }
                socket.destroy();

                return;
            }

            writeFrame(socket, Buffer.from([
                "HTTP/1.1 101 Switching Protocols",
                "Upgrade: websocket",
                "Connection: Upgrade",
                `Sec-WebSocket-Accept: ${createWebSocketAcceptKey(key)}`,
                "",
                "",
            ].join("\r\n")));

            if (head.length > 0) {
                socket.unshift(head);
            }

            handleConnection(socket);
        } catch (error) {
            console.error("Error upgrading WebSocket connection:", error);
            socket.destroy();
        }
    });

    server.listen(port, host, () => {
        console.log(`AI agent WebSocket server is running at ws://${host}:${port}/chat`);
        console.log(`Health check is available at http://${host}:${port}/health`);
    });

    const shutdown = async () => {
        console.log("Shutting down AI agent server...");
        server.close();
        await client.close();
        process.exit(0);
    };

    process.on("SIGINT", shutdown);
    process.on("SIGTERM", shutdown);
}

runAgentServer().catch(console.error);
