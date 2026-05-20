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

import {Config} from '@thunderid/javascript';

/**
 * Session cookie configuration options shared across all server-side SDKs.
 *
 * All fields are optional; unset fields fall back to the defaults defined in
 * each framework SDK's `CookieConfig` constants.
 */
export interface SessionCookieConfig {
  /**
   * Session lifetime in seconds. Controls both the server-side session validation
   * window and the browser cookie max-age (multiplied by 1000 for ms).
   *
   * Resolution order (first defined value wins):
   *   1. This field — set programmatically at SDK initialisation.
   *   2. `THUNDERID_SESSION_COOKIE_EXPIRY_TIME` environment variable.
   *   3. Built-in default of 86400 seconds (24 hours).
   *
   * @example
   * // 8-hour session
   * { sessionCookie: { expiryTime: 28800 } }
   */
  expiryTime?: number;
  /** Whether the cookie is inaccessible to JavaScript. Default: `true`. */
  httpOnly?: boolean;
  /** SameSite policy. Default: `'lax'`. */
  sameSite?: 'lax' | 'strict' | 'none';
  /** Whether the cookie requires HTTPS. Default: `false` (dev-friendly). */
  secure?: boolean;
}

/**
 * Configuration type for the ThunderID Node.js SDK.
 * Extends the base Config type from @thunderid/javascript with Node.js specific settings.
 */
export type ThunderIDNodeConfig = Config & {
  /**
   * Session cookie settings. Groups all cookie-related configuration in one place
   * so that any server SDK (Node, Express, Next.js, …) inherits the same shape.
   */
  sessionCookie?: SessionCookieConfig;
};
