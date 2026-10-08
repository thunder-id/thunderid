// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, act, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import UserTypeQueryKeys from '../../constants/userTypeQueryKeys';
import type {ApiUserType, CreateUserTypeRequest} from '../../types/user-types';
import useCreateUserType from '../useCreateUserType';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({useThunderID: () => ({http: {request: mockHttpRequest}})}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: vi.fn()};
});

const {useConfig} = await import('@thunderid/contexts');

describe('useCreateUserType', () => {
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

  const mockRequest: CreateUserTypeRequest = {
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
    const {result} = renderHook(() => useCreateUserType());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(typeof result.current.mutate).toBe('function');
    expect(typeof result.current.mutateAsync).toBe('function');
  });

  it('should successfully create a user type', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserType,
    });

    const {result} = renderHook(() => useCreateUserType());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockUserType);
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/user-types',
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: mockRequest,
      }),
    );
  });

  it('should set pending state during creation', async () => {
    let resolveMutation!: (value: {data: ApiUserType}) => void;
    mockHttpRequest.mockReturnValue(
      new Promise<{data: ApiUserType}>((resolve) => {
        resolveMutation = resolve;
      }),
    );

    const {result} = renderHook(() => useCreateUserType());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    act(() => {
      resolveMutation({data: mockUserType});
    });

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });

    expect(result.current.isSuccess).toBe(true);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to create user type');

    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useCreateUserType());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('should invalidate user types query on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserType,
    });

    const {result, queryClient} = renderHook(() => useCreateUserType());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({
      queryKey: [UserTypeQueryKeys.USER_TYPES],
    });
  });

  it('should handle invalidateQueries rejection gracefully', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserType,
    });

    const {result, queryClient} = renderHook(() => useCreateUserType());
    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValueOnce(new Error('Invalidation failed'));

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockUserType);
  });

  it('should support mutateAsync for promise-based workflows', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserType,
    });

    const {result} = renderHook(() => useCreateUserType());

    const promise = result.current.mutateAsync(mockRequest);

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

    const {result} = renderHook(() => useCreateUserType());

    result.current.mutate(mockRequest);

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
