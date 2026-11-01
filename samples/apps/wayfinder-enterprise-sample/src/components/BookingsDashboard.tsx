// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

'use client';

import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';
import { Booking, evaluatePolicy, Flight, PolicyStatus, TravelPolicy } from '@/lib/travel';

const STATUS_LABELS: Record<PolicyStatus, string> = {
  'in-policy': 'In policy',
  'approval-required': 'Needs approval',
  'out-of-policy': 'Out of policy',
};

const STATUS_ORDER: PolicyStatus[] = ['in-policy', 'approval-required', 'out-of-policy'];

const request = async <T,>(url: string, init?: RequestInit): Promise<T> => {
  const response = await fetch(url, { ...init, headers: { 'Content-Type': 'application/json' } });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(body.error ?? 'Something went wrong. Please try again.');
  }
  return body;
};

export default function BookingsDashboard({ canManageBookings }: { canManageBookings: boolean }) {
  const [allFlights, setAllFlights] = useState<Flight[]>([]);
  const [policy, setPolicy] = useState<TravelPolicy | null>(null);
  const [bookings, setBookings] = useState<Booking[]>([]);
  const [showTeam, setShowTeam] = useState(false);
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [search, setSearch] = useState({ from: '', to: '' });
  const [busyFlightId, setBusyFlightId] = useState<string | null>(null);
  const [message, setMessage] = useState<{ kind: 'error' | 'success'; text: string } | null>(null);

  const loadBookings = useCallback(
    () =>
      request<Booking[]>(showTeam ? '/api/bookings?scope=team' : '/api/bookings')
        .then(setBookings)
        .catch((error: Error) => setMessage({ kind: 'error', text: error.message })),
    [showTeam],
  );

  useEffect(() => {
    Promise.all([request<Flight[]>('/api/flights'), request<TravelPolicy>('/api/policy')])
      .then(([flights, travelPolicy]) => {
        setAllFlights(flights);
        setPolicy(travelPolicy);
      })
      .catch((error: Error) => setMessage({ kind: 'error', text: error.message }));
  }, []);

  useEffect(() => {
    void loadBookings();
  }, [loadBookings]);

  const origins = useMemo(() => [...new Set(allFlights.map((flight) => flight.from))].sort(), [allFlights]);
  const destinations = useMemo(
    () => [...new Set(allFlights.filter((f) => !from || f.from === from).map((f) => f.to))].sort(),
    [allFlights, from],
  );

  const results = useMemo(() => {
    if (!policy) {
      return [];
    }
    return allFlights
      .filter((flight) => (!search.from || flight.from === search.from) && (!search.to || flight.to === search.to))
      .map((flight) => ({ flight, evaluation: evaluatePolicy(flight, policy) }))
      .sort(
        (a, b) =>
          STATUS_ORDER.indexOf(a.evaluation.status) - STATUS_ORDER.indexOf(b.evaluation.status) ||
          a.flight.price - b.flight.price,
      );
  }, [allFlights, policy, search]);

  const handleSearch = (event: FormEvent) => {
    event.preventDefault();
    setSearch({ from, to });
  };

  const book = async (flight: Flight) => {
    setBusyFlightId(flight.id);
    setMessage(null);
    try {
      const booking = await request<Booking>('/api/bookings', {
        method: 'POST',
        body: JSON.stringify({ flightId: flight.id, travelers: 1 }),
      });
      setMessage({ kind: 'success', text: `Booked ${flight.from} to ${flight.to}. Reference ${booking.reference}.` });
      await loadBookings();
    } catch (error) {
      setMessage({ kind: 'error', text: (error as Error).message });
    } finally {
      setBusyFlightId(null);
    }
  };

  const cancel = async (booking: Booking) => {
    setMessage(null);
    try {
      await request(`/api/bookings/${booking.id}`, { method: 'DELETE' });
      await loadBookings();
    } catch (error) {
      setMessage({ kind: 'error', text: (error as Error).message });
    }
  };

  return (
    <div className="stack">
      {policy && (
        <section className="policy-banner" aria-label="Travel policy">
          <strong>Your travel policy</strong>
          <span>Up to {policy.maxCabin}</span>
          <span>Fares up to ${policy.maxPrice}</span>
          <span>Up to {policy.approvalMarginPercent}% over with approval</span>
        </section>
      )}

      {message && (
        <p className={message.kind === 'error' ? 'alert error' : 'alert success'} role={message.kind === 'error' ? 'alert' : 'status'}>
          {message.text}
        </p>
      )}

      <section className="panel">
        <form className="search-form" onSubmit={handleSearch}>
          <label>
            From
            <select value={from} onChange={(event) => { setFrom(event.target.value); setTo(''); }}>
              <option value="">Anywhere</option>
              {origins.map((city) => (
                <option key={city}>{city}</option>
              ))}
            </select>
          </label>
          <label>
            To
            <select value={to} onChange={(event) => setTo(event.target.value)}>
              <option value="">Anywhere</option>
              {destinations.map((city) => (
                <option key={city}>{city}</option>
              ))}
            </select>
          </label>
          <button type="submit" className="button primary">
            Search flights
          </button>
        </form>

        <div className="flight-grid">
          {results.map(({ flight, evaluation }) => {
            const isBlocked = evaluation.status === 'out-of-policy' && !canManageBookings;
            return (
              <article key={flight.id} className={`flight-card ${evaluation.status}`}>
                <div className="flight-route">
                  <strong>
                    {flight.from} → {flight.to}
                  </strong>
                  <span className={`badge ${evaluation.status}`}>{STATUS_LABELS[evaluation.status]}</span>
                </div>
                <p className="flight-meta">
                  {flight.airline} · {flight.departure} to {flight.arrival} · {flight.duration} ·{' '}
                  {flight.stops === 0 ? 'Non-stop' : `${flight.stops} stop`}
                </p>
                <p className="flight-meta">{flight.cabin}</p>
                {evaluation.violations.length > 0 && (
                  <ul className="violations">
                    {evaluation.violations.map((violation) => (
                      <li key={violation}>{violation}</li>
                    ))}
                  </ul>
                )}
                <div className="flight-footer">
                  <span className="price">${flight.price}</span>
                  <button
                    type="button"
                    className="button primary"
                    disabled={isBlocked || busyFlightId !== null}
                    title={isBlocked ? 'Outside your company’s travel policy' : undefined}
                    onClick={() => void book(flight)}
                  >
                    {busyFlightId === flight.id ? 'Booking…' : 'Book'}
                  </button>
                </div>
              </article>
            );
          })}
        </div>
      </section>

      <section className="panel">
        <div className="panel-heading">
          <h2>{showTeam ? 'Team bookings' : 'My bookings'}</h2>
          {canManageBookings && (
            <div className="segmented" role="group" aria-label="Bookings to show">
              <button type="button" aria-pressed={!showTeam} onClick={() => setShowTeam(false)}>
                Mine
              </button>
              <button type="button" aria-pressed={showTeam} onClick={() => setShowTeam(true)}>
                Team
              </button>
            </div>
          )}
        </div>
        {bookings.length === 0 ? (
          <p className="muted">No bookings yet.</p>
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>Reference</th>
                <th>Flight</th>
                {showTeam && <th>Booked by</th>}
                <th>Fare</th>
                <th>Status</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              {bookings.map((booking) => (
                <tr key={booking.id}>
                  <td>{booking.reference}</td>
                  <td>
                    {booking.flight ? `${booking.flight.from} → ${booking.flight.to}, ${booking.flight.airline}` : booking.flightId}
                  </td>
                  {showTeam && <td>{booking.bookedByName}</td>}
                  <td>${booking.totalPrice}</td>
                  <td>
                    <span className={`badge ${booking.status === 'confirmed' ? 'in-policy' : 'muted'}`}>
                      {booking.status === 'confirmed' ? 'Confirmed' : 'Cancelled'}
                    </span>
                  </td>
                  <td>
                    {booking.status === 'confirmed' && (
                      <button type="button" className="button ghost small" onClick={() => void cancel(booking)}>
                        Cancel
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}
