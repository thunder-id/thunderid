// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import 'server-only';

import { createPublicKey, JsonWebKey, verify } from 'node:crypto';
import { thunderid } from '@thunderid/nextjs/server';
import config from './config';

/** The signed-in user, as the app's access token describes them. */
export interface WayfinderUser {
  sub: string;
  username?: string;
  email?: string;
  ouId: string;
  ouHandle?: string;
  ouName?: string;
  roles: string[];
  scopes: string[];
  isAdmin: boolean;
}

interface Jwk extends JsonWebKey {
  kid?: string;
}

let jwksCache: { keys: Jwk[]; fetchedAt: number } | undefined;
let lastJwksFetch = 0;

const getSigningKey = async (kid: string): Promise<Jwk | undefined> => {
  const isStale = !jwksCache || Date.now() - jwksCache.fetchedAt > 10 * 60 * 1000;
  const isUnknownKid = !jwksCache?.keys.some((key) => key.kid === kid);
  // A token with an unknown key ID refetches the keys at most every 30 seconds, so callers cannot force a fetch per request.
  if (isStale || (isUnknownKid && Date.now() - lastJwksFetch > 30_000)) {
    lastJwksFetch = Date.now();
    const response = await fetch(`${config.thunderIdUrl}/oauth2/jwks`, { cache: 'no-store' });
    const body = response.ok ? await response.json() : undefined;
    // A failed fetch keeps the keys already cached.
    if (Array.isArray(body?.keys)) {
      jwksCache = { keys: body.keys, fetchedAt: Date.now() };
    }
  }
  return jwksCache?.keys.find((key) => key.kid === kid);
};

const decodeSegment = (segment: string): Record<string, unknown> =>
  JSON.parse(Buffer.from(segment, 'base64url').toString('utf8'));

/**
 * Verifies an access token ThunderID issued to this app for its resource server and returns the user it
 * names. Returns null for any token that is not one.
 */
export const verifyAccessToken = async (token: string): Promise<WayfinderUser | null> => {
  const [headerSegment, payloadSegment, signature] = token.split('.');
  if (!headerSegment || !payloadSegment || !signature) {
    return null;
  }

  try {
    const header = decodeSegment(headerSegment);
    if (header.alg !== 'RS256' || typeof header.kid !== 'string') {
      return null;
    }

    const jwk = await getSigningKey(header.kid);
    const isValidSignature =
      jwk !== undefined &&
      verify(
        'RSA-SHA256',
        Buffer.from(`${headerSegment}.${payloadSegment}`),
        createPublicKey({ format: 'jwk', key: jwk }),
        Buffer.from(signature, 'base64url'),
      );
    if (!isValidSignature) {
      return null;
    }

    const claims = decodeSegment(payloadSegment);
    const audiences = Array.isArray(claims.aud) ? claims.aud : [claims.aud];
    if (
      claims.iss !== config.thunderIdUrl ||
      !audiences.includes(config.apiResource) ||
      claims.client_id !== config.clientId ||
      typeof claims.exp !== 'number' ||
      claims.exp * 1000 < Date.now() ||
      typeof claims.sub !== 'string' ||
      typeof claims.ouId !== 'string'
    ) {
      return null;
    }

    const roles = Array.isArray(claims.roles) ? (claims.roles as string[]) : [];
    return {
      sub: claims.sub,
      username: claims.username as string | undefined,
      email: claims.email as string | undefined,
      ouId: claims.ouId,
      ouHandle: claims.ouHandle as string | undefined,
      ouName: claims.ouName as string | undefined,
      roles,
      scopes: typeof claims.scope === 'string' ? claims.scope.split(' ') : [],
      isAdmin: roles.includes(config.adminRoleName),
    };
  } catch {
    return null;
  }
};

/** The user of the current browser session, or null when no one is signed in. */
export const getSessionUser = async (): Promise<WayfinderUser | null> => {
  try {
    const client = await thunderid();
    const sessionId = await client.getSessionId();
    if (!sessionId) {
      return null;
    }
    return verifyAccessToken(await client.getAccessToken(sessionId));
  } catch {
    return null;
  }
};

/** The user behind an API request: its bearer token, or else the browser session. */
export const getRequestUser = async (request: Request): Promise<WayfinderUser | null> => {
  const authorization = request.headers.get('authorization');
  if (authorization?.startsWith('Bearer ')) {
    return verifyAccessToken(authorization.slice('Bearer '.length));
  }
  return getSessionUser();
};

export const hasScope = (user: WayfinderUser, scope: string): boolean => user.scopes.includes(scope);
