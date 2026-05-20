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

import {navigate as browserNavigate} from '@thunderid/browser';
import {type Component, defineComponent, onMounted} from 'vue';
import {createVueLogger} from '../../utils/logger';

const logger: ReturnType<typeof createVueLogger> = createVueLogger('Callback');

interface CallbackSetupProps {
  onError: ((error: Error) => void) | undefined;
  onNavigate: ((path: string) => void) | undefined;
}

/**
 * Callback — headless component that handles OAuth callback parameter forwarding.
 *
 * Extracts OAuth parameters (code, state, error) from the URL and forwards them
 * to the original component that initiated the OAuth flow.
 *
 * Works standalone using the browser navigate utility (History API) for navigation by default.
 * Pass an `onNavigate` prop to enable framework-specific navigation (e.g., via Vue Router).
 *
 * Flow: Extract OAuth parameters from URL -> Parse state parameter -> Redirect to original path with parameters
 */
const Callback: Component = defineComponent({
  name: 'Callback',
  props: {
    onError: {default: undefined, type: Function as unknown as () => (error: Error) => void},
    onNavigate: {default: undefined, type: Function as unknown as () => (path: string) => void},
  },
  setup(props: CallbackSetupProps) {
    const navigate = (path: string): void => {
      if (props.onNavigate) {
        props.onNavigate(path);
      } else {
        browserNavigate(path);
      }
    };

    onMounted(() => {
      let returnPath = '/';

      try {
        // 1. Extract OAuth parameters from URL
        const urlParams: URLSearchParams = new URLSearchParams(window.location.search);
        const code: string | null = urlParams.get('code');
        const state: string | null = urlParams.get('state');
        const nonce: string | null = urlParams.get('nonce');
        const oauthError: string | null = urlParams.get('error');
        const errorDescription: string | null = urlParams.get('error_description');

        // If no OAuth parameters are present, this component is not on a real callback
        // route — do nothing and return early.
        if (!code && !state && !oauthError) {
          return;
        }

        // 1a. If running inside a popup (e.g. social signup), send OAuth params
        //     back to the opener via postMessage. The parent window's handler will
        //     close this popup after processing the code.
        if (window.opener) {
          window.opener.postMessage({code, error: oauthError, errorDescription, nonce, state}, window.location.origin);
          return;
        }

        // 2. Validate and retrieve OAuth state from sessionStorage
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

        const {path, timestamp} = JSON.parse(storedData);
        returnPath = path || '/';

        // 3. Validate state freshness
        const MAX_STATE_AGE = 300000; // 5 minutes
        if (Date.now() - timestamp > MAX_STATE_AGE) {
          sessionStorage.removeItem(`thunderid_oauth_${state}`);
          throw new Error('OAuth state expired - please try again');
        }

        // 4. Clean up state
        sessionStorage.removeItem(`thunderid_oauth_${state}`);

        // 5. Handle OAuth error response
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

        // 6. Validate required parameters
        if (!code) {
          throw new Error('Missing OAuth authorization code');
        }

        // 7. Forward OAuth code to original component
        const params: URLSearchParams = new URLSearchParams();
        params.set('code', code);
        if (nonce) {
          params.set('nonce', nonce);
        }

        navigate(`${returnPath}?${params.toString()}`);
      } catch (err) {
        const errorMessage: string = err instanceof Error ? err.message : 'OAuth callback processing failed';
        logger.error('OAuth callback error:', err);

        props.onError?.(err instanceof Error ? err : new Error(errorMessage));

        const params: URLSearchParams = new URLSearchParams();
        params.set('error', 'callback_error');
        params.set('error_description', errorMessage);

        navigate(`${returnPath}?${params.toString()}`);
      }
    });

    // Headless component — renders nothing
    return (): null => null;
  },
});

export default Callback;
