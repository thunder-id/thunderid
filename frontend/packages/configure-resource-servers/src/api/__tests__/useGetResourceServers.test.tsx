// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import type {ResourceServerListResponse} from '../../models/resource-server';
import useGetResourceServers from '../useGetResourceServers';

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

describe('useGetResourceServers', () => {
  const firstPageData: ResourceServerListResponse = {
    totalResults: 12,
    startIndex: 0,
    count: 10,
    resourceServers: [
      {id: 'rs-1', name: 'API One', identifier: 'https://api.one', ouId: 'ou-1', delimiter: ':', type: 'API'},
      {id: 'rs-2', name: 'API Two', identifier: 'https://api.two', ouId: 'ou-1', delimiter: ':', type: 'API'},
      ...Array.from({length: 8}, (_, index) => ({
        id: `rs-${index + 3}`,
        name: `API ${index + 3}`,
        identifier: `https://api.${index + 3}`,
        ouId: 'ou-1',
        delimiter: ':',
        type: 'API' as const,
      })),
    ],
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReturnValue('https://localhost:8090');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should keep previous page data while fetching the next page', async () => {
    const nextPageData: ResourceServerListResponse = {
      totalResults: 12,
      startIndex: 10,
      count: 2,
      resourceServers: [
        {
          id: 'rs-11',
          name: 'API Eleven',
          identifier: 'https://api.eleven',
          ouId: 'ou-1',
          delimiter: ':',
          type: 'API',
        },
        {
          id: 'rs-12',
          name: 'API Twelve',
          identifier: 'https://api.twelve',
          ouId: 'ou-1',
          delimiter: ':',
          type: 'API',
        },
      ],
    };
    let resolveNextPage: ((value: {data: ResourceServerListResponse}) => void) | undefined;
    mockHttpRequest.mockImplementation((request: {url: string}): unknown => {
      if (request.url.includes('offset=0')) {
        return Promise.resolve({data: firstPageData});
      }

      if (request.url.includes('offset=10')) {
        return new Promise((resolve) => {
          resolveNextPage = resolve;
        });
      }

      throw new Error(`Unexpected resource-server request: ${request.url}`);
    });

    const {result, rerender} = renderHook(({offset}: {offset: number}) => useGetResourceServers({limit: 10, offset}), {
      initialProps: {offset: 0},
    });

    await waitFor(() => {
      expect(result.current.data).toEqual(firstPageData);
    });

    rerender({offset: 10});

    expect(result.current.data).toEqual(firstPageData);
    expect(result.current.isFetching).toBe(true);
    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: 'https://localhost:8090/resource-servers?limit=10&offset=10',
        }),
      );
      expect(resolveNextPage).toBeDefined();
    });

    if (!resolveNextPage) {
      throw new Error('The next-page request was not captured');
    }
    resolveNextPage({data: nextPageData});

    await waitFor(() => {
      expect(result.current.isFetching).toBe(false);
      expect(result.current.data).toEqual(nextPageData);
    });
  });
});
