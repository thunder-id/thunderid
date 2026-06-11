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

import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { ChevronLeft, Plane } from "lucide-react";
import { getFlight } from "../api";
import { formatPrice } from "../utils/bookings";
import { buildFlightPaymentPath } from "../utils/routes";

export function FlightDetailsPage({ criteria, flightId }) {
  const location = useLocation();
  const navigate = useNavigate();
  const [flight, setFlight] = useState(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState("");

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
  }, [flightId]);

  return (
    <main className="bookings-page">
      <section className="management-header">
        <div>
          <Link className="back-link" to={`/results${location.search}`}>
            <ChevronLeft size={18} />
            Back to results
          </Link>
          <p className="eyebrow">Flight details</p>
          <h1>{flight ? `${flight.from} to ${flight.to}` : "Flight details"}</h1>
          <p>{flight ? `${flight.airline} · ${flight.dates}` : "Review your selected flight"}</p>
        </div>
      </section>

      {error && (
        <div className="api-status api-status--error" role="status">
          {error}
        </div>
      )}

      {isLoading && <p className="empty-state management-message">Loading flight details...</p>}

      {!isLoading && flight && (
        <section className="booking-detail-panel flight-review-panel" aria-label="Flight information">
          <div className="flight-review-layout">
            <div className="flight-review-main">
              <div className="booking-itinerary-card">
                <div className="booking-detail-topline">
                  <span className="booking-status">Selected</span>
                  <strong>{flight.airline}</strong>
                </div>
                <div className="itinerary-route">
                  <div>
                    <span>{flight.departureTime}</span>
                    <strong>{flight.from}</strong>
                  </div>
                  <div className="itinerary-line">
                    <Plane size={20} />
                  </div>
                  <div>
                    <span>{flight.arrivalTime}</span>
                    <strong>{flight.to}</strong>
                  </div>
                </div>
                <div className="itinerary-meta">
                  <span>{flight.duration}</span>
                  <span>{flight.stops === 0 ? "Nonstop" : `${flight.stops} stop`}</span>
                  <span>{flight.cabin}</span>
                  <span>{criteria.travelers || "1 Adult, Economy"}</span>
                </div>
              </div>

              <div className="booking-detail-sections flight-review-sections">
                <section>
                  <h2>Schedule</h2>
                  <dl>
                    <div>
                      <dt>Travel dates</dt>
                      <dd>{flight.dates}</dd>
                    </div>
                    <div>
                      <dt>Duration</dt>
                      <dd>{flight.duration}</dd>
                    </div>
                  </dl>
                </section>
                <section>
                  <h2>Fare</h2>
                  <dl>
                    <div>
                      <dt>Cabin</dt>
                      <dd>{flight.cabin}</dd>
                    </div>
                    <div>
                      <dt>Stops</dt>
                      <dd>{flight.stops === 0 ? "Nonstop" : `${flight.stops} stop`}</dd>
                    </div>
                  </dl>
                </section>
              </div>
            </div>

            <aside className="booking-receipt-card flight-review-summary" aria-label="Flight price">
              <div className="flight-review-summary-content">
                <span>Total</span>
                <strong>{formatPrice(flight.currency, flight.price)}</strong>
                <p>Includes taxes and charges.</p>
                <dl>
                  <div>
                    <dt>Airline</dt>
                    <dd>{flight.airline}</dd>
                  </div>
                  <div>
                    <dt>Travelers</dt>
                    <dd>{criteria.travelers || "1 Adult, Economy"}</dd>
                  </div>
                </dl>
              </div>
              <button
                className="wallet-button flight-review-confirm"
                type="button"
                onClick={() => navigate(buildFlightPaymentPath(flight.id, criteria))}
              >
                Confirm
              </button>
            </aside>
          </div>
        </section>
      )}
    </main>
  );
}
