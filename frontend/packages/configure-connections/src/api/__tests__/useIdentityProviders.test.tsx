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

import {QueryClient} from '@tanstack/react-query';
import {waitFor, act, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import ConnectionQueryKeys from '../../constants/query-keys';
import type {ConnectionListResponse} from '../../models/connection';
import {IdentityProviderTypes} from '../../models/identity-provider';
import type {IdentityProviderListResponse} from '../../models/responses';
import useIdentityProviders from '../useIdentityProviders';

const mockHttpRequest = vi.fn();

// Mock the dependencies
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'http://localhost:8090'}),
  };
});

/** Builds a GET /connections envelope response for the mock HTTP client. */
const connectionsResponse = (connections: ConnectionListResponse['connections']): {data: ConnectionListResponse} => ({
  data: {totalResults: connections.length, startIndex: 1, count: connections.length, connections, links: []},
});

describe('useIdentityProviders', () => {
  const mockIdentityProviders: IdentityProviderListResponse = [
    {
      id: 'idp-1',
      name: 'Google',
      description: 'Login with Google',
      type: IdentityProviderTypes.GOOGLE,
    },
    {
      id: 'idp-2',
      name: 'GitHub',
      description: 'Login with GitHub',
      type: IdentityProviderTypes.GITHUB,
    },
    {
      id: 'idp-3',
      name: 'Custom OIDC',
      description: 'Custom OpenID Connect Provider',
      type: IdentityProviderTypes.OIDC,
    },
  ];

  const mockConnections: ConnectionListResponse['connections'] = [
    {id: 'idp-1', name: 'Google', description: 'Login with Google', type: 'google', categories: ['identity-provider']},
    {id: 'idp-2', name: 'GitHub', description: 'Login with GitHub', type: 'github', categories: ['identity-provider']},
    {
      id: 'idp-3',
      name: 'Custom OIDC',
      description: 'Custom OpenID Connect Provider',
      type: 'oidc',
      categories: ['identity-provider'],
    },
  ];

  beforeEach(() => {
    mockHttpRequest.mockReset().mockResolvedValue(connectionsResponse(mockConnections));
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('Initialization', () => {
    it('should initialize with loading state', () => {
      const {result} = renderHook(() => useIdentityProviders());

      expect(result.current.isLoading).toBe(true);
      expect(result.current.data).toBeUndefined();
      expect(result.current.error).toBeNull();
    });
  });

  describe('Successful Fetch', () => {
    it('should fetch identity providers successfully, mapped to UPPERCASE type', async () => {
      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockIdentityProviders);
      expect(result.current.error).toBeNull();
    });

    it('should call GET /connections?category=identity-provider', async () => {
      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: 'http://localhost:8090/connections?category=identity-provider',
          method: 'GET',
          headers: {
            'Content-Type': 'application/json',
          },
        }),
      );
    });

    it('should return array of identity providers', async () => {
      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(Array.isArray(result.current.data)).toBe(true);
      expect(result.current.data).toHaveLength(3);
    });

    it('should return identity providers with correct structure', async () => {
      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      const firstProvider = result.current.data?.[0];
      expect(firstProvider).toHaveProperty('id');
      expect(firstProvider).toHaveProperty('name');
      expect(firstProvider).toHaveProperty('type');
    });

    it('should handle empty list response', async () => {
      mockHttpRequest.mockResolvedValue(connectionsResponse([]));

      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual([]);
      expect(Array.isArray(result.current.data)).toBe(true);
    });

    it('should drop instances whose vendor type has no IdP mapping', async () => {
      mockHttpRequest.mockResolvedValue(
        connectionsResponse([
          ...mockConnections,
          {id: 'sender-1', name: 'Twilio SMS', type: 'twilio', categories: ['sms-provider']},
        ]),
      );

      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toHaveLength(3);
      expect(result.current.data?.some((idp) => idp.id === 'sender-1')).toBe(false);
    });
  });

  describe('Error Handling', () => {
    it('should handle fetch errors', async () => {
      const mockError = new Error('Failed to fetch identity providers');
      mockHttpRequest.mockRejectedValue(mockError);

      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error).toEqual(mockError);
      expect(result.current.data).toBeUndefined();
    });

    it('should handle 404 errors', async () => {
      const notFoundError = new Error('Not found');
      mockHttpRequest.mockRejectedValue(notFoundError);

      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error?.message).toBe('Not found');
    });

    it('should handle network errors', async () => {
      const networkError = new Error('Network error');
      mockHttpRequest.mockRejectedValue(networkError);

      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error?.message).toBe('Network error');
      expect(result.current.isSuccess).toBe(false);
    });

    it('should maintain error state after failed fetch', async () => {
      const mockError = new Error('Server error');
      mockHttpRequest.mockRejectedValue(mockError);

      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error?.message).toBe('Server error');
      expect(result.current.data).toBeUndefined();
      expect(result.current.isSuccess).toBe(false);
    });
  });

  describe('Loading State', () => {
    it('should show loading state during fetch', async () => {
      mockHttpRequest.mockReturnValue(
        new Promise((resolve) => {
          setTimeout(() => resolve(connectionsResponse(mockConnections)), 100);
        }),
      );

      const {result} = renderHook(() => useIdentityProviders());

      expect(result.current.isLoading).toBe(true);
      expect(result.current.data).toBeUndefined();

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.isLoading).toBe(false);
      expect(result.current.data).toEqual(mockIdentityProviders);
    });
  });

  describe('Refetching', () => {
    it('should refetch identity providers on demand', async () => {
      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(mockHttpRequest).toHaveBeenCalledTimes(1);

      await act(async () => {
        await result.current.refetch();
      });

      expect(mockHttpRequest).toHaveBeenCalledTimes(2);
    });

    it('should fetch fresh data on refetch', async () => {
      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toHaveLength(3);

      mockHttpRequest.mockResolvedValue(
        connectionsResponse([
          ...mockConnections,
          {id: 'idp-4', name: 'OAuth Provider', type: 'oauth', categories: ['identity-provider']},
        ]),
      );

      await act(async () => {
        await result.current.refetch();
      });

      await waitFor(() => {
        expect(result.current.data).toHaveLength(4);
      });

      expect(mockHttpRequest).toHaveBeenCalledTimes(2);
    });
  });

  describe('Query Keys', () => {
    it('should use correct query key structure', async () => {
      const {result, queryClient} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      // Verify query key is accessible through the query cache
      const queries = queryClient.getQueryCache().getAll();
      const query = queries.find((q) => {
        const key = q.queryKey as string[];
        return key[0] === ConnectionQueryKeys.CONNECTIONS && key[1] === ConnectionQueryKeys.IDENTITY_PROVIDERS;
      });

      expect(query).toBeDefined();
    });
  });

  describe('Caching', () => {
    it('should use cached results on subsequent renders', async () => {
      // This test needs a shared QueryClient to test caching behavior
      const cacheQueryClient = new QueryClient({
        defaultOptions: {
          queries: {
            retry: false,
            gcTime: Infinity,
            staleTime: Infinity,
          },
        },
      });

      const {result: result1, unmount: unmount1} = renderHook(() => useIdentityProviders(), {
        queryClient: cacheQueryClient,
      });

      await waitFor(() => {
        expect(result1.current.isSuccess).toBe(true);
      });

      expect(mockHttpRequest).toHaveBeenCalledTimes(1);

      // Unmount first hook
      unmount1();

      const {result: result2} = renderHook(() => useIdentityProviders(), {
        queryClient: cacheQueryClient,
      });

      await waitFor(() => {
        expect(result2.current.isSuccess).toBe(true);
      });

      // Should still be 1 API call, second hook uses cache
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
      expect(result2.current.data).toEqual(mockIdentityProviders);
    });
  });

  describe('Multiple Provider Types', () => {
    it('should handle different identity provider types', async () => {
      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      const types = result.current.data?.map((idp) => idp.type);
      expect(types).toContain(IdentityProviderTypes.GOOGLE);
      expect(types).toContain(IdentityProviderTypes.GITHUB);
      expect(types).toContain(IdentityProviderTypes.OIDC);
    });

    it('should include provider with optional description', async () => {
      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      const googleProvider = result.current.data?.find((idp) => idp.type === IdentityProviderTypes.GOOGLE);
      expect(googleProvider?.description).toBe('Login with Google');
    });

    it('should handle provider without description', async () => {
      mockHttpRequest.mockResolvedValue(
        connectionsResponse([{id: 'idp-1', name: 'Google', type: 'google', categories: ['identity-provider']}]),
      );

      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data?.[0].description).toBeUndefined();
    });
  });

  describe('Edge Cases', () => {
    it('should handle response with single provider', async () => {
      mockHttpRequest.mockResolvedValue(
        connectionsResponse([{id: 'idp-1', name: 'Google', type: 'google', categories: ['identity-provider']}]),
      );

      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toHaveLength(1);
      expect(result.current.data?.[0].id).toBe('idp-1');
    });

    it('should handle very large list of providers', async () => {
      const largeList: ConnectionListResponse['connections'] = Array.from({length: 100}, (_, i) => ({
        id: `idp-${i}`,
        name: `Provider ${i}`,
        type: 'oidc',
        categories: ['identity-provider'],
      }));

      mockHttpRequest.mockResolvedValue(connectionsResponse(largeList));

      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toHaveLength(100);
    });
  });

  describe('React Query Features', () => {
    it('should support isLoading flag', async () => {
      mockHttpRequest.mockReturnValue(
        new Promise((resolve) => {
          setTimeout(() => resolve(connectionsResponse(mockConnections)), 50);
        }),
      );

      const {result} = renderHook(() => useIdentityProviders());

      expect(result.current.isLoading).toBe(true);

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });
    });

    it('should support isFetching flag', async () => {
      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.isFetching).toBe(false);

      await act(async () => {
        await result.current.refetch();
      });

      expect(result.current.isFetching).toBe(false);
    });

    it('should complete refetch successfully', async () => {
      const {result} = renderHook(() => useIdentityProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      const initialData = result.current.data;
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);

      await act(async () => {
        await result.current.refetch();
      });

      expect(mockHttpRequest).toHaveBeenCalledTimes(2);
      expect(result.current.isSuccess).toBe(true);
      expect(result.current.data).toEqual(initialData);
    });
  });
});
