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

import {readBody, deleteCookie, setCookie} from 'h3';
import {describe, it, expect, vi, beforeEach} from 'vitest';

// ── Imports (after mocks) ─────────────────────────────────────────────────────

import signinHandler from '../../src/runtime/server/routes/auth/session/signin.post';
import {
  issueSessionCookie,
  createTempSessionToken,
  verifyTempSessionToken,
} from '../../src/runtime/server/utils/session';

// ── vi.hoisted — shared mutable state accessible inside mock factories ────────
const state = vi.hoisted(() => ({
  cookieStore: {} as Record<string, string>,
  tempCookie: undefined as string | undefined,
  mockAuthorizeUrl: 'https://api.asgardeo.io/t/org/oauth2/authorize?code_challenge=x',
  liveSession: null as any,
}));

const mockClientInstance = vi.hoisted(() => ({
  getAuthorizeRequestUrl: vi
    .fn<() => Promise<string>>()
    .mockResolvedValue('https://api.asgardeo.io/t/org/oauth2/authorize?code_challenge=x'),
  signIn: vi.fn<() => Promise<any>>().mockResolvedValue(undefined),
}));

// ── Module mocks ──────────────────────────────────────────────────────────────

vi.mock('h3', () => ({
  defineEventHandler: (fn: Function) => fn,
  readBody: vi.fn(),
  getCookie: vi.fn((_event: any, name: string) => (name.includes('temp') ? state.tempCookie : undefined)),
  setCookie: vi.fn((_event: any, name: string, value: string) => {
    state.cookieStore[name] = value;
  }),
  deleteCookie: vi.fn((_event: any, name: string) => {
    delete state.cookieStore[name];
  }),
  createError: vi.fn((opts: any) => Object.assign(new Error(opts.statusMessage), opts)),
}));

vi.mock('#imports', () => ({
  useRuntimeConfig: vi.fn(() => ({
    thunderid: {sessionSecret: 'test-secret-for-signin-route!!-32chars'},
    public: {
      thunderid: {afterSignInUrl: '/dashboard'},
    },
  })),
}));

vi.mock('../../src/runtime/server/ThunderIDNuxtClient', () => ({
  default: {
    getInstance: () => mockClientInstance,
  },
}));

vi.mock('../../src/runtime/server/utils/session', () => ({
  issueSessionCookie: vi.fn().mockResolvedValue(undefined),
  createTempSessionToken: vi.fn().mockResolvedValue('temp-token-123'),
  verifyTempSessionToken: vi.fn().mockResolvedValue({sessionId: 'session-from-temp-cookie'}),
  getTempSessionCookieName: vi.fn().mockReturnValue('thunderid-temp-session'),
  getTempSessionCookieOptions: vi.fn().mockReturnValue({httpOnly: true, path: '/'}),
}));

vi.mock('../../src/runtime/server/utils/serverSession', () => ({
  useServerSession: vi.fn().mockImplementation(() => Promise.resolve(state.liveSession)),
}));

vi.mock('@thunderid/node', () => ({
  generateSessionId: vi.fn().mockReturnValue('new-session-id'),
  isEmpty: vi.fn((obj: any) => !obj || Object.keys(obj).length === 0),
  EmbeddedSignInFlowStatus: {SuccessCompleted: 'SUCCESS_COMPLETED'},
}));

// ── Helpers ───────────────────────────────────────────────────────────────────

const mockEvent = {};

function resetState() {
  state.cookieStore = {};
  state.tempCookie = undefined;
  state.liveSession = null;
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('POST /api/auth/signin', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetState();
  });

  describe('flow initiation (empty payload)', () => {
    it('returns signInUrl when no payload is provided', async () => {
      vi.mocked(readBody).mockResolvedValue({});

      const result = await (signinHandler as any)(mockEvent);

      expect(result.success).toBe(true);
      expect(result.data.signInUrl).toBe('https://api.asgardeo.io/t/org/oauth2/authorize?code_challenge=x');
    });

    it('mints a temp session cookie when no existing session', async () => {
      vi.mocked(readBody).mockResolvedValue({});

      await (signinHandler as any)(mockEvent);

      // Signature: createTempSessionToken(sessionId, sessionSecret, returnTo?)
      expect(createTempSessionToken).toHaveBeenCalledWith(expect.any(String), expect.any(String));
      expect(setCookie).toHaveBeenCalled();
    });

    it('reuses sessionId from existing temp cookie', async () => {
      state.tempCookie = 'existing-temp-token';
      vi.mocked(readBody).mockResolvedValue({});

      await (signinHandler as any)(mockEvent);

      expect(verifyTempSessionToken).toHaveBeenCalledWith('existing-temp-token', expect.any(String));
    });

    it('reuses sessionId from live session when present', async () => {
      state.liveSession = {sessionId: 'live-session-id'};
      vi.mocked(readBody).mockResolvedValue({});

      await (signinHandler as any)(mockEvent);

      // No temp cookie needed when live session exists
      expect(createTempSessionToken).not.toHaveBeenCalled();
    });
  });

  describe('embedded flow step execution (intermediate step)', () => {
    it('returns flow step data when flow is not yet complete', async () => {
      const flowPayload = {
        flowId: 'flow-abc',
        selectedAuthenticator: {authenticatorId: 'BasicAuthenticator'},
        flowInputs: [{name: 'username', value: 'user@example.com'}],
      };
      const stepResponse = {
        flowId: 'flow-abc',
        flowStatus: 'INCOMPLETE',
        nextStep: {authenticators: []},
      };
      mockClientInstance.signIn.mockResolvedValueOnce(stepResponse);
      vi.mocked(readBody).mockResolvedValue({payload: flowPayload});

      const result = await (signinHandler as any)(mockEvent);

      expect(result).toEqual({data: stepResponse, success: true});
    });
  });

  describe('embedded flow completion (SUCCESS_COMPLETED)', () => {
    it('exchanges code and issues session cookie on flow completion', async () => {
      const flowPayload = {
        flowId: 'flow-abc',
        selectedAuthenticator: {authenticatorId: 'BasicAuthenticator'},
        flowInputs: [
          {name: 'username', value: 'user@example.com'},
          {name: 'password', value: 'pass'},
        ],
      };
      const completedResponse = {
        flowStatus: 'SUCCESS_COMPLETED',
        authData: {
          code: 'auth-code-xyz',
          state: 'state-123',
          session_state: 'sess-state-abc',
        },
      };
      const tokenResponse = {accessToken: 'at-new', idToken: 'id-token'};

      // First signIn call → completed flow response; second → token response.
      mockClientInstance.signIn.mockResolvedValueOnce(completedResponse).mockResolvedValueOnce(tokenResponse);

      vi.mocked(readBody).mockResolvedValue({payload: flowPayload});

      const result = await (signinHandler as any)(mockEvent);

      expect(issueSessionCookie).toHaveBeenCalled();
      expect(deleteCookie).toHaveBeenCalled();
      expect(result).toEqual({data: {afterSignInUrl: '/dashboard'}, success: true});
    });

    it('throws 502 when authorization code is missing from completed flow', async () => {
      const flowPayload = {flowId: 'flow-abc', selectedAuthenticator: {authenticatorId: 'Basic'}, flowInputs: []};
      const completedNoCode = {flowStatus: 'SUCCESS_COMPLETED', authData: {}};

      mockClientInstance.signIn.mockResolvedValueOnce(completedNoCode);
      vi.mocked(readBody).mockResolvedValue({payload: flowPayload});

      await expect((signinHandler as any)(mockEvent)).rejects.toMatchObject({
        statusCode: 502,
      });
    });
  });
});
