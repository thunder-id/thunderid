// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {ApiAgentType} from '../../models/agent-type';
import useGetAgentType from '../useGetAgentType';

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

describe('useGetAgentType', () => {
  const mockAgentType: ApiAgentType = {
    id: 'aaa-bbb-ccc',
    handle: 'default',
    displayName: 'Default',
    ouId: '111e8400-e29b-41d4-a716-446655440000',
    schema: {
      modelProvider: {
        type: 'string',
        required: true,
        enum: ['openai', 'anthropic', 'gemini', 'mistral', 'custom'],
      },
      model: {type: 'string', required: true},
      function: {
        type: 'string',
        required: false,
        enum: [
          'task-automation',
          'rag-retrieval',
          'code-gen',
          'data-analysis',
          'orchestrator',
          'sub-agent',
          'assistant',
          'custom',
        ],
      },
    },
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should not fetch when id is empty', () => {
    mockHttpRequest.mockReturnValue(new Promise(() => null));

    const {result} = renderHook(() => useGetAgentType(undefined));

    expect(result.current.isLoading).toBe(false);
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should fetch agent type by ID', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgentType});

    const {result} = renderHook(() => useGetAgentType(mockAgentType.id));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockAgentType);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/agent-types/${mockAgentType.id}?include=display`,
        method: 'GET',
      }),
    );
  });

  it('should handle API error', async () => {
    const apiError = new Error('Agent type not found');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetAgentType('aaa-bbb-ccc'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
  });
});
