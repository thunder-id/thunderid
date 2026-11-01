// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { randomUUID } from 'node:crypto';
import { NextResponse } from 'next/server';
import { getRequestUser, hasScope } from '@/lib/auth';
import FLIGHTS from '@/lib/flights';
import { addBooking, getPolicy, listBookings } from '@/lib/store';
import { evaluatePolicy } from '@/lib/travel';

const withFlight = <T extends { flightId: string }>(booking: T) => ({
  ...booking,
  flight: FLIGHTS.find((flight) => flight.id === booking.flightId),
});

// ?scope=team lists every booking of the company, for those who manage bookings.
export async function GET(request: Request) {
  const user = await getRequestUser(request);
  if (!user || !hasScope(user, 'booking:view')) {
    return NextResponse.json({ error: 'You cannot view bookings.' }, { status: user ? 403 : 401 });
  }

  const wantsTeam = new URL(request.url).searchParams.get('scope') === 'team';
  if (wantsTeam && !hasScope(user, 'booking:manage')) {
    return NextResponse.json({ error: 'You cannot view your team’s bookings.' }, { status: 403 });
  }

  return NextResponse.json(listBookings(user.ouId, wantsTeam ? undefined : user.sub).map(withFlight));
}

export async function POST(request: Request) {
  const user = await getRequestUser(request);
  if (!user || !hasScope(user, 'booking:create')) {
    return NextResponse.json({ error: 'You cannot book flights.' }, { status: user ? 403 : 401 });
  }

  const { flightId, travelers = 1 } = (await request.json().catch(() => null)) ?? {};
  const flight = FLIGHTS.find((candidate) => candidate.id === flightId);
  if (!flight || !Number.isInteger(travelers) || travelers < 1 || travelers > 9) {
    return NextResponse.json({ error: 'Choose a flight and between 1 and 9 travelers.' }, { status: 400 });
  }

  // The policy is enforced here, not only in the browser. Admins may book outside it.
  const evaluation = evaluatePolicy(flight, getPolicy(user.ouId));
  if (evaluation.status === 'out-of-policy' && !hasScope(user, 'booking:manage')) {
    return NextResponse.json(
      { error: `This flight is outside your company’s travel policy. ${evaluation.violations.join('. ')}.` },
      { status: 403 },
    );
  }

  const booking = addBooking({
    id: randomUUID(),
    reference: randomUUID().slice(0, 6).toUpperCase(),
    ouId: user.ouId,
    flightId,
    travelers,
    totalPrice: flight.price * travelers,
    bookedBySub: user.sub,
    bookedByName: user.username ?? user.email ?? user.sub,
    status: 'confirmed',
    createdAt: new Date().toISOString(),
  });

  return NextResponse.json(withFlight(booking), { status: 201 });
}
