// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ApplicationQueryKeys} from '@thunderid/configure-applications';
import type {ApplicationListResponse} from '@thunderid/configure-applications';
import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import useDeleteApplication from '../useDeleteApplication';

// Mock the dependencies
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

const FLOW_ID = '01900000-0000-7000-8000-000000000079';
const FLOW_HANDLE = 'default-application-deletion-flow';

const flowConfiguredConfig = {merged: {applicationDeletionFlow: {defaultHandle: FLOW_HANDLE}}};
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

describe('useDeleteApplication', () => {
  let mockHttpRequest: HttpRequestMock;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;
  let mockShowToast: ReturnType<typeof vi.fn>;

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
   * Answers the mocked client by URL rather than by call order.
   *
   * A deletion issues up to three requests (read the configured handle, resolve it to a flow id, then
   * execute), so a positional mock would attach a response to the wrong call. A route value may be an
   * `Error` to reject, or a function returning a promise when a test needs per-call behaviour.
   */
  function routeHttp(routes: Record<string, unknown>): void {
    mockHttpRequest.mockImplementation((config: unknown): Promise<{data?: unknown}> => {
      const {url} = config as RecordedRequest;
      const key = Object.keys(routes).find((candidate) => url.includes(candidate));

      if (key === undefined) {
        return Promise.reject(new Error(`unexpected request: ${url}`));
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
   * Arranges a successful flow deletion, which is what the shipped configuration produces.
   */
  function mockFlowDeletion(overrides: Record<string, unknown> = {}): void {
    routeHttp({
      '/flow/execute': {flowStatus: 'COMPLETE'},
      '/flows': administrationFlows,
      '/server-config/flow': flowConfiguredConfig,
      ...overrides,
    });
  }

  /**
   * The requests whose URL contains the given fragment, so assertions can ignore the lookups that
   * precede the deletion itself.
   */
  function requestsTo(fragment: string): RecordedRequest[] {
    return mockHttpRequest.mock.calls
      .map(([config]: [unknown]) => config as RecordedRequest)
      .filter((config: RecordedRequest) => config.url.includes(fragment));
  }

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useDeleteApplication());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(typeof result.current.mutate).toBe('function');
    expect(typeof result.current.mutateAsync).toBe('function');
  });

  it('should successfully delete an application', async () => {
    mockFlowDeletion();

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('should delete through the configured administration flow', async () => {
    mockFlowDeletion();

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/flow/execute',
        method: 'POST',
        data: {flowId: FLOW_ID, inputs: {targetApplicationId: applicationId}},
      }),
    );
    // Going through the flow is what revokes the application's tokens, so the native endpoint must not
    // also be called: that would be a second, unrevoked deletion path.
    expect(requestsTo('/applications/')).toHaveLength(0);
  });

  // A deployment that configures no deletion flow keeps deleting applications through the endpoint.
  it('should fall back to the native endpoint when no flow is configured', async () => {
    routeHttp({'/applications/': {}, '/server-config/flow': noFlowConfiguredConfig});

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/applications/${applicationId}`,
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );
  });

  // A flow that exists but fails must never be downgraded to a native delete: that would remove the
  // application while leaving every token it issued valid.
  it('should not fall back to the native endpoint when the flow fails', async () => {
    mockFlowDeletion({'/applications/': {}, '/flow/execute': new Error('flow refused')});

    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate('550e8400-e29b-41d4-a716-446655440000');

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(requestsTo('/applications/')).toHaveLength(0);
  });

  it('should set pending state during deletion', async () => {
    mockFlowDeletion({
      '/flow/execute': () =>
        new Promise((resolve) => {
          setTimeout(() => resolve({data: {flowStatus: 'COMPLETE'}}), 100);
        }),
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    await waitFor(
      () => {
        expect(result.current.isSuccess).toBe(true);
      },
      {timeout: 200},
    );

    expect(result.current.isPending).toBe(false);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to delete application');
    mockFlowDeletion({'/flow/execute': apiError});

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('should not show a toast on error', async () => {
    mockFlowDeletion({'/flow/execute': new Error('Failed to delete application')});

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('should handle network error', async () => {
    const networkError = new Error('Network request failed');
    mockFlowDeletion({'/flow/execute': networkError});

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(networkError);
    expect(result.current.isPending).toBe(false);
  });

  it('should handle 404 Not Found error', async () => {
    const notFoundError = new Error('Application not found');
    mockFlowDeletion({'/flow/execute': notFoundError});

    const applicationId = 'non-existent-id';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(notFoundError);
  });

  it('should remove application from cache on successful deletion', async () => {
    mockFlowDeletion();

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result, queryClient} = renderHook(() => useDeleteApplication());

    // Pre-populate cache with application
    queryClient.setQueryData([ApplicationQueryKeys.APPLICATION, applicationId], {
      id: applicationId,
      name: 'App to Delete',
    });

    const removeQueriesSpy = vi.spyOn(queryClient, 'removeQueries');

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify that removeQueries was called for the specific application
    expect(removeQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [ApplicationQueryKeys.APPLICATION, applicationId],
      }),
    );
  });

  it('should invalidate applications list on successful deletion', async () => {
    mockFlowDeletion();

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result, queryClient} = renderHook(() => useDeleteApplication());

    // Pre-populate cache with applications list
    const mockApplicationsList: ApplicationListResponse = {
      applications: [
        {
          id: applicationId,
          name: 'App to Delete',
          description: 'Description',
          logoUrl: 'https://test.com/logo.png',
          authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
          registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
          isRegistrationFlowEnabled: false,
        },
      ],
      totalResults: 1,
      count: 1,
    };

    queryClient.setQueryData([ApplicationQueryKeys.APPLICATIONS], mockApplicationsList);

    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify that invalidateQueries was called for the applications list
    expect(invalidateQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [ApplicationQueryKeys.APPLICATIONS],
      }),
    );
  });

  it('should handle invalidateQueries rejection gracefully', async () => {
    mockFlowDeletion();

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result, queryClient} = renderHook(() => useDeleteApplication());

    // Mock invalidateQueries to reject
    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValueOnce(new Error('Invalidation failed'));

    result.current.mutate(applicationId);

    // The mutation should still succeed even if invalidateQueries fails
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('should handle multiple sequential deletions', async () => {
    mockFlowDeletion();

    const app1Id = 'app-1';
    const app2Id = 'app-2';

    const {result} = renderHook(() => useDeleteApplication());

    // Delete first application
    result.current.mutate(app1Id);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Delete second application
    result.current.mutate(app2Id);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const executions = requestsTo('/flow/execute');
    expect(executions).toHaveLength(2);
    expect(executions[0].data).toEqual({flowId: FLOW_ID, inputs: {targetApplicationId: app1Id}});
    expect(executions[1].data).toEqual({flowId: FLOW_ID, inputs: {targetApplicationId: app2Id}});
  });

  it('should handle permission error (403 Forbidden)', async () => {
    const forbiddenError = new Error('Permission denied');
    mockFlowDeletion({'/flow/execute': forbiddenError});

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(forbiddenError);
  });

  it('should use mutateAsync for promise-based deletion', async () => {
    mockFlowDeletion();

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    const deletePromise = result.current.mutateAsync(applicationId);

    await expect(deletePromise).resolves.toBeUndefined();

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('should reject mutateAsync on error', async () => {
    const apiError = new Error('Deletion failed');
    mockFlowDeletion({'/flow/execute': apiError});

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    const deletePromise = result.current.mutateAsync(applicationId);

    await expect(deletePromise).rejects.toEqual(apiError);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });

  it('should not affect other cached applications on deletion', async () => {
    mockFlowDeletion();

    const app1Id = 'app-1';
    const app2Id = 'app-2';

    // Pre-populate cache with two applications
    const app1Data = {id: app1Id, name: 'App 1'};
    const app2Data = {id: app2Id, name: 'App 2'};

    const {result, queryClient} = renderHook(() => useDeleteApplication());

    queryClient.setQueryData([ApplicationQueryKeys.APPLICATION, app1Id], app1Data);
    queryClient.setQueryData([ApplicationQueryKeys.APPLICATION, app2Id], app2Data);

    // Delete first application
    result.current.mutate(app1Id);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify that app2 is still in the cache
    const app2InCache = queryClient.getQueryData([ApplicationQueryKeys.APPLICATION, app2Id]);
    expect(app2InCache).toEqual(app2Data);
  });

  it('should handle concurrent deletion attempts', async () => {
    mockFlowDeletion();

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    // Trigger multiple deletions concurrently
    result.current.mutate(applicationId);
    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Note: TanStack Query will handle the concurrent mutations
    // The second mutation will override the first one's state
    expect(requestsTo('/flow/execute')).toHaveLength(2);
  });

  it('should clear error state on successful retry', async () => {
    const apiError = new Error('Temporary error');
    let attempt = 0;
    mockFlowDeletion({
      '/flow/execute': () => {
        attempt += 1;
        return attempt === 1 ? Promise.reject(apiError) : Promise.resolve({data: {flowStatus: 'COMPLETE'}});
      },
    });

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    // First attempt - should fail
    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);

    // Second attempt - should succeed
    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.error).toBeNull();
    expect(requestsTo('/flow/execute')).toHaveLength(2);
  });

  it('should handle server returning 204 No Content', async () => {
    // 204 No Content is the typical response for successful DELETE
    mockFlowDeletion();

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeUndefined();
  });

  it('should pass through server error messages', async () => {
    const serverError = new Error('Application has active users and cannot be deleted');
    mockFlowDeletion({'/flow/execute': serverError});

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(serverError);
    expect(result.current.error?.message).toBe('Application has active users and cannot be deleted');
  });
});
