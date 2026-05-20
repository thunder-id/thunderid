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

import {CookieConfig} from '@thunderid/node';
import type {IdToken, TokenResponse} from '@thunderid/node';
import {setCookie} from 'h3';
import type {H3Event} from 'h3';
import {SignJWT, jwtVerify} from 'jose';
import type {ThunderIDSessionPayload} from '../../types';

const DEFAULT_EXPIRY_SECONDS: number = 3600;

/**
 * Get the signing secret from environment or runtime config.
 */
function getSecret(sessionSecret?: string): Uint8Array {
  const secret: string | undefined = sessionSecret || process.env['THUNDERID_SESSION_SECRET'];

  if (!secret) {
    if (process.env['NODE_ENV'] === 'production') {
      throw new Error(
        '[thunderid] THUNDERID_SESSION_SECRET environment variable is required in production. ' +
          'Set it to a secure random string of at least 32 characters.',
      );
    }
    // eslint-disable-next-line no-console
    console.warn(
      '[thunderid] Using default session secret for development. Set THUNDERID_SESSION_SECRET for production.',
    );
    return new TextEncoder().encode('thunderid-dev-secret-not-for-production');
  }

  return new TextEncoder().encode(secret);
}

/**
 * Create a signed JWT session token.
 */
export async function createSessionToken(
  params: {
    accessToken: string;
    /** Unix timestamp (seconds) when the access token expires. */
    accessTokenExpiresAt?: number;
    expirySeconds?: number;
    /** Raw ID token string. */
    idToken?: string;
    organizationId?: string;
    /** Refresh token for silent re-auth. */
    refreshToken?: string;
    scopes: string;
    sessionId: string;
    userId: string;
  },
  sessionSecret?: string,
): Promise<string> {
  const secret: Uint8Array = getSecret(sessionSecret);

  return new SignJWT({
    accessToken: params.accessToken,
    accessTokenExpiresAt: params.accessTokenExpiresAt,
    idToken: params.idToken,
    organizationId: params.organizationId,
    refreshToken: params.refreshToken,
    scopes: params.scopes,
    sessionId: params.sessionId,
    type: 'session',
  } as Omit<ThunderIDSessionPayload, 'sub' | 'iat' | 'exp'>)
    .setProtectedHeader({alg: 'HS256'})
    .setSubject(params.userId)
    .setIssuedAt()
    .setExpirationTime(Date.now() / 1000 + (params.expirySeconds ?? DEFAULT_EXPIRY_SECONDS))
    .sign(secret);
}

/**
 * Create a signed JWT temp session token (used during OAuth flow).
 */
export async function createTempSessionToken(
  sessionId: string,
  sessionSecret?: string,
  returnTo?: string,
): Promise<string> {
  const secret: Uint8Array = getSecret(sessionSecret);

  const payload: Record<string, unknown> = {
    sessionId,
    type: 'temp',
  };

  if (returnTo) {
    payload['returnTo'] = returnTo;
  }

  return new SignJWT(payload).setProtectedHeader({alg: 'HS256'}).setIssuedAt().setExpirationTime('15m').sign(secret);
}

/**
 * Verify and decode a session JWT.
 */
export async function verifySessionToken(token: string, sessionSecret?: string): Promise<ThunderIDSessionPayload> {
  const secret: Uint8Array = getSecret(sessionSecret);
  const {payload} = await jwtVerify(token, secret);
  return payload as ThunderIDSessionPayload;
}

/**
 * Verify and decode a temp session JWT.
 */
export async function verifyTempSessionToken(
  token: string,
  sessionSecret?: string,
): Promise<{returnTo?: string; sessionId: string}> {
  const secret: Uint8Array = getSecret(sessionSecret);
  const {payload} = await jwtVerify(token, secret);

  if (payload['type'] !== 'temp') {
    throw new Error('Invalid token type: expected temp session');
  }

  return {
    returnTo: payload['returnTo'] as string | undefined,
    sessionId: payload['sessionId'] as string,
  };
}

/**
 * Session cookie name.
 */
export function getSessionCookieName(): string {
  return CookieConfig.SESSION_COOKIE_NAME;
}

/**
 * Temp session cookie name.
 */
export function getTempSessionCookieName(): string {
  return CookieConfig.TEMP_SESSION_COOKIE_NAME;
}

/**
 * Session cookie options.
 */
type SessionCookieOptions = {
  httpOnly: boolean;
  maxAge: number;
  path: string;
  sameSite: 'lax';
  secure: boolean;
};

export function getSessionCookieOptions(): SessionCookieOptions {
  return {
    httpOnly: true,
    maxAge: DEFAULT_EXPIRY_SECONDS,
    path: '/',
    sameSite: 'lax' as const,
    secure: process.env['NODE_ENV'] === 'production',
  };
}

/**
 * Temp session cookie options (15 min TTL).
 */
export function getTempSessionCookieOptions(): SessionCookieOptions {
  return {
    httpOnly: true,
    maxAge: 15 * 60,
    path: '/',
    sameSite: 'lax' as const,
    secure: process.env['NODE_ENV'] === 'production',
  };
}

/**
 * Decode a token response into a signed session JWT and write it as the
 * session cookie on the H3 event.
 *
 * Extracted from the inline blocks in `callback.get.ts` and
 * `organizations/switch.post.ts` so that all three callers (callback.get,
 * switch.post, and the new `signin.post`) share one implementation.
 */
export async function issueSessionCookie(
  event: H3Event,
  sessionId: string,
  tokenResponse: TokenResponse,
  sessionSecret?: string,
): Promise<void> {
  // Lazy-import to avoid circular dep: session.ts → ThunderIDNuxtClient → session.ts
  const {default: ThunderIDNuxtClient} = await import('../ThunderIDNuxtClient');
  const client: ReturnType<typeof ThunderIDNuxtClient.getInstance> = ThunderIDNuxtClient.getInstance();

  const idToken: IdToken = await client.getDecodedIdToken(sessionId, tokenResponse.idToken);

  const userId: string = (idToken.sub || sessionId) as string;
  const organizationId: string | undefined = (idToken['user_org'] || idToken['organization_id']) as string | undefined;
  const expiresInSeconds: number = parseInt(tokenResponse.expiresIn ?? '3600', 10);
  const accessTokenExpiresAt: number =
    Math.floor(Date.now() / 1000) + (Number.isFinite(expiresInSeconds) ? expiresInSeconds : 3600);

  const sessionToken: string = await createSessionToken(
    {
      accessToken: tokenResponse.accessToken,
      accessTokenExpiresAt,
      idToken: tokenResponse.idToken || undefined,
      organizationId,
      refreshToken: tokenResponse.refreshToken || undefined,
      scopes: tokenResponse.scope || '',
      sessionId,
      userId,
    },
    sessionSecret,
  );

  setCookie(event, getSessionCookieName(), sessionToken, getSessionCookieOptions());
}
