// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes} from '@thunderid/contexts';
import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import ApplicationQueryKeys from '../../constants/application-query-keys';
import type {ApplicationListResponse} from '../../models/responses';
import useDeleteApplication from '../useDeleteApplication';

// Mock the dependencies
vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
    useToast: vi.fn(),
    useAdministrationOperation: () => administrationOperation,
  };
});

// How the console says this operation runs. A console that declares nothing gets the flow mode.
let administrationOperation: unknown = AdministrationModes.FLOW;

const {useThunderID} = await import('@thunderid/react');
const {useConfig, useToast} = await import('@thunderid/contexts');

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
    administrationOperation = AdministrationModes.FLOW;

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
   * Answers the mocked client by URL. A route value may be an `Error` to reject, or a function
   * returning a promise when a test needs per-call behaviour.
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
   * Arranges a successful deletion, or one answered by the given value.
   */
  function mockDeletion(response: unknown = {}): void {
    routeHttp({'/applications/': response});
  }

  /**
   * The requests whose URL contains the given fragment.
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
    mockDeletion();

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

  it('should delete through the endpoint', async () => {
    mockDeletion();

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
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

  it('should set pending state during deletion', async () => {
    let resolveDeletion: () => void;
    mockDeletion(
      () =>
        new Promise((resolve) => {
          resolveDeletion = () => resolve({data: {}});
        }),
    );

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    resolveDeletion!();

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.isPending).toBe(false);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to delete application');
    mockDeletion(apiError);

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
    mockDeletion(new Error('Failed to delete application'));

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
    mockDeletion(networkError);

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
    mockDeletion(notFoundError);

    const applicationId = 'non-existent-id';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(notFoundError);
  });

  it('should remove application from cache on successful deletion', async () => {
    mockDeletion();

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
    mockDeletion();

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
    mockDeletion();

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
    mockDeletion();

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

    const deletions = requestsTo('/applications/');
    expect(deletions).toHaveLength(2);
    expect(deletions[0].url).toBe(`https://api.test.com/applications/${app1Id}`);
    expect(deletions[1].url).toBe(`https://api.test.com/applications/${app2Id}`);
  });

  it('should handle permission error (403 Forbidden)', async () => {
    const forbiddenError = new Error('Permission denied');
    mockDeletion(forbiddenError);

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(forbiddenError);
  });

  it('should use mutateAsync for promise-based deletion', async () => {
    mockDeletion();

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
    mockDeletion(apiError);

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    const deletePromise = result.current.mutateAsync(applicationId);

    await expect(deletePromise).rejects.toEqual(apiError);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });

  it('should not affect other cached applications on deletion', async () => {
    mockDeletion();

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
    mockDeletion();

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
    expect(requestsTo('/applications/')).toHaveLength(2);
  });

  it('should clear error state on successful retry', async () => {
    const apiError = new Error('Temporary error');
    let attempt = 0;
    mockDeletion(() => {
      attempt += 1;
      return attempt === 1 ? Promise.reject(apiError) : Promise.resolve({data: {}});
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
    expect(requestsTo('/applications/')).toHaveLength(2);
  });

  it('should handle server returning 204 No Content', async () => {
    // 204 No Content is the typical response for successful DELETE
    mockDeletion();

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
    mockDeletion(serverError);

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(serverError);
    expect(result.current.error?.message).toBe('Application has active users and cannot be deleted');
  });

  // The server revokes what the application issued, so native and flow modes both use the endpoint.
  it('deletes through the endpoint when the console says native', async () => {
    administrationOperation = AdministrationModes.NATIVE;
    mockDeletion();

    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate('550e8400-e29b-41d4-a716-446655440000');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(requestsTo('/applications/')).toHaveLength(1);
  });

  // A distributor can replace one operation without forking this package.
  it('calls the function the console supplied', async () => {
    const deleted: string[] = [];
    administrationOperation = (applicationId: string): Promise<void> => {
      deleted.push(applicationId);
      return Promise.resolve();
    };
    routeHttp({});

    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate('550e8400-e29b-41d4-a716-446655440000');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(deleted).toEqual(['550e8400-e29b-41d4-a716-446655440000']);
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});
