// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

'use client';

import { SignOutButton } from '@thunderid/nextjs';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { ReactNode } from 'react';

const ADMIN_NAV = [
  { href: '/workspace/bookings', label: 'Flight booking' },
  { href: '/workspace/policy', label: 'Travel policy' },
  { href: '/workspace/team', label: 'Team and roles' },
  { href: '/workspace/enterprise-sso', label: 'Enterprise SSO' },
];

interface WorkspaceShellProps {
  children: ReactNode;
  companyName: string;
  displayName: string;
  email?: string;
  isAdmin: boolean;
}

/** Admins get a sidebar with the admin pages; members get a top bar and the booking page only. */
export default function WorkspaceShell({ children, companyName, displayName, email, isAdmin }: WorkspaceShellProps) {
  const pathname = usePathname();
  const initials = displayName.slice(0, 2).toUpperCase();

  const account = (
    <div className="account">
      <span className="avatar" aria-hidden="true">
        {initials}
      </span>
      <span className="account-text">
        <strong>{displayName}</strong>
        {email && email !== displayName && <span>{email}</span>}
      </span>
      <SignOutButton>
        {({ isLoading, signOut }) => (
          <button type="button" className="button ghost" disabled={isLoading} onClick={() => void signOut()}>
            Sign out
          </button>
        )}
      </SignOutButton>
    </div>
  );

  if (!isAdmin) {
    return (
      <div className="member-shell">
        <header className="member-header">
          <span className="brand">
            <img className="brand-logo" src="/wayfinder-logo.svg" alt="Wayfinder Enterprise" width={232} height={48} />
            <span className="company-tag">{companyName}</span>
          </span>
          {account}
        </header>
        <main className="workspace">{children}</main>
      </div>
    );
  }

  return (
    <div className="admin-shell">
      <aside className="sidebar">
        <img
          className="brand-logo"
          src="/wayfinder-logo-inverse.svg"
          alt="Wayfinder Enterprise"
          width={232}
          height={48}
        />
        <p className="sidebar-company">{companyName}</p>
        <nav aria-label="Workspace">
          {ADMIN_NAV.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={pathname.startsWith(item.href) ? 'nav-link active' : 'nav-link'}
              aria-current={pathname.startsWith(item.href) ? 'page' : undefined}
            >
              {item.label}
            </Link>
          ))}
        </nav>
      </aside>
      <div className="admin-main">
        <header className="admin-header">
          <span className="role-badge">Admin</span>
          {account}
        </header>
        <main className="workspace">{children}</main>
      </div>
    </div>
  );
}
