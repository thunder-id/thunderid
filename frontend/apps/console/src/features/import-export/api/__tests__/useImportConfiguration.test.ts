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

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import type {ImportRequest, ImportResponse} from '../../models/import-configuration';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {request: mockHttpRequest},
  }),
}));

const mockGetServerUrl = vi.fn<() => string>(() => 'https://localhost:8090');
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: mockGetServerUrl}),
  };
});

const {default: useImportConfiguration} = await import('../useImportConfiguration');

describe('useImportConfiguration', () => {
  const mockRequest: ImportRequest = {
    dryRun: true,
    resources: {application: [{name: 'test-app'}]},
    environmentVariables: {},
  } as unknown as ImportRequest;

  const mockResponse: ImportResponse = {
    summary: {total: 1, succeeded: 1, failed: 0},
    results: [],
  } as unknown as ImportResponse;

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('initializes with idle state', () => {
    const {result} = renderHook(() => useImportConfiguration());

    expect(result.current.isIdle).toBe(true);
    expect(result.current.data).toBeUndefined();
  });

  it('imports configuration successfully', async () => {
    mockHttpRequest.mockResolvedValue({data: mockResponse});

    const {result} = renderHook(() => useImportConfiguration());
    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockResponse);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/import',
        method: 'POST',
        data: mockRequest,
      }),
    );
  });

  it('sets isPending during import', async () => {
    let resolveRequest: (value: unknown) => void;
    const requestPromise = new Promise((resolve) => {
      resolveRequest = resolve;
    });
    mockHttpRequest.mockReturnValue(requestPromise);

    const {result} = renderHook(() => useImportConfiguration());
    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    resolveRequest!({data: mockResponse});

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('surfaces error on failure', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Import failed'));

    const {result} = renderHook(() => useImportConfiguration());
    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toBe('Import failed');
  });

  it('sends Content-Type application/json header', async () => {
    mockHttpRequest.mockResolvedValue({data: mockResponse});

    const {result} = renderHook(() => useImportConfiguration());
    result.current.mutate(mockRequest);

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        headers: expect.objectContaining({'Content-Type': 'application/json'}) as Record<string, string>,
      }),
    );
  });
});
