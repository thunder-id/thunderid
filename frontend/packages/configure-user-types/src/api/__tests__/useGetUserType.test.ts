// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import UserTypeQueryKeys from '../../constants/userTypeQueryKeys';
import type {ApiUserType} from '../../types/user-types';
import useGetUserType from '../useGetUserType';

const mockHttpRequest = vi.fn();
// Mock the dependencies
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
  };
});

const {useConfig} = await import('@thunderid/contexts');

describe('useGetUserType', () => {
  let mockGetServerUrl: ReturnType<typeof vi.fn>;

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

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: mockGetServerUrl,
    } as unknown as ReturnType<typeof useConfig>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with loading state when id is provided', () => {
    mockHttpRequest.mockReturnValue(new Promise(() => null)); // Never resolves

    const {result} = renderHook(() => useGetUserType('123'));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
  });

  it('should not fetch when no id is provided', () => {
    const {result} = renderHook(() => useGetUserType());

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should not fetch when id is an empty string', () => {
    const {result} = renderHook(() => useGetUserType(''));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should successfully fetch a single user type', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserType,
    });

    const {result} = renderHook(() => useGetUserType('123'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockUserType);
    expect(result.current.data?.id).toBe('123');
    expect(result.current.data?.displayName).toBe('Person');
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to fetch user type');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetUserType('123'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should use correct server URL and endpoint', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserType,
    });

    renderHook(() => useGetUserType('123'));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/user-types/123?include=display',
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );
  });

  it('should use correct query key', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserType,
    });

    const {result, queryClient} = renderHook(() => useGetUserType('123'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const queryKey = [UserTypeQueryKeys.USER_TYPE, '123'];
    const cachedData = queryClient.getQueryData(queryKey);
    expect(cachedData).toEqual(mockUserType);
  });

  it('should support refetching', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockUserType});

    const {result} = renderHook(() => useGetUserType('123'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.displayName).toBe('Person');

    const updatedUserType: ApiUserType = {
      ...mockUserType,
      displayName: 'Updated Person',
    };

    mockHttpRequest.mockResolvedValueOnce({data: updatedUserType});

    await result.current.refetch();

    await waitFor(() => {
      expect(result.current.data?.displayName).toBe('Updated Person');
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });
});
