// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ApplicationQueryKeys} from '@thunderid/configure-applications';
import type {Application, InboundAuthConfig} from '@thunderid/configure-applications';
import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import useRegenerateClientSecret from '../useRegenerateClientSecret';

vi.mock('@thunderid/react', () => ({
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
    useToast: vi.fn(),
  };
});

const {useThunderID} = await import('@thunderid/react');
const {useConfig, useToast} = await import('@thunderid/contexts');

const FLOW_ID = '01900000-0000-7000-8000-00000000007a';
const FLOW_HANDLE = 'default-client-secret-regeneration-flow';
const FLOW_SECRET = 'KdJL7PrfDTh-XwT8kGtg1nMydcDQg_L5an25vV52Ej8';

const flowConfiguredConfig = {merged: {clientSecretRegenerationFlow: {defaultHandle: FLOW_HANDLE}}};
const noFlowConfiguredConfig = {merged: {}};
const administrationFlows = {flows: [{flowType: 'ADMINISTRATION', handle: FLOW_HANDLE, id: FLOW_ID}]};

/**
 * The request shape the hook issues, as far as these tests inspect it.
 */
interface RecordedRequest {
  url: string;
  method: string;
  data?: unknown;
}

/**
 * The call signature of the mocked client, so a routed implementation returning a promise type-checks.
 */
type HttpRequest = (config: unknown) => Promise<{data?: unknown}>;

type HttpRequestMock = ReturnType<typeof vi.fn<HttpRequest>>;

describe('useRegenerateClientSecret', () => {
  let mockHttpRequest: HttpRequestMock;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;
  let mockShowToast: ReturnType<typeof vi.fn>;

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
              userConfig: {
                validityPeriod: 3600,
                attributes: ['email'],
              },
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
    mockHttpRequest = vi.fn<HttpRequest>();
    mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');
    mockShowToast = vi.fn();

    vi.mocked(useThunderID).mockReturnValue({
      http: {
        request: mockHttpRequest,
      },
    } as unknown as ReturnType<typeof useThunderID>);

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: mockGetServerUrl,
    } as unknown as ReturnType<typeof useConfig>);

    vi.mocked(useToast).mockReturnValue({
      showToast: mockShowToast,
    } as unknown as ReturnType<typeof useToast>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  /**
   * Answers the mocked client by request rather than by call order.
   *
   * A regeneration issues up to four requests (read the configured handle, resolve it to a flow id,
   * then either execute the flow or read and update the application), so a positional mock would
   * attach a response to the wrong call. A route key may be prefixed with a method, as in
   * `'PUT /applications/'`, because the fallback path reads and writes the same URL. A route value may
   * be an `Error` to reject, or a function returning a promise when a test needs per-call behaviour.
   */
  function routeHttp(routes: Record<string, unknown>): void {
    mockHttpRequest.mockImplementation((config: unknown): Promise<{data?: unknown}> => {
      const {method, url} = config as RecordedRequest;
      const keys = Object.keys(routes);
      const key =
        keys.find((candidate) => candidate.startsWith(`${method} `) && url.includes(candidate.split(' ')[1])) ??
        keys.find((candidate) => !candidate.includes(' ') && url.includes(candidate));

      if (key === undefined) {
        return Promise.reject(new Error(`unexpected request: ${method} ${url}`));
      }

      const value = routes[key];

      if (value instanceof Error) {
        return Promise.reject(value);
      }
      if (typeof value === 'function') {
        return (value as () => Promise<{data?: unknown}>)();
      }

      return Promise.resolve({data: value});
    });
  }

  /**
   * Arranges a successful rotation through the regeneration flow, which is what the shipped
   * configuration produces.
   */
  function mockFlowRotation(overrides: Record<string, unknown> = {}): void {
    routeHttp({
      '/flow/execute': {data: {additionalData: {clientSecret: FLOW_SECRET}}, flowStatus: 'COMPLETE'},
      '/flows': administrationFlows,
      '/server-config/flow': flowConfiguredConfig,
      ...overrides,
    });
  }

  /**
   * Arranges a successful rotation through the update endpoint, as a deployment with no regeneration
   * flow configured gets.
   */
  function mockNativeRotation(overrides: Record<string, unknown> = {}): void {
    routeHttp({
      'GET /applications/': mockApplication,
      'PUT /applications/': mockUpdatedApplication,
      '/server-config/flow': noFlowConfiguredConfig,
      ...overrides,
    });
  }

  /**
   * The requests matching a method and URL fragment, so assertions can ignore the lookups that
   * precede the rotation itself.
   */
  function requestsTo(method: string, fragment: string): RecordedRequest[] {
    return mockHttpRequest.mock.calls
      .map(([config]: [unknown]) => config as RecordedRequest)
      .filter((config: RecordedRequest) => config.method === method && config.url.includes(fragment));
  }

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useRegenerateClientSecret());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
  });

  // The flow generates the secret server-side and revokes the artifacts issued under the old one, so
  // the returned value must be the flow's rather than one the browser invented.
  it('should return the secret the regeneration flow produced', async () => {
    mockFlowRotation();

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.clientSecret).toBe(FLOW_SECRET);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/flow/execute',
        method: 'POST',
        data: {flowId: FLOW_ID, inputs: {targetApplicationId: applicationId}},
      }),
    );
    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  // Rotating through the flow and then through the update endpoint would rotate twice, leaving the
  // secret the caller was handed already stale.
  it('should not update the application when the flow rotated the secret', async () => {
    mockFlowRotation();

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(requestsTo('PUT', '/applications/')).toHaveLength(0);
  });

  // A flow that completed already rotated the credential, so falling back would rotate it a second
  // time and hand back a secret that no longer authenticates.
  it('should fail when the flow completes without returning a secret', async () => {
    mockFlowRotation({'/flow/execute': {flowStatus: 'COMPLETE'}});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toContain('without returning a secret');
    expect(requestsTo('PUT', '/applications/')).toHaveLength(0);
  });

  it('should fetch current application then update with new secret', async () => {
    mockNativeRotation();

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(requestsTo('GET', `/applications/${applicationId}`)).toHaveLength(1);
    expect(requestsTo('PUT', `/applications/${applicationId}`)).toHaveLength(1);
  });

  it('should return application and new client secret on success', async () => {
    mockNativeRotation();

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.application).toEqual(mockUpdatedApplication);
    expect(result.current.data?.clientSecret).toBeDefined();
    expect(typeof result.current.data?.clientSecret).toBe('string');
    expect(result.current.data!.clientSecret.length).toBeGreaterThan(0);
    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('should generate a base64url-encoded secret (no +, /, or = characters)', async () => {
    mockNativeRotation();

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const secret = result.current.data!.clientSecret;
    expect(secret).not.toMatch(/[+/=]/);
  });

  it('should strip server-generated fields (id, createdAt, updatedAt) from update request', async () => {
    mockNativeRotation();

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const putCall = requestsTo('PUT', '/applications/')[0] as {data: Record<string, unknown>};
    expect(putCall.data).not.toHaveProperty('id');
    expect(putCall.data).not.toHaveProperty('createdAt');
    expect(putCall.data).not.toHaveProperty('updatedAt');
  });

  it('should include the new client secret in the PUT request body', async () => {
    mockNativeRotation();

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const putCall = requestsTo('PUT', '/applications/')[0] as unknown as {
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

    mockNativeRotation({'GET /applications/': appWithoutOAuth});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toBe(
      'Application does not have an OAuth2 configuration. Cannot regenerate client secret.',
    );
  });

  // Treating a failed config read as "no flow configured" would rotate the secret without revoking
  // anything and report it as a success.
  it('should propagate a failed flow config read rather than rotating natively', async () => {
    routeHttp({
      'GET /applications/': mockApplication,
      'PUT /applications/': mockUpdatedApplication,
      '/server-config/flow': new Error('config unavailable'),
    });

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toBe('config unavailable');
    expect(requestsTo('PUT', '/applications/')).toHaveLength(0);
  });

  it('should handle GET request failure', async () => {
    mockNativeRotation({'GET /applications/': new Error('Failed to fetch application')});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toBe('Failed to fetch application');
    expect(requestsTo('PUT', '/applications/')).toHaveLength(0);
  });

  it('should handle PUT request failure', async () => {
    mockNativeRotation({'PUT /applications/': new Error('Failed to update application')});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toBe('Failed to update application');
    expect(requestsTo('GET', '/applications/')).toHaveLength(1);
  });

  it('should not show a toast on error', async () => {
    mockNativeRotation({'PUT /applications/': new Error('Failed to update application')});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('should invalidate queries on successful regeneration', async () => {
    mockFlowRotation();

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
    mockNativeRotation();

    const {result, queryClient} = renderHook(() => useRegenerateClientSecret());

    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValue(new Error('Invalidation failed'));

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.application).toEqual(mockUpdatedApplication);
  });

  it('should generate unique secrets on consecutive calls', async () => {
    mockNativeRotation();

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const firstSecret = result.current.data!.clientSecret;

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(requestsTo('PUT', '/applications/')).toHaveLength(2);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const secondSecret = result.current.data!.clientSecret;

    // Cryptographically random secrets should be different
    expect(firstSecret).not.toBe(secondSecret);
  });
});
