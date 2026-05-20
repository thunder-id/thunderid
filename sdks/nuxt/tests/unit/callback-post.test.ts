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

import {readBody, deleteCookie} from 'h3';
import {describe, it, expect, vi, beforeEach} from 'vitest';

// ── Imports (after mocks) ─────────────────────────────────────────────────────

import callbackHandler from '../../src/runtime/server/routes/auth/session/callback.post';
import {issueSessionCookie, verifyTempSessionToken} from '../../src/runtime/server/utils/session';

// ── vi.hoisted ────────────────────────────────────────────────────────────────
const state = vi.hoisted(() => ({
  tempCookie: undefined as string | undefined,
  cookieStore: {} as Record<string, string>,
}));

const mockClientInstance = vi.hoisted(() => ({
  signIn: vi.fn<() => Promise<any>>().mockResolvedValue({accessToken: 'at-xyz', idToken: 'id-token-xyz'}),
}));

// ── Module mocks ──────────────────────────────────────────────────────────────

vi.mock('h3', () => ({
  defineEventHandler: (fn: Function) => fn,
  readBody: vi.fn(),
  getCookie: vi.fn((_event: any, name: string) => (name.includes('temp') ? state.tempCookie : undefined)),
  deleteCookie: vi.fn((_event: any, name: string) => {
    delete state.cookieStore[name];
  }),
  createError: vi.fn((opts: any) => Object.assign(new Error(opts.statusMessage), opts)),
}));

vi.mock('#imports', () => ({
  useRuntimeConfig: vi.fn(() => ({
    thunderid: {sessionSecret: 'test-secret-for-callback-route!!-32chars'},
    public: {
      thunderid: {afterSignInUrl: '/home'},
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
  verifyTempSessionToken: vi.fn().mockResolvedValue({sessionId: 'session-from-temp'}),
  getTempSessionCookieName: vi.fn().mockReturnValue('thunderid-temp-session'),
  getTempSessionCookieOptions: vi.fn().mockReturnValue({httpOnly: true, path: '/'}),
}));

// ── Helpers ───────────────────────────────────────────────────────────────────

const mockEvent = {};

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('POST /api/auth/callback', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    state.tempCookie = 'valid-temp-token';
    state.cookieStore = {};
    mockClientInstance.signIn.mockResolvedValue({accessToken: 'at-xyz', idToken: 'id-token-xyz'});
  });

  describe('happy path', () => {
    it('exchanges code for tokens and returns redirectUrl on success', async () => {
      vi.mocked(readBody).mockResolvedValue({
        code: 'auth-code-abc',
        state: 'state-xyz',
        sessionState: 'sess-state-abc',
      });

      const result = await (callbackHandler as any)(mockEvent);

      expect(issueSessionCookie).toHaveBeenCalled();
      expect(deleteCookie).toHaveBeenCalled();
      expect(result).toEqual({redirectUrl: '/home', success: true});
    });

    it('verifies the temp session cookie to resolve sessionId', async () => {
      vi.mocked(readBody).mockResolvedValue({code: 'code-123', state: 'state-abc'});

      await (callbackHandler as any)(mockEvent);

      expect(verifyTempSessionToken).toHaveBeenCalledWith('valid-temp-token', expect.any(String));
    });
  });

  describe('error cases', () => {
    it('throws 400 when code is missing', async () => {
      vi.mocked(readBody).mockResolvedValue({state: 'state-only'});

      await expect((callbackHandler as any)(mockEvent)).rejects.toMatchObject({
        statusCode: 400,
      });
    });

    it('throws 400 when temp session cookie is absent', async () => {
      state.tempCookie = undefined;
      vi.mocked(readBody).mockResolvedValue({code: 'auth-code', state: 'state-123'});

      await expect((callbackHandler as any)(mockEvent)).rejects.toMatchObject({
        statusCode: 400,
      });
    });

    it('throws 400 when temp session cookie is expired or tampered', async () => {
      vi.mocked(verifyTempSessionToken).mockRejectedValueOnce(new Error('expired'));
      vi.mocked(readBody).mockResolvedValue({code: 'auth-code', state: 'state-123'});

      await expect((callbackHandler as any)(mockEvent)).rejects.toMatchObject({
        statusCode: 400,
      });
    });

    it('returns {success: false, error} when token exchange fails', async () => {
      mockClientInstance.signIn.mockRejectedValueOnce(new Error('token exchange error'));
      vi.mocked(readBody).mockResolvedValue({code: 'auth-code', state: 'state-123'});

      const result = await (callbackHandler as any)(mockEvent);

      expect(result).toMatchObject({success: false, error: expect.stringContaining('token exchange error')});
    });

    it('returns {success: false, error} when issueSessionCookie fails', async () => {
      vi.mocked(issueSessionCookie).mockRejectedValueOnce(new Error('cookie write error'));
      vi.mocked(readBody).mockResolvedValue({code: 'auth-code', state: 'state-123'});

      const result = await (callbackHandler as any)(mockEvent);

      expect(result).toMatchObject({success: false, error: expect.stringContaining('cookie write error')});
    });
  });
});
