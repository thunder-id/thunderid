// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import NotificationTemplateQueryKeys from '../../constants/notification-template-query-keys';
import type {CreateTemplateRequest} from '../../models/requests';
import useCreateNotificationTemplate from '../useCreateNotificationTemplate';

const {mockHttpRequest, mockShowToast} = vi.hoisted(() => ({
  mockHttpRequest: vi.fn(),
  mockShowToast: vi.fn(),
}));

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

const emailData: CreateTemplateRequest = {
  handle: 'otp-verification',
  ouId: 'ou-default',
  displayName: 'OTP Verification',
  design: {colorScheme: 'light'},
  content: {subject: 'Your code', body: '<p>{{ctx(otpCode)}}</p>'},
};

describe('useCreateNotificationTemplate', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('posts the template to the channel collection and returns the created template', async () => {
    const created = {
      id: 'tmpl-1',
      handle: 'otp-verification',
      displayName: 'OTP Verification',
      content: emailData.content,
    };
    mockHttpRequest.mockResolvedValueOnce({data: created});

    const {result} = renderHook(() => useCreateNotificationTemplate());
    result.current.mutate({channel: 'email', data: emailData});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://api.test.com/notification-templates/email/templates',
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      data: emailData,
    });
    expect(result.current.data).toEqual(created);
  });

  it('invalidates the channel list and shows a success toast on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: {id: 'tmpl-1'}});

    const {result, queryClient} = renderHook(() => useCreateNotificationTemplate());
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({channel: 'email', data: emailData});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalledWith({queryKey: [NotificationTemplateQueryKeys.TEMPLATES, 'email']});
    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('surfaces the error and does not toast when the request fails', async () => {
    const apiError = new Error('Create failed');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useCreateNotificationTemplate());
    result.current.mutate({
      channel: 'sms',
      data: {handle: 'h', ouId: 'ou-default', displayName: 'n', content: {body: 'x'}},
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(mockShowToast).not.toHaveBeenCalled();
  });
});
