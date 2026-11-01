// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import 'server-only';

import { redirect } from 'next/navigation';
import { getSessionUser, WayfinderUser } from './auth';

/** The signed-in admin. Sends anyone else to the booking page. */
const requireAdmin = async (): Promise<WayfinderUser> => {
  const user = await getSessionUser();
  if (!user?.isAdmin) {
    redirect('/workspace/bookings');
  }
  return user;
};

export default requireAdmin;
