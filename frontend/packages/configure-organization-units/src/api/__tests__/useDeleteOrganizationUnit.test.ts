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
import useDeleteOrganizationUnit from '../useDeleteOrganizationUnit';

// Mock useThunderID
const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {
      request: mockHttpRequest,
    },
  }),
}));

// Mock useConfig
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      getServerUrl: () => 'https://localhost:8090',
    }),
  };
});

describe('useDeleteOrganizationUnit', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should be idle initially', () => {
    const {result} = renderHook(() => useDeleteOrganizationUnit());

    expect(result.current.isPending).toBe(false);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
  });

  it('should delete organization unit successfully', async () => {
    mockHttpRequest.mockResolvedValue({});

    const {result} = renderHook(() => useDeleteOrganizationUnit());

    result.current.mutate('ou-123');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/organization-units/ou-123',
        method: 'DELETE',
      }),
    );
  });

  it('should set pending state during mutation', async () => {
    let resolvePromise: (value: object) => void;
    mockHttpRequest.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolvePromise = resolve;
        }),
    );

    const {result} = renderHook(() => useDeleteOrganizationUnit());

    result.current.mutate('ou-123');

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    // Resolve to clean up
    resolvePromise!({});

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });
  });

  it('should handle API error', async () => {
    const errorMessage = 'Failed to delete organization unit';
    mockHttpRequest.mockRejectedValue(new Error(errorMessage));

    const {result} = renderHook(() => useDeleteOrganizationUnit());

    result.current.mutate('ou-123');

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
      expect(result.current.error?.message).toBe(errorMessage);
    });
  });

  it('should call onSuccess callback when provided', async () => {
    mockHttpRequest.mockResolvedValue({});
    const onSuccess = vi.fn();

    const {result} = renderHook(() => useDeleteOrganizationUnit());

    result.current.mutate('ou-123', {onSuccess});

    await waitFor(() => {
      expect(onSuccess).toHaveBeenCalled();
    });
  });

  it('should call onError callback when provided', async () => {
    const error = new Error('Delete failed');
    mockHttpRequest.mockRejectedValue(error);
    const onError = vi.fn();

    const {result} = renderHook(() => useDeleteOrganizationUnit());

    result.current.mutate('ou-123', {onError});

    await waitFor(() => {
      expect(onError).toHaveBeenCalled();
    });
  });

  it('should delete different organization units', async () => {
    mockHttpRequest.mockResolvedValue({});

    const {result} = renderHook(() => useDeleteOrganizationUnit());

    result.current.mutate('ou-456');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/organization-units/ou-456',
      }),
    );
  });

  it('should use mutateAsync for promise-based mutation', async () => {
    mockHttpRequest.mockResolvedValue({});

    const {result} = renderHook(() => useDeleteOrganizationUnit());

    await result.current.mutateAsync('ou-123');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('should handle not found error', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Organization unit not found'));

    const {result} = renderHook(() => useDeleteOrganizationUnit());

    result.current.mutate('non-existent-ou');

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
      expect(result.current.error?.message).toBe('Organization unit not found');
    });
  });
});
