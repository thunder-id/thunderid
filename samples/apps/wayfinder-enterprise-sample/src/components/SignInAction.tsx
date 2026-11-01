// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

'use client';

import { SignInButton } from '@thunderid/nextjs';
import { ReactNode } from 'react';

interface SignInActionProps {
  children: ReactNode;
  variant: 'primary' | 'secondary';
}

/**
 * Starts sign-in at ThunderID. Its sign-in page offers a password, enterprise SSO, or registering a
 * company. Registering runs the app's sign-up flow inside the sign-in, so the new admin comes back to
 * the app already signed in. Every landing page action uses this, whatever its label.
 */
export default function SignInAction({ children, variant }: SignInActionProps) {
  return (
    <SignInButton>
      {({ isLoading, signIn }) => (
        <button type="button" className={`button ${variant}`} disabled={isLoading} onClick={() => void signIn()}>
          {children}
        </button>
      )}
    </SignInButton>
  );
}
