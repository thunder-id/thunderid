// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

'use client';

import { FormEvent, useEffect, useState } from 'react';
import { Cabin, CABINS, TravelPolicy } from '@/lib/travel';

export default function PolicyEditor() {
  const [policy, setPolicy] = useState<TravelPolicy | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [message, setMessage] = useState<{ kind: 'error' | 'success'; text: string } | null>(null);

  useEffect(() => {
    fetch('/api/policy')
      .then((response) => {
        if (!response.ok) {
          throw new Error(response.statusText);
        }
        return response.json();
      })
      .then(setPolicy)
      .catch(() => setMessage({ kind: 'error', text: 'Could not load the travel policy.' }));
  }, []);

  const save = async (event: FormEvent) => {
    event.preventDefault();
    setIsSaving(true);
    setMessage(null);
    try {
      const response = await fetch('/api/policy', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(policy),
      });
      const body = await response.json();
      if (response.ok) {
        setPolicy(body);
        setMessage({ kind: 'success', text: 'Travel policy saved. Employees see it the next time they search.' });
      } else {
        setMessage({ kind: 'error', text: body.error });
      }
    } catch {
      setMessage({ kind: 'error', text: 'Could not save the travel policy.' });
    } finally {
      setIsSaving(false);
    }
  };

  if (!policy) {
    return <p className="muted">Loading the travel policy…</p>;
  }

  return (
    <form className="panel rule-builder" onSubmit={save}>
      {message && (
        <p className={message.kind === 'error' ? 'alert error' : 'alert success'} role={message.kind === 'error' ? 'alert' : 'status'}>
          {message.text}
        </p>
      )}
      <div className="rule-row">
        <div>
          <h2>Cabin</h2>
          <p className="muted">The highest cabin employees can book without approval.</p>
        </div>
        <select
          aria-label="Highest cabin"
          value={policy.maxCabin}
          onChange={(event) => setPolicy({ ...policy, maxCabin: event.target.value as Cabin })}
        >
          {CABINS.map((cabin) => (
            <option key={cabin}>{cabin}</option>
          ))}
        </select>
      </div>
      <div className="rule-row">
        <div>
          <h2>Fare limit</h2>
          <p className="muted">The highest fare, in USD, employees can book without approval.</p>
        </div>
        <input
          aria-label="Fare limit in USD"
          type="number"
          min={1}
          value={policy.maxPrice}
          onChange={(event) => setPolicy({ ...policy, maxPrice: Number(event.target.value) })}
        />
      </div>
      <div className="rule-row">
        <div>
          <h2>Approval margin</h2>
          <p className="muted">How far over the limits a booking may go with approval. Beyond it, members cannot book.</p>
        </div>
        <input
          aria-label="Approval margin in percent"
          type="number"
          min={0}
          max={100}
          value={policy.approvalMarginPercent}
          onChange={(event) => setPolicy({ ...policy, approvalMarginPercent: Number(event.target.value) })}
        />
      </div>
      <div className="form-actions">
        <button type="submit" className="button primary" disabled={isSaving}>
          {isSaving ? 'Saving…' : 'Save policy'}
        </button>
      </div>
    </form>
  );
}
