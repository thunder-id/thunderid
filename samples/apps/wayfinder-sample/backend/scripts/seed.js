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

import { existsSync, mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { DatabaseSync } from "../src/sqlite.js";

const __dirname = dirname(fileURLToPath(import.meta.url));
const apiRoot = resolve(__dirname, "..");
const dbPath = resolve(apiRoot, "wayfinder.sqlite");

const flights = [
  {
    id: "flight-cmb-sin-01",
    from: "Colombo",
    to: "Singapore",
    airline: "Serendib Air",
    departure_time: "08:45",
    arrival_time: "15:05",
    duration: "3h 50m",
    stops: 0,
    price: 314,
    currency: "USD",
    cabin: "Economy",
    dates: "Jun 12 - Jun 18",
    tags: JSON.stringify(["Best value", "Nonstop"]),
    available: 1
  },
  {
    id: "flight-sfo-tyo-01",
    from: "San Francisco",
    to: "Tokyo",
    airline: "Pacifica",
    departure_time: "11:20",
    arrival_time: "15:35",
    duration: "11h 15m",
    stops: 0,
    price: 782,
    currency: "USD",
    cabin: "Economy",
    dates: "Jul 04 - Jul 16",
    tags: JSON.stringify(["Nonstop", "Popular"]),
    available: 1
  },
  {
    id: "flight-lon-lis-01",
    from: "London",
    to: "Lisbon",
    airline: "Northline",
    departure_time: "17:10",
    arrival_time: "20:05",
    duration: "2h 55m",
    stops: 0,
    price: 168,
    currency: "USD",
    cabin: "Economy",
    dates: "Aug 21 - Aug 27",
    tags: JSON.stringify(["Weekend"]),
    available: 1
  },
  {
    id: "flight-cmb-dxb-01",
    from: "Colombo",
    to: "Dubai",
    airline: "Ceylon Wings",
    departure_time: "21:15",
    arrival_time: "00:25",
    duration: "4h 40m",
    stops: 0,
    price: 289,
    currency: "USD",
    cabin: "Economy",
    dates: "Jun 20 - Jun 26",
    tags: JSON.stringify(["Evening"]),
    available: 1
  },
  {
    id: "flight-cmb-sin-02",
    from: "Colombo",
    to: "Singapore",
    airline: "IslandJet",
    departure_time: "13:30",
    arrival_time: "19:50",
    duration: "3h 50m",
    stops: 0,
    price: 342,
    currency: "USD",
    cabin: "Economy",
    dates: "Jun 12 - Jun 18",
    tags: JSON.stringify(["Flexible ticket", "Carry-on included"]),
    available: 1
  },
  {
    id: "flight-cmb-sin-03",
    from: "Colombo",
    to: "Singapore",
    airline: "Meridian Airways",
    departure_time: "01:10",
    arrival_time: "09:20",
    duration: "5h 40m",
    stops: 1,
    price: 276,
    currency: "USD",
    cabin: "Economy",
    dates: "Jun 12 - Jun 18",
    tags: JSON.stringify(["Lowest price", "1 stop"]),
    available: 1
  },
  {
    id: "flight-cmb-tyo-01",
    from: "Colombo",
    to: "Tokyo",
    airline: "Serendib Air",
    departure_time: "07:25",
    arrival_time: "22:10",
    duration: "11h 15m",
    stops: 1,
    price: 598,
    currency: "USD",
    cabin: "Economy",
    dates: "Jul 04 - Jul 16",
    tags: JSON.stringify(["Good connection", "Meal included"]),
    available: 1
  },
  {
    id: "flight-dxb-lon-01",
    from: "Dubai",
    to: "London",
    airline: "Gulfline",
    departure_time: "09:40",
    arrival_time: "14:35",
    duration: "7h 55m",
    stops: 0,
    price: 431,
    currency: "USD",
    cabin: "Economy",
    dates: "Sep 02 - Sep 09",
    tags: JSON.stringify(["Nonstop", "Morning"]),
    available: 1
  },
  {
    id: "flight-sin-syd-01",
    from: "Singapore",
    to: "Sydney",
    airline: "Pacifica",
    departure_time: "20:15",
    arrival_time: "06:05",
    duration: "7h 50m",
    stops: 0,
    price: 502,
    currency: "USD",
    cabin: "Economy",
    dates: "Oct 10 - Oct 18",
    tags: JSON.stringify(["Overnight", "Nonstop"]),
    available: 1
  },
  {
    id: "flight-cmb-sin-01-biz",
    from: "Colombo",
    to: "Singapore",
    airline: "Serendib Air",
    departure_time: "08:45",
    arrival_time: "15:05",
    duration: "3h 50m",
    stops: 0,
    price: 680,
    currency: "USD",
    cabin: "Business",
    dates: "Jun 12 - Jun 18",
    tags: JSON.stringify(["Nonstop", "Business class"]),
    available: 0
  },
  {
    id: "flight-sfo-tyo-01-biz",
    from: "San Francisco",
    to: "Tokyo",
    airline: "Pacifica",
    departure_time: "11:20",
    arrival_time: "15:35",
    duration: "11h 15m",
    stops: 0,
    price: 1650,
    currency: "USD",
    cabin: "Business",
    dates: "Jul 04 - Jul 16",
    tags: JSON.stringify(["Nonstop", "Business class"]),
    available: 0
  },
  {
    id: "flight-lon-lis-01-biz",
    from: "London",
    to: "Lisbon",
    airline: "Northline",
    departure_time: "17:10",
    arrival_time: "20:05",
    duration: "2h 55m",
    stops: 0,
    price: 390,
    currency: "USD",
    cabin: "Business",
    dates: "Aug 21 - Aug 27",
    tags: JSON.stringify(["Business class"]),
    available: 0
  },
  {
    id: "flight-cmb-dxb-01-biz",
    from: "Colombo",
    to: "Dubai",
    airline: "Ceylon Wings",
    departure_time: "21:15",
    arrival_time: "00:25",
    duration: "4h 40m",
    stops: 0,
    price: 620,
    currency: "USD",
    cabin: "Business",
    dates: "Jun 20 - Jun 26",
    tags: JSON.stringify(["Business class"]),
    available: 0
  },
  {
    id: "flight-cmb-sin-02-biz",
    from: "Colombo",
    to: "Singapore",
    airline: "IslandJet",
    departure_time: "13:30",
    arrival_time: "19:50",
    duration: "3h 50m",
    stops: 0,
    price: 710,
    currency: "USD",
    cabin: "Business",
    dates: "Jun 12 - Jun 18",
    tags: JSON.stringify(["Business class", "Flexible ticket"]),
    available: 0
  },
  {
    id: "flight-cmb-sin-03-biz",
    from: "Colombo",
    to: "Singapore",
    airline: "Meridian Airways",
    departure_time: "01:10",
    arrival_time: "09:20",
    duration: "5h 40m",
    stops: 1,
    price: 590,
    currency: "USD",
    cabin: "Business",
    dates: "Jun 12 - Jun 18",
    tags: JSON.stringify(["Business class", "1 stop"]),
    available: 0
  },
  {
    id: "flight-cmb-tyo-01-biz",
    from: "Colombo",
    to: "Tokyo",
    airline: "Serendib Air",
    departure_time: "07:25",
    arrival_time: "22:10",
    duration: "11h 15m",
    stops: 1,
    price: 1280,
    currency: "USD",
    cabin: "Business",
    dates: "Jul 04 - Jul 16",
    tags: JSON.stringify(["Business class", "Meal included"]),
    available: 0
  },
  {
    id: "flight-dxb-lon-01-biz",
    from: "Dubai",
    to: "London",
    airline: "Gulfline",
    departure_time: "09:40",
    arrival_time: "14:35",
    duration: "7h 55m",
    stops: 0,
    price: 980,
    currency: "USD",
    cabin: "Business",
    dates: "Sep 02 - Sep 09",
    tags: JSON.stringify(["Nonstop", "Business class"]),
    available: 0
  },
  {
    id: "flight-sin-syd-01-biz",
    from: "Singapore",
    to: "Sydney",
    airline: "Pacifica",
    departure_time: "20:15",
    arrival_time: "06:05",
    duration: "7h 50m",
    stops: 0,
    price: 1100,
    currency: "USD",
    cabin: "Business",
    dates: "Oct 10 - Oct 18",
    tags: JSON.stringify(["Overnight", "Nonstop", "Business class"]),
    available: 0
  }
];

const hotels = [
  {
    id: "hotel-harborlight-suites",
    name: "Harborlight Suites",
    location: "Singapore Marina",
    nightly_rate: 142,
    currency: "USD",
    rating: 9.1,
    amenities: JSON.stringify(["Breakfast", "Pool", "Airport shuttle"])
  },
  {
    id: "hotel-saffron-yard",
    name: "The Saffron Yard",
    location: "Lisbon Old Town",
    nightly_rate: 119,
    currency: "USD",
    rating: 8.8,
    amenities: JSON.stringify(["Boutique rooms", "Rooftop bar", "Late checkout"])
  },
  {
    id: "hotel-north-pier-rooms",
    name: "North Pier Rooms",
    location: "Tokyo Bay",
    nightly_rate: 173,
    currency: "USD",
    rating: 9.4,
    amenities: JSON.stringify(["Bay view", "Onsen access", "Workspace"])
  },
  {
    id: "hotel-cinnamon-court",
    name: "Cinnamon Court",
    location: "Colombo Fort",
    nightly_rate: 96,
    currency: "USD",
    rating: 8.9,
    amenities: JSON.stringify(["Central location", "Breakfast", "Gym"])
  },
  {
    id: "hotel-garden-quay",
    name: "Garden Quay Hotel",
    location: "Singapore Riverside",
    nightly_rate: 128,
    currency: "USD",
    rating: 8.7,
    amenities: JSON.stringify(["River view", "Metro nearby", "Breakfast"])
  },
  {
    id: "hotel-orchid-house",
    name: "Orchid House",
    location: "Singapore Orchard",
    nightly_rate: 156,
    currency: "USD",
    rating: 9.0,
    amenities: JSON.stringify(["Family rooms", "Pool", "Shopping district"])
  },
  {
    id: "hotel-shibuya-harbor",
    name: "Shibuya Harbor",
    location: "Tokyo Shibuya",
    nightly_rate: 188,
    currency: "USD",
    rating: 9.2,
    amenities: JSON.stringify(["Train access", "Compact suites", "Late checkout"])
  },
  {
    id: "hotel-dubai-creek-lofts",
    name: "Dubai Creek Lofts",
    location: "Dubai Creek",
    nightly_rate: 135,
    currency: "USD",
    rating: 8.6,
    amenities: JSON.stringify(["Creek view", "Airport transfer", "Pool"])
  },
  {
    id: "hotel-kings-cross-nest",
    name: "Kings Cross Nest",
    location: "London Kings Cross",
    nightly_rate: 161,
    currency: "USD",
    rating: 8.5,
    amenities: JSON.stringify(["Station nearby", "Breakfast", "Workspace"])
  },
  {
    id: "hotel-darling-harbor-stay",
    name: "Darling Harbor Stay",
    location: "Sydney Darling Harbour",
    nightly_rate: 149,
    currency: "USD",
    rating: 8.9,
    amenities: JSON.stringify(["Harbor access", "Kitchenette", "Laundry"])
  }
];

const trips = [
  {
    id: "trip-singapore-week",
    title: "Singapore city week",
    destination: "Singapore",
    flight_id: "flight-cmb-sin-01",
    hotel_id: "hotel-harborlight-suites",
    status: "planning",
    total_estimate: 1166,
    currency: "USD"
  },
  {
    id: "trip-lisbon-weekend",
    title: "Lisbon weekend",
    destination: "Lisbon",
    flight_id: "flight-lon-lis-01",
    hotel_id: "hotel-saffron-yard",
    status: "saved",
    total_estimate: 525,
    currency: "USD"
  },
  {
    id: "trip-tokyo-first-timer",
    title: "Tokyo first-timer route",
    destination: "Tokyo",
    flight_id: "flight-cmb-tyo-01",
    hotel_id: "hotel-shibuya-harbor",
    status: "planning",
    total_estimate: 1540,
    currency: "USD"
  },
  {
    id: "trip-singapore-family",
    title: "Singapore family break",
    destination: "Singapore",
    flight_id: "flight-cmb-sin-02",
    hotel_id: "hotel-orchid-house",
    status: "saved",
    total_estimate: 1278,
    currency: "USD"
  },
  {
    id: "trip-dubai-stopover",
    title: "Dubai stopover",
    destination: "Dubai",
    flight_id: "flight-cmb-dxb-01",
    hotel_id: "hotel-dubai-creek-lofts",
    status: "planning",
    total_estimate: 694,
    currency: "USD"
  },
  {
    id: "trip-london-week",
    title: "London rail-and-city week",
    destination: "London",
    flight_id: "flight-dxb-lon-01",
    hotel_id: "hotel-kings-cross-nest",
    status: "saved",
    total_estimate: 1558,
    currency: "USD"
  }
];

if (!existsSync(apiRoot)) {
  mkdirSync(apiRoot, { recursive: true });
}

const db = new DatabaseSync(dbPath);

db.exec("PRAGMA journal_mode = WAL");
db.exec("PRAGMA foreign_keys = ON");

db.exec(`
  DROP TABLE IF EXISTS upgrade_requests;
  DROP TABLE IF EXISTS bookings;
  DROP TABLE IF EXISTS trips;
  DROP TABLE IF EXISTS hotels;
  DROP TABLE IF EXISTS flights;

  CREATE TABLE flights (
    id TEXT PRIMARY KEY,
    from_city TEXT NOT NULL,
    to_city TEXT NOT NULL,
    airline TEXT NOT NULL,
    departure_time TEXT NOT NULL,
    arrival_time TEXT NOT NULL,
    duration TEXT NOT NULL,
    stops INTEGER NOT NULL,
    price REAL NOT NULL,
    currency TEXT NOT NULL,
    cabin TEXT NOT NULL,
    dates TEXT NOT NULL,
    tags TEXT NOT NULL,
    available INTEGER NOT NULL DEFAULT 1
  );

  CREATE TABLE hotels (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    location TEXT NOT NULL,
    nightly_rate REAL NOT NULL,
    currency TEXT NOT NULL,
    rating REAL NOT NULL,
    amenities TEXT NOT NULL
  );

  CREATE TABLE trips (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    destination TEXT NOT NULL,
    flight_id TEXT NOT NULL,
    hotel_id TEXT NOT NULL,
    status TEXT NOT NULL,
    total_estimate REAL NOT NULL,
    currency TEXT NOT NULL,
    FOREIGN KEY (flight_id) REFERENCES flights(id),
    FOREIGN KEY (hotel_id) REFERENCES hotels(id)
  );

  CREATE TABLE bookings (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    username TEXT NOT NULL,
    type TEXT NOT NULL,
    item_id TEXT NOT NULL,
    travelers INTEGER NOT NULL,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL
  );

  CREATE TABLE upgrade_requests (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    username TEXT NOT NULL,
    booking_id TEXT NOT NULL,
    from_flight_id TEXT NOT NULL,
    to_flight_id TEXT,
    price_difference REAL NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (booking_id) REFERENCES bookings(id),
    FOREIGN KEY (from_flight_id) REFERENCES flights(id)
  );
`);

const insertFlight = db.prepare(`
  INSERT INTO flights (
    id,
    from_city,
    to_city,
    airline,
    departure_time,
    arrival_time,
    duration,
    stops,
    price,
    currency,
    cabin,
    dates,
    tags,
    available
  ) VALUES (
    @id,
    @from,
    @to,
    @airline,
    @departure_time,
    @arrival_time,
    @duration,
    @stops,
    @price,
    @currency,
    @cabin,
    @dates,
    @tags,
    @available
  )
`);

const insertHotel = db.prepare(`
  INSERT INTO hotels (
    id,
    name,
    location,
    nightly_rate,
    currency,
    rating,
    amenities
  ) VALUES (
    @id,
    @name,
    @location,
    @nightly_rate,
    @currency,
    @rating,
    @amenities
  )
`);

const insertTrip = db.prepare(`
  INSERT INTO trips (
    id,
    title,
    destination,
    flight_id,
    hotel_id,
    status,
    total_estimate,
    currency
  ) VALUES (
    @id,
    @title,
    @destination,
    @flight_id,
    @hotel_id,
    @status,
    @total_estimate,
    @currency
  )
`);

try {
  db.exec("BEGIN TRANSACTION");

  for (const flight of flights) {
    insertFlight.run(flight);
  }

  for (const hotel of hotels) {
    insertHotel.run(hotel);
  }

  for (const trip of trips) {
    insertTrip.run(trip);
  }

  db.exec("COMMIT");
} catch (error) {
  db.exec("ROLLBACK");
  console.error(`Failed to seed SQLite database at ${dbPath}: ${error.message}`);
  process.exitCode = 1;
} finally {
  db.close();
}

if (process.exitCode !== 1) {
  console.log(`Seeded SQLite database at ${dbPath}`);
}
