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

import type {Ref} from 'vue';
import type {RouteLocationNormalized} from 'vue-router';
import type {ThunderIDAuthState} from '../types';
import {defineNuxtRouteMiddleware, navigateTo, useState} from '#app';

export interface ThunderIDMiddlewareOptions {
  /**
   * The path to redirect unauthenticated (or unauthorised) requests to.
   * Defaults to `'/api/auth/signin'`.
   */
  redirectTo?: string;
  /**
   * If `true`, the middleware will also require that the user has an
   * `organizationId` in their session.  Redirects to `redirectTo` if not.
   */
  requireOrganization?: boolean;
  /**
   * Required OAuth scopes.  The middleware checks that every listed scope
   * is present in the session before allowing access.
   */
  requireScopes?: string[];
}

const DEFAULT_REDIRECT_TO: string = '/api/auth/signin';

/**
 * Typed factory for ThunderID route middleware.
 *
 * Usage in a page component:
 * ```vue
 * <script setup>
 * definePageMeta({
 *   middleware: [defineThunderIDMiddleware({ requireOrganization: true })]
 * });
 * </script>
 * ```
 *
 * Or add it as a named middleware in `middleware/` and reference by name.
 *
 * The built-in `'auth'` middleware registered by this module is equivalent
 * to calling `defineThunderIDMiddleware()` with no options.
 */
export function defineThunderIDMiddleware(
  options: ThunderIDMiddlewareOptions = {},
): ReturnType<typeof defineNuxtRouteMiddleware> {
  const {redirectTo = DEFAULT_REDIRECT_TO, requireOrganization = false, requireScopes = []} = options;

  return defineNuxtRouteMiddleware((to: RouteLocationNormalized) => {
    const authState: Ref<ThunderIDAuthState> = useState<ThunderIDAuthState>('thunderid:auth');

    if (!authState.value?.isSignedIn) {
      const returnTo: string = encodeURIComponent(to.fullPath);
      return navigateTo(`${redirectTo}?returnTo=${returnTo}`, {external: true});
    }

    const user: Record<string, unknown> | null = authState.value.user as Record<string, unknown> | null;

    if (requireOrganization && !user?.['organizationId']) {
      return navigateTo(redirectTo, {external: true});
    }

    if (requireScopes.length > 0) {
      const sessionScopes: string[] = String(user?.['scopes'] ?? '').split(' ');
      const hasAllScopes: boolean = requireScopes.every((s: string) => sessionScopes.includes(s));
      if (!hasAllScopes) {
        return navigateTo(redirectTo, {external: true});
      }
    }

    return undefined;
  });
}
