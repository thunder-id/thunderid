// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import NotificationTemplateQueryKeys from '../../constants/notification-template-query-keys';
import useDeleteNotificationTemplate from '../useDeleteNotificationTemplate';

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

describe('useDeleteNotificationTemplate', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('deletes the template at its id endpoint', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useDeleteNotificationTemplate());
    result.current.mutate({channel: 'sms', id: 'tmpl-9'});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://api.test.com/notification-templates/sms/templates/tmpl-9',
      method: 'DELETE',
      headers: {'Content-Type': 'application/json'},
    });
  });

  it('removes the single-template query, invalidates the list, and toasts on success', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result, queryClient} = renderHook(() => useDeleteNotificationTemplate());
    const removeSpy = vi.spyOn(queryClient, 'removeQueries');
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({channel: 'email', id: 'tmpl-1'});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(removeSpy).toHaveBeenCalledWith({queryKey: [NotificationTemplateQueryKeys.TEMPLATE, 'email', 'tmpl-1']});
    expect(invalidateSpy).toHaveBeenCalledWith({queryKey: [NotificationTemplateQueryKeys.TEMPLATES, 'email']});
    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('surfaces the error and does not toast when the request fails', async () => {
    const apiError = new Error('Delete failed');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useDeleteNotificationTemplate());
    result.current.mutate({channel: 'email', id: 'tmpl-1'});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(mockShowToast).not.toHaveBeenCalled();
  });
});
