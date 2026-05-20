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
import RoleQueryKeys from '../../constants/role-query-keys';
import type {Role} from '../../models/role';
import useGetRole from '../useGetRole';

// Mock the dependencies
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

describe('useGetRole', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;

  const mockRole: Role = {
    id: 'role-1',
    name: 'Admin Role',
    description: 'Administrator role with full permissions',
    ouId: 'ou-1',
    permissions: [
      {
        resourceServerId: 'rs-1',
        permissions: ['read', 'write'],
      },
    ],
  };

  beforeEach(() => {
    mockHttpRequest = vi.fn();
    mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');

    vi.mocked(useThunderID).mockReturnValue({
      http: {
        request: mockHttpRequest,
      },
    } as unknown as ReturnType<typeof useThunderID>);

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: mockGetServerUrl,
    } as unknown as ReturnType<typeof useConfig>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with loading state when roleId is provided', () => {
    mockHttpRequest.mockReturnValue(
      new Promise(() => {
        /* noop */
      }),
    );

    const {result} = renderHook(() => useGetRole('role-1'));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
  });

  it('should successfully fetch a single role', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockRole,
    });

    const {result} = renderHook(() => useGetRole('role-1'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockRole);
    expect(result.current.data?.id).toBe('role-1');
    expect(result.current.data?.name).toBe('Admin Role');
    expect(result.current.data?.permissions).toHaveLength(1);
  });

  it('should make correct API call with role ID', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockRole,
    });

    renderHook(() => useGetRole('role-1'));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toBe('https://api.test.com/roles/role-1?include=display');
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.method).toBe('GET');
  });

  it('should use correct query key', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockRole,
    });

    const {result, queryClient} = renderHook(() => useGetRole('role-1'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const cache = queryClient.getQueryCache();
    const queries = cache.findAll({
      queryKey: [RoleQueryKeys.ROLE, 'role-1'],
    });

    expect(queries).toHaveLength(1);
  });

  it('should not fetch when roleId is empty string', () => {
    const {result} = renderHook(() => useGetRole(''));

    expect(result.current.isLoading).toBe(false);
    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to fetch role');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetRole('role-1'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should handle 404 Not Found error', async () => {
    const notFoundError = new Error('Role not found');
    mockHttpRequest.mockRejectedValueOnce(notFoundError);

    const {result} = renderHook(() => useGetRole('non-existent-id'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(notFoundError);
  });

  it('should support refetching data', async () => {
    mockHttpRequest.mockResolvedValue({
      data: mockRole,
    });

    const {result} = renderHook(() => useGetRole('role-1'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    await result.current.refetch();

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should use correct server URL from config', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockRole,
    });

    renderHook(() => useGetRole('role-1'));

    await waitFor(() => {
      expect(mockGetServerUrl).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('https://api.test.com');
  });
});
