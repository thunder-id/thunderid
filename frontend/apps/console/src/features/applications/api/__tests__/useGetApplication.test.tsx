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
import type {Application} from '../../models/application';
import useGetApplication from '../useGetApplication';

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

describe('useGetApplication', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;

  const mockApplication: Application = {
    id: '550e8400-e29b-41d4-a716-446655440000',
    name: 'Test Application',
    description: 'Test application description',
    url: 'https://test-app.com',
    logoUrl: 'https://test-app.com/logo.png',
    tosUri: 'https://test-app.com/terms',
    policyUri: 'https://test-app.com/privacy',
    contacts: ['admin@test-app.com'],
    authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
    registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
    isRegistrationFlowEnabled: true,
    inboundAuthConfig: [
      {
        type: 'oauth2',
        config: {
          clientId: 'test-client-id',
          redirectUris: ['https://test-app.com/callback'],
          grantTypes: ['authorization_code'],
          responseTypes: ['code'],
          pkceRequired: false,
          tokenEndpointAuthMethod: 'none',
          publicClient: true,
          token: {
            accessToken: {
              validityPeriod: 3600,
              userAttributes: ['given_name', 'family_name', 'email', 'groups', 'name'],
            },
            idToken: {
              validityPeriod: 3600,
              userAttributes: ['given_name', 'family_name', 'email', 'groups', 'name'],
            },
          },
          scopeClaims: {
            profile: ['name', 'given_name', 'family_name', 'picture'],
            email: ['email', 'email_verified'],
            phone: ['phone_number', 'phone_number_verified'],
            group: ['groups'],
          },
          scopes: ['openid', 'email', 'profile'],
        },
      },
    ],
    userAttributes: ['email', 'username'],
    createdAt: '2025-11-13T10:00:00Z',
    updatedAt: '2025-11-13T10:00:00Z',
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

  it('should initialize with loading state when applicationId is provided', () => {
    mockHttpRequest.mockReturnValue(new Promise(() => null)); // Never resolves

    const {result} = renderHook(() => useGetApplication('550e8400-e29b-41d4-a716-446655440000'));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
  });

  it('should successfully fetch a single application', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useGetApplication(applicationId));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockApplication);
    expect(result.current.data?.id).toBe(applicationId);
    expect(result.current.data?.name).toBe('Test Application');
  });

  it('should make correct API call with application ID', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    renderHook(() => useGetApplication(applicationId));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/applications/${applicationId}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );
  });

  it('should use correct query key', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result, queryClient} = renderHook(() => useGetApplication(applicationId));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const queryKey = [ApplicationQueryKeys.APPLICATION, applicationId];
    const cachedData = queryClient.getQueryData(queryKey);
    expect(cachedData).toEqual(mockApplication);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to fetch application');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetApplication('550e8400-e29b-41d4-a716-446655440000'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should handle 404 Not Found error', async () => {
    const notFoundError = new Error('Application not found');
    mockHttpRequest.mockRejectedValueOnce(notFoundError);

    const {result} = renderHook(() => useGetApplication('non-existent-id'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(notFoundError);
    expect(result.current.data).toBeUndefined();
  });

  it('should handle network error', async () => {
    const networkError = new Error('Network request failed');
    mockHttpRequest.mockRejectedValueOnce(networkError);

    const {result} = renderHook(() => useGetApplication('550e8400-e29b-41d4-a716-446655440000'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(networkError);
  });

  it('should not make API call when applicationId is empty string', () => {
    const {result} = renderHook(() => useGetApplication(''));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should not make API call when applicationId is falsy', () => {
    const {result} = renderHook(() => useGetApplication(''));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should handle different application IDs', async () => {
    const app1 = {...mockApplication, id: 'app-1', name: 'App 1'};
    const app2 = {...mockApplication, id: 'app-2', name: 'App 2'};

    mockHttpRequest.mockResolvedValueOnce({data: app1});

    const {result: result1} = renderHook(() => useGetApplication('app-1'));

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    expect(result1.current.data?.id).toBe('app-1');

    mockHttpRequest.mockResolvedValueOnce({data: app2});

    const {result: result2} = renderHook(() => useGetApplication('app-2'));

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true);
    });

    expect(result2.current.data?.id).toBe('app-2');
  });

  it('should handle server returning empty application details', async () => {
    const emptyApplication: Application = {
      id: '550e8400-e29b-41d4-a716-446655440000',
      name: '',
      description: '',
      authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
      registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
      isRegistrationFlowEnabled: false,
      createdAt: '2025-11-13T10:00:00Z',
      updatedAt: '2025-11-13T10:00:00Z',
    };

    mockHttpRequest.mockResolvedValueOnce({
      data: emptyApplication,
    });

    const {result} = renderHook(() => useGetApplication('550e8400-e29b-41d4-a716-446655440000'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(emptyApplication);
  });

  it('should refetch when applicationId changes', async () => {
    const app1 = {...mockApplication, id: 'app-1', name: 'App 1'};
    const app2 = {...mockApplication, id: 'app-2', name: 'App 2'};

    mockHttpRequest.mockResolvedValueOnce({data: app1}).mockResolvedValueOnce({data: app2});

    const {result, rerender} = renderHook(({appId}: {appId: string}) => useGetApplication(appId), {
      initialProps: {appId: 'app-1'},
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.id).toBe('app-1');
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    // Change the application ID
    rerender({appId: 'app-2'});

    await waitFor(() => {
      expect(result.current.data?.id).toBe('app-2');
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should cache application data', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';

    // First call - get the queryClient from the render result
    const {result: result1, queryClient} = renderHook(() => useGetApplication(applicationId));

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    // Set the data as fresh to prevent refetch
    queryClient.setQueryDefaults([ApplicationQueryKeys.APPLICATION, applicationId], {
      staleTime: Infinity,
    });

    // Second call with same queryClient should use cache
    const {result: result2} = renderHook(() => useGetApplication(applicationId), {
      queryClient,
    });

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true);
    });

    // Should still be called only once due to caching
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    expect(result2.current.data).toEqual(mockApplication);
  });

  it('should handle successful fetch with complete application data', async () => {
    const completeApplication: Application = {
      id: '550e8400-e29b-41d4-a716-446655440000',
      name: 'Complete Test App',
      description: 'A complete test application with all fields',
      url: 'https://complete-app.com',
      logoUrl: 'https://complete-app.com/logo.png',
      tosUri: 'https://complete-app.com/terms',
      policyUri: 'https://complete-app.com/privacy',
      contacts: ['admin@complete-app.com', 'support@complete-app.com'],
      authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
      registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
      isRegistrationFlowEnabled: true,
      inboundAuthConfig: [
        {
          type: 'oauth2',
          config: {
            clientId: 'test-client-id',
            redirectUris: ['https://complete-app.com/callback'],
            grantTypes: ['authorization_code'],
            responseTypes: ['code'],
            pkceRequired: false,
            tokenEndpointAuthMethod: 'none',
            publicClient: true,
            token: {
              accessToken: {
                validityPeriod: 3600,
                userAttributes: ['given_name', 'family_name', 'email', 'groups', 'name'],
              },
              idToken: {
                validityPeriod: 3600,
                userAttributes: ['given_name', 'family_name', 'email', 'groups', 'name'],
              },
            },
            scopeClaims: {
              profile: ['name', 'given_name', 'family_name', 'picture'],
              email: ['email', 'email_verified'],
              phone: ['phone_number', 'phone_number_verified'],
              group: ['groups'],
            },
            scopes: ['openid', 'email', 'profile'],
          },
        },
      ],
      userAttributes: ['email', 'username', 'profile', 'phone'],
      createdAt: '2025-11-13T10:00:00Z',
      updatedAt: '2025-11-14T15:30:00Z',
    };

    mockHttpRequest.mockResolvedValueOnce({
      data: completeApplication,
    });

    const {result} = renderHook(() => useGetApplication('550e8400-e29b-41d4-a716-446655440000'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(completeApplication);
    expect(result.current.data?.userAttributes).toHaveLength(4);
  });
});
