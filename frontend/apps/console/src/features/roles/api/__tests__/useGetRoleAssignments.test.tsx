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
import type {RoleAssignmentListResponse} from '../../models/role';
import useGetRoleAssignments from '../useGetRoleAssignments';

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

describe('useGetRoleAssignments', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;

  const mockAssignmentListResponse: RoleAssignmentListResponse = {
    totalResults: 2,
    startIndex: 0,
    count: 2,
    assignments: [
      {
        id: 'user-1',
        type: 'user',
        display: 'John Doe',
      },
      {
        id: 'group-1',
        type: 'group',
        display: 'Admins',
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

    const {result} = renderHook(() => useGetRoleAssignments({roleId: 'role-1'}));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
  });

  it('should successfully fetch role assignments', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockAssignmentListResponse,
    });

    const {result} = renderHook(() => useGetRoleAssignments({roleId: 'role-1'}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockAssignmentListResponse);
    expect(result.current.data?.assignments).toHaveLength(2);
    expect(result.current.data?.totalResults).toBe(2);
  });

  it('should use correct URL with roleId', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockAssignmentListResponse,
    });

    renderHook(() => useGetRoleAssignments({roleId: 'role-1'}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('/roles/role-1/assignments');
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.method).toBe('GET');
  });

  it('should use default pagination parameters', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockAssignmentListResponse,
    });

    renderHook(() => useGetRoleAssignments({roleId: 'role-1'}));

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
      data: mockAssignmentListResponse,
    });

    renderHook(() => useGetRoleAssignments({roleId: 'role-1', limit: 10, offset: 5}));

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

  it('should pass include=display query parameter when specified', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockAssignmentListResponse,
    });

    renderHook(() => useGetRoleAssignments({roleId: 'role-1', include: 'display'}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('include=display');
  });

  it('should pass type query parameter when specified', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockAssignmentListResponse,
    });

    renderHook(() => useGetRoleAssignments({roleId: 'role-1', type: 'user'}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('type=user');
  });

  it('should not include type parameter when not specified', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockAssignmentListResponse,
    });

    renderHook(() => useGetRoleAssignments({roleId: 'role-1'}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).not.toContain('type=');
  });

  it('should not include include parameter when not specified', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockAssignmentListResponse,
    });

    renderHook(() => useGetRoleAssignments({roleId: 'role-1'}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).not.toContain('include=');
  });

  it('should not fetch when roleId is empty', () => {
    const {result} = renderHook(() => useGetRoleAssignments({roleId: ''}));

    expect(result.current.isLoading).toBe(false);
    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should use correct query key', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockAssignmentListResponse,
    });

    const {result, queryClient} = renderHook(() =>
      useGetRoleAssignments({roleId: 'role-1', limit: 10, offset: 0, include: 'display'}),
    );

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const cache = queryClient.getQueryCache();
    const queries = cache.findAll({
      queryKey: [RoleQueryKeys.ROLE_ASSIGNMENTS, 'role-1', {limit: 10, offset: 0, include: 'display', type: undefined}],
    });

    expect(queries).toHaveLength(1);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to fetch assignments');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetRoleAssignments({roleId: 'role-1'}));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should handle empty assignments list', async () => {
    const emptyResponse: RoleAssignmentListResponse = {
      totalResults: 0,
      startIndex: 0,
      count: 0,
      assignments: [],
    };

    mockHttpRequest.mockResolvedValueOnce({
      data: emptyResponse,
    });

    const {result} = renderHook(() => useGetRoleAssignments({roleId: 'role-1'}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.assignments).toHaveLength(0);
    expect(result.current.data?.totalResults).toBe(0);
  });
});
