// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { NextResponse } from 'next/server';
import { getRequestUser, hasScope } from '@/lib/auth';
import { getPolicy, savePolicy } from '@/lib/store';
import { CABINS, Cabin } from '@/lib/travel';

export async function GET(request: Request) {
  const user = await getRequestUser(request);
  if (!user || !hasScope(user, 'policy:view')) {
    return NextResponse.json({ error: 'You cannot view the travel policy.' }, { status: user ? 403 : 401 });
  }
  return NextResponse.json(getPolicy(user.ouId));
}

export async function PUT(request: Request) {
  const user = await getRequestUser(request);
  if (!user || !hasScope(user, 'policy:manage')) {
    return NextResponse.json({ error: 'You cannot change the travel policy.' }, { status: user ? 403 : 401 });
  }

  const { maxCabin, maxPrice, approvalMarginPercent } = (await request.json().catch(() => null)) ?? {};
  if (
    !CABINS.includes(maxCabin) ||
    !Number.isFinite(maxPrice) ||
    maxPrice <= 0 ||
    !Number.isFinite(approvalMarginPercent) ||
    approvalMarginPercent < 0 ||
    approvalMarginPercent > 100
  ) {
    return NextResponse.json({ error: 'Enter a cabin, a fare above 0 and a margin from 0 to 100.' }, { status: 400 });
  }

  return NextResponse.json(savePolicy(user.ouId, { maxCabin: maxCabin as Cabin, maxPrice, approvalMarginPercent }));
}
