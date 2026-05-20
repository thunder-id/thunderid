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

import {useThunderID} from '@thunderid/react';
import {useConfig} from '@thunderid/contexts';
import {waitFor, act, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import ApplicationQueryKeys from '../../constants/application-query-keys';
import type {Application} from '../../models/application';
import type {CreateApplicationRequest} from '../../models/requests';
import useCreateApplication from '../useCreateApplication';

// Import mocked modules

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

describe('useCreateApplication', () => {
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
          clientId: 'CONSOLE',
          redirectUris: ['https://localhost:5191'],
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

  const mockRequest: CreateApplicationRequest = {
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
          clientId: 'CONSOLE',
          redirectUris: ['https://localhost:5191'],
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
              scopeClaims: {
                profile: ['name', 'given_name', 'family_name', 'picture'],
                email: ['email', 'email_verified'],
                phone: ['phone_number', 'phone_number_verified'],
                group: ['groups'],
              },
            },
          },
          scopes: ['openid', 'email', 'profile'],
        },
      },
    ],
    userAttributes: ['email', 'username'],
  };

  let mockHttpRequest: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    // Mock HTTP request function
    mockHttpRequest = vi.fn();

    // Mock useThunderID hook
    vi.mocked(useThunderID).mockReturnValue({
      http: {
        request: mockHttpRequest,
      },
    } as unknown as ReturnType<typeof useThunderID>);

    // Mock useConfig hook
    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => 'https://localhost:8090',
    } as ReturnType<typeof useConfig>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useCreateApplication());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(typeof result.current.mutate).toBe('function');
    expect(typeof result.current.mutateAsync).toBe('function');
  });

  it('should successfully create an application', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const {result} = renderHook(() => useCreateApplication());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockApplication);
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://localhost:8090/applications',
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      data: mockRequest,
    });
  });

  it('should set pending state during creation', async () => {
    let resolveRequest!: (value: {data: Application}) => void;
    mockHttpRequest.mockReturnValue(
      new Promise((resolve) => {
        resolveRequest = resolve;
      }),
    );

    const {result} = renderHook(() => useCreateApplication());

    act(() => {
      result.current.mutate(mockRequest);
    });

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    act(() => {
      resolveRequest({data: mockApplication});
    });

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });

    expect(result.current.isSuccess).toBe(true);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to create application');

    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useCreateApplication());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('should handle network error', async () => {
    const networkError = new Error('Network error');

    mockHttpRequest.mockRejectedValueOnce(networkError);

    const {result} = renderHook(() => useCreateApplication());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(networkError);
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('should invalidate applications query on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const {result, queryClient} = renderHook(() => useCreateApplication());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({
      queryKey: [ApplicationQueryKeys.APPLICATIONS],
    });
  });

  it('should handle invalidateQueries rejection gracefully', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const {result, queryClient} = renderHook(() => useCreateApplication());
    // Mock invalidateQueries to reject
    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValueOnce(new Error('Invalidation failed'));

    result.current.mutate(mockRequest);

    // The mutation should still succeed even if invalidateQueries fails
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockApplication);
  });

  it('should support mutateAsync for promise-based workflows', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const {result} = renderHook(() => useCreateApplication());

    const promise = result.current.mutateAsync(mockRequest);

    await expect(promise).resolves.toEqual(mockApplication);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toEqual(mockApplication);
  });

  it('should handle onSuccess callback', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const onSuccess = vi.fn();

    const {result} = renderHook(() => useCreateApplication());

    result.current.mutate(mockRequest, {
      onSuccess,
    });

    await waitFor(() => {
      expect(onSuccess).toHaveBeenCalledWith(
        mockApplication,
        mockRequest,
        undefined,
        expect.objectContaining({
          // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
          client: expect.anything(),
        }),
      );
    });
  });

  it('should handle onError callback', async () => {
    const apiError = new Error('Failed to create application');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const onError = vi.fn();

    const {result} = renderHook(() => useCreateApplication());

    result.current.mutate(mockRequest, {
      onError,
    });

    await waitFor(() => {
      expect(onError).toHaveBeenCalledWith(
        apiError,
        mockRequest,
        undefined,
        expect.objectContaining({
          // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
          client: expect.anything(),
        }),
      );
    });
  });

  it('should reset mutation state', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const {result} = renderHook(() => useCreateApplication());

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

  it('should handle multiple sequential mutations', async () => {
    const firstApp = {...mockApplication, id: 'first-id'};
    const secondApp = {...mockApplication, id: 'second-id'};

    mockHttpRequest.mockResolvedValueOnce({
      data: firstApp,
    });

    const {result} = renderHook(() => useCreateApplication());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
      expect(result.current.data).toEqual(firstApp);
    });

    mockHttpRequest.mockResolvedValueOnce({
      data: secondApp,
    });

    result.current.mutate({...mockRequest, name: 'Second App'});

    await waitFor(() => {
      expect(result.current.data).toEqual(secondApp);
    });
  });

  it('should use correct server URL from config', async () => {
    const customServerUrl = 'https://custom-server.com:9090';

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => customServerUrl,
    } as ReturnType<typeof useConfig>);

    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const {result} = renderHook(() => useCreateApplication());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: `${customServerUrl}/applications`,
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      data: mockRequest,
    });
  });

  it('should properly serialize request data as JSON', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const {result} = renderHook(() => useCreateApplication());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.data).toEqual(mockRequest);
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.headers['Content-Type']).toBe('application/json');
  });
});
