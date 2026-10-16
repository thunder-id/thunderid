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

/** Answers the flow section read and the administration flow listing. */
const serve = (section: Record<string, {defaultHandle: string}>, flows: {id: string; handle: string}[]): void => {
  mockHttpRequest.mockImplementation(({url}: {url: string}) => {
    if (url.endsWith('/server-config/flow')) {
      return Promise.resolve({data: {merged: section}});
    }
    return Promise.resolve({data: {flows: flows.map((flow) => ({...flow, flowType: 'ADMINISTRATION'}))}});
  });
};

describe('useGetAccountLifecycleFlows', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it('resolves every lifecycle purpose to the id of the flow its handle names', async () => {
    const purposes = [
      'userSuspendFlow',
      'userUnsuspendFlow',
      'userUnlockFlow',
      'agentSuspendFlow',
      'agentUnsuspendFlow',
      'agentUnlockFlow',
      'applicationSuspendFlow',
      'applicationUnsuspendFlow',
    ];
    serve(
      {
        ...Object.fromEntries(purposes.map((purpose) => [purpose, {defaultHandle: `${purpose}-handle`}])),
        userRegistrationFlow: {defaultHandle: 'not-lifecycle'},
      },
      // Every handle exists except the application unsuspend one.
      [...purposes.slice(0, -1), 'userRegistrationFlow'].map((purpose) => ({
        id: `${purpose}-id`,
        handle: `${purpose}-handle`,
      })),
    );

    const {result} = renderHook(() => useGetAccountLifecycleFlows());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toEqual(
      Object.fromEntries(purposes.slice(0, -1).map((purpose) => [purpose, `${purpose}-id`])),
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
