/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
import {FC, useEffect, useRef} from 'react';

/**
 * Props for Callback component
 */
export interface CallbackProps {
  /**
   * Callback function called when an error occurs
   */
  onError?: (error: Error) => void;

  /**
   * Function to navigate to a different path.
   * If not provided, falls back to the browser navigate utility (SPA navigation via History API for same-origin paths).
   * Provide this prop to enable framework-specific navigation (e.g., from React Router).
   */
  onNavigate?: (path: string) => void;
}

/**
 * BaseCallback is a headless component that handles OAuth callback parameter forwarding.
 * This component extracts OAuth parameters (code, state, error) from the URL and forwards them
 * to the original component that initiated the OAuth flow.
 *
 * Works standalone using the browser navigate utility (History API) for navigation by default.
 * Pass an onNavigate prop to enable framework-specific navigation (e.g., via React Router).
 *
 * Flow: Extract OAuth parameters from URL -> Parse state parameter -> Redirect to original path with parameters
 *
 * The original component (SignIn/AcceptInvite) is responsible for:
 * - Processing the OAuth code via the SDK
 * - Calling /flow/execute
 * - Handling the assertion and auth/callback POST
 * - Managing the authenticated session
 */
export const Callback: FC<CallbackProps> = ({onNavigate, onError}: CallbackProps) => {
  // Prevent double execution in React Strict Mode
  const processingRef: any = useRef(false);

  // Resolve navigation: use provided onNavigate (router-aware) or fall back to browser navigate utility
  const navigate = (path: string): void => {
    if (onNavigate) {
      onNavigate(path);
    } else {
      browserNavigate(path);
    }
  };

  useEffect(() => {
    const processOAuthCallback = (): void => {
      // Guard against double execution
      if (processingRef.current) {
        return;
      }
      processingRef.current = true;

      // Declare variables outside try block for use in catch
      let returnPath = '/';

      try {
        // 1. Extract OAuth parameters from URL
        const urlParams: URLSearchParams = new URLSearchParams(window.location.search);
        const code: string | null = urlParams.get('code');
        const state: string | null = urlParams.get('state');
        const nonce: string | null = urlParams.get('nonce');
        const oauthError: string | null = urlParams.get('error');
        const errorDescription: string | null = urlParams.get('error_description');

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
          // If state not found, might be an error callback - try to handle gracefully
          if (oauthError) {
            const errorMsg: string = errorDescription || oauthError || 'OAuth authentication failed';
            const err: Error = new Error(errorMsg);
            onError?.(err);

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
        const MAX_STATE_AGE = 600000; // 10 minutes
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
          onError?.(err);

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
        // The component (SignIn/AcceptInvite) will retrieve flowId/authId from sessionStorage
        const params: URLSearchParams = new URLSearchParams();
        params.set('code', code);
        if (nonce) {
          params.set('nonce', nonce);
        }

        navigate(`${returnPath}?${params.toString()}`);
      } catch (err) {
        const errorMessage: string = err instanceof Error ? err.message : 'OAuth callback processing failed';
        // eslint-disable-next-line no-console
        console.error('OAuth callback error:', err);

        onError?.(err instanceof Error ? err : new Error(errorMessage));

        // Redirect back with OAuth error format
        const params: URLSearchParams = new URLSearchParams();
        params.set('error', 'callback_error');
        params.set('error_description', errorMessage);

        navigate(`${returnPath}?${params.toString()}`);
      }
    };

    processOAuthCallback();
  }, [onNavigate, onError]);

  // Headless component - no UI, just processing logic
  return null;
};

export default Callback;
