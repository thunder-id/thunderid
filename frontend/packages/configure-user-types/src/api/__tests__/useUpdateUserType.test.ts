// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, act, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import UserTypeQueryKeys from '../../constants/userTypeQueryKeys';
import type {ApiUserType, UpdateUserTypeRequest} from '../../types/user-types';
import useUpdateUserType from '../useUpdateUserType';
import type {UpdateUserTypeVariables} from '../useUpdateUserType';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({useThunderID: () => ({http: {request: mockHttpRequest}})}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: vi.fn()};
});

const {useConfig} = await import('@thunderid/contexts');

describe('useUpdateUserType', () => {
  const mockUserTypeId = '123';

  const mockUserType: ApiUserType = {
    id: '123',
    handle: 'person',
    displayName: 'Person',
    ouId: 'ou-1',
    allowSelfRegistration: true,
    schema: {
      email: {
        type: 'string',
        required: true,
      },
    },
  };

  const mockUpdateRequest: UpdateUserTypeRequest = {
    displayName: 'Person',
    ouId: 'ou-1',
    allowSelfRegistration: true,
    schema: {
      email: {
        type: 'string',
        required: true,
      },
    },
  };

  const mockVariables: UpdateUserTypeVariables = {
    userTypeId: mockUserTypeId,
    data: mockUpdateRequest,
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => 'https://api.test.com',
    } as ReturnType<typeof useConfig>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useUpdateUserType());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(typeof result.current.mutate).toBe('function');
    expect(typeof result.current.mutateAsync).toBe('function');
  });

  it('should successfully update a user type', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserType,
    });

    const {result} = renderHook(() => useUpdateUserType());

    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockUserType);
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/user-types/${mockUserTypeId}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data: mockUpdateRequest,
      }),
    );
  });

  it('should set pending state during update', async () => {
    mockHttpRequest.mockImplementation(
      () =>
        new Promise((resolve) => {
          setTimeout(() => {
            resolve({
              data: mockUserType,
            });
          }, 100);
        }),
    );

    const {result} = renderHook(() => useUpdateUserType());

    act(() => {
      result.current.mutate(mockVariables);
    });

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
    const apiError = new Error('Failed to update user type');

    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useUpdateUserType());

    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('should invalidate correct queries on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserType,
    });

    const {result, queryClient} = renderHook(() => useUpdateUserType());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({
      queryKey: [UserTypeQueryKeys.USER_TYPE, mockUserTypeId],
    });
    expect(invalidateQueriesSpy).toHaveBeenCalledWith({
      queryKey: [UserTypeQueryKeys.USER_TYPES],
    });
  });

  it('should handle invalidateQueries rejection gracefully', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserType,
    });

    const {result, queryClient} = renderHook(() => useUpdateUserType());
    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValue(new Error('Invalidation failed'));

    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockUserType);
  });

  it('should support mutateAsync for promise-based workflows', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserType,
    });

    const {result} = renderHook(() => useUpdateUserType());

    const promise = result.current.mutateAsync(mockVariables);

    await expect(promise).resolves.toEqual(mockUserType);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toEqual(mockUserType);
  });

  it('should reset mutation state', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserType,
    });

    const {result} = renderHook(() => useUpdateUserType());

    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    act(() => {
      result.current.reset();
    });

    await waitFor(() => {
      expect(result.current.data).toBeUndefined();
    });
    expect(result.current.error).toBeNull();
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
  });
});
