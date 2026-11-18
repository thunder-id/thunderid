// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import useGetCodingAgentPrompt from '../useGetCodingAgentPrompt';

const mockFetch = vi.fn();

describe('useGetCodingAgentPrompt', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('returns the prompt as written, with its placeholders unfilled', async () => {
    mockFetch.mockResolvedValue({ok: true, status: 200, text: () => Promise.resolve('Use {{clientId}}')});

    const {result} = renderHook(() => useGetCodingAgentPrompt('https://example.com/prompts/react.txt'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(mockFetch).toHaveBeenCalledWith('https://example.com/prompts/react.txt');
    expect(result.current.data).toBe('Use {{clientId}}');
  });

  it('fails when the docs site does not return the prompt', async () => {
    mockFetch.mockResolvedValue({ok: false, status: 404, text: () => Promise.resolve('')});

    const {result} = renderHook(() => useGetCodingAgentPrompt('https://example.com/prompts/react.txt'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });

  it('does not fetch without a prompt URL', () => {
    renderHook(() => useGetCodingAgentPrompt(undefined));

    expect(mockFetch).not.toHaveBeenCalled();
  });
});
