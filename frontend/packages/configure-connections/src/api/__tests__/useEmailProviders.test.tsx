// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ConnectionQueryKeys from '../../constants/query-keys';
import type {ConnectionInstance, ConnectionListResponse} from '../../models/connection';
import useEmailProviders from '../useEmailProviders';

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

describe('useEmailProviders', () => {
  const mockSenders: ConnectionInstance[] = [
    {
      id: 'sender-1',
      name: 'Corp SMTP',
      description: 'Transactional mail',
      type: 'email-smtp',
      categories: ['email-provider'],
    },
    {id: 'sender-2', name: 'Backup SMTP', type: 'email-smtp', categories: ['email-provider']},
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Query Configuration', () => {
    it('should use correct query keys', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useEmailProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(ConnectionQueryKeys.CONNECTIONS).toBe('connections');
      expect(ConnectionQueryKeys.EMAIL_PROVIDERS).toBe('email-providers');
    });

    it('should make HTTP request to GET /connections?category=email-provider', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useEmailProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(mockHttpRequest).toHaveBeenCalledWith({
        url: 'https://api.example.com/connections?category=email-provider',
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });
    });

    it('should use server URL from config', async () => {
      mockGetServerUrl.mockReturnValueOnce('https://custom-server.com');
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useEmailProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(mockGetServerUrl).toHaveBeenCalled();
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: 'https://custom-server.com/connections?category=email-provider',
        }),
      );
    });
  });

  describe('Successful Response', () => {
    it('should return the unwrapped connections array on success', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useEmailProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockSenders);
    });

    it('should return empty array when no senders exist', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse([]));

      const {result} = renderHook(() => useEmailProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual([]);
    });
  });

  describe('Loading State', () => {
    it('should be loading initially', () => {
      mockHttpRequest.mockImplementation(() => new Promise(() => null)); // Never resolves

      const {result} = renderHook(() => useEmailProviders());

      expect(result.current.isLoading).toBe(true);
      expect(result.current.data).toBeUndefined();
    });

    it('should not be loading after data is fetched', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useEmailProviders());

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });
    });
  });

  describe('Error Handling', () => {
    it('should handle network errors', async () => {
      mockHttpRequest.mockRejectedValueOnce(new Error('Network error'));

      const {result} = renderHook(() => useEmailProviders());

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error).toBeDefined();
    });

    it('should handle 401 unauthorized errors', async () => {
      mockHttpRequest.mockRejectedValueOnce(new Error('Unauthorized'));

      const {result} = renderHook(() => useEmailProviders());

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });
    });
  });

  describe('Data Structure', () => {
    it('should return senders with required fields', async () => {
      mockHttpRequest.mockResolvedValueOnce(connectionsResponse(mockSenders));

      const {result} = renderHook(() => useEmailProviders());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      result.current.data?.forEach((sender) => {
        expect(sender).toHaveProperty('id');
        expect(sender).toHaveProperty('name');
        expect(sender.type).toBe('email-smtp');
        expect(sender.categories).toContain('email-provider');
      });
    });
  });
});
