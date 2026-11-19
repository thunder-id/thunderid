// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {SessionConfigResponse, SessionValue} from '../../models/session';

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

const {default: useGetServerConfigSection} = await import('../useGetServerConfigSection');

describe('useGetServerConfigSection', () => {
  const body: SessionConfigResponse = {
    readOnly: {},
    writable: {idleTimeoutSeconds: 1800},
    merged: {idleTimeoutSeconds: 1800, absoluteTimeoutSeconds: 28800},
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('fetches the section from /server-config/{name}', async () => {
    mockHttpRequest.mockResolvedValue({data: body});
    const {result} = renderHook(() => useGetServerConfigSection<SessionValue>('session'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/server-config/session',
        method: 'GET',
      }),
    );
    expect(result.current.data).toEqual(body);
  });

  it('surfaces fetch errors', async () => {
    mockHttpRequest.mockRejectedValue(new Error('boom'));
    const {result} = renderHook(() => useGetServerConfigSection<SessionValue>('session'));

    await waitFor(() => {
      expect(result.current.error?.message).toBe('boom');
    });
  });
});
