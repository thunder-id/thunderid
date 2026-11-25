// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueryClient} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {useThunderID} from '@thunderid/react';
import {useEffect, useRef} from 'react';

interface FailedCall {
  response?: {status?: number};
  config?: {url?: string};
}

/** The URL a call was refused at for want of a valid session, when it was. */
function refusedAt(error: unknown): string | undefined {
  const failed = error as FailedCall | null | undefined;
  return failed?.response?.status === 401 ? failed.config?.url : undefined;
}

/**
 * Whether a URL is the control plane's: the same origin, and within the path it is served under, by whole
 * path segments, so another host or path that merely starts with the same text is not taken for it.
 */
function isServerUrl(url: string, serverUrl: string): boolean {
  try {
    const called = new URL(url);
    const server = new URL(serverUrl);
    if (called.origin !== server.origin) return false;
    const base = server.pathname.replace(/\/+$/, '');
    return base === '' || called.pathname === base || called.pathname.startsWith(`${base}/`);
  } catch {
    return false;
  }
}

/**
 * Sends the console to sign in again when the control plane refuses a call for want of a valid
 * session: the session has ended, and the pages would otherwise load empty. A refusal from another
 * service is that service's to explain, as the page that read it does. A sign-in that cannot start, such
 * as when the sign-in service cannot be reached, is logged; the next refusal, such as a retry on the
 * page, starts it again. One sign-in starts at a time, however many calls are refused together.
 */
export default function SignInOnUnauthorized(): null {
  const queryClient = useQueryClient();
  const {signIn} = useThunderID();
  const {getServerUrl} = useConfig();
  const logger = useLogger();
  // Held across re-subscriptions: concurrent sign-ins would overwrite each other's authorization state.
  const signingIn = useRef(false);

  useEffect(() => {
    const serverUrl = getServerUrl();
    const seen = (error: unknown): void => {
      const url = refusedAt(error);
      if (!url || !serverUrl || !isServerUrl(url, serverUrl) || signingIn.current) return;
      signingIn.current = true;
      signIn()
        .catch((failure: unknown) => logger.error('Signing in again failed', {error: failure}))
        .finally(() => {
          signingIn.current = false;
        });
    };
    const queries = queryClient.getQueryCache().subscribe((event) => {
      if (event.type === 'updated' && event.action.type === 'error') seen(event.action.error);
    });
    const mutations = queryClient.getMutationCache().subscribe((event) => {
      if (event.type === 'updated' && event.action.type === 'error') seen(event.action.error);
    });
    return () => {
      queries();
      mutations();
    };
  }, [queryClient, signIn, getServerUrl, logger]);

  return null;
}
