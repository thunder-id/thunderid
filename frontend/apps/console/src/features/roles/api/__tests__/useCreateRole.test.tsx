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
import type {CreateRoleRequest} from '../../models/requests';
import type {Role} from '../../models/role';
import useCreateRole from '../useCreateRole';

// Mock the dependencies
vi.mock('@thunderid/react', () => ({
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
    useToast: vi.fn(),
  };
});

const {useThunderID} = await import('@thunderid/react');
const {useConfig, useToast} = await import('@thunderid/contexts');

describe('useCreateRole', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;
  let mockShowToast: ReturnType<typeof vi.fn>;

  const mockCreatedRole: Role = {
    id: 'new-role-1',
    name: 'New Role',
    description: 'A newly created role',
    ouId: 'ou-1',
    permissions: [],
  };

  const mockCreateRequest: CreateRoleRequest = {
    name: 'New Role',
    description: 'A newly created role',
    ouId: 'ou-1',
  };

  beforeEach(() => {
    mockHttpRequest = vi.fn();
    mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');
    mockShowToast = vi.fn();

    vi.mocked(useThunderID).mockReturnValue({
      http: {
        request: mockHttpRequest,
      },
    } as unknown as ReturnType<typeof useThunderID>);

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: mockGetServerUrl,
    } as unknown as ReturnType<typeof useConfig>);

    vi.mocked(useToast).mockReturnValue({
      showToast: mockShowToast,
    } as unknown as ReturnType<typeof useToast>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useCreateRole());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(typeof result.current.mutate).toBe('function');
    expect(typeof result.current.mutateAsync).toBe('function');
  });

  it('should successfully create a role', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockCreatedRole,
    });

    const {result} = renderHook(() => useCreateRole());

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockCreatedRole);
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
  });

  it('should make correct API call', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockCreatedRole,
    });

    const {result} = renderHook(() => useCreateRole());

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/roles',
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        data: mockCreateRequest,
      }),
    );
  });

  it('should set pending state during creation', async () => {
    mockHttpRequest.mockReturnValue(
      new Promise((resolve) => {
        setTimeout(() => resolve({data: mockCreatedRole}), 100);
      }),
    );

    const {result} = renderHook(() => useCreateRole());

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    await waitFor(
      () => {
        expect(result.current.isSuccess).toBe(true);
      },
      {timeout: 200},
    );

    expect(result.current.isPending).toBe(false);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to create role');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useCreateRole());

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('should invalidate ROLES cache on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockCreatedRole,
    });

    const {result, queryClient} = renderHook(() => useCreateRole());

    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [RoleQueryKeys.ROLES],
      }),
    );
  });

  it('should show success toast on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockCreatedRole,
    });

    const {result} = renderHook(() => useCreateRole());

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('should show error toast on error', async () => {
    mockHttpRequest.mockRejectedValueOnce(new Error('Failed'));

    const {result} = renderHook(() => useCreateRole());

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'error');
  });

  it('should handle invalidateQueries rejection gracefully', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockCreatedRole,
    });

    const {result, queryClient} = renderHook(() => useCreateRole());

    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValueOnce(new Error('Invalidation failed'));

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('should use mutateAsync for promise-based creation', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockCreatedRole,
    });

    const {result} = renderHook(() => useCreateRole());

    const createPromise = result.current.mutateAsync(mockCreateRequest);

    await expect(createPromise).resolves.toEqual(mockCreatedRole);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });
});
