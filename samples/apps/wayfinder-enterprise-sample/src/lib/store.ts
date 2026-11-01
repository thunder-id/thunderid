// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import 'server-only';

import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { Booking, DEFAULT_POLICY, TravelPolicy } from './travel';

// Bookings and travel policies live in a JSON file so the sample needs no database. Every record is
// keyed by the company's organization unit, taken from the caller's verified token, so one company
// never sees another's data.

interface Data {
  bookings: Booking[];
  policies: Record<string, TravelPolicy>;
}

const DATA_DIR = join(process.cwd(), 'data');
const DATA_FILE = join(DATA_DIR, 'wayfinder.json');

const read = (): Data =>
  existsSync(DATA_FILE) ? JSON.parse(readFileSync(DATA_FILE, 'utf8')) : { bookings: [], policies: {} };

const write = (data: Data): void => {
  mkdirSync(DATA_DIR, { recursive: true });
  writeFileSync(DATA_FILE, JSON.stringify(data, null, 2));
};

export const getPolicy = (ouId: string): TravelPolicy => read().policies[ouId] ?? DEFAULT_POLICY;

export const savePolicy = (ouId: string, policy: TravelPolicy): TravelPolicy => {
  const data = read();
  data.policies[ouId] = policy;
  write(data);
  return policy;
};

export const listBookings = (ouId: string, bookedBySub?: string): Booking[] =>
  read()
    .bookings.filter((booking) => booking.ouId === ouId && (!bookedBySub || booking.bookedBySub === bookedBySub))
    .sort((a, b) => b.createdAt.localeCompare(a.createdAt));

export const addBooking = (booking: Booking): Booking => {
  const data = read();
  data.bookings.push(booking);
  write(data);
  return booking;
};

export const cancelBooking = (ouId: string, id: string, bookedBySub?: string): Booking | undefined => {
  const data = read();
  const booking = data.bookings.find(
    (candidate) => candidate.id === id && candidate.ouId === ouId && (!bookedBySub || candidate.bookedBySub === bookedBySub),
  );
  if (booking) {
    booking.status = 'cancelled';
    write(data);
  }
  return booking;
};
