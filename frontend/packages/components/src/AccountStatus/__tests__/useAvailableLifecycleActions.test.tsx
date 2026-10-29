// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, waitFor} from '@testing-library/react';
import {AdministrationModes, AdministrationProvider, type AdministrationConfig} from '@thunderid/contexts';
import {render, renderHook} from '@thunderid/test-utils';
import type {JSX} from 'react';
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

const {
  default: useAvailableLifecycleActions,
  getLifecycleFlowPurposes,
  LIFECYCLE_FLOWS_QUERY_KEY,
} = await import('../useAvailableLifecycleActions');

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

describe('getLifecycleFlowPurposes', () => {
  it('pairs each operation of the category with the purpose naming its flow', () => {
    expect(getLifecycleFlowPurposes({}, 'application')).toEqual([
      ['suspend', 'applicationSuspendFlow'],
      ['unsuspend', 'applicationUnsuspendFlow'],
    ]);
  });

  it('keeps only the operations that run as flows', () => {
    const administration: AdministrationConfig = {
      features: {users: {operations: {unlock: AdministrationModes.FLOW}}},
      mode: AdministrationModes.NATIVE,
    };

    expect(getLifecycleFlowPurposes(administration, 'user')).toEqual([['unlock', 'userUnlockFlow']]);
    expect(getLifecycleFlowPurposes(administration, 'agent')).toEqual([]);
  });
});

describe('useAvailableLifecycleActions', () => {
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
    const {result} = renderHook(() => useAvailableLifecycleActions('user'));

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
    const {result} = renderHook(() => useAvailableLifecycleActions('application'));

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });

    expect(result.current.available).toEqual({suspend: 'a1', unsuspend: 'a2'});
  });

  it('omits an operation whose handle names no existing flow', async () => {
    serve({agentSuspendFlow: {defaultHandle: 'gone'}, agentUnlockFlow: {defaultHandle: 'unlock-h'}}, [
      {id: 'g1', handle: 'unlock-h'},
    ]);
    const {result} = renderHook(() => useAvailableLifecycleActions('agent'));

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });

    expect(result.current.available).toEqual({unlock: 'g1'});
  });

  it('offers nothing and reports the error when resolution fails', async () => {
    const failure = new Error('boom');
    mockHttpRequest.mockRejectedValue(failure);
    const {result} = renderHook(() => useAvailableLifecycleActions('user'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBe(failure);
    expect(result.current.isPending).toBe(false);
    expect(result.current.available).toEqual({});
  });

  it('keeps the resolution fresh for five minutes, not for the whole session', async () => {
    serve({userSuspendFlow: {defaultHandle: 'suspend-h'}}, [{id: 'f1', handle: 'suspend-h'}]);
    const {result, queryClient} = renderHook(() => useAvailableLifecycleActions('user'));

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });

    const query = queryClient.getQueryCache().find({queryKey: LIFECYCLE_FLOWS_QUERY_KEY});
    expect(query?.isStale()).toBe(false);
    expect((query?.options as {staleTime?: number}).staleTime).toBe(5 * 60 * 1000);
  });

  it('returns the same availability across renders until the resolution changes', async () => {
    serve({userSuspendFlow: {defaultHandle: 'suspend-h'}}, [{id: 'f1', handle: 'suspend-h'}]);
    const {result, rerender} = renderHook(() => useAvailableLifecycleActions('user'));

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });
    const first = result.current.available;
    rerender();

    expect(result.current.available).toBe(first);
  });

  describe('administration mode', () => {
    const ALL_USER_FLOWS = {
      userSuspendFlow: {defaultHandle: 'suspend-h'},
      userUnlockFlow: {defaultHandle: 'unlock-h'},
      userUnsuspendFlow: {defaultHandle: 'unsuspend-h'},
    };
    const USER_FLOWS = [
      {id: 'f1', handle: 'suspend-h'},
      {id: 'f2', handle: 'unlock-h'},
      {id: 'f3', handle: 'unsuspend-h'},
    ];

    function Probe(): JSX.Element {
      const {available, isPending} = useAvailableLifecycleActions('user');
      return <output>{isPending ? 'pending' : JSON.stringify(available)}</output>;
    }

    const renderUnder = (administration: AdministrationConfig) =>
      render(
        <AdministrationProvider administration={administration}>
          <Probe />
        </AdministrationProvider>,
      );

    it('offers nothing and resolves no flows in native mode', async () => {
      serve(ALL_USER_FLOWS, USER_FLOWS);
      renderUnder({mode: AdministrationModes.NATIVE});

      await waitFor(() => {
        expect(screen.getByRole('status')).toHaveTextContent('{}');
      });
      expect(mockHttpRequest).not.toHaveBeenCalled();
    });

    it('offers only the operations configured to run as flows', async () => {
      serve(ALL_USER_FLOWS, USER_FLOWS);
      renderUnder({
        features: {users: {operations: {suspend: AdministrationModes.FLOW}}},
        mode: AdministrationModes.NATIVE,
      });

      await waitFor(() => {
        expect(screen.getByRole('status')).toHaveTextContent(JSON.stringify({suspend: 'f1'}));
      });
    });
  });
});
