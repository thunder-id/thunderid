// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { redirect } from 'next/navigation';
import { ReactNode } from 'react';
import WorkspaceShell from '@/components/WorkspaceShell';
import { getSessionUser } from '@/lib/auth';

export default async function WorkspaceLayout({ children }: { children: ReactNode }) {
  const user = await getSessionUser();
  if (!user) {
    redirect('/');
  }

  return (
    <WorkspaceShell
      isAdmin={user.isAdmin}
      displayName={user.username ?? user.email ?? 'Signed in'}
      email={user.email}
      companyName={user.ouName ?? user.ouHandle ?? 'Your company'}
    >
      {children}
    </WorkspaceShell>
  );
}
