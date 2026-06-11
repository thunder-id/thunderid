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
import { ChevronLeft, Plane, ShieldCheck } from "lucide-react";
import { getBookedFlights } from "../api";
import { formatPrice, getBookingReference } from "../utils/bookings";

const walletCredentialOffer = import.meta.env.VITE_WALLET_CREDENTIAL_OFFER || "";

export function BookingDetailsPageWithAuth({ bookingId }) {
  const { getAccessToken, isSignedIn, signIn, user } = useThunderID();
  const [booking, setBooking] = useState(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const userKey = user?.sub || user?.username || user?.userName || user?.email || "signed-in";
  const getAccessTokenRef = useRef(getAccessToken);

  useEffect(() => {
    getAccessTokenRef.current = getAccessToken;
  }, [getAccessToken]);

  useEffect(() => {
    let isCurrent = true;

    async function loadBooking() {
      if (!isSignedIn) {
        return;
      }

      setIsLoading(true);
      setError("");

      try {
        const accessToken = getAccessTokenRef.current ? await getAccessTokenRef.current() : null;
        const bookings = await getBookedFlights(accessToken);
        const selectedBooking = bookings.find((item) => String(item.id) === String(bookingId));

        if (isCurrent) {
          setBooking(selectedBooking || null);
          setError(selectedBooking ? "" : "Booking not found.");
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

    loadBooking();

    return () => {
      isCurrent = false;
    };
  }, [bookingId, isSignedIn, userKey]);

  if (!isSignedIn) {
    return (
      <main className="bookings-page">
        <section className="management-empty">
          <div>
            <p className="eyebrow">Booking details</p>
            <h1>Sign in to view this booking.</h1>
            <p>Booking details are available after authentication.</p>
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
          <Link className="back-link" to="/bookings">
            <ChevronLeft size={18} />
            Back to bookings
          </Link>
          <h1>{booking ? `${booking.flight.from} to ${booking.flight.to}` : "Booking"}</h1>
          <p>{booking ? `Reference ${getBookingReference(booking)}` : "Loading booking information"}</p>
        </div>
      </section>

      {error && (
        <div className="api-status api-status--error" role="status">
          {error}
        </div>
      )}

      {isLoading && <p className="empty-state management-message">Loading booking details...</p>}

      {!isLoading && booking && (
        <section className="booking-detail-panel booking-confirmed-panel" aria-label="Booking information">
          <div className="booking-flight-widget">
            <div className="booking-flight-main">
              <div className="booking-detail-topline">
                <span className="booking-status">{booking.status}</span>
                <strong>{booking.flight.airline}</strong>
              </div>
              <div className="itinerary-route">
                <div>
                  <span>{booking.flight.departureTime}</span>
                  <strong>{booking.flight.from}</strong>
                </div>
                <div className="itinerary-line">
                  <Plane size={20} />
                </div>
                <div>
                  <span>{booking.flight.arrivalTime}</span>
                  <strong>{booking.flight.to}</strong>
                </div>
              </div>
              <div className="itinerary-meta">
                <span>{booking.flight.duration}</span>
                <span>{booking.flight.stops === 0 ? "Nonstop" : `${booking.flight.stops} stop`}</span>
                <span>{booking.flight.cabin}</span>
              </div>
            </div>
            <aside className="wallet-qr-panel" aria-label="Wallet QR code">
              <span>Add to wallet</span>
              <div className="wallet-qr-frame">
                <img
                  alt="QR code for adding this booking to a wallet"
                  src={`https://api.qrserver.com/v1/create-qr-code/?size=180x180&data=${encodeURIComponent(walletCredentialOffer)}`}
                />
              </div>
              <p>Scan with a compatible wallet app.</p>
            </aside>
          </div>

          <div className="booking-detail-sections booking-confirmed-sections">
            <section>
              <h2>Trip details</h2>
              <dl>
                <div>
                  <dt>Travel dates</dt>
                  <dd>{booking.flight.dates}</dd>
                </div>
                <div>
                  <dt>Travelers</dt>
                  <dd>
                    {booking.travelers} traveler{booking.travelers === 1 ? "" : "s"}
                  </dd>
                </div>
                <div>
                  <dt>Duration</dt>
                  <dd>{booking.flight.duration}</dd>
                </div>
              </dl>
            </section>
            <section>
              <h2>Booking details</h2>
              <dl>
                <div>
                  <dt>Reference</dt>
                  <dd>{getBookingReference(booking)}</dd>
                </div>
                <div>
                  <dt>Booked on</dt>
                  <dd>{new Date(booking.createdAt).toLocaleDateString()}</dd>
                </div>
                <div>
                  <dt>Status</dt>
                  <dd>{booking.status}</dd>
                </div>
              </dl>
            </section>
            <section>
              <h2>Payment</h2>
              <dl>
                <div>
                  <dt>Total paid</dt>
                  <dd>{formatPrice(booking.flight.currency, booking.flight.price)}</dd>
                </div>
                <div>
                  <dt>Fare type</dt>
                  <dd>{booking.flight.cabin}</dd>
                </div>
              </dl>
            </section>
          </div>

          <section className="travel-insurance-section" aria-label="Travel insurance offer">
            <div className="travel-insurance-icon" aria-hidden="true">
              <ShieldCheck size={26} />
            </div>
            <div>
              <span>Travel protection</span>
              <h2>Add travel insurance before you fly.</h2>
              <p>
                Cover unexpected delays, medical emergencies, and baggage issues for this trip.
              </p>
            </div>
            <button className="travel-insurance-button" type="button">
              Buy insurance
            </button>
          </section>
        </section>
      )}
    </main>
  );
}
