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

    const {result} = renderHook(() => useGetAccountLifecycleFlows());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.flows).toEqual([listed('suspend'), listed('unlock')]);
    expect([...(result.current.data?.unconfigured ?? [])].sort()).toEqual(
      [
        'agentUnlockFlow',
        'agentUnsuspendFlow',
        'applicationSuspendFlow',
        'applicationUnsuspendFlow',
        'userUnsuspendFlow',
      ].sort(),
    );
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://localhost:8090/server-config/flow', method: 'GET'}),
    );
  });

  it('surfaces a failed read', async () => {
    mockHttpRequest.mockRejectedValue(new Error('boom'));

    const {result} = renderHook(() => useGetAccountLifecycleFlows());

    await waitFor(() => {
      expect(result.current.error?.message).toBe('boom');
    });
  });
});
