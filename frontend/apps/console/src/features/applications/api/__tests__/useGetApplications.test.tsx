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
import ApplicationQueryKeys from '../../constants/application-query-keys';
import type {ApplicationListResponse} from '../../models/responses';
import useGetApplications from '../useGetApplications';

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

describe('useGetApplications', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;

  const mockApplicationListResponse: ApplicationListResponse = {
    totalResults: 2,
    count: 2,
    applications: [
      {
        id: '550e8400-e29b-41d4-a716-446655440000',
        name: 'Test App 1',
        description: 'First test application',
        logoUrl: 'https://test-app-1.com/logo.png',
        authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
        registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
        isRegistrationFlowEnabled: true,
      },
      {
        id: '660e8400-e29b-41d4-a716-446655440001',
        name: 'Test App 2',
        description: 'Second test application',
        logoUrl: 'https://test-app-2.com/logo.png',
        authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
        registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
        isRegistrationFlowEnabled: false,
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
    mockHttpRequest.mockReturnValue(new Promise(() => null)); // Never resolves

    const {result} = renderHook(() => useGetApplications());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
  });

  it('should successfully fetch applications list', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplicationListResponse,
    });

    const {result} = renderHook(() => useGetApplications());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockApplicationListResponse);
    expect(result.current.data?.applications).toHaveLength(2);
    expect(result.current.data?.totalResults).toBe(2);
    expect(result.current.data?.count).toBe(2);
  });

  it('should use default pagination parameters (limit=30, offset=0)', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplicationListResponse,
    });

    renderHook(() => useGetApplications());

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
      data: mockApplicationListResponse,
    });

    renderHook(() => useGetApplications({limit: 10, offset: 5}));

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
    const apiError = new Error('Failed to fetch applications');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetApplications());

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should handle network error', async () => {
    const networkError = new Error('Network request failed');
    mockHttpRequest.mockRejectedValueOnce(networkError);

    const {result} = renderHook(() => useGetApplications());

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(networkError);
  });

  it('should use correct server URL from config', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplicationListResponse,
    });

    renderHook(() => useGetApplications());

    await waitFor(() => {
      expect(mockGetServerUrl).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('https://api.test.com/applications');
  });

  it('should include correct headers', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplicationListResponse,
    });

    renderHook(() => useGetApplications());

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

  it('should use correct query key with pagination params', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplicationListResponse,
    });

    const {result, queryClient} = renderHook(() => useGetApplications({limit: 20, offset: 10}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const cache = queryClient.getQueryCache();
    const queries = cache.findAll({
      queryKey: [ApplicationQueryKeys.APPLICATIONS, {limit: 20, offset: 10}],
    });

    expect(queries).toHaveLength(1);
  });

  it('should cache results for same parameters', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplicationListResponse,
    });

    // First call - get the queryClient from the render result
    const {result: result1, queryClient} = renderHook(() => useGetApplications({limit: 10, offset: 0}));

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    // Set the data as fresh to prevent refetch
    queryClient.setQueryDefaults([ApplicationQueryKeys.APPLICATIONS, {limit: 10, offset: 0}], {
      staleTime: Infinity,
    });

    // Second call with same queryClient should use cache
    const {result: result2} = renderHook(() => useGetApplications({limit: 10, offset: 0}), {
      queryClient,
    });

    await waitFor(() => {
      expect(result2.current.data).toEqual(mockApplicationListResponse);
    });
    expect(mockHttpRequest).toHaveBeenCalledTimes(1); // Should not make another request
  });

  it('should make new request for different parameters', async () => {
    mockHttpRequest.mockResolvedValue({
      data: mockApplicationListResponse,
    });

    const {result: result1} = renderHook(() => useGetApplications({limit: 10, offset: 0}));

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    // Second call with different parameters should make new request
    const {result: result2} = renderHook(() => useGetApplications({limit: 20, offset: 5}));

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should support refetching data', async () => {
    mockHttpRequest.mockResolvedValue({
      data: mockApplicationListResponse,
    });

    const {result} = renderHook(() => useGetApplications());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    // Refetch the data
    await result.current.refetch();

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should handle empty applications list', async () => {
    const emptyResponse: ApplicationListResponse = {
      totalResults: 0,
      count: 0,
      applications: [],
    };

    mockHttpRequest.mockResolvedValueOnce({
      data: emptyResponse,
    });

    const {result} = renderHook(() => useGetApplications());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(emptyResponse);
    expect(result.current.data?.applications).toHaveLength(0);
    expect(result.current.data?.totalResults).toBe(0);
  });

  it('should handle partial pagination correctly', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: {
        totalResults: 25,
        count: 10,
        applications: mockApplicationListResponse.applications,
      },
    });

    const {result} = renderHook(() => useGetApplications({limit: 10, offset: 0}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.totalResults).toBe(25);
    expect(result.current.data?.count).toBe(10);
    expect(result.current.data?.applications).toHaveLength(2);
  });

  it('should properly construct query string with URL encoding', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplicationListResponse,
    });

    renderHook(() => useGetApplications({limit: 15, offset: 30}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    const url = callArgs.url as string;
    expect(url).toContain('applications?');
    expect(url).toContain('limit=15');
    expect(url).toContain('offset=30');
  });

  it('should maintain correct loading state during fetch', async () => {
    let resolveRequest: (value: {data: ApplicationListResponse}) => void;
    const requestPromise = new Promise<{data: ApplicationListResponse}>((resolve) => {
      resolveRequest = resolve;
    });

    mockHttpRequest.mockReturnValueOnce(requestPromise);

    const {result} = renderHook(() => useGetApplications());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.isFetching).toBe(true);
    expect(result.current.data).toBeUndefined();

    resolveRequest!({data: mockApplicationListResponse});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toEqual(mockApplicationListResponse);
  });
});
