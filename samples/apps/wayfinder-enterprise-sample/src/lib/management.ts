// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import 'server-only';

import config from './config';

const cachedTokens = new Map<string, { value: string; expiresAt: number }>();

/**
 * A token for the app's backend client, issued for one company: the client is shared with every company,
 * so the OU-qualified token endpoint issues it a token that names the company in its ouId. The client
 * holds the root "system" permission, so the admin routes use it only after they have checked the caller
 * and pinned the request to the caller's company.
 */
const getBackendToken = async (ouId: string): Promise<string> => {
  const cached = cachedTokens.get(ouId);
  if (cached && cached.expiresAt - 60_000 > Date.now()) {
    return cached.value;
  }

  const credentials = Buffer.from(`${config.backendClientId}:${config.backendClientSecret}`).toString('base64');
  const response = await fetch(`${config.thunderIdUrl}/ou/${encodeURIComponent(ouId)}/oauth2/token`, {
    method: 'POST',
    headers: {
      Authorization: `Basic ${credentials}`,
      'Content-Type': 'application/x-www-form-urlencoded',
    },
    body: new URLSearchParams({
      grant_type: 'client_credentials',
      scope: 'system',
      resource: config.systemResource,
    }),
    cache: 'no-store',
  });
  if (!response.ok) {
    throw new Error(`The backend client could not get a token for the company (HTTP ${response.status}).`);
  }

  const token = await response.json();
  cachedTokens.set(ouId, { value: token.access_token, expiresAt: Date.now() + token.expires_in * 1000 });
  return token.access_token;
};

/**
 * Calls the ThunderID management API as the app's backend client, with a token issued for the given
 * company, and relays the reply.
 */
export const callManagementApi = async (
  ouId: string,
  method: string,
  path: string,
  query?: URLSearchParams,
  body?: unknown,
): Promise<Response> => {
  const url = `${config.thunderIdUrl}/${path}${query && query.size > 0 ? `?${query}` : ''}`;
  const response = await fetch(url, {
    method,
    headers: {
      Accept: 'application/json',
      Authorization: `Bearer ${await getBackendToken(ouId)}`,
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
    cache: 'no-store',
    redirect: 'error',
  });

  return new Response(response.status === 204 ? null : await response.text(), {
    status: response.status,
    headers: { 'Content-Type': response.headers.get('Content-Type') ?? 'application/json' },
  });
};
