/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {defineEventHandler, deleteCookie} from 'h3';
import type {H3Event} from 'h3';
import ThunderIDNuxtClient from '../../../ThunderIDNuxtClient';
import {verifyAndRehydrateSession} from '../../../utils/serverSession';
import {
  getSessionCookieName,
  getSessionCookieOptions,
  getTempSessionCookieName,
  getTempSessionCookieOptions,
} from '../../../utils/session';
import {useRuntimeConfig} from '#imports';

/**
 * POST /api/auth/signout
 *
 * Signs the user out by:
 * 1. Getting the sign-out URL from ThunderID (for RP-Initiated Logout)
 * 2. Clearing all session cookies
 * 3. Returning `{ redirectUrl }` for the client to navigate to
 *
 * Using POST instead of GET prevents CSRF-based forced sign-outs.
 */
export default defineEventHandler(async (event: H3Event): Promise<{redirectUrl: string}> => {
  const config: ReturnType<typeof useRuntimeConfig> = useRuntimeConfig();
  const sessionSecret: string | undefined = config.thunderid?.sessionSecret;
  const publicConfig: typeof config.public.thunderid = config.public.thunderid;
  const fallbackUrl: string = (publicConfig as any).afterSignOutUrl || '/';

  const clearCookies = (): void => {
    deleteCookie(event, getSessionCookieName(), getSessionCookieOptions());
    deleteCookie(event, getTempSessionCookieName(), getTempSessionCookieOptions());
  };

  // Decode + rehydrate so the legacy client can read the id_token from the
  // in-memory store when building the RP-Initiated Logout URL.
  const session: Awaited<ReturnType<typeof verifyAndRehydrateSession>> = await verifyAndRehydrateSession(
    event,
    sessionSecret,
  );
  if (!session) {
    clearCookies();
    return {redirectUrl: fallbackUrl};
  }

  try {
    const client: ThunderIDNuxtClient = ThunderIDNuxtClient.getInstance();
    const signOutUrl: string = await client.signOut(session.sessionId);

    clearCookies();

    return {redirectUrl: signOutUrl || fallbackUrl};
  } catch (err: any) {
    // eslint-disable-next-line no-console
    console.error('[thunderid] Sign-out error:', err?.message || err);
    clearCookies();
    return {redirectUrl: fallbackUrl};
  }
});
