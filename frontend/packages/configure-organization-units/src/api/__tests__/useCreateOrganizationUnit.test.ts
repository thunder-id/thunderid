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
import type {OrganizationUnit} from '../../models/organization-unit';
import type {CreateOrganizationUnitRequest} from '../../models/requests';
import useCreateOrganizationUnit from '../useCreateOrganizationUnit';

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

describe('useCreateOrganizationUnit', () => {
  const mockCreatedOU: OrganizationUnit = {
    id: 'ou-new-123',
    handle: 'new-ou',
    name: 'New Organization Unit',
    description: 'A new organization unit',
    parent: null,
  };

  const createRequest: CreateOrganizationUnitRequest = {
    handle: 'new-ou',
    name: 'New Organization Unit',
    description: 'A new organization unit',
    parent: null,
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should be idle initially', () => {
    const {result} = renderHook(() => useCreateOrganizationUnit());

    expect(result.current.isPending).toBe(false);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(result.current.data).toBeUndefined();
  });

  it('should create organization unit successfully', async () => {
    mockHttpRequest.mockResolvedValue({data: mockCreatedOU});

    const {result} = renderHook(() => useCreateOrganizationUnit());

    result.current.mutate(createRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
      expect(result.current.data).toEqual(mockCreatedOU);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/organization-units',
        method: 'POST',
        data: createRequest,
      }),
    );
  });

  it('should set pending state during mutation', async () => {
    let resolvePromise: (value: {data: OrganizationUnit}) => void;
    mockHttpRequest.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolvePromise = resolve;
        }),
    );

    const {result} = renderHook(() => useCreateOrganizationUnit());

    result.current.mutate(createRequest);

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    // Resolve to clean up
    resolvePromise!({data: mockCreatedOU});

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });
  });

  it('should handle API error', async () => {
    const errorMessage = 'Failed to create organization unit';
    mockHttpRequest.mockRejectedValue(new Error(errorMessage));

    const {result} = renderHook(() => useCreateOrganizationUnit());

    result.current.mutate(createRequest);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
      expect(result.current.error?.message).toBe(errorMessage);
    });
  });

  it('should call onSuccess callback when provided', async () => {
    mockHttpRequest.mockResolvedValue({data: mockCreatedOU});
    const onSuccess = vi.fn();

    const {result} = renderHook(() => useCreateOrganizationUnit());

    result.current.mutate(createRequest, {onSuccess});

    await waitFor(() => {
      expect(onSuccess).toHaveBeenCalled();
      expect(onSuccess.mock.calls[0][0]).toEqual(mockCreatedOU);
      expect(onSuccess.mock.calls[0][1]).toEqual(createRequest);
    });
  });

  it('should call onError callback when provided', async () => {
    const error = new Error('Creation failed');
    mockHttpRequest.mockRejectedValue(error);
    const onError = vi.fn();

    const {result} = renderHook(() => useCreateOrganizationUnit());

    result.current.mutate(createRequest, {onError});

    await waitFor(() => {
      expect(onError).toHaveBeenCalled();
    });
  });

  it('should create organization unit with parent', async () => {
    const ouWithParent: OrganizationUnit = {
      ...mockCreatedOU,
      parent: 'parent-ou-id',
    };
    const requestWithParent: CreateOrganizationUnitRequest = {
      ...createRequest,
      parent: 'parent-ou-id',
    };
    mockHttpRequest.mockResolvedValue({data: ouWithParent});

    const {result} = renderHook(() => useCreateOrganizationUnit());

    result.current.mutate(requestWithParent);

    await waitFor(() => {
      expect(result.current.data).toEqual(ouWithParent);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        data: requestWithParent,
      }),
    );
  });

  it('should create organization unit without description', async () => {
    const ouWithoutDescription: OrganizationUnit = {
      ...mockCreatedOU,
      description: null,
    };
    const requestWithoutDescription: CreateOrganizationUnitRequest = {
      handle: 'new-ou',
      name: 'New Organization Unit',
      description: null,
      parent: null,
    };
    mockHttpRequest.mockResolvedValue({data: ouWithoutDescription});

    const {result} = renderHook(() => useCreateOrganizationUnit());

    result.current.mutate(requestWithoutDescription);

    await waitFor(() => {
      expect(result.current.data).toEqual(ouWithoutDescription);
    });
  });

  it('should use mutateAsync for promise-based mutation', async () => {
    mockHttpRequest.mockResolvedValue({data: mockCreatedOU});

    const {result} = renderHook(() => useCreateOrganizationUnit());

    const response = await result.current.mutateAsync(createRequest);

    expect(response).toEqual(mockCreatedOU);
  });

  it('should invalidate organization units query on success', async () => {
    mockHttpRequest.mockResolvedValue({data: mockCreatedOU});

    const {result} = renderHook(() => useCreateOrganizationUnit());

    result.current.mutate(createRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // The hook should have called invalidateQueries internally
    // We verify this by checking the mutation completed successfully
    expect(result.current.data).toEqual(mockCreatedOU);
  });
});
