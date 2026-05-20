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
  listBookedFlights,
  listLocations,
  listTrips
} from "./db.js";

const port = Number(process.env.PORT || 8787);
const frontendOrigin = process.env.FRONTEND_ORIGIN || "http://localhost:5173";

function sendJson(response, statusCode, body) {
  response.writeHead(statusCode, {
    "Content-Type": "application/json",
    "Access-Control-Allow-Origin": frontendOrigin,
    "Access-Control-Allow-Methods": "GET,POST,DELETE,OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type,Authorization"
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
  const username = user.username || user.email || user.id;

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
    return sendJson(response, 204, {});
  }

  try {
    if (request.method === "GET" && url.pathname === "/health") {
      return sendJson(response, 200, { status: "ok" });
    }

    if (request.method === "GET" && url.pathname === "/api/flights") {
      return sendJson(response, 200, {
        data: searchFlights(url.searchParams)
      });
    }

    if (request.method === "GET" && url.pathname.startsWith("/api/flights/")) {
      const flightId = decodeURIComponent(url.pathname.replace("/api/flights/", ""));
      const flight = findFlightById(flightId);

      if (!flight) {
        return sendJson(response, 404, { error: "Flight not found" });
      }

      return sendJson(response, 200, {
        data: flight
      });
    }

    if (request.method === "GET" && url.pathname === "/api/hotels") {
      return sendJson(response, 200, {
        data: searchHotels(url.searchParams)
      });
    }

    if (request.method === "GET" && url.pathname === "/api/locations") {
      return sendJson(response, 200, {
        data: listLocations({
          category: url.searchParams.get("category")
        })
      });
    }

    if (request.method === "GET" && url.pathname === "/api/trips") {
      return sendJson(response, 200, {
        data: listTrips({
          destination: url.searchParams.get("destination")
        })
      });
    }

    if (request.method === "GET" && url.pathname === "/api/me") {
      const user = await resolveUser(request);

      return sendJson(response, 200, { data: user });
    }

    if (request.method === "GET" && url.pathname === "/api/bookings/flights") {
      const user = await resolveUser(request);

      requireScope(user, "booking:read");

      const username = user.username || user.email || user.id;

      return sendJson(response, 200, {
        data: listBookedFlights(username)
      });
    }

    if (request.method === "POST" && url.pathname === "/api/bookings") {
      const result = await handleBooking(request);

      return sendJson(response, result.statusCode, result.body);
    }

    if (request.method === "DELETE" && url.pathname === "/api/bookings/flights") {
      const user = await resolveUser(request);

      requireScope(user, "booking:cancel");

      const username = user.username || user.email || user.id;
      const result = deleteBookingsForUser(username);

      return sendJson(response, 200, {
        data: { deleted: result.deleted, username }
      });
    }

    return sendJson(response, 404, { error: "Route not found" });
  } catch (error) {
    const statusCode = error.statusCode
      || (error.message.toLowerCase().includes("token") ? 401 : 500);

    return sendJson(response, statusCode, {
      error: error.message
    });
  }
}

createServer(route).listen(port, () => {
  console.log(`Wayfinder Travel API listening on http://localhost:${port}`);
});
