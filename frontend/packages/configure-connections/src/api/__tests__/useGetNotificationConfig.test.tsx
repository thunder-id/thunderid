// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import type {NotificationConfigResponse} from '../../models/connection';
import useGetNotificationConfig from '../useGetNotificationConfig';

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

describe('useGetNotificationConfig', () => {
  const mockResponse: NotificationConfigResponse = {
    readOnly: {},
    writable: {defaultEmailSenderId: 'sm1'},
    merged: {defaultEmailSenderId: 'sm1'},
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => vi.clearAllMocks());

  it('GETs the notification section', async () => {
    mockHttpRequest.mockResolvedValue({data: mockResponse});
    const {result} = renderHook(() => useGetNotificationConfig());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://localhost:8090/server-config/notification', method: 'GET'}),
    );
    expect(result.current.data).toEqual(mockResponse);
  });

  it('does not fetch when disabled', () => {
    renderHook(() => useGetNotificationConfig({enabled: false}));

    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('surfaces fetch errors', async () => {
    mockHttpRequest.mockRejectedValue(new Error('boom'));
    const {result} = renderHook(() => useGetNotificationConfig());

    await waitFor(() => expect(result.current.error?.message).toBe('boom'));
  });
});
