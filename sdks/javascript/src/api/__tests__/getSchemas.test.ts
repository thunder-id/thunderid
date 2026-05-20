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

import {describe, it, expect, vi, beforeEach} from 'vitest';
import ThunderIDAPIError from '../../errors/ThunderIDAPIError';
import type {Schema} from '../../models/scim2-schema';
import getSchemas from '../getSchemas';

describe('getSchemas', (): void => {
  beforeEach((): void => {
    vi.resetAllMocks();
  });

  it('should fetch schemas successfully (default fetch)', async (): Promise<void> => {
    const mockSchemas: Schema[] = [
      {id: 'urn:ietf:params:scim:schemas:core:2.0:User', name: 'User'} as any,
      {id: 'urn:ietf:params:scim:schemas:core:2.0:Group', name: 'Group'} as any,
    ];

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockSchemas),
      ok: true,
    });

    const url = 'https://api.asgardeo.io/t/demo/scim2/Schemas';
    const result: Schema[] = await getSchemas({url});

    expect(fetch).toHaveBeenCalledWith(url, {
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      method: 'GET',
    });
    expect(result).toEqual(mockSchemas);
  });

  it('should fall back to baseUrl when url is not provided', async (): Promise<void> => {
    const mockSchemas: Schema[] = [{id: 'core', name: 'Core'} as any];

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockSchemas),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const result: Schema[] = await getSchemas({baseUrl});

    expect(fetch).toHaveBeenCalledWith(`${baseUrl}/scim2/Schemas`, {
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      method: 'GET',
    });
    expect(result).toEqual(mockSchemas);
  });

  it('should use custom fetcher when provided', async (): Promise<void> => {
    const mockSchemas: Schema[] = [{id: 'ext', name: 'Extension'} as any];

    const customFetcher: typeof fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockSchemas),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const result: Schema[] = await getSchemas({baseUrl, fetcher: customFetcher});

    expect(result).toEqual(mockSchemas);
    expect(customFetcher).toHaveBeenCalledWith(
      `${baseUrl}/scim2/Schemas`,
      expect.objectContaining({
        headers: expect.objectContaining({
          Accept: 'application/json',
          'Content-Type': 'application/json',
        }),
        method: 'GET',
      }),
    );
  });

  it('should handle errors thrown directly by custom fetcher', async (): Promise<void> => {
    const customFetcher: typeof fetch = vi.fn().mockImplementation(() => {
      throw new Error('Custom fetcher failure');
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';

    await expect(getSchemas({baseUrl, fetcher: customFetcher})).rejects.toThrow(
      'Network or parsing error: Custom fetcher failure',
    );
  });

  it('should throw ThunderIDAPIError for invalid URL', async (): Promise<void> => {
    await expect(getSchemas({url: 'invalid-url' as any})).rejects.toThrow(ThunderIDAPIError);
    await expect(getSchemas({url: 'invalid-url' as any})).rejects.toThrow('Invalid URL provided.');
  });

  it('should throw ThunderIDAPIError when both url and baseUrl are undefined', async (): Promise<void> => {
    await expect(getSchemas({baseUrl: undefined as any, url: undefined as any})).rejects.toThrow(ThunderIDAPIError);
    await expect(getSchemas({baseUrl: undefined as any, url: undefined as any})).rejects.toThrow(
      'Invalid URL provided.',
    );
  });

  it('should throw ThunderIDAPIError when both url and baseUrl are empty strings', async (): Promise<void> => {
    await expect(getSchemas({baseUrl: '', url: ''})).rejects.toThrow(ThunderIDAPIError);
    await expect(getSchemas({baseUrl: '', url: ''})).rejects.toThrow('Invalid URL provided.');
  });

  it('should handle HTTP error responses', async (): Promise<void> => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      text: () => Promise.resolve('Server exploded'),
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';

    await expect(getSchemas({baseUrl})).rejects.toThrow(ThunderIDAPIError);
    await expect(getSchemas({baseUrl})).rejects.toThrow('Failed to fetch SCIM2 schemas: Server exploded');
  });

  it('should handle network or parsing errors', async (): Promise<void> => {
    global.fetch = vi.fn().mockRejectedValue(new Error('Network error'));

    const baseUrl = 'https://api.asgardeo.io/t/demo';

    await expect(getSchemas({baseUrl})).rejects.toThrow(ThunderIDAPIError);
    await expect(getSchemas({baseUrl})).rejects.toThrow('Network or parsing error: Network error');
  });

  it('should handle non-Error rejections gracefully', async (): Promise<void> => {
    global.fetch = vi.fn().mockRejectedValue('unexpected failure');

    const baseUrl = 'https://api.asgardeo.io/t/demo';

    await expect(getSchemas({baseUrl})).rejects.toThrow('Network or parsing error: Unknown error');
  });

  it('should prefer url over baseUrl when both are provided', async (): Promise<void> => {
    const mockSchemas: Schema[] = [{id: 'x', name: 'X'} as any];

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockSchemas),
      ok: true,
    });

    const url = 'https://api.asgardeo.io/t/demo/scim2/Schemas';
    const baseUrl = 'https://api.asgardeo.io/t/ignored';
    await getSchemas({baseUrl, url});

    expect(fetch).toHaveBeenCalledWith(url, expect.any(Object));
  });

  it('should include custom headers when provided', async (): Promise<void> => {
    const mockSchemas: Schema[] = [{id: 'y', name: 'Y'} as any];

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockSchemas),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const customHeaders: Record<string, string> = {
      Authorization: 'Bearer token',
      'X-Custom-Header': 'custom-value',
    };

    await getSchemas({baseUrl, headers: customHeaders});

    expect(fetch).toHaveBeenCalledWith(`${baseUrl}/scim2/Schemas`, {
      headers: {
        Accept: 'application/json',
        Authorization: 'Bearer token',
        'Content-Type': 'application/json',
        'X-Custom-Header': 'custom-value',
      },
      method: 'GET',
    });
  });
});
