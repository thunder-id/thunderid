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

// src/server/actions/__tests__/getAccessToken.test.ts
import {cookies} from 'next/headers';
import {describe, it, expect, vi, beforeEach, afterEach, Mock} from 'vitest';

// SUT
import SessionManager from '../../../utils/SessionManager';
import getAccessToken from '../getAccessToken';

// Pull mocked modules so we can control them

// ---- Mocks ----
vi.mock('next/headers', () => ({
  cookies: vi.fn(),
}));

vi.mock('../../../utils/SessionManager', () => ({
  default: {
    getSessionCookieName: vi.fn(),
    verifySessionToken: vi.fn(),
  },
}));

// A tiny helper type for the cookie store the SUT expects
interface CookieVal {
  value: string;
}
interface CookieStore {
  get: (name: string) => CookieVal | undefined;
}

describe('getAccessToken', () => {
  const SESSION_COOKIE_NAME = 'app_session';

  const makeCookieStore = (map: Record<string, string | undefined>): CookieStore => ({
    get: (name: string): CookieVal | undefined => {
      const v: string | undefined = map[name];
      return typeof v === 'string' ? {value: v} : undefined;
    },
  });

  beforeEach(() => {
    vi.resetAllMocks();

    // Default cookie name
    (SessionManager.getSessionCookieName as unknown as Mock).mockReturnValue(SESSION_COOKIE_NAME);

    // Default cookies() returns an object with get()
    (cookies as unknown as Mock).mockResolvedValue(makeCookieStore({}));

    // Default verification returns an object with a string token
    (SessionManager.verifySessionToken as unknown as Mock).mockResolvedValue({
      accessToken: 'tok-123',
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should return the access token when the session cookie exists and verification succeeds', async () => {
    // Arrange
    (cookies as unknown as Mock).mockResolvedValue(makeCookieStore({[SESSION_COOKIE_NAME]: 'signed.jwt.token'}));

    // Act
    const token: string | undefined = await getAccessToken();

    // Assert
    expect(SessionManager.getSessionCookieName).toHaveBeenCalledTimes(1);
    expect(cookies).toHaveBeenCalledTimes(1);
    expect(SessionManager.verifySessionToken).toHaveBeenCalledWith('signed.jwt.token');
    expect(token).toBe('tok-123');
  });

  it('should return undefined when the session cookie is missing', async () => {
    // Arrange: no cookie present (default makeCookieStore({}))
    // Act
    const token: string | undefined = await getAccessToken();

    // Assert
    expect(SessionManager.getSessionCookieName).toHaveBeenCalledTimes(1);
    expect(SessionManager.verifySessionToken).not.toHaveBeenCalled();
    expect(token).toBeUndefined();
  });

  it('should return undefined when the session cookie value is an empty string', async () => {
    (cookies as unknown as Mock).mockResolvedValue(makeCookieStore({[SESSION_COOKIE_NAME]: ''}));

    const token: string | undefined = await getAccessToken();

    expect(SessionManager.verifySessionToken).not.toHaveBeenCalled();
    expect(token).toBeUndefined();
  });

  it('should return undefined when verifySessionToken throws (invalid or expired session)', async () => {
    (cookies as unknown as Mock).mockResolvedValue(makeCookieStore({[SESSION_COOKIE_NAME]: 'bad.token'}));
    (SessionManager.verifySessionToken as unknown as Mock).mockRejectedValue(new Error('invalid signature'));

    const token: string | undefined = await getAccessToken();

    expect(SessionManager.verifySessionToken).toHaveBeenCalledWith('bad.token');
    expect(token).toBeUndefined();
  });

  it('should return undefined when verification succeeds but accessToken is missing', async () => {
    (cookies as unknown as Mock).mockResolvedValue(makeCookieStore({[SESSION_COOKIE_NAME]: 'signed.jwt.token'}));
    (SessionManager.verifySessionToken as unknown as Mock).mockResolvedValue({
      // no accessToken field
      sub: 'user@tenant',
    });

    const token: string | undefined = await getAccessToken();

    expect(token).toBeUndefined();
  });
});
