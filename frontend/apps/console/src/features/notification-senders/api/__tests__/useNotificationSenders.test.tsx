/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import NotificationSenderQueryKeys from '../../constants/query-keys';
import type {NotificationSenderListResponse} from '../../models/notification-sender';
import useNotificationSenders from '../useNotificationSenders';

// Mock useConfig
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

// Mock useThunderID
const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {
      request: mockHttpRequest,
    },
  }),
}));

describe('useNotificationSenders', () => {
  const mockNotificationSenders: NotificationSenderListResponse = [
    {
      id: 'sender-1',
      name: 'Twilio SMS',
      description: 'Twilio SMS sender',
      provider: 'twilio',
      properties: [
        {name: 'accountSid', value: 'AC123'},
        {name: 'authToken', value: 'token123', isSecret: true},
      ],
    },
    {
      id: 'sender-2',
      name: 'Vonage SMS',
      provider: 'vonage',
    },
    {
      id: 'sender-3',
      name: 'Custom Sender',
      description: 'Custom SMS provider',
      provider: 'custom',
      properties: [],
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Query Configuration', () => {
    it('should use correct query keys', async () => {
      mockHttpRequest.mockResolvedValueOnce({data: mockNotificationSenders});

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      // Verify query keys are correct
      expect(NotificationSenderQueryKeys.NOTIFICATION_SENDERS).toBe('notification-senders');
      expect(NotificationSenderQueryKeys.MESSAGE_SENDERS).toBe('message-senders');
    });

    it('should make HTTP request to correct endpoint', async () => {
      mockHttpRequest.mockResolvedValueOnce({data: mockNotificationSenders});

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(mockHttpRequest).toHaveBeenCalledWith({
        url: 'https://api.example.com/notification-senders/message',
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });
    });

    it('should use server URL from config', async () => {
      mockGetServerUrl.mockReturnValueOnce('https://custom-server.com');
      mockHttpRequest.mockResolvedValueOnce({data: mockNotificationSenders});

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(mockGetServerUrl).toHaveBeenCalled();
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: 'https://custom-server.com/notification-senders/message',
        }),
      );
    });
  });

  describe('Successful Response', () => {
    it('should return notification senders list on success', async () => {
      mockHttpRequest.mockResolvedValueOnce({data: mockNotificationSenders});

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockNotificationSenders);
    });

    it('should return empty array when no senders exist', async () => {
      mockHttpRequest.mockResolvedValueOnce({data: []});

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual([]);
    });

    it('should return senders with all providers', async () => {
      mockHttpRequest.mockResolvedValueOnce({data: mockNotificationSenders});

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      const providers = result.current.data?.map((sender) => sender.provider);
      expect(providers).toContain('twilio');
      expect(providers).toContain('vonage');
      expect(providers).toContain('custom');
    });

    it('should return senders with properties', async () => {
      mockHttpRequest.mockResolvedValueOnce({data: mockNotificationSenders});

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      const twilioSender = result.current.data?.find((s) => s.id === 'sender-1');
      expect(twilioSender?.properties).toHaveLength(2);
      expect(twilioSender?.properties?.[0]).toEqual({name: 'accountSid', value: 'AC123'});
      expect(twilioSender?.properties?.[1]).toEqual({name: 'authToken', value: 'token123', isSecret: true});
    });
  });

  describe('Loading State', () => {
    it('should be loading initially', () => {
      mockHttpRequest.mockImplementation(() => new Promise(() => null)); // Never resolves

      const {result} = renderHook(() => useNotificationSenders());

      expect(result.current.isLoading).toBe(true);
      expect(result.current.data).toBeUndefined();
    });

    it('should not be loading after data is fetched', async () => {
      mockHttpRequest.mockResolvedValueOnce({data: mockNotificationSenders});

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });
    });
  });

  describe('Error Handling', () => {
    it('should handle network errors', async () => {
      mockHttpRequest.mockRejectedValueOnce(new Error('Network error'));

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error).toBeDefined();
    });

    it('should handle server errors', async () => {
      mockHttpRequest.mockRejectedValueOnce(new Error('Internal Server Error'));

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });
    });

    it('should handle 401 unauthorized errors', async () => {
      mockHttpRequest.mockRejectedValueOnce(new Error('Unauthorized'));

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });
    });
  });

  describe('Return Type', () => {
    it('should return UseQueryResult type', async () => {
      mockHttpRequest.mockResolvedValueOnce({data: mockNotificationSenders});

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      // Check that all expected properties exist
      expect(result.current).toHaveProperty('data');
      expect(result.current).toHaveProperty('isLoading');
      expect(result.current).toHaveProperty('isError');
      expect(result.current).toHaveProperty('error');
      expect(result.current).toHaveProperty('isSuccess');
      expect(result.current).toHaveProperty('refetch');
    });

    it('should have refetch function', async () => {
      mockHttpRequest.mockResolvedValueOnce({data: mockNotificationSenders});

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(typeof result.current.refetch).toBe('function');
    });
  });

  describe('Data Structure', () => {
    it('should return senders with required fields', async () => {
      mockHttpRequest.mockResolvedValueOnce({data: mockNotificationSenders});

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      result.current.data?.forEach((sender) => {
        expect(sender).toHaveProperty('id');
        expect(sender).toHaveProperty('name');
        expect(sender).toHaveProperty('provider');
      });
    });

    it('should return senders with optional description', async () => {
      mockHttpRequest.mockResolvedValueOnce({data: mockNotificationSenders});

      const {result} = renderHook(() => useNotificationSenders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      const senderWithDescription = result.current.data?.find((s) => s.description);
      const senderWithoutDescription = result.current.data?.find((s) => !s.description);

      expect(senderWithDescription?.description).toBeDefined();
      expect(senderWithoutDescription?.description).toBeUndefined();
    });
  });
});
