// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect, vi} from 'vitest';
import fetchAllPages, {MAX_PAGE_SIZE} from '../fetchAllPages';

const items = (count: number, from = 0): number[] => Array.from({length: count}, (_, index) => from + index);

describe('fetchAllPages', () => {
  it('returns a single short page without requesting another', async () => {
    const fetchPage = vi.fn().mockResolvedValue({items: items(3), totalResults: 3});

    await expect(fetchAllPages(fetchPage)).resolves.toEqual(items(3));
    expect(fetchPage).toHaveBeenCalledTimes(1);
    expect(fetchPage).toHaveBeenCalledWith(MAX_PAGE_SIZE, 0);
  });

  it('pages on from the number of items read so far until a short page', async () => {
    const fetchPage = vi
      .fn()
      .mockResolvedValueOnce({items: items(MAX_PAGE_SIZE), totalResults: MAX_PAGE_SIZE + 5})
      .mockResolvedValueOnce({items: items(5, MAX_PAGE_SIZE), totalResults: MAX_PAGE_SIZE + 5});

    await expect(fetchAllPages(fetchPage)).resolves.toHaveLength(MAX_PAGE_SIZE + 5);
    expect(fetchPage).toHaveBeenNthCalledWith(2, MAX_PAGE_SIZE, MAX_PAGE_SIZE);
  });

  it('stops after a full page that reaches the reported total', async () => {
    const fetchPage = vi.fn().mockResolvedValue({items: items(MAX_PAGE_SIZE), totalResults: MAX_PAGE_SIZE});

    await expect(fetchAllPages(fetchPage)).resolves.toHaveLength(MAX_PAGE_SIZE);
    expect(fetchPage).toHaveBeenCalledTimes(1);
  });

  it('keeps paging after a full page when no total is reported, until an empty page', async () => {
    const fetchPage = vi
      .fn()
      .mockResolvedValueOnce({items: items(MAX_PAGE_SIZE)})
      .mockResolvedValueOnce({items: []});

    await expect(fetchAllPages(fetchPage)).resolves.toHaveLength(MAX_PAGE_SIZE);
    expect(fetchPage).toHaveBeenCalledTimes(2);
  });

  it('rejects when a page fails to load', async () => {
    const fetchPage = vi.fn().mockRejectedValue(new Error('boom'));

    await expect(fetchAllPages(fetchPage)).rejects.toThrow('boom');
  });
});
