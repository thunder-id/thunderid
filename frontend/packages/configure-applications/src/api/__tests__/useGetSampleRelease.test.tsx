// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import useGetSampleRelease from '../useGetSampleRelease';

const mockFetch = vi.fn();
const releasesUrl = 'https://example.com/data/ecosystem-releases.json';

const respondWith = (body: unknown) =>
  mockFetch.mockResolvedValue({ok: true, status: 200, json: () => Promise.resolve(body)});

describe('useGetSampleRelease', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns the sample's released archive", async () => {
    respondWith({
      releases: [
        {type: 'sdk', packageId: 'quickstarts/react', assets: [{downloadUrl: 'https://example.com/sdk.zip'}]},
        {
          type: 'sample',
          packageId: 'quickstarts/react',
          latestVersion: '1.0.0',
          assets: [
            {downloadUrl: 'https://example.com/react.tar.gz'},
            {downloadUrl: 'https://example.com/react.zip', sizeLabel: '48 KB'},
          ],
        },
      ],
    });

    const {result} = renderHook(() => useGetSampleRelease(releasesUrl, 'quickstarts/react'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(mockFetch).toHaveBeenCalledWith(releasesUrl);
    expect(result.current.data).toEqual({
      downloadUrl: 'https://example.com/react.zip',
      sizeLabel: '48 KB',
      version: '1.0.0',
    });
  });

  it('returns null when the sample has no released archive', async () => {
    respondWith({releases: [{type: 'sample', packageId: 'quickstarts/vue', assets: []}]});

    const {result} = renderHook(() => useGetSampleRelease(releasesUrl, 'quickstarts/react'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toBeNull();
  });

  it('fails when the feed cannot be fetched', async () => {
    mockFetch.mockResolvedValue({ok: false, status: 500});

    const {result} = renderHook(() => useGetSampleRelease(releasesUrl, 'quickstarts/react'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });

  it('does not fetch without a feed URL or a sample', () => {
    renderHook(() => useGetSampleRelease(undefined, 'quickstarts/react'));
    renderHook(() => useGetSampleRelease(releasesUrl, undefined));

    expect(mockFetch).not.toHaveBeenCalled();
  });
});
