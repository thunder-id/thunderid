// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {CspConfigResponse} from '../../models/responses';

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

const {default: useUpdateCspConfig} = await import('../useUpdateCspConfig');

describe('useUpdateCspConfig', () => {
  const mockResponse: CspConfigResponse = {
    readOnly: {reportOnly: false},
    writable: {directives: {'img-src': ["'self'", 'https://cdn.example.com']}},
    merged: {reportOnly: false, directives: {'img-src': ["'self'", 'https://cdn.example.com']}},
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('PUTs the writable value to /server-config/csp', async () => {
    mockHttpRequest.mockResolvedValue({data: mockResponse});
    const {result} = renderHook(() => useUpdateCspConfig());

    result.current.mutate({data: {directives: {'img-src': ["'self'", 'https://cdn.example.com']}}});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/server-config/csp',
        method: 'PUT',
        data: {directives: {'img-src': ["'self'", 'https://cdn.example.com']}},
      }),
    );
  });

  it('writes the returned config into the query cache on success', async () => {
    mockHttpRequest.mockResolvedValue({data: mockResponse});
    const {result, queryClient} = renderHook(() => useUpdateCspConfig());

    result.current.mutate({data: {directives: {'img-src': ["'self'", 'https://cdn.example.com']}}});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(queryClient.getQueryData(['server-config', 'csp'])).toEqual(mockResponse);
  });

  it('surfaces update errors', async () => {
    mockHttpRequest.mockRejectedValue(new Error('nope'));
    const {result} = renderHook(() => useUpdateCspConfig());

    result.current.mutate({data: {}});

    await waitFor(() => {
      expect(result.current.error?.message).toBe('nope');
    });
  });
});
