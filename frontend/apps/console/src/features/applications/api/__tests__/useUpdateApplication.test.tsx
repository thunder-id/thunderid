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
import type {CreateApplicationRequest} from '../../models/requests';
import useUpdateApplication from '../useUpdateApplication';

// Mock the dependencies
vi.mock('@thunderid/react', () => ({
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
    useToast: vi.fn().mockReturnValue({showToast: vi.fn()}),
  };
});

const {useThunderID} = await import('@thunderid/react');
const {useConfig} = await import('@thunderid/contexts');

describe('useUpdateApplication', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;

  const mockApplication: Application = {
    id: '550e8400-e29b-41d4-a716-446655440000',
    name: 'Updated Test Application',
    description: 'Updated test application description',
    url: 'https://updated-test-app.com',
    logoUrl: 'https://updated-test-app.com/logo.png',
    tosUri: 'https://updated-test-app.com/terms',
    policyUri: 'https://updated-test-app.com/privacy',
    contacts: ['admin@updated-test-app.com'],
    authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
    registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
    isRegistrationFlowEnabled: true,
    inboundAuthConfig: [
      {
        type: 'oauth2',
        config: {
          clientId: 'test-client-id',
          redirectUris: ['https://updated-test-app.com/callback'],
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
    userAttributes: ['email', 'username', 'profile'],
    createdAt: '2025-11-13T10:00:00Z',
    updatedAt: '2025-11-14T15:30:00Z',
  };

  const mockUpdateRequest: CreateApplicationRequest = {
    name: 'Updated Test Application',
    description: 'Updated test application description',
    url: 'https://updated-test-app.com',
    logoUrl: 'https://updated-test-app.com/logo.png',
    tosUri: 'https://updated-test-app.com/terms',
    policyUri: 'https://updated-test-app.com/privacy',
    contacts: ['admin@updated-test-app.com'],
    authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
    registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
    isRegistrationFlowEnabled: true,
    inboundAuthConfig: {
      grantTypes: ['authorization_code', 'refresh_token'],
      redirectUris: ['https://updated-test-app.com/callback'],
      allowed_origins: ['https://updated-test-app.com'],
      publicClient: false,
      pkce_mandatory: true,
      access_token_ttl: 7200,
      refresh_token_ttl: 172800,
      id_token_ttl: 7200,
    },
    userAttributes: ['email', 'username', 'profile'],
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

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useUpdateApplication());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(typeof result.current.mutate).toBe('function');
    expect(typeof result.current.mutateAsync).toBe('function');
  });

  it('should successfully update an application', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useUpdateApplication());

    result.current.mutate({applicationId, data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockApplication);
    expect(result.current.data?.name).toBe('Updated Test Application');
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
  });

  it('should make correct API call with application ID and data', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useUpdateApplication());

    result.current.mutate({applicationId, data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/applications/${applicationId}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data: mockUpdateRequest,
      }),
    );
  });

  it('should set pending state during update', async () => {
    let resolveRequest!: (value: {data: Application}) => void;
    mockHttpRequest.mockReturnValue(
      new Promise((resolve) => {
        resolveRequest = resolve;
      }),
    );

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useUpdateApplication());

    result.current.mutate({applicationId, data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    resolveRequest({data: mockApplication});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.isPending).toBe(false);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to update application');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useUpdateApplication());

    result.current.mutate({applicationId, data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('should handle network error', async () => {
    const networkError = new Error('Network request failed');
    mockHttpRequest.mockRejectedValueOnce(networkError);

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useUpdateApplication());

    result.current.mutate({applicationId, data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(networkError);
    expect(result.current.isPending).toBe(false);
  });

  it('should handle validation error', async () => {
    const validationError = new Error('Invalid application data');
    mockHttpRequest.mockRejectedValueOnce(validationError);

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useUpdateApplication());

    result.current.mutate({applicationId, data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(validationError);
  });

  it('should invalidate queries on successful update', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result, queryClient} = renderHook(() => useUpdateApplication());

    // Pre-populate cache with original application
    const originalApp = {...mockApplication, name: 'Original Name'};
    queryClient.setQueryData([ApplicationQueryKeys.APPLICATION, applicationId], originalApp);
    queryClient.setQueryData([ApplicationQueryKeys.APPLICATIONS], {
      applications: [originalApp],
      totalResults: 1,
      count: 1,
    });

    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({applicationId, data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify that invalidateQueries was called for both the specific application and the list
    expect(invalidateQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [ApplicationQueryKeys.APPLICATION, applicationId],
      }),
    );
    expect(invalidateQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [ApplicationQueryKeys.APPLICATIONS],
      }),
    );
  });

  it('should handle invalidateQueries rejection gracefully', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result, queryClient} = renderHook(() => useUpdateApplication());

    // Mock invalidateQueries to reject
    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValue(new Error('Invalidation failed'));

    result.current.mutate({applicationId, data: mockUpdateRequest});

    // The mutation should still succeed even if invalidateQueries fails
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockApplication);
  });

  it('should handle 404 Not Found error for non-existent application', async () => {
    const notFoundError = new Error('Application not found');
    mockHttpRequest.mockRejectedValueOnce(notFoundError);

    const applicationId = 'non-existent-id';
    const {result} = renderHook(() => useUpdateApplication());

    result.current.mutate({applicationId, data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(notFoundError);
  });

  it('should handle partial update', async () => {
    const partialUpdateRequest: CreateApplicationRequest = {
      name: 'Partially Updated App',
      description: mockUpdateRequest.description,
    };

    const partiallyUpdatedApp: Application = {
      ...mockApplication,
      name: 'Partially Updated App',
    };

    mockHttpRequest.mockResolvedValueOnce({
      data: partiallyUpdatedApp,
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useUpdateApplication());

    result.current.mutate({applicationId, data: partialUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.name).toBe('Partially Updated App');
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        data: partialUpdateRequest,
      }),
    );
  });

  it('should handle multiple sequential updates', async () => {
    const app1 = {...mockApplication, name: 'Update 1'};
    const app2 = {...mockApplication, name: 'Update 2'};

    mockHttpRequest.mockResolvedValueOnce({data: app1}).mockResolvedValueOnce({data: app2});

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useUpdateApplication());

    // First update
    result.current.mutate({
      applicationId,
      data: {...mockUpdateRequest, name: 'Update 1'},
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.name).toBe('Update 1');

    // Second update
    result.current.mutate({
      applicationId,
      data: {...mockUpdateRequest, name: 'Update 2'},
    });

    await waitFor(() => {
      expect(result.current.data?.name).toBe('Update 2');
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should update different applications independently', async () => {
    const app1 = {...mockApplication, id: 'app-1', name: 'App 1 Updated'};
    const app2 = {...mockApplication, id: 'app-2', name: 'App 2 Updated'};

    mockHttpRequest.mockResolvedValueOnce({data: app1}).mockResolvedValueOnce({data: app2});

    const {result} = renderHook(() => useUpdateApplication());

    // Update first application
    result.current.mutate({
      applicationId: 'app-1',
      data: {...mockUpdateRequest, name: 'App 1 Updated'},
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.id).toBe('app-1');

    // Update second application
    result.current.mutate({
      applicationId: 'app-2',
      data: {...mockUpdateRequest, name: 'App 2 Updated'},
    });

    await waitFor(() => {
      expect(result.current.data?.id).toBe('app-2');
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should handle updating application with empty optional fields', async () => {
    const minimalUpdateRequest: CreateApplicationRequest = {
      name: 'Minimal App',
    };

    const minimalApp: Application = {
      id: '550e8400-e29b-41d4-a716-446655440000',
      name: 'Minimal App',
      authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
      registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
      isRegistrationFlowEnabled: false,
      createdAt: '2025-11-13T10:00:00Z',
      updatedAt: '2025-11-14T15:30:00Z',
    };

    mockHttpRequest.mockResolvedValueOnce({
      data: minimalApp,
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useUpdateApplication());

    result.current.mutate({applicationId, data: minimalUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.name).toBe('Minimal App');
    expect(result.current.data?.description).toBeUndefined();
    expect(result.current.data?.url).toBeUndefined();
  });

  it('should use mutateAsync for promise-based updates', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockApplication,
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useUpdateApplication());

    const updatePromise = result.current.mutateAsync({applicationId, data: mockUpdateRequest});

    await expect(updatePromise).resolves.toEqual(mockApplication);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('should reject mutateAsync on error', async () => {
    const apiError = new Error('Update failed');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useUpdateApplication());

    const updatePromise = result.current.mutateAsync({applicationId, data: mockUpdateRequest});

    await expect(updatePromise).rejects.toEqual(apiError);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });

  it('should handle server returning updated timestamps', async () => {
    const updatedApp: Application = {
      ...mockApplication,
      updatedAt: '2025-11-15T12:00:00Z', // More recent timestamp
    };

    mockHttpRequest.mockResolvedValueOnce({
      data: updatedApp,
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useUpdateApplication());

    result.current.mutate({applicationId, data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.updatedAt).toBe('2025-11-15T12:00:00Z');
    expect(result.current.data?.createdAt).toBe(mockApplication.createdAt);
  });
});
