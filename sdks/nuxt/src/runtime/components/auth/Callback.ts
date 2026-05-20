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

import {type Component, defineComponent, onMounted} from 'vue';
import {navigateTo} from '#app';

const error = (msg: string, ...args: unknown[]): void => {
  // eslint-disable-next-line no-console
  console.error(`[@thunderid/nuxt] Callback: ${msg}`, ...args);
};

interface CallbackSetupProps {
  onError: ((err: Error) => void) | undefined;
  onNavigate: ((path: string) => void) | undefined;
}

/**
 * Nuxt-specific Callback component.
 *
 * Handles OAuth callback parameter forwarding — extracts `code`, `state`, and
 * `error` from the URL, validates the state stored in `sessionStorage`, and
 * forwards the OAuth parameters to the originating route.
 *
 * **SSR-safe**: all `window` / `sessionStorage` access is gated inside
 * `onMounted`, which only runs on the client. Navigation uses Nuxt's
 * `navigateTo` instead of `window.location` so the redirect is handled
 * correctly in both SSR and CSR contexts.
 *
 * Pass `onNavigate` to override the navigation handler (e.g. for testing or
 * custom routing logic).
 *
 * @example
 * ```vue
 * <!-- pages/callback.vue -->
 * <template>
 *   <ThunderIDCallback :on-error="handleError" />
 * </template>
 * ```
 */
const Callback: Component = defineComponent({
  name: 'Callback',
  props: {
    onError: {default: undefined, type: Function as unknown as () => (err: Error) => void},
    onNavigate: {default: undefined, type: Function as unknown as () => (path: string) => void},
  },
  setup(props: CallbackSetupProps) {
    /** Navigate using the prop override, falling back to Nuxt's navigateTo. */
    const navigate = (path: string): void => {
      if (props.onNavigate) {
        props.onNavigate(path);
      } else {
        // navigateTo handles both SSR (sets redirect response) and client (pushState).
        navigateTo(path);
      }
    };

    onMounted(() => {
      // All browser APIs are safe here — onMounted only runs on the client.
      let returnPath: string = '/';

      try {
        // 1. Extract OAuth parameters from the current URL.
        const urlParams: URLSearchParams = new URLSearchParams(window.location.search);
        const code: string | null = urlParams.get('code');
        const state: string | null = urlParams.get('state');
        const nonce: string | null = urlParams.get('nonce');
        const oauthError: string | null = urlParams.get('error');
        const errorDescription: string | null = urlParams.get('error_description');

        // No OAuth parameters present — not on a real callback route.
        if (!code && !state && !oauthError) {
          return;
        }

        // 2. Validate state presence.
        if (!state) {
          throw new Error('Missing OAuth state parameter - possible security issue');
        }

        const storedData: string | null = sessionStorage.getItem(`thunderid_oauth_${state}`);

        if (!storedData) {
          if (oauthError) {
            const errorMsg: string = errorDescription || oauthError || 'OAuth authentication failed';
            const err: Error = new Error(errorMsg);
            props.onError?.(err);

            const params: URLSearchParams = new URLSearchParams();
            params.set('error', oauthError);
            if (errorDescription) {
              params.set('error_description', errorDescription);
            }

            navigate(`/?${params.toString()}`);
            return;
          }
          throw new Error('Invalid OAuth state - possible CSRF attack');
        }

        const {path, timestamp} = JSON.parse(storedData) as {path: string; timestamp: number};
        returnPath = path || '/';

        // 3. Validate state freshness (5 minute window).
        const MAX_STATE_AGE: number = 300_000;
        if (Date.now() - timestamp > MAX_STATE_AGE) {
          sessionStorage.removeItem(`thunderid_oauth_${state}`);
          throw new Error('OAuth state expired - please try again');
        }

        // 4. Clean up consumed state.
        sessionStorage.removeItem(`thunderid_oauth_${state}`);

        // 5. Handle an OAuth error response.
        if (oauthError) {
          const errorMsg: string = errorDescription || oauthError || 'OAuth authentication failed';
          const err: Error = new Error(errorMsg);
          props.onError?.(err);

          const params: URLSearchParams = new URLSearchParams();
          params.set('error', oauthError);
          if (errorDescription) {
            params.set('error_description', errorDescription);
          }

          navigate(`${returnPath}?${params.toString()}`);
          return;
        }

        // 6. Validate authorization code.
        if (!code) {
          throw new Error('Missing OAuth authorization code');
        }

        // 7. Forward the code (and optional nonce) to the originating route.
        const params: URLSearchParams = new URLSearchParams();
        params.set('code', code);
        if (nonce) {
          params.set('nonce', nonce);
        }

        navigate(`${returnPath}?${params.toString()}`);
      } catch (err) {
        const errorMessage: string = err instanceof Error ? err.message : 'OAuth callback processing failed';
        error('OAuth callback error:', err);

        props.onError?.(err instanceof Error ? err : new Error(errorMessage));

        const params: URLSearchParams = new URLSearchParams();
        params.set('error', 'callback_error');
        params.set('error_description', errorMessage);

        navigate(`${returnPath}?${params.toString()}`);
      }
    });

    // Headless component — renders nothing.
    return (): null => null;
  },
});

export default Callback;
