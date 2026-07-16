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
import ConnectionQueryKeys from '../../constants/query-keys';
import type {ConnectionInstance, ConnectionListResponse} from '../../models/connection';
import useSMSProviders from '../useSMSProviders';

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

/** Builds a GET /connections envelope response for the mock HTTP client. */
const connectionsResponse = (connections: ConnectionInstance[]): {data: ConnectionListResponse} => ({
  data: {totalResults: connections.length, startIndex: 1, count: connections.length, connections, links: []},
});

describe('useSMSProviders', () => {
  const mockSenders: ConnectionInstance[] = [
    {
      id: 'sender-1',
      name: 'Twilio SMS',
      description: 'Twilio SMS sender',
      type: 'twilio',
      categories: ['sms-provider'],
    },
    {id: 'sender-2', name: 'Vonage SMS', type: 'vonage', categories: ['sms-provider']},
    {
      id: 'sender-3',
      name: 'Custom Sender',
      description: 'Custom SMS provider',
      type: 'custom',
      categories: ['sms-provider'],
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Query Configuration', () => {
    it('should use correct query keys', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useSMSProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(ConnectionQueryKeys.CONNECTIONS).toBe('connections');
      expect(ConnectionQueryKeys.SMS_PROVIDERS).toBe('sms-providers');
    });

    it('should make HTTP request to GET /connections?category=sms-provider', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useSMSProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(mockHttpRequest).toHaveBeenCalledWith({
        url: 'https://api.example.com/connections?category=sms-provider',
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });
    });

    it('should use server URL from config', async () => {
      mockGetServerUrl.mockReturnValueOnce('https://custom-server.com');
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useSMSProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(mockGetServerUrl).toHaveBeenCalled();
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: 'https://custom-server.com/connections?category=sms-provider',
        }),
      );
    });
  });

  describe('Successful Response', () => {
    it('should return the unwrapped connections array on success', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useSMSProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockSenders);
    });

    it('should return empty array when no senders exist', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse([]));

      const {result} = renderHook(() => useSMSProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual([]);
    });

    it('should return senders with all provider types', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useSMSProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      const types = result.current.data?.map((sender) => sender.type);
      expect(types).toContain('twilio');
      expect(types).toContain('vonage');
      expect(types).toContain('custom');
    });
  });

  describe('Loading State', () => {
    it('should be loading initially', () => {
      mockHttpRequest.mockImplementation(() => new Promise(() => null)); // Never resolves

      const {result} = renderHook(() => useSMSProviders());

      expect(result.current.isLoading).toBe(true);
      expect(result.current.data).toBeUndefined();
    });

    it('should not be loading after data is fetched', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useSMSProviders());

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });
    });
  });

  describe('Error Handling', () => {
    it('should handle network errors', async () => {
      mockHttpRequest.mockRejectedValueOnce(new Error('Network error'));

      const {result} = renderHook(() => useSMSProviders());

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error).toBeDefined();
    });

    it('should handle server errors', async () => {
      mockHttpRequest.mockRejectedValueOnce(new Error('Internal Server Error'));

      const {result} = renderHook(() => useSMSProviders());

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });
    });

    it('should handle 401 unauthorized errors', async () => {
      mockHttpRequest.mockRejectedValueOnce(new Error('Unauthorized'));

      const {result} = renderHook(() => useSMSProviders());

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });
    });
  });

  describe('Return Type', () => {
    it('should return UseQueryResult type', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useSMSProviders());

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
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useSMSProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(typeof result.current.refetch).toBe('function');
    });
  });

  describe('Data Structure', () => {
    it('should return senders with required fields', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useSMSProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      result.current.data?.forEach((sender) => {
        expect(sender).toHaveProperty('id');
        expect(sender).toHaveProperty('name');
        expect(sender).toHaveProperty('type');
        expect(sender).toHaveProperty('categories');
      });
    });

    it('should return senders with optional description', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useSMSProviders());

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
