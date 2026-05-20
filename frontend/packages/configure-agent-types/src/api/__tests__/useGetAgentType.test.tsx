/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {ApiAgentType} from '../../models/agent-type';
import useGetAgentType from '../useGetAgentType';

const mockHttpRequest = vi.fn();
const mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {request: mockHttpRequest},
  }),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: mockGetServerUrl}),
  };
});

describe('useGetAgentType', () => {
  const mockAgentType: ApiAgentType = {
    id: 'aaa-bbb-ccc',
    name: 'default',
    ouId: '111e8400-e29b-41d4-a716-446655440000',
    schema: {
      model: {type: 'string', required: false},
      department: {type: 'string', required: false},
      purpose: {type: 'string', required: false},
    },
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should not fetch when id is empty', () => {
    mockHttpRequest.mockReturnValue(new Promise(() => null));

    const {result} = renderHook(() => useGetAgentType(undefined));

    expect(result.current.isLoading).toBe(false);
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should fetch agent type by ID', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgentType});

    const {result} = renderHook(() => useGetAgentType(mockAgentType.id));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockAgentType);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/agent-types/${mockAgentType.id}?include=display`,
        method: 'GET',
      }),
    );
  });

  it('should handle API error', async () => {
    const apiError = new Error('Agent type not found');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetAgentType('aaa-bbb-ccc'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
  });
});
