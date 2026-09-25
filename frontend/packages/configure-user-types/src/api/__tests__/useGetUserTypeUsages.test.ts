// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import UserTypeQueryKeys from '../../constants/userTypeQueryKeys';
import type {UserTypeUsagesResponse} from '../../types/user-types';
import useGetUserTypeUsages from '../useGetUserTypeUsages';

const mockHttpRequest = vi.fn();
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

describe('useGetUserTypeUsages', () => {
  let mockGetServerUrl: ReturnType<typeof vi.fn>;

  const mockUsages: UserTypeUsagesResponse = {
    totalResults: 2,
    count: 2,
    summary: {user: 2},
    usages: [
      {resourceType: 'user', id: '', displayName: '', behaviorOnDelete: 'restrict'},
      {resourceType: 'user', id: '', displayName: '', behaviorOnDelete: 'restrict'},
    ],
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

  it('should not fetch when no userTypeId is provided', () => {
    const {result} = renderHook(() => useGetUserTypeUsages(null));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should not fetch when enabled is false', () => {
    const {result} = renderHook(() => useGetUserTypeUsages('schema-123', false));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should successfully fetch usages for a user type', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockUsages});

    const {result} = renderHook(() => useGetUserTypeUsages('schema-123'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockUsages);
  });

  it('should use correct server URL and endpoint', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockUsages});

    renderHook(() => useGetUserTypeUsages('schema-123'));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/user-types/schema-123/usages',
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );
  });

  it('should use correct query key', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockUsages});

    const {result, queryClient} = renderHook(() => useGetUserTypeUsages('schema-123'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const queryKey = [UserTypeQueryKeys.USER_TYPE_USAGES, 'schema-123'];
    const cachedData = queryClient.getQueryData(queryKey);
    expect(cachedData).toEqual(mockUsages);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to fetch usages');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetUserTypeUsages('schema-123'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });
});
