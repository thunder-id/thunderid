// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { ThunderIDProvider } from '@thunderid/nextjs/server';
import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import { ReactNode } from 'react';
import './globals.css';

// Inter, the variable font ThunderID's sign-in pages bundle, so the app and those pages match.
const inter = Inter({ subsets: ['latin'] });

export const dynamic = 'force-dynamic';

export const metadata: Metadata = {
  title: 'Wayfinder Enterprise',
  description: 'Corporate travel for every company, signed in with ThunderID.',
};

const appUrl = process.env.WAYFINDER_APP_URL;
const apiResource = process.env.WAYFINDER_API_RESOURCE;

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body className={inter.className}>
        {/* Reads NEXT_PUBLIC_THUNDERID_* and THUNDERID_* from the environment. Sign-in requests tokens for
            the app's resource server, and the profile comes from the ID token, so the user's token is never
            sent to the ThunderID management API. The admin components reach it through the app's own
            /api/admin routes instead, which pin every call to the signed-in admin's company. */}
        <ThunderIDProvider
          signInOptions={{ resource: apiResource }}
          preferences={{
            user: { fetchUserProfile: false },
            // The ThunderID components use the app's own font, blue and corner radius.
            theme: {
              overrides: {
                borderRadius: { large: '12px', medium: '8px', small: '6px' },
                colors: { primary: { contrastText: '#ffffff', dark: '#1e3a8a', main: '#1d4ed8' } },
                typography: { fontFamily: inter.style.fontFamily },
              },
            },
          }}
          endpoints={{
            connections: `${appUrl}/api/admin/connections`,
            flows: `${appUrl}/api/admin/flows`,
            groups: `${appUrl}/api/admin/groups`,
            roles: `${appUrl}/api/admin/roles`,
            userTypes: `${appUrl}/api/admin/user-types`,
            users: `${appUrl}/api/admin/users`,
          }}
        >
          {children}
        </ThunderIDProvider>
      </body>
    </html>
  );
}
