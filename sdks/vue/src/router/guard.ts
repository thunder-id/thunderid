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

import {inject} from 'vue';
import {THUNDERID_KEY} from '../keys';
import type {ThunderIDContext} from '../models/contexts';
import {createVueLogger} from '../utils/logger';

const logger: ReturnType<typeof createVueLogger> = createVueLogger('Guard');

/**
 * Options for the ThunderID navigation guard.
 */
export interface GuardOptions {
  /**
   * Maximum time (in ms) to wait for SDK initialization before redirecting.
   * Only applicable when `waitForInit` is `true`.
   * @default 10000
   */
  initTimeout?: number;

  /**
   * The path to redirect unauthenticated users to.
   * @default '/'
   */
  redirectTo?: string;

  /**
   * If `true`, the guard will wait for the SDK to finish initializing before
   * evaluating the authentication state. If `false`, the guard will reject
   * immediately when the SDK is not yet initialized.
   * @default true
   */
  waitForInit?: boolean;
}

/**
 * A minimal navigation guard type compatible with Vue Router's `NavigationGuard`.
 *
 * This avoids a hard dependency on `vue-router` while remaining structurally compatible.
 * When consumers pass this to `beforeEnter` or `router.beforeEach`, Vue Router will
 * accept it because it satisfies the shape of `NavigationGuard`.
 */
export type NavigationGuardReturn = boolean | string | {path: string} | {name: string} | undefined;
export type ThunderIDNavigationGuard = (
  to: {fullPath: string; path: string; query: Record<string, string | (string | null)[] | null | undefined>},
  from: {fullPath: string; path: string},
  next: (target?: NavigationGuardReturn) => void,
) => void | Promise<void>;

/**
 * Creates a Vue Router navigation guard that protects routes by requiring authentication.
 *
 * The guard injects the ThunderID context to check `isSignedIn` state.
 * If the user is not authenticated, they are redirected to `redirectTo`.
 *
 * **Requires `vue-router` as a peer dependency.**
 *
 * @param options - Guard configuration options.
 * @returns A navigation guard function compatible with Vue Router's `beforeEnter` or `router.beforeEach`.
 *
 * @example
 * ```typescript
 * import { createRouter, createWebHistory } from 'vue-router';
 * import { createThunderIDGuard } from '@thunderid/vue';
 *
 * const router = createRouter({
 *   history: createWebHistory(),
 *   routes: [
 *     {
 *       path: '/dashboard',
 *       component: Dashboard,
 *       beforeEnter: createThunderIDGuard({ redirectTo: '/login' }),
 *     },
 *   ],
 * });
 * ```
 *
 * @example
 * ```typescript
 * // Global guard on all routes
 * router.beforeEach(createThunderIDGuard({ redirectTo: '/' }));
 * ```
 */
export const createThunderIDGuard = (options: GuardOptions = {}): ThunderIDNavigationGuard => {
  const {redirectTo = '/', waitForInit = true, initTimeout = 10000} = options;

  return async (_to: unknown, _from: unknown, next: (target?: {path: string}) => void): Promise<void> => {
    const ctx: ThunderIDContext | undefined = inject(THUNDERID_KEY);

    if (!ctx) {
      logger.error(
        'createThunderIDGuard: ThunderID context not found. ' +
          'Ensure the ThunderIDPlugin is installed before using the router guard.',
      );
      next({path: redirectTo});

      return;
    }

    // If initialized and signed in, allow navigation
    if (ctx.isInitialized.value && ctx.isSignedIn.value) {
      next();

      return;
    }

    // If initialized and not signed in, redirect
    if (ctx.isInitialized.value && !ctx.isSignedIn.value) {
      next({path: redirectTo});

      return;
    }

    // SDK not yet initialized — optionally wait for it
    if (!waitForInit) {
      next({path: redirectTo});

      return;
    }

    // Wait for initialization to complete
    try {
      await new Promise<void>((resolve: () => void, reject: (reason?: Error) => void) => {
        const timeout: ReturnType<typeof setTimeout> = setTimeout(() => {
          reject(new Error('ThunderID SDK initialization timed out'));
        }, initTimeout);

        const check = (): void => {
          if (ctx.isInitialized.value) {
            clearTimeout(timeout);
            resolve();
          } else {
            requestAnimationFrame(check);
          }
        };

        check();
      });

      if (ctx.isSignedIn.value) {
        next();
      } else {
        next({path: redirectTo});
      }
    } catch {
      // Timed out — redirect to fallback
      next({path: redirectTo});
    }
  };
};

export default createThunderIDGuard;
