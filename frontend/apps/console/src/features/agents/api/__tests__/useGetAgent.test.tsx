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
import type {Agent} from '../../models/agent';
import useGetAgent from '../useGetAgent';

vi.mock('@thunderid/react', () => ({
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
  };
});

const {useThunderID} = await import('@thunderid/react');
const {useConfig} = await import('@thunderid/contexts');

describe('useGetAgent', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;

  const mockAgent: Agent = {
    id: '550e8400-e29b-41d4-a716-446655440000',
    ouId: '111e8400-e29b-41d4-a716-446655440000',
    type: 'default',
    name: 'Billing Service',
    description: 'Service-to-service billing agent',
  };

  beforeEach(() => {
    mockHttpRequest = vi.fn();

    vi.mocked(useThunderID).mockReturnValue({
      http: {request: mockHttpRequest},
    } as unknown as ReturnType<typeof useThunderID>);

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => 'https://api.test.com',
    } as ReturnType<typeof useConfig>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should not fetch when agentId is empty', () => {
    mockHttpRequest.mockReturnValue(new Promise(() => null));

    const {result} = renderHook(() => useGetAgent(''));

    expect(result.current.isLoading).toBe(false);
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should fetch agent by ID', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgent});

    const {result} = renderHook(() => useGetAgent(mockAgent.id));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockAgent);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/agents/${mockAgent.id}?include=display`,
        method: 'GET',
      }),
    );
  });

  it('should handle API error', async () => {
    const apiError = new Error('Agent not found');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetAgent(mockAgent.id));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
  });
});
