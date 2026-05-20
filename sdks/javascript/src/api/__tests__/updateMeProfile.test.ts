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

import {Mock, beforeEach, describe, expect, it, vi} from 'vitest';
import ThunderIDAPIError from '../../errors/ThunderIDAPIError';
import type {User} from '../../models/user';
import updateMeProfile from '../updateMeProfile';

describe('updateMeProfile', (): void => {
  beforeEach((): void => {
    vi.resetAllMocks();
  });

  it('should update profile successfully using default fetch', async (): Promise<void> => {
    const mockUser: User = {
      email: 'alice@example.com',
      id: 'u1',
      name: 'Alice',
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockUser),
      ok: true,
    });

    const url = 'https://api.asgardeo.io/t/demo/scim2/Me';
    const payload: Record<string, unknown> = {'urn:scim:wso2:schema': {mobileNumbers: ['0777933830']}};

    const result: User = await updateMeProfile({payload, url});

    expect(fetch).toHaveBeenCalledTimes(1);
    const [calledUrl, init]: [string, RequestInit] = (fetch as unknown as Mock).mock.calls[0];

    expect(calledUrl).toBe(url);
    expect(init.method).toBe('PATCH');
    expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/scim+json');
    expect((init.headers as Record<string, string>)['Accept']).toBe('application/json');

    const parsed: Record<string, unknown> = JSON.parse(init.body as string);
    expect(parsed.schemas).toEqual(['urn:ietf:params:scim:api:messages:2.0:PatchOp']);
    expect(parsed.Operations).toEqual([{op: 'replace', value: payload}]);

    expect(result).toEqual(mockUser);
  });

  it('should fall back to baseUrl when url is not provided', async (): Promise<void> => {
    const mockUser: User = {
      email: 'bob@example.com',
      id: 'u2',
      name: 'Bob',
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockUser),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const payload: Record<string, unknown> = {profile: {givenName: 'Bob'}};

    const result: User = await updateMeProfile({baseUrl, payload});

    expect(fetch).toHaveBeenCalledWith(`${baseUrl}/scim2/Me`, expect.any(Object));
    expect(result).toEqual(mockUser);
  });

  it('should use custom fetcher when provided', async (): Promise<void> => {
    const mockUser: User = {email: 'carol@example.com', id: 'u3', name: 'Carol'};

    const customFetcher: Mock = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockUser),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const payload: Record<string, unknown> = {profile: {familyName: 'Doe'}};

    const result: User = await updateMeProfile({baseUrl, fetcher: customFetcher, payload});

    expect(result).toEqual(mockUser);
    expect(customFetcher).toHaveBeenCalledWith(
      `${baseUrl}/scim2/Me`,
      expect.objectContaining({
        headers: expect.objectContaining({
          Accept: 'application/json',
          'Content-Type': 'application/scim+json',
        }),
        method: 'PATCH',
      }),
    );
  });

  it('should prefer url over baseUrl when both are provided', async (): Promise<void> => {
    const mockUser: User = {email: 'dan@example.com', id: 'u4', name: 'Dan'};
    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockUser),
      ok: true,
    });

    const url = 'https://api.asgardeo.io/t/demo/scim2/Me';
    const baseUrl = 'https://api.asgardeo.io/t/ignored';
    await updateMeProfile({baseUrl, payload: {x: 1}, url});

    expect(fetch).toHaveBeenCalledWith(url, expect.any(Object));
  });

  it('should throw ThunderIDAPIError for invalid URL or baseUrl', async (): Promise<void> => {
    await expect(updateMeProfile({payload: {}, url: 'not-a-valid-url' as any})).rejects.toThrow(ThunderIDAPIError);

    await expect(updateMeProfile({payload: {}, url: 'not-a-valid-url' as any})).rejects.toThrow(
      'Invalid URL provided.',
    );
  });

  it('should throw ThunderIDAPIError when both url and baseUrl are missing', async (): Promise<void> => {
    await expect(updateMeProfile({baseUrl: undefined as any, payload: {}, url: undefined as any})).rejects.toThrow(
      ThunderIDAPIError,
    );
  });

  it('should throw ThunderIDAPIError when both url and baseUrl are empty strings', async (): Promise<void> => {
    await expect(updateMeProfile({baseUrl: '', payload: {}, url: ''})).rejects.toThrow(ThunderIDAPIError);
  });

  it('should handle HTTP error responses', async (): Promise<void> => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      text: () => Promise.resolve('SCIM validation failed'),
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    await expect(updateMeProfile({baseUrl, payload: {bad: 'data'}})).rejects.toThrow(ThunderIDAPIError);

    await expect(updateMeProfile({baseUrl, payload: {bad: 'data'}})).rejects.toThrow(
      'Failed to update user profile: SCIM validation failed',
    );
  });

  it('should handle network or unknown errors with the generic message', async (): Promise<void> => {
    // Rejection with Error
    global.fetch = vi.fn().mockRejectedValue(new Error('Network error'));
    await expect(updateMeProfile({payload: {a: 1}, url: 'https://api.asgardeo.io/t/demo/scim2/Me'})).rejects.toThrow(
      ThunderIDAPIError,
    );
    await expect(updateMeProfile({payload: {a: 1}, url: 'https://api.asgardeo.io/t/demo/scim2/Me'})).rejects.toThrow(
      'An error occurred while updating the user profile. Please try again.',
    );

    // Rejection with non-Error
    global.fetch = vi.fn().mockRejectedValue('weird failure');
    await expect(updateMeProfile({payload: {a: 1}, url: 'https://api.asgardeo.io/t/demo/scim2/Me'})).rejects.toThrow(
      'An error occurred while updating the user profile. Please try again.',
    );
  });

  it('should pass through custom headers (and enforces content-type & accept)', async (): Promise<void> => {
    const mockUser: User = {email: 'eve@example.com', id: 'u5', name: 'Eve'};

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockUser),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const customHeaders: Record<string, string> = {
      Accept: 'text/plain',
      Authorization: 'Bearer token',
      'Content-Type': 'text/plain',
      'X-Custom-Header': 'custom-value',
    };

    await updateMeProfile({baseUrl, headers: customHeaders, payload: {y: 2}});

    const [, init]: [string, RequestInit] = (fetch as unknown as Mock).mock.calls[0];
    expect((init as Record<string, unknown>).headers).toMatchObject({
      Accept: 'application/json',
      Authorization: 'Bearer token',
      'Content-Type': 'application/scim+json',
      'X-Custom-Header': 'custom-value',
    });
  });

  it('should build the SCIM PatchOp body correctly', async (): Promise<void> => {
    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve({} as User),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const payload: Record<string, unknown> = {'urn:scim:wso2:schema': {mobileNumbers: ['123']}};

    await updateMeProfile({baseUrl, payload});

    const [, init]: [string, RequestInit] = (fetch as unknown as Mock).mock.calls[0];
    const body: Record<string, unknown> = JSON.parse((init as Record<string, unknown>).body as string);

    expect(body.schemas).toEqual(['urn:ietf:params:scim:api:messages:2.0:PatchOp']);
    expect(body.Operations).toHaveLength(1);
    expect((body.Operations as Record<string, unknown>[])[0]).toEqual({op: 'replace', value: payload});
  });

  it('should allow method override when provided in requestConfig', async (): Promise<void> => {
    // Note: due to `{ method: 'PATCH', ...requestConfig }` order, requestConfig.method overrides PATCH
    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve({} as User),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    await updateMeProfile({baseUrl, method: 'PUT' as any, payload: {z: 9}});

    const [, init]: [string, RequestInit] = (fetch as unknown as Mock).mock.calls[0];
    expect(init.method).toBe('PUT');
  });
});
