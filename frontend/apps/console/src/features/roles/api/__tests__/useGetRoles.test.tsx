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
import type {RoleListResponse} from '../../models/role';
import useGetRoles from '../useGetRoles';

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

describe('useGetRoles', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;

  const mockRoleListResponse: RoleListResponse = {
    totalResults: 2,
    startIndex: 0,
    count: 2,
    roles: [
      {
        id: 'role-1',
        name: 'Admin Role',
        description: 'Administrator role',
        ouId: 'ou-1',
      },
      {
        id: 'role-2',
        name: 'Viewer Role',
        description: 'Read-only role',
        ouId: 'ou-2',
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

  it('should initialize with loading state', () => {
    mockHttpRequest.mockReturnValue(
      new Promise(() => {
        /* noop */
      }),
    );

    const {result} = renderHook(() => useGetRoles());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
  });

  it('should successfully fetch roles list', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockRoleListResponse,
    });

    const {result} = renderHook(() => useGetRoles());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockRoleListResponse);
    expect(result.current.data?.roles).toHaveLength(2);
    expect(result.current.data?.totalResults).toBe(2);
    expect(result.current.data?.count).toBe(2);
  });

  it('should use default pagination parameters (limit=30, offset=0)', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockRoleListResponse,
    });

    renderHook(() => useGetRoles());

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('limit=30');
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('offset=0');
  });

  it('should use custom pagination parameters', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockRoleListResponse,
    });

    renderHook(() => useGetRoles({limit: 10, offset: 5}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('limit=10');
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('offset=5');
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to fetch roles');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetRoles());

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should use correct server URL from config', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockRoleListResponse,
    });

    renderHook(() => useGetRoles());

    await waitFor(() => {
      expect(mockGetServerUrl).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('https://api.test.com/roles');
  });

  it('should make GET request', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockRoleListResponse,
    });

    renderHook(() => useGetRoles());

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.method).toBe('GET');
  });

  it('should use correct query key with pagination params', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockRoleListResponse,
    });

    const {result, queryClient} = renderHook(() => useGetRoles({limit: 20, offset: 10}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const cache = queryClient.getQueryCache();
    const queries = cache.findAll({
      queryKey: [RoleQueryKeys.ROLES, {limit: 20, offset: 10}],
    });

    expect(queries).toHaveLength(1);
  });

  it('should cache results for same parameters', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockRoleListResponse,
    });

    const {result: result1, queryClient} = renderHook(() => useGetRoles({limit: 10, offset: 0}));

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    queryClient.setQueryDefaults([RoleQueryKeys.ROLES, {limit: 10, offset: 0}], {
      staleTime: Infinity,
    });

    const {result: result2} = renderHook(() => useGetRoles({limit: 10, offset: 0}), {
      queryClient,
    });

    await waitFor(() => {
      expect(result2.current.data).toEqual(mockRoleListResponse);
    });
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
  });

  it('should make new request for different parameters', async () => {
    mockHttpRequest.mockResolvedValue({
      data: mockRoleListResponse,
    });

    const {result: result1} = renderHook(() => useGetRoles({limit: 10, offset: 0}));

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    const {result: result2} = renderHook(() => useGetRoles({limit: 20, offset: 5}));

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should handle empty roles list', async () => {
    const emptyResponse: RoleListResponse = {
      totalResults: 0,
      startIndex: 0,
      count: 0,
      roles: [],
    };

    mockHttpRequest.mockResolvedValueOnce({
      data: emptyResponse,
    });

    const {result} = renderHook(() => useGetRoles());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(emptyResponse);
    expect(result.current.data?.roles).toHaveLength(0);
    expect(result.current.data?.totalResults).toBe(0);
  });

  it('should properly construct query string', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockRoleListResponse,
    });

    renderHook(() => useGetRoles({limit: 15, offset: 30}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    const url = callArgs.url as string;
    expect(url).toContain('roles?');
    expect(url).toContain('limit=15');
    expect(url).toContain('offset=30');
  });

  it('should maintain correct loading state during fetch', async () => {
    let resolveRequest: (value: {data: RoleListResponse}) => void;
    const requestPromise = new Promise<{data: RoleListResponse}>((resolve) => {
      resolveRequest = resolve;
    });

    mockHttpRequest.mockReturnValueOnce(requestPromise);

    const {result} = renderHook(() => useGetRoles());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.isFetching).toBe(true);
    expect(result.current.data).toBeUndefined();

    resolveRequest!({data: mockRoleListResponse});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toEqual(mockRoleListResponse);
  });

  it('should support refetching data', async () => {
    mockHttpRequest.mockResolvedValue({
      data: mockRoleListResponse,
    });

    const {result} = renderHook(() => useGetRoles());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    await result.current.refetch();

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });
});
