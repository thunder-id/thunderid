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

import {renderHook, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import useConnections from '../useConnections';

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => 'https://localhost:8090'})};
});

describe('useConnections', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset().mockResolvedValue({
      data: {
        totalResults: 3,
        startIndex: 1,
        count: 3,
        connections: [
          {id: 'g-1', name: 'My Google', type: 'google', categories: ['identity-provider']},
          {
            id: 'o-1',
            name: 'Corp OIDC',
            description: 'Corporate login',
            type: 'oidc',
            categories: ['identity-provider'],
          },
          {id: 's-1', name: 'Twilio SMS', type: 'twilio', categories: ['sms-provider']},
        ],
        links: [],
      },
    });
  });

  afterEach(() => vi.clearAllMocks());

  it('returns the paginated envelope', async () => {
    const {result} = renderHook(() => useConnections());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.totalResults).toBe(3);
    expect(result.current.data?.startIndex).toBe(1);
    expect(result.current.data?.count).toBe(3);
    expect(result.current.data?.connections).toHaveLength(3);
    expect(result.current.data?.connections[0]).toEqual({
      id: 'g-1',
      name: 'My Google',
      type: 'google',
      categories: ['identity-provider'],
    });
  });

  it('calls GET /connections without query params when none are provided', async () => {
    const {result} = renderHook(() => useConnections());
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://localhost:8090/connections', method: 'GET'}),
    );
  });

  it('appends category, limit and offset query params when provided', async () => {
    const {result} = renderHook(() => useConnections({category: 'identity-provider', limit: 10, offset: 20}));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/connections?category=identity-provider&limit=10&offset=20',
        method: 'GET',
      }),
    );
  });

  it('does not fetch when disabled', async () => {
    renderHook(() => useConnections(undefined, {enabled: false}));

    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('surfaces errors', async () => {
    mockHttpRequest.mockRejectedValue(new Error('boom'));
    const {result} = renderHook(() => useConnections());

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe('boom');
  });
});
