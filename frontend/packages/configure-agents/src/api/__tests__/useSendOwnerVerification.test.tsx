// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {FlowExecutionFailure} from '@thunderid/utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import useSendOwnerVerification from '../useSendOwnerVerification';

const HANDLE = 'default-agent-owner-verification-flow';

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
  };
});

const {useThunderID} = await import('@thunderid/react');
const {useConfig} = await import('@thunderid/contexts');

describe('useSendOwnerVerification', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockHttpRequest = vi.fn();
    vi.mocked(useThunderID).mockReturnValue({
      http: {request: mockHttpRequest},
    } as unknown as ReturnType<typeof useThunderID>);
    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => 'https://api.test.com',
    } as ReturnType<typeof useConfig>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('starts the configured verification flow for the agent and returns where the link went', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: {
        flowStatus: 'INCOMPLETE',
        data: {additionalData: {channel: 'email', emailSent: 'true', maskedDestination: 'j•••@example.com'}},
      },
    });
    const {result} = renderHook(() => useSendOwnerVerification());

    result.current.mutate({agentId: 'agent-1', handle: HANDLE});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/flow/execute',
        method: 'POST',
        data: {
          flowType: 'ADMIN_INITIATED',
          handle: HANDLE,
          inputs: {subject: 'agent-1'},
          verbose: true,
        },
      }),
    );
    expect(result.current.data?.data?.additionalData?.['maskedDestination']).toBe('j•••@example.com');
  });

  it('takes a link sent by SMS as sent', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: {flowStatus: 'INCOMPLETE', data: {additionalData: {channel: 'sms', smsSent: 'true'}}},
    });
    const {result} = renderHook(() => useSendOwnerVerification());

    result.current.mutate({agentId: 'agent-1', handle: HANDLE});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
  });

  it('reports a flow that paused without sending the link', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: {flowStatus: 'INCOMPLETE', data: {additionalData: {channel: 'email', emailSent: 'false'}}},
    });
    const {result} = renderHook(() => useSendOwnerVerification());

    result.current.mutate({agentId: 'agent-1', handle: HANDLE});

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBeInstanceOf(FlowExecutionFailure);
  });

  it('reports a flow that paused at a step other than the sent link', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: {flowStatus: 'INCOMPLETE', data: {additionalData: {}}},
    });
    const {result} = renderHook(() => useSendOwnerVerification());

    result.current.mutate({agentId: 'agent-1', handle: HANDLE});

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBeInstanceOf(FlowExecutionFailure);
  });

  it('reports a flow that refused to send the link', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: {flowStatus: 'ERROR', error: {code: 'FET-1112', message: {key: 'k', defaultValue: 'Refused'}}},
    });
    const {result} = renderHook(() => useSendOwnerVerification());

    result.current.mutate({agentId: 'agent-1', handle: HANDLE});

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBeInstanceOf(FlowExecutionFailure);
    expect((result.current.error as FlowExecutionFailure).code).toBe('FET-1112');
  });

  it('reports a request that failed', async () => {
    mockHttpRequest.mockRejectedValueOnce(new Error('Network down'));
    const {result} = renderHook(() => useSendOwnerVerification());

    result.current.mutate({agentId: 'agent-1', handle: HANDLE});

    await waitFor(() => expect(result.current.isError).toBe(true));
  });
});
