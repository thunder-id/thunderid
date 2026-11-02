// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {AgentTypeListResponse} from '../../models/responses';
import useGetAgentTypes from '../useGetAgentTypes';

const mockHttpRequest = vi.fn();
const mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {request: mockHttpRequest},
  }),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: mockGetServerUrl}),
  };
});

describe('useGetAgentTypes', () => {
  const mockResponse: AgentTypeListResponse = {
    totalResults: 1,
    startIndex: 1,
    count: 1,
    types: [
      {
        id: 'aaa-bbb-ccc',
        handle: 'default',
        displayName: 'Default',
        ouId: '111e8400-e29b-41d4-a716-446655440000',
      },
    ],
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should fetch agent types list with include=display', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockResponse});

    const {result} = renderHook(() => useGetAgentTypes());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockResponse);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/agent-types?include=display',
        method: 'GET',
      }),
    );
  });

  it('should pass pagination params when supplied', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockResponse});

    const {result} = renderHook(() => useGetAgentTypes({limit: 5, offset: 10}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/agent-types?limit=5&offset=10&include=display',
      }),
    );
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to load agent types');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetAgentTypes());

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
  });
});
