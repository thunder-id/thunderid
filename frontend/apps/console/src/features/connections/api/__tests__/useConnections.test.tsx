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

import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {renderHook, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import useConnections from '../useConnections';

vi.mock('@thunderid/react', () => ({useThunderID: vi.fn()}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: vi.fn()};
});

describe('useConnections', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockHttpRequest = vi.fn().mockResolvedValue({
      data: {
        connections: [
          {type: 'google', configured: true, instanceCount: 1},
          {type: 'github', configured: false, instanceCount: 0},
        ],
      },
    });
    vi.mocked(useThunderID).mockReturnValue({http: {request: mockHttpRequest}} as unknown as ReturnType<
      typeof useThunderID
    >);
    vi.mocked(useConfig).mockReturnValue({getServerUrl: () => 'https://localhost:8090'} as ReturnType<
      typeof useConfig
    >);
  });

  afterEach(() => vi.clearAllMocks());

  it('unwraps the connections array from the response envelope', async () => {
    const {result} = renderHook(() => useConnections());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toHaveLength(2);
    expect(result.current.data?.[0]).toEqual({type: 'google', configured: true, instanceCount: 1});
  });

  it('calls GET /connections', async () => {
    const {result} = renderHook(() => useConnections());
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://localhost:8090/connections', method: 'GET'}),
    );
  });

  it('surfaces errors', async () => {
    mockHttpRequest.mockRejectedValue(new Error('boom'));
    const {result} = renderHook(() => useConnections());

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe('boom');
  });
});
