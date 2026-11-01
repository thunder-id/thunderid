// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { NextResponse } from 'next/server';
import { getRequestUser, hasScope } from '@/lib/auth';
import { cancelBooking } from '@/lib/store';

// Members cancel their own bookings; those who manage bookings can cancel any in the company.
export async function DELETE(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const user = await getRequestUser(request);
  if (!user || !hasScope(user, 'booking:create')) {
    return NextResponse.json({ error: 'You cannot cancel bookings.' }, { status: user ? 403 : 401 });
  }

  const { id } = await params;
  const booking = cancelBooking(user.ouId, id, hasScope(user, 'booking:manage') ? undefined : user.sub);
  if (!booking) {
    return NextResponse.json({ error: 'Booking not found.' }, { status: 404 });
  }
  return NextResponse.json(booking);
}
