// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import BookingsDashboard from '@/components/BookingsDashboard';
import { getSessionUser, hasScope } from '@/lib/auth';

export default async function BookingsPage() {
  const user = await getSessionUser();

  return (
    <>
      <div className="page-heading">
        <p className="eyebrow">Flight booking</p>
        <h1>Book within your travel policy</h1>
      </div>
      <BookingsDashboard canManageBookings={user ? hasScope(user, 'booking:manage') : false} />
    </>
  );
}
