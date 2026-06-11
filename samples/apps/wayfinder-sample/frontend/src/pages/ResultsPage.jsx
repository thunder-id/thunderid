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
import { useThunderID } from "@thunderid/react";
import { useNavigate } from "react-router-dom";
import { SearchPanel } from "../components/SearchPanel";
import {
  createBooking,
  getBookedFlights,
  getFlights,
  getHotels,
  getTrips
} from "../api";
import { formatPrice, isSameFlight } from "../utils/bookings";
import { buildFlightDetailsPath } from "../utils/routes";

function BookingButton({ bookingState, children, onClick }) {
  const isBooking = bookingState === "booking";
  const isConfirmed = bookingState === "confirmed";

  return (
    <button
      className={`card-action ${isConfirmed ? "card-action--confirmed" : ""}`}
      type="button"
      disabled={isBooking || isConfirmed}
      onClick={onClick}
    >
      {isBooking ? "Booking..." : isConfirmed ? "Booked" : children}
    </button>
  );
}

function ResultCard({ bookingState, category, item, onBook, onSelectFlight }) {
  if (category === "hotels") {
    return (
      <article className="result-card">
        <div>
          <p className="result-label">Hotel · Rating {item.rating}</p>
          <h2>{item.name}</h2>
          <p>{item.location}</p>
          <div className="result-tags">
            {item.amenities?.map((amenity) => (
              <span key={amenity}>{amenity}</span>
            ))}
          </div>
        </div>
        <div className="result-side">
          <strong>{formatPrice(item.currency, item.nightlyRate)}</strong>
          <span>per night</span>
          <BookingButton bookingState={bookingState} onClick={() => onBook("hotel", item.id)}>
            Reserve
          </BookingButton>
        </div>
      </article>
    );
  }

  if (category === "trips") {
    return (
      <article className="result-card">
        <div>
          <p className="result-label">Trip · {item.status}</p>
          <h2>{item.title}</h2>
          <p>{item.destination}</p>
        </div>
        <div className="result-side">
          <strong>{formatPrice(item.currency, item.totalEstimate)}</strong>
          <span>estimate</span>
          <BookingButton bookingState={bookingState} onClick={() => onBook("trip", item.id)}>
            Book trip
          </BookingButton>
        </div>
      </article>
    );
  }

  return (
    <article className="result-card">
      <div>
        <p className="result-label">
          {item.airline} · {item.stops === 0 ? "Nonstop" : `${item.stops} stop`}
        </p>
        <h2>{item.from} to {item.to}</h2>
        <p>
          {item.departureTime} - {item.arrivalTime} · {item.duration} · {item.dates}
        </p>
        <div className="result-tags">
          {item.tags?.map((tag) => (
            <span key={tag}>{tag}</span>
          ))}
        </div>
      </div>
      <div className="result-side">
        <strong>{formatPrice(item.currency, item.price)}</strong>
        <span>{item.cabin}</span>
        <BookingButton bookingState={bookingState} onClick={() => onSelectFlight(item.id)}>
          Book flight
        </BookingButton>
      </div>
    </article>
  );
}

export function ResultsPage({ criteria, getAccessToken, locations, onSearch }) {
  const navigate = useNavigate();
  const [results, setResults] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState("");
  const [bookingStates, setBookingStates] = useState({});

  useEffect(() => {
    let isCurrent = true;

    async function loadResults() {
      setIsLoading(true);
      setError("");
      setBookingStates({});

      try {
        let data;

        if (criteria.category === "hotels") {
          data = await getHotels({ location: criteria.to });
        } else if (criteria.category === "trips") {
          data = await getTrips({ destination: criteria.to });
        } else {
          data = await getFlights({
            from: criteria.from,
            to: criteria.to
          });
        }

        if (isCurrent) {
          setResults(data);

          if (criteria.category === "flights" && getAccessToken) {
            try {
              const accessToken = await getAccessToken();
              const bookedFlights = await getBookedFlights(accessToken);
              const nextBookingStates = {};

              for (const result of data) {
                if (bookedFlights.some((booking) => isSameFlight(result, booking.flight))) {
                  nextBookingStates[result.id] = "confirmed";
                }
              }

              if (isCurrent) {
                setBookingStates(nextBookingStates);
              }
            } catch {
              // Results should remain usable even if existing bookings cannot be checked.
            }
          }
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

    loadResults();

    return () => {
      isCurrent = false;
    };
  }, [criteria, getAccessToken]);

  async function handleBooking(type, itemId) {
    setError("");
    setBookingStates((current) => ({
      ...current,
      [itemId]: "booking"
    }));

    try {
      const accessToken = getAccessToken ? await getAccessToken() : null;

      await createBooking({
        type,
        itemId,
        travelers: Number.parseInt(criteria.travelers, 10) || 1
      }, accessToken);

      setBookingStates((current) => ({
        ...current,
        [itemId]: "confirmed"
      }));
    } catch (requestError) {
      setBookingStates((current) => ({
        ...current,
        [itemId]: requestError.message.includes("already exists") ? "confirmed" : "idle"
      }));
      setError(requestError.message);
    }
  }

  function handleFlightSelection(itemId) {
    navigate(buildFlightDetailsPath(itemId, criteria));
  }

  const title =
    criteria.category === "hotels"
      ? `Hotels in ${criteria.to || "your destination"}`
      : criteria.category === "trips"
        ? `Trips to ${criteria.to || "your destination"}`
        : `${criteria.from || "Anywhere"} to ${criteria.to || "anywhere"}`;

  return (
    <main>
      <section className="results-hero">
        <div>
          <p className="eyebrow">Search results</p>
          <h1>{title}</h1>
          <p>
            {criteria.dates || "Flexible dates"} · {criteria.travelers || "Any travelers"}
          </p>
        </div>
        <SearchPanel
          initialCriteria={criteria}
          key={`${criteria.category}-${criteria.from}-${criteria.to}-${criteria.dates}-${criteria.travelers}`}
          locations={locations}
          onSearch={onSearch}
        />
      </section>

      {error && (
        <div className="api-status api-status--error" role="status">
          {error}
        </div>
      )}

      <section className="results-section" aria-label="Search results">
        {isLoading && <p className="empty-state">Loading results...</p>}
        {!isLoading && results.length === 0 && (
          <p className="empty-state">No results matched this search.</p>
        )}
        {!isLoading &&
          results.map((item) => (
            <ResultCard
              category={criteria.category}
              item={item}
              key={item.id}
              bookingState={bookingStates[item.id] || "idle"}
              onBook={handleBooking}
              onSelectFlight={handleFlightSelection}
            />
          ))}
      </section>
    </main>
  );
}

export function ResultsPageWithAuth(props) {
  const { getAccessToken } = useThunderID();

  return <ResultsPage {...props} getAccessToken={getAccessToken} />;
}
