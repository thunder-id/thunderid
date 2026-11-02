// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import useGetNotificationTemplate from '../useGetNotificationTemplate';

const {mockHttpRequest} = vi.hoisted(() => ({mockHttpRequest: vi.fn()}));

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://api.test.com'}),
  };
});

describe('useGetNotificationTemplate', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches a single template by channel and id', async () => {
    const tmpl = {id: 'tmpl-1', handle: 'otp', displayName: 'OTP', content: {subject: 's', body: 'b'}};
    mockHttpRequest.mockResolvedValueOnce({data: tmpl});

    const {result} = renderHook(() => useGetNotificationTemplate('email', 'tmpl-1'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://api.test.com/notification-templates/email/templates/tmpl-1',
      method: 'GET',
      headers: {'Content-Type': 'application/json'},
    });
    expect(result.current.data).toEqual(tmpl);
  });

  it('is disabled (no request) when no id is provided', () => {
    const {result} = renderHook(() => useGetNotificationTemplate('email', ''));

    expect(mockHttpRequest).not.toHaveBeenCalled();
    expect(result.current.fetchStatus).toBe('idle');
  });
});
