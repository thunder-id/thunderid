// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import {LifecycleActionIncompleteError, type LifecycleAction} from '../lifecycleAction';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {request: mockHttpRequest},
  }),
}));

const mockShowToast = vi.fn();
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://localhost:8090'}),
    useToast: () => ({showToast: mockShowToast}),
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback?: string) => fallback ?? _key,
  }),
}));

const {default: useLifecycleAction} = await import('../useLifecycleAction');

const COMPLETE = {data: {flowStatus: 'COMPLETE'}};

describe('useLifecycleAction', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockShowToast.mockReset();
  });

  it('executes the flow with the subject and the operator note', async () => {
    mockHttpRequest.mockResolvedValue(COMPLETE);
    const {result} = renderHook(() => useLifecycleAction());

    result.current.mutate({action: 'suspend', flowIds: ['flow-1'], operatorNote: 'exposed', subject: 'u-1'});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/flow/execute',
        method: 'POST',
        data: {flowId: 'flow-1', inputs: {subject: 'u-1', operatorNote: 'exposed'}},
      }),
    );
  });

  it('runs the steps in order', async () => {
    mockHttpRequest.mockResolvedValue(COMPLETE);
    const {result} = renderHook(() => useLifecycleAction());

    result.current.mutate({action: 'unsuspendAndUnlock', flowIds: ['first', 'second'], subject: 'u-1'});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const flowIds = mockHttpRequest.mock.calls.map((call) => (call[0] as {data: {flowId: string}}).data.flowId);
    expect(flowIds).toEqual(['first', 'second']);
  });

  it.each<[LifecycleAction, string]>([
    ['suspend', 'Account suspended'],
    ['unlock', 'Account unlocked'],
    ['unsuspend', 'Suspension removed'],
    ['unsuspendAndUnlock', 'Suspension removed and account unlocked'],
  ])('toasts success for %s and calls onSettled', async (action, message) => {
    mockHttpRequest.mockResolvedValue(COMPLETE);
    const onSettled = vi.fn();
    const {result} = renderHook(() => useLifecycleAction(onSettled));

    result.current.mutate({action, flowIds: ['flow-1'], subject: 'u-1'});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith(message, 'success');
    expect(onSettled).toHaveBeenCalledWith(expect.objectContaining({action, subject: 'u-1'}));
  });

  it('fails with the original error and does not toast when the first step fails', async () => {
    mockHttpRequest.mockResolvedValue({data: {error: {message: {defaultValue: 'denied'}}, flowStatus: 'ERROR'}});
    const onSettled = vi.fn();
    const {result} = renderHook(() => useLifecycleAction(onSettled));

    result.current.mutate({action: 'suspend', flowIds: ['flow-1'], subject: 'u-1'});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).not.toBeInstanceOf(LifecycleActionIncompleteError);
    expect(result.current.error?.message).toBe('denied');
    expect(mockShowToast).not.toHaveBeenCalled();
    expect(onSettled).not.toHaveBeenCalled();
  });

  it('throws LifecycleActionIncompleteError when a later step fails', async () => {
    mockHttpRequest
      .mockResolvedValueOnce(COMPLETE)
      .mockResolvedValueOnce({data: {error: {message: {defaultValue: 'unlock failed'}}, flowStatus: 'ERROR'}});
    const {result} = renderHook(() => useLifecycleAction());

    result.current.mutate({action: 'unsuspendAndUnlock', flowIds: ['first', 'second'], subject: 'u-1'});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeInstanceOf(LifecycleActionIncompleteError);
  });
});
