// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', async (importOriginal) => {
  // The real module, since @thunderid/components reads more of it than this test needs.
  const actual = await importOriginal<Record<string, unknown>>();
  return {
    ...actual,
    useThunderID: () => ({
      http: {request: mockHttpRequest},
    }),
  };
});

const mockGetServerUrl = vi.fn<() => string>(() => 'https://localhost:8090');
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: mockGetServerUrl}),
  };
});

const {default: useGetAccountLifecycleFlows} = await import('../useGetAccountLifecycleFlows');

/** A listed administration flow, as `GET /flows` returns it. */
const listed = (handle: string) => ({
  id: `${handle}-id`,
  handle,
  name: `${handle} name`,
  flowType: 'ADMINISTRATION',
  updatedAt: '2026-10-01T00:00:00Z',
  isReadOnly: false,
});

/** Answers the flow section read and the administration flow listing. */
const serve = (section: Record<string, {defaultHandle: string}>, flows: ReturnType<typeof listed>[]): void => {
  mockHttpRequest.mockImplementation(({url}: {url: string}) => {
    if (url.endsWith('/server-config/flow')) {
      return Promise.resolve({data: {merged: section}});
    }
    return Promise.resolve({data: {flows}});
  });
};

/** Every lifecycle purpose, as the settings card passes them when each runs as a flow. */
const ALL_PURPOSES = [
  'agentSuspendFlow',
  'agentUnlockFlow',
  'agentUnsuspendFlow',
  'applicationSuspendFlow',
  'applicationUnsuspendFlow',
  'userSuspendFlow',
  'userUnlockFlow',
  'userUnsuspendFlow',
] as const;

describe('useGetAccountLifecycleFlows', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it('lists each flow the lifecycle purposes name once, and the purposes no flow serves', async () => {
    serve(
      {
        userSuspendFlow: {defaultHandle: 'suspend'},
        agentSuspendFlow: {defaultHandle: 'suspend'},
        userUnlockFlow: {defaultHandle: 'unlock'},
        agentUnlockFlow: {defaultHandle: 'deleted'},
        userRegistrationFlow: {defaultHandle: 'registration'},
      },
      [listed('suspend'), listed('unlock'), listed('registration')],
    );

    const {result} = renderHook(() => useGetAccountLifecycleFlows(ALL_PURPOSES));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.flows).toEqual([listed('suspend'), listed('unlock')]);
    expect(result.current.data?.unconfigured).toEqual([
      'agentUnlockFlow',
      'agentUnsuspendFlow',
      'applicationSuspendFlow',
      'applicationUnsuspendFlow',
      'userUnsuspendFlow',
    ]);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://localhost:8090/server-config/flow', method: 'GET'}),
    );
  });

  // The record pages offer only the operations that run as flows, so the card lists only those.
  it('lists only the given purposes', async () => {
    serve({userSuspendFlow: {defaultHandle: 'suspend'}, agentSuspendFlow: {defaultHandle: 'agent-suspend'}}, [
      listed('suspend'),
      listed('agent-suspend'),
    ]);

    const {result} = renderHook(() => useGetAccountLifecycleFlows(['userSuspendFlow', 'userUnlockFlow']));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toEqual({flows: [listed('suspend')], unconfigured: ['userUnlockFlow']});
  });

  it('reads nothing when no purpose is given', async () => {
    const {result} = renderHook(() => useGetAccountLifecycleFlows([]));

    await waitFor(() => {
      expect(result.current.fetchStatus).toBe('idle');
    });
    expect(result.current.data).toBeUndefined();
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  // One resolution serves this card and the record pages' actions, so neither reads the flows twice.
  it('reads the resolution the record pages share', async () => {
    serve({userSuspendFlow: {defaultHandle: 'suspend'}}, [listed('suspend')]);

    const {result, queryClient} = renderHook(() => useGetAccountLifecycleFlows(ALL_PURPOSES));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    const queries = queryClient.getQueryCache().getAll();
    expect(queries).toHaveLength(1);
    // The key useAvailableLifecycleActions reads in @thunderid/components.
    expect(queries[0].queryKey).toEqual(['lifecycle-flows']);
    expect(queries[0].state.data).toEqual({userSuspendFlow: listed('suspend')});
  });

  it('surfaces a failed read', async () => {
    mockHttpRequest.mockRejectedValue(new Error('boom'));

    const {result} = renderHook(() => useGetAccountLifecycleFlows(ALL_PURPOSES));

    await waitFor(() => {
      expect(result.current.error?.message).toBe('boom');
    });
  });
});
