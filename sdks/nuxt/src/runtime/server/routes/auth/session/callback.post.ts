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

import type {TokenResponse} from '@thunderid/node';
import {defineEventHandler, readBody, getCookie, deleteCookie, createError} from 'h3';
import type {H3Event} from 'h3';
import ThunderIDNuxtClient from '../../../ThunderIDNuxtClient';
import {
  issueSessionCookie,
  verifyTempSessionToken,
  getTempSessionCookieName,
  getTempSessionCookieOptions,
} from '../../../utils/session';
import {useRuntimeConfig} from '#imports';

function isTokenResponse(value: unknown): value is TokenResponse {
  return (
    typeof value === 'object' &&
    value !== null &&
    ('accessToken' in value || 'idToken' in value || 'refreshToken' in value)
  );
}

/**
 * POST /api/auth/callback
 *
 * Exchanges an authorization code for tokens and issues a session cookie.
 * Called by the client-side `ThunderIDCallback` component after the IDP
 * redirects back with `?code=...&state=...`.
 *
 * Request body:
 * - `code` — authorization code from the IDP redirect
 * - `state` — state parameter from the redirect
 * - `sessionState` — session_state parameter from the redirect (optional)
 *
 * Response shape (success):
 * ```json
 * { "redirectUrl": "/dashboard", "success": true }
 * ```
 * Response shape (error):
 * ```json
 * { "success": false, "error": "..." }
 * ```
 */
export default defineEventHandler(async (event: H3Event) => {
  const config: ReturnType<typeof useRuntimeConfig> = useRuntimeConfig();
  const sessionSecret: string | undefined = config.thunderid?.sessionSecret;
  const afterSignInUrl: string = ((config.public.thunderid as any)?.afterSignInUrl as string | undefined) || '/';

  // ── Parse request body ────────────────────────────────────────────────────
  const body: {code?: string; sessionState?: string; state?: string} = await readBody(event);
  const {code, state, sessionState} = body ?? {};

  if (!code) {
    throw createError({statusCode: 400, statusMessage: 'Missing required parameter: code'});
  }

  // ── Resolve sessionId from temp session cookie ────────────────────────────
  const tempCookie: string | undefined = getCookie(event, getTempSessionCookieName());
  if (!tempCookie) {
    throw createError({statusCode: 400, statusMessage: 'No active auth session found. Please restart sign-in.'});
  }

  let sessionId: string;
  try {
    const tempSession: Awaited<ReturnType<typeof verifyTempSessionToken>> = await verifyTempSessionToken(
      tempCookie,
      sessionSecret,
    );
    sessionId = tempSession.sessionId;
  } catch {
    throw createError({statusCode: 400, statusMessage: 'Auth session expired or invalid. Please restart sign-in.'});
  }

  // ── Exchange code for tokens ──────────────────────────────────────────────
  const client: ThunderIDNuxtClient = ThunderIDNuxtClient.getInstance();

  let tokenResponse: unknown;
  try {
    tokenResponse = await client.signIn({code, session_state: sessionState, state}, {}, sessionId);
  } catch (err: any) {
    return {error: err?.message ?? String(err), success: false};
  }

  if (!isTokenResponse(tokenResponse)) {
    return {error: 'Invalid token response from Identity Provider.', success: false};
  }

  // ── Issue session cookie ──────────────────────────────────────────────────
  try {
    await issueSessionCookie(event, sessionId, tokenResponse, sessionSecret);
    deleteCookie(event, getTempSessionCookieName(), getTempSessionCookieOptions());
  } catch (err: any) {
    return {error: `Failed to establish session: ${err?.message ?? String(err)}`, success: false};
  }

  return {redirectUrl: afterSignInUrl, success: true};
});
