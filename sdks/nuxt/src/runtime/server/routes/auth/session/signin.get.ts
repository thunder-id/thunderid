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

import {generateSessionId} from '@thunderid/node';
import {defineEventHandler, getQuery, sendRedirect, setCookie, createError} from 'h3';
import type {H3Event} from 'h3';
import ThunderIDNuxtClient from '../../../ThunderIDNuxtClient';
import {createTempSessionToken, getTempSessionCookieName, getTempSessionCookieOptions} from '../../../utils/session';
import {useRuntimeConfig} from '#imports';

/**
 * GET /api/auth/signin
 *
 * Initiates the OAuth2 authorization code flow with PKCE.
 * Creates a temp session, stores it in a signed JWT cookie,
 * and redirects the user to ThunderID's authorization endpoint.
 *
 * Accepts an optional `returnTo` query parameter to redirect
 * the user to a specific page after sign-in.
 */
export default defineEventHandler(async (event: H3Event) => {
  const client: ThunderIDNuxtClient = ThunderIDNuxtClient.getInstance();
  const config: ReturnType<typeof useRuntimeConfig> = useRuntimeConfig();
  const sessionSecret: string | undefined = config.thunderid?.sessionSecret;

  const query: Record<string, unknown> = getQuery(event) as Record<string, unknown>;
  const returnTo: string | undefined = query['returnTo'] as string | undefined;

  // Validate returnTo is a relative path to prevent open redirect
  const safeReturnTo: string | undefined =
    returnTo && returnTo.startsWith('/') && !returnTo.startsWith('//') ? returnTo : undefined;

  const sessionId: string = generateSessionId();

  // Create temp session JWT and set as cookie (includes returnTo if provided)
  const tempToken: string = await createTempSessionToken(sessionId, sessionSecret, safeReturnTo);
  setCookie(event, getTempSessionCookieName(), tempToken, getTempSessionCookieOptions());

  // Get the authorization URL from the Node SDK
  // The signIn method calls the callback with the authorization URL when no code is provided
  let authorizationUrl: string | null = null;
  await client.signIn(
    (url: string): void => {
      authorizationUrl = url;
    },
    sessionId,
    undefined, // no authorization code (initial redirect)
    undefined, // no session_state
    undefined, // no state
    {},
  );

  if (!authorizationUrl) {
    throw createError({
      statusCode: 500,
      statusMessage: 'Failed to generate authorization URL.',
    });
  }

  return sendRedirect(event, authorizationUrl, 302);
});
