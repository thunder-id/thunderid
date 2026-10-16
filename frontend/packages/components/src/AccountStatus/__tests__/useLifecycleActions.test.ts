// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {request: mockHttpRequest},
  }),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://localhost:8090'}),
  };
});

const {default: useLifecycleActions, LIFECYCLE_FLOWS_QUERY_KEY} = await import('../useLifecycleActions');

/**
 * Serves the flow section and the administration flow listing.
 */
function serve(section: Record<string, {defaultHandle: string}>, flows: {id: string; handle: string}[]): void {
  mockHttpRequest.mockImplementation(({url}: {url: string}) =>
    Promise.resolve(
      url.endsWith('/server-config/flow')
        ? {data: {merged: section}}
        : {data: {flows: flows.map((flow) => ({...flow, flowType: 'ADMINISTRATION'}))}},
    ),
  );
}

describe('useLifecycleActions', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it('is pending until the flows resolve, then offers every configured user operation', async () => {
    serve(
      {
        userSuspendFlow: {defaultHandle: 'suspend-h'},
        userUnlockFlow: {defaultHandle: 'unlock-h'},
        userUnsuspendFlow: {defaultHandle: 'unsuspend-h'},
      },
      [
        {id: 'f1', handle: 'suspend-h'},
        {id: 'f2', handle: 'unlock-h'},
        {id: 'f3', handle: 'unsuspend-h'},
      ],
    );
    const {result} = renderHook(() => useLifecycleActions('user'));

    expect(result.current.isPending).toBe(true);
    expect(result.current.available).toEqual({});

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });

    expect(result.current.available).toEqual({suspend: 'f1', unlock: 'f2', unsuspend: 'f3'});
  });

  it('offers only the operations of the requested category', async () => {
    serve(
      {
        applicationSuspendFlow: {defaultHandle: 'app-suspend'},
        applicationUnsuspendFlow: {defaultHandle: 'app-unsuspend'},
        userUnlockFlow: {defaultHandle: 'unlock-h'},
      },
      [
        {id: 'a1', handle: 'app-suspend'},
        {id: 'a2', handle: 'app-unsuspend'},
        {id: 'u1', handle: 'unlock-h'},
      ],
    );
    const {result} = renderHook(() => useLifecycleActions('application'));

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });

    expect(result.current.available).toEqual({suspend: 'a1', unsuspend: 'a2'});
  });

  it('omits an operation whose handle names no existing flow', async () => {
    serve({agentSuspendFlow: {defaultHandle: 'gone'}, agentUnlockFlow: {defaultHandle: 'unlock-h'}}, [
      {id: 'g1', handle: 'unlock-h'},
    ]);
    const {result} = renderHook(() => useLifecycleActions('agent'));

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });

    expect(result.current.available).toEqual({unlock: 'g1'});
  });

  it('offers nothing when resolution fails', async () => {
    mockHttpRequest.mockRejectedValue(new Error('boom'));
    const {result, queryClient} = renderHook(() => useLifecycleActions('user'));

    await waitFor(() => {
      expect(queryClient.getQueryState(LIFECYCLE_FLOWS_QUERY_KEY)?.status).toBe('error');
    });

    expect(result.current.available).toEqual({});
  });

  it('does not cache the resolution for the whole session', async () => {
    // A changed flow section must show on the next mount, so the entry has to go stale.
    serve({userSuspendFlow: {defaultHandle: 'suspend-h'}}, [{id: 'f1', handle: 'suspend-h'}]);
    const {result, queryClient} = renderHook(() => useLifecycleActions('user'));

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });

    expect(queryClient.getQueryCache().find({queryKey: LIFECYCLE_FLOWS_QUERY_KEY})?.isStale()).toBe(true);
  });
});
