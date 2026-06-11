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
import { Link, useNavigate } from "react-router-dom";
import { ChevronLeft, CreditCard } from "lucide-react";
import { createBooking, getBookedFlights, getFlight } from "../api";
import { formatPrice, isSameFlight } from "../utils/bookings";
import { buildFlightDetailsPath } from "../utils/routes";

export function PaymentPageWithAuth({ criteria, flightId }) {
  const navigate = useNavigate();
  const { getAccessToken, isSignedIn, signIn, user } = useThunderID();
  const [flight, setFlight] = useState(null);
  const [isLoading, setIsLoading] = useState(true);
  const [paymentState, setPaymentState] = useState("idle");
  const [error, setError] = useState("");
  const userKey = user?.sub || user?.username || user?.userName || user?.email || "signed-in";
  const getAccessTokenRef = useRef(getAccessToken);

  useEffect(() => {
    getAccessTokenRef.current = getAccessToken;
  }, [getAccessToken]);

  useEffect(() => {
    let isCurrent = true;

    async function loadFlight() {
      setIsLoading(true);
      setError("");

      try {
        const data = await getFlight(flightId);

        if (isCurrent) {
          setFlight(data);
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

    loadFlight();

    return () => {
      isCurrent = false;
    };
  }, [flightId, userKey]);

  async function handlePayment() {
    if (!flight) {
      return;
    }

    setPaymentState("paying");
    setError("");

    try {
      const accessToken = getAccessTokenRef.current ? await getAccessTokenRef.current() : null;
      const booking = await createBooking({
        type: "flight",
        itemId: flight.id,
        travelers: Number.parseInt(criteria.travelers, 10) || 1
      }, accessToken);

      navigate(`/bookings/${encodeURIComponent(booking.id)}`);
    } catch (requestError) {
      try {
        if (requestError.message.includes("already exists")) {
          const accessToken = getAccessTokenRef.current ? await getAccessTokenRef.current() : null;
          const bookings = await getBookedFlights(accessToken);
          const existingBooking = bookings.find((booking) => isSameFlight(flight, booking.flight));

          if (existingBooking) {
            navigate(`/bookings/${encodeURIComponent(existingBooking.id)}`);
            return;
          }
        }
      } catch {
        // Keep the original payment error visible.
      }

      setPaymentState("idle");
      setError(requestError.message);
    }
  }

  if (!isSignedIn) {
    return (
      <main className="bookings-page">
        <section className="management-empty">
          <div>
            <p className="eyebrow">Payment</p>
            <h1>Sign in to complete payment.</h1>
            <p>Your flight will be booked only after payment is completed.</p>
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
          <Link className="back-link" to={buildFlightDetailsPath(flightId, criteria)}>
            <ChevronLeft size={18} />
            Back to flight
          </Link>
          <p className="eyebrow">Payment</p>
          <h1>Complete payment</h1>
          <p>{flight ? `${flight.from} to ${flight.to} · ${flight.airline}` : "Preparing checkout"}</p>
        </div>
      </section>

      {error && (
        <div className="api-status api-status--error" role="status">
          {error}
        </div>
      )}

      {isLoading && <p className="empty-state management-message">Loading payment details...</p>}

      {!isLoading && flight && (
        <section className="payment-panel" aria-label="Payment details">
          <div className="payment-form-card">
            <div className="booking-detail-topline">
              <span className="booking-status">Secure payment</span>
              <CreditCard size={22} />
            </div>
            <label className="payment-field">
              <span>Card number</span>
              <input readOnly value="4242 4242 4242 4242" aria-label="Card number" />
            </label>
            <div className="payment-field-grid">
              <label className="payment-field">
                <span>Expiry</span>
                <input readOnly value="12 / 30" aria-label="Expiry" />
              </label>
              <label className="payment-field">
                <span>CVC</span>
                <input readOnly value="123" aria-label="CVC" />
              </label>
            </div>
            <button
              className="search-button standalone-button"
              type="button"
              disabled={paymentState === "paying"}
              onClick={handlePayment}
            >
              {paymentState === "paying" ? "Processing..." : "Pay and confirm booking"}
            </button>
          </div>

          <aside className="booking-receipt-card" aria-label="Payment summary">
            <span>Total due</span>
            <strong>{formatPrice(flight.currency, flight.price)}</strong>
            <p>{criteria.travelers || "1 Adult, Economy"} · {flight.dates}</p>
          </aside>
        </section>
      )}
    </main>
  );
}
