// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import useSendUserAttributeVerification from '../useSendUserAttributeVerification';

const mockHttpRequest = vi.fn();
const mockShowToast = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://api.test.com'}),
    useToast: () => ({showToast: mockShowToast}),
  };
});

const linkSent = (additionalData: Record<string, string>) => ({
  data: {flowStatus: 'INCOMPLETE', executionId: 'exec-1', data: {additionalData}},
});

async function send(): Promise<ReturnType<typeof useSendUserAttributeVerification>> {
  const {result} = renderHook(() => useSendUserAttributeVerification());

  result.current.mutate({userId: 'user-1', attribute: 'email', handle: 'custom-verification-flow'});
  await waitFor(() => expect(result.current.isIdle).toBe(false));
  await waitFor(() => expect(result.current.isPending).toBe(false));

  return result.current;
}

describe('useSendUserAttributeVerification', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockShowToast.mockReset();
  });

  it('starts the configured administrator-initiated flow for the user and attribute', async () => {
    mockHttpRequest.mockResolvedValue(linkSent({channel: 'email', emailSent: 'true'}));

    await send();

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/flow/execute',
        method: 'POST',
        data: {
          flowType: 'ADMIN_INITIATED',
          handle: 'custom-verification-flow',
          inputs: {subject: 'user-1', attribute: 'email'},
          verbose: true,
        },
      }),
    );
  });

  it('reports the masked destination once the link is sent', async () => {
    mockHttpRequest.mockResolvedValue(
      linkSent({channel: 'email', emailSent: 'true', maskedDestination: 'j***@example.com'}),
    );

    const result = await send();

    expect(result.isSuccess).toBe(true);
    expect(result.data).toEqual({channel: 'email', maskedDestination: 'j***@example.com'});
    expect(mockShowToast).toHaveBeenCalledWith('Verification link sent to j***@example.com.', 'success');
  });

  it('reports a generic success when the flow gives no destination', async () => {
    mockHttpRequest.mockResolvedValue(linkSent({channel: 'email', emailSent: 'true'}));

    await send();

    expect(mockShowToast).toHaveBeenCalledWith('Verification link sent.', 'success');
  });

  it.each([
    {name: 'says the email was not sent', additionalData: {channel: 'email', emailSent: 'false'}},
    {name: 'pauses before an email node reports a send', additionalData: {channel: 'email'}},
  ])('fails when the flow $name', async ({additionalData}) => {
    mockHttpRequest.mockResolvedValue(linkSent(additionalData));

    const result = await send();

    expect(result.isError).toBe(true);
    expect(result.error?.message).toBe('The verification link was not sent');
  });

  it('fails with the flow error when the user is suspended', async () => {
    mockHttpRequest.mockResolvedValue({
      data: {
        flowStatus: 'ERROR',
        error: {code: 'FET-1114', message: {defaultValue: 'Account suspended'}},
      },
    });

    const result = await send();

    expect(result.isError).toBe(true);
    expect(result.error).toMatchObject({code: 'FET-1114'});
  });

  it('fails when the flow returns without reporting a sent link', async () => {
    mockHttpRequest.mockResolvedValue({data: {flowStatus: 'INCOMPLETE', data: {additionalData: {}}}});

    const result = await send();

    expect(result.isError).toBe(true);
    expect(result.error?.message).toBe('The verification flow did not report a sent link');
  });

  it('fails when the flow completes instead of waiting for the user', async () => {
    mockHttpRequest.mockResolvedValue({data: {flowStatus: 'COMPLETE', data: {additionalData: {channel: 'email'}}}});

    const result = await send();

    expect(result.isError).toBe(true);
    expect(result.error?.message).toBe('The verification flow did not report a sent link');
  });

  it('leaves a request failure to the caller', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Network down'));

    const result = await send();

    expect(result.isError).toBe(true);
    expect(mockShowToast).not.toHaveBeenCalled();
  });
});
