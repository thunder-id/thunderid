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
import UserQueryKeys from '../../constants/user-query-keys';
import type {ApiUserType} from '../../models/users';
import useGetUserType from '../useGetUserType';

const mockHttpRequest = vi.fn();
const mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');

// Mock the dependencies
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {
      request: mockHttpRequest,
    },
  }),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      getServerUrl: mockGetServerUrl,
    }),
  };
});

describe('useGetUserType', () => {
  const mockSchema: ApiUserType = {
    id: 'schema-1',
    name: 'Employee',
    schema: {
      username: {type: 'string', required: true, unique: true},
      email: {type: 'string', required: true},
      isActive: {type: 'boolean'},
    },
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with loading state when id is provided', () => {
    mockHttpRequest.mockReturnValue(new Promise(() => null)); // Never resolves

    const {result} = renderHook(() => useGetUserType('schema-1'));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
  });

  it('should successfully fetch a single user type', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockSchema,
    });

    const schemaId = 'schema-1';
    const {result} = renderHook(() => useGetUserType(schemaId));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockSchema);
    expect(result.current.data?.id).toBe(schemaId);
    expect(result.current.data?.name).toBe('Employee');
  });

  it('should make correct API call with schema ID', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockSchema,
    });

    const schemaId = 'schema-1';
    renderHook(() => useGetUserType(schemaId));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/user-types/schema-1`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );
  });

  it('should use correct query key', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockSchema,
    });

    const schemaId = 'schema-1';
    const {result, queryClient} = renderHook(() => useGetUserType(schemaId));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const queryKey = [UserQueryKeys.USER_TYPE, schemaId];
    const cachedData = queryClient.getQueryData(queryKey);
    expect(cachedData).toEqual(mockSchema);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to fetch user type');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetUserType('schema-1'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should handle network error', async () => {
    const networkError = new Error('Network request failed');
    mockHttpRequest.mockRejectedValueOnce(networkError);

    const {result} = renderHook(() => useGetUserType('schema-1'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(networkError);
  });

  it('should not make API call when id is undefined', () => {
    const {result} = renderHook(() => useGetUserType(undefined));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should not make API call when id is empty string', () => {
    const {result} = renderHook(() => useGetUserType(''));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should use correct server URL from config', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockSchema,
    });

    renderHook(() => useGetUserType('schema-1'));

    await waitFor(() => {
      expect(mockGetServerUrl).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('https://api.test.com/user-types/schema-1');
  });

  it('should include correct headers', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockSchema,
    });

    renderHook(() => useGetUserType('schema-1'));

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

  it('should refetch when id changes', async () => {
    const schema1 = {...mockSchema, id: 'schema-1', name: 'Employee'};
    const schema2 = {...mockSchema, id: 'schema-2', name: 'Contractor'};

    mockHttpRequest.mockResolvedValueOnce({data: schema1}).mockResolvedValueOnce({data: schema2});

    const {result, rerender} = renderHook(({id}: {id: string}) => useGetUserType(id), {
      initialProps: {id: 'schema-1'},
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.id).toBe('schema-1');
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    // Change the schema ID
    rerender({id: 'schema-2'});

    await waitFor(() => {
      expect(result.current.data?.id).toBe('schema-2');
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should cache schema data', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockSchema,
    });

    const schemaId = 'schema-1';

    // First call - get the queryClient from the render result
    const {result: result1, queryClient} = renderHook(() => useGetUserType(schemaId));

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    // Set the data as fresh to prevent refetch
    queryClient.setQueryDefaults([UserQueryKeys.USER_TYPE, schemaId], {
      staleTime: Infinity,
    });

    // Second call with same queryClient should use cache
    const {result: result2} = renderHook(() => useGetUserType(schemaId), {
      queryClient,
    });

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true);
    });

    // Should still be called only once due to caching
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    expect(result2.current.data).toEqual(mockSchema);
  });

  it('should support refetching data', async () => {
    mockHttpRequest.mockResolvedValue({
      data: mockSchema,
    });

    const {result} = renderHook(() => useGetUserType('schema-1'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    // Refetch the data
    await result.current.refetch();

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should handle schema with complex properties', async () => {
    const complexSchema: ApiUserType = {
      id: 'schema-complex',
      name: 'ComplexUser',
      schema: {
        name: {type: 'string', required: true},
        age: {type: 'number', required: false},
        isActive: {type: 'boolean', required: true},
        roles: {
          type: 'array',
          items: {type: 'string'},
          required: false,
        },
        address: {
          type: 'object',
          properties: {
            street: {type: 'string', required: true},
            city: {type: 'string', required: true},
          },
          required: false,
        },
      },
    };

    mockHttpRequest.mockResolvedValueOnce({
      data: complexSchema,
    });

    const {result} = renderHook(() => useGetUserType('schema-complex'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(complexSchema);
  });

  it('should handle different schema IDs', async () => {
    const schema1 = {...mockSchema, id: 'schema-1', name: 'Employee'};
    const schema2 = {...mockSchema, id: 'schema-2', name: 'Contractor'};

    mockHttpRequest.mockResolvedValueOnce({data: schema1});

    const {result: result1} = renderHook(() => useGetUserType('schema-1'));

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    expect(result1.current.data?.id).toBe('schema-1');

    mockHttpRequest.mockResolvedValueOnce({data: schema2});

    const {result: result2} = renderHook(() => useGetUserType('schema-2'));

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true);
    });

    expect(result2.current.data?.id).toBe('schema-2');
  });

  it('should maintain correct loading state during fetch', async () => {
    let resolveRequest: (value: {data: ApiUserType}) => void;
    const requestPromise = new Promise<{data: ApiUserType}>((resolve) => {
      resolveRequest = resolve;
    });

    mockHttpRequest.mockReturnValueOnce(requestPromise);

    const {result} = renderHook(() => useGetUserType('schema-1'));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.isFetching).toBe(true);
    expect(result.current.data).toBeUndefined();

    resolveRequest!({data: mockSchema});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toEqual(mockSchema);
  });
});
