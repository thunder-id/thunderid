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

/**
 * Navigates to a new URL within the browser.
 *
 * - For same-origin URLs (relative paths or absolute URLs with the same origin),
 *   uses the History API and dispatches a `popstate` event (SPA navigation).
 * - For cross-origin URLs, performs a full page load using `window.location.assign`.
 *
 * This allows seamless navigation for both SPA routes and external links.
 *
 * @param url - The target URL to navigate to. Can be a path, query, or absolute URL.
 *
 * @example
 * ```typescript
 * // SPA navigation (same origin)
 * navigate('/dashboard');
 *
 * // SPA navigation with query
 * navigate('/search?q=thunderid');
 *
 * // Cross-origin navigation (full page load)
 * navigate('https://accounts.asgardeo.io/t/dxlab/accountrecoveryendpoint/register.do');
 * ```
 */
const navigate = (url: string): void => {
  try {
    const targetUrl: URL = new URL(url, window.location.origin);
    if (targetUrl.origin === window.location.origin) {
      window.history.pushState(null, '', targetUrl.pathname + targetUrl.search + targetUrl.hash);
      window.dispatchEvent(new PopStateEvent('popstate', {state: null}));
    } else {
      window.location.assign(targetUrl.href);
    }
  } catch {
    // If URL constructor fails (e.g., malformed URL), fallback to location.assign
    window.location.assign(url);
  }
};

export default navigate;
