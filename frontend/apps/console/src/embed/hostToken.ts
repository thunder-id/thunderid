// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Host-token mode: the console runs inside a host that has no backend anywhere.
 *
 * The host signs the user in once and then exchanges that session for a token minted for this
 * console (RFC 8693), as the console's own client. What reaches us is the finished token, bound to
 * this console's server and carrying only its scopes, so the console signs nobody in and never sees
 * the host's own token.
 *
 * The token is written as the SDK's session before the provider mounts, because initializing is
 * when the SDK asks whether there is one. A short-lived client of the same instance writes it
 * through the SDK's own `setSession`, so the storage layout stays the SDK's business, and
 * initializing that client makes no network call.
 *
 * A first-class "continue from this token" entry point on the SDK would replace this file.
 */

import {BrowserStorage, ThunderIDBrowserClient} from '@thunderid/browser';
import type {RemoteHostToken, RemoteServerConfig} from './contract';

/** Re-exchange this long before expiry, so the console never makes a call with a dead token. */
const RENEW_MARGIN_MS = 60_000;

const openSeedClient = async (config: RemoteServerConfig, scopes: string[]): Promise<ThunderIDBrowserClient> => {
  const client: ThunderIDBrowserClient = new ThunderIDBrowserClient(0);

  await client.initialize({
    baseUrl: config.serverUrl,
    clientId: config.clientId,
    scopes,
    storage: BrowserStorage.SessionStorage,
  } as never);

  return client;
};

const writeSession = async (
  client: ThunderIDBrowserClient,
  accessToken: string,
  expiresAt: number,
  scopes: string[],
): Promise<void> => {
  const now: number = Date.now();

  await client.setSession({
    access_token: accessToken,
    // The SDK reads these two to decide whether the session is still alive.
    created_at: now,
    expires_in: String(Math.max(1, Math.round((expiresAt - now) / 1000))),
    scope: scopes.join(' '),
    token_type: 'Bearer',
  });
};

/**
 * Seeds the SDK's session from the host's token and keeps it fresh.
 *
 * @returns A disposer the mount handle calls on unmount.
 */
export async function installHostToken(config: RemoteServerConfig, token: RemoteHostToken): Promise<() => void> {
  const client: ThunderIDBrowserClient = await openSeedClient(config, token.scopes);

  await writeSession(client, token.accessToken, token.expiresAt, token.scopes);

  let timer = 0;
  let disposed = false;

  const scheduleRenewal = (expiresAt: number): void => {
    const delay: number = Math.max(5_000, expiresAt - RENEW_MARGIN_MS - Date.now());

    timer = window.setTimeout(() => {
      if (disposed) {
        return;
      }

      token
        .renew()
        .then(async (next: {accessToken: string; expiresAt: number}) => {
          if (disposed) {
            return;
          }

          await writeSession(client, next.accessToken, next.expiresAt, token.scopes);
          scheduleRenewal(next.expiresAt);
        })
        .catch(() => {
          // The host owns the sign-in; if it cannot renew, it is the one that reacts.
        });
    }, delay);
  };

  scheduleRenewal(token.expiresAt);

  return () => {
    disposed = true;
    window.clearTimeout(timer);
    void client.clearSession();
  };
}
