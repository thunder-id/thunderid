// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import useAuthZENPDPConnections from '../useAuthZENPDPConnections';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

const mockGetServerUrl = vi.fn<() => string>(() => 'https://localhost:8090');
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: mockGetServerUrl}),
  };
});

describe('useAuthZENPDPConnections', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReturnValue('https://localhost:8090');
  });

  it('returns the first page when all connections fit in one request', async () => {
    const connections = [{id: 'pdp-1', name: 'PDP One'}];
    mockHttpRequest.mockResolvedValue({data: {totalResults: 1, connections}});

    const {result} = renderHook(() => useAuthZENPDPConnections());

    await waitFor(() => {
      expect(result.current.data).toEqual(connections);
    });
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/connections?category=authorization-pdp&limit=100&offset=0',
      }),
    );
  });

  it('loads and combines every page of PDP connections', async () => {
    mockHttpRequest.mockImplementation((request: {url: string}) => {
      const offset = new URL(request.url).searchParams.get('offset');
      if (offset === '0') {
        return Promise.resolve({data: {totalResults: 201, connections: [{id: 'pdp-1', name: 'PDP One'}]}});
      }
      if (offset === '100') {
        return Promise.resolve({data: {totalResults: 201, connections: [{id: 'pdp-2', name: 'PDP Two'}]}});
      }
      if (offset === '200') {
        return Promise.resolve({data: {totalResults: 201, connections: [{id: 'pdp-3', name: 'PDP Three'}]}});
      }
      throw new Error(`Unexpected PDP connection request: ${request.url}`);
    });

    const {result} = renderHook(() => useAuthZENPDPConnections());

    await waitFor(() => {
      expect(result.current.data).toEqual([
        {id: 'pdp-1', name: 'PDP One'},
        {id: 'pdp-2', name: 'PDP Two'},
        {id: 'pdp-3', name: 'PDP Three'},
      ]);
    });
    expect(mockHttpRequest).toHaveBeenCalledTimes(3);
  });
});
