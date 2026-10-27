// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import type {Root} from 'react-dom/client';
import type {MountContext} from './contract';
import {EmbedContext} from './EmbedContext';
import {installHostToken} from './hostToken';

export interface EmbedRefs {
  navigateRef: {current: ((path: string) => void) | null};
  colorModeRef: {current: ((mode: 'light' | 'dark') => void) | null};
  /** Cleanup the renderer owns, run by the mount handle on unmount. Host-token mode installs one. */
  disposeRef: {current: (() => void) | null};
}

/** The console's navigate function, handed to the host through the mount handle. */
const setNavigate = (refs: EmbedRefs, navigate: ((path: string) => void) | null): void => {
  refs.navigateRef.current = navigate;
};

const DEFAULT_SCOPES: string[] = ['openid', 'profile', 'email'];

/**
 * Renders the console into the host's node, in whichever of the two modes the host asked for:
 *
 *  - `ctx.auth`: the host runs a backend for frontend. The console signs in through it with the
 *    SDK's backend mode, so no token reaches the browser and API calls go through the proxy.
 *  - `ctx.token`: the host has no backend. It already holds a token minted for this console, so the
 *    console signs nobody in: it runs on that token (see hostToken.ts) and calls the server directly.
 */
export async function renderEmbedded(root: Root, ctx: MountContext, refs: EmbedRefs): Promise<void> {
  const scopes: string[] = ctx.token?.scopes ?? ctx.config.scopes ?? DEFAULT_SCOPES;

  // In host-token mode the session has to exist before the provider initializes.
  refs.disposeRef.current = ctx.token ? await installHostToken(ctx.config, ctx.token) : null;

  window.__THUNDERID_RUNTIME_CONFIG__ = {
    client: {
      base: ctx.basename,
      client_id: ctx.config.clientId,
      embedded: true,
      scopes,
      ...(ctx.auth
        ? {
            backend: {
              check_session: ctx.auth.checkSession,
              check_session_interval: 30,
              proxy: ctx.auth.proxy,
              refresh: ctx.auth.refresh,
              session: ctx.auth.session,
              sign_in: ctx.auth.signIn,
              sign_out: ctx.auth.signOut,
            },
          }
        : {}),
    },
    server: {public_url: ctx.config.serverUrl},
    // Host-token mode is the console's federated ("trusted issuer") shape: the token came from
    // another authorization server, so discovery and the token endpoint live there while every
    // resource endpoint stays on this server. The console never runs a sign-in against it, the
    // session is already installed above, but it needs to know where its token came from to renew
    // or sign out.
    ...(ctx.token
      ? {
          trusted_issuer: {
            client_id: ctx.config.clientId,
            hostname: new URL(ctx.token.issuerUrl).hostname,
            http_only: new URL(ctx.token.issuerUrl).protocol === 'http:',
            port: Number(new URL(ctx.token.issuerUrl).port || (ctx.token.issuerUrl.startsWith('https') ? 443 : 80)),
            public_url: ctx.token.issuerUrl,
            scopes: ctx.token.scopes,
          },
        }
      : {}),
  } as typeof window.__THUNDERID_RUNTIME_CONFIG__;

  // Imported after the config is in place: these modules read it, and set up i18n, on evaluation.
  const [{ConfigProvider}, {LoggerProvider, LogLevel}, {setCnPrefix}, {default: AppWithDecorators}] = await Promise.all([
    import('@thunderid/contexts'),
    import('@thunderid/logger/react'),
    import('@thunderid/utils'),
    import('../AppWithDecorators'),
  ]);

  setCnPrefix('ThunderID');

  const queryClient: QueryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: (failureCount: number, error: unknown): boolean => {
          const status: number | undefined = (error as {response?: {status?: number}})?.response?.status;

          if (status && status >= 400 && status < 500) {
            return false;
          }

          return failureCount < 3;
        },
      },
    },
  });

  root.render(
    <EmbedContext.Provider value={{ctx, setNavigate: (navigate) => setNavigate(refs, navigate)}}>
      <ConfigProvider>
        <LoggerProvider logger={{level: LogLevel.INFO}}>
          <QueryClientProvider client={queryClient}>
            <AppWithDecorators />
          </QueryClientProvider>
        </LoggerProvider>
      </ConfigProvider>
    </EmbedContext.Provider>,
  );
}
