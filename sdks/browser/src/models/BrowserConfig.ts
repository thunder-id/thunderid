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

import {AuthClientConfig} from '@thunderid/javascript';
import {BrowserStorage} from './BrowserStorage';

/**
 * Browser-specific SDK configuration that extends the base OIDC config.
 */
export interface BrowserClientConfig {
  /**
   * Storage backend to use for session data.
   * @default BrowserStorage.SessionStorage
   */
  storage?: BrowserStorage | 'sessionStorage' | 'localStorage' | 'browserMemory';
  /** Enable OIDC Session Management via RP Iframe. Requires same-domain or third-party cookies. */
  syncSession?: boolean;
  /** Interval in seconds between session-check polls. @default 3 */
  checkSessionInterval?: number;
  /** Interval in seconds for silent token refresh. @default 300 */
  sessionRefreshInterval?: number;
  /** Allowed external URL prefixes for `httpRequest` calls. */
  allowedExternalUrls?: string[];
  /** Additional query params to append to every authorize request. */
  authParams?: Record<string, string>;
  /** Automatically refresh the access token before it expires. */
  periodicTokenRefresh?: boolean;
  /** Sign the user out when a token refresh attempt fails. @default false */
  autoLogoutOnTokenRefreshError?: boolean;
}

/** Full browser SDK configuration, combining base OIDC config with browser-specific fields. */
export type BrowserAuthConfig = AuthClientConfig<BrowserClientConfig>;
