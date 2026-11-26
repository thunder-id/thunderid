// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {FlowExecutionFailure} from '@thunderid/utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import useSendUserCredentialReset from '../useSendUserCredentialReset';

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

const mockShowToast = vi.fn();

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://api.test.com'}),
    useToast: () => ({showToast: mockShowToast}),
  };
});

async function send(): Promise<ReturnType<typeof useSendUserCredentialReset>> {
  const {result} = renderHook(() => useSendUserCredentialReset());

  result.current.mutate({userId: 'user-1', credential: 'pin', label: 'PIN', handle: 'custom-reset-flow'});
  await waitFor(() => expect(result.current.isIdle).toBe(false));
  await waitFor(() => expect(result.current.isPending).toBe(false));

  return result.current;
}

describe('useSendUserCredentialReset', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockShowToast.mockReset();
  });

  it('starts the configured flow verbosely for the user and credential', async () => {
    mockHttpRequest.mockResolvedValue({
      data: {flowStatus: 'INCOMPLETE', executionId: 'exec-1', data: {additionalData: {emailSent: 'true'}}},
    });

    const result = await send();

    expect(result.isSuccess).toBe(true);
    expect(mockShowToast).toHaveBeenCalledWith(
      'Reset link sent. PIN no longer works until the user sets a new one.',
      'success',
    );
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/flow/execute',
        method: 'POST',
        data: {
          flowType: 'ADMIN_INITIATED',
          handle: 'custom-reset-flow',
          inputs: {subject: 'user-1', credential: 'pin'},
          verbose: true,
        },
      }),
    );
  });

  it.each([
    {
      name: 'a paused flow that sent nothing',
      data: {flowStatus: 'INCOMPLETE', data: {additionalData: {emailSent: 'false'}}},
    },
    {name: 'a flow that asks for more input', data: {flowStatus: 'INCOMPLETE', data: {}}},
    {name: 'a completed flow', data: {flowStatus: 'COMPLETE', data: {additionalData: {emailSent: 'true'}}}},
  ])('fails on $name', async ({data}) => {
    mockHttpRequest.mockResolvedValue({data});

    const result = await send();

    expect(result.isError).toBe(true);
    expect(result.error).toBeInstanceOf(FlowExecutionFailure);
    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('carries the code of the step that refused', async () => {
    mockHttpRequest.mockResolvedValue({
      data: {flowStatus: 'ERROR', error: {code: 'FET-1037', message: {defaultValue: 'Email recipient is required'}}},
    });

    const result = await send();

    expect(result.isError).toBe(true);
    expect((result.error as FlowExecutionFailure).code).toBe('FET-1037');
  });

  it('fails when the request fails', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Request failed with status code 500'));

    const result = await send();

    expect(result.isError).toBe(true);
    expect(result.error?.message).toBe('Request failed with status code 500');
  });
});
