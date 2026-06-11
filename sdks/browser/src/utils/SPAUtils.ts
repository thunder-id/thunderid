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

import {OIDCRequestConstants} from '@thunderid/javascript';
import {
  ERROR,
  ERROR_DESCRIPTION,
  INITIALIZED_SILENT_SIGN_IN,
  PROMPT_NONE_REQUEST_SENT,
  SILENT_SIGN_IN_STATE,
  STATE_QUERY,
} from '../constants/SPAConstants';
import {SignOutError} from '../models/SignOutError';

/**
 * Static utility methods for SPA authentication flows including PKCE storage,
 * sign-out URL management, and URL-based state detection.
 */
class SPAUtils {
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private constructor() {}

  /**
   * Removes the `code` search parameter from the current URL without a page reload.
   */
  public static removeAuthorizationCode(): void {
    const url = location.href;

    history.pushState({}, document.title, url.replace(/\?code=.*$/, ''));
  }

  /**
   * Retrieves a PKCE verifier from storage.
   *
   * @param pkceKey - The storage key for the PKCE verifier.
   * @returns The stored verifier string, or an empty string if not found.
   */
  public static getPKCE(pkceKey: string): string {
    return localStorage.getItem(pkceKey) ?? '';
  }

  /**
   * Persists a PKCE verifier in storage.
   *
   * @param pkceKey - The storage key.
   * @param pkce - The PKCE verifier value.
   */
  public static setPKCE(pkceKey: string, pkce: string): void {
    localStorage.setItem(pkceKey, pkce);
  }

  /**
   * Persists the post-sign-out redirect URL for the given client and instance.
   *
   * @param url - The sign-out redirect URL.
   * @param clientId - The OAuth2 client ID.
   * @param instanceId - The client instance ID.
   */
  public static setSignOutURL(url: string, clientId: string, instanceId: number): void {
    sessionStorage.setItem(
      `${OIDCRequestConstants.SignOut.Storage.StorageKeys.SIGN_OUT_URL}-instance_${instanceId}-${clientId}`,
      url,
    );
  }

  /**
   * Retrieves the stored sign-out redirect URL for the given client and instance.
   *
   * @param clientId - The OAuth2 client ID.
   * @param instanceId - The client instance ID.
   * @returns The stored sign-out URL, or an empty string.
   */
  public static getSignOutUrl(clientId: string, instanceId: number): string {
    return (
      sessionStorage.getItem(
        `${OIDCRequestConstants.SignOut.Storage.StorageKeys.SIGN_OUT_URL}-instance_${instanceId}-${clientId}`,
      ) ?? ''
    );
  }

  /**
   * Removes a PKCE verifier from storage.
   *
   * @param pkceKey - The storage key to remove.
   */
  public static removePKCE(pkceKey: string): void {
    localStorage.removeItem(pkceKey);
  }

  /**
   * Determines whether the `signIn` method should continue based on the `callOnlyOnRedirect` flag.
   *
   * @param callOnlyOnRedirect - True if the call should only proceed when redirected back from the IdP.
   * @param authorizationCode - Authorization code passed directly (form_post mode).
   * @returns `true` if sign-in should proceed.
   */
  public static canContinueSignIn(callOnlyOnRedirect: boolean, authorizationCode?: string): boolean {
    if (callOnlyOnRedirect && !SPAUtils.hasErrorInURL() && !SPAUtils.hasAuthSearchParamsInURL() && !authorizationCode) {
      return false;
    }

    return true;
  }

  /**
   * Returns `true` if silent sign-in is in progress (silent-state present in the URL).
   */
  public static isInitializedSilentSignIn(): boolean {
    return SPAUtils.isSilentStatePresentInURL();
  }

  /**
   * Returns `true` if the `signIn` method was already called this navigation
   * (auth code or error is present in the URL, but not a silent flow).
   */
  public static wasSignInCalled(): boolean {
    if (SPAUtils.hasErrorInURL() || SPAUtils.hasAuthSearchParamsInURL()) {
      if (!this.isSilentStatePresentInURL()) {
        return true;
      }
    }

    return false;
  }

  /**
   * Returns `true` if a silent sign-in was previously initialized in this session.
   */
  public static wasSilentSignInCalled(): boolean {
    const raw = sessionStorage.getItem(INITIALIZED_SILENT_SIGN_IN);

    return Boolean(raw ? JSON.parse(raw) : null);
  }

  /**
   * Checks whether the current URL indicates a successful sign-out redirect.
   * Clears the query string and session data if `true`.
   *
   * @param isSignOutSuccessful - Static method from the JS client for URL inspection.
   * @param clearSession - Callback to clear session data after successful sign-out.
   * @returns `true` if the sign-out completed successfully.
   */
  public static async isSignOutSuccessful(
    isSignOutSuccessfulFn: (url: string) => boolean,
    clearSession: () => Promise<void>,
  ): Promise<boolean> {
    if (isSignOutSuccessfulFn(window.location.href)) {
      const newUrl = window.location.href.split('?')[0];
      history.pushState({}, document.title, newUrl);
      await clearSession();

      return true;
    }

    return false;
  }

  /**
   * Checks whether the current URL indicates a sign-out failure.
   * Returns the error details if present, or `false` otherwise.
   *
   * @param didSignOutFailFn - Static method from the JS client for URL inspection.
   * @returns The `SignOutError` if sign-out failed, or `false`.
   */
  public static didSignOutFail(didSignOutFailFn: (url: string) => boolean): boolean | SignOutError {
    if (didSignOutFailFn(window.location.href)) {
      const url: URL = new URL(window.location.href);
      const error: string | null = url.searchParams.get(ERROR);
      const description: string | null = url.searchParams.get(ERROR_DESCRIPTION);
      const newUrl = window.location.href.split('?')[0];
      history.pushState({}, document.title, newUrl);

      return {
        description: description ?? '',
        error: error ?? '',
      };
    }

    return false;
  }

  /**
   * Returns `true` if the URL contains a silent sign-in state parameter.
   */
  public static isSilentStatePresentInURL(): boolean {
    const state = new URL(window.location.href).searchParams.get('state');

    return state?.includes(SILENT_SIGN_IN_STATE) ?? false;
  }

  /**
   * Returns `true` if the current URL contains an authorization code (`code` parameter).
   *
   * @param params - Search params string (defaults to `window.location.search`).
   */
  public static hasAuthSearchParamsInURL(params: string = window.location.search): boolean {
    const AUTH_CODE_REGEXP = /[?&]code=[^&]+/;

    return AUTH_CODE_REGEXP.test(params);
  }

  /**
   * Returns `true` if the current URL contains an OAuth2 error parameter
   * (but not a sign-out success state).
   *
   * @param url - URL to inspect (defaults to `window.location.href`).
   */
  public static hasErrorInURL(url: string = window.location.href): boolean {
    const urlObject: URL = new URL(url);

    return (
      !!urlObject.searchParams.get(ERROR) &&
      urlObject.searchParams.get(STATE_QUERY) !== OIDCRequestConstants.Params.SIGN_OUT_SUCCESS
    );
  }

  /**
   * Returns `true` if no prompt-none request has been sent yet this session.
   */
  public static canSendPromptNoneRequest(): boolean {
    const raw = sessionStorage.getItem(PROMPT_NONE_REQUEST_SENT);

    return !(raw ? JSON.parse(raw) : null);
  }

  /**
   * Records whether a prompt-none request has been sent.
   *
   * @param canSend - `true` marks the request as sent.
   */
  public static setPromptNoneRequestSent(canSend: boolean): void {
    sessionStorage.setItem(PROMPT_NONE_REQUEST_SENT, JSON.stringify(canSend));
  }

  /**
   * Waits until the browser has redirected (non-blocking delay).
   *
   * @param time - Time to wait in seconds (default: 3).
   */
  public static async waitTillPageRedirect(time?: number): Promise<void> {
    const timeToWait = time ?? 3000;

    await new Promise((resolve) => setTimeout(resolve, timeToWait * 1000));
  }

  /**
   * Returns a Promise that resolves when `condition()` returns `true`.
   *
   * @param condition - Predicate to poll.
   * @param timeout - Poll interval in milliseconds (default: 500).
   */
  public static until = (condition: () => boolean, timeout = 500): Promise<void> => {
    const poll = (done: () => void): void => {
      if (condition()) {
        done();
      } else {
        setTimeout(() => poll(done), timeout);
      }
    };

    return new Promise(poll);
  };
}

export default SPAUtils;
