// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import type {NotificationConfigResponse} from '../../models/connection';
import useSetDefaultEmailSender from '../useSetDefaultEmailSender';

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://localhost:8090'}),
  };
});

describe('useSetDefaultEmailSender', () => {
  const mockResponse: NotificationConfigResponse = {
    readOnly: {},
    writable: {defaultEmailSenderId: 'sm2'},
    merged: {defaultEmailSenderId: 'sm2'},
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => vi.clearAllMocks());

  it('PUTs the sender id to the notification section', async () => {
    mockHttpRequest.mockResolvedValue({data: mockResponse});
    const {result} = renderHook(() => useSetDefaultEmailSender());

    result.current.mutate({defaultEmailSenderId: 'sm2'});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/server-config/notification',
        method: 'PUT',
        data: {defaultEmailSenderId: 'sm2'},
      }),
    );
  });

  it('writes the returned layers into the query cache', async () => {
    mockHttpRequest.mockResolvedValue({data: mockResponse});
    const {result, queryClient} = renderHook(() => useSetDefaultEmailSender());

    result.current.mutate({defaultEmailSenderId: 'sm2'});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(queryClient.getQueryData(['server-config', 'notification'])).toEqual(mockResponse);
  });

  it('surfaces update errors', async () => {
    mockHttpRequest.mockRejectedValue(new Error('nope'));
    const {result} = renderHook(() => useSetDefaultEmailSender());

    result.current.mutate({defaultEmailSenderId: 'sm2'});

    await waitFor(() => expect(result.current.error?.message).toBe('nope'));
  });
});
