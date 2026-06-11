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

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || "http://localhost:8787";

async function requestJson(path, options = {}) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    headers: {
      "Content-Type": "application/json",
      ...options.headers
    },
    ...options
  });

  const body = await response.json().catch(() => ({}));

  if (!response.ok) {
    throw new Error(body.error || "API request failed");
  }

  return body;
}

export async function getFlights(searchParams = {}) {
  const params = new URLSearchParams();

  for (const [key, value] of Object.entries(searchParams)) {
    if (value) {
      params.set(key, value);
    }
  }

  const query = params.toString();
  const response = await requestJson(`/api/flights${query ? `?${query}` : ""}`);

  return response.data;
}

export async function getFlight(flightId) {
  const response = await requestJson(`/api/flights/${encodeURIComponent(flightId)}`);

  return response.data;
}

export async function getHotels(searchParams = {}) {
  const params = new URLSearchParams();

  for (const [key, value] of Object.entries(searchParams)) {
    if (value) {
      params.set(key, value);
    }
  }

  const query = params.toString();
  const response = await requestJson(`/api/hotels${query ? `?${query}` : ""}`);

  return response.data;
}

export async function getTrips(searchParams = {}) {
  const params = new URLSearchParams();

  for (const [key, value] of Object.entries(searchParams)) {
    if (value) {
      params.set(key, value);
    }
  }

  const query = params.toString();
  const response = await requestJson(`/api/trips${query ? `?${query}` : ""}`);

  return response.data;
}

export async function getLocations(searchParams = {}) {
  const params = new URLSearchParams();

  for (const [key, value] of Object.entries(searchParams)) {
    if (value) {
      params.set(key, value);
    }
  }

  const query = params.toString();
  const response = await requestJson(`/api/locations${query ? `?${query}` : ""}`);

  return response.data;
}

export async function createBooking(booking, accessToken) {
  const response = await requestJson("/api/bookings", {
    method: "POST",
    headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {},
    body: JSON.stringify(booking)
  });

  return response;
}

export async function getBookedFlights(accessToken) {
  const response = await requestJson("/api/bookings/flights", {
    headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {}
  });

  return response.data;
}
