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

const {default: useGetCspConfig} = await import('../useGetCspConfig');

describe('useGetCspConfig', () => {
  const mockResponse: CspConfigResponse = {
    readOnly: {reportOnly: false, paths: [{location: '/console/', directives: {'img-src': ["'self'", 'data:']}}]},
    writable: {directives: {'frame-src': ["'self'"]}},
    merged: {
      reportOnly: false,
      directives: {'frame-src': ["'self'"]},
      paths: [{location: '/console/', directives: {'img-src': ["'self'", 'data:']}}],
    },
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('fetches the CSP config from /server-config/csp', async () => {
    mockHttpRequest.mockResolvedValue({data: mockResponse});
    const {result} = renderHook(() => useGetCspConfig());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockResponse);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/server-config/csp',
        method: 'GET',
      }),
    );
  });

  it('surfaces fetch errors', async () => {
    mockHttpRequest.mockRejectedValue(new Error('boom'));
    const {result} = renderHook(() => useGetCspConfig());

    await waitFor(() => {
      expect(result.current.error?.message).toBe('boom');
    });
  });
});
