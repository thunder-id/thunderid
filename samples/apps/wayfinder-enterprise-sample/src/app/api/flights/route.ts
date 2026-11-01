// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { NextResponse } from 'next/server';
import { getRequestUser, hasScope } from '@/lib/auth';
import FLIGHTS from '@/lib/flights';

export async function GET(request: Request) {
  const user = await getRequestUser(request);
  if (!user) {
    return NextResponse.json({ error: 'Sign in to search flights.' }, { status: 401 });
  }
  if (!hasScope(user, 'booking:view')) {
    return NextResponse.json({ error: 'You cannot view flights.' }, { status: 403 });
  }

  const search = new URL(request.url).searchParams;
  const from = search.get('from');
  const to = search.get('to');

  return NextResponse.json(
    FLIGHTS.filter((flight) => (!from || flight.from === from) && (!to || flight.to === to)),
  );
}
