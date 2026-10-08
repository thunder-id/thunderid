// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {NotificationTemplateListResponse, NotificationTemplateSummary} from '../../models/notification-templates';
import useGetNotificationTemplates from '../useGetNotificationTemplates';

const mockGetServerUrl = vi.fn(() => 'https://api.example.com');

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      getServerUrl: mockGetServerUrl,
    }),
  };
});

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {
      request: mockHttpRequest,
    },
  }),
}));

/** Builds a list envelope response for the mock HTTP client. */
const listResponse = (
  templates: NotificationTemplateSummary[],
  totalResults = templates.length,
  startIndex = 1,
): {data: NotificationTemplateListResponse} => ({
  data: {totalResults, startIndex, count: templates.length, templates},
});

/** Builds a single template summary for a given channel. */
const template = (index: number, channel = 'email'): NotificationTemplateSummary => ({
  id: `id-${index}`,
  handle: `template-${index}`,
  displayName: `Template ${index}`,
  self: `/notification-templates/${channel}/templates/id-${index}`,
});

describe('useGetNotificationTemplates', () => {
  const mockTemplates: NotificationTemplateSummary[] = [
    {
      id: 'id-1',
      handle: 'user-invite',
      displayName: 'User Invite',
      self: '/notification-templates/email/templates/id-1',
    },
    {
      id: 'id-2',
      handle: 'password-recovery',
      displayName: 'Password Recovery',
      self: '/notification-templates/email/templates/id-2',
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should request the channel template collection', async () => {
    mockHttpRequest.mockResolvedValueOnce(listResponse(mockTemplates));

    const {result} = renderHook(() => useGetNotificationTemplates('email'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://api.example.com/notification-templates/email/templates?limit=100&offset=0',
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });
  });

  it('should request the sms channel when asked', async () => {
    mockHttpRequest.mockResolvedValueOnce(listResponse([]));

    const {result} = renderHook(() => useGetNotificationTemplates('sms'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.example.com/notification-templates/sms/templates?limit=100&offset=0',
      }),
    );
  });

  it('should return the unwrapped templates array on success', async () => {
    mockHttpRequest.mockResolvedValueOnce(listResponse(mockTemplates));

    const {result} = renderHook(() => useGetNotificationTemplates('email'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockTemplates);
  });

  it('should page through with offset until all templates are loaded', async () => {
    const firstPage: NotificationTemplateSummary[] = Array.from({length: 100}, (_, i) => template(i));
    const secondPage: NotificationTemplateSummary[] = [template(100), template(101)];

    mockHttpRequest
      .mockResolvedValueOnce(listResponse(firstPage, 102, 1))
      .mockResolvedValueOnce(listResponse(secondPage, 102, 101));

    const {result} = renderHook(() => useGetNotificationTemplates('email'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        url: 'https://api.example.com/notification-templates/email/templates?limit=100&offset=0',
      }),
    );
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        url: 'https://api.example.com/notification-templates/email/templates?limit=100&offset=100',
      }),
    );
    expect(result.current.data).toHaveLength(102);
  });

  it('should keep paging past a full page when totalResults is absent', async () => {
    const firstPage: NotificationTemplateSummary[] = Array.from({length: 100}, (_, i) => template(i));
    const secondPage: NotificationTemplateSummary[] = [template(100)];

    // Neither page reports totalResults, so only the short second page can end the loop.
    mockHttpRequest
      .mockResolvedValueOnce({data: {templates: firstPage}})
      .mockResolvedValueOnce({data: {templates: secondPage}});

    const {result} = renderHook(() => useGetNotificationTemplates('email'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
    expect(result.current.data).toHaveLength(101);
  });

  it('should stop after a short page when totalResults is absent', async () => {
    const page: NotificationTemplateSummary[] = [template(0), template(1)];

    mockHttpRequest.mockResolvedValueOnce({data: {templates: page}});

    const {result} = renderHook(() => useGetNotificationTemplates('email'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    expect(result.current.data).toEqual(page);
  });

  it('should be loading initially', () => {
    mockHttpRequest.mockImplementation(() => new Promise(() => null));

    const {result} = renderHook(() => useGetNotificationTemplates('email'));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
  });

  it('should handle errors', async () => {
    mockHttpRequest.mockRejectedValueOnce(new Error('Network error'));

    const {result} = renderHook(() => useGetNotificationTemplates('email'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBeDefined();
  });
});
