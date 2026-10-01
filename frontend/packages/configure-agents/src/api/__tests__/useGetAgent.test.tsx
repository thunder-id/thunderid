// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {Agent} from '../../models/agent';
import useGetAgent from '../useGetAgent';

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
  };
});

const {useThunderID} = await import('@thunderid/react');
const {useConfig} = await import('@thunderid/contexts');

describe('useGetAgent', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;

  const mockAgent: Agent = {
    id: '550e8400-e29b-41d4-a716-446655440000',
    ouId: '111e8400-e29b-41d4-a716-446655440000',
    type: 'default',
    name: 'Billing Service',
    description: 'Service-to-service billing agent',
  };

  beforeEach(() => {
    mockHttpRequest = vi.fn();

    vi.mocked(useThunderID).mockReturnValue({
      http: {request: mockHttpRequest},
    } as unknown as ReturnType<typeof useThunderID>);

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => 'https://api.test.com',
    } as ReturnType<typeof useConfig>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should not fetch when agentId is empty', () => {
    mockHttpRequest.mockReturnValue(new Promise(() => null));

    const {result} = renderHook(() => useGetAgent(''));

    expect(result.current.isLoading).toBe(false);
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should fetch agent by ID', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgent});

    const {result} = renderHook(() => useGetAgent(mockAgent.id));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockAgent);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/agents/${mockAgent.id}?include=display`,
        method: 'GET',
      }),
    );
  });

  it('should handle API error', async () => {
    const apiError = new Error('Agent not found');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetAgent(mockAgent.id));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
  });
});
