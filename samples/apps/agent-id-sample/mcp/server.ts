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

import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { z } from "zod";

const apiBaseUrl = process.env.API_BASE_URL || "http://localhost:8787";
const port = Number(process.env.PORT || process.env.MCP_PORT || 8000);
const host = process.env.HOST || "localhost";

type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };

function getAuthorizationHeader(request: IncomingMessage): string | undefined {
    const authorization = request.headers.authorization;

    return Array.isArray(authorization) ? authorization[0] : authorization;
}

function createApiClient(authorization?: string) {
    async function requestApi(path: string, options: RequestInit = {}): Promise<JsonValue> {
        const headers = new Headers(options.headers);

        headers.set("Accept", "application/json");

        if (options.body && !headers.has("Content-Type")) {
            headers.set("Content-Type", "application/json");
        }

        if (authorization) {
            headers.set("Authorization", authorization);
        }

        const response = await fetch(`${apiBaseUrl}${path}`, {
            ...options,
            headers,
        });

        const contentType = response.headers.get("content-type") || "";
        const body = contentType.includes("application/json")
            ? await response.json()
            : await response.text();

        if (!response.ok) {
            throw new Error(`API request failed with ${response.status}: ${JSON.stringify(body)}`);
        }

        return body as JsonValue;
    }

    return {
        get: (path: string) => requestApi(path),
        post: (path: string, body: JsonValue) => requestApi(path, {
            method: "POST",
            body: JSON.stringify(body),
        }),
        delete: (path: string) => requestApi(path, { method: "DELETE" }),
    };
}

function toToolContent(data: JsonValue) {
    return {
        content: [
            {
                type: "text" as const,
                text: typeof data === "string" ? data : JSON.stringify(data, null, 2),
            },
        ],
    };
}

function createTravelMcpServer(authorization?: string) {
    const api = createApiClient(authorization);
    const server = new McpServer({
        name: "wayfinder-travel-api",
        version: "1.0.0",
    });

    server.tool(
        "search_flights",
        "Search available flights from the travel API.",
        {
            from: z.string().optional().describe("Departure location, for example Colombo."),
            to: z.string().optional().describe("Arrival location, for example Singapore."),
        },
        async ({ from, to }) => {
            const params = new URLSearchParams();

            if (from) {
                params.set("from", from);
            }

            if (to) {
                params.set("to", to);
            }

            const query = params.toString();

            return toToolContent(await api.get(`/api/flights${query ? `?${query}` : ""}`));
        },
    );

    server.tool(
        "search_hotels",
        "Search available hotels from the travel API.",
        {
            location: z.string().optional().describe("Hotel location, for example Singapore."),
        },
        async ({ location }) => {
            const params = new URLSearchParams();

            if (location) {
                params.set("location", location);
            }

            const query = params.toString();

            return toToolContent(await api.get(`/api/hotels${query ? `?${query}` : ""}`));
        },
    );

    server.tool(
        "get_trips",
        "Get saved trip ideas from the travel API.",
        {},
        async () => toToolContent(await api.get("/api/trips")),
    );

    server.tool(
        "get_locations",
        "Get available travel locations from the travel API.",
        {
            category: z.enum(["flights", "hotels", "trips"]).optional().describe("Optional location category."),
        },
        async ({ category }) => {
            const query = category ? `?${new URLSearchParams({ category }).toString()}` : "";

            return toToolContent(await api.get(`/api/locations${query}`));
        },
    );

    server.tool(
        "create_booking",
        "Create a sample booking in the travel API.",
        {
            type: z.enum(["flight", "hotel", "trip"]).describe("Booking type."),
            itemId: z.string().describe("Flight or hotel item ID to book."),
            travelers: z.number().int().optional().describe("Number of travelers."),
        },
        async ({ type, itemId, travelers }) => toToolContent(await api.post("/api/bookings", {
            type,
            itemId,
            travelers: travelers ?? 1,
        })),
    );

    server.tool(
        "get_flight_bookings",
        "Get flight bookings for the current authenticated user.",
        {},
        async () => toToolContent(await api.get("/api/bookings/flights")),
    );

    server.tool(
        "delete_all_bookings",
        "Delete ALL flight bookings for the current authenticated user. Use this to reset the user's bookings (e.g. when the user explicitly says 'clear all my bookings' or 'reset my bookings'). Destructive — only call when explicitly requested.",
        {},
        async () => toToolContent(await api.delete("/api/bookings/flights")),
    );

    server.tool(
        "get_profile",
        "Get the current authenticated user's profile from the travel API.",
        {},
        async () => toToolContent(await api.get("/api/me")),
    );

    return server;
}

async function readJsonBody(request: IncomingMessage): Promise<unknown> {
    const chunks: Buffer[] = [];

    for await (const chunk of request) {
        chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
    }

    if (chunks.length === 0) {
        return undefined;
    }

    const body = Buffer.concat(chunks).toString("utf8");

    return body ? JSON.parse(body) : undefined;
}

function sendJson(response: ServerResponse, statusCode: number, body: JsonValue) {
    response.writeHead(statusCode, { "Content-Type": "application/json" });
    response.end(JSON.stringify(body));
}

const httpServer = createServer(async (request, response) => {
    if (request.url === "/health") {
        sendJson(response, 200, { status: "ok" });

        return;
    }

    if (request.url !== "/mcp") {
        sendJson(response, 404, { error: "Not found" });

        return;
    }

    if (request.method !== "POST") {
        sendJson(response, 405, { error: "Method not allowed" });

        return;
    }

    let body: unknown;
    try {
        body = await readJsonBody(request);
    } catch (error) {
        sendJson(response, 400, {
            error: error instanceof Error ? `Malformed JSON body: ${error.message}` : "Malformed JSON body",
        });
        return;
    }

    try {
        const server = createTravelMcpServer(getAuthorizationHeader(request));
        const transport = new StreamableHTTPServerTransport({
            sessionIdGenerator: undefined,
        });

        response.on("close", () => {
            transport.close();
        });

        await server.connect(transport);
        await transport.handleRequest(request, response, body);
    } catch (error) {
        console.error("Error handling MCP request:", error);

        if (!response.headersSent) {
            sendJson(response, 500, {
                error: error instanceof Error ? error.message : "Failed to handle MCP request.",
            });
        }
    }
});

httpServer.listen(port, host, () => {
    console.log(`Travel MCP server is running at http://${host}:${port}/mcp`);
    console.log(`Health check is available at http://${host}:${port}/health`);
});
