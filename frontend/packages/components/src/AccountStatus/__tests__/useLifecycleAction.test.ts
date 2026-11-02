// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import {LifecycleActionIncompleteError, type LifecycleAction, type LifecycleCategory} from '../lifecycleAction';

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
const LIST_KEY = ['users'];

describe('useLifecycleAction', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockShowToast.mockReset();
  });

  it('executes the flow with the subject and the operator note', async () => {
    mockHttpRequest.mockResolvedValue(COMPLETE);
    const {result} = renderHook(() => useLifecycleAction(LIST_KEY));

    result.current.mutate({
      action: 'suspend',
      category: 'user',
      flowIds: ['flow-1'],
      operatorNote: 'exposed',
      subject: 'u-1',
    });

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
    const {result} = renderHook(() => useLifecycleAction(LIST_KEY));

    result.current.mutate({
      action: 'unsuspendAndUnlock',
      category: 'user',
      flowIds: ['first', 'second'],
      subject: 'u-1',
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const flowIds = mockHttpRequest.mock.calls.map((call) => (call[0] as {data: {flowId: string}}).data.flowId);
    expect(flowIds).toEqual(['first', 'second']);
  });

  it.each<[LifecycleAction, LifecycleCategory, string]>([
    ['suspend', 'user', 'User suspended'],
    ['suspend', 'agent', 'Agent suspended'],
    ['suspend', 'application', 'Application suspended'],
    ['unlock', 'user', 'User unlocked'],
    ['unlock', 'agent', 'Agent unlocked'],
    ['unsuspend', 'user', 'User unsuspended'],
    ['unsuspend', 'agent', 'Agent unsuspended'],
    ['unsuspend', 'application', 'Application unsuspended'],
    ['unsuspendAndUnlock', 'user', 'User unsuspended and unlocked'],
  ])('toasts success for %s naming the %s and calls onSuccess', async (action, category, message) => {
    mockHttpRequest.mockResolvedValue(COMPLETE);
    const onSuccess = vi.fn();
    const {result} = renderHook(() => useLifecycleAction(LIST_KEY, onSuccess));

    result.current.mutate({action, category, flowIds: ['flow-1'], subject: 'u-1'});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith(message, 'success');
    expect(onSuccess).toHaveBeenCalledWith(expect.objectContaining({action, subject: 'u-1'}));
  });

  it.each<[LifecycleAction, string]>([
    ['suspend', 'User suspended, but no notification email was sent. Check that the user has'],
    ['unlock', 'User unlocked, but no notification email was sent. Check that the user has'],
    ['unsuspend', 'User unsuspended, but no notification email was sent. Check that the user has'],
  ])('toasts a warning for %s naming the user when its email was not sent', async (action, message) => {
    mockHttpRequest.mockResolvedValue({data: {data: {additionalData: {emailSent: 'false'}}, flowStatus: 'COMPLETE'}});
    const onSuccess = vi.fn();
    const {result} = renderHook(() => useLifecycleAction(LIST_KEY, onSuccess));

    result.current.mutate({action, category: 'user', flowIds: ['flow-1'], subject: 'u-1'});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledTimes(1);
    expect(mockShowToast).toHaveBeenCalledWith(expect.stringContaining(message), 'warning');
    expect(onSuccess).toHaveBeenCalledWith(expect.objectContaining({action, subject: 'u-1'}));
  });

  it.each<[LifecycleAction, LifecycleCategory, string]>([
    ['suspend', 'agent', 'Agent suspended'],
    ['suspend', 'application', 'Application suspended'],
    ['unlock', 'agent', 'Agent unlocked'],
    ['unsuspend', 'agent', 'Agent unsuspended'],
    ['unsuspend', 'application', 'Application unsuspended'],
  ])('toasts success for %s naming the %s when no email was sent', async (action, category, message) => {
    mockHttpRequest.mockResolvedValue({data: {data: {additionalData: {emailSent: 'false'}}, flowStatus: 'COMPLETE'}});
    const {result} = renderHook(() => useLifecycleAction(LIST_KEY));

    result.current.mutate({action, category, flowIds: ['flow-1'], subject: 'u-1'});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledTimes(1);
    expect(mockShowToast).toHaveBeenCalledWith(message, 'success');
  });

  it('toasts a warning when any step of a two-step action sent no email', async () => {
    mockHttpRequest
      .mockResolvedValueOnce({data: {data: {additionalData: {emailSent: 'false'}}, flowStatus: 'COMPLETE'}})
      .mockResolvedValueOnce({data: {data: {additionalData: {emailSent: 'true'}}, flowStatus: 'COMPLETE'}});
    const {result} = renderHook(() => useLifecycleAction(LIST_KEY));

    result.current.mutate({
      action: 'unsuspendAndUnlock',
      category: 'user',
      flowIds: ['first', 'second'],
      subject: 'u-1',
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith(
      expect.stringContaining(
        'User unsuspended and unlocked, but a notification email was not sent. Check that the user has',
      ),
      'warning',
    );
  });

  it('toasts success when the email was sent', async () => {
    mockHttpRequest.mockResolvedValue({data: {data: {additionalData: {emailSent: 'true'}}, flowStatus: 'COMPLETE'}});
    const {result} = renderHook(() => useLifecycleAction(LIST_KEY));

    result.current.mutate({action: 'suspend', category: 'user', flowIds: ['flow-1'], subject: 'u-1'});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith('User suspended', 'success');
  });

  it('fails with the original error and does not toast when the first step fails', async () => {
    mockHttpRequest.mockResolvedValue({data: {error: {message: {defaultValue: 'denied'}}, flowStatus: 'ERROR'}});
    const onSuccess = vi.fn();
    const {result} = renderHook(() => useLifecycleAction(LIST_KEY, onSuccess));

    result.current.mutate({action: 'suspend', category: 'user', flowIds: ['flow-1'], subject: 'u-1'});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).not.toBeInstanceOf(LifecycleActionIncompleteError);
    expect(result.current.error?.message).toBe('denied');
    expect(mockShowToast).not.toHaveBeenCalled();
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('toasts success when the flow reports no email node at all', async () => {
    mockHttpRequest.mockResolvedValue({data: {data: {additionalData: {}}, flowStatus: 'COMPLETE'}});
    const {result} = renderHook(() => useLifecycleAction(LIST_KEY));

    result.current.mutate({action: 'unlock', category: 'user', flowIds: ['flow-1'], subject: 'u-1'});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith('User unlocked', 'success');
  });

  it('fails when the flow asks for more input', async () => {
    mockHttpRequest.mockResolvedValue({data: {flowStatus: 'INCOMPLETE'}});
    const onSuccess = vi.fn();
    const {result} = renderHook(() => useLifecycleAction(LIST_KEY, onSuccess));

    result.current.mutate({action: 'suspend', category: 'user', flowIds: ['flow-1'], subject: 'u-1'});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).not.toBeInstanceOf(LifecycleActionIncompleteError);
    expect(onSuccess).not.toHaveBeenCalled();
    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('passes an HTTP rejection through unchanged', async () => {
    const rejection = new Error('network down');
    mockHttpRequest.mockRejectedValue(rejection);
    const {result} = renderHook(() => useLifecycleAction(LIST_KEY));

    result.current.mutate({action: 'suspend', category: 'user', flowIds: ['flow-1'], subject: 'u-1'});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBe(rejection);
    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('throws LifecycleActionIncompleteError when a later step fails', async () => {
    mockHttpRequest
      .mockResolvedValueOnce(COMPLETE)
      .mockResolvedValueOnce({data: {error: {message: {defaultValue: 'unlock failed'}}, flowStatus: 'ERROR'}});
    const {result} = renderHook(() => useLifecycleAction(LIST_KEY));

    result.current.mutate({
      action: 'unsuspendAndUnlock',
      category: 'user',
      flowIds: ['first', 'second'],
      subject: 'u-1',
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeInstanceOf(LifecycleActionIncompleteError);
    expect((result.current.error?.cause as Error).message).toBe('unlock failed');
  });

  it('wraps an HTTP rejection on a later step too', async () => {
    mockHttpRequest.mockResolvedValueOnce(COMPLETE).mockRejectedValueOnce(new Error('network down'));
    const {result} = renderHook(() => useLifecycleAction(LIST_KEY));

    result.current.mutate({
      action: 'unsuspendAndUnlock',
      category: 'user',
      flowIds: ['first', 'second'],
      subject: 'u-1',
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeInstanceOf(LifecycleActionIncompleteError);
  });

  describe('list cache', () => {
    it('invalidates the list query after a successful run', async () => {
      mockHttpRequest.mockResolvedValue(COMPLETE);
      const {result, queryClient} = renderHook(() => useLifecycleAction(LIST_KEY));
      const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

      result.current.mutate({action: 'suspend', category: 'user', flowIds: ['flow-1'], subject: 'u-1'});

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });
      expect(invalidate).toHaveBeenCalledWith({queryKey: LIST_KEY});
    });

    it('invalidates the list query when a later step fails, since the record changed', async () => {
      mockHttpRequest.mockResolvedValueOnce(COMPLETE).mockRejectedValueOnce(new Error('network down'));
      const {result, queryClient} = renderHook(() => useLifecycleAction(LIST_KEY));
      const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

      result.current.mutate({
        action: 'unsuspendAndUnlock',
        category: 'user',
        flowIds: ['first', 'second'],
        subject: 'u-1',
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });
      expect(invalidate).toHaveBeenCalledWith({queryKey: LIST_KEY});
    });

    it('leaves the list query alone when the first step fails', async () => {
      mockHttpRequest.mockRejectedValue(new Error('network down'));
      const {result, queryClient} = renderHook(() => useLifecycleAction(LIST_KEY));
      const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

      result.current.mutate({action: 'suspend', category: 'user', flowIds: ['flow-1'], subject: 'u-1'});

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });
      expect(invalidate).not.toHaveBeenCalled();
    });
  });
});
