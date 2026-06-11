/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {renderHook, waitFor} from '@testing-library/react';
import type {ReactNode} from 'react';
import {createElement} from 'react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import type {ReleasesData} from '../../models/download-assets';
import useWayfinderReleases from '../useWayfinderReleases';

const mockReleasesData: ReleasesData = {
  latestRelease: {
    tagName: 'v1.0.0',
    assets: [
      {
        name: 'wayfinder-linux-amd64',
        downloadUrl: 'https://example.com/wayfinder-linux-amd64',
        sizeLabel: '10 MB',
      },
    ],
  },
  releases: [
    {
      tagName: 'v1.0.0',
      assets: [
        {
          name: 'wayfinder-linux-amd64',
          downloadUrl: 'https://example.com/wayfinder-linux-amd64',
          sizeLabel: '10 MB',
        },
      ],
    },
  ],
};

function createWrapper(): ({children}: {children: ReactNode}) => ReactNode {
  const queryClient = new QueryClient({
    defaultOptions: {queries: {retryDelay: 0}},
  });
  function Wrapper({children}: {children: ReactNode}): ReactNode {
    return createElement(QueryClientProvider, {client: queryClient}, children);
  }
  return Wrapper;
}

describe('useWayfinderReleases', () => {
  const mockFetch = vi.fn();

  beforeEach(() => {
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it('returns data on successful fetch', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => mockReleasesData,
    });

    const {result} = renderHook(() => useWayfinderReleases('https://example.com/releases.json'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toEqual(mockReleasesData);
    expect(mockFetch).toHaveBeenCalledWith('https://example.com/releases.json');
  });

  it('returns error when fetch fails with non-ok status', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
    });

    const {result} = renderHook(() => useWayfinderReleases('https://example.com/releases.json'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(result.current.error?.message).toBe('Failed to fetch releases: 404');
  });

  it('returns error when fetch throws a network error', async () => {
    mockFetch.mockRejectedValue(new Error('Network error'));

    const {result} = renderHook(() => useWayfinderReleases('https://example.com/releases.json'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(result.current.error?.message).toBe('Network error');
  });

  it('uses releasesUrl as part of query key', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => mockReleasesData,
    });

    const wrapper = createWrapper();

    const {result: result1} = renderHook(() => useWayfinderReleases('https://example.com/v1/releases.json'), {
      wrapper,
    });

    const {result: result2} = renderHook(() => useWayfinderReleases('https://example.com/v2/releases.json'), {
      wrapper,
    });

    await waitFor(() => expect(result1.current.isSuccess).toBe(true));
    await waitFor(() => expect(result2.current.isSuccess).toBe(true));

    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch).toHaveBeenCalledWith('https://example.com/v1/releases.json');
    expect(mockFetch).toHaveBeenCalledWith('https://example.com/v2/releases.json');
  });
});
