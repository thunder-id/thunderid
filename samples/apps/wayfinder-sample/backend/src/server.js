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

import { createServer } from "node:http";
import { randomUUID } from "node:crypto";
import { URL } from "node:url";
import "dotenv/config";

// Local-dev TLS bypass: Thunder ships with a self-signed cert on localhost,
// so the JWKS fetch in auth.js would otherwise fail with UNABLE_TO_VERIFY_LEAF_SIGNATURE.
// We disable Node's TLS verification ONLY when the configured issuer base URL
// points at localhost / 127.0.0.1.
const __thunderBaseUrl = process.env.THUNDER_BASE_URL || "";
if (/^https?:\/\/(localhost|127\.0\.0\.1)(:\d+)?(\/|$)/.test(__thunderBaseUrl)) {
  process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";
  console.warn("[api] Local Thunder detected — NODE_TLS_REJECT_UNAUTHORIZED set to 0. Do not use this build in production.");
}

import { resolveUser, requireScope } from "./auth.js";
import {
  createBookingRecord,
  deleteBookingsForUser,
  findDuplicateBooking,
  findFlightById,
  findFlights,
  findHotels,
  findRecommendedFlights,
  listBookedFlights,
  listLocations,
  listTrips,
  setAllBusinessFlightsAvailable,
  setAllBusinessFlightsUnavailable
} from "./db.js";
import { getProtectedResourceMetadata, handleMcpRequest } from "./mcp.js";

const port = Number(process.env.PORT || 8787);

// CORS: every endpoint here requires a Bearer token, so there is no cookie-
// based session to protect. We echo back the request Origin (so MCP Inspector
// at :6274 and the sample CLI on a loopback port both work), falling back to
// FRONTEND_ORIGIN for non-browser callers without an Origin header.
const frontendOrigin = process.env.FRONTEND_ORIGIN || "http://localhost:5173";

function corsOrigin(request) {
  return request?.headers?.origin || frontendOrigin;
}

function decodeTokenClaims(authHeader) {
  if (!authHeader || !authHeader.startsWith("Bearer ")) return null;
  try {
    const parts = authHeader.slice(7).split(".");
    return JSON.parse(Buffer.from(parts[1], "base64url").toString());
  } catch {
    return null;
  }
}

function logRequest(method, pathname, claims) {
  if (!claims) {
    console.log(`${method} ${pathname}`);
    return;
  }
  const type = claims.grant_type === "client_credentials" ? "m2m" : "user";
  const aud = Array.isArray(claims.aud) ? claims.aud.join(",") : (claims.aud || "-");
  const act = claims.act?.sub ? ` | act: ${claims.act.sub}` : "";
  console.log(
    `${method} ${pathname} | type: ${type} | client_id: ${claims.client_id || "-"} | sub: ${claims.sub || "-"}${act} | aud: ${aud} | scope: ${claims.scope || "-"}`
  );
}

function sendJson(response, statusCode, body, request) {
  response.writeHead(statusCode, {
    "Content-Type": "application/json",
    "Access-Control-Allow-Origin": corsOrigin(request),
    "Access-Control-Allow-Methods": "GET,POST,DELETE,OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type,Authorization,mcp-session-id,mcp-protocol-version,last-event-id",
    "Access-Control-Expose-Headers": "WWW-Authenticate,mcp-session-id",
    "Access-Control-Max-Age": "86400"
  });
  response.end(JSON.stringify(body));
}

async function readJsonBody(request) {
  const chunks = [];

  for await (const chunk of request) {
    chunks.push(chunk);
  }

  if (chunks.length === 0) {
    return {};
  }

  try {
    return JSON.parse(Buffer.concat(chunks).toString("utf8"));
  } catch (error) {
    const badJsonError = new Error("Request body is not valid JSON.");
    badJsonError.statusCode = 400;
    throw badJsonError;
  }
}

function searchFlights(params) {
  return findFlights({
    from: params.get("from"),
    to: params.get("to"),
    cabin: params.get("cabin")
  });
}

function searchHotels(params) {
  return findHotels({
    location: params.get("location"),
    maxNightlyRate: Number(params.get("maxNightlyRate") || 0)
  });
}

function generateBookingReference() {
  return `WF-${randomUUID().replace(/-/g, "").slice(0, 8).toUpperCase()}`;
}

async function handleBooking(request) {
  const user = await resolveUser(request);

  requireScope(user, "booking:create");

  const body = await readJsonBody(request);
  const itemType = body.type;
  const itemId = body.itemId;
  const requestedTravelers = body.travelers ?? 1;
  const travelers = Number(requestedTravelers);
  const username = user.id;

  if (!["flight", "hotel", "trip"].includes(itemType)) {
    return {
      statusCode: 400,
      body: { error: "type must be one of: flight, hotel, trip" }
    };
  }

  if (!itemId) {
    return {
      statusCode: 400,
      body: { error: "itemId is required" }
    };
  }

  if (!Number.isInteger(travelers) || travelers < 1) {
    return {
      statusCode: 400,
      body: { error: "travelers must be a positive integer" }
    };
  }

  const duplicateBooking = findDuplicateBooking({
    username,
    type: itemType,
    itemId
  });

  if (duplicateBooking) {
    return {
      statusCode: 409,
      body: { error: "This booking already exists." }
    };
  }

  const booking = createBookingRecord({
    id: `booking-${randomUUID()}`,
    bookingReference: generateBookingReference(),
    user,
    type: itemType,
    itemId,
    travelers,
    status: "confirmed",
    createdAt: new Date().toISOString()
  });

  return {
    statusCode: 201,
    body: booking
  };
}

async function route(request, response) {
  const url = new URL(request.url, `http://${request.headers.host}`);

  if (request.method === "OPTIONS") {
    return sendJson(response, 204, {}, request);
  }

  const claims = decodeTokenClaims(request.headers.authorization);
  logRequest(request.method, url.pathname, claims);

  try {
    if (request.method === "POST" && url.pathname === "/mcp") {
      let body;

      try {
        body = await readJsonBody(request);
      } catch (parseError) {
        return sendJson(response, 400, {
          error: parseError.message || "Malformed JSON body"
        }, request);
      }

      return handleMcpRequest(request, response, body);
    }

    if (request.method === "GET" && url.pathname === "/.well-known/oauth-protected-resource") {
      return sendJson(response, 200, getProtectedResourceMetadata(request), request);
    }

    if (request.method === "GET" && url.pathname === "/health") {
      return sendJson(response, 200, { status: "ok" }, request);
    }

    if (request.method === "GET" && url.pathname === "/api/flights") {
      return sendJson(response, 200, {
        data: searchFlights(url.searchParams)
      }, request);
    }

    if (request.method === "GET" && url.pathname === "/api/bookings/recommended") {
      const user = await resolveUser(request);

      requireScope(user, "booking:recommend");

      const rawLimit = Number(url.searchParams.get("limit") || 3);
      const limit = Math.min(Math.max(Number.isFinite(rawLimit) ? rawLimit : 3, 1), 10);

      return sendJson(response, 200, {
        data: findRecommendedFlights({ limit })
      }, request);
    }

    if (request.method === "GET" && url.pathname.startsWith("/api/flights/")) {
      const flightId = decodeURIComponent(url.pathname.replace("/api/flights/", ""));
      const flight = findFlightById(flightId);

      if (!flight) {
        return sendJson(response, 404, { error: "Flight not found" }, request);
      }

      return sendJson(response, 200, {
        data: flight
      }, request);
    }

    if (request.method === "GET" && url.pathname === "/api/hotels") {
      return sendJson(response, 200, {
        data: searchHotels(url.searchParams)
      }, request);
    }

    if (request.method === "GET" && url.pathname === "/api/locations") {
      return sendJson(response, 200, {
        data: listLocations({
          category: url.searchParams.get("category")
        })
      }, request);
    }

    if (request.method === "GET" && url.pathname === "/api/trips") {
      return sendJson(response, 200, {
        data: listTrips({
          destination: url.searchParams.get("destination")
        })
      }, request);
    }

    if (request.method === "GET" && url.pathname === "/api/me") {
      const user = await resolveUser(request);

      return sendJson(response, 200, { data: user }, request);
    }

    if (request.method === "GET" && url.pathname === "/api/bookings/flights") {
      const user = await resolveUser(request);

      requireScope(user, "booking:read");

      const username = user.id;

      return sendJson(response, 200, {
        data: listBookedFlights(username)
      }, request);
    }

    if (request.method === "POST" && url.pathname === "/api/bookings") {
      const result = await handleBooking(request);

      return sendJson(response, result.statusCode, result.body, request);
    }

    if (request.method === "DELETE" && url.pathname === "/api/bookings/flights") {
      const user = await resolveUser(request);

      requireScope(user, "booking:cancel");

      const username = user.id;
      const result = deleteBookingsForUser(username);

      return sendJson(response, 200, {
        data: { deleted: result.deleted, username }
      }, request);
    }

    if (request.method === "POST" && url.pathname === "/api/demo/unlock-business-class") {
      const result = setAllBusinessFlightsAvailable();

      return sendJson(response, 200, {
        message: `Made ${result.updated} Business class flights available for direct upgrade.`,
        ...result
      }, request);
    }

    if (request.method === "POST" && url.pathname === "/api/demo/lock-business-class") {
      const result = setAllBusinessFlightsUnavailable();

      return sendJson(response, 200, {
        message: `Made ${result.updated} Business class flights unavailable (CIBA approval required).`,
        ...result
      }, request);
    }

    return sendJson(response, 404, { error: "Route not found" }, request);
  } catch (error) {
    const statusCode = error.statusCode
      || (error.message.toLowerCase().includes("token") ? 401 : 500);

    return sendJson(response, statusCode, {
      error: error.message
    }, request);
  }
}

createServer(route).listen(port, () => {
  console.log(`Wayfinder Travel API listening on http://localhost:${port}`);
  console.log(`  REST API: http://localhost:${port}/api`);
  console.log(`  MCP server: http://localhost:${port}/mcp`);
  console.log(`  Discovery: http://localhost:${port}/.well-known/oauth-protected-resource`);
});
