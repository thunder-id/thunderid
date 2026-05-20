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

import {describe, it, expect, vi} from 'vitest';
import ThunderIDAPIError from '../../errors/ThunderIDAPIError';
import getScim2Me from '../getScim2Me';

// Mock user data
const mockUser: Record<string, unknown> = {
  email: 'test@example.com',
  familyName: 'User',
  givenName: 'Test',
  id: '123',
  username: 'testuser',
};

describe('getScim2Me', () => {
  it('should fetch user profile successfully with default fetch', async () => {
    // Mock fetch
    const mockFetch: typeof fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockUser),
      ok: true,
      status: 200,
      statusText: 'OK',
      text: () => Promise.resolve(JSON.stringify(mockUser)),
    });

    // Replace global fetch
    global.fetch = mockFetch;

    const result: Record<string, unknown> = await getScim2Me({
      url: 'https://api.asgardeo.io/t/test/scim2/Me',
    });

    expect(result).toEqual(mockUser);
    expect(mockFetch).toHaveBeenCalledWith('https://api.asgardeo.io/t/test/scim2/Me', {
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/scim+json',
      },
      method: 'GET',
    });
  });

  it('should use custom fetcher when provided', async () => {
    const customFetcher: typeof fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockUser),
      ok: true,
      status: 200,
      statusText: 'OK',
      text: () => Promise.resolve(JSON.stringify(mockUser)),
    });

    const result: Record<string, unknown> = await getScim2Me({
      fetcher: customFetcher,
      url: 'https://api.asgardeo.io/t/test/scim2/Me',
    });

    expect(result).toEqual(mockUser);
    expect(customFetcher).toHaveBeenCalledWith('https://api.asgardeo.io/t/test/scim2/Me', {
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/scim+json',
      },
      method: 'GET',
    });
  });

  it('should handle errors thrown directly by custom fetcher', async (): Promise<void> => {
    const customFetcher: typeof fetch = vi.fn().mockImplementation(() => {
      throw new Error('Custom fetcher failure');
    });

    await expect(
      getScim2Me({
        fetcher: customFetcher,
        url: 'https://api.asgardeo.io/t/test/scim2/Me',
      }),
    ).rejects.toThrow(ThunderIDAPIError);
    await expect(
      getScim2Me({
        fetcher: customFetcher,
        url: 'https://api.asgardeo.io/t/test/scim2/Me',
      }),
    ).rejects.toThrow('Network or parsing error: Custom fetcher failure');
  });

  it('should throw ThunderIDAPIError for invalid URL', async () => {
    await expect(
      getScim2Me({
        url: 'invalid-url',
      }),
    ).rejects.toThrow(ThunderIDAPIError);

    await expect(
      getScim2Me({
        baseUrl: 'invalid-url',
      }),
    ).rejects.toThrow(ThunderIDAPIError);
  });

  it('should throw ThunderIDAPIError for undefined URL', async () => {
    await expect(getScim2Me({})).rejects.toThrow(ThunderIDAPIError);

    const error: ThunderIDAPIError = await getScim2Me({
      baseUrl: undefined,
      url: undefined,
    }).catch((e: ThunderIDAPIError) => e);

    expect(error.name).toBe('ThunderIDAPIError');
    expect(error.code).toBe('getScim2Me-ValidationError-001');
  });

  it('should throw ThunderIDAPIError for empty string URL', async () => {
    await expect(
      getScim2Me({
        url: '',
      }),
    ).rejects.toThrow(ThunderIDAPIError);

    const error: ThunderIDAPIError = await getScim2Me({
      url: '',
    }).catch((e: ThunderIDAPIError) => e);

    expect(error.name).toBe('ThunderIDAPIError');
    expect(error.code).toBe('getScim2Me-ValidationError-001');
  });

  it('should throw ThunderIDAPIError for failed response', async () => {
    const mockFetch: typeof fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      statusText: 'Not Found',
      text: () => Promise.resolve('User not found'),
    });

    global.fetch = mockFetch;

    await expect(
      getScim2Me({
        url: 'https://api.asgardeo.io/t/test/scim2/Me',
      }),
    ).rejects.toThrow(ThunderIDAPIError);
  });

  it('should handle network errors', async () => {
    const mockFetch: typeof fetch = vi.fn().mockRejectedValue(new Error('Network error'));

    global.fetch = mockFetch;

    await expect(
      getScim2Me({
        url: 'https://api.asgardeo.io/t/test/scim2/Me',
      }),
    ).rejects.toThrow(ThunderIDAPIError);
  });

  it('should handle non-Error rejections', async (): Promise<void> => {
    global.fetch = vi.fn().mockRejectedValue('unexpected failure');

    const baseUrl = 'https://api.asgardeo.io/t/dxlab';

    await expect(getScim2Me({baseUrl})).rejects.toThrow(ThunderIDAPIError);
    await expect(getScim2Me({baseUrl})).rejects.toThrow('Network or parsing error: Unknown error');
  });

  it('should pass through custom headers', async () => {
    const mockFetch: typeof fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockUser),
      ok: true,
      status: 200,
      statusText: 'OK',
      text: () => Promise.resolve(JSON.stringify(mockUser)),
    });

    global.fetch = mockFetch;
    const customHeaders: Record<string, string> = {
      Authorization: 'Bearer token',
      'X-Custom-Header': 'custom-value',
    };

    await getScim2Me({
      headers: customHeaders,
      url: 'https://api.asgardeo.io/t/test/scim2/Me',
    });

    expect(mockFetch).toHaveBeenCalledWith('https://api.asgardeo.io/t/test/scim2/Me', {
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/scim+json',
        ...customHeaders,
      },
      method: 'GET',
    });
  });

  it('should default to baseUrl if url is not provided', async () => {
    const mockFetch: typeof fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockUser),
      ok: true,
      status: 200,
      statusText: 'OK',
      text: () => Promise.resolve(JSON.stringify(mockUser)),
    });
    global.fetch = mockFetch;

    const baseUrl = 'https://api.asgardeo.io/t/test';
    await getScim2Me({
      baseUrl,
    });
    expect(mockFetch).toHaveBeenCalledWith(`${baseUrl}/scim2/Me`, {
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/scim+json',
      },
      method: 'GET',
    });
  });
});
