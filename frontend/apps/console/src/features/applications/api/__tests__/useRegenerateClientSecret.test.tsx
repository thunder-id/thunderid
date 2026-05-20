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
import ApplicationQueryKeys from '../../constants/application-query-keys';
import type {Application} from '../../models/application';
import type {InboundAuthConfig} from '../../models/inbound-auth';
import useRegenerateClientSecret from '../useRegenerateClientSecret';

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

describe('useRegenerateClientSecret', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;

  const applicationId = '550e8400-e29b-41d4-a716-446655440000';

  const mockApplication: Application = {
    id: applicationId,
    name: 'Test Application',
    description: 'Test description',
    url: 'https://test-app.com',
    authFlowId: 'flow-1',
    registrationFlowId: 'reg-flow-1',
    isRegistrationFlowEnabled: true,
    inboundAuthConfig: [
      {
        type: 'oauth2',
        config: {
          clientId: 'test-client-id',
          clientSecret: 'old-secret',
          redirectUris: ['https://test-app.com/callback'],
          grantTypes: ['authorization_code'],
          responseTypes: ['code'],
          pkceRequired: false,
          tokenEndpointAuthMethod: 'client_secret_basic',
          publicClient: false,
          token: {
            accessToken: {
              validityPeriod: 3600,
              userAttributes: ['email'],
            },
            idToken: {
              validityPeriod: 3600,
              userAttributes: ['email'],
            },
          },
          scopeClaims: {
            profile: ['name'],
            email: ['email'],
          },
          scopes: ['openid'],
        },
      },
    ],
    createdAt: '2025-11-13T10:00:00Z',
    updatedAt: '2025-11-14T15:30:00Z',
  };

  const mockUpdatedApplication: Application = {
    ...mockApplication,
    updatedAt: '2025-11-15T12:00:00Z',
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
    const {result} = renderHook(() => useRegenerateClientSecret());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
  });

  it('should fetch current application then update with new secret', async () => {
    // First call: GET current application
    mockHttpRequest.mockResolvedValueOnce({data: mockApplication});
    // Second call: PUT updated application
    mockHttpRequest.mockResolvedValueOnce({data: mockUpdatedApplication});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Should have made two API calls: GET then PUT
    expect(mockHttpRequest).toHaveBeenCalledTimes(2);

    // First call should be GET
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        url: `https://api.test.com/applications/${applicationId}`,
        method: 'GET',
      }),
    );

    // Second call should be PUT
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        url: `https://api.test.com/applications/${applicationId}`,
        method: 'PUT',
      }),
    );
  });

  it('should return application and new client secret on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockApplication});
    mockHttpRequest.mockResolvedValueOnce({data: mockUpdatedApplication});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.application).toEqual(mockUpdatedApplication);
    expect(result.current.data?.clientSecret).toBeDefined();
    expect(typeof result.current.data?.clientSecret).toBe('string');
    expect(result.current.data!.clientSecret.length).toBeGreaterThan(0);
  });

  it('should generate a base64url-encoded secret (no +, /, or = characters)', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockApplication});
    mockHttpRequest.mockResolvedValueOnce({data: mockUpdatedApplication});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const secret = result.current.data!.clientSecret;
    expect(secret).not.toMatch(/[+/=]/);
  });

  it('should strip server-generated fields (id, createdAt, updatedAt) from update request', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockApplication});
    mockHttpRequest.mockResolvedValueOnce({data: mockUpdatedApplication});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const putCall = mockHttpRequest.mock.calls[1][0] as {data: Record<string, unknown>};
    expect(putCall.data).not.toHaveProperty('id');
    expect(putCall.data).not.toHaveProperty('createdAt');
    expect(putCall.data).not.toHaveProperty('updatedAt');
  });

  it('should include the new client secret in the PUT request body', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockApplication});
    mockHttpRequest.mockResolvedValueOnce({data: mockUpdatedApplication});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const putCall = mockHttpRequest.mock.calls[1][0] as {
      data: {inboundAuthConfig: {type: string; config: {clientSecret: string}}[]};
    };
    const oauth2Config = putCall.data.inboundAuthConfig.find((c: {type: string}) => c.type === 'oauth2');
    expect(oauth2Config?.config.clientSecret).toBe(result.current.data?.clientSecret);
  });

  it('should throw error when application has no OAuth2 configuration', async () => {
    const appWithoutOAuth: Application = {
      ...mockApplication,
      inboundAuthConfig: [
        {
          type: 'saml',
          config: {} as InboundAuthConfig['config'],
        },
      ],
    };

    mockHttpRequest.mockResolvedValueOnce({data: appWithoutOAuth});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toBe(
      'Application does not have an OAuth2 configuration. Cannot regenerate client secret.',
    );
  });

  it('should handle GET request failure', async () => {
    mockHttpRequest.mockRejectedValueOnce(new Error('Failed to fetch application'));

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toBe('Failed to fetch application');
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
  });

  it('should handle PUT request failure', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockApplication});
    mockHttpRequest.mockRejectedValueOnce(new Error('Failed to update application'));

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toBe('Failed to update application');
    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should invalidate queries on successful regeneration', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockApplication});
    mockHttpRequest.mockResolvedValueOnce({data: mockUpdatedApplication});

    const {result, queryClient} = renderHook(() => useRegenerateClientSecret());

    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

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
    mockHttpRequest.mockResolvedValueOnce({data: mockApplication});
    mockHttpRequest.mockResolvedValueOnce({data: mockUpdatedApplication});

    const {result, queryClient} = renderHook(() => useRegenerateClientSecret());

    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValue(new Error('Invalidation failed'));

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.application).toEqual(mockUpdatedApplication);
  });

  it('should generate unique secrets on consecutive calls', async () => {
    // First regeneration
    mockHttpRequest.mockResolvedValueOnce({data: mockApplication});
    mockHttpRequest.mockResolvedValueOnce({data: mockUpdatedApplication});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const firstSecret = result.current.data!.clientSecret;

    // Second regeneration
    mockHttpRequest.mockResolvedValueOnce({data: mockApplication});
    mockHttpRequest.mockResolvedValueOnce({data: mockUpdatedApplication});

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(4);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const secondSecret = result.current.data!.clientSecret;

    // Cryptographically random secrets should be different
    expect(firstSecret).not.toBe(secondSecret);
  });
});
