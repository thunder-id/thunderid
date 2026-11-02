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

const mockShowToast = vi.fn();
const mockGetServerUrl = vi.fn<() => string>(() => 'https://localhost:8090');
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: mockGetServerUrl}),
    useToast: () => ({showToast: mockShowToast}),
  };
});

const {default: useUpdateAccountAccessConfig} = await import('../useUpdateAccountAccessConfig');

describe('useUpdateAccountAccessConfig', () => {
  const value = {user: {default: {enabled: true, threshold: 5}}};
  const mockResponse: AccountAccessConfigResponse = {
    readOnly: {user: {default: {enabled: false}}},
    writable: value,
    merged: value,
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('PUTs the writable value to /server-config/accountAccess', async () => {
    mockHttpRequest.mockResolvedValue({data: mockResponse});
    const {result} = renderHook(() => useUpdateAccountAccessConfig());

    result.current.mutate({data: value});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/server-config/accountAccess',
        method: 'PUT',
        data: value,
      }),
    );
  });

  it('writes the returned config into the query cache and toasts on success', async () => {
    mockHttpRequest.mockResolvedValue({data: mockResponse});
    const {result, queryClient} = renderHook(() => useUpdateAccountAccessConfig());

    result.current.mutate({data: value});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(queryClient.getQueryData(['server-config', 'accountAccess'])).toEqual(mockResponse);
    expect(mockShowToast).toHaveBeenCalledWith('Account security settings updated.', 'success');
  });

  it('surfaces update errors without caching or toasting', async () => {
    mockHttpRequest.mockRejectedValue(new Error('nope'));
    const {result, queryClient} = renderHook(() => useUpdateAccountAccessConfig());

    result.current.mutate({data: {}});

    await waitFor(() => {
      expect(result.current.error?.message).toBe('nope');
    });

    expect(queryClient.getQueryData(['server-config', 'accountAccess'])).toBeUndefined();
    expect(mockShowToast).not.toHaveBeenCalled();
  });
});
