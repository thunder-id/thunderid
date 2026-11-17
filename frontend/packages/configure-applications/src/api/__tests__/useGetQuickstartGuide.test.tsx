// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import useGetQuickstartGuide from '../useGetQuickstartGuide';

const mockFetch = vi.fn();

describe('useGetQuickstartGuide', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('fetches the Markdown export of the docs page and returns its steps', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      text: () => Promise.resolve('## Create an Application\n\nCreate it.\n\n## Install the SDK\n\nInstall it.'),
    });

    const {result} = renderHook(() => useGetQuickstartGuide('https://example.com/docs/react/'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(mockFetch).toHaveBeenCalledWith('https://example.com/docs/react.md');
    expect(result.current.data).toEqual({
      steps: [{title: 'Install the SDK', blocks: [{type: 'paragraph', text: 'Install it.'}]}],
      sample: null,
    });
  });

  it('returns the steps for the choices given, of a page written for several', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      text: () =>
        Promise.resolve(
          '## Register an Agent\n\nAdd it.\n\n## Run the Agent\n\n<!-- variant mode=own -->\n\nRun it alone.\n\n<!-- /variant -->\n\n<!-- variant mode=obo -->\n\nRun it for a user.\n\n<!-- /variant -->',
        ),
    });

    const {result} = renderHook(() =>
      useGetQuickstartGuide('https://example.com/docs/langchain/', undefined, {mode: 'obo'}),
    );

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.steps).toEqual([
      {title: 'Run the Agent', blocks: [{type: 'paragraph', text: 'Run it for a user.'}]},
    ]);
  });

  it('returns null for a page that is not a quickstart', async () => {
    mockFetch.mockResolvedValue({ok: true, status: 200, text: () => Promise.resolve('<!doctype html>')});

    const {result} = renderHook(() => useGetQuickstartGuide('https://example.com/docs/react'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toBeNull();
  });

  it('fails when the docs site does not return the page', async () => {
    mockFetch.mockResolvedValue({ok: false, status: 404, text: () => Promise.resolve('')});

    const {result} = renderHook(() => useGetQuickstartGuide('https://example.com/docs/react/'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });

  it('does not fetch without a docs URL', () => {
    renderHook(() => useGetQuickstartGuide(undefined));

    expect(mockFetch).not.toHaveBeenCalled();
  });
});
