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

import {type Component, defineComponent, h} from 'vue';
import Callback from '../components/auth/Callback';

/**
 * Options for creating a callback route.
 */
export interface CallbackRouteOptions {
  /**
   * The route name. If not provided, no name is set on the route record.
   */
  name?: string;

  /**
   * Optional error handler called when the OAuth callback encounters an error.
   */
  onError?: (error: Error) => void;

  /**
   * The URL path for the callback route.
   * @default '/callback'
   */
  path?: string;
}

/**
 * A minimal route record type compatible with Vue Router's `RouteRecordRaw`.
 *
 * This avoids a hard dependency on `vue-router` while remaining structurally compatible.
 */
export interface ThunderIDRouteRecord {
  component: ReturnType<typeof defineComponent>;
  meta?: Record<string, unknown>;
  name?: string;
  path: string;
}

/**
 * Creates a Vue Router route record for the OAuth2 callback.
 *
 * The generated route renders the `<Callback>` component which extracts OAuth parameters
 * (code, state, error) from the URL and redirects the user back to the original path.
 *
 * **Requires `vue-router` as a peer dependency.**
 *
 * @param options - Callback route configuration.
 * @returns A route record compatible with Vue Router's `RouteRecordRaw`.
 *
 * @example
 * ```typescript
 * import { createRouter, createWebHistory } from 'vue-router';
 * import { createCallbackRoute } from '@thunderid/vue';
 *
 * const router = createRouter({
 *   history: createWebHistory(),
 *   routes: [
 *     createCallbackRoute({ path: '/callback' }),
 *     { path: '/', component: Home },
 *     { path: '/dashboard', component: Dashboard },
 *   ],
 * });
 * ```
 *
 * @example
 * ```typescript
 * // With error handling and Vue Router navigation
 * import { useRouter } from 'vue-router';
 *
 * createCallbackRoute({
 *   path: '/auth/callback',
 *   name: 'oauth-callback',
 *   onError: (error) => console.error('OAuth error:', error),
 * });
 * ```
 */
export const createCallbackRoute = (options: CallbackRouteOptions = {}): ThunderIDRouteRecord => {
  const {path = '/callback', name, onError} = options;

  const CallbackWrapper: Component = defineComponent({
    name: 'ThunderIDCallbackRoute',
    setup() {
      return (): ReturnType<typeof h> =>
        h(Callback, {
          ...(onError && {onError}),
        });
    },
  });

  return {
    ...(name && {name}),
    component: CallbackWrapper,
    meta: {isThunderIDCallback: true},
    path,
  };
};

export default createCallbackRoute;
