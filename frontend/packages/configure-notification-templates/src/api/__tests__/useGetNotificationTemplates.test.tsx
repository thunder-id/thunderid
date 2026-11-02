// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {TemplateListResponse} from '../../models/responses';
import useGetNotificationTemplates from '../useGetNotificationTemplates';

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

const listResponse = (): {data: TemplateListResponse} => ({
  data: {
    totalResults: 1,
    startIndex: 1,
    count: 1,
    templates: [{id: 'id-1', handle: 'otp', displayName: 'OTP'}],
  },
});

describe('useGetNotificationTemplates', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('requests the channel collection with default pagination', async () => {
    mockHttpRequest.mockResolvedValueOnce(listResponse());

    const {result} = renderHook(() => useGetNotificationTemplates('email'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://api.test.com/notification-templates/email/templates?limit=30&offset=0',
      method: 'GET',
      headers: {'Content-Type': 'application/json'},
    });
  });

  it('forwards custom limit and offset to the query string', async () => {
    mockHttpRequest.mockResolvedValueOnce(listResponse());

    const {result} = renderHook(() => useGetNotificationTemplates('sms', {limit: 10, offset: 20}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://api.test.com/notification-templates/sms/templates?limit=10&offset=20',
      method: 'GET',
      headers: {'Content-Type': 'application/json'},
    });
  });

  it('surfaces an error when the request fails', async () => {
    mockHttpRequest.mockRejectedValueOnce(new Error('List failed'));

    const {result} = renderHook(() => useGetNotificationTemplates('email'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });
});
