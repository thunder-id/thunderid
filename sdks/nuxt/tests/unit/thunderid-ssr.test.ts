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

/* eslint-disable @typescript-eslint/typedef, sort-keys, @typescript-eslint/explicit-function-return-type */

import {getCookie, getRequestURL} from 'h3';
import {describe, it, expect, vi, beforeAll, beforeEach} from 'vitest';

// ── Imports (must come after vi.mock declarations) ───────────────────────────

import {verifySessionToken} from '../../src/runtime/server/utils/session';
import {useRuntimeConfig} from '#imports';
// Side-effect import triggers defineNitroPlugin, capturing the factory
import '../../src/runtime/server/plugins/thunderid-ssr';

// ── vi.hoisted — objects that need to be accessible inside vi.mock factories ──
// (vi.hoisted runs before any mock factory or import, avoiding TDZ issues)

const captured = vi.hoisted(() => ({
  pluginFactory: undefined as ((nitro: any) => void) | undefined,
}));

const mockClient = vi.hoisted(() => ({
  isInitialized: false as boolean,
  initialize: vi.fn<() => Promise<void>>().mockResolvedValue(undefined),
  getUser: vi.fn<(sessionId: string) => Promise<any>>().mockResolvedValue({sub: 'user-123', email: 'test@example.com'}),
  getUserProfile: vi.fn<(sessionId: string) => Promise<any>>().mockResolvedValue({
    profile: {sub: 'user-123', email: 'test@example.com'},
    flattenedProfile: {email: 'test@example.com'},
    schemas: [],
  }),
  getMyOrganizations: vi
    .fn<(sessionId: string) => Promise<any>>()
    .mockResolvedValue([{id: 'org-1', name: 'Test Org', orgHandle: 'test-org'}]),
  getCurrentOrganization: vi.fn<(sessionId: string) => Promise<any>>().mockResolvedValue({
    id: 'org-1',
    name: 'Test Org',
    orgHandle: 'test-org',
  }),
  getBrandingPreference: vi.fn<(config: any) => Promise<any>>().mockResolvedValue({organizationName: 'TestOrg'}),
  getDecodedIdToken: vi.fn<(sessionId: string) => Promise<any>>().mockResolvedValue({sub: 'user-123'}),
}));

// ── Module mocks ──────────────────────────────────────────────────────────────

vi.mock('nitropack/runtime', () => ({
  defineNitroPlugin: (factory: (nitro: any) => void) => {
    captured.pluginFactory = factory;
    return factory;
  },
}));

vi.mock('h3', () => ({
  getCookie: vi.fn(),
  getRequestURL: vi.fn(),
}));

vi.mock('#imports', () => ({
  useRuntimeConfig: vi.fn(() => ({
    public: {thunderid: {baseUrl: 'https://api.asgardeo.io/t/testorg', preferences: undefined}},
  })),
}));

vi.mock('../../src/runtime/server/ThunderIDNuxtClient', () => ({
  default: {
    getInstance: () => mockClient,
  },
}));

vi.mock('../../src/runtime/server/utils/session', () => ({
  verifySessionToken: vi.fn(),
  getSessionCookieName: vi.fn(() => 'thunderid-session'),
}));

vi.mock('../../src/runtime/utils/log', () => ({
  createLogger: vi.fn(() => ({
    error: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
    info: vi.fn(),
  })),
}));

// augments.d.ts is a pure declaration file — no runtime exports needed
vi.mock('../../src/runtime/types/augments.d', () => ({}));

// ── Constants ─────────────────────────────────────────────────────────────────

const MOCK_SESSION = {
  sessionId: 'test-session-id',
  sub: 'user-123',
  accessToken: 'test-access-token',
  scopes: 'openid profile',
  iat: Math.floor(Date.now() / 1000),
  exp: Math.floor(Date.now() / 1000) + 3600,
};

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('thunderid-ssr Nitro plugin', () => {
  let requestHandler: ((event: any) => Promise<void>) | undefined;

  beforeAll(() => {
    // Invoke the captured plugin factory with a minimal nitro mock so the
    // 'request' hook handler is registered and available for all tests.
    const mockNitro = {
      hooks: {
        hook: vi.fn((eventName: string, handler: any) => {
          if (eventName === 'request') {
            requestHandler = handler;
          }
        }),
      },
    };
    captured.pluginFactory!(mockNitro);
  });

  beforeEach(() => {
    vi.clearAllMocks();

    // Mark the client as already initialized so the initialization guard in
    // the plugin is skipped; tests focus on SSR data-resolution behavior.
    mockClient.isInitialized = true;
    mockClient.initialize.mockResolvedValue(undefined);
    mockClient.getUser.mockResolvedValue({sub: 'user-123', email: 'test@example.com'});
    mockClient.getUserProfile.mockResolvedValue({
      profile: {sub: 'user-123', email: 'test@example.com'},
      flattenedProfile: {email: 'test@example.com'},
      schemas: [],
    });
    mockClient.getMyOrganizations.mockResolvedValue([{id: 'org-1', name: 'Test Org', orgHandle: 'test-org'}]);
    mockClient.getCurrentOrganization.mockResolvedValue({
      id: 'org-1',
      name: 'Test Org',
      orgHandle: 'test-org',
    });
    mockClient.getBrandingPreference.mockResolvedValue({organizationName: 'TestOrg'});
    mockClient.getDecodedIdToken.mockResolvedValue({sub: 'user-123'});

    // Default: no session cookie, root path
    vi.mocked(getCookie).mockReturnValue(undefined);
    vi.mocked(getRequestURL).mockReturnValue(new URL('http://localhost:3000/'));

    // Default runtime config — all preferences enabled (undefined = default true).
    vi.mocked(useRuntimeConfig).mockReturnValue({
      public: {
        thunderid: {baseUrl: 'https://api.asgardeo.io/t/testorg', clientId: 'test-client-id', preferences: undefined},
      },
    } as any);

    // Default session verification — returns the mock session
    vi.mocked(verifySessionToken).mockResolvedValue(MOCK_SESSION as any);
  });

  /** Helper: set up event mocks and call the request handler. */
  async function callHandler(path = '/', cookieValue?: string | null): Promise<any> {
    vi.mocked(getCookie).mockReturnValue(cookieValue ?? undefined);
    vi.mocked(getRequestURL).mockReturnValue(new URL(`http://localhost:3000${path}`));
    // event.path is read by the plugin for its early-return path filter.
    const event: any = {context: {}, path};
    await requestHandler!(event);
    return event;
  }

  // ── Early-exit paths ────────────────────────────────────────────────────

  it('returns early for /api/ paths without setting thunderid context', async () => {
    const event = await callHandler('/api/auth/session');
    expect(event.context.thunderid).toBeUndefined();
  });

  it('returns early for /_nuxt/ internal paths', async () => {
    const event = await callHandler('/_nuxt/chunks/some-file.js');
    expect(event.context.thunderid).toBeUndefined();
  });

  it('returns early for /__nuxt_ prefixed paths', async () => {
    const event = await callHandler('/__nuxt_error');
    expect(event.context.thunderid).toBeUndefined();
  });

  // ── Unauthenticated paths ────────────────────────────────────────────────

  it('sets isSignedIn:false when no session cookie is present', async () => {
    const event = await callHandler('/');
    expect(event.context.thunderid).toEqual({session: null, isSignedIn: false});
  });

  it('sets isSignedIn:false when session token verification fails', async () => {
    vi.mocked(verifySessionToken).mockRejectedValueOnce(new Error('invalid token'));
    const event = await callHandler('/', 'bad-token-value');
    expect(event.context.thunderid).toEqual({session: null, isSignedIn: false});
  });

  // ── Authenticated — full SSR data ────────────────────────────────────────

  it('populates full SSR data on a valid session with default preferences', async () => {
    const event = await callHandler('/', 'valid-cookie');

    expect(event.context.thunderid.isSignedIn).toBe(true);
    const {ssr} = event.context.thunderid;
    expect(ssr).toBeDefined();
    expect(ssr.isSignedIn).toBe(true);
    expect(ssr.session).toEqual(MOCK_SESSION);
    expect(ssr.user).toEqual({sub: 'user-123', email: 'test@example.com'});
    expect(ssr.userProfile).toBeDefined();
    expect(ssr.myOrganizations).toHaveLength(1);
    expect(ssr.currentOrganization).toEqual({id: 'org-1', name: 'Test Org', orgHandle: 'test-org'});
    expect(ssr.brandingPreference).toEqual({organizationName: 'TestOrg'});
  });

  it('calls all client methods with the correct session ID', async () => {
    await callHandler('/', 'valid-cookie');

    expect(mockClient.getUser).toHaveBeenCalledWith(MOCK_SESSION.sessionId);
    expect(mockClient.getUserProfile).toHaveBeenCalledWith(MOCK_SESSION.sessionId);
    expect(mockClient.getMyOrganizations).toHaveBeenCalledWith(MOCK_SESSION.sessionId);
    expect(mockClient.getCurrentOrganization).toHaveBeenCalledWith(MOCK_SESSION.sessionId);
    expect(mockClient.getBrandingPreference).toHaveBeenCalled();
  });

  it('writes legacy __thunderidAuth to event context for backwards compatibility', async () => {
    const event = await callHandler('/', 'valid-cookie');

    expect(event.context.__thunderidAuth).toBeDefined();
    expect(event.context.__thunderidAuth.isSignedIn).toBe(true);
    expect(event.context.__thunderidAuth.user).toEqual({sub: 'user-123', email: 'test@example.com'});
  });

  // ── Preference gating ────────────────────────────────────────────────────

  it('skips getUserProfile when preferences.user.fetchUserProfile is false', async () => {
    vi.mocked(useRuntimeConfig).mockReturnValue({
      public: {
        thunderid: {
          baseUrl: 'https://api.asgardeo.io/t/testorg',
          preferences: {user: {fetchUserProfile: false}},
        },
      },
    } as any);

    const event = await callHandler('/', 'valid-cookie');

    expect(mockClient.getUserProfile).not.toHaveBeenCalled();
    expect(event.context.thunderid.ssr.userProfile).toBeNull();
    // other fields should still be populated
    expect(event.context.thunderid.ssr.myOrganizations).toHaveLength(1);
    expect(event.context.thunderid.ssr.brandingPreference).toBeDefined();
  });

  it('skips org fetches when preferences.user.fetchOrganizations is false', async () => {
    vi.mocked(useRuntimeConfig).mockReturnValue({
      public: {
        thunderid: {
          baseUrl: 'https://api.asgardeo.io/t/testorg',
          preferences: {user: {fetchOrganizations: false}},
        },
      },
    } as any);

    const event = await callHandler('/', 'valid-cookie');

    expect(mockClient.getMyOrganizations).not.toHaveBeenCalled();
    expect(mockClient.getCurrentOrganization).not.toHaveBeenCalled();
    expect(event.context.thunderid.ssr.myOrganizations).toEqual([]);
    expect(event.context.thunderid.ssr.currentOrganization).toBeNull();
    // user and branding should still be populated
    expect(event.context.thunderid.ssr.user).toBeDefined();
    expect(event.context.thunderid.ssr.brandingPreference).toBeDefined();
  });

  it('skips branding fetch when preferences.theme.inheritFromBranding is false', async () => {
    vi.mocked(useRuntimeConfig).mockReturnValue({
      public: {
        thunderid: {
          baseUrl: 'https://api.asgardeo.io/t/testorg',
          preferences: {theme: {inheritFromBranding: false}},
        },
      },
    } as any);

    const event = await callHandler('/', 'valid-cookie');

    expect(mockClient.getBrandingPreference).not.toHaveBeenCalled();
    expect(event.context.thunderid.ssr.brandingPreference).toBeNull();
    // other fields should still be populated
    expect(event.context.thunderid.ssr.user).toBeDefined();
    expect(event.context.thunderid.ssr.myOrganizations).toHaveLength(1);
  });

  // ── Non-fatal partial failures ────────────────────────────────────────────

  it('still writes SSR data when getUserProfile throws (non-fatal)', async () => {
    mockClient.getUserProfile.mockRejectedValueOnce(new Error('SCIM2 error'));

    const event = await callHandler('/', 'valid-cookie');

    expect(event.context.thunderid.ssr).toBeDefined();
    expect(event.context.thunderid.ssr.isSignedIn).toBe(true);
    expect(event.context.thunderid.ssr.userProfile).toBeNull();
    // user fetch ran independently and should succeed
    expect(event.context.thunderid.ssr.user).toEqual({sub: 'user-123', email: 'test@example.com'});
  });

  it('still writes SSR data when getMyOrganizations throws (non-fatal)', async () => {
    mockClient.getMyOrganizations.mockRejectedValueOnce(new Error('org fetch error'));

    const event = await callHandler('/', 'valid-cookie');

    expect(event.context.thunderid.ssr.isSignedIn).toBe(true);
    expect(event.context.thunderid.ssr.myOrganizations).toEqual([]);
    expect(event.context.thunderid.ssr.user).toBeDefined();
  });

  it('still writes SSR data when getBrandingPreference throws (non-fatal)', async () => {
    mockClient.getBrandingPreference.mockRejectedValueOnce(new Error('branding error'));

    const event = await callHandler('/', 'valid-cookie');

    expect(event.context.thunderid.ssr.isSignedIn).toBe(true);
    expect(event.context.thunderid.ssr.brandingPreference).toBeNull();
    expect(event.context.thunderid.ssr.user).toBeDefined();
  });

  // ── Org-scoped base URL resolution ────────────────────────────────────────

  it('sets resolvedBaseUrl to baseUrl/o when session has organizationId', async () => {
    vi.mocked(verifySessionToken).mockResolvedValueOnce({
      ...MOCK_SESSION,
      organizationId: 'org-123',
    } as any);

    const event = await callHandler('/', 'valid-cookie');

    expect(event.context.thunderid.ssr.resolvedBaseUrl).toBe('https://api.asgardeo.io/t/testorg/o');
  });

  it('uses plain baseUrl when session has no organizationId and ID token has no user_org', async () => {
    mockClient.getDecodedIdToken.mockResolvedValueOnce({sub: 'user-123'});
    // MOCK_SESSION has no organizationId

    const event = await callHandler('/', 'valid-cookie');

    expect(event.context.thunderid.ssr.resolvedBaseUrl).toBe('https://api.asgardeo.io/t/testorg');
  });

  it('sets resolvedBaseUrl to baseUrl/o when ID token contains user_org claim', async () => {
    mockClient.getDecodedIdToken.mockResolvedValueOnce({sub: 'user-123', user_org: 'org-from-token'});

    const event = await callHandler('/', 'valid-cookie');

    expect(event.context.thunderid.ssr.resolvedBaseUrl).toBe('https://api.asgardeo.io/t/testorg/o');
  });
});
