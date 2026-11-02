// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {AccountAccessConfigResponse} from '../../models/accountAccess';

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

const {default: useGetAccountAccessConfig} = await import('../useGetAccountAccessConfig');

describe('useGetAccountAccessConfig', () => {
  const mockResponse: AccountAccessConfigResponse = {
    readOnly: {user: {default: {enabled: false}}},
    writable: {user: {default: {enabled: true, threshold: 5}}},
    merged: {user: {default: {enabled: true, threshold: 5}}},
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('fetches the account access config from /server-config/accountAccess', async () => {
    mockHttpRequest.mockResolvedValue({data: mockResponse});
    const {result} = renderHook(() => useGetAccountAccessConfig());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockResponse);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/server-config/accountAccess',
        method: 'GET',
      }),
    );
  });

  it('surfaces fetch errors', async () => {
    mockHttpRequest.mockRejectedValue(new Error('boom'));
    const {result} = renderHook(() => useGetAccountAccessConfig());

    await waitFor(() => {
      expect(result.current.error?.message).toBe('boom');
    });
  });
});
