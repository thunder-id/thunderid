// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import AgentTypeQueryKeys from '../../constants/agentTypeQueryKeys';
import type {AgentTypeListItem} from '../../models/agent-type';
import useGetAllAgentTypes from '../useGetAllAgentTypes';

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

const agentType = (index: number): AgentTypeListItem => ({
  id: `type-${index}`,
  handle: `handle-${index}`,
  displayName: `Type ${index}`,
  ouId: `ou-${index}`,
});

const page = (types: AgentTypeListItem[], totalResults: number) => ({
  data: {totalResults, startIndex: 1, count: types.length, types},
});

const requestedUrls = (): string[] => mockHttpRequest.mock.calls.map(([config]) => (config as {url: string}).url);

describe('useGetAllAgentTypes', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should return every agent type when they fit on one page', async () => {
    mockHttpRequest.mockResolvedValueOnce(page([agentType(1), agentType(2)], 2));

    const {result} = renderHook(() => useGetAllAgentTypes());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual([agentType(1), agentType(2)]);
    expect(requestedUrls()).toEqual(['https://api.test.com/agent-types?limit=100&offset=0&include=display']);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({method: 'GET', headers: {'Content-Type': 'application/json'}}),
    );
  });

  // A handle lookup must find a type that sits beyond the first page.
  it('should follow pagination until every agent type is fetched', async () => {
    const firstPage = Array.from({length: 100}, (_, index) => agentType(index));
    mockHttpRequest.mockResolvedValueOnce(page(firstPage, 101)).mockResolvedValueOnce(page([agentType(100)], 101));

    const {result} = renderHook(() => useGetAllAgentTypes());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toHaveLength(101);
    expect(result.current.data?.find((type) => type.handle === 'handle-100')).toEqual(agentType(100));
    expect(requestedUrls()).toEqual([
      'https://api.test.com/agent-types?limit=100&offset=0&include=display',
      'https://api.test.com/agent-types?limit=100&offset=100&include=display',
    ]);
  });

  it('should stop when a page comes back empty before the reported total', async () => {
    mockHttpRequest.mockResolvedValueOnce(page([agentType(1)], 5)).mockResolvedValueOnce(page([], 5));

    const {result} = renderHook(() => useGetAllAgentTypes());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual([agentType(1)]);
    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should return an empty list when there are no agent types', async () => {
    mockHttpRequest.mockResolvedValueOnce(page([], 0));

    const {result} = renderHook(() => useGetAllAgentTypes());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual([]);
  });

  it('should surface a failed page as an error', async () => {
    const apiError = new Error('List failed');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetAllAgentTypes());

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
  });

  it('should not fetch when disabled', () => {
    const {result} = renderHook(() => useGetAllAgentTypes({enabled: false}));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should cache under the agent type list key so list invalidations refresh it', async () => {
    mockHttpRequest.mockResolvedValueOnce(page([agentType(1)], 1));

    const {result, queryClient} = renderHook(() => useGetAllAgentTypes());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(queryClient.getQueryData([AgentTypeQueryKeys.AGENT_TYPES, 'all'])).toEqual([agentType(1)]);
  });

  it('should serve a cached result without another request', async () => {
    mockHttpRequest.mockResolvedValueOnce(page([agentType(1)], 1));

    const {result, rerender} = renderHook(() => useGetAllAgentTypes());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    rerender();

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
  });

  it('should refetch on demand', async () => {
    mockHttpRequest.mockResolvedValue(page([agentType(1)], 1));

    const {result} = renderHook(() => useGetAllAgentTypes());

    await waitFor(() => {
      expect(result.current.data).toEqual([agentType(1)]);
    });

    mockHttpRequest.mockResolvedValue(page([agentType(2)], 1));
    await result.current.refetch();

    await waitFor(() => {
      expect(result.current.data).toEqual([agentType(2)]);
    });
  });
});
