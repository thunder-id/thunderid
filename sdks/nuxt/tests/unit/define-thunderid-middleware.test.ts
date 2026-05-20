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

/* eslint-disable @typescript-eslint/typedef, sort-keys, @typescript-eslint/explicit-function-return-type, @typescript-eslint/no-unused-vars, @typescript-eslint/naming-convention */

import {describe, it, expect, vi, beforeEach} from 'vitest';

import {
  defineThunderIDMiddleware,
  ThunderIDMiddlewareOptions,
} from '../../src/runtime/middleware/defineThunderIDMiddleware';

// ── Mock Nuxt's #app module ────────────────────────────────────────────────
// defineThunderIDMiddleware depends on defineNuxtRouteMiddleware, navigateTo,
// and useState from '#app'.  We stub all three so the tests run in pure Node.

let _navigateToTarget: string | undefined;
let _authState = {isSignedIn: false, user: null as Record<string, unknown> | null, isLoading: false};

vi.mock('#app', () => {
  const navigateTo = vi.fn(async (url: string) => {
    _navigateToTarget = url;
  });

  const useState = vi.fn((_key: string) => ({
    get value() {
      return _authState;
    },
    set value(v: typeof _authState) {
      _authState = v;
    },
  }));

  // defineNuxtRouteMiddleware just returns the handler unchanged in tests
  const defineNuxtRouteMiddleware = vi.fn((fn: Function) => fn);

  return {navigateTo, useState, defineNuxtRouteMiddleware};
});

/** Build a fake `to` route object */
const makeTo = (fullPath = '/dashboard') => ({fullPath});

beforeEach(() => {
  _navigateToTarget = undefined;
  _authState = {isSignedIn: false, user: null, isLoading: false};
});

describe('defineThunderIDMiddleware', () => {
  describe('unauthenticated user', () => {
    it('redirects to /api/auth/signin by default', async () => {
      const middleware = defineThunderIDMiddleware();
      await (middleware as Function)(makeTo('/dashboard'), makeTo('/'));
      expect(_navigateToTarget).toContain('/api/auth/signin');
    });

    it('includes a returnTo query param encoding the original path', async () => {
      const middleware = defineThunderIDMiddleware();
      await (middleware as Function)(makeTo('/dashboard'), makeTo('/'));
      expect(_navigateToTarget).toContain('returnTo=');
      expect(_navigateToTarget).toContain(encodeURIComponent('/dashboard'));
    });

    it('honours a custom redirectTo', async () => {
      const middleware = defineThunderIDMiddleware({redirectTo: '/login'});
      await (middleware as Function)(makeTo('/secret'), makeTo('/'));
      expect(_navigateToTarget).toContain('/login');
    });
  });

  describe('authenticated user', () => {
    beforeEach(() => {
      _authState = {isSignedIn: true, user: {organizationId: 'org1', scopes: 'openid profile'}, isLoading: false};
    });

    it('allows access when signed in', async () => {
      const middleware = defineThunderIDMiddleware();
      await (middleware as Function)(makeTo('/dashboard'), makeTo('/'));
      // navigateTo should NOT have been called
      expect(_navigateToTarget).toBeUndefined();
    });

    it('allows access when required scopes are present', async () => {
      const middleware = defineThunderIDMiddleware({requireScopes: ['openid', 'profile']});
      await (middleware as Function)(makeTo('/dashboard'), makeTo('/'));
      expect(_navigateToTarget).toBeUndefined();
    });

    it('redirects when a required scope is missing', async () => {
      const middleware = defineThunderIDMiddleware({requireScopes: ['admin']});
      await (middleware as Function)(makeTo('/admin'), makeTo('/'));
      expect(_navigateToTarget).toBeDefined();
    });

    it('allows access when requireOrganization is true and organizationId is present', async () => {
      const middleware = defineThunderIDMiddleware({requireOrganization: true});
      await (middleware as Function)(makeTo('/org-page'), makeTo('/'));
      expect(_navigateToTarget).toBeUndefined();
    });

    it('redirects when requireOrganization is true but organizationId is absent', async () => {
      _authState.user = {email: 'user@example.com'}; // no organizationId
      const middleware = defineThunderIDMiddleware({requireOrganization: true});
      await (middleware as Function)(makeTo('/org-page'), makeTo('/'));
      expect(_navigateToTarget).toBeDefined();
    });
  });

  describe('ThunderIDMiddlewareOptions type', () => {
    it('accepts an empty options object', () => {
      const opts: ThunderIDMiddlewareOptions = {};
      expect(opts).toBeDefined();
    });

    it('accepts all fields', () => {
      const opts: ThunderIDMiddlewareOptions = {
        redirectTo: '/login',
        requireOrganization: true,
        requireScopes: ['openid', 'admin'],
      };
      expect(opts.redirectTo).toBe('/login');
      expect(opts.requireOrganization).toBe(true);
      expect(opts.requireScopes).toHaveLength(2);
    });
  });
});
