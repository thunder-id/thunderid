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

import { randomUUID } from "node:crypto";

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { z } from "zod";

import { resolveUser, validateIdToken } from "./auth.js";
import {
    createAuthzenAuthorizer,
    getAuthorizationMode,
} from "./authzen.js";
import {
  createBookingRecord,
  createUpgradeRequest,
  deleteBookingsForUser,
  findDuplicateBooking,
  findFlightById,
  findFlights,
  findHotels,
  findMatchingBusinessFlight,
  findRecommendedFlights,
  getBookingById,
  getOnePendingUpgrade,
  getUpgradeRequestById,
  listBookedFlights,
  listLocations,
  listTrips,
  updateBookingFlight,
  updateUpgradeStatus
} from "./db.js";

// Per-tool permission requirements. Scope mode checks these values in the
// caller token; AuthZEN mode sends them to the PDP as action names. Tools
// mapped to null require only a valid token.
const TOOL_PERMISSIONS = {
  search_flights: null,
  recommend_bookings: "booking:recommend",
  search_hotels: null,
  get_trips: null,
  get_locations: null,
  get_profile: null,
  get_flight_bookings: "booking:read",
  create_booking: "booking:create",
  delete_all_bookings: "booking:cancel",
  find_upgrade_options: "upgrade:search",
  upgrade_booking: "booking:upgrade",   // direct immediate upgrade (seats available)
  request_upgrade: "booking:upgrade",   // async CIBA upgrade (seats not yet available)
  get_pending_upgrade: "upgrade:read",
  process_upgrade: "upgrade:process"
};

let evaluateAuthzenAccess;

function getAuthzenAuthorizer() {
    if (!evaluateAuthzenAccess) {
        evaluateAuthzenAccess = createAuthzenAuthorizer();
    }

    return evaluateAuthzenAccess;
}

function generateBookingReference() {
    return `WF-${randomUUID().replace(/-/g, "").slice(0, 8).toUpperCase()}`;
}

function toToolContent(data) {
    return {
        content: [
            {
                type: "text",
                text:
                    typeof data === "string"
                        ? data
                        : JSON.stringify(data, null, 2),
            },
        ],
    };
}

async function requireToolAuthorization(user, toolName) {
    if (!Object.hasOwn(TOOL_PERMISSIONS, toolName)) {
        throw new Error(`Missing permission mapping for tool ${toolName}`);
    }

    const requiredPermission = TOOL_PERMISSIONS[toolName];

    if (requiredPermission === null) {
        return;
    }

    if (getAuthorizationMode() === "authzen") {
        await getAuthzenAuthorizer()({
            subject: {
                id: user.id,
            },
            resource: {
                type: "http://localhost:8787/mcp",
            },
            action: {
                name: requiredPermission,
            },
        });

        return;
    }

    const scopes = user?.scopes || [];

    if (!scopes.includes(requiredPermission)) {
        const error = new Error(
            `Insufficient scope for tool ${toolName}. Required: ${requiredPermission}`,
        );
        error.code = "insufficient_scope";
        error.requiredScope = requiredPermission;
        throw error;
    }
}

function callerTag(user) {
    if (!user) {
        return "anon";
    }

    const type =
        user.rawClaims?.grant_type === "client_credentials" ? "m2m" : "user";
    const base = `${type}:${user.id || "-"}`;
    const actor = user.rawClaims?.act?.sub;

    return actor ? `${base} (on-behalf-of agent ${actor})` : base;
}

function logMcpToolCall(body, user) {
    if (!body || typeof body !== "object") {
        return;
    }

    const method = body.method;

    if (method !== "tools/call") {
        return;
    }

    const toolName = body.params?.name || "-";
    const args = body.params?.arguments;
    const scope = user?.scopes?.join(" ") || "-";

    console.log(
        `  → TOOL ${toolName} | caller: ${callerTag(user)} | scope: ${scope} | args: ${JSON.stringify(args || {})}`,
    );
}

function createTravelMcpServer(user, idToken) {
    const server = new McpServer({
        name: "wayfinder-travel-api",
        version: "1.0.0",
    });

    // Enforce the configured authorization mode at the MCP layer rather than
    // relying on the REST API downstream.
    function tool(name, description, schema, handler) {
        server.tool(name, description, schema, async (args) => {
            await requireToolAuthorization(user, name);

            return handler(args);
        });
    }

    tool(
        "search_flights",
        "Search available flights from the travel API.",
        {
            from: z
                .string()
                .optional()
                .describe("Departure location, for example Colombo."),
            to: z
                .string()
                .optional()
                .describe("Arrival location, for example Singapore."),
        },
        async ({ from, to }) => toToolContent(findFlights({ from, to })),
    );

    tool(
        "recommend_bookings",
        "Get a small set of recommended flights from the travel API. Use this when the user asks for 'recommendations', 'suggestions', 'deals', or 'what's good today' rather than a specific route.",
        {
            limit: z
                .number()
                .int()
                .min(1)
                .max(10)
                .optional()
                .describe("Number of flights to return (1-10, default 3)."),
        },
        async ({ limit }) => {
            const safeLimit = Math.min(
                Math.max(Number.isFinite(limit) ? limit : 3, 1),
                10,
            );

            return toToolContent(findRecommendedFlights({ limit: safeLimit }));
        },
    );

    tool(
        "search_hotels",
        "Search available hotels from the travel API.",
        {
            location: z
                .string()
                .optional()
                .describe("Hotel location, for example Singapore."),
        },
        async ({ location }) => toToolContent(findHotels({ location })),
    );

    tool(
        "get_trips",
        "Get saved trip ideas from the travel API.",
        {},
        async () => toToolContent(listTrips({})),
    );

    tool(
        "get_locations",
        "Get available travel locations from the travel API.",
        {
            category: z
                .enum(["flights", "hotels", "trips"])
                .optional()
                .describe("Optional location category."),
        },
        async ({ category }) => toToolContent(listLocations({ category })),
    );

    tool(
        "get_profile",
        "Get the current authenticated user's profile from the travel API.",
        {},
        async () =>
            toToolContent({
                id: user.id,
                username: user.username,
                email: user.email,
                givenName: user.givenName,
                familyName: user.familyName,
            }),
    );

    tool(
        "get_flight_bookings",
        "Get flight bookings for the current authenticated user.",
        {},
        async () => toToolContent(listBookedFlights(user.id)),
    );

    tool(
        "create_booking",
        "Create a sample booking in the travel API.",
        {
            type: z.enum(["flight", "hotel", "trip"]).describe("Booking type."),
            itemId: z.string().describe("Flight or hotel item ID to book."),
            travelers: z
                .number()
                .int()
                .optional()
                .describe("Number of travelers."),
        },
        async ({ type, itemId, travelers }) => {
            const requestedTravelers = travelers ?? 1;

            if (
                !Number.isInteger(requestedTravelers) ||
                requestedTravelers < 1
            ) {
                throw new Error("travelers must be a positive integer");
            }

            const duplicate = findDuplicateBooking({
                username: user.id,
                type,
                itemId,
            });

            if (duplicate) {
                throw new Error("This booking already exists.");
            }

            const booking = createBookingRecord({
                id: `booking-${randomUUID()}`,
                bookingReference: generateBookingReference(),
                user,
                type,
                itemId,
                travelers: requestedTravelers,
                status: "confirmed",
                createdAt: new Date().toISOString(),
            });

            return toToolContent(booking);
        },
    );

    tool(
        "delete_all_bookings",
        "Delete ALL flight bookings for the current authenticated user. Use this to reset the user's bookings (e.g. when the user explicitly says 'clear all my bookings' or 'reset my bookings'). Destructive — only call when explicitly requested.",
        {},
        async () =>
            toToolContent({
                ...deleteBookingsForUser(user.id),
                username: user.id,
            }),
    );

  tool(
    "find_upgrade_options",
    "Find the Business class option for an Economy flight booking. Pass the booking ID to get the matching Business class flight (if any) and whether it can be upgraded directly.",
    {
      bookingId: z.string().describe("The ID of the Economy booking to find a Business class upgrade for.")
    },
    async ({ bookingId }) => {
      const isM2M = user.rawClaims?.grant_type === "client_credentials";
      const existingBooking = isM2M
        ? getBookingById(bookingId)
        : listBookedFlights(user.id).find((b) => b.id === bookingId);

      if (!existingBooking) {
        throw new Error("Booking not found or does not belong to the current user.");
      }

      if (existingBooking.flight.cabin?.toLowerCase() !== "economy") {
        throw new Error("Only Economy bookings can be upgraded.");
      }

      const bizFlight = findMatchingBusinessFlight(existingBooking.flight.id);

      if (!bizFlight) {
        return toToolContent({ available: false, message: "No Business class option exists for this flight." });
      }

      return toToolContent({
        available: true,
        canUpgradeDirectly: bizFlight.available === 1,
        businessFlight: bizFlight,
        economyFlight: existingBooking.flight,
        priceDifference: Math.max(0, bizFlight.price - existingBooking.flight.price)
      });
    }
  );

  tool(
    "upgrade_booking",
    "Directly upgrade an Economy booking to its Business class counterpart when seats are available. Only call this when find_upgrade_options shows canUpgradeDirectly: true. This upgrades the booking immediately without the asynchronous CIBA approval process.",
    {
      bookingId: z.string().describe("The ID of the existing Economy flight booking to upgrade."),
      toFlightId: z.string().describe("The ID of the target Business class flight.")
    },
    async ({ bookingId, toFlightId }) => {
      const allBookings = listBookedFlights(user.id);
      const existingBooking = allBookings.find((b) => b.id === bookingId);

            if (!existingBooking) {
                throw new Error(
                    "Booking not found or does not belong to the current user.",
                );
            }

            if (existingBooking.flight.cabin?.toLowerCase() !== "economy") {
                throw new Error("Only Economy bookings can be upgraded.");
            }

            const targetFlight = findFlightById(toFlightId);

            if (!targetFlight) {
                throw new Error("Target upgrade flight not found.");
            }

            if (targetFlight.cabin?.toLowerCase() !== "business") {
                throw new Error(
                    "Target flight must be a Business class flight.",
                );
            }

            if (
                targetFlight.from.toLowerCase() !==
                    existingBooking.flight.from.toLowerCase() ||
                targetFlight.to.toLowerCase() !==
                    existingBooking.flight.to.toLowerCase()
            ) {
                throw new Error(
                    "Target flight must be on the same route as the booked flight.",
                );
            }

      if (targetFlight.available !== 1) {
        throw new Error("This Business class flight is not available for direct upgrade. Please use request_upgrade instead.");
      }

      const { updated } = updateBookingFlight({ bookingId, newFlightId: toFlightId });

      if (!updated) {
        throw new Error(`Booking ${bookingId} could not be updated — it may no longer exist.`);
      }

      return toToolContent({
        success: true,
        bookingId,
        newFlightId: toFlightId,
        flight: targetFlight,
        message: "Your booking has been upgraded to Business class."
      });
    }
  );

  tool(
    "request_upgrade",
    "Queue a flight upgrade request for the current user. Only call this when find_upgrade_options shows canUpgradeDirectly: false. The upgrade scheduler will find the matching Business class flight and process it asynchronously via CIBA approval.",
    {
      bookingId: z.string().describe("The ID of the existing Economy flight booking to upgrade.")
    },
    async ({ bookingId }) => {
      const allBookings = listBookedFlights(user.id);
      const existingBooking = allBookings.find((b) => b.id === bookingId);

      if (!existingBooking) {
        throw new Error("Booking not found or does not belong to the current user.");
      }

      if (existingBooking.flight.cabin?.toLowerCase() !== "economy") {
        throw new Error("Only Economy bookings can be upgraded.");
      }

      if (!user.email && !idToken) {
        throw new Error("User email or ID token is required to process the upgrade request.");
      }

      const upgradeRequest = createUpgradeRequest({
        id: `upgrade-${randomUUID()}`,
        userId: user.id,
        email: user.email,
        idToken: idToken ?? null,
        bookingId,
        fromFlightId: existingBooking.flight.id,
        createdAt: new Date().toISOString()
      });

      return toToolContent({
        ...upgradeRequest,
        message: "Upgrade request submitted. It will be processed shortly and you will be notified via your registered device."
      });
    }
  );

    tool(
        "get_pending_upgrade",
        "For the upgrade scheduler agent only. Returns the count of pending upgrade requests and one request to process next. Requires upgrade:read scope (M2M token).",
        {},
        async () => toToolContent(getOnePendingUpgrade()),
    );

  tool(
    "process_upgrade",
    "Process a specific upgrade request. Validates that the authenticated user owns the request, resolves the matching Business class flight, updates the booking, and marks the request as success or failed. Requires upgrade:process scope (CIBA user token).",
    {
      upgradeRequestId: z.string().describe("The ID of the upgrade request to process.")
    },
    async ({ upgradeRequestId }) => {
      const upgradeRequest = getUpgradeRequestById(upgradeRequestId);

            if (!upgradeRequest) {
                throw new Error("Upgrade request not found.");
            }

            if (upgradeRequest.userId !== user.id) {
                const err = new Error(
                    "Not authorized to process this upgrade request.",
                );
                err.code = "insufficient_scope";
                throw err;
            }

            if (upgradeRequest.status !== "pending") {
                throw new Error(
                    `Upgrade request is already ${upgradeRequest.status}.`,
                );
            }

      const bizFlight = findMatchingBusinessFlight(upgradeRequest.fromFlightId);

      if (!bizFlight) {
        throw new Error(`No matching Business class flight found for flight ${upgradeRequest.fromFlightId}.`);
      }

      if (bizFlight.available !== 1) {
        throw new Error(`Business class flight ${bizFlight.id} is not yet available for upgrade.`);
      }

      const now = new Date().toISOString();

      try {
        const { updated } = updateBookingFlight({ bookingId: upgradeRequest.bookingId, newFlightId: bizFlight.id });

        if (!updated) {
          updateUpgradeStatus({ id: upgradeRequestId, status: "failed", updatedAt: now });
          throw new Error(`Booking ${upgradeRequest.bookingId} could not be updated — it may no longer exist.`);
        }

        updateUpgradeStatus({ id: upgradeRequestId, status: "success", updatedAt: now });

        return toToolContent({
          upgradeRequestId,
          status: "success",
          bookingId: upgradeRequest.bookingId,
          newFlightId: bizFlight.id,
          updatedAt: now
        });
      } catch (err) {
        updateUpgradeStatus({ id: upgradeRequestId, status: "failed", updatedAt: now });
        throw new Error(`Upgrade processing failed: ${err.message}`);
      }
    }
  );

    return server;
}

function corsOrigin(request) {
    return request?.headers?.origin || "*";
}

function sendUnauthorized(request, response, message) {
    const protocol = request.headers["x-forwarded-proto"] || "http";
    const host =
        request.headers.host || `localhost:${process.env.PORT || 8787}`;
    const resourceMetadataUrl = `${protocol}://${host}/.well-known/oauth-protected-resource`;

    response.writeHead(401, {
        "Content-Type": "application/json",
        "WWW-Authenticate": `Bearer resource_metadata="${resourceMetadataUrl}", error="invalid_token"`,
        "Access-Control-Allow-Origin": corsOrigin(request),
        "Access-Control-Expose-Headers": "WWW-Authenticate",
    });
    response.end(JSON.stringify({ error: message || "Unauthorized" }));
}

export async function handleMcpRequest(request, response, body) {
    // Set CORS headers up front. Node's response.setHeader() persists through
    // subsequent writeHead() calls unless the writeHead explicitly overrides the
    // same header name. The MCP SDK's StreamableHTTPServerTransport writes its
    // own response (Content-Type, etc.) but does not set Access-Control-Allow-*,
    // so our pre-set CORS headers survive — letting browser-based MCP clients
    // like MCP Inspector at http://localhost:6274 connect.
    response.setHeader("Access-Control-Allow-Origin", corsOrigin(request));
    response.setHeader(
        "Access-Control-Expose-Headers",
        "WWW-Authenticate, mcp-session-id",
    );

    let user;

    try {
        user = await resolveUser(request);
    } catch (err) {
        sendUnauthorized(request, response, err.message || "Unauthorized");
        return;
    }

    logMcpToolCall(body, user);

  // x-id-token carries the caller's OIDC ID token as an identity assertion,
  // following the HTTP convention of keeping auth metadata in headers (same plane
  // as the Bearer token). We do a basic sub-match to ensure the assertion belongs
  // to the authenticated user before trusting it downstream in request_upgrade.
  let idToken = null;
  const rawIdToken = request.headers["x-id-token"];

  if (rawIdToken) {
    try {
      const claims = await validateIdToken(rawIdToken);

      if (claims.sub && claims.sub === user.id) {
        idToken = rawIdToken;
      } else {
        console.warn(`[mcp] x-id-token sub (${claims.sub}) does not match authenticated user (${user.id}) — ignoring`);
      }
    } catch (err) {
      console.warn(`[mcp] x-id-token validation failed — ignoring: ${err.message}`);
    }
  }

  try {
    const server = createTravelMcpServer(user, idToken);
    const transport = new StreamableHTTPServerTransport({
      sessionIdGenerator: undefined
    });

        response.on("close", () => {
            transport.close();
        });

        await server.connect(transport);
        await transport.handleRequest(request, response, body);
    } catch (error) {
        console.error("[mcp] error handling request:", error);

        if (!response.headersSent) {
            response.writeHead(500, { "Content-Type": "application/json" });
            response.end(
                JSON.stringify({
                    error: error.message || "Failed to handle MCP request.",
                }),
            );
        }
    }
}

export function getProtectedResourceMetadata(request) {
    const protocol = request.headers["x-forwarded-proto"] || "http";
    const host =
        request.headers.host || `localhost:${process.env.PORT || 8787}`;
    const issuer =
        process.env.THUNDER_ISSUER || process.env.THUNDER_BASE_URL || "";

    return {
        resource: `${protocol}://${host}/mcp`,
        authorization_servers: issuer ? [issuer] : [],
        scopes_supported: [
            "booking:read",
            "booking:create",
            "booking:cancel",
            "booking:recommend",
            "booking:upgrade",
            "upgrade:read",
            "upgrade:search",
            "upgrade:process",
        ],
        bearer_methods_supported: ["header"],
    };
}
