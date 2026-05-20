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

import {navigate} from '@thunderid/browser';

/**
 * Initiates OAuth redirect with CSRF protection.
 * Generates random state, stores return path in sessionStorage, and redirects to OAuth provider.
 *
 * @param redirectURL - OAuth authorization URL from the server
 */
export function initiateOAuthRedirect(redirectURL: string): void {
  const basePath: string = document.querySelector('base')?.getAttribute('href') || '';
  let returnPath: string = window.location.pathname;

  if (basePath && returnPath.startsWith(basePath)) {
    returnPath = returnPath.slice(basePath.length) || '/';
  }

  const state: string = crypto.randomUUID();

  sessionStorage.setItem(
    `thunderid_oauth_${state}`,
    JSON.stringify({
      path: returnPath,
      timestamp: Date.now(),
    }),
  );

  const redirectUrlObj: URL = new URL(redirectURL);
  redirectUrlObj.searchParams.set('state', state);

  navigate(redirectUrlObj.toString());
}
