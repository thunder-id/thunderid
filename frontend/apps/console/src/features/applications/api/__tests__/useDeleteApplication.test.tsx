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
    useToast: vi.fn().mockReturnValue({showToast: vi.fn()}),
  };
});

const {useThunderID} = await import('@thunderid/react');
const {useConfig} = await import('@thunderid/contexts');

describe('useDeleteApplication', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;

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
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
  });

  it('should make correct API call with application ID', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

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

  it('should set pending state during deletion', async () => {
    mockHttpRequest.mockReturnValue(
      new Promise((resolve) => {
        setTimeout(() => resolve(undefined), 100);
      }),
    );

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
    mockHttpRequest.mockRejectedValueOnce(apiError);

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

  it('should handle network error', async () => {
    const networkError = new Error('Network request failed');
    mockHttpRequest.mockRejectedValueOnce(networkError);

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
    mockHttpRequest.mockRejectedValueOnce(notFoundError);

    const applicationId = 'non-existent-id';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(notFoundError);
  });

  it('should remove application from cache on successful deletion', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

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
    mockHttpRequest.mockResolvedValueOnce(undefined);

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
    mockHttpRequest.mockResolvedValueOnce(undefined);

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
    mockHttpRequest.mockResolvedValue(undefined);

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

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        url: `https://api.test.com/applications/${app1Id}`,
      }),
    );
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        url: `https://api.test.com/applications/${app2Id}`,
      }),
    );
  });

  it('should handle permission error (403 Forbidden)', async () => {
    const forbiddenError = new Error('Permission denied');
    mockHttpRequest.mockRejectedValueOnce(forbiddenError);

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    result.current.mutate(applicationId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(forbiddenError);
  });

  it('should use mutateAsync for promise-based deletion', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

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
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const applicationId = '550e8400-e29b-41d4-a716-446655440000';
    const {result} = renderHook(() => useDeleteApplication());

    const deletePromise = result.current.mutateAsync(applicationId);

    await expect(deletePromise).rejects.toEqual(apiError);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });

  it('should not affect other cached applications on deletion', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

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
    mockHttpRequest.mockResolvedValue(undefined);

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
    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should clear error state on successful retry', async () => {
    const apiError = new Error('Temporary error');
    mockHttpRequest.mockRejectedValueOnce(apiError).mockResolvedValueOnce(undefined);

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
    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should handle server returning 204 No Content', async () => {
    // 204 No Content is the typical response for successful DELETE
    mockHttpRequest.mockResolvedValueOnce(undefined);

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
    mockHttpRequest.mockRejectedValueOnce(serverError);

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
