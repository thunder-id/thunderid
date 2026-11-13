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

const {default: useGetServerConfigSection} = await import('../useGetServerConfigSection');
const {default: useUpdateServerConfigSection} = await import('../useUpdateServerConfigSection');

describe('useUpdateServerConfigSection', () => {
  const value: SessionValue = {idleTimeoutSeconds: 1800};
  const body: SessionConfigResponse = {
    readOnly: {},
    writable: value,
    merged: {idleTimeoutSeconds: 1800, absoluteTimeoutSeconds: 28800},
  };

  const renderUpdate = () =>
    renderHook(() => useUpdateServerConfigSection<SessionValue>('session', 'Session settings updated.'));

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('PUTs the writable value to /server-config/{name}', async () => {
    mockHttpRequest.mockResolvedValue({data: body});
    const {result} = renderUpdate();

    result.current.mutate({data: value});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/server-config/session',
        method: 'PUT',
        data: value,
      }),
    );
  });

  // PUT returns the recomputed layers, so the card shows them without a refetch.
  it('caches the returned section and confirms the save', async () => {
    mockHttpRequest.mockResolvedValue({data: body});
    const {result, queryClient} = renderUpdate();

    result.current.mutate({data: value});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(queryClient.getQueryData(['server-config', 'session'])).toEqual(body);
    expect(mockShowToast).toHaveBeenCalledWith('Session settings updated.', 'success');
  });

  // A read started before the save would otherwise land after it and put the old layers back.
  it('cancels a read of the section still in flight before caching the saved section', async () => {
    let finishRead: (response: unknown) => void = () => undefined;
    mockHttpRequest.mockImplementation(({method}: {method: string}) =>
      method === 'GET'
        ? new Promise((resolve) => {
            finishRead = resolve;
          })
        : Promise.resolve({data: body}),
    );
    const {result, queryClient} = renderHook(() => ({
      read: useGetServerConfigSection<SessionValue>('session'),
      update: useUpdateServerConfigSection<SessionValue>('session', 'Session settings updated.'),
    }));
    await waitFor(() => expect(result.current.read.isFetching).toBe(true));

    result.current.update.mutate({data: value});
    await waitFor(() => expect(result.current.update.isSuccess).toBe(true));
    finishRead({data: {...body, writable: {}}});
    // Every callback the late read could still run has had its turn.
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(queryClient.getQueryData(['server-config', 'session'])).toEqual(body);
    await waitFor(() => expect(result.current.read.data?.writable).toEqual(value));
  });

  it('surfaces a refused write without caching or toasting', async () => {
    mockHttpRequest.mockRejectedValue(
      Object.assign(new Error('Bad Request'), {response: {status: 400, data: {code: 'SCF-1003'}}}),
    );
    const {result, queryClient} = renderUpdate();

    result.current.mutate({data: value});

    await waitFor(() => {
      expect(result.current.error?.message).toBe('Bad Request');
    });

    expect(queryClient.getQueryData(['server-config', 'session'])).toBeUndefined();
    expect(mockShowToast).not.toHaveBeenCalled();
  });
});
