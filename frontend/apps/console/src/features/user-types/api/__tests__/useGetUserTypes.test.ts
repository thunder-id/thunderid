/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
import UserTypeQueryKeys from '../../constants/userTypeQueryKeys';
import type {UserTypeListResponse} from '../../types/user-types';
import useGetUserTypes from '../useGetUserTypes';

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

describe('useGetUserTypes', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;

  const mockUserTypeListResponse: UserTypeListResponse = {
    totalResults: 2,
    startIndex: 1,
    count: 2,
    types: [
      {id: '123', name: 'UserType1', ouId: 'root-ou', allowSelfRegistration: false},
      {id: '456', name: 'UserType2', ouId: 'child-ou', allowSelfRegistration: true},
    ],
    links: [{rel: 'self', href: 'https://api.test.com/user-types'}],
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
    mockHttpRequest.mockReturnValue(new Promise(() => null)); // Never resolves

    const {result} = renderHook(() => useGetUserTypes());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
  });

  it('should successfully fetch user types list', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserTypeListResponse,
    });

    const {result} = renderHook(() => useGetUserTypes());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockUserTypeListResponse);
    expect(result.current.data?.types).toHaveLength(2);
    expect(result.current.data?.totalResults).toBe(2);
    expect(result.current.data?.count).toBe(2);
  });

  it('should build URL without query params when none provided', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserTypeListResponse,
    });

    renderHook(() => useGetUserTypes());

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toBe('https://api.test.com/user-types?include=display');
  });

  it('should build URL with limit parameter only', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserTypeListResponse,
    });

    renderHook(() => useGetUserTypes({limit: 10}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toBe('https://api.test.com/user-types?limit=10&include=display');
  });

  it('should build URL with offset parameter only', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserTypeListResponse,
    });

    renderHook(() => useGetUserTypes({offset: 5}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toBe('https://api.test.com/user-types?offset=5&include=display');
  });

  it('should build URL with both limit and offset parameters', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserTypeListResponse,
    });

    renderHook(() => useGetUserTypes({limit: 10, offset: 5}));

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
    const apiError = new Error('Failed to fetch user types');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetUserTypes());

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should use correct server URL from config', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserTypeListResponse,
    });

    renderHook(() => useGetUserTypes());

    await waitFor(() => {
      expect(mockGetServerUrl).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('https://api.test.com/user-types');
  });

  it('should include correct headers', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserTypeListResponse,
    });

    renderHook(() => useGetUserTypes());

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.method).toBe('GET');
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.headers['Content-Type']).toBe('application/json');
  });

  it('should use correct query key', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserTypeListResponse,
    });

    const {result, queryClient} = renderHook(() => useGetUserTypes({limit: 20, offset: 10}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const cache = queryClient.getQueryCache();
    const queries = cache.findAll({
      queryKey: [UserTypeQueryKeys.USER_TYPES, {limit: 20, offset: 10}],
    });

    expect(queries).toHaveLength(1);
  });

  it('should cache results for same parameters', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserTypeListResponse,
    });

    // First call - get the queryClient from the render result
    const {result: result1, queryClient} = renderHook(() => useGetUserTypes({limit: 10, offset: 0}));

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    // Set the data as fresh to prevent refetch
    queryClient.setQueryDefaults([UserTypeQueryKeys.USER_TYPES, {limit: 10, offset: 0}], {
      staleTime: Infinity,
    });

    // Second call with same queryClient should use cache
    const {result: result2} = renderHook(() => useGetUserTypes({limit: 10, offset: 0}), {
      queryClient,
    });

    await waitFor(() => {
      expect(result2.current.data).toEqual(mockUserTypeListResponse);
    });
    expect(mockHttpRequest).toHaveBeenCalledTimes(1); // Should not make another request
  });

  it('should support refetching data', async () => {
    mockHttpRequest.mockResolvedValue({
      data: mockUserTypeListResponse,
    });

    const {result} = renderHook(() => useGetUserTypes());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    // Refetch the data
    await result.current.refetch();

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should handle empty list', async () => {
    const emptyResponse: UserTypeListResponse = {
      totalResults: 0,
      startIndex: 0,
      count: 0,
      types: [],
    };

    mockHttpRequest.mockResolvedValueOnce({
      data: emptyResponse,
    });

    const {result} = renderHook(() => useGetUserTypes());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(emptyResponse);
    expect(result.current.data?.types).toHaveLength(0);
    expect(result.current.data?.totalResults).toBe(0);
  });
});
