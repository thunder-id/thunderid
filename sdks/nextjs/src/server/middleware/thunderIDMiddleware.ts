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

import {NextRequest, NextResponse} from 'next/server';
import {REFRESH_BUFFER_SECONDS} from '../../constants/sessionConstants';
import {ThunderIDNextConfig} from '../../models/config';
import decorateConfigWithNextEnv from '../../utils/decorateConfigWithNextEnv';
import handleRefreshToken from '../../utils/handleRefreshToken';
import SessionManager, {SessionTokenPayload} from '../../utils/SessionManager';
import {getSessionFromRequest, getSessionIdFromRequest} from '../../utils/sessionUtils';

export type ThunderIDMiddlewareOptions = Partial<ThunderIDNextConfig>;

export interface ThunderIDMiddlewareContext {
  /** Get the session payload from JWT session if available */
  getSession: () => Promise<SessionTokenPayload | undefined>;
  /** Get the session ID from the current request */
  getSessionId: () => string | undefined;
  /** Check if the current request has a valid ThunderID session */
  isSignedIn: () => boolean;
  /**
   * Protect a route by redirecting unauthenticated users.
   * Redirect URL fallback order:
   * 1. options.redirect
   * 2. resolvedOptions.signInUrl
   * 3. resolvedOptions.defaultRedirect
   * 4. referer (if from same origin)
   * If none are available, falls back to '/'.
   */
  protectRoute: (routeOptions?: {redirect?: string}) => Promise<NextResponse | void>;
}

type ThunderIDMiddlewareHandler = (
  thunderid: ThunderIDMiddlewareContext,
  req: NextRequest,
) => Promise<NextResponse | void> | NextResponse | void;

/**
 * Removes a named cookie from a raw Cookie header string.
 */
const removeCookieFromHeader = (cookieHeader: string, name: string): string =>
  cookieHeader
    .split(';')
    .map((p: string) => p.trim())
    .filter((p: string) => {
      const eqIdx: number = p.indexOf('=');
      const partName: string = eqIdx === -1 ? p : p.slice(0, eqIdx).trim();
      return partName !== name;
    })
    .join('; ');

/**
 * Replaces the value of a named cookie inside a raw Cookie header string.
 * If the cookie does not already appear in the header it is appended.
 */
const replaceCookieInHeader = (cookieHeader: string, name: string, value: string): string => {
  const parts: string[] = cookieHeader
    .split(';')
    .map((p: string) => p.trim())
    .filter(Boolean);

  let found = false;
  const updated: string[] = parts.map((part: string) => {
    const eqIdx: number = part.indexOf('=');
    const partName: string = eqIdx === -1 ? part : part.slice(0, eqIdx).trim();
    if (partName === name) {
      found = true;
      return `${name}=${value}`;
    }
    return part;
  });

  if (!found) {
    updated.push(`${name}=${value}`);
  }

  return updated.join('; ');
};

/**
 * ThunderID middleware that integrates authentication into your Next.js application.
 * Similar to Clerk's clerkMiddleware pattern.
 *
 * Proactively refreshes the access token when it is within REFRESH_BUFFER_SECONDS of
 * expiry so that Server Components always receive a fresh session. The refresh also
 * recovers expired tokens as long as a refresh token is present.
 *
 * The updated session cookie is written to:
 *   - The response  → browser stores the new cookie for subsequent requests.
 *   - The forwarded request headers → the same-request Server Component render sees
 *     the fresh token immediately without waiting for the next navigation.
 *
 * Token refresh requires baseUrl, clientId, and clientSecret. These are resolved from
 * the options argument first, then from the standard ThunderID environment variables
 * (NEXT_PUBLIC_THUNDERID_BASE_URL, NEXT_PUBLIC_THUNDERID_CLIENT_ID,
 * THUNDERID_CLIENT_SECRET). If none are available the refresh step is skipped silently.
 *
 * @param handler - Optional handler function to customize middleware behavior
 * @param options - Configuration options for the middleware
 * @returns Next.js middleware function
 *
 * @example
 * ```typescript
 * // middleware.ts - Basic usage (config read from env vars automatically)
 * import { thunderIDMiddleware } from '@thunderid/nextjs';
 *
 * export default thunderIDMiddleware();
 *
 * export const config = {
 *   matcher: ['/((?!_next/static|_next/image|favicon.ico).*)'],
 * };
 * ```
 *
 * @example
 * ```typescript
 * // With route protection
 * import { thunderIDMiddleware, createRouteMatcher } from '@thunderid/nextjs';
 *
 * const isProtectedRoute = createRouteMatcher(['/dashboard(.*)']);
 *
 * export default thunderIDMiddleware(async (thunderid, req) => {
 *   if (isProtectedRoute(req)) {
 *     await thunderid.protectRoute();
 *   }
 * });
 * ```
 */
const thunderIDMiddleware =
  (
    handler?: ThunderIDMiddlewareHandler,
    options?: ThunderIDMiddlewareOptions | ((req: NextRequest) => ThunderIDMiddlewareOptions),
  ): ((request: NextRequest) => Promise<NextResponse>) =>
  async (request: NextRequest): Promise<NextResponse> => {
    const resolvedOptions: ThunderIDMiddlewareOptions =
      typeof options === 'function' ? options(request) : options || {};

    // Resolve full config from passed options + environment variable fallbacks.
    const resolvedConfig: ThunderIDNextConfig = decorateConfigWithNextEnv(resolvedOptions as ThunderIDNextConfig);

    // ── OAuth callback detection ──────────────────────────────────────────────
    const url: URL = new URL(request.url);
    const hasCallbackParams: boolean = url.searchParams.has('code') && url.searchParams.has('state');

    let isValidOAuthCallback = false;
    if (hasCallbackParams && !url.searchParams.has('error')) {
      const tempSessionToken: string | undefined = request.cookies.get(
        SessionManager.getTempSessionCookieName(),
      )?.value;
      if (tempSessionToken) {
        try {
          await SessionManager.verifyTempSession(tempSessionToken);
          isValidOAuthCallback = true;
        } catch {
          isValidOAuthCallback = false;
        }
      }
    }

    // ── Session resolution ────────────────────────────────────────────────────
    // Step 1: Attempt to get a fully verified (signature + expiry) session.
    const verifiedSession: SessionTokenPayload | undefined = await getSessionFromRequest(request);

    // Step 2: If no verified session exists, verify the raw cookie's signature
    // without enforcing expiry. This allows the middleware to recover from an
    // expired access token as long as the JWT is authentic and a refresh token
    // is present. Skipping the signature check here would let a tampered cookie
    // drive identity-confusion attacks since handleRefreshToken reuses `sub`,
    // `sessionId`, and `organizationId` from the input payload when minting the
    // new session JWT.
    let expiredSession: SessionTokenPayload | undefined;
    if (!verifiedSession) {
      const rawToken: string | undefined = request.cookies.get(SessionManager.getSessionCookieName())?.value;
      if (rawToken) {
        try {
          const decoded: SessionTokenPayload = await SessionManager.verifySessionTokenForRefresh(rawToken);
          if (decoded.refreshToken) {
            expiredSession = decoded;
          }
        } catch {
          // Forged, tampered, wrong type, or malformed — ignore.
        }
      }
    }

    // ── Token refresh ─────────────────────────────────────────────────────────
    const now: number = Math.floor(Date.now() / 1000);
    const candidateSession: SessionTokenPayload | undefined = verifiedSession ?? expiredSession;

    // Config is required to call the token endpoint.
    const hasRefreshConfig = !!(resolvedConfig.baseUrl && resolvedConfig.clientId && resolvedConfig.clientSecret);

    // Refresh when:
    //   a) Token is verified but within the proactive buffer window, OR
    //   b) Token has already expired but a refresh token is available.
    const needsRefresh: boolean =
      !isValidOAuthCallback &&
      hasRefreshConfig &&
      !!candidateSession?.refreshToken &&
      ((!!verifiedSession && verifiedSession.exp <= now + REFRESH_BUFFER_SECONDS) || !!expiredSession);

    let activeSession: SessionTokenPayload | undefined = verifiedSession;
    let refreshCookieUpdate: {expiry: number; token: string} | undefined;

    if (needsRefresh && candidateSession) {
      try {
        const {newSessionToken, sessionCookieExpiryTime} = await handleRefreshToken(candidateSession, {
          baseUrl: resolvedConfig.baseUrl!,
          clientId: resolvedConfig.clientId!,
          clientSecret: resolvedConfig.clientSecret!,
          sessionCookie: resolvedConfig.sessionCookie,
        });
        // Verify the newly minted token so activeSession reflects fresh claims.
        activeSession = await SessionManager.verifySessionToken(newSessionToken);
        refreshCookieUpdate = {expiry: sessionCookieExpiryTime, token: newSessionToken};
      } catch {
        // Refresh failed — clear the irrecoverable session.
        activeSession = undefined;
      }
    }

    // ── Session cleanup detection ─────────────────────────────────────────────
    // Mark stale cookies for deletion when the session is irrecoverable. Skipped
    // during OAuth callbacks where a session cookie may not exist yet.
    const rawSessionCookie: string | undefined = request.cookies.get(SessionManager.getSessionCookieName())?.value;

    let shouldClearCookie = false;

    if (!isValidOAuthCallback && rawSessionCookie && !activeSession && !refreshCookieUpdate) {
      // A cookie was present but all resolution paths (verify, decode, refresh)
      // failed — the session is dead and cannot be recovered.
      shouldClearCookie = true;
    }

    const sessionId: string | undefined = activeSession?.sessionId ?? (await getSessionIdFromRequest(request));
    const isAuthenticated = !!activeSession;

    // ── Middleware context ────────────────────────────────────────────────────
    const thunderid: ThunderIDMiddlewareContext = {
      getSession: async (): Promise<SessionTokenPayload | undefined> => activeSession,
      getSessionId: (): string | undefined => sessionId,
      isSignedIn: (): boolean => isAuthenticated,
      protectRoute: async (routeOptions?: {redirect?: string}): Promise<NextResponse | void> => {
        // Skip during a valid OAuth callback to avoid redirecting before the
        // callback action has had a chance to complete.
        if (isValidOAuthCallback) {
          return undefined;
        }

        if (!isAuthenticated) {
          const referer: string | null = request.headers.get('referer');
          let fallbackRedirect = '/';

          if (referer) {
            try {
              const refererUrl: URL = new URL(referer);
              const requestUrl: URL = new URL(request.url);
              if (refererUrl.origin === requestUrl.origin) {
                fallbackRedirect = refererUrl.pathname + refererUrl.search;
              }
            } catch {
              // Invalid referer — ignore.
            }
          }

          const redirectUrl: string = routeOptions?.redirect ?? resolvedConfig.signInUrl! ?? fallbackRedirect;

          return NextResponse.redirect(new URL(redirectUrl, request.url));
        }

        return undefined;
      },
    };

    // ── Handler ───────────────────────────────────────────────────────────────
    const handlerResponse: NextResponse | void = handler ? await handler(thunderid, request) : undefined;

    // ── Build final response ──────────────────────────────────────────────────
    if (shouldClearCookie) {
      const cookieName: string = SessionManager.getSessionCookieName();

      if (handlerResponse) {
        // Handler returned a response (e.g. a redirect from protectRoute).
        // Attach the deletion so the browser discards the stale cookie.
        handlerResponse.cookies.delete(cookieName);
        return handlerResponse;
      }

      // Pass-through: strip the dead cookie from the forwarded request headers
      // so the same-request Server Component render sees no session at all.
      const requestHeaders: Headers = new Headers(request.headers);
      requestHeaders.set('cookie', removeCookieFromHeader(request.headers.get('cookie') ?? '', cookieName));
      const cleanResponse: NextResponse = NextResponse.next({request: {headers: requestHeaders}});
      cleanResponse.cookies.delete(cookieName);
      return cleanResponse;
    }

    if (!refreshCookieUpdate) {
      return handlerResponse ?? NextResponse.next();
    }

    // A token refresh occurred — the new session cookie must be applied to:
    //   1. The HTTP response so the browser stores the updated cookie.
    //   2. The forwarded request headers so the same-request Server Component
    //      render reads the fresh session token instead of the expired one.
    const cookieName: string = SessionManager.getSessionCookieName();
    const cookieOptions: ReturnType<typeof SessionManager.getSessionCookieOptions> =
      SessionManager.getSessionCookieOptions(refreshCookieUpdate.expiry);

    if (handlerResponse) {
      // Handler returned a response (e.g. a redirect from protectRoute).
      // Attach the refresh cookie so the browser receives it even on redirects.
      handlerResponse.cookies.set(cookieName, refreshCookieUpdate.token, cookieOptions);
      return handlerResponse;
    }

    // Default pass-through: update both the response cookie and the request
    // Cookie header so the downstream Server Component render is not stale.
    const requestHeaders: Headers = new Headers(request.headers);
    const updatedCookieHeader: string = replaceCookieInHeader(
      request.headers.get('cookie') ?? '',
      cookieName,
      refreshCookieUpdate.token,
    );
    requestHeaders.set('cookie', updatedCookieHeader);

    const response: NextResponse = NextResponse.next({request: {headers: requestHeaders}});
    response.cookies.set(cookieName, refreshCookieUpdate.token, cookieOptions);
    return response;
  };

export default thunderIDMiddleware;
