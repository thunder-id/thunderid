// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Shared by the browser and the API routes, so a booking is checked against the travel policy the
// same way on both sides.

export const CABINS = ['Economy', 'Premium Economy', 'Business', 'First Class'] as const;

export type Cabin = (typeof CABINS)[number];

export interface Flight {
  id: string;
  from: string;
  to: string;
  airline: string;
  departure: string;
  arrival: string;
  duration: string;
  stops: number;
  price: number;
  cabin: Cabin;
}

export interface TravelPolicy {
  /** The highest cabin employees may book without approval. */
  maxCabin: Cabin;
  /** The fare employees may book without approval, in USD. */
  maxPrice: number;
  /** How far above maxPrice (in percent) a fare may go and still be booked with approval. */
  approvalMarginPercent: number;
}

export interface Booking {
  id: string;
  reference: string;
  ouId: string;
  flightId: string;
  travelers: number;
  totalPrice: number;
  bookedBySub: string;
  bookedByName: string;
  status: 'confirmed' | 'cancelled';
  createdAt: string;
  flight?: Flight;
}

export type PolicyStatus = 'in-policy' | 'approval-required' | 'out-of-policy';

export interface PolicyEvaluation {
  status: PolicyStatus;
  violations: string[];
}

export const DEFAULT_POLICY: TravelPolicy = { maxCabin: 'Economy', maxPrice: 500, approvalMarginPercent: 20 };

/**
 * Checks a flight against a company's travel policy. A flight within the policy can be booked; one a
 * little over it needs approval (shown, not enforced, in this sample); one well over it is blocked for
 * members.
 */
export const evaluatePolicy = (flight: Flight, policy: TravelPolicy): PolicyEvaluation => {
  const allowedRank = CABINS.indexOf(policy.maxCabin);
  const cabinRank = CABINS.indexOf(flight.cabin);
  const approvalCap = policy.maxPrice * (1 + policy.approvalMarginPercent / 100);
  const violations: string[] = [];

  if (flight.price > policy.maxPrice) {
    violations.push(`Fare $${flight.price} is above the $${policy.maxPrice} limit`);
  }
  if (cabinRank > allowedRank) {
    violations.push(`${flight.cabin} is above the ${policy.maxCabin} cabin limit`);
  }

  if (flight.price > approvalCap || cabinRank > allowedRank + 1) {
    return { status: 'out-of-policy', violations };
  }
  return { status: violations.length > 0 ? 'approval-required' : 'in-policy', violations };
};
