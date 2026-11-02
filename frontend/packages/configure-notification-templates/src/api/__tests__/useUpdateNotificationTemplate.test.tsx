// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import NotificationTemplateQueryKeys from '../../constants/notification-template-query-keys';
import type {UpdateTemplateRequest} from '../../models/requests';
import useUpdateNotificationTemplate from '../useUpdateNotificationTemplate';

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

const updateData: UpdateTemplateRequest = {
  displayName: 'OTP Verification',
  design: {colorScheme: 'dark'},
  content: {subject: 'Your code', body: '<p>{{ctx(otpCode)}}</p>'},
};

describe('useUpdateNotificationTemplate', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('puts the template to its id endpoint and returns the updated template', async () => {
    const updated = {
      id: 'tmpl-1',
      handle: 'otp-verification',
      displayName: 'OTP Verification',
      content: updateData.content,
    };
    mockHttpRequest.mockResolvedValueOnce({data: updated});

    const {result} = renderHook(() => useUpdateNotificationTemplate());
    result.current.mutate({channel: 'email', id: 'tmpl-1', data: updateData});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://api.test.com/notification-templates/email/templates/tmpl-1',
      method: 'PUT',
      headers: {'Content-Type': 'application/json'},
      data: updateData,
    });
    expect(result.current.data).toEqual(updated);
  });

  it('invalidates both the single template and the channel list, and toasts on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: {id: 'tmpl-1'}});

    const {result, queryClient} = renderHook(() => useUpdateNotificationTemplate());
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({channel: 'email', id: 'tmpl-1', data: updateData});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalledWith({queryKey: [NotificationTemplateQueryKeys.TEMPLATE, 'email', 'tmpl-1']});
    expect(invalidateSpy).toHaveBeenCalledWith({queryKey: [NotificationTemplateQueryKeys.TEMPLATES, 'email']});
    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('surfaces the error and does not toast when the request fails', async () => {
    const apiError = new Error('Update failed');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useUpdateNotificationTemplate());
    result.current.mutate({channel: 'email', id: 'tmpl-1', data: updateData});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(mockShowToast).not.toHaveBeenCalled();
  });
});
