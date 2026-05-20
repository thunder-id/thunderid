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

import {EmbeddedSignInFlowStatus, getRedirectBasedSignUpUrl} from '@thunderid/browser';
import {useThunderID as useThunderIDVue, type ThunderIDContext} from '@thunderid/vue';
import type {Ref} from 'vue';
import type {ThunderIDAuthState} from '../types';
import {navigateTo, useState, useRuntimeConfig} from '#app';

/**
 * Nuxt-aware primary composable for ThunderID authentication.
 *
 * Mirrors the Next.js `useThunderID` hook: a thin wrapper over the base SDK's
 * `useThunderID` that re-binds the redirect-based actions (`signIn`, `signOut`,
 * `signUp`) to Nuxt's {@link navigateTo} so SSR redirects use the correct
 * response mechanism instead of `window.location`.
 *
 * The surrounding context is guaranteed to be present by the Nuxt plugin
 * (`THUNDERID_KEY`) and {@link ThunderIDRoot} (the auxiliary provider tree),
 * so this composable does not carry a fallback branch.
 *
 * @example
 * ```vue
 * <script setup>
 * const { isSignedIn, user, signIn, signOut } = useThunderID();
 * </script>
 * ```
 */
export function useThunderID(): ThunderIDContext {
  const context: ThunderIDContext = useThunderIDVue();

  /**
   * Sign in the user.
   *
   * **Embedded flow**: call with `(payload, request)` where `payload` has a
   * `flowId` property (use `{flowId: ''}` to initiate).  The method POSTs to
   * `/api/auth/signin` and returns the flow-step response or redirects on
   * completion.
   *
   * **Redirect flow**: call with an optional `options` object (or no args).
   * Navigates to `/api/auth/signin` (which triggers a server redirect to the
   * IdP).
   */
  const signIn = async (...args: any[]): Promise<any> => {
    // Embedded-flow path: second arg is a non-null object with `flowId`.
    const arg0: unknown = args[0];
    const isEmbedded: boolean = typeof arg0 === 'object' && arg0 !== null && 'flowId' in arg0;

    if (isEmbedded) {
      const payload: Record<string, unknown> = arg0 as Record<string, unknown>;
      const request: Record<string, unknown> = (args[1] ?? {}) as Record<string, unknown>;
      const res: {data: any; success: boolean} = await $fetch<{data: any; success: boolean}>('/api/auth/signin', {
        body: {payload, request},
        method: 'POST',
      });

      // Flow complete — server has set the session cookie. Refresh the client
      // auth state so `useThunderID().isSignedIn` flips to true *immediately*
      // (without waiting for a full page reload). Then return a synthetic
      // SuccessCompleted response so `BaseSignIn` emits its `success` event
      // and the wrapper component (`<ThunderIDSignIn>`) drives navigation via
      // `onSuccess`.
      //
      // `authData` is intentionally empty: the auth code / state were already
      // consumed server-side in `signin.post.ts`, so there is nothing to
      // forward to the client. Keeping it `{}` also stops the wrapper's
      // `handleSuccess` from appending stray query params to `afterSignInUrl`.
      if (res.data?.afterSignInUrl) {
        if (import.meta.client) {
          try {
            const session: ThunderIDAuthState = await $fetch<ThunderIDAuthState>('/api/auth/session');
            const authState: Ref<ThunderIDAuthState> = useState<ThunderIDAuthState>('thunderid:auth');
            authState.value = session;
          } catch {
            // Best-effort — the cookie is set; a navigation will recover state.
          }
        }
        return {
          authData: {},
          flowStatus: EmbeddedSignInFlowStatus.SuccessCompleted,
        };
      }
      return res.data;
    }

    // Redirect flow.
    const options: Record<string, unknown> | undefined = arg0 as Record<string, unknown> | undefined;
    const returnTo: string | undefined = typeof options?.['returnTo'] === 'string' ? options['returnTo'] : undefined;
    const url: string = returnTo ? `/api/auth/signin?returnTo=${encodeURIComponent(returnTo)}` : '/api/auth/signin';
    await navigateTo(url, {external: true});
    return undefined;
  };

  const signOut = async (): Promise<void> => {
    const res: {redirectUrl: string} = await $fetch<{redirectUrl: string}>('/api/auth/signout', {method: 'POST'});
    await navigateTo(res.redirectUrl || '/', {external: true});
  };

  /**
   * Sign up the user.
   *
   * **Embedded flow**: call with a payload object that has a `flowType` key.
   * POSTs to `/api/auth/signup` and returns the flow-step response or redirects
   * on completion.
   *
   * **Redirect flow** (no args, or anything that doesn't look like a flow
   * payload): navigates to the ThunderID-hosted account-recovery `register.do`
   * page. Mirrors `ThunderIDReactClient.signUp` — when the consumer configures
   * an explicit `signUpUrl`, that wins; otherwise the URL is derived from
   * `baseUrl` / `clientId` / `applicationId` via `getRedirectBasedSignUpUrl`.
   */
  const signUp = async (...args: any[]): Promise<any> => {
    const payload: unknown = args[0];

    // Embedded flow — payload must look like an EmbeddedFlowExecuteRequestPayload
    // (i.e. have a `flowType` field). Plain options objects without `flowType`
    // fall through to the redirect path so `signUp({applicationId: '...'})`
    // still goes to the hosted register page.
    if (payload && typeof payload === 'object' && 'flowType' in payload) {
      const res: {data: any; success: boolean} = await $fetch<{data: any; success: boolean}>('/api/auth/signup', {
        body: {payload},
        method: 'POST',
      });
      if (res.data?.afterSignUpUrl) {
        await navigateTo(res.data.afterSignUpUrl as string, {external: false});
        return undefined;
      }
      return res.data;
    }

    // Redirect flow.
    const cfg: {
      applicationId?: string;
      baseUrl?: string;
      clientId?: string;
      signUpUrl?: string;
    } = (useRuntimeConfig().public.thunderid ?? {}) as {
      applicationId?: string;
      baseUrl?: string;
      clientId?: string;
      signUpUrl?: string;
    };

    // Explicit override always wins.
    if (cfg.signUpUrl) {
      await navigateTo(cfg.signUpUrl, {external: true});
      return undefined;
    }

    const redirectUrl: string = getRedirectBasedSignUpUrl({
      applicationId: cfg.applicationId,
      baseUrl: cfg.baseUrl,
      clientId: cfg.clientId,
    } as any);

    if (redirectUrl) {
      await navigateTo(redirectUrl, {external: true});
      return undefined;
    }

    // Last-resort fallback: the embedded sign-up page on the consumer app.
    // Reached only if the baseUrl is unrecognised by getRedirectBasedSignUpUrl
    // (e.g. self-hosted Identity Server with a non-standard host pattern) and
    // no signUpUrl override was configured.
    await navigateTo('/sign-up', {external: false});
    return undefined;
  };

  return {...context, signIn, signOut, signUp} as ThunderIDContext;
}
