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

import { DatabaseSync } from "node:sqlite";
import { existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const defaultDbPath = resolve(__dirname, "..", "wayfinder.sqlite");
const dbPath = process.env.SQLITE_DB_PATH || defaultDbPath;

let db;

function ensureSchema(database) {
  database.exec(`
    CREATE TABLE IF NOT EXISTS bookings (
      id TEXT PRIMARY KEY,
      booking_reference TEXT NOT NULL,
      user_id TEXT NOT NULL,
      username TEXT NOT NULL,
      type TEXT NOT NULL,
      item_id TEXT NOT NULL,
      travelers INTEGER NOT NULL,
      status TEXT NOT NULL,
      created_at TEXT NOT NULL
    );

    CREATE TABLE IF NOT EXISTS upgrade_requests (
      id TEXT PRIMARY KEY,
      user_id TEXT NOT NULL,
      username TEXT NOT NULL,
      booking_id TEXT NOT NULL,
      from_flight_id TEXT NOT NULL,
      to_flight_id TEXT NOT NULL,
      price_difference REAL NOT NULL,
      status TEXT NOT NULL DEFAULT 'pending',
      created_at TEXT NOT NULL,
      updated_at TEXT NOT NULL
    );
  `);

  const bookingColumns = database.prepare("PRAGMA table_info(bookings)").all();
  const hasBookingReference = bookingColumns.some((column) => column.name === "booking_reference");

  if (!hasBookingReference) {
    database.exec("ALTER TABLE bookings ADD COLUMN booking_reference TEXT;");
  }

  const bookingsWithoutReference = database
    .prepare("SELECT id FROM bookings WHERE booking_reference IS NULL OR booking_reference = ''")
    .all();

  const updateBookingReference = database.prepare(
    "UPDATE bookings SET booking_reference = @bookingReference WHERE id = @id"
  );

  for (const booking of bookingsWithoutReference) {
    const source = String(booking.id || "").replace(/^booking-/i, "").replace(/[^a-z0-9]/gi, "");
    const bookingReference = `WF-${source.toUpperCase().padEnd(8, "0").slice(0, 8)}`;

    updateBookingReference.run({
      id: booking.id,
      bookingReference
    });
  }
}

function getDatabase() {
  if (!existsSync(dbPath)) {
    throw new Error("SQLite database not found. Run `npm run seed` from the backend directory.");
  }

  if (!db) {
    db = new DatabaseSync(dbPath);
    ensureSchema(db);
  }

  return db;
}

function parseJsonArray(value) {
  try {
    return JSON.parse(value || "[]");
  } catch {
    return [];
  }
}

function mapFlight(row) {
  return {
    id: row.id,
    from: row.from_city,
    to: row.to_city,
    airline: row.airline,
    departureTime: row.departure_time,
    arrivalTime: row.arrival_time,
    duration: row.duration,
    stops: row.stops,
    price: row.price,
    currency: row.currency,
    cabin: row.cabin,
    dates: row.dates,
    tags: parseJsonArray(row.tags)
  };
}

function mapHotel(row) {
  return {
    id: row.id,
    name: row.name,
    location: row.location,
    nightlyRate: row.nightly_rate,
    currency: row.currency,
    rating: row.rating,
    amenities: parseJsonArray(row.amenities)
  };
}

function mapTrip(row) {
  return {
    id: row.id,
    title: row.title,
    destination: row.destination,
    flightId: row.flight_id,
    hotelId: row.hotel_id,
    status: row.status,
    totalEstimate: row.total_estimate,
    currency: row.currency
  };
}

export function findFlights({ from, to, cabin }) {
  const conditions = [];
  const params = {};

  if (from) {
    conditions.push("LOWER(from_city) LIKE LOWER(@from)");
    params.from = `%${from}%`;
  }

  if (to) {
    conditions.push("LOWER(to_city) LIKE LOWER(@to)");
    params.to = `%${to}%`;
  }

  if (cabin) {
    conditions.push("LOWER(cabin) LIKE LOWER(@cabin)");
    params.cabin = `%${cabin}%`;
  }

  const whereClause = conditions.length ? `WHERE ${conditions.join(" AND ")}` : "";
  const rows = getDatabase()
    .prepare(`SELECT * FROM flights ${whereClause} ORDER BY price ASC`)
    .all(params);

  return rows.map(mapFlight);
}

export function findRecommendedFlights({ limit = 3 } = {}) {
  const rows = getDatabase()
    .prepare("SELECT * FROM flights ORDER BY RANDOM() LIMIT @limit")
    .all({ limit });

  return rows.map(mapFlight);
}

export function findFlightById(id) {
  const row = getDatabase()
    .prepare("SELECT * FROM flights WHERE id = @id")
    .get({ id });

  return row ? mapFlight(row) : null;
}

export function findHotels({ location, maxNightlyRate }) {
  const conditions = [];
  const params = {};

  if (location) {
    conditions.push("LOWER(location) LIKE LOWER(@location)");
    params.location = `%${location}%`;
  }

  if (maxNightlyRate !== undefined && maxNightlyRate !== null && !Number.isNaN(maxNightlyRate)) {
    conditions.push("nightly_rate <= @maxNightlyRate");
    params.maxNightlyRate = maxNightlyRate;
  }

  const whereClause = conditions.length ? `WHERE ${conditions.join(" AND ")}` : "";
  const rows = getDatabase()
    .prepare(`SELECT * FROM hotels ${whereClause} ORDER BY rating DESC`)
    .all(params);

  return rows.map(mapHotel);
}

export function listTrips({ destination } = {}) {
  const conditions = [];
  const params = {};

  if (destination) {
    conditions.push("LOWER(destination) LIKE LOWER(@destination)");
    params.destination = `%${destination}%`;
  }

  const whereClause = conditions.length ? `WHERE ${conditions.join(" AND ")}` : "";
  const rows = getDatabase()
    .prepare(`SELECT * FROM trips ${whereClause} ORDER BY total_estimate ASC`)
    .all(params);

  return rows.map(mapTrip);
}

export function listLocations({ category } = {}) {
  let query = `
    SELECT from_city AS name, 'city' AS type FROM flights
    UNION
    SELECT to_city AS name, 'city' AS type FROM flights
  `;

  if (category === "hotels") {
    query = `
      SELECT location AS name, 'area' AS type FROM hotels
    `;
  }

  if (category === "trips") {
    query = `
      SELECT destination AS name, 'destination' AS type FROM trips
    `;
  }

  const rows = getDatabase()
    .prepare(`SELECT DISTINCT name, type FROM (${query}) ORDER BY name ASC`)
    .all();

  return rows;
}

export function createBookingRecord({
  id,
  bookingReference,
  user,
  type,
  itemId,
  travelers,
  status,
  createdAt
}) {
  const username = user.id;

  getDatabase()
    .prepare(
      `
        INSERT INTO bookings (
          id,
          booking_reference,
          user_id,
          username,
          type,
          item_id,
          travelers,
          status,
          created_at
        ) VALUES (
          @id,
          @bookingReference,
          @userId,
          @username,
          @type,
          @itemId,
          @travelers,
          @status,
          @createdAt
        )
      `
    )
    .run({
      id,
      bookingReference,
      userId: user.id,
      username,
      type,
      itemId,
      travelers,
      status,
      createdAt
    });

  return {
    id,
    bookingReference,
    userId: user.id,
    username,
    type,
    itemId,
    travelers,
    status,
    createdAt
  };
}

export function findDuplicateBooking({ username, type, itemId }) {
  if (type !== "flight") {
    return getDatabase()
      .prepare(
        `
          SELECT id
          FROM bookings
          WHERE username = @username
            AND type = @type
            AND item_id = @itemId
          LIMIT 1
        `
      )
      .get({ username, type, itemId });
  }

  return getDatabase()
    .prepare(
      `
        SELECT bookings.id
        FROM bookings
        INNER JOIN flights booked_flight ON bookings.item_id = booked_flight.id
        INNER JOIN flights requested_flight ON requested_flight.id = @itemId
        WHERE bookings.username = @username
          AND bookings.type = 'flight'
          AND booked_flight.from_city = requested_flight.from_city
          AND booked_flight.to_city = requested_flight.to_city
          AND booked_flight.departure_time = requested_flight.departure_time
          AND booked_flight.arrival_time = requested_flight.arrival_time
          AND booked_flight.dates = requested_flight.dates
        LIMIT 1
      `
    )
    .get({ username, itemId });
}

export function listBookedFlights(username) {
  const rows = getDatabase()
    .prepare(
      `
        SELECT
          bookings.id AS booking_id,
          bookings.booking_reference,
          bookings.username,
          bookings.travelers,
          bookings.status,
          bookings.created_at,
          flights.*
        FROM bookings
        INNER JOIN flights ON bookings.item_id = flights.id
        WHERE bookings.type = 'flight'
          AND bookings.username = @username
        ORDER BY bookings.created_at DESC
      `
    )
    .all({ username });

  return rows.map((row) => ({
    id: row.booking_id,
    bookingReference: row.booking_reference,
    username: row.username,
    travelers: row.travelers,
    status: row.status,
    createdAt: row.created_at,
    flight: mapFlight(row)
  }));
}

export function deleteBookingsForUser(username) {
  const result = getDatabase()
    .prepare(`DELETE FROM bookings WHERE username = @username`)
    .run({ username });

  return { deleted: result.changes };
}

export function findBusinessFlightForRoute({ fromCity, toCity, airline }) {
  const row = getDatabase()
    .prepare(
      `SELECT * FROM flights
       WHERE LOWER(from_city) = LOWER(@fromCity)
         AND LOWER(to_city) = LOWER(@toCity)
         AND LOWER(airline) = LOWER(@airline)
         AND LOWER(cabin) = 'business'
       LIMIT 1`
    )
    .get({ fromCity, toCity, airline });

  return row ? mapFlight(row) : null;
}

export function findBusinessFlightsForRoute({ fromCity, toCity }) {
  const rows = getDatabase()
    .prepare(
      `SELECT * FROM flights
       WHERE LOWER(from_city) = LOWER(@fromCity)
         AND LOWER(to_city) = LOWER(@toCity)
         AND LOWER(cabin) = 'business'
       ORDER BY price ASC`
    )
    .all({ fromCity, toCity });

  return rows.map(mapFlight);
}

export function createUpgradeRequest({ id, userId, email, bookingId, fromFlightId, toFlightId, priceDifference, createdAt }) {
  getDatabase()
    .prepare(
      `INSERT INTO upgrade_requests
         (id, user_id, username, booking_id, from_flight_id, to_flight_id, price_difference, status, created_at, updated_at)
       VALUES
         (@id, @userId, @email, @bookingId, @fromFlightId, @toFlightId, @priceDifference, 'pending', @createdAt, @createdAt)`
    )
    .run({ id, userId, email: email ?? null, bookingId, fromFlightId, toFlightId, priceDifference, createdAt });

  return { id, userId, email, bookingId, fromFlightId, toFlightId, priceDifference, status: "pending", createdAt };
}

export function getOnePendingUpgrade() {
  const db = getDatabase();

  const countRow = db
    .prepare(`SELECT COUNT(*) AS count FROM upgrade_requests WHERE status = 'pending'`)
    .get();

  const pendingCount = countRow ? countRow.count : 0;

  if (pendingCount === 0) {
    return { pendingCount: 0, request: null };
  }

  const row = db
    .prepare(
      `SELECT ur.*,
              ur.username AS email,
              f_from.from_city, f_from.to_city, f_from.airline,
              f_from.price AS from_price, f_from.cabin AS from_cabin,
              f_to.price AS to_price, f_to.cabin AS to_cabin
       FROM upgrade_requests ur
       JOIN flights f_from ON ur.from_flight_id = f_from.id
       JOIN flights f_to ON ur.to_flight_id = f_to.id
       WHERE ur.status = 'pending'
       ORDER BY ur.created_at ASC
       LIMIT 1`
    )
    .get();

  if (!row) {
    return { pendingCount: 0, request: null };
  }

  return {
    pendingCount,
    request: {
      id: row.id,
      userId: row.user_id,
      email: row.email,
      bookingId: row.booking_id,
      fromFlightId: row.from_flight_id,
      toFlightId: row.to_flight_id,
      priceDifference: row.price_difference,
      status: row.status,
      createdAt: row.created_at,
      updatedAt: row.updated_at,
      route: { from: row.from_city, to: row.to_city, airline: row.airline },
      fromCabin: row.from_cabin,
      toCabin: row.to_cabin
    }
  };
}

export function updateUpgradeStatus({ id, status, updatedAt }) {
  const result = getDatabase()
    .prepare(
      `UPDATE upgrade_requests SET status = @status, updated_at = @updatedAt WHERE id = @id`
    )
    .run({ id, status, updatedAt });

  return { updated: result.changes };
}

export function getUpgradeRequestById(id) {
  const row = getDatabase()
    .prepare(`SELECT * FROM upgrade_requests WHERE id = @id`)
    .get({ id });

  if (!row) return null;

  return {
    id: row.id,
    userId: row.user_id,
    username: row.username,
    bookingId: row.booking_id,
    fromFlightId: row.from_flight_id,
    toFlightId: row.to_flight_id,
    priceDifference: row.price_difference,
    status: row.status,
    createdAt: row.created_at,
    updatedAt: row.updated_at
  };
}

export function updateBookingFlight({ bookingId, newFlightId }) {
  const result = getDatabase()
    .prepare(`UPDATE bookings SET item_id = @newFlightId WHERE id = @bookingId AND type = 'flight'`)
    .run({ bookingId, newFlightId });

  return { updated: result.changes };
}
