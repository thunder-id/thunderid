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

import { useEffect, useRef, useState } from "react";
import { useThunderID } from "@thunderid/react";
import { Link } from "react-router-dom";
import { getBookedFlights } from "../api";
import { formatPrice, getBookingReference } from "../utils/bookings";

export function BookingsPageWithAuth() {
  const { getAccessToken, isSignedIn, signIn, user } = useThunderID();
  const [bookings, setBookings] = useState([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const userKey = user?.sub || user?.username || user?.userName || user?.email || "signed-in";
  const getAccessTokenRef = useRef(getAccessToken);

  useEffect(() => {
    getAccessTokenRef.current = getAccessToken;
  }, [getAccessToken]);

  useEffect(() => {
    let isCurrent = true;

    async function loadBookings() {
      if (!isSignedIn) {
        return;
      }

      setIsLoading(true);
      setError("");

      try {
        const accessToken = getAccessTokenRef.current ? await getAccessTokenRef.current() : null;
        const data = await getBookedFlights(accessToken);

        if (isCurrent) {
          setBookings(data);
        }
      } catch (requestError) {
        if (isCurrent) {
          setError(requestError.message);
        }
      } finally {
        if (isCurrent) {
          setIsLoading(false);
        }
      }
    }

    loadBookings();

    return () => {
      isCurrent = false;
    };
  }, [isSignedIn, userKey]);

  if (!isSignedIn) {
    return (
      <main className="bookings-page">
        <section className="management-empty">
          <div>
            <p className="eyebrow">Bookings</p>
            <h1>Sign in to manage your bookings.</h1>
            <p>View confirmed trips, booking status, passenger count, and flight details.</p>
          </div>
          <button className="dashboard-action dashboard-action--secondary" type="button" onClick={() => signIn({ acr_values: "urn:thunder:auth:user" })}>
            Sign in
          </button>
        </section>
      </main>
    );
  }

  return (
    <main className="bookings-page">
      <section className="management-header">
        <div>
          <p className="eyebrow">Management</p>
          <h1>Bookings</h1>
        </div>
      </section>

      {error && (
        <div className="api-status api-status--error" role="status">
          {error}
        </div>
      )}

      <section className="management-panel" aria-label="Booked flights">
        {isLoading && <p className="empty-state management-message">Loading booked flights...</p>}
        {!isLoading && bookings.length === 0 && (
          <div className="management-empty-state">
            <h2>No bookings yet</h2>
            <p>Your confirmed flights will appear here after booking.</p>
            <Link className="dashboard-action dashboard-action--secondary" to="/flights#search">
              Start searching
            </Link>
          </div>
        )}
        {!isLoading &&
          bookings.length > 0 && (
            <div className="booking-table-heading" aria-hidden="true">
              <span>Route</span>
              <span>Reference</span>
              <span>Schedule</span>
              <span>Travelers</span>
              <span>Total</span>
            </div>
          )}
        {!isLoading &&
          bookings.length > 0 &&
          bookings.map((booking) => (
            <Link className="booking-row" to={`/bookings/${booking.id}`} key={booking.id}>
              <div className="booking-route">
                <span className="booking-status">{booking.status}</span>
                <strong>{booking.flight.from} to {booking.flight.to}</strong>
                <small>Booked {new Date(booking.createdAt).toLocaleDateString()}</small>
              </div>
              <div className="booking-cell">
                <strong>{getBookingReference(booking)}</strong>
                <span>Booking reference</span>
              </div>
              <div className="booking-cell">
                <strong>{booking.flight.departureTime} - {booking.flight.arrivalTime}</strong>
                <span>{booking.flight.duration} · {booking.flight.dates}</span>
              </div>
              <div className="booking-cell">
                <strong>
                  {booking.travelers} traveler{booking.travelers === 1 ? "" : "s"}
                </strong>
                <span>{booking.flight.cabin}</span>
              </div>
              <div className="booking-price">
                <strong>{formatPrice(booking.flight.currency, booking.flight.price)}</strong>
              </div>
            </Link>
          ))}
      </section>
    </main>
  );
}
